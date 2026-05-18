package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGlobalAPIRateLimitSkipsRequestDumpEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	originalGlobalEnabled := common.GlobalApiRateLimitEnable
	originalGlobalNum := common.GlobalApiRateLimitNum
	originalGlobalDuration := common.GlobalApiRateLimitDuration
	originalLimiter := inMemoryRateLimiter
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.GlobalApiRateLimitEnable = originalGlobalEnabled
		common.GlobalApiRateLimitNum = originalGlobalNum
		common.GlobalApiRateLimitDuration = originalGlobalDuration
		inMemoryRateLimiter = originalLimiter
	})

	common.RedisEnabled = false
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 3600
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	api := router.Group("/api")
	api.Use(GlobalAPIRateLimit())
	api.GET("/request_dump/events", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.GET("/request_dump/status", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.POST("/request_dump/start", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.POST("/request_dump/stop", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.POST("/request_dump/clear", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.GET("/other", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	api.POST("/request_dumps/start", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	requests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/request_dump/events?after_id=0&limit=100"},
		{method: http.MethodGet, path: "/api/request_dump/status"},
		{method: http.MethodPost, path: "/api/request_dump/start"},
		{method: http.MethodPost, path: "/api/request_dump/stop"},
		{method: http.MethodPost, path: "/api/request_dump/clear"},
	}
	for _, request := range requests {
		for i := 0; i < 3; i++ {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(request.method, request.path, nil)
			req.RemoteAddr = "203.0.113.10:1234"
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code)
		}
	}

	firstOther := httptest.NewRecorder()
	firstOtherReq := httptest.NewRequest(http.MethodGet, "/api/other", nil)
	firstOtherReq.RemoteAddr = "203.0.113.20:1234"
	router.ServeHTTP(firstOther, firstOtherReq)
	require.Equal(t, http.StatusOK, firstOther.Code)

	secondOther := httptest.NewRecorder()
	secondOtherReq := httptest.NewRequest(http.MethodGet, "/api/other", nil)
	secondOtherReq.RemoteAddr = "203.0.113.20:1234"
	router.ServeHTTP(secondOther, secondOtherReq)
	require.Equal(t, http.StatusTooManyRequests, secondOther.Code)

	firstNonDumpPost := httptest.NewRecorder()
	firstNonDumpPostReq := httptest.NewRequest(http.MethodPost, "/api/request_dumps/start", nil)
	firstNonDumpPostReq.RemoteAddr = "203.0.113.30:1234"
	router.ServeHTTP(firstNonDumpPost, firstNonDumpPostReq)
	require.Equal(t, http.StatusOK, firstNonDumpPost.Code)

	secondNonDumpPost := httptest.NewRecorder()
	secondNonDumpPostReq := httptest.NewRequest(http.MethodPost, "/api/request_dumps/start", nil)
	secondNonDumpPostReq.RemoteAddr = "203.0.113.30:1234"
	router.ServeHTTP(secondNonDumpPost, secondNonDumpPostReq)
	require.Equal(t, http.StatusTooManyRequests, secondNonDumpPost.Code)
}
