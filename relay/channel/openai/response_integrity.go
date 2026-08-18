package openai

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
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	openAIIntegrityMaxBufferedEvents = 256
	openAIIntegrityMaxBufferedBytes  = 1 << 20
	openAIIntegrityMaxSSEEventBytes  = 64 << 20
)

type openAIIntegrityProtocol string

const (
	openAIIntegrityChat            openAIIntegrityProtocol = "openai_chat"
	openAIIntegrityResponses       openAIIntegrityProtocol = "openai_responses"
	openAIIntegrityResponsesToChat openAIIntegrityProtocol = "openai_responses_to_chat"
)

type openAIIntegrityScanItem struct {
	data       string
	doneMarker bool
	err        error
}

type openAIIntegrityEvent struct {
	meaningful bool
	terminal   bool
	allowEmpty bool
	terminalID string
	failure    *types.NewAPIError
}

type openAIIntegrityStreamResult struct {
	MeaningfulOutputSeen bool
	ResponseCommitted    bool
	TerminalSeen         bool
	TerminalEvent        string
	StopReason           string
	BufferedEvents       int
	BufferedBytes        int
	PostCommitFailure    bool
}

func scanOpenAIIntegrityEvents(resp *http.Response, done <-chan struct{}) <-chan openAIIntegrityScanItem {
	items := make(chan openAIIntegrityScanItem, 1)
	go func() {
		defer close(items)
		if resp == nil || resp.Body == nil {
			select {
			case items <- openAIIntegrityScanItem{err: errors.New("OpenAI stream response body is nil")}:
			case <-done:
			}
			return
		}
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), openAIIntegrityMaxSSEEventBytes)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
			item := openAIIntegrityScanItem{data: data}
			if strings.HasPrefix(data, "[DONE]") {
				item.doneMarker = true
			}
			select {
			case items <- item:
			case <-done:
				return
			}
			if item.doneMarker {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case items <- openAIIntegrityScanItem{err: err}:
			case <-done:
			}
		}
	}()
	return items
}

