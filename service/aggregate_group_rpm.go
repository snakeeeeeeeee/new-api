package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	aggregateRouteRPMWindowSeconds = int64(60)
	aggregateRouteRPMTTL           = 2 * time.Hour
	aggregateRouteRPMRedisPrefix   = "new-api:aggregate_route_rpm:v2"

	aggregateRouteRPMMetricAttempt         = "attempt"
	aggregateRouteRPMMetricSuccess         = "success"
	aggregateRouteRPMMetricFailure         = "failure"
	aggregateRouteRPMMetricStrategyFailure = "strategy_failure"
	aggregateRouteRPMMetricSlowSuccess     = "slow_success"
)

type AggregateRouteRPMStats struct {
	RPM                int `json:"rpm"`
	SuccessRPM         int `json:"success_rpm"`
	FailureRPM         int `json:"failure_rpm"`
	StrategyFailureRPM int `json:"strategy_failure_rpm"`
	SlowSuccessRPM     int `json:"slow_success_rpm"`
}

type AggregateRouteWindowStats struct {
	WindowSeconds    int `json:"window_seconds"`
	Attempts         int `json:"attempts"`
	Successes        int `json:"successes"`
	Failures         int `json:"failures"`
	StrategyFailures int `json:"strategy_failures"`
	SlowSuccesses    int `json:"slow_successes"`
}

type aggregateRouteRPMMemoryEntry struct {
	Count     int64
	ExpiresAt int64
}

var (
	aggregateRouteRPMNow        = time.Now
	aggregateRouteRPMMemoryMu   sync.Mutex
	aggregateRouteRPMMemoryData = map[string]aggregateRouteRPMMemoryEntry{}
)

func buildAggregateRouteRPMBaseKey(groupName string, modelName string, routePool string, routeGroup string) string {
	routePool = normalizeAggregateClusterRoutePool(routePool)
	return common.Sha1([]byte(groupName + "\n" + modelName + "\n" + routePool + "\n" + routeGroup))
}

func buildAggregateRouteRPMKey(groupName string, modelName string, routePool string, routeGroup string, metric string, unixSecond int64) string {
	return fmt.Sprintf("%s:%s:%s:%d", aggregateRouteRPMRedisPrefix, buildAggregateRouteRPMBaseKey(groupName, modelName, routePool, routeGroup), metric, unixSecond)
}

func RecordAggregateRouteRPMAttempt(c *gin.Context, modelName string, routeGroup string) {
	recordAggregateRouteRPMFromContext(c, modelName, routeGroup, aggregateRouteRPMMetricAttempt)
}

func RecordAggregateRouteRPMSuccess(c *gin.Context, modelName string, routeGroup string) {
	recordAggregateRouteRPMFromContext(c, modelName, routeGroup, aggregateRouteRPMMetricSuccess)
}

func RecordAggregateRouteRPMFailure(c *gin.Context, modelName string) {
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	recordAggregateRouteRPMFromContext(c, modelName, routeGroup, aggregateRouteRPMMetricFailure)
}

func RecordAggregateRouteStrategyFailure(c *gin.Context, modelName string, routeGroup string) {
	recordAggregateRouteRPMFromContext(c, modelName, routeGroup, aggregateRouteRPMMetricStrategyFailure)
}

func RecordAggregateRouteSlowSuccess(c *gin.Context, modelName string, routeGroup string) {
	recordAggregateRouteRPMFromContext(c, modelName, routeGroup, aggregateRouteRPMMetricSlowSuccess)
}

func recordAggregateRouteRPMFromContext(c *gin.Context, modelName string, routeGroup string, metric string) {
	if c == nil {
		return
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	if aggregateGroup == "" {
		return
	}
	routePool := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool)
	recordAggregateRouteRPMForPool(aggregateGroup, modelName, routePool, routeGroup, metric)
}

func recordAggregateRouteRPM(groupName string, modelName string, routeGroup string, metric string) {
	recordAggregateRouteRPMForPool(groupName, modelName, aggregateClusterDefaultRoutePool, routeGroup, metric)
}

func recordAggregateRouteRPMForPool(groupName string, modelName string, routePool string, routeGroup string, metric string) {
	if groupName == "" || modelName == "" || routeGroup == "" || metric == "" {
		return
	}
	routePool = normalizeAggregateClusterRoutePool(routePool)
	now := aggregateRouteRPMNow()
	second := now.Unix()
	key := buildAggregateRouteRPMKey(groupName, modelName, routePool, routeGroup, metric, second)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		pipe := common.RDB.TxPipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, aggregateRouteRPMTTL)
		if _, err := pipe.Exec(ctx); err == nil {
			return
		}
	}

	aggregateRouteRPMMemoryMu.Lock()
	defer aggregateRouteRPMMemoryMu.Unlock()
	cleanupAggregateRouteRPMMemoryLocked(now.Unix())
	entry := aggregateRouteRPMMemoryData[key]
	entry.Count++
	entry.ExpiresAt = now.Add(aggregateRouteRPMTTL).Unix()
	aggregateRouteRPMMemoryData[key] = entry
}

