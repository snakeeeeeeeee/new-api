package claude

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	claudeIntegrityMaxBufferedEvents = 256
	claudeIntegrityMaxBufferedBytes  = 1 << 20
	claudeIntegrityMaxSSEEventBytes  = 64 << 20
)

type claudeIntegrityBlockState struct {
	open bool
}

type claudeIntegrityState struct {
	blocks       map[int]claudeIntegrityBlockState
	startedBlock bool
	messageStop  bool
	allowEmpty   bool
}

type claudeIntegrityObservation struct {
	firstBlock bool
	terminal   bool
}

func newClaudeIntegrityState(allowEmpty bool) *claudeIntegrityState {
	return &claudeIntegrityState{
		blocks:     make(map[int]claudeIntegrityBlockState),
		allowEmpty: allowEmpty,
	}
}

func (s *claudeIntegrityState) observe(event *dto.ClaudeResponse) (claudeIntegrityObservation, error) {
	if event == nil {
		return claudeIntegrityObservation{}, errors.New("nil Claude stream event")
	}
	if strings.TrimSpace(event.Type) == "" {
		return claudeIntegrityObservation{}, errors.New("Claude stream event has no type")
	}
	if s.messageStop {
		return claudeIntegrityObservation{}, errors.New("event received after message_stop")
	}

	switch event.Type {
	case "content_block_start":
		if event.Index == nil || *event.Index < 0 {
			return claudeIntegrityObservation{}, errors.New("content_block_start has invalid index")
		}
		if event.ContentBlock == nil || strings.TrimSpace(event.ContentBlock.Type) == "" {
			return claudeIntegrityObservation{}, errors.New("content_block_start has no block type")
		}
		index := *event.Index
		if _, exists := s.blocks[index]; exists {
			return claudeIntegrityObservation{}, fmt.Errorf("duplicate content_block_start index %d", index)
		}
		s.blocks[index] = claudeIntegrityBlockState{open: true}
		first := !s.startedBlock
		s.startedBlock = true
		return claudeIntegrityObservation{firstBlock: first}, nil
	case "content_block_delta":
		if event.Index == nil || *event.Index < 0 {
			return claudeIntegrityObservation{}, errors.New("content_block_delta has invalid index")
		}
		state, exists := s.blocks[*event.Index]
		if !exists || !state.open {
			return claudeIntegrityObservation{}, fmt.Errorf("content_block_delta references unknown index %d", *event.Index)
		}
		if event.Delta == nil || strings.TrimSpace(event.Delta.Type) == "" {
			return claudeIntegrityObservation{}, fmt.Errorf("content_block_delta index %d has no delta type", *event.Index)
		}
	case "content_block_stop":
		if event.Index == nil || *event.Index < 0 {
			return claudeIntegrityObservation{}, errors.New("content_block_stop has invalid index")
		}
		state, exists := s.blocks[*event.Index]
		if !exists || !state.open {
			return claudeIntegrityObservation{}, fmt.Errorf("content_block_stop references unknown index %d", *event.Index)
		}
		state.open = false
		s.blocks[*event.Index] = state
	case "message_stop":
		for index, state := range s.blocks {
			if state.open {
				return claudeIntegrityObservation{}, fmt.Errorf("message_stop received with open content block %d", index)
			}
		}
		if !s.startedBlock && !s.allowEmpty {
			return claudeIntegrityObservation{}, errors.New("message_stop received before any content block")
		}
		s.messageStop = true
		return claudeIntegrityObservation{terminal: true}, nil
	}
	return claudeIntegrityObservation{}, nil
}

type claudeIntegrityBufferedEvent struct {
	raw    string
	parsed dto.ClaudeResponse
}

type claudeIntegrityScanItem struct {
	data string
	err  error
}

func scanClaudeIntegrityEvents(resp *http.Response, done <-chan struct{}) <-chan claudeIntegrityScanItem {
	items := make(chan claudeIntegrityScanItem, 1)
	go func() {
		defer close(items)
		if resp == nil || resp.Body == nil {
			select {
			case items <- claudeIntegrityScanItem{err: errors.New("Claude stream response body is nil")}:
			case <-done:
			}
			return
		}
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), claudeIntegrityMaxSSEEventBytes)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
			if strings.HasPrefix(data, "[DONE]") {
				select {
				case items <- claudeIntegrityScanItem{err: errors.New("unexpected [DONE] before Claude message_stop")}:
				case <-done:
				}
				return
			}
			select {
			case items <- claudeIntegrityScanItem{data: data}:
			case <-done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case items <- claudeIntegrityScanItem{err: err}:
			case <-done:
			}
		}
	}()
	return items
}

