package aws

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockruntimeTypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go/auth/bearer"
)

// getAwsErrorStatusCode extracts HTTP status code from AWS SDK error
func getAwsErrorStatusCode(err error) int {
	// Check for HTTP response error which contains status code
	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatusCode()
	}
	// Default to 500 if we can't determine the status code
	return http.StatusInternalServerError
}

func newAwsInvokeContext() (context.Context, context.CancelFunc) {
	if common.RelayTimeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), time.Duration(common.RelayTimeout)*time.Second)
}

func newAwsClaudeIntegrityInvokeContext(c *gin.Context, info *relaycommon.RelayInfo) (context.Context, context.CancelFunc) {
	parent := c.Request.Context()
	var cancel context.CancelFunc
	if common.RelayTimeout > 0 {
		parent, cancel = context.WithTimeout(parent, time.Duration(common.RelayTimeout)*time.Second)
	} else {
		parent, cancel = context.WithCancel(parent)
	}
	return info.BeginClaudeResponseIntegrityAttempt(parent), cancel
}

func awsClaudeIntegrityInvokeError(c *gin.Context, info *relaycommon.RelayInfo, operation string, err error) *types.NewAPIError {
	relaycommon.LogClaudeToolSchemaCompatOriginalSchemasOnError(info, err)
	if c.Request.Context().Err() != nil {
		return types.NewErrorWithStatusCode(c.Request.Context().Err(), types.ErrorCodeDoRequestFailed, 499, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	if info.ClaudeResponseIntegrityFirstBlockTimedOut() {
		return types.WithClaudeError(types.ClaudeError{
			Type:    "api_error",
			Message: errors.Wrap(err, "Claude first content block timeout").Error(),
			Code:    string(types.ErrorCodeClaudeContentBlockMissing),
			Status:  http.StatusBadGateway,
		}, http.StatusBadGateway)
	}
	return types.NewOpenAIError(errors.Wrap(err, operation), types.ErrorCodeAwsInvokeError, getAwsErrorStatusCode(err))
}

func newAwsClient(c *gin.Context, info *relaycommon.RelayInfo) (*bedrockruntime.Client, error) {
	var (
		httpClient *http.Client
		err        error
	)
	if info.ChannelSetting.Proxy != "" {
		httpClient, err = service.NewProxyHttpClient(info.ChannelSetting.Proxy)
		if err != nil {
			return nil, fmt.Errorf("new proxy http client failed: %w", err)
		}
	} else {
		httpClient = service.GetHttpClient()
	}

	awsSecret := strings.Split(info.ApiKey, "|")
	var client *bedrockruntime.Client
	switch len(awsSecret) {
	case 2:
		apiKey := awsSecret[0]
		region := awsSecret[1]
		client = bedrockruntime.New(bedrockruntime.Options{
			Region:                  region,
			BearerAuthTokenProvider: bearer.StaticTokenProvider{Token: bearer.Token{Value: apiKey}},
			HTTPClient:              httpClient,
		})
	case 3:
		ak := awsSecret[0]
		sk := awsSecret[1]
		region := awsSecret[2]
		client = bedrockruntime.New(bedrockruntime.Options{
			Region:      region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(ak, sk, "")),
			HTTPClient:  httpClient,
		})
	default:
		return nil, errors.New("invalid aws secret key")
	}

	return client, nil
}

func doAwsClientRequest(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor, requestBody io.Reader) (any, error) {
	awsCli, err := newAwsClient(c, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelAwsClientError)
	}
	a.AwsClient = awsCli

	// 获取对应的AWS模型ID
	awsModelId := getAwsModelID(info.UpstreamModelName)

	awsRegionPrefix := getAwsRegionPrefix(awsCli.Options().Region)
	canCrossRegion := awsModelCanCrossRegion(awsModelId, awsRegionPrefix)
	if canCrossRegion {
		awsModelId = awsModelCrossRegion(awsModelId, awsRegionPrefix)
	}

	// init empty request.header
	requestHeader := http.Header{}
	a.SetupRequestHeader(c, &requestHeader, info)
	headerOverride, err := channel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for key, value := range headerOverride {
		requestHeader.Set(key, value)
	}

	if isNovaModel(awsModelId) {
		var novaReq *NovaRequest
		err = common.DecodeJson(requestBody, &novaReq)
		if err != nil {
			return nil, types.NewError(errors.Wrap(err, "decode nova request fail"), types.ErrorCodeBadRequestBody)
		}

		// 使用InvokeModel API，但使用Nova格式的请求体
		awsReq := &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(awsModelId),
			Accept:      aws.String("application/json"),
			ContentType: aws.String("application/json"),
		}

		reqBody, err := common.Marshal(novaReq)
		if err != nil {
			return nil, types.NewError(errors.Wrap(err, "marshal nova request"), types.ErrorCodeBadResponseBody)
		}
		awsReq.Body = reqBody
		a.AwsReq = awsReq
		return nil, nil
	} else {
		awsClaudeReq, err := formatRequest(requestBody, requestHeader)
		if err != nil {
			return nil, types.NewError(errors.Wrap(err, "format aws request fail"), types.ErrorCodeBadRequestBody)
		}

		if info.IsStream {
			awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
				ModelId:     aws.String(awsModelId),
				Accept:      aws.String("application/json"),
				ContentType: aws.String("application/json"),
			}
			awsReq.Body, err = buildAwsRequestBody(c, info, awsClaudeReq)
			if err != nil {
				return nil, types.NewError(errors.Wrap(err, "marshal aws request fail"), types.ErrorCodeBadRequestBody)
			}
			a.AwsReq = awsReq
			return nil, nil
		} else {
			awsReq := &bedrockruntime.InvokeModelInput{
				ModelId:     aws.String(awsModelId),
				Accept:      aws.String("application/json"),
				ContentType: aws.String("application/json"),
			}
			awsReq.Body, err = buildAwsRequestBody(c, info, awsClaudeReq)
			if err != nil {
				return nil, types.NewError(errors.Wrap(err, "marshal aws request fail"), types.ErrorCodeBadRequestBody)
			}
			a.AwsReq = awsReq
			return nil, nil
		}
	}
}