func GetAggregateRouteRPMStats(groupName string, modelName string, routeGroup string) AggregateRouteRPMStats {
	return GetAggregateRouteRPMStatsForPool(groupName, modelName, aggregateClusterDefaultRoutePool, routeGroup)
}

func GetAggregateRouteRPMStatsForPool(groupName string, modelName string, routePool string, routeGroup string) AggregateRouteRPMStats {
	stats := GetAggregateRouteWindowStatsForPool(groupName, modelName, routePool, routeGroup, int(aggregateRouteRPMWindowSeconds))
	return AggregateRouteRPMStats{
		RPM:                stats.Attempts,
		SuccessRPM:         stats.Successes,
		FailureRPM:         stats.Failures,
		StrategyFailureRPM: stats.StrategyFailures,
		SlowSuccessRPM:     stats.SlowSuccesses,
	}
}

func GetAggregateRouteWindowStatsForPool(groupName string, modelName string, routePool string, routeGroup string, windowSeconds int) AggregateRouteWindowStats {
	if windowSeconds <= 0 {
		windowSeconds = int(aggregateRouteRPMWindowSeconds)
	}
	return AggregateRouteWindowStats{
		WindowSeconds:    windowSeconds,
		Attempts:         int(sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, aggregateRouteRPMMetricAttempt, windowSeconds)),
		Successes:        int(sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, aggregateRouteRPMMetricSuccess, windowSeconds)),
		Failures:         int(sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, aggregateRouteRPMMetricFailure, windowSeconds)),
		StrategyFailures: int(sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, aggregateRouteRPMMetricStrategyFailure, windowSeconds)),
		SlowSuccesses:    int(sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, aggregateRouteRPMMetricSlowSuccess, windowSeconds)),
	}
}

func sumAggregateRouteRPM(groupName string, modelName string, routePool string, routeGroup string, metric string) int64 {
	return sumAggregateRouteRPMForWindow(groupName, modelName, routePool, routeGroup, metric, int(aggregateRouteRPMWindowSeconds))
}

func sumAggregateRouteRPMForWindow(groupName string, modelName string, routePool string, routeGroup string, metric string, windowSeconds int) int64 {
	if groupName == "" || modelName == "" || routeGroup == "" || metric == "" {
		return 0
	}
	if windowSeconds <= 0 {
		windowSeconds = int(aggregateRouteRPMWindowSeconds)
	}
	routePool = normalizeAggregateClusterRoutePool(routePool)
	now := aggregateRouteRPMNow().Unix()
	from := now - int64(windowSeconds) + 1
	if common.RedisEnabled && common.RDB != nil {
		total, ok := sumAggregateRouteRPMRedis(groupName, modelName, routePool, routeGroup, metric, from, now)
		if ok {
			return total
		}
	}
	return sumAggregateRouteRPMMemory(groupName, modelName, routePool, routeGroup, metric, from, now)
}

func sumAggregateRouteRPMRedis(groupName string, modelName string, routePool string, routeGroup string, metric string, from int64, to int64) (int64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringCmd, 0, to-from+1)
	for second := from; second <= to; second++ {
		cmds = append(cmds, pipe.Get(ctx, buildAggregateRouteRPMKey(groupName, modelName, routePool, routeGroup, metric, second)))
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return 0, false
	}
	total := int64(0)
	for _, cmd := range cmds {
		value, err := cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return 0, false
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		total += count
	}
	return total, true
}

func sumAggregateRouteRPMMemory(groupName string, modelName string, routePool string, routeGroup string, metric string, from int64, to int64) int64 {
	aggregateRouteRPMMemoryMu.Lock()
	defer aggregateRouteRPMMemoryMu.Unlock()
	cleanupAggregateRouteRPMMemoryLocked(to)

	total := int64(0)
	for second := from; second <= to; second++ {
		key := buildAggregateRouteRPMKey(groupName, modelName, routePool, routeGroup, metric, second)
		entry, ok := aggregateRouteRPMMemoryData[key]
		if !ok || entry.ExpiresAt <= to {
			continue
		}
		total += entry.Count
	}
	return total
}

func cleanupAggregateRouteRPMMemoryLocked(now int64) {
	for key, entry := range aggregateRouteRPMMemoryData {
		if entry.ExpiresAt <= now {
			delete(aggregateRouteRPMMemoryData, key)
		}
	}
}