func runOpenAIIntegrityStream(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	resp *http.Response,
	protocol openAIIntegrityProtocol,
	handle func(data string) bool,
) (openAIIntegrityStreamResult, *types.NewAPIError) {
	result := openAIIntegrityStreamResult{}
	if info == nil || !info.OpenAIResponseIntegrityEnabled {
		return result, nil
	}

	scanDone := make(chan struct{})
	defer close(scanDone)
	items := scanOpenAIIntegrityEvents(resp, scanDone)
	buffer := make([]string, 0, 8)

	flushBuffer := func() bool {
		for _, data := range buffer {
			handled := handle(data)
			if c.Writer.Written() {
				result.ResponseCommitted = true
			}
			if !handled {
				return false
			}
		}
		buffer = nil
		return true
	}

	for {
		select {
		case <-c.Request.Context().Done():
			result.StopReason = "client_disconnected"
			markOpenAIIntegrityStopReason(c, result.StopReason)
			if !result.MeaningfulOutputSeen {
				return result, newOpenAIIntegrityClientDisconnectedError(c.Request.Context().Err())
			}
			result.PostCommitFailure = true
			return result, nil
		case <-info.OpenAIResponseIntegrityAttemptDone():
			if info.OpenAIResponseIntegrityFirstOutputTimedOut() && !result.MeaningfulOutputSeen {
				result.StopReason = "first_output_timeout"
				markOpenAIIntegrityStopReason(c, result.StopReason)
				return result, newOpenAIIntegrityError(info, protocol, result.StopReason, result, nil, nil)
			}
			if c.Request.Context().Err() != nil {
				result.StopReason = "client_disconnected"
				markOpenAIIntegrityStopReason(c, result.StopReason)
				if !result.MeaningfulOutputSeen {
					return result, newOpenAIIntegrityClientDisconnectedError(c.Request.Context().Err())
				}
				result.PostCommitFailure = true
				return result, nil
			}
		case item, ok := <-items:
			if !ok {
				result.StopReason = "eof"
				markOpenAIIntegrityStopReason(c, result.StopReason)
				if !result.MeaningfulOutputSeen {
					return result, newOpenAIIntegrityError(info, protocol, "eof_before_meaningful_output", result, io.ErrUnexpectedEOF, nil)
				}
				if !result.TerminalSeen {
					result.PostCommitFailure = true
				}
				return result, nil
			}
			if item.err != nil {
				result.StopReason = "scanner_error"
				markOpenAIIntegrityStopReason(c, result.StopReason)
				if !result.MeaningfulOutputSeen {
					return result, newOpenAIIntegrityError(info, protocol, "scanner_error_before_meaningful_output", result, item.err, nil)
				}
				result.PostCommitFailure = true
				return result, nil
			}
			if item.doneMarker {
				result.StopReason = "done_marker"
				markOpenAIIntegrityStopReason(c, result.StopReason)
				if !result.MeaningfulOutputSeen {
					return result, newOpenAIIntegrityError(info, protocol, "done_before_meaningful_output", result, io.ErrUnexpectedEOF, nil)
				}
				if protocol != openAIIntegrityChat && !result.TerminalSeen {
					result.PostCommitFailure = true
				}
				return result, nil
			}

			info.SetFirstResponseTime()
			info.ReceivedResponseCount++
			event, parseErr := classifyOpenAIIntegrityEvent(item.data, protocol)
			if parseErr != nil {
				if !result.MeaningfulOutputSeen {
					result.StopReason = "malformed_json"
					markOpenAIIntegrityStopReason(c, result.StopReason)
					return result, newOpenAIIntegrityError(info, protocol, "malformed_json_before_meaningful_output", result, parseErr, nil)
				}
				result.PostCommitFailure = true
				if !handle(item.data) {
					return result, nil
				}
				continue
			}
			if event.terminal {
				result.TerminalSeen = true
				result.TerminalEvent = event.terminalID
			}

			if !result.MeaningfulOutputSeen {
				buffer = append(buffer, item.data)
				result.BufferedEvents = len(buffer)
				result.BufferedBytes += len(item.data)
				if result.BufferedEvents > openAIIntegrityMaxBufferedEvents || result.BufferedBytes > openAIIntegrityMaxBufferedBytes {
					result.StopReason = "first_output_buffer_limit"
					markOpenAIIntegrityStopReason(c, result.StopReason)
					return result, newOpenAIIntegrityError(info, protocol, result.StopReason, result, errors.New("first output buffer limit exceeded"), nil)
				}
				if event.failure != nil {
					result.StopReason = event.terminalID
					markOpenAIIntegrityStopReason(c, result.StopReason)
					return result, attachOpenAIIntegrityDiagnostic(event.failure, info, protocol, "upstream_error_before_meaningful_output", result)
				}
				if event.meaningful {
					result.MeaningfulOutputSeen = true
					info.MarkOpenAIResponseIntegrityFirstOutput()
					if !flushBuffer() {
						result.StopReason = "handler_stop"
						markOpenAIIntegrityStopReason(c, result.StopReason)
						return result, nil
					}
					if event.terminal {
						result.StopReason = event.terminalID
						markOpenAIIntegrityStopReason(c, result.StopReason)
						return result, nil
					}
					continue
				}
				if event.terminal {
					result.StopReason = event.terminalID
					markOpenAIIntegrityStopReason(c, result.StopReason)
					if event.allowEmpty {
						info.MarkOpenAIResponseIntegrityFirstOutput()
						if !flushBuffer() {
							result.StopReason = "handler_stop"
							result.PostCommitFailure = result.ResponseCommitted
							markOpenAIIntegrityStopReason(c, result.StopReason)
						}
						return result, nil
					}
					return result, newOpenAIIntegrityError(info, protocol, "terminal_before_meaningful_output", result, errors.New(event.terminalID), nil)
				}
				continue
			}

			if event.failure != nil {
				result.PostCommitFailure = true
			}
			if event.terminal && !event.allowEmpty && event.terminalID != "response.completed" {
				result.PostCommitFailure = true
			}
			handled := handle(item.data)
			if c.Writer.Written() {
				result.ResponseCommitted = true
			}
			if !handled {
				result.StopReason = "handler_stop"
				if event.terminal {
					result.StopReason = event.terminalID
				}
				markOpenAIIntegrityStopReason(c, result.StopReason)
				return result, nil
			}
			if event.terminal {
				result.StopReason = event.terminalID
				markOpenAIIntegrityStopReason(c, result.StopReason)
				return result, nil
			}
		}
	}
}

