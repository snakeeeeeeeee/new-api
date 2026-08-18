package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)
	beginOpenAIResponseDiagnostics(c, openAIResponsesDiagnosticProtocol)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, openAIIntegrityReadError(c, info, openAIIntegrityResponses, err, responseBody)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, openAIIntegrityParseError(info, openAIIntegrityResponses, err, responseBody)
	}
	observation := &openAIResponseObservation{}
	observation.observeResponsesResponse(&responsesResponse, true)
	observation.TerminalEvent = observation.ResponseStatus

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		service.NormalizeResponsesInputUsage(&usage, responsesResponse.Usage)
	}
	if oaiError := responsesResponse.GetOpenAIError(); openAIIntegrityErrorPresent(oaiError) {
		return nil, openAIIntegrityResponseError(info, openAIIntegrityResponses, oaiError, resp.StatusCode, responseBody)
	}
	if integrityErr := validateOpenAIResponsesResponseIntegrity(info, &responsesResponse, responseBody, false); integrityErr != nil {
		return nil, integrityErr
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)
	observation.capture(c, info, openAIResponsesDiagnosticProtocol, false, &usage, responseBody, responseBody)
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func isResponsesTerminalStreamType(eventType string) bool {
	switch eventType {
	case "response.completed", "response.failed", "response.error", "response.incomplete", "response.cancelled":
		return true
	default:
		return false
	}
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)
	beginOpenAIResponseDiagnostics(c, openAIResponsesDiagnosticProtocol)
	observation := &openAIResponseObservation{}

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	streamStartedAt := time.Now()
	streamSequence := 0

	handleStreamData := func(data string) bool {
		recordOpenAIResponseUpstream(c, data)

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err == nil {
			streamSequence++
			dumpResponsesStreamEvent(c, streamSequence, data, streamResponse)
			sendResponsesStreamData(c, streamResponse, data)
			terminal := isResponsesTerminalStreamType(streamResponse.Type)
			if streamResponse.Item != nil {
				observation.observeResponsesOutput(streamResponse.Item, false)
			}
			if terminal {
				observation.markResponsesTerminal(streamResponse.Type)
				observation.observeResponsesResponse(streamResponse.Response, true)
			}
			if strings.Contains(streamResponse.Type, "reasoning") && strings.HasSuffix(streamResponse.Type, ".delta") {
				observation.ReasoningText.WriteString(streamResponse.Delta)
			}
			switch streamResponse.Type {
			case "response.completed":
				markResponsesStreamStopReason(c, streamResponse.Type)
				if streamResponse.Response != nil {
					if streamResponse.Response.Usage != nil {
						if streamResponse.Response.Usage.InputTokens != 0 {
							usage.PromptTokens = streamResponse.Response.Usage.InputTokens
						}
						if streamResponse.Response.Usage.OutputTokens != 0 {
							usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
						}
						if streamResponse.Response.Usage.TotalTokens != 0 {
							usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
						}
						service.NormalizeResponsesInputUsage(usage, streamResponse.Response.Usage)
					}
					if streamResponse.Response.HasImageGenerationCall() {
						c.Set("image_generation_call", true)
						c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
						c.Set("image_generation_call_size", streamResponse.Response.GetSize())
					}
				}
				return false
			case "response.output_text.delta":
				// 处理输出文本
				responseTextBuilder.WriteString(streamResponse.Delta)
				observation.VisibleText.WriteString(streamResponse.Delta)
			case dto.ResponsesOutputTypeItemDone:
				// 函数调用处理
				if streamResponse.Item != nil {
					switch streamResponse.Item.Type {
					case dto.BuildInCallWebSearchCall:
						if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
							if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
								webSearchTool.CallCount++
							}
						}
					}
				}
			}
			if terminal {
				markResponsesStreamStopReason(c, streamResponse.Type)
				return false
			}
		} else {
			streamSequence++
			dumpResponsesStreamParseError(c, streamSequence, err)
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
		}
		return true
	}
	integrityResult := openAIIntegrityStreamResult{}
	if info.OpenAIResponseIntegrityEnabled {
		var integrityErr *types.NewAPIError
		integrityResult, integrityErr = runOpenAIIntegrityStream(c, info, resp, openAIIntegrityResponses, handleStreamData)
		if integrityErr != nil {
			return nil, integrityErr
		}
		logOpenAIIntegrityPostCommitFailure(c, integrityResult)
	} else {
		helper.StreamScannerHandler(c, resp, info, handleStreamData)
	}
	dumpResponsesStreamSummary(c, streamStartedAt, streamSequence, info.ReceivedResponseCount)

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
			observation.UsageSource = "mixed_local_output_estimate"
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		observation.UsageSource = "mixed_local_estimate"
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	observation.capture(c, info, openAIResponsesDiagnosticProtocol, true, usage, nil, nil)

	return usage, nil
}
