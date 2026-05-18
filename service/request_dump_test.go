package service

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRequestDumpTestContext(t *testing.T, body string, contentType string) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?debug=1", bytes.NewBufferString(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	c.Request.Header.Set("Authorization", "Bearer secret")
	c.Request.Header.Set("X-Trace-Id", "trace-1")
	c.Request.Header.Set("X-Api-Key", "hidden")
	c.Set(common.RequestIdKey, "req-dump-test")
	c.Set("id", 123)
	c.Set("token_id", 456)
	c.Set("token_name", "dump-token")
	c.Set("original_model", "gpt-test")
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, "ag-test")
	common.SetContextKey(c, constant.ContextKeyChannelId, 7)
	common.SetContextKey(c, constant.ContextKeyChannelName, "channel-test")
	t.Cleanup(func() {
		common.CleanupBodyStorage(c)
		resetRequestDumpForTest()
	})
	return c
}

func resetRequestDumpForTest() {
	requestDump.mu.Lock()
	requestDump.state = requestDumpState{}
	requestDump.mu.Unlock()
}

func startRequestDumpForTest(t *testing.T, rule RequestDumpRule) {
	t.Helper()
	_, err := StartRequestDump(rule)
	require.NoError(t, err)
}

func TestRequestDumpRequiresUserIDs(t *testing.T) {
	_, err := StartRequestDump(DefaultRequestDumpRule())
	require.Error(t, err)
}

func TestRequestDumpFiltersAndCapturesRawWithoutBreakingReusableBody(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		TokenIDs:        []int{456},
		Models:          []string{"gpt-test"},
		Paths:           []string{"/v1/chat"},
		AggregateGroups: []string{"ag-test"},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintURL:        true,
		PrintHeaders:    true,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})

	DumpRawRequestIfNeeded(c)
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageRawRequest, events[0].Stage)
	require.Contains(t, events[0].RawBody, `"content":"hello"`)
	require.Equal(t, "/v1/chat/completions", events[0].Path)
	require.Equal(t, "/v1/chat/completions?debug=1", events[0].RawURL)
	require.Equal(t, "trace-1", events[0].Headers["X-Trace-Id"])
	require.NotContains(t, events[0].Headers, "Authorization")
	require.NotContains(t, events[0].Headers, "X-Api-Key")

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	body, err := storage.Bytes()
	require.NoError(t, err)
	require.Contains(t, string(body), `"messages"`)

	DumpRawRequestIfNeeded(c)
	require.Len(t, GetRequestDumpEvents(0, 10), 1, "raw request is recorded once per request")
}

func TestRequestDumpFilterMissDoesNotCapture(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		TokenIDs:        []int{999},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
	})

	DumpRawRequestIfNeeded(c)
	require.Empty(t, GetRequestDumpEvents(0, 10))
}

func TestRequestDumpKeywordMissDoesNotCaptureOrCount(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","messages":[{"role":"user","content":"ordinary"}]}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"needle"},
		DurationSeconds: 60,
		MaxCount:        1,
		PrintOn:         RequestDumpPrintOnAll,
		PrintHeaders:    true,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})

	DumpRawRequestIfNeeded(c)
	require.Empty(t, GetRequestDumpEvents(0, 10))
	status := GetRequestDumpStatus()
	require.True(t, status.Enabled, "unprinted keyword misses must not consume max_count")
	require.Equal(t, 0, status.MatchedCount)
}

func TestRequestDumpKeywordMatchesBodyHeaderAndError(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","messages":[{"role":"user","content":"Find Needle In Body"}]}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"needle in body"},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintHeaders:    true,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c)
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Contains(t, events[0].RawBody, "Needle In Body")
	require.Equal(t, 1, GetRequestDumpStatus().MatchedCount)

	ClearRequestDumpEvents()
	c2 := newRequestDumpTestContext(t, `{"model":"gpt-test"}`, "application/json")
	c2.Request.Header.Set("X-Dump-Marker", "HeaderNeedle")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"headerneedle"},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintHeaders:    true,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c2)
	events = GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, "HeaderNeedle", events[0].Headers["X-Dump-Marker"])

	ClearRequestDumpEvents()
	c3 := newRequestDumpTestContext(t, `{"model":"gpt-test","bad":true}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"invalid_request"},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnErrorOnly,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c3)
	require.Empty(t, GetRequestDumpEvents(0, 10))
	apiErr := types.NewErrorWithStatusCode(fmt.Errorf("bad request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	DumpRelayErrorIfNeeded(c3, apiErr)
	events = GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageRelayError, events[0].Stage)
	require.Equal(t, "invalid_request", events[0].ErrorCode)
}

func TestRequestDumpKeywordMatchesUpstreamBodyAndCountsOnce(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","messages":[{"role":"user","content":"ordinary"}]}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:           []int{123},
		Keywords:          []string{"converted-needle"},
		DurationSeconds:   60,
		MaxCount:          1,
		PrintOn:           RequestDumpPrintOnAll,
		PrintBody:         true,
		PrintUpstreamBody: true,
		MaxBodyBytes:      1024,
	})

	DumpRawRequestIfNeeded(c)
	DumpUpstreamRequestIfNeeded(c, []byte(`{"converted":"converted-needle"}`))
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageUpstreamRequest, events[0].Stage)
	require.False(t, GetRequestDumpStatus().Enabled)
	require.Equal(t, 1, GetRequestDumpStatus().MatchedCount)
}

func TestRequestDumpKeywordCaseSensitive(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","message":"Needle"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"needle"},
		CaseSensitive:   true,
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c)
	require.Empty(t, GetRequestDumpEvents(0, 10))

	c2 := newRequestDumpTestContext(t, `{"model":"gpt-test","message":"Needle"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		Keywords:        []string{"Needle"},
		CaseSensitive:   true,
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c2)
	require.Len(t, GetRequestDumpEvents(0, 10), 1)
}