func classifyOpenAIIntegrityEvent(data string, protocol openAIIntegrityProtocol) (openAIIntegrityEvent, error) {
	switch protocol {
	case openAIIntegrityChat:
		return classifyOpenAIChatIntegrityEvent(data)
	case openAIIntegrityResponses, openAIIntegrityResponsesToChat:
		return classifyOpenAIResponsesIntegrityEvent(data, protocol == openAIIntegrityResponsesToChat)
	default:
		return openAIIntegrityEvent{}, fmt.Errorf("unsupported OpenAI integrity protocol %q", protocol)
	}
}

func classifyOpenAIChatIntegrityEvent(data string) (openAIIntegrityEvent, error) {
	var response dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &response); err != nil {
		return openAIIntegrityEvent{}, err
	}
	event := openAIIntegrityEvent{}
	for _, choice := range response.Choices {
		if openAIChatDeltaMeaningful(choice.Delta, data) {
			event.meaningful = true
		}
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			reason := strings.TrimSpace(*choice.FinishReason)
			event.terminal = true
			event.terminalID = reason
			if reason == constant.FinishReasonLength || reason == constant.FinishReasonContentFilter {
				event.allowEmpty = true
			}
		}
	}
	return event, nil
}

func openAIChatDeltaMeaningful(delta dto.ChatCompletionsStreamResponseChoiceDelta, data string) bool {
	if strings.TrimSpace(delta.GetContentString()) != "" ||
		strings.TrimSpace(delta.GetReasoningContent()) != "" ||
		(delta.Thinking != nil && strings.TrimSpace(*delta.Thinking) != "") ||
		len(delta.ToolCalls) > 0 {
		return true
	}
	var payload map[string]any
	if common.UnmarshalJsonStr(data, &payload) != nil {
		return false
	}
	choices, _ := payload["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		rawDelta, _ := choice["delta"].(map[string]any)
		for key, value := range rawDelta {
			if key == "role" {
				continue
			}
			if openAIIntegrityValueMeaningful(value) {
				return true
			}
		}
	}
	return false
}

func classifyOpenAIResponsesIntegrityEvent(data string, toChat bool) (openAIIntegrityEvent, error) {
	var response dto.ResponsesStreamResponse
	if err := common.UnmarshalJsonStr(data, &response); err != nil {
		return openAIIntegrityEvent{}, err
	}
	event := openAIIntegrityEvent{terminal: isResponsesTerminalStreamType(response.Type), terminalID: response.Type}
	if response.Type == "response.failed" || response.Type == "response.error" {
		event.failure = openAIIntegrityResponsesFailure(response.Response)
		if event.failure == nil {
			event.failure = openAIIntegrityTopLevelStreamFailure(data)
		}
	}
	if response.Type == "response.incomplete" {
		event.allowEmpty = openAIResponsesLimitOrSafetyReason(response.Response, data)
	}
	if openAIResponsesStreamMeaningful(&response, data, toChat) {
		event.meaningful = true
	}
	return event, nil
}

func openAIIntegrityTopLevelStreamFailure(data string) *types.NewAPIError {
	var payload map[string]any
	if common.UnmarshalJsonStr(data, &payload) != nil {
		return nil
	}
	openAIError := dto.GetOpenAIError(payload["error"])
	if !openAIIntegrityErrorPresent(openAIError) {
		return nil
	}
	return types.WithOpenAIError(*openAIError, openAIIntegrityProviderErrorStatus(openAIError))
}

