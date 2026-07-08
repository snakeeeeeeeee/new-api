package openai

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const responsesStreamStopReasonKey = "responses_stream_stop_reason"
const streamScannerStopReasonKey = "stream_scanner_stop_reason"

func dumpResponsesStreamEvent(c *gin.Context, sequence int, data string, streamResp dto.ResponsesStreamResponse) {
	meta := buildResponsesStreamDumpMeta(data, streamResp)
	meta.Sequence = sequence
	service.DumpResponsesStreamEventIfNeeded(c, meta)
}

func dumpResponsesStreamParseError(c *gin.Context, sequence int, err error) {
	if err == nil {
		return
	}
	service.DumpResponsesStreamEventIfNeeded(c, service.ResponsesStreamDumpMeta{
		EventType:    "parse_error",
		Sequence:     sequence,
		ErrorType:    "bad_response_body",
		ErrorCode:    "bad_response_body",
		ErrorMessage: err.Error(),
	})
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
	})
}

func buildResponsesStreamDumpMeta(data string, streamResp dto.ResponsesStreamResponse) service.ResponsesStreamDumpMeta {
	meta := service.ResponsesStreamDumpMeta{
		EventType: streamResp.Type,
	}
	if streamResp.Item != nil {
		meta.ItemType = streamResp.Item.Type
	}
	if streamResp.Response != nil {
		if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil {
			meta.ErrorType = oaiErr.Type
			meta.ErrorCode = common.Interface2String(oaiErr.Code)
			meta.ErrorMessage = oaiErr.Message
		}
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

func markResponsesStreamStopReason(c *gin.Context, reason string) {
	if c == nil || reason == "" {
		return
	}
	c.Set(responsesStreamStopReasonKey, reason)
}
