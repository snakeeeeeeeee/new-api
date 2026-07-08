package openai

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const responsesStreamStopReasonKey = "responses_stream_stop_reason"
const streamScannerStopReasonKey = "stream_scanner_stop_reason"
const responsesStreamTraceStateKey = "responses_stream_trace_state"

type responsesStreamTraceState struct {
	EventCounts         map[string]int
	ItemCounts          map[string]int
	ToolCalls           []string
	FunctionCallCount   int
	CustomToolCallCount int
	ArgumentBytes       int
	LastEventType       string
	LastItemType        string
	LastToolName        string
	LastCallID          string
}

func dumpResponsesStreamEvent(c *gin.Context, sequence int, data string, streamResp dto.ResponsesStreamResponse) {
	meta := buildResponsesStreamDumpMeta(data, streamResp)
	meta.Sequence = sequence
	recordResponsesStreamTraceState(c, meta)
	service.DumpResponsesStreamEventIfNeeded(c, meta)
}

func dumpResponsesStreamParseError(c *gin.Context, sequence int, err error) {
	if err == nil {
		return
	}
	meta := service.ResponsesStreamDumpMeta{
		EventType:    "parse_error",
		Sequence:     sequence,
		ErrorType:    "bad_response_body",
		ErrorCode:    "bad_response_body",
		ErrorMessage: err.Error(),
	}
	recordResponsesStreamTraceState(c, meta)
	service.DumpResponsesStreamEventIfNeeded(c, meta)
}

func dumpResponsesStreamSummary(c *gin.Context, startedAt time.Time, sequence int, receivedCount int) {
	stopReason := c.GetString(responsesStreamStopReasonKey)
	if stopReason == "" {
		stopReason = c.GetString(streamScannerStopReasonKey)
	}
	service.DumpResponsesStreamSummaryIfNeeded(c, service.ResponsesStreamDumpMeta{
		Sequence:      sequence,
		StopReason:    stopReason,
		ElapsedMs:     time.Since(startedAt).Milliseconds(),
		ReceivedCount: receivedCount,
		Details:       buildResponsesStreamTraceSummary(c),
	})
}

func buildResponsesStreamDumpMeta(data string, streamResp dto.ResponsesStreamResponse) service.ResponsesStreamDumpMeta {
	meta := service.ResponsesStreamDumpMeta{
		EventType: streamResp.Type,
	}
	details := map[string]any{}
	if streamResp.Item != nil {
		meta.ItemType = streamResp.Item.Type
		meta.ItemID = strings.TrimSpace(streamResp.Item.ID)
		meta.CallID = strings.TrimSpace(streamResp.Item.CallId)
		meta.ToolName = strings.TrimSpace(streamResp.Item.Name)
		if streamResp.Item.Arguments != "" {
			meta.Arguments = streamResp.Item.Arguments
			meta.ArgumentsSize = len(streamResp.Item.Arguments)
			details["arguments_source"] = "item.arguments"
		}
	}
	if streamResp.ItemID != "" {
		meta.ItemID = streamResp.ItemID
	}
	if streamResp.OutputIndex != nil {
		details["output_index"] = *streamResp.OutputIndex
	}
	if streamResp.ContentIndex != nil {
		details["content_index"] = *streamResp.ContentIndex
	}
	if streamResp.SummaryIndex != nil {
		details["summary_index"] = *streamResp.SummaryIndex
	}
	if streamResp.Delta != "" && strings.Contains(streamResp.Type, "function_call_arguments") {
		meta.Arguments = streamResp.Delta
		meta.ArgumentsSize = len(streamResp.Delta)
		details["arguments_source"] = "delta"
	}
	if streamResp.Response != nil {
		if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil {
			meta.ErrorType = oaiErr.Type
			meta.ErrorCode = common.Interface2String(oaiErr.Code)
			meta.ErrorMessage = oaiErr.Message
		}
		addResponsesOutputSummary(details, streamResp.Response.Output)
	}
	enrichResponsesStreamDumpMetaFromRaw(data, &meta, details)
	if len(details) > 0 {
		meta.Details = details
	}
	if meta.EventType != "" {
		return meta
	}

	var errResp dto.GeneralErrorResponse
	if err := common.UnmarshalJsonStr(data, &errResp); err != nil {
		meta.EventType = "unknown"
		return meta
	}
	if oaiErr := errResp.TryToOpenAIError(); oaiErr != nil {
		meta.EventType = "bare_error"
		meta.ErrorType = oaiErr.Type
		meta.ErrorCode = common.Interface2String(oaiErr.Code)
		meta.ErrorMessage = oaiErr.Message
		return meta
	}
	if msg := errResp.ToMessage(); msg != "" {
		meta.EventType = "bare_error"
		meta.ErrorMessage = msg
		return meta
	}
	meta.EventType = "unknown"
	return meta
}