func claudeIntegrityAllowsEmpty(info *relaycommon.RelayInfo) bool {
	if info == nil || info.Request == nil {
		return false
	}
	switch request := info.Request.(type) {
	case *dto.ClaudeRequest:
		return request.MaxTokens != nil && *request.MaxTokens == 0
	case *dto.GeneralOpenAIRequest:
		return request.MaxTokens != nil && *request.MaxTokens == 0
	default:
		return false
	}
}

func newClaudeIntegrityError(reason string, err error) *types.NewAPIError {
	if err == nil {
		err = errors.New(reason)
	}
	message := fmt.Sprintf("Claude response integrity failure (%s): %v", reason, err)
	return types.WithClaudeError(types.ClaudeError{
		Type:    "api_error",
		Message: message,
		Code:    string(types.ErrorCodeClaudeContentBlockMissing),
		Status:  http.StatusBadGateway,
	}, http.StatusBadGateway)
}

func newClaudeClientDisconnectedError(err error) *types.NewAPIError {
	if err == nil {
		err = errors.New("client disconnected")
	}
	return types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, 499, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

func writeClaudeIntegrityEvent(c *gin.Context, eventType string, data string) error {
	helper.SetEventStreamHeaders(c)
	_, err := c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)))
	if err != nil {
		return err
	}
	return flushClaudeIntegrityWriter(c)
}

func writeOpenAIIntegrityData(c *gin.Context, data string) error {
	helper.SetEventStreamHeaders(c)
	_, err := c.Writer.Write([]byte("data: " + data + "\n\n"))
	if err != nil {
		return err
	}
	return flushClaudeIntegrityWriter(c)
}

func flushClaudeIntegrityWriter(c *gin.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("flush panic recovered: %v", recovered)
		}
	}()
	if c == nil || c.Writer == nil {
		return errors.New("Claude integrity response writer is nil")
	}
	writer := http.ResponseWriter(c.Writer)
	for {
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		unwrapped := unwrapper.Unwrap()
		if unwrapped == nil || unwrapped == writer {
			break
		}
		writer = unwrapped
	}
	return http.NewResponseController(writer).Flush()
}

func claudeIntegrityDownstreamCommitted(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written()
}

func handleClaudeIntegrityStreamEvent(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, event *dto.ClaudeResponse, data string) (bool, *types.NewAPIError) {
	if claudeError := event.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return false, types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if event.StopReason != "" {
		maybeMarkClaudeRefusal(c, event.StopReason)
	}
	if event.Delta != nil && event.Delta.StopReason != nil {
		maybeMarkClaudeRefusal(c, *event.Delta.StopReason)
	}

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		FormatClaudeResponseInfo(event, nil, claudeInfo)
		if event.Type == "message_start" && event.Message != nil {
			info.UpstreamModelName = event.Message.Model
			data = patchClaudeCacheTTLBillingCompatUsageData(data, info, "message.usage", event.Message.Usage)
		} else if event.Type == "message_delta" {
			patchUsage := buildMessageDeltaPatchUsage(event, claudeInfo)
			if !shouldSkipClaudeMessageDeltaUsagePatch(info) {
				data = patchClaudeMessageDeltaUsageData(data, patchUsage)
			}
			data = patchClaudeCacheTTLBillingCompatUsageData(data, info, "usage", patchUsage)
		}
		if err := writeClaudeIntegrityEvent(c, event.Type, data); err != nil {
			return false, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		return true, nil
	case types.RelayFormatOpenAI:
		response := StreamResponseClaude2OpenAI(event)
		if !FormatClaudeResponseInfo(event, response, claudeInfo) {
			return false, nil
		}
		jsonData, err := common.Marshal(response)
		if err != nil {
			return false, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		if err = writeOpenAIIntegrityData(c, string(jsonData)); err != nil {
			return false, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		return true, nil
	default:
		return false, types.NewError(fmt.Errorf("unsupported Claude integrity relay format %s", info.RelayFormat), types.ErrorCodeBadResponseBody)
	}
}

func finalizeClaudeIntegrityUsage(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) *dto.Usage {
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &dto.Usage{}
	}
	if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
		claudeInfo.Usage = service.ResponseText2Usage(c, claudeInfo.ResponseText.String(), info.UpstreamModelName, claudeInfo.Usage.PromptTokens)
	}
	claudeInfo.Usage.UsageSemantic = "anthropic"
	return claudeInfo.Usage
}

