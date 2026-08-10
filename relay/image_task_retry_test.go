package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewAsyncImageRetryStateSnapshotsRetryTimes(t *testing.T) {
	originalRetryTimes := common.RetryTimes
	t.Cleanup(func() { common.RetryTimes = originalRetryTimes })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	parent := &model.Task{TaskID: "task_retry_snapshot", ChannelId: 7, Group: "default", Quota: 100}
	info := &relaycommon.RelayInfo{
		UserGroup: "default", TokenGroup: "default", OriginModelName: "gpt-image-2",
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 7}, TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}

	common.RetryTimes = 1
	oldTaskState := newAsyncImageRetryState(c, parent, info)
	common.RetryTimes = 2
	newTaskState := newAsyncImageRetryState(c, parent, info)

	assert.Equal(t, 1, oldTaskState.RetryLimit)
	assert.Equal(t, 2, newTaskState.RetryLimit)
	assert.Equal(t, 1, oldTaskState.RetryLimit, "in-flight state must not follow later global configuration")
}

func TestNewAsyncImageRetryStateSnapshotsLockedChannel(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "7")
	parent := &model.Task{TaskID: "task_retry_locked", ChannelId: 7, Group: "default"}
	info := &relaycommon.RelayInfo{
		UserGroup: "default", TokenGroup: "default", OriginModelName: "gpt-image-2",
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 7}, TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}

	state := newAsyncImageRetryState(c, parent, info)

	assert.True(t, state.LockedChannel)
}