func enrichResponsesStreamDumpMetaFromRaw(data string, meta *service.ResponsesStreamDumpMeta, details map[string]any) {
	if meta == nil {
		return
	}
	var root map[string]any
	if err := common.UnmarshalJsonStr(data, &root); err != nil {
		return
	}
	if meta.ItemID == "" {
		meta.ItemID = responseTraceString(root["item_id"])
	}
	if outputIndex, ok := responseTraceNumber(root["output_index"]); ok {
		details["output_index"] = outputIndex
	}
	if contentIndex, ok := responseTraceNumber(root["content_index"]); ok {
		details["content_index"] = contentIndex
	}
	if itemMap, ok := root["item"].(map[string]any); ok {
		if meta.ItemType == "" {
			meta.ItemType = responseTraceString(itemMap["type"])
		}
		if meta.ItemID == "" {
			meta.ItemID = responseTraceString(itemMap["id"])
		}
		if meta.CallID == "" {
			meta.CallID = responseTraceString(itemMap["call_id"])
		}
		if meta.ToolName == "" {
			meta.ToolName = responseTraceString(itemMap["name"])
		}
		if meta.Arguments == "" {
			if args := responseTracePreview(itemMap["arguments"], 1000); args != "" {
				meta.Arguments = args
				meta.ArgumentsSize = responseTraceValueSize(itemMap["arguments"])
				details["arguments_source"] = "item.arguments"
			} else if input := responseTracePreview(itemMap["input"], 1000); input != "" {
				meta.Arguments = input
				meta.ArgumentsSize = responseTraceValueSize(itemMap["input"])
				details["arguments_source"] = "item.input"
			}
		}
		if status := responseTraceString(itemMap["status"]); status != "" {
			details["item_status"] = status
		}
	}
	if meta.Arguments == "" && strings.Contains(meta.EventType, "function_call_arguments") {
		if delta := responseTraceString(root["delta"]); delta != "" {
			meta.Arguments = delta
			meta.ArgumentsSize = len(delta)
			details["arguments_source"] = "delta"
		}
	}
}

func addResponsesOutputSummary(details map[string]any, outputs []dto.ResponsesOutput) {
	if len(outputs) == 0 {
		return
	}
	typeCounts := map[string]int{}
	toolCalls := make([]string, 0, min(len(outputs), 50))
	for _, output := range outputs {
		if output.Type != "" {
			typeCounts[output.Type]++
		}
		if output.Type == "function_call" || output.Type == "custom_tool_call" {
			toolCalls = append(toolCalls, responseTraceToolCallSummary(output.Name, output.CallId, output.ID, len(output.Arguments)))
		}
	}
	details["response_output_count"] = len(outputs)
	if len(typeCounts) > 0 {
		details["response_output_type_counts"] = typeCounts
	}
	if len(toolCalls) > 0 {
		details["response_tool_calls"] = toolCalls
	}
}

func recordResponsesStreamTraceState(c *gin.Context, meta service.ResponsesStreamDumpMeta) {
	if c == nil {
		return
	}
	state := getResponsesStreamTraceState(c)
	eventType := strings.TrimSpace(meta.EventType)
	if eventType == "" {
		eventType = "unknown"
	}
	itemType := strings.TrimSpace(meta.ItemType)
	state.EventCounts[eventType]++
	if itemType != "" {
		state.ItemCounts[itemType]++
	}
	state.LastEventType = eventType
	state.LastItemType = itemType
	if meta.ArgumentsSize > 0 {
		state.ArgumentBytes += meta.ArgumentsSize
	}
	if meta.ToolName != "" {
		state.LastToolName = meta.ToolName
	}
	if meta.CallID != "" {
		state.LastCallID = meta.CallID
	}
	if itemType == "function_call" || strings.Contains(eventType, "function_call") {
		if meta.ToolName != "" || meta.CallID != "" || meta.ItemID != "" {
			state.FunctionCallCount++
			state.ToolCalls = appendLimitedResponsesStreamToolCall(state.ToolCalls, responseTraceToolCallSummary(meta.ToolName, meta.CallID, meta.ItemID, meta.ArgumentsSize), 80)
		}
	}
	if itemType == "custom_tool_call" {
		state.CustomToolCallCount++
		state.ToolCalls = appendLimitedResponsesStreamToolCall(state.ToolCalls, responseTraceToolCallSummary(meta.ToolName, meta.CallID, meta.ItemID, meta.ArgumentsSize), 80)
	}
	c.Set(responsesStreamTraceStateKey, state)
}