// buildAwsRequestBody prepares the payload for AWS requests, applying passthrough rules when enabled.
func buildAwsRequestBody(c *gin.Context, info *relaycommon.RelayInfo, awsClaudeReq any) ([]byte, error) {
	info.MarkFinalRequestRelayFormat(types.RelayFormatClaude)
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, errors.Wrap(err, "get request body for pass-through fail")
		}
		body, err := storage.Bytes()
		if err != nil {
			return nil, errors.Wrap(err, "get request body bytes fail")
		}
		var data map[string]interface{}
		if err := common.Unmarshal(body, &data); err != nil {
			return nil, errors.Wrap(err, "pass-through unmarshal request body fail")
		}
		if model_setting.GetClaudeSettings().OpenAIToolCallCompatEnabled && isAwsPassThroughOpenAIChatBody(data) {
			var openAIReq dto.GeneralOpenAIRequest
			if err := common.Unmarshal(body, &openAIReq); err != nil {
				return nil, errors.Wrap(err, "pass-through unmarshal openai request body fail")
			}
			claudeReq, err := claude.RequestOpenAI2ClaudeMessage(c, info, openAIReq)
			if err != nil {
				return nil, errors.Wrap(err, "pass-through convert openai request to claude fail")
			}
			if compatErr := relaycommon.NormalizeClaudeRequestCompat(claudeReq, info); compatErr != nil {
				return nil, compatErr
			}
			bodyWithoutModel, err := common.Marshal(claudeReq)
			if err != nil {
				return nil, errors.Wrap(err, "pass-through marshal converted claude request fail")
			}
			if err := common.Unmarshal(bodyWithoutModel, &data); err != nil {
				return nil, errors.Wrap(err, "pass-through unmarshal converted claude request body fail")
			}
			delete(data, "model")
			delete(data, "stream")
			ensureAwsClaudeAnthropicVersion(data, awsClaudeReq)
			relaycommon.CaptureClaudeToolSchemaCompatFinalSchemas(data["tools"], info)
			finalBody, err := common.Marshal(data)
			if err != nil {
				return nil, err
			}
			relaycommon.CaptureClaudeCacheTTLBillingCompat(info, finalBody)
			return finalBody, nil
		}
		if !model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled && model_setting.GetClaudeSettings().DropDefaultSamplingForOpusEnabled {
			normalizedBody, compatErr := relaycommon.NormalizeClaudeSamplingCompatJSON(body, info)
			if compatErr != nil {
				return nil, compatErr
			}
			var normalizedData map[string]interface{}
			if err := common.Unmarshal(normalizedBody, &normalizedData); err != nil {
				return nil, errors.Wrap(err, "pass-through unmarshal sampling normalized request body fail")
			}
			data = normalizedData
		}
		delete(data, "model")
		delete(data, "stream")
		ensureAwsClaudeAnthropicVersion(data, awsClaudeReq)
		bodyWithoutModel, err := common.Marshal(data)
		if err != nil {
			return nil, errors.Wrap(err, "pass-through marshal request body fail")
		}
		if relaycommon.ShouldApplyClaudeToolSchemaCompat(info) {
			normalizedBody, err := relaycommon.NormalizeClaudeRequestToolSchemasInJSON(bodyWithoutModel, info)
			if err != nil {
				return nil, errors.Wrap(err, "pass-through normalize tool schema fail")
			}
			bodyWithoutModel = normalizedBody
			var normalizedData map[string]interface{}
			if err := common.Unmarshal(bodyWithoutModel, &normalizedData); err != nil {
				return nil, errors.Wrap(err, "pass-through unmarshal tool schema normalized request body fail")
			}
			data = normalizedData
		}
		if model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled {
			normalizedBody, compatErr := relaycommon.NormalizeClaudeRequestCompatJSON(bodyWithoutModel, info)
			if compatErr != nil {
				return nil, compatErr
			}
			bodyWithoutModel = normalizedBody
			var normalizedData map[string]interface{}
			if err := common.Unmarshal(bodyWithoutModel, &normalizedData); err != nil {
				return nil, errors.Wrap(err, "pass-through unmarshal normalized request body fail")
			}
			data = normalizedData
		}
		relaycommon.CaptureClaudeToolSchemaCompatFinalSchemas(data["tools"], info)
		finalBody, err := common.Marshal(data)
		if err != nil {
			return nil, err
		}
		relaycommon.CaptureClaudeCacheTTLBillingCompat(info, finalBody)
		return finalBody, nil
	}
	if req, ok := awsClaudeReq.(*dto.ClaudeRequest); ok {
		if compatErr := relaycommon.NormalizeClaudeRequestCompat(req, info); compatErr != nil {
			return nil, compatErr
		}
	}
	if req, ok := awsClaudeReq.(*AwsClaudeRequest); ok {
		relaycommon.NormalizeClaudeToolsValue(req.Tools, info)
		relaycommon.CaptureClaudeToolSchemaCompatFinalSchemas(req.Tools, info)
	}
	finalBody, err := common.Marshal(awsClaudeReq)
	if err != nil {
		return nil, err
	}
	relaycommon.CaptureClaudeCacheTTLBillingCompat(info, finalBody)
	return finalBody, nil
}

