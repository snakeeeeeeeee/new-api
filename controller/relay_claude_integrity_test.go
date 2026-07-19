package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateGroupInternalRetryBudgetSkipsCurrentGroupForClaudeIntegrityFailure(t *testing.T) {
	integrityErr := types.NewErrorWithStatusCode(
		errors.New("missing content block"),
		types.ErrorCodeClaudeContentBlockMissing,
		http.StatusBadGateway,
	)
	require.Equal(t, 0, aggregateGroupInternalRetryBudget(integrityErr, 3))

	ordinaryErr := types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	require.Equal(t, 3, aggregateGroupInternalRetryBudget(ordinaryErr, 3))
}

func TestClaudeIntegrityFallbackHonorsAggregateRetryStatusCodes(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	group := &model.AggregateGroup{
		Name:                    "claude-integrity-retry-status",
		DisplayName:             "Claude integrity retry status",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 30,
		RetryStatusCodes:        "500-501,503-599",
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"default"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}, {RealGroup: "vip"}}))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, group.Name)
	integrityErr := types.NewErrorWithStatusCode(errors.New("missing content block"), types.ErrorCodeClaudeContentBlockMissing, http.StatusBadGateway)
	require.False(t, shouldRetry(ctx, integrityErr, 1))

	group.RetryStatusCodes = "500-599"
	require.NoError(t, group.UpdateWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}, {RealGroup: "vip"}}))
	require.True(t, shouldRetry(ctx, integrityErr, 1))

	group.RetryStatusCodes = ""
	require.NoError(t, group.UpdateWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}, {RealGroup: "vip"}}))
	decision, configured := service.ShouldRetryStatusCodeByAggregateGroup(group.Name, http.StatusBadGateway)
	require.False(t, configured)
	require.False(t, decision)
	require.True(t, shouldRetry(ctx, integrityErr, 1), "empty aggregate rule must use the global default retry policy")
}

func TestClaudeIntegrityClientDisconnectDoesNotRecordAggregateRouteFailure(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("original_model", "claude-test")
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "client-disconnect-metrics")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	service.RecordAggregateRouteRPMAttempt(ctx, "claude-test", "default")
	disconnectErr := types.NewErrorWithStatusCode(
		errors.New("client canceled request"),
		types.ErrorCodeDoRequestFailed,
		499,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)

	processChannelError(ctx, *types.NewChannelError(101, 0, "test", false, "", false), disconnectErr)

	stats := service.GetAggregateRouteRPMStats("client-disconnect-metrics", "claude-test", "default")
	require.Equal(t, 1, stats.RPM)
	require.Zero(t, stats.FailureRPM)
	require.False(t, shouldRetry(ctx, disconnectErr, 1))
}
