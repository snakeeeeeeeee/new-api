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
	if c == nil || strings.TrimSpace(aggregateGroup) == "" {
		return ""
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if userID <= 0 {
		return ""
	}
	return fmt.Sprintf("%s\nuser:%d", aggregateGroup, userID)
}

func GetAggregateRouteAffinity(c *gin.Context, modelName string, aggregateGroup string) (string, bool) {
	key := buildAggregateRouteAffinityKey(c, aggregateGroup)
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
	routeGroup = strings.TrimSpace(routeGroup)
	if routeGroup == "" {
		return
	}
	key := buildAggregateRouteAffinityKey(c, aggregateGroup)
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
