package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func AssetKeyAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		keyValue := c.GetHeader("Authorization")
		if strings.HasPrefix(keyValue, "Bearer ") || strings.HasPrefix(keyValue, "bearer ") {
			keyValue = strings.TrimSpace(keyValue[7:])
		}
		if !strings.HasPrefix(keyValue, model.AssetKeyPrefix) {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "未提供有效的资源 API Key", types.ErrorCodeAccessDenied)
			return
		}

		assetKey, err := model.GetAssetKeyByKey(keyValue)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "无效的资源 API Key", types.ErrorCodeAccessDenied)
			return
		}
		if assetKey.Status != model.AssetKeyStatusEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "资源 API Key 已禁用", types.ErrorCodeAccessDenied)
			return
		}
		if assetKey.IsExpired(time.Now().Unix()) {
			abortWithOpenAiMessage(c, http.StatusForbidden, "资源 API Key 已过期", types.ErrorCodeAccessDenied)
			return
		}

		allowIPs := assetKey.GetIPLimits()
		if len(allowIPs) > 0 {
			clientIP := c.ClientIP()
			ip := net.ParseIP(clientIP)
			if ip == nil || !common.IsIpInCIDRList(ip, allowIPs) {
				abortWithOpenAiMessage(c, http.StatusForbidden, "您的 IP 不在资源 API Key 允许访问的列表中", types.ErrorCodeAccessDenied)
				return
			}
		}

		userCache, err := model.GetUserCache(assetKey.UserID)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		if userCache.Status != common.UserStatusEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "用户已被封禁", types.ErrorCodeAccessDenied)
			return
		}

		userCache.WriteContext(c)
		c.Set("id", assetKey.UserID)
		c.Set("asset_key_id", assetKey.ID)
		c.Set("asset_key_name", assetKey.Name)
		c.Set("asset_key_scopes", assetKey.Scopes)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, userCache.Group)
		model.TouchAssetKeyLastUsed(assetKey.ID)
		c.Next()
	}
}

// AssetOrTokenAuth keeps existing read-only resource keys working while making
// the normal API token the default credential for Resource Center APIs.
func AssetOrTokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		keyValue := c.GetHeader("Authorization")
		if strings.HasPrefix(keyValue, "Bearer ") || strings.HasPrefix(keyValue, "bearer ") {
			keyValue = strings.TrimSpace(keyValue[7:])
		}
		if strings.HasPrefix(keyValue, model.AssetKeyPrefix) {
			AssetKeyAuth()(c)
			return
		}
		TokenAuth()(c)
	}
}
