package service

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPublicVideoTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalMemoryCache := common.MemoryCacheEnabled
	originalServerAddress := system_setting.ServerAddress
	common.MemoryCacheEnabled = false
	system_setting.ServerAddress = "https://gateway.example"
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}, &model.VideoTaskRequest{}, &model.Asset{}))
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCache
		system_setting.ServerAddress = originalServerAddress
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestBuildPublicVideoTaskExposesTruthfulProgressMetadata(t *testing.T) {
	public := buildPublicVideoTask(&model.Task{
		Status: model.TaskStatusInProgress, Progress: "47%",
		PrivateData: model.TaskPrivateData{
			ProgressMetadataSet: true, ProgressKnown: true,
			ProgressSource: "upstream_percent", ProgressStage: "generating",
		},
	}, nil, nil)

	assert.Equal(t, 47, public.Progress)
	assert.True(t, public.ProgressKnown)
	assert.Equal(t, "upstream_percent", public.ProgressSource)
	assert.Equal(t, "generating", public.Stage)
}

func TestBuildOpenAIVideoCompatibilityTaskProjectsOfficialFields(t *testing.T) {
	db := setupPublicVideoTaskTestDB(t)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_openai_video_projection", UserId: 7, Platform: "59",
		Action: constant.TaskActionVideoGeneration, Status: model.TaskStatusSuccess,
		Progress: "100%", SubmitTime: now - 5, FinishTime: now,
		Properties: model.Properties{OriginModelName: "adobe-seedance-2.0-fast-480p", AssetType: constant.TaskAssetTypeVideo, Operation: "generation"},
		PrivateData: model.TaskPrivateData{OpenAIVideoCompatibility: &dto.OpenAIVideoCompatibilityMetadata{
			Version: dto.OpenAIVideoCompatibilityVersion, Seconds: 6, Size: "1280x720",
		}},
	}
	require.NoError(t, db.Create(task).Error)
	requestJSON, err := common.Marshal(dto.VideoTaskCreateRequest{
		Model: task.Properties.OriginModelName, Operation: "generation",
		Input: dto.VideoTaskInputRequest{Prompt: "animate"}, Metadata: map[string]any{"job": "demo"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewVideoTaskRequest(task, 7, nil, "fingerprint", "", requestJSON)).Error)

	video, err := BuildOpenAIVideoCompatibilityTask(task)
	require.NoError(t, err)
	assert.Equal(t, "video", video.Object)
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	assert.Equal(t, "6", video.Seconds)
	assert.Equal(t, "1280x720", video.Size)
	assert.Equal(t, map[string]any{"job": "demo"}, video.Metadata)
	assert.Equal(t, now, video.CompletedAt)

	payload, err := common.Marshal(video)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "openai_video_compatibility")
}

func TestBuildPublicVideoTaskProjectsDirectAndProxiedOutputs(t *testing.T) {
	db := setupPublicVideoTaskTestDB(t)
	baseURL := "https://upstream.example/api"
	require.NoError(t, db.Create(&model.Channel{Id: 9, Type: constant.ChannelTypeXai, Key: "secret", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}).Error)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_public_video", UserId: 7, ChannelId: 9, Platform: "48",
		Action: constant.TaskActionVideoGeneration, Status: model.TaskStatusSuccess,
		Progress: "100%", SubmitTime: now, FinishTime: now,
		Properties: model.Properties{OriginModelName: "client-video-alias", AssetType: constant.TaskAssetTypeVideo, Operation: "generation"},
	}
	require.NoError(t, db.Create(task).Error)
	requestJSON, err := common.Marshal(dto.VideoTaskCreateRequest{
		Model: "client-video-alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "generate"},
		ClientReferenceID: "order-123", Metadata: map[string]any{"tenant": "public"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewVideoTaskRequest(task, 7, nil, "fingerprint", "order-123", requestJSON)).Error)
	require.NoError(t, model.CreateAssetsForTaskTx(db, []model.AssetCreateInput{
		{Task: task, AssetIndex: 0, AssetType: model.AssetTypeVideo, URL: "https://cdn.example/video.mp4", MimeType: "video/mp4", DurationMS: 5000},
		{Task: task, AssetIndex: 1, AssetType: model.AssetTypeVideo, URL: "/v1/videos/upstream/content", Metadata: model.AssetMetadata{"resolver": "channel", "internal_secret": "hidden"}},
	}))

	public, err := BuildPublicVideoTask(task)
	require.NoError(t, err)
	require.NotNil(t, public.Result)
	require.Len(t, public.Result.Videos, 2)
	assert.Equal(t, "video.task", public.Object)
	assert.Equal(t, "client-video-alias", public.Model)
	assert.Equal(t, "order-123", public.ClientReferenceID)
	assert.Equal(t, "https://cdn.example/video.mp4", public.Result.Videos[0].URL)
	assert.Equal(t, VideoURLAuthNone, public.Result.Videos[0].URLAuth)
	assert.Equal(t, "https://gateway.example/v1/assets/"+public.Result.Videos[1].AssetID+"/content", public.Result.Videos[1].URL)
	assert.Equal(t, VideoURLAuthResourceAPIKey, public.Result.Videos[1].URLAuth)
	assert.True(t, public.Result.Videos[1].Temporary)

	payload, err := common.Marshal(public)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "channel_id")
	assert.NotContains(t, string(payload), "upstream_task_id")
	assert.NotContains(t, string(payload), "resolver")
}