func openAIResponsesStreamMeaningful(response *dto.ResponsesStreamResponse, data string, toChat bool) bool {
	if response == nil {
		return false
	}
	switch response.Type {
	case "response.output_text.delta", "response.reasoning_summary_text.delta":
		return strings.TrimSpace(response.Delta) != ""
	case "response.refusal.delta":
		return !toChat && strings.TrimSpace(response.Delta) != ""
	case "response.reasoning_text.delta":
		return !toChat && strings.TrimSpace(response.Delta) != ""
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		return response.ItemID != "" || strings.TrimSpace(response.Delta) != ""
	}
	if response.Item != nil && openAIResponsesOutputMeaningful(response.Item, toChat) {
		return true
	}
	if response.Response != nil && openAIResponsesResponseMeaningful(response.Response, toChat) {
		return true
	}
	if toChat {
		return false
	}
	var payload map[string]any
	if common.UnmarshalJsonStr(data, &payload) != nil {
		return false
	}
	if part, ok := payload["part"].(map[string]any); ok && openAIIntegrityMapHasPayload(part, "type") {
		return true
	}
	if item, ok := payload["item"].(map[string]any); ok && openAIResponsesGenericOutputMeaningful(item, false) {
		return true
	}
	return false
}

func openAIResponsesResponseMeaningful(response *dto.OpenAIResponsesResponse, toChat bool) bool {
	if response == nil {
		return false
	}
	for i := range response.Output {
		if openAIResponsesOutputMeaningful(&response.Output[i], toChat) {
			return true
		}
	}
	return false
}

func openAIResponsesOutputMeaningful(output *dto.ResponsesOutput, toChat bool) bool {
	if output == nil {
		return false
	}
	outputType := strings.TrimSpace(output.Type)
	switch outputType {
	case "message":
		for _, content := range output.Content {
			contentType := strings.TrimSpace(content.Type)
			if strings.TrimSpace(content.Text) == "" {
				continue
			}
			if !toChat || contentType == "output_text" || contentType == "text" || strings.Contains(contentType, "reasoning") {
				return true
			}
		}
		return false
	case "reasoning":
		if toChat {
			return false
		}
		for _, content := range output.Content {
			if strings.TrimSpace(content.Text) != "" {
				return true
			}
		}
		return false
	case "function_call":
		return true
	case "custom_tool_call":
		return !toChat
	default:
		return !toChat && outputType != ""
	}
}

func openAIResponsesGenericOutputMeaningful(output map[string]any, toChat bool) bool {
	outputType := strings.TrimSpace(common.Interface2String(output["type"]))
	if toChat && outputType != "function_call" && outputType != "message" {
		return false
	}
	if outputType == "message" {
		contents, _ := output["content"].([]any)
		for _, value := range contents {
			content, _ := value.(map[string]any)
			contentType := strings.TrimSpace(common.Interface2String(content["type"]))
			if toChat && contentType != "output_text" && contentType != "text" && !strings.Contains(contentType, "reasoning") {
				continue
			}
			if openAIIntegrityMapHasPayload(content, "type", "annotations", "logprobs") {
				return true
			}
		}
		return false
	}
	if outputType == "reasoning" {
		return !toChat && openAIIntegrityMapHasPayload(output, "type", "id", "status")
	}
	if outputType == "function_call" {
		return true
	}
	if outputType == "custom_tool_call" {
		return !toChat
	}
	return !toChat && outputType != "" && openAIIntegrityMapHasPayload(output, "type", "id", "status")
}

func openAIIntegrityMapHasPayload(value map[string]any, ignored ...string) bool {
	ignore := make(map[string]struct{}, len(ignored))
	for _, key := range ignored {
		ignore[key] = struct{}{}
	}
	for key, item := range value {
		if _, ok := ignore[key]; ok {
			continue
		}
		if openAIIntegrityValueMeaningful(item) {
			return true
		}
	}
	return false
}

func openAIIntegrityValueMeaningful(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case bool:
		return typed
	default:
		return true
	}
}

