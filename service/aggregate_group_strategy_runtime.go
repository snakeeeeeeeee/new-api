package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const aggregateGroupRouteStrategyStateTTL = 24 * time.Hour

type AggregateGroupRouteStrategyState struct {
	ConsecutiveFailures int   `json:"consecutive_failures"`
	ConsecutiveSlows    int   `json:"consecutive_slows"`
	DegradedUntil       int64 `json:"degraded_until"`
	LastFailureAt       int64 `json:"last_failure_at"`
	LastSlowAt          int64 `json:"last_slow_at"`
	LastSuccessAt       int64 `json:"last_success_at"`
}

type aggregateGroupRouteStrategyStateEntry struct {
	State     AggregateGroupRouteStrategyState
	ExpiresAt int64
}

var aggregateGroupRouteStrategyStateMemory sync.Map

func buildAggregateGroupRouteStrategyStateKey(groupName string, modelName string, routeGroup string) string {
	return fmt.Sprintf("aggregate_group:smart_state:%s:%s:%s", groupName, modelName, routeGroup)
}

func GetAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string) (*AggregateGroupRouteStrategyState, error) {
	if groupName == "" || modelName == "" || routeGroup == "" {
		return nil, nil
	}
	key := buildAggregateGroupRouteStrategyStateKey(groupName, modelName, routeGroup)
	if common.RedisEnabled {
		var state AggregateGroupRouteStrategyState
		err := common.RedisHGetObj(key, &state)
		if err == nil {
			return &state, nil
		}
	}
	if value, ok := aggregateGroupRouteStrategyStateMemory.Load(key); ok {
		entry, ok := value.(aggregateGroupRouteStrategyStateEntry)
		if ok {
			if entry.ExpiresAt <= time.Now().Unix() {
				aggregateGroupRouteStrategyStateMemory.Delete(key)
				return nil, nil
			}
			state := entry.State
			return &state, nil
		}
	}
	return nil, nil
}

func SetAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string, state *AggregateGroupRouteStrategyState) error {
	if groupName == "" || modelName == "" || routeGroup == "" || state == nil {
		return nil
	}
	key := buildAggregateGroupRouteStrategyStateKey(groupName, modelName, routeGroup)
	if common.RedisEnabled {
		if err := common.RedisHSetObj(key, state, aggregateGroupRouteStrategyStateTTL); err != nil {
			return err
		}
		return nil
	}
	aggregateGroupRouteStrategyStateMemory.Store(key, aggregateGroupRouteStrategyStateEntry{
		State:     *state,
		ExpiresAt: time.Now().Add(aggregateGroupRouteStrategyStateTTL).Unix(),
	})
	return nil
}

func ResetAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string) error {
	if groupName == "" || modelName == "" || routeGroup == "" {
		return nil
	}
	key := buildAggregateGroupRouteStrategyStateKey(groupName, modelName, routeGroup)
	if common.RedisEnabled {
		return common.RedisDelKey(key)
	}
	aggregateGroupRouteStrategyStateMemory.Delete(key)
	return nil
}
