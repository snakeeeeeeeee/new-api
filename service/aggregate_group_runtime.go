package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const aggregateGroupRuntimeStateTTL = 24 * time.Hour

type AggregateGroupRuntimeState struct {
	ActiveIndex   int    `json:"active_index"`
	ActiveGroup   string `json:"active_group"`
	RoutingMode   string `json:"routing_mode,omitempty"`
	LastFailAt    int64  `json:"last_fail_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	LastSwitchAt  int64  `json:"last_switch_at"`
	ActiveSinceAt int64  `json:"active_since_at"`
}

type aggregateGroupRuntimeStateEntry struct {
	State     AggregateGroupRuntimeState
	ExpiresAt int64
}

var aggregateGroupRuntimeStateMemory sync.Map

func buildAggregateGroupRuntimeStateKey(groupName string, modelName string) string {
	return buildAggregateGroupRuntimeStateKeyForPool(groupName, modelName, "")
}

func buildAggregateGroupRuntimeStateKeyForPool(groupName string, modelName string, routePool string) string {
	routePool = normalizeAggregateClusterRoutePool(routePool)
	if routePool != aggregateClusterDefaultRoutePool {
		return fmt.Sprintf("aggregate_group:state:%s:%s:pool:%s", groupName, modelName, routePool)
	}
	return fmt.Sprintf("aggregate_group:state:%s:%s", groupName, modelName)
}

func GetAggregateGroupRuntimeState(groupName string, modelName string) (*AggregateGroupRuntimeState, error) {
	return GetAggregateGroupRuntimeStateForPool(groupName, modelName, "")
}

func GetAggregateGroupRuntimeStateForPool(groupName string, modelName string, routePool string) (*AggregateGroupRuntimeState, error) {
	if groupName == "" || modelName == "" {
		return nil, nil
	}
	key := buildAggregateGroupRuntimeStateKeyForPool(groupName, modelName, routePool)
	if common.RedisEnabled {
		var state AggregateGroupRuntimeState
		err := common.RedisHGetObj(key, &state)
		if err == nil {
			return &state, nil
		}
	}
	if value, ok := aggregateGroupRuntimeStateMemory.Load(key); ok {
		entry, ok := value.(aggregateGroupRuntimeStateEntry)
		if ok {
			if entry.ExpiresAt <= time.Now().Unix() {
				aggregateGroupRuntimeStateMemory.Delete(key)
				return nil, nil
			}
			state := entry.State
			return &state, nil
		}
	}
	return nil, nil
}

func SetAggregateGroupRuntimeState(groupName string, modelName string, state *AggregateGroupRuntimeState) error {
	return SetAggregateGroupRuntimeStateForPool(groupName, modelName, "", state)
}

func SetAggregateGroupRuntimeStateForPool(groupName string, modelName string, routePool string, state *AggregateGroupRuntimeState) error {
	if groupName == "" || modelName == "" || state == nil {
		return nil
	}
	key := buildAggregateGroupRuntimeStateKeyForPool(groupName, modelName, routePool)
	if common.RedisEnabled {
		if err := common.RedisHSetObj(key, state, aggregateGroupRuntimeStateTTL); err != nil {
			return err
		}
		return nil
	}
	aggregateGroupRuntimeStateMemory.Store(key, aggregateGroupRuntimeStateEntry{
		State:     *state,
		ExpiresAt: time.Now().Add(aggregateGroupRuntimeStateTTL).Unix(),
	})
	return nil
}

func ResetAggregateGroupRuntimeState(groupName string, modelName string) error {
	return ResetAggregateGroupRuntimeStateForPool(groupName, modelName, "")
}

func ResetAggregateGroupRuntimeStateForPool(groupName string, modelName string, routePool string) error {
	if groupName == "" || modelName == "" {
		return nil
	}
	key := buildAggregateGroupRuntimeStateKeyForPool(groupName, modelName, routePool)
	if common.RedisEnabled {
		return common.RedisDelKey(key)
	}
	aggregateGroupRuntimeStateMemory.Delete(key)
	return nil
}