func TestPublicVideoAssetURLDoesNotExposeAuthenticatedProviderURL(t *testing.T) {
	db := setupPublicVideoTaskTestDB(t)
	baseURL := "https://generativelanguage.googleapis.com"
	require.NoError(t, db.Create(&model.Channel{
		Id: 10, Type: constant.ChannelTypeGemini, Key: "secret", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled,
	}).Error)
	asset := &model.Asset{
		AssetID: "asset_private_video", AssetType: model.AssetTypeVideo, ChannelID: 10,
		URL: "https://files.example/video.mp4?key=provider-secret",
	}

	publicURL, urlAuth := PublicVideoAssetURL(asset)

	assert.Equal(t, "https://gateway.example/v1/assets/asset_private_video/content", publicURL)
	assert.Equal(t, VideoURLAuthResourceAPIKey, urlAuth)
	assert.NotContains(t, publicURL, "provider-secret")
}

func TestBuildPublicVideoTaskPreservesKnownReferenceDurationErrors(t *testing.T) {
	for _, code := range []string{
		"invalid_reference_media_duration",
		"reference_media_duration_exceeded",
	} {
		t.Run(code, func(t *testing.T) {
			task := &model.Task{
				TaskID:     "task_" + code,
				Status:     model.TaskStatusFailure,
				FailReason: "reference media rejected",
			}
			task.SetData(map[string]any{
				"error": map[string]any{
					"code":    code,
					"message": "reference media rejected",
				},
			})

			public := buildPublicVideoTask(task, nil, nil)

			require.NotNil(t, public.Error)
			assert.Equal(t, code, public.Error.Code)
			assert.Equal(t, "reference media rejected", public.Error.Message)
		})
	}
}

func TestBuildPublicVideoTaskMasksUnknownProviderErrorCodes(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_private_provider_error",
		Status:     model.TaskStatusFailure,
		FailReason: "provider rejected the request",
	}
	task.SetData(map[string]any{
		"error": map[string]any{
			"code":    "provider_internal_policy",
			"message": "provider rejected the request",
		},
	})

	public := buildPublicVideoTask(task, nil, nil)

	require.NotNil(t, public.Error)
	assert.Equal(t, "video_task_failed", public.Error.Code)
	assert.Equal(t, "Video task failed", public.Error.Message)
	assert.NotContains(t, public.Error.Message, "provider")
}

func TestBuildPublicVideoTaskProjectsStructuredProviderNeutralError(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_upstream_status_unavailable",
		Status:     model.TaskStatusFailure,
		FailReason: `video poll failed: 408 {"error_code":"timeout_error","message":"Gateway timeout from fal-ai-video"}`,
		PrivateData: model.TaskPrivateData{
			LastUpstreamStatus: http.StatusRequestTimeout,
			BillingContext: &model.TaskBillingContext{
				RequestId: "req_public_trace_1",
			},
		},
	}
	task.SetData(map[string]any{
		"error_code": "timeout_error",
		"message":    "Gateway timeout from fal-ai-video",
	})

	public := buildPublicVideoTask(task, nil, nil)

	require.NotNil(t, public.Error)
	assert.Equal(t, "upstream_timeout", public.Error.Code)
	assert.Equal(t, "Generation status was temporarily unavailable for too long", public.Error.Message)
	assert.False(t, public.Error.Retryable)
	assert.Equal(t, http.StatusRequestTimeout, public.Error.UpstreamStatus)
	assert.Equal(t, "timeout_error", public.Error.UpstreamErrorCode)
	assert.Equal(t, "req_public_trace_1", public.Error.RequestID)
	encoded, err := common.Marshal(public)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "fal-ai-video")
	assert.NotContains(t, string(encoded), "Gateway timeout")
}

func TestBuildPublicVideoTaskSanitizesLegacySubmissionFailure(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_submission_unknown",
		Status:     model.TaskStatusFailure,
		FailReason: "Adobe submission connection ended after it may have been accepted",
	}
	task.SetData(map[string]any{
		"error": map[string]any{
			"code":    "submission_unknown",
			"message": task.FailReason,
		},
	})

	public := buildPublicVideoTask(task, nil, nil)

	assert.Equal(t, "failed", public.Status)
	require.NotNil(t, public.Error)
	assert.Equal(t, "video_task_failed", public.Error.Code)
	assert.Equal(t, "Submission connection ended before the result was confirmed", public.Error.Message)
	assert.False(t, public.Error.Retryable)
	assert.NotContains(t, public.Error.Message, "Adobe")
}

func TestBuildPublicVideoTaskProjectsModerationAsRetryable(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_content_moderated",
		Status:     model.TaskStatusFailure,
		FailReason: "The generation was rejected by content moderation. Please revise the prompt or reference media and try again.",
	}
	task.SetData(map[string]any{
		"error": map[string]any{
			"code":    "content_moderated",
			"message": task.FailReason,
		},
	})

	public := buildPublicVideoTask(task, nil, nil)

	require.NotNil(t, public.Error)
	assert.Equal(t, "content_moderated", public.Error.Code)
	assert.Equal(t, task.FailReason, public.Error.Message)
	assert.True(t, public.Error.Retryable)
}

func TestBuildPublicVideoTaskProjectsPrivateEntitlementError(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_private_generation_unavailable",
		Status:     model.TaskStatusFailure,
		FailReason: "Private generation is unavailable for the selected account",
	}
	task.SetData(map[string]any{
		"error": map[string]any{
			"code":    "private_generation_unavailable",
			"message": task.FailReason,
		},
	})

	public := buildPublicVideoTask(task, nil, nil)

	require.NotNil(t, public.Error)
	assert.Equal(t, "private_generation_unavailable", public.Error.Code)
	assert.Equal(t, task.FailReason, public.Error.Message)
	assert.False(t, public.Error.Retryable)
}
