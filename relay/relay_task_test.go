package relay

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskModel2DtoOnlyReturnsResultURLForSuccessfulTask(t *testing.T) {
	success := &model.Task{
		TaskID: "task_success",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://vidgen.x.ai/video.mp4",
		},
	}
	assert.Equal(t, "https://vidgen.x.ai/video.mp4", TaskModel2Dto(success).ResultURL)

	failure := &model.Task{
		TaskID:     "task_failure",
		Status:     model.TaskStatusFailure,
		FailReason: "Generated video rejected by content moderation.",
	}
	assert.Empty(t, TaskModel2Dto(failure).ResultURL)
}

func setupRelayTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Channel{}))
	t.Cleanup(func() {
		model.DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestTaskModel2DtoDisplaysRealChannelPlatformForImageHandleTask(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:   321,
		Type: constant.ChannelTypeOpenAI,
		Key:  "test-key",
		Name: "openai-image-channel",
	}).Error)

	task := &model.Task{
		TaskID:    "task_image_handle",
		Platform:  constant.TaskPlatform("58"),
		ChannelId: 321,
		Status:    model.TaskStatusQueued,
	}

	result := TaskModel2Dto(task)

	assert.Equal(t, "58", result.Platform)
	assert.Equal(t, strconv.Itoa(constant.ChannelTypeOpenAI), result.DisplayPlatform)
}

func TestRelayTaskSubmitImageHandleClientTaskIDIdempotency(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_external_id",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		UserId:     7,
		ChannelId:  123,
		Quota:      42,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_external_id",
		"model":"gpt-image-2",
		"prompt":"already queued"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", "58")
	c.Set("model_mapping", "{}")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeImageHandle)
	common.SetContextKey(c, constant.ContextKeyChannelId, 123)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:8787")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "provider-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2")

	result, taskErr := RelayTaskSubmit(c, &relaycommon.RelayInfo{
		UserId:        7,
		UsingGroup:    "default",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.NotNil(t, result.ExistingTask)
	assert.Equal(t, "task_external_id", result.ExistingTask.TaskID)
	assert.Equal(t, 42, result.Quota)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"queued","task_id":"task_external_id"}`, recorder.Body.String())
}