func openAIResponsesLimitOrSafetyReason(response *dto.OpenAIResponsesResponse, raw string) bool {
	reason := ""
	if response != nil && response.IncompleteDetails != nil {
		reason = strings.TrimSpace(response.IncompleteDetails.GetReason())
	}
	if reason == "" {
		var payload map[string]any
		if common.UnmarshalJsonStr(raw, &payload) == nil {
			responseMap := payload
			if nested, ok := payload["response"].(map[string]any); ok {
				responseMap = nested
			}
			if details, ok := responseMap["incomplete_details"].(map[string]any); ok {
				reason = strings.TrimSpace(common.Interface2String(details["reason"]))
				if reason == "" {
					reason = strings.TrimSpace(common.Interface2String(details["reasoning"]))
				}
			}
		}
	}
	return reason == "max_output_tokens" || reason == "content_filter"
}

func validateOpenAIChatResponseIntegrity(
	info *relaycommon.RelayInfo,
	response *dto.OpenAITextResponse,
	rawBody []byte,
) *types.NewAPIError {
	if info == nil || !info.OpenAIResponseIntegrityEnabled {
		return nil
	}
	if openAIChatResponseMeaningful(response, rawBody) {
		info.MarkOpenAIResponseIntegrityFirstOutput()
		return nil
	}
	result := openAIIntegrityStreamResult{StopReason: "empty_chat_response"}
	if response != nil {
		for _, choice := range response.Choices {
			reason := strings.TrimSpace(choice.FinishReason)
			if reason != "" {
				result.TerminalSeen = true
				result.TerminalEvent = reason
			}
			if reason == constant.FinishReasonLength || reason == constant.FinishReasonContentFilter {
				info.MarkOpenAIResponseIntegrityFirstOutput()
				return nil
			}
		}
	}
	return newOpenAIIntegrityError(info, openAIIntegrityChat, result.StopReason, result, errors.New("chat completion contains no usable output"), rawBody)
}

func openAIChatResponseMeaningful(response *dto.OpenAITextResponse, rawBody []byte) bool {
	if response != nil {
		for _, choice := range response.Choices {
			if strings.TrimSpace(choice.Message.StringContent()) != "" ||
				strings.TrimSpace(choice.Message.ReasoningContent) != "" ||
				strings.TrimSpace(choice.Message.Reasoning) != "" ||
				strings.TrimSpace(choice.Message.Thinking) != "" ||
				hasOpenAIResponseToolCalls(choice.Message.ToolCalls) {
				return true
			}
		}
	}
	var payload map[string]any
	if common.Unmarshal(rawBody, &payload) != nil {
		return false
	}
	choices, _ := payload["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if openAIIntegrityMapHasPayload(message, "role") {
			return true
		}
	}
	return false
}

func validateOpenAIResponsesResponseIntegrity(
	info *relaycommon.RelayInfo,
	response *dto.OpenAIResponsesResponse,
	rawBody []byte,
	toChat bool,
) *types.NewAPIError {
	if info == nil || !info.OpenAIResponseIntegrityEnabled {
		return nil
	}
	protocol := openAIIntegrityResponses
	if toChat {
		protocol = openAIIntegrityResponsesToChat
	}
	if openAIResponsesResponseMeaningful(response, toChat) || openAIResponsesRawResponseMeaningful(rawBody, toChat) {
		info.MarkOpenAIResponseIntegrityFirstOutput()
		return nil
	}
	status := ""
	if response != nil {
		status = strings.Trim(string(response.Status), "\" ")
	}
	result := openAIIntegrityStreamResult{
		StopReason:    "empty_responses_response",
		TerminalSeen:  status != "" && status != "in_progress" && status != "queued",
		TerminalEvent: status,
	}
	if status == "incomplete" && openAIResponsesLimitOrSafetyReason(response, string(rawBody)) {
		info.MarkOpenAIResponseIntegrityFirstOutput()
		return nil
	}
	return newOpenAIIntegrityError(info, protocol, result.StopReason, result, errors.New("responses payload contains no usable output"), rawBody)
}

func openAIResponsesRawResponseMeaningful(rawBody []byte, toChat bool) bool {
	var payload map[string]any
	if common.Unmarshal(rawBody, &payload) != nil {
		return false
	}
	outputs, _ := payload["output"].([]any)
	for _, value := range outputs {
		output, _ := value.(map[string]any)
		if openAIResponsesGenericOutputMeaningful(output, toChat) {
			return true
		}
	}
	return false
}