func getResponsesStreamTraceState(c *gin.Context) *responsesStreamTraceState {
	if existing, ok := c.Get(responsesStreamTraceStateKey); ok {
		if state, ok := existing.(*responsesStreamTraceState); ok && state != nil {
			return state
		}
	}
	return &responsesStreamTraceState{
		EventCounts: map[string]int{},
		ItemCounts:  map[string]int{},
		ToolCalls:   []string{},
	}
}

func buildResponsesStreamTraceSummary(c *gin.Context) map[string]any {
	if c == nil {
		return nil
	}
	existing, ok := c.Get(responsesStreamTraceStateKey)
	if !ok {
		return nil
	}
	state, ok := existing.(*responsesStreamTraceState)
	if !ok || state == nil {
		return nil
	}
	details := map[string]any{}
	if len(state.EventCounts) > 0 {
		details["event_counts"] = sortedResponsesStreamCounts(state.EventCounts)
	}
	if len(state.ItemCounts) > 0 {
		details["item_counts"] = sortedResponsesStreamCounts(state.ItemCounts)
	}
	if len(state.ToolCalls) > 0 {
		details["tool_calls"] = state.ToolCalls
	}
	if state.FunctionCallCount > 0 {
		details["function_call_events"] = state.FunctionCallCount
	}
	if state.CustomToolCallCount > 0 {
		details["custom_tool_call_events"] = state.CustomToolCallCount
	}
	if state.ArgumentBytes > 0 {
		details["argument_bytes_seen"] = state.ArgumentBytes
	}
	if state.LastEventType != "" {
		details["last_event_type"] = state.LastEventType
	}
	if state.LastItemType != "" {
		details["last_item_type"] = state.LastItemType
	}
	if state.LastToolName != "" {
		details["last_tool_name"] = state.LastToolName
	}
	if state.LastCallID != "" {
		details["last_call_id"] = state.LastCallID
	}
	return details
}

func sortedResponsesStreamCounts(counts map[string]int) map[string]int {
	if len(counts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]int, len(counts))
	for _, key := range keys {
		result[key] = counts[key]
	}
	return result
}

func appendLimitedResponsesStreamToolCall(values []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	values = append(values, value)
	if limit > 0 && len(values) > limit {
		return values[len(values)-limit:]
	}
	return values
}

func responseTraceToolCallSummary(name string, callID string, itemID string, argBytes int) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(name) != "" {
		parts = append(parts, strings.TrimSpace(name))
	}
	if strings.TrimSpace(callID) != "" {
		parts = append(parts, "call_id="+strings.TrimSpace(callID))
	}
	if strings.TrimSpace(itemID) != "" {
		parts = append(parts, "item_id="+strings.TrimSpace(itemID))
	}
	if argBytes > 0 {
		parts = append(parts, fmt.Sprintf("args_bytes=%d", argBytes))
	}
	return strings.Join(parts, " ")
}

func responseTraceString(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func responseTraceNumber(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func responseTracePreview(value any, limit int) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return limitResponseTraceString(typed, limit)
	default:
		data, err := common.Marshal(typed)
		if err != nil {
			return limitResponseTraceString(fmt.Sprintf("%v", typed), limit)
		}
		return limitResponseTraceString(string(data), limit)
	}
}

func responseTraceValueSize(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case string:
		return len(typed)
	default:
		data, err := common.Marshal(typed)
		if err != nil {
			return len(fmt.Sprintf("%v", typed))
		}
		return len(data)
	}
}

func limitResponseTraceString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func markResponsesStreamStopReason(c *gin.Context, reason string) {
	if c == nil || reason == "" {
		return
	}
	c.Set(responsesStreamStopReasonKey, reason)
}
