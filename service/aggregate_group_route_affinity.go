package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
)

const aggregateRouteAffinityCacheNamespace = "new-api:aggregate_route_affinity:v3"

var (
	aggregateRouteAffinityCacheOnce sync.Once
	aggregateRouteAffinityCache     *cachex.HybridCache[string]
)

func getAggregateRouteAffinityCache() *cachex.HybridCache[string] {
	aggregateRouteAffinityCacheOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}

		aggregateRouteAffinityCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace:  cachex.Namespace(aggregateRouteAffinityCacheNamespace),
			Redis:      common.RDB,
			RedisCodec: cachex.StringCodec{},
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return aggregateRouteAffinityCache
}

func buildAggregateRouteAffinityKey(c *gin.Context, aggregateGroup string) string {
	return buildAggregateRouteAffinityKeyForPool(c, "", aggregateGroup, "")
}

func defaultAggregateRouteAffinityKeySources() []model.AggregateGroupRouteAffinityKeySource {
	return []model.AggregateGroupRouteAffinityKeySource{
		{Type: "header", Key: "X-Aggregate-Affinity-Key"},
		{Type: "query", Key: "aggregate_route_affinity_key"},
		{Type: "gjson", Path: "metadata.aggregate_route_affinity_key"},
		{Type: "gjson", Path: "metadata.user_id"},
		{Type: "gjson", Path: "prompt_cache_key"},
		{Type: "gjson", Path: "user"},
		{Type: "gjson", Path: "cachedContent"},
	}
}

func getAggregateRouteAffinityStrategyFromContext(c *gin.Context) string {
	strategy := model.AggregateGroupRouteAffinityStrategyPlatformUser
	if c != nil {
		strategy = common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityStrategy)
	}
	return model.NormalizeAggregateGroupRouteAffinityStrategy(strategy)
}

func getAggregateRouteAffinityScopeFromContext(c *gin.Context) string {
	scope := model.AggregateGroupRouteAffinityScopeShared
	if c != nil {
		scope = common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityScope)
	}
	return model.NormalizeAggregateGroupRouteAffinityScope(scope)
}

func getAggregateRouteAffinitySourcesFromContext(c *gin.Context) []model.AggregateGroupRouteAffinityKeySource {
	if c == nil {
		return defaultAggregateRouteAffinityKeySources()
	}
	if value, ok := common.GetContextKey(c, constant.ContextKeyAggregateRouteAffinitySources); ok {
		if sources, ok := value.([]model.AggregateGroupRouteAffinityKeySource); ok && len(sources) > 0 {
			return sources
		}
	}
	return defaultAggregateRouteAffinityKeySources()
}

func routeAffinitySourceIdentity(source model.AggregateGroupRouteAffinityKeySource) string {
	switch strings.TrimSpace(source.Type) {
	case "gjson":
		return "gjson:" + strings.TrimSpace(source.Path)
	default:
		return strings.TrimSpace(source.Type) + ":" + strings.TrimSpace(source.Key)
	}
}

func setAggregateRouteAffinitySourceContext(c *gin.Context, strategy string, source model.AggregateGroupRouteAffinityKeySource, value string) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityStrategy, strategy)
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourceType, strings.TrimSpace(source.Type))
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourceKey, strings.TrimSpace(source.Key))
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourcePath, strings.TrimSpace(source.Path))
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityKeyHint, buildChannelAffinityKeyHint(value))
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityKeyFP, affinityFingerprint(value))
}

func clearAggregateRouteAffinitySourceContext(c *gin.Context, strategy string) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityStrategy, strategy)
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourceType, "")
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourceKey, "")
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinitySourcePath, "")
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityKeyHint, "")
	common.SetContextKey(c, constant.ContextKeyAggregateRouteAffinityKeyFP, "")
}

func formatAggregateRouteAffinityKeyPrefix(aggregateGroup string, routePool string, modelName string, scope string) string {
	parts := []string{aggregateGroup}
	if routePool != "" && routePool != aggregateClusterDefaultRoutePool {
		parts = append(parts, "pool:"+routePool)
	}
	if model.NormalizeAggregateGroupRouteAffinityScope(scope) == model.AggregateGroupRouteAffinityScopeModel && strings.TrimSpace(modelName) != "" {
		parts = append(parts, "model:"+strings.TrimSpace(modelName))
	}
	return strings.Join(parts, "\n")
}

func buildAggregateRouteAffinityRequestKey(c *gin.Context, aggregateGroup string, routePool string, modelName string, strategy string, scope string) string {
	for _, source := range getAggregateRouteAffinitySourcesFromContext(c) {
		value := extractChannelAffinityValue(c, operation_setting.ChannelAffinityKeySource{
			Type: source.Type,
			Key:  source.Key,
			Path: source.Path,
		})
		if value == "" {
			continue
		}
		setAggregateRouteAffinitySourceContext(c, strategy, source, value)
		sourceID := routeAffinitySourceIdentity(source)
		fp := affinityFingerprint(value)
		return fmt.Sprintf("%s\nrequest:%s\nfp:%s", formatAggregateRouteAffinityKeyPrefix(aggregateGroup, routePool, modelName, scope), sourceID, fp)
	}
	return ""
}