func openAIIntegrityResponseError(
	info *relaycommon.RelayInfo,
	protocol openAIIntegrityProtocol,
	openAIError *types.OpenAIError,
	upstreamStatus int,
	upstreamBody []byte,
) *types.NewAPIError {
	if openAIError == nil {
		return nil
	}
	status := upstreamStatus
	if info != nil && info.OpenAIResponseIntegrityEnabled && (status >= 200 && status < 300) {
		status = openAIIntegrityProviderErrorStatus(openAIError)
	}
	apiErr := types.WithOpenAIError(*openAIError, status)
	if info != nil && info.OpenAIResponseIntegrityEnabled {
		result := openAIIntegrityStreamResult{
			StopReason:    "upstream_error_response",
			TerminalSeen:  true,
			TerminalEvent: "response.failed",
		}
		apiErr = attachOpenAIIntegrityDiagnostic(apiErr, info, protocol, "upstream_error_before_meaningful_output", result)
	}
	if apiErr.Diagnostic == nil {
		apiErr.Diagnostic = &types.RelayErrorDiagnostic{}
	}
	apiErr.Diagnostic.UpstreamResponseBody = append([]byte(nil), upstreamBody...)
	apiErr.Diagnostic.UpstreamBodySize = int64(len(upstreamBody))
	return apiErr
}

func openAIIntegrityReadError(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	protocol openAIIntegrityProtocol,
	err error,
	upstreamBody []byte,
) *types.NewAPIError {
	if info != nil && info.OpenAIResponseIntegrityEnabled {
		if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
			return newOpenAIIntegrityClientDisconnectedError(c.Request.Context().Err())
		}
		if info.OpenAIResponseIntegrityFirstOutputTimedOut() {
			result := openAIIntegrityStreamResult{StopReason: "first_output_timeout"}
			return newOpenAIIntegrityError(info, protocol, result.StopReason, result, err, upstreamBody)
		}
		result := openAIIntegrityStreamResult{StopReason: "read_error_before_meaningful_output"}
		return newOpenAIIntegrityError(info, protocol, result.StopReason, result, err, upstreamBody)
	}
	return attachOpenAIResponseErrorDiagnostic(types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), upstreamBody)
}

func openAIIntegrityParseError(
	info *relaycommon.RelayInfo,
	protocol openAIIntegrityProtocol,
	err error,
	upstreamBody []byte,
) *types.NewAPIError {
	if info != nil && info.OpenAIResponseIntegrityEnabled {
		result := openAIIntegrityStreamResult{StopReason: "malformed_json_before_meaningful_output"}
		return newOpenAIIntegrityError(info, protocol, result.StopReason, result, err, upstreamBody)
	}
	return attachOpenAIResponseErrorDiagnostic(types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError), upstreamBody)
}

func openAIIntegrityResponsesFailure(response *dto.OpenAIResponsesResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	openAIError := response.GetOpenAIError()
	if !openAIIntegrityErrorPresent(openAIError) {
		return nil
	}
	status := openAIIntegrityProviderErrorStatus(openAIError)
	return types.WithOpenAIError(*openAIError, status)
}

func openAIIntegrityErrorPresent(openAIError *types.OpenAIError) bool {
	return openAIError != nil && (openAIError.Type != "" || openAIError.Message != "" || openAIError.Code != nil)
}

