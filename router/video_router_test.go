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
}