func buildAggregateRouteAffinityPlatformUserKey(c *gin.Context, aggregateGroup string, routePool string, modelName string, strategy string, scope string) string {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if userID <= 0 {
		return ""
	}
	setAggregateRouteAffinitySourceContext(c, strategy, model.AggregateGroupRouteAffinityKeySource{
		Type: "platform_user",
		Key:  string(constant.ContextKeyUserId),
	}, fmt.Sprintf("%d", userID))
	return fmt.Sprintf("%s\nuser:%d", formatAggregateRouteAffinityKeyPrefix(aggregateGroup, routePool, modelName, scope), userID)
}

func buildAggregateRouteAffinityKeyForPool(c *gin.Context, modelName string, aggregateGroup string, routePool string) string {
	if c == nil || strings.TrimSpace(aggregateGroup) == "" {
		return ""
	}
	routePool = strings.TrimSpace(routePool)
	strategy := getAggregateRouteAffinityStrategyFromContext(c)
	scope := getAggregateRouteAffinityScopeFromContext(c)
	switch strategy {
	case model.AggregateGroupRouteAffinityStrategyOff:
		clearAggregateRouteAffinitySourceContext(c, strategy)
		return ""
	case model.AggregateGroupRouteAffinityStrategyRequestOnly:
		key := buildAggregateRouteAffinityRequestKey(c, aggregateGroup, routePool, modelName, strategy, scope)
		if key == "" {
			clearAggregateRouteAffinitySourceContext(c, strategy)
		}
		return key
	case model.AggregateGroupRouteAffinityStrategyRequestFirst:
		if key := buildAggregateRouteAffinityRequestKey(c, aggregateGroup, routePool, modelName, strategy, scope); key != "" {
			return key
		}
		return buildAggregateRouteAffinityPlatformUserKey(c, aggregateGroup, routePool, modelName, strategy, scope)
	default:
		return buildAggregateRouteAffinityPlatformUserKey(c, aggregateGroup, routePool, modelName, strategy, scope)
	}
}

func GetAggregateRouteAffinity(c *gin.Context, modelName string, aggregateGroup string) (string, bool) {
	return GetAggregateRouteAffinityForPool(c, modelName, aggregateGroup, "")
}

func GetAggregateRouteAffinityForPool(c *gin.Context, modelName string, aggregateGroup string, routePool string) (string, bool) {
	key := buildAggregateRouteAffinityKeyForPool(c, modelName, aggregateGroup, routePool)
	if key == "" {
		return "", false
	}
	routeGroup, found, err := getAggregateRouteAffinityCache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("aggregate route affinity cache get failed: aggregate_group=%s, model=%s, err=%v", aggregateGroup, modelName, err))
		return "", false
	}
	routeGroup = strings.TrimSpace(routeGroup)
	return routeGroup, found && routeGroup != ""
}

func resolveAggregateRouteAffinityTTL(c *gin.Context) time.Duration {
	if c == nil {
		return time.Duration(model.AggregateGroupClusterAffinityTTLDefaultSeconds) * time.Second
	}
	interval := model.NormalizeAggregateGroupClusterAffinityTTLSeconds(
		common.GetContextKeyInt(c, constant.ContextKeyAggregateClusterAffinityTTL),
	)
	return time.Duration(interval) * time.Second
}

func RecordAggregateRouteAffinity(c *gin.Context, modelName string, aggregateGroup string, routeGroup string) {
	RecordAggregateRouteAffinityForPool(c, modelName, aggregateGroup, "", routeGroup)
}

func RecordAggregateRouteAffinityForPool(c *gin.Context, modelName string, aggregateGroup string, routePool string, routeGroup string) {
	routeGroup = strings.TrimSpace(routeGroup)
	if routeGroup == "" {
		return
	}
	key := buildAggregateRouteAffinityKeyForPool(c, modelName, aggregateGroup, routePool)
	if key == "" {
		return
	}
	affinityTTL := resolveAggregateRouteAffinityTTL(c)
	if common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityHit) == routeGroup {
		return
	}
	currentRouteGroup, found, err := getAggregateRouteAffinityCache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("aggregate route affinity cache get before set failed: aggregate_group=%s, model=%s, err=%v", aggregateGroup, modelName, err))
	} else if found && strings.TrimSpace(currentRouteGroup) == routeGroup {
		return
	}
	if err := getAggregateRouteAffinityCache().SetWithTTL(key, routeGroup, affinityTTL); err != nil {
		common.SysError(fmt.Sprintf("aggregate route affinity cache set failed: aggregate_group=%s, model=%s, route_group=%s, err=%v", aggregateGroup, modelName, routeGroup, err))
	}
}

func ClearAggregateRouteAffinityCacheAll() int {
	cache := getAggregateRouteAffinityCache()
	keys, err := cache.Keys()
	if err != nil {
		common.SysError(fmt.Sprintf("aggregate route affinity cache list keys failed: err=%v", err))
		keys = nil
	}
	if len(keys) > 0 {
		if _, err := cache.DeleteMany(keys); err != nil {
			common.SysError(fmt.Sprintf("aggregate route affinity cache delete many failed: err=%v", err))
		}
	}
	return len(keys)
}
