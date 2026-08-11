package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCanvasOAuthTokenPostAllowsCanvasOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/canvas/oauth/token", strings.NewReader(`{"grant_type":"authorization_code"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestCanvasModelSyncPostAllowsCanvasOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/canvas/authorization/sync", strings.NewReader(`{"client_id":"infinite-canvas"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
}