func finishClaudeIntegrityStream(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) *types.NewAPIError {
	finalizeClaudeIntegrityUsage(c, info, claudeInfo)
	if info.RelayFormat != types.RelayFormatOpenAI {
		return nil
	}
	if info.ShouldIncludeUsage {
		openAIUsage := buildDownstreamOpenAIStyleUsageFromClaudeUsage(info, claudeInfo.Usage)
		response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.UpstreamModelName, openAIUsage)
		data, err := common.Marshal(response)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		if err = writeOpenAIIntegrityData(c, string(data)); err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		info.SendResponseCount++
	}
	if err := writeOpenAIIntegrityData(c, "[DONE]"); err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	info.SendResponseCount++
	return nil
}

func sendClaudeIntegrityStreamError(c *gin.Context, info *relaycommon.RelayInfo) {
	message := "Upstream Claude stream ended before completion."
	if info.RelayFormat == types.RelayFormatClaude {
		payload := struct {
			Type  string            `json:"type"`
			Error types.ClaudeError `json:"error"`
		}{
			Type: "error",
			Error: types.ClaudeError{
				Type:    "api_error",
				Message: message,
				Code:    types.ErrorCodeClaudeStreamIncomplete,
			},
		}
		data, err := common.Marshal(payload)
		if err == nil && writeClaudeIntegrityEvent(c, "error", string(data)) == nil {
			info.SendResponseCount++
		}
		return
	}
	payload := struct {
		Error types.OpenAIError `json:"error"`
	}{
		Error: types.OpenAIError{
			Message: message,
			Type:    "new_api_error",
			Code:    types.ErrorCodeClaudeStreamIncomplete,
		},
	}
	data, err := common.Marshal(payload)
	if err == nil && writeOpenAIIntegrityData(c, string(data)) == nil {
		info.SendResponseCount++
	}
}

func markClaudeIntegrityStreamIncomplete(c *gin.Context, info *relaycommon.RelayInfo, reason string, upstreamFailure bool) {
	common.SetContextKey(c, constant.ContextKeyClaudeStreamIncomplete, true)
	common.SetContextKey(c, constant.ContextKeyClaudeStreamIncompleteReason, reason)
	logger.LogError(c, "claude_stream_incomplete: "+reason)
	if upstreamFailure {
		service.RecordAggregateRouteRPMFailure(c, info.OriginModelName)
		routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
		service.RecordAggregateRouteSmartFailure(c, info.OriginModelName, routeGroup, http.StatusBadGateway)
	}
}

func ClaudeIntegrityStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)
	defer info.EndClaudeResponseIntegrityAttempt()

	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	state := newClaudeIntegrityState(claudeIntegrityAllowsEmpty(info))
	items := scanClaudeIntegrityEvents(resp, info.ClaudeResponseIntegrityAttemptDone())
	buffer := make([]claudeIntegrityBufferedEvent, 0, 8)
	bufferedBytes := 0
	committed := false
	normalStop := false
	firstEventRecorded := false

	writeEvent := func(event *dto.ClaudeResponse, raw string) *types.NewAPIError {
		wrote, apiErr := handleClaudeIntegrityStreamEvent(c, info, claudeInfo, event, raw)
		if apiErr != nil {
			return apiErr
		}
		if wrote {
			info.SetFirstResponseTime()
			info.SendResponseCount++
		}
		return nil
	}

	for {
		select {
		case <-c.Request.Context().Done():
			info.CancelClaudeResponseIntegrityAttempt()
			if !committed {
				return nil, newClaudeClientDisconnectedError(c.Request.Context().Err())
			}
			markClaudeIntegrityStreamIncomplete(c, info, "client_disconnected", false)
			return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
		case <-info.ClaudeResponseIntegrityAttemptDone():
			if info.ClaudeResponseIntegrityFirstBlockTimedOut() && !committed {
				return nil, newClaudeIntegrityError("first_block_timeout", errors.New("first content block timeout"))
			}
			if c.Request.Context().Err() != nil {
				if !committed {
					return nil, newClaudeClientDisconnectedError(c.Request.Context().Err())
				}
				markClaudeIntegrityStreamIncomplete(c, info, "client_disconnected", false)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}
		case item, ok := <-items:
			if c.Request.Context().Err() != nil {
				info.CancelClaudeResponseIntegrityAttempt()
				if !committed {
					return nil, newClaudeClientDisconnectedError(c.Request.Context().Err())
				}
				markClaudeIntegrityStreamIncomplete(c, info, "client_disconnected", false)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}
			if !ok {
				if normalStop {
					return claudeInfo.Usage, nil
				}
				if !committed {
					return nil, newClaudeIntegrityError("eof_before_first_block", io.ErrUnexpectedEOF)
				}
				markClaudeIntegrityStreamIncomplete(c, info, "eof_before_message_stop", true)
				sendClaudeIntegrityStreamError(c, info)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}
			if item.err != nil {
				if !committed {
					return nil, newClaudeIntegrityError("scanner_error_before_first_block", item.err)
				}
				markClaudeIntegrityStreamIncomplete(c, info, "scanner_error", true)
				sendClaudeIntegrityStreamError(c, info)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}
			if !firstEventRecorded {
				firstEventRecorded = true
				elapsed := info.ClaudeResponseIntegrityAttemptElapsed().Milliseconds()
				common.SetContextKey(c, constant.ContextKeyUpstreamFirstEventMs, int(elapsed))
			}
			info.ReceivedResponseCount++
			var event dto.ClaudeResponse
			if err := common.UnmarshalJsonStr(item.data, &event); err != nil {
				if !committed {
					return nil, newClaudeIntegrityError("malformed_json_before_first_block", err)
				}
				markClaudeIntegrityStreamIncomplete(c, info, "malformed_json", true)
				sendClaudeIntegrityStreamError(c, info)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}
			if claudeError := event.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
				if !committed {
					return nil, types.WithClaudeError(*claudeError, http.StatusInternalServerError)
				}
				markClaudeIntegrityStreamIncomplete(c, info, "upstream_error_event", true)
				sendClaudeIntegrityStreamError(c, info)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}

			observation, observeErr := state.observe(&event)
			if observeErr != nil {
				if !committed {
					return nil, newClaudeIntegrityError("invalid_sequence_before_first_block", observeErr)
				}
				markClaudeIntegrityStreamIncomplete(c, info, observeErr.Error(), true)
				sendClaudeIntegrityStreamError(c, info)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}

			if !committed {
				bufferedBytes += len(item.data)
				buffer = append(buffer, claudeIntegrityBufferedEvent{raw: item.data, parsed: event})
				if len(buffer) > claudeIntegrityMaxBufferedEvents || bufferedBytes > claudeIntegrityMaxBufferedBytes {
					return nil, newClaudeIntegrityError("first_block_buffer_limit", errors.New("first content block buffer limit exceeded"))
				}
				if !observation.firstBlock && !(observation.terminal && state.allowEmpty) {
					continue
				}
				info.MarkClaudeResponseIntegrityFirstBlock()
				for i := range buffer {
					if apiErr := writeEvent(&buffer[i].parsed, buffer[i].raw); apiErr != nil {
						if claudeIntegrityDownstreamCommitted(c) {
							committed = true
							markClaudeIntegrityStreamIncomplete(c, info, "downstream_write_error", false)
							return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
						}
						return nil, apiErr
					}
					if claudeIntegrityDownstreamCommitted(c) {
						committed = true
					}
				}
				buffer = nil
				bufferedBytes = 0
				committed = true
			} else if apiErr := writeEvent(&event, item.data); apiErr != nil {
				markClaudeIntegrityStreamIncomplete(c, info, "downstream_write_error", false)
				return finalizeClaudeIntegrityUsage(c, info, claudeInfo), nil
			}

			if observation.terminal {
				normalStop = true
				if apiErr := finishClaudeIntegrityStream(c, info, claudeInfo); apiErr != nil {
					markClaudeIntegrityStreamIncomplete(c, info, "downstream_final_write_error", false)
					return claudeInfo.Usage, nil
				}
				return claudeInfo.Usage, nil
			}
		}
	}
}

func ClaudeIntegrityHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)
	defer info.EndClaudeResponseIntegrityAttempt()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return nil, newClaudeClientDisconnectedError(c.Request.Context().Err())
		}
		return nil, newClaudeIntegrityError("read_error", err)
	}
	var response dto.ClaudeResponse
	if err = common.Unmarshal(responseBody, &response); err != nil {
		return nil, newClaudeIntegrityError("malformed_json", err)
	}
	if claudeError := response.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return nil, types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if len(response.Content) == 0 && !claudeIntegrityAllowsEmpty(info) {
		return nil, newClaudeIntegrityError("empty_content", errors.New("Claude response content is empty"))
	}
	for _, block := range response.Content {
		if strings.TrimSpace(block.Type) == "" {
			return nil, newClaudeIntegrityError("content_block_without_type", errors.New("Claude response content block type is empty"))
		}
	}
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	if handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody); handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}
