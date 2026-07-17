package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func RequireAssetKeyScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !model.AssetKeyHasScope(c.GetString("asset_key_scopes"), scope) {
			abortWithOpenAiMessage(c, http.StatusForbidden, "资源 API Key 缺少所需 scope: "+scope, types.ErrorCodeAccessDenied)
			return
		}
		c.Next()
	}
}