func openAIIntegrityProviderErrorStatus(openAIError *types.OpenAIError) int {
	if openAIError == nil {
		return http.StatusBadGateway
	}
	value := strings.ToLower(strings.Join([]string{
		openAIError.Type,
		common.Interface2String(openAIError.Code),
		openAIError.Message,
	}, " "))
	switch {
	case strings.Contains(value, "rate_limit"), strings.Contains(value, "insufficient_quota"):
		return http.StatusTooManyRequests
	case strings.Contains(value, "authentication"), strings.Contains(value, "invalid_api_key"):
		return http.StatusUnauthorized
	case strings.Contains(value, "permission"), strings.Contains(value, "access_denied"):
		return http.StatusForbidden
	case strings.Contains(value, "invalid_request"), strings.Contains(value, "context_length"), strings.Contains(value, "content_filter"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}

func newOpenAIIntegrityError(
	info *relaycommon.RelayInfo,
	protocol openAIIntegrityProtocol,
	reason string,
	result openAIIntegrityStreamResult,
	cause error,
	upstreamBody []byte,
) *types.NewAPIError {
	if cause == nil {
		cause = errors.New(reason)
	}
	apiErr := types.NewErrorWithStatusCode(
		errors.New("upstream returned no usable output"),
		types.ErrorCodeEmptyResponse,
		http.StatusBadGateway,
		types.ErrOptionWithClientSafe(),
	)
	apiErr.Diagnostic = &types.RelayErrorDiagnostic{
		UpstreamResponseBody: append([]byte(nil), upstreamBody...),
		UpstreamBodySize:     int64(len(upstreamBody)),
		StreamSummary: map[string]any{
			"protocol":                string(protocol),
			"integrity_reason":        strings.TrimSpace(reason),
			"stream_stop_reason":      result.StopReason,
			"terminal_event":          result.TerminalEvent,
			"terminal_seen":           result.TerminalSeen,
			"meaningful_output_seen":  result.MeaningfulOutputSeen,
			"response_committed":      result.ResponseCommitted,
			"received_events":         integrityReceivedEvents(info),
			"sent_events":             integritySentEvents(info),
			"buffered_events":         result.BufferedEvents,
			"buffered_bytes":          result.BufferedBytes,
			"first_output_elapsed_ms": integrityElapsedMilliseconds(info),
			"cause":                   cause.Error(),
		},
	}
	return apiErr
}

func attachOpenAIIntegrityDiagnostic(apiErr *types.NewAPIError, info *relaycommon.RelayInfo, protocol openAIIntegrityProtocol, reason string, result openAIIntegrityStreamResult) *types.NewAPIError {
	if apiErr == nil {
		return nil
	}
	if apiErr.Diagnostic == nil {
		apiErr.Diagnostic = &types.RelayErrorDiagnostic{}
	}
	apiErr.Diagnostic.StreamSummary = map[string]any{
		"protocol":                string(protocol),
		"integrity_reason":        strings.TrimSpace(reason),
		"stream_stop_reason":      result.StopReason,
		"terminal_event":          result.TerminalEvent,
		"terminal_seen":           result.TerminalSeen,
		"meaningful_output_seen":  result.MeaningfulOutputSeen,
		"response_committed":      result.ResponseCommitted,
		"received_events":         integrityReceivedEvents(info),
		"sent_events":             integritySentEvents(info),
		"buffered_events":         result.BufferedEvents,
		"buffered_bytes":          result.BufferedBytes,
		"first_output_elapsed_ms": integrityElapsedMilliseconds(info),
	}
	return apiErr
}

func newOpenAIIntegrityClientDisconnectedError(err error) *types.NewAPIError {
	if err == nil {
		err = errors.New("client disconnected")
	}
	return types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, 499, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

func markOpenAIIntegrityStopReason(c *gin.Context, reason string) {
	if c == nil {
		return
	}
	c.Set("stream_scanner_stop_reason", strings.TrimSpace(reason))
}

func integrityReceivedEvents(info *relaycommon.RelayInfo) int {
	if info == nil {
		return 0
	}
	return info.ReceivedResponseCount
}

func integritySentEvents(info *relaycommon.RelayInfo) int {
	if info == nil {
		return 0
	}
	return info.SendResponseCount
}

func integrityElapsedMilliseconds(info *relaycommon.RelayInfo) int64 {
	if info == nil {
		return 0
	}
	return info.OpenAIResponseIntegrityAttemptElapsed().Milliseconds()
}

func logOpenAIIntegrityPostCommitFailure(c *gin.Context, result openAIIntegrityStreamResult) {
	if !result.PostCommitFailure {
		return
	}
	logger.LogError(c, fmt.Sprintf("openai response integrity failure after meaningful output: stop_reason=%s terminal=%s", result.StopReason, result.TerminalEvent))
}