func ensureAwsClaudeAnthropicVersion(data map[string]any, awsClaudeReq any) {
	if data == nil {
		return
	}
	if strings.TrimSpace(common.Interface2String(data["anthropic_version"])) != "" {
		return
	}
	if req, ok := awsClaudeReq.(*AwsClaudeRequest); ok && strings.TrimSpace(req.AnthropicVersion) != "" {
		data["anthropic_version"] = req.AnthropicVersion
		return
	}
	data["anthropic_version"] = "bedrock-2023-05-31"
}

func isAwsPassThroughOpenAIChatBody(data map[string]any) bool {
	messages, ok := data["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}
	if _, ok := data["input"]; ok {
		return false
	}
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(common.Interface2String(message["role"]))
		if role == "tool" || role == "developer" || role == "system" {
			return true
		}
		if _, ok := message["tool_calls"]; ok {
			return true
		}
		if _, ok := message["tool_call_id"]; ok {
			return true
		}
	}
	tools, ok := data["tools"].([]any)
	if !ok {
		return false
	}
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := tool["function"]; ok {
			return true
		}
	}
	return false
}

func getAwsRegionPrefix(awsRegionId string) string {
	parts := strings.Split(awsRegionId, "-")
	regionPrefix := ""
	if len(parts) > 0 {
		regionPrefix = parts[0]
	}
	return regionPrefix
}

