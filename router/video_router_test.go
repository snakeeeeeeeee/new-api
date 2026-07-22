package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestXaiCompatibleVideoRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	assert.True(t, routes[http.MethodPost+" /v1/videos/generations"])
	assert.True(t, routes[http.MethodPost+" /v1/videos/edits"])
	assert.True(t, routes[http.MethodPost+" /v1/videos/extensions"])
	assert.True(t, routes[http.MethodGet+" /v1/videos/:task_id"])
	assert.True(t, routes[http.MethodGet+" /v1/videos/:task_id/content"])
	assert.True(t, routes[http.MethodPost+" /v1/video/tasks"])
	assert.True(t, routes[http.MethodGet+" /v1/video/tasks"])
	assert.True(t, routes[http.MethodGet+" /v1/video/tasks/:task_id"])
	assert.True(t, routes[http.MethodPost+" /v1/video/tasks/query"])
	assert.True(t, routes[http.MethodGet+" /v1/assets/:asset_id/content"])
	assert.True(t, routes[http.MethodPost+" /v1/image/tasks"])
	assert.True(t, routes[http.MethodGet+" /v1/image/tasks"])
	assert.True(t, routes[http.MethodPost+" /v1/image/uploads"])
	assert.False(t, routes[http.MethodGet+" /v1/webhook/endpoints"])
	assert.False(t, routes[http.MethodPost+" /v1/webhook/endpoints"])
	assert.False(t, routes[http.MethodPost+" /v1/webhook/deliveries/:delivery_id/retry"])
}
