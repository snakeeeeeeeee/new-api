package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAsyncImageRouteTestContext() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func seedAsyncImageRouteChannel(t *testing.T, id int, group, modelName string, priority int64, weight uint) {
	t.Helper()
	channel := &model.Channel{
		Id: id, Name: group + "-retry-channel", Key: "sk-retry-test", Status: common.ChannelStatusEnabled,
		Group: group, Models: modelName, Priority: &priority, Weight: &weight, CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestAsyncImageOrdinaryRouteHonorsBudgetPriorityWeightAndExclusion(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	const modelName = "image-retry-model"
	seedAsyncImageRouteChannel(t, 7101, "default", modelName, 100, 1)
	seedAsyncImageRouteChannel(t, 7102, "default", modelName, 100, 0)
	seedAsyncImageRouteChannel(t, 7103, "default", modelName, 10, 5000)

	heavySelections := 0
	for index := 0; index < 200; index++ {
		channel, err := model.GetRandomSatisfiedChannelExcluding("default", modelName, nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.NotEqual(t, 7103, channel.Id, "lower priority must not participate while a higher priority remains")
		if channel.Id == 7101 {
			heavySelections++
		}
	}
	require.Equal(t, 200, heavySelections, "a zero-weight peer must not receive traffic while a positive-weight peer exists")

	channel, err := model.GetRandomSatisfiedChannelExcluding("default", modelName, []int{7101, 7102})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 7103, channel.Id)

	zeroRetry := &model.ImageTaskRetryState{
		Status: model.ImageTaskRetryStateActive, RetryLimit: 0, OriginalModel: modelName,
		CurrentRouteGroup: "default", CurrentRouteIndex: -1, CurrentGroupAttempts: 1,
		FailedChannelIDs: []int{7101},
	}
	next, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), zeroRetry)
	require.NoError(t, err)
	require.Nil(t, next)

	oneRetry := *zeroRetry
	oneRetry.RetryLimit = 1
	next, err = SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), &oneRetry)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotEqual(t, 7101, next.Channel.Id)
	require.Equal(t, 2, oneRetry.CurrentGroupAttempts)
}

func TestAsyncImageOrdinaryRouteRetryTimesTwoAllowsThreeDistinctChannels(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	const modelName = "image-retry-two-model"
	seedAsyncImageRouteChannel(t, 7151, "default", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7152, "default", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7153, "default", modelName, 100, 10)
	state := &model.ImageTaskRetryState{
		Status: model.ImageTaskRetryStateActive, RetryLimit: 2, OriginalModel: modelName,
		CurrentRouteGroup: "default", CurrentRouteIndex: -1, CurrentGroupAttempts: 1,
		FailedChannelIDs: []int{7151},
	}

	second, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), state)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, 7151, second.Channel.Id)
	state.AddFailedChannel(second.Channel.Id)
	third, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), state)
	require.NoError(t, err)
	require.NotNil(t, third)
	require.NotContains(t, state.FailedChannelIDs, third.Channel.Id)
	state.AddFailedChannel(third.Channel.Id)
	exhausted, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), state)
	require.NoError(t, err)
	require.Nil(t, exhausted)
	require.Equal(t, 3, state.CurrentGroupAttempts)
	require.Len(t, state.FailedChannelIDs, 3)
}

func TestAsyncImageFailoverUsesSubgroupBudgetBeforeSwitch(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	const modelName = "image-failover-model"
	aggregate := seedAggregateGroup(t, "image-failover", 1, 60, []string{"vip"}, []string{"image-primary", "image-secondary"})
	seedAsyncImageRouteChannel(t, 7201, "image-primary", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7202, "image-primary", modelName, 50, 10)
	seedAsyncImageRouteChannel(t, 7203, "image-secondary", modelName, 100, 10)

	state := &model.ImageTaskRetryState{
		Status: model.ImageTaskRetryStateActive, RetryLimit: 1, OriginalModel: modelName,
		AggregateGroup: aggregate.Name, RoutingMode: model.AggregateGroupRoutingModeFailover,
		CurrentRouteGroup: "image-primary", CurrentRouteIndex: 0, CurrentRoutePool: aggregateClusterDefaultRoutePool,
		CurrentGroupAttempts: 1, FailedChannelIDs: []int{7201}, AttemptedRouteKeys: []string{},
	}
	ctx := newAsyncImageRouteTestContext()
	next, err := SelectNextAsyncImageRoute(ctx, state)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, 7202, next.Channel.Id)
	require.False(t, next.SwitchedGroup)
	require.Equal(t, 2, state.CurrentGroupAttempts)

	state.AddFailedChannel(7202)
	next, err = SelectNextAsyncImageRoute(ctx, state)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, 7203, next.Channel.Id)
	require.True(t, next.SwitchedGroup)
	require.Equal(t, "image-secondary", state.CurrentRouteGroup)
	require.Equal(t, 1, state.CurrentGroupAttempts)
	require.True(t, state.HasAttemptedRouteKey(buildAggregateRouteAttemptKey(aggregateClusterDefaultRoutePool, "image-primary")))
}

func TestAsyncImageAggregateRetryTimesZeroStillTriesNextSubgroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	const modelName = "image-zero-aggregate-model"
	aggregate := seedAggregateGroup(t, "image-zero-failover", 1, 60, []string{"vip"}, []string{"image-a", "image-b"})
	seedAsyncImageRouteChannel(t, 7301, "image-a", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7302, "image-b", modelName, 100, 10)
	state := &model.ImageTaskRetryState{
		Status: model.ImageTaskRetryStateActive, RetryLimit: 0, OriginalModel: modelName,
		AggregateGroup: aggregate.Name, RoutingMode: model.AggregateGroupRoutingModeFailover,
		CurrentRouteGroup: "image-a", CurrentRouteIndex: 0, CurrentRoutePool: aggregateClusterDefaultRoutePool,
		CurrentGroupAttempts: 1, FailedChannelIDs: []int{7301},
	}
	next, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), state)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, 7302, next.Channel.Id)
	require.Equal(t, "image-b", next.RouteGroup)
}

func TestAsyncImageClusterExcludesExhaustedSubgroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	const modelName = "image-cluster-model"
	aggregate := seedAggregateGroupWithWeightedTargets(t, "image-cluster", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "cluster-a", Weight: common.GetPointer(100)},
		{RealGroup: "cluster-b", Weight: common.GetPointer(100)},
		{RealGroup: "cluster-c", Weight: common.GetPointer(100)},
	})
	seedAsyncImageRouteChannel(t, 7401, "cluster-a", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7402, "cluster-b", modelName, 100, 10)
	seedAsyncImageRouteChannel(t, 7403, "cluster-c", modelName, 100, 10)
	state := &model.ImageTaskRetryState{
		Status: model.ImageTaskRetryStateActive, RetryLimit: 0, OriginalModel: modelName,
		AggregateGroup: aggregate.Name, RoutingMode: model.AggregateGroupRoutingModeCluster,
		CurrentRouteGroup: "cluster-a", CurrentRouteIndex: 0, CurrentRoutePool: aggregateClusterDefaultRoutePool,
		CurrentGroupAttempts: 1, FailedChannelIDs: []int{7401},
	}
	next, err := SelectNextAsyncImageRoute(newAsyncImageRouteTestContext(), state)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotEqual(t, "cluster-a", next.RouteGroup)
	require.Contains(t, []int{7402, 7403}, next.Channel.Id)
	require.True(t, state.HasAttemptedRouteKey(buildAggregateRouteAttemptKey(aggregateClusterDefaultRoutePool, "cluster-a")))
}