func awsModelCanCrossRegion(awsModelId, awsRegionPrefix string) bool {
	regionSet, exists := awsModelCanCrossRegionMap[awsModelId]
	return exists && regionSet[awsRegionPrefix]
}

func awsModelCrossRegion(awsModelId, awsRegionPrefix string) string {
	modelPrefix, find := awsRegionCrossModelPrefixMap[awsRegionPrefix]
	if !find {
		return awsModelId
	}
	return modelPrefix + "." + awsModelId
}

func getAwsModelID(requestModel string) string {
	if awsModelIDName, ok := awsModelIDMap[requestModel]; ok {
		return awsModelIDName
	}
	return requestModel
}

func awsHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.NewAPIError, *dto.Usage) {
	if info.ClaudeResponseIntegrityEnabled && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return awsClaudeIntegrityHandler(c, info, a)
	}

	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModel(ctx, a.AwsReq.(*bedrockruntime.InvokeModelInput))
	if err != nil {
		relaycommon.LogClaudeToolSchemaCompatOriginalSchemasOnError(info, err)
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModel"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}

	claudeInfo := &claude.ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}

	// 复制上游 Content-Type 到客户端响应头
	if awsResp.ContentType != nil && *awsResp.ContentType != "" {
		c.Writer.Header().Set("Content-Type", *awsResp.ContentType)
	}

	handlerErr := claude.HandleClaudeResponseData(c, info, claudeInfo, nil, awsResp.Body)
	if handlerErr != nil {
		return handlerErr, nil
	}
	return nil, claudeInfo.Usage
}

func awsClaudeIntegrityHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.NewAPIError, *dto.Usage) {
	ctx, cancel := newAwsClaudeIntegrityInvokeContext(c, info)
	defer cancel()
	awsResp, err := a.AwsClient.InvokeModel(ctx, a.AwsReq.(*bedrockruntime.InvokeModelInput))
	if err != nil {
		apiErr := awsClaudeIntegrityInvokeError(c, info, "InvokeModel", err)
		info.EndClaudeResponseIntegrityAttempt()
		return apiErr, nil
	}

	contentType := "application/json"
	if awsResp.ContentType != nil && *awsResp.ContentType != "" {
		contentType = *awsResp.ContentType
		c.Writer.Header().Set("Content-Type", contentType)
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewReader(awsResp.Body)),
	}
	usage, apiErr := claude.ClaudeIntegrityHandler(c, resp, info)
	return apiErr, usage
}

type awsClaudeIntegrityResponseStream interface {
	Events() <-chan bedrockruntimeTypes.ResponseStream
	Err() error
}