func TestRequestDumpNormalizesLogLevel(t *testing.T) {
	status, err := StartRequestDump(RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		LogLevel:        "WARN",
	})
	require.NoError(t, err)
	require.Equal(t, RequestDumpLogLevelWarn, status.Rule.LogLevel)

	_, err = StartRequestDump(RequestDumpRule{
		UserIDs:  []int{123},
		PrintOn:  RequestDumpPrintOnAll,
		LogLevel: "verbose",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "log_level")
}

func TestRequestDumpStopExpiryAndMaxCount(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        1,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
	})

	DumpRawRequestIfNeeded(c)
	status := GetRequestDumpStatus()
	require.False(t, status.Enabled, "max_count disables the rule after first matched request")
	require.Len(t, GetRequestDumpEvents(0, 10), 1)

	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 1,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
	})
	time.Sleep(1100 * time.Millisecond)
	require.False(t, GetRequestDumpStatus().Enabled)

	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
	})
	require.True(t, GetRequestDumpStatus().Enabled)
	StopRequestDump()
	require.False(t, GetRequestDumpStatus().Enabled)
}

func TestRequestDumpUnsupportedAndTooLargeBodiesSkipBody(t *testing.T) {
	c := newRequestDumpTestContext(t, `------boundary`, "multipart/form-data; boundary=----boundary")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})
	DumpRawRequestIfNeeded(c)
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageRawRequest, events[0].Stage)
	require.Equal(t, "unsupported_content_type", events[0].SkipReason)
	require.Empty(t, events[0].RawBody)

	ClearRequestDumpEvents()
	c2 := newRequestDumpTestContext(t, `{"model":"gpt-test","payload":"abcdef"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        10,
		PrintOn:         RequestDumpPrintOnAll,
		PrintBody:       true,
		MaxBodyBytes:    8,
	})
	DumpRawRequestIfNeeded(c2)
	events = GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, "body_too_large", events[0].SkipReason)
	require.Empty(t, events[0].RawBody)
}

func TestRequestDumpErrorOnlyCapturesBodyOnErrorAndDoesNotDoubleCount(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test","bad":true}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:         []int{123},
		DurationSeconds: 60,
		MaxCount:        1,
		PrintOn:         RequestDumpPrintOnErrorOnly,
		PrintBody:       true,
		MaxBodyBytes:    1024,
	})

	DumpRawRequestIfNeeded(c)
	require.Empty(t, GetRequestDumpEvents(0, 10))
	apiErr := types.NewErrorWithStatusCode(fmt.Errorf("bad request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	DumpRelayErrorIfNeeded(c, apiErr)
	DumpRelayErrorIfNeeded(c, apiErr)
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageRelayError, events[0].Stage)
	require.Contains(t, events[0].RawBody, `"bad":true`)
	require.False(t, GetRequestDumpStatus().Enabled)
}

func TestRequestDumpRingBufferAndAfterID(t *testing.T) {
	resetRequestDumpForTest()
	for i := 0; i < defaultRequestDumpBufferSize+5; i++ {
		appendRequestDumpEvent(RequestDumpEvent{
			Stage:     RequestDumpStageRawRequest,
			RequestID: fmt.Sprintf("req-%d", i),
		})
	}
	status := GetRequestDumpStatus()
	require.Equal(t, defaultRequestDumpBufferSize, status.ConsoleEventCount)
	require.Equal(t, int64(6), status.ConsoleOldestEventID)
	require.Equal(t, int64(defaultRequestDumpBufferSize+5), status.ConsoleLatestEventID)
	events := GetRequestDumpEvents(status.ConsoleLatestEventID-2, 10)
	require.Len(t, events, 2)
	require.Equal(t, status.ConsoleLatestEventID-1, events[0].ID)
}

func TestRequestDumpUpstreamBodyRespectsSwitch(t *testing.T) {
	c := newRequestDumpTestContext(t, `{"model":"gpt-test"}`, "application/json")
	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:           []int{123},
		DurationSeconds:   60,
		MaxCount:          10,
		PrintOn:           RequestDumpPrintOnAll,
		PrintUpstreamBody: false,
		MaxBodyBytes:      1024,
	})
	DumpUpstreamRequestIfNeeded(c, []byte(`{"converted":true}`))
	require.Empty(t, GetRequestDumpEvents(0, 10))

	startRequestDumpForTest(t, RequestDumpRule{
		UserIDs:           []int{123},
		DurationSeconds:   60,
		MaxCount:          10,
		PrintOn:           RequestDumpPrintOnAll,
		PrintUpstreamBody: true,
		MaxBodyBytes:      1024,
	})
	DumpUpstreamRequestIfNeeded(c, []byte(`{"converted":true}`))
	events := GetRequestDumpEvents(0, 10)
	require.Len(t, events, 1)
	require.Equal(t, RequestDumpStageUpstreamRequest, events[0].Stage)
	require.Equal(t, `{"converted":true}`, events[0].UpstreamBody)
}
