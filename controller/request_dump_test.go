package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requestDumpControllerCtx(method string, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, recorder
}

func TestRequestDumpControllerLifecycle(t *testing.T) {
	t.Cleanup(func() {
		service.StopRequestDump()
		service.ClearRequestDumpEvents()
	})

	startCtx, startRecorder := requestDumpControllerCtx(http.MethodPost, "/api/request_dump/start", `{
		"user_ids":[123],
		"duration_seconds":60,
		"max_count":5,
		"print_on":"all",
		"print_body":true
	}`)
	StartRequestDump(startCtx)
	var startResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(startRecorder.Body.Bytes(), &startResp))
	require.True(t, startResp.Success, startResp.Message)
	require.Contains(t, string(startResp.Data), `"enabled":true`)

	statusCtx, statusRecorder := requestDumpControllerCtx(http.MethodGet, "/api/request_dump/status", "")
	GetRequestDumpStatus(statusCtx)
	var statusResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &statusResp))
	require.True(t, statusResp.Success, statusResp.Message)
	require.Contains(t, string(statusResp.Data), `"matched_count":0`)

	eventsCtx, eventsRecorder := requestDumpControllerCtx(http.MethodGet, "/api/request_dump/events?after_id=0&limit=20", "")
	GetRequestDumpEvents(eventsCtx)
	var eventsResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(eventsRecorder.Body.Bytes(), &eventsResp))
	require.True(t, eventsResp.Success, eventsResp.Message)
	require.Contains(t, string(eventsResp.Data), `"events"`)

	stopCtx, stopRecorder := requestDumpControllerCtx(http.MethodPost, "/api/request_dump/stop", "")
	StopRequestDump(stopCtx)
	var stopResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(stopRecorder.Body.Bytes(), &stopResp))
	require.True(t, stopResp.Success, stopResp.Message)
	require.Contains(t, string(stopResp.Data), `"enabled":false`)
}

func TestRequestDumpControllerRejectsMissingUserIDs(t *testing.T) {
	t.Cleanup(func() {
		service.StopRequestDump()
		service.ClearRequestDumpEvents()
	})
	ctx, recorder := requestDumpControllerCtx(http.MethodPost, "/api/request_dump/start", `{"print_on":"all"}`)
	StartRequestDump(ctx)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "user_ids")
}