func awsClaudeIntegrityStreamBody(stream awsClaudeIntegrityResponseStream, done <-chan struct{}) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		for event := range stream.Events() {
			select {
			case <-done:
				_ = writer.Close()
				return
			default:
			}
			switch value := event.(type) {
			case *bedrockruntimeTypes.ResponseStreamMemberChunk:
				frame := make([]byte, 0, len(value.Value.Bytes)+8)
				frame = append(frame, "data: "...)
				frame = append(frame, value.Value.Bytes...)
				frame = append(frame, '\n', '\n')
				if _, err := writer.Write(frame); err != nil {
					return
				}
			case *bedrockruntimeTypes.UnknownUnionMember:
				_ = writer.CloseWithError(fmt.Errorf("unknown AWS response stream tag %q", value.Tag))
				return
			default:
				_ = writer.CloseWithError(errors.New("nil or unknown AWS response stream type"))
				return
			}
		}
		if err := stream.Err(); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()
	return reader
}

func awsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.NewAPIError, *dto.Usage) {
	if info.ClaudeResponseIntegrityEnabled && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return awsClaudeIntegrityStreamHandler(c, info, a)
	}
	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModelWithResponseStream(ctx, a.AwsReq.(*bedrockruntime.InvokeModelWithResponseStreamInput))
	if err != nil {
		relaycommon.LogClaudeToolSchemaCompatOriginalSchemasOnError(info, err)
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModelWithResponseStream"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	claudeInfo := &claude.ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}

	for event := range stream.Events() {
		switch v := event.(type) {
		case *bedrockruntimeTypes.ResponseStreamMemberChunk:
			info.SetFirstResponseTime()
			respErr := claude.HandleStreamResponseData(c, info, claudeInfo, string(v.Value.Bytes))
			if respErr != nil {
				return respErr, nil
			}
		case *bedrockruntimeTypes.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)
			return types.NewError(errors.New("unknown response type"), types.ErrorCodeInvalidRequest), nil
		default:
			fmt.Println("union is nil or unknown type")
			return types.NewError(errors.New("nil or unknown response type"), types.ErrorCodeInvalidRequest), nil
		}
	}

	claude.HandleStreamFinalResponse(c, info, claudeInfo)
	return nil, claudeInfo.Usage
}

func awsClaudeIntegrityStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.NewAPIError, *dto.Usage) {
	ctx, cancel := newAwsClaudeIntegrityInvokeContext(c, info)
	defer cancel()
	awsResp, err := a.AwsClient.InvokeModelWithResponseStream(ctx, a.AwsReq.(*bedrockruntime.InvokeModelWithResponseStreamInput))
	if err != nil {
		apiErr := awsClaudeIntegrityInvokeError(c, info, "InvokeModelWithResponseStream", err)
		info.EndClaudeResponseIntegrityAttempt()
		return apiErr, nil
	}
	stream := awsResp.GetStream()
	defer stream.Close()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       awsClaudeIntegrityStreamBody(stream, info.ClaudeResponseIntegrityAttemptDone()),
	}
	usage, apiErr := claude.ClaudeIntegrityStreamHandler(c, resp, info)
	return apiErr, usage
}

// Nova模型处理函数
func handleNovaRequest(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.NewAPIError, *dto.Usage) {

	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModel(ctx, a.AwsReq.(*bedrockruntime.InvokeModelInput))
	if err != nil {
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModel"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}

	// 解析Nova响应
	var novaResp struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}

	if err := common.Unmarshal(awsResp.Body, &novaResp); err != nil {
		return types.NewError(errors.Wrap(err, "unmarshal nova response"), types.ErrorCodeBadResponseBody), nil
	}

	// 构造OpenAI格式响应
	response := dto.OpenAITextResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   info.UpstreamModelName,
		Choices: []dto.OpenAITextResponseChoice{{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: novaResp.Output.Message.Content[0].Text,
			},
			FinishReason: "stop",
		}},
		Usage: dto.Usage{
			PromptTokens:     novaResp.Usage.InputTokens,
			CompletionTokens: novaResp.Usage.OutputTokens,
			TotalTokens:      novaResp.Usage.TotalTokens,
		},
	}

	c.JSON(http.StatusOK, response)
	return nil, &response.Usage
}
