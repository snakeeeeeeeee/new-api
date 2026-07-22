package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoTaskCreateRequestPreservesExplicitZero(t *testing.T) {
	var request dto.VideoTaskCreateRequest
	require.NoError(t, common.UnmarshalStrict([]byte(`{
		"model":"grok-imagine-video-1.5",
		"operation":"generation",
		"input":{"prompt":"animate"},
		"output":{"duration":0}
	}`), &request))
	require.NotNil(t, request.Output.Duration)
	assert.Zero(t, *request.Output.Duration)
	canonical, err := common.Marshal(request)
	require.NoError(t, err)
	assert.Contains(t, string(canonical), `"duration":0`)
}

func TestValidateVideoTaskCreateRequestOperations(t *testing.T) {
	validURL := dto.VideoTaskSource{URL: "https://example.com/source.mp4"}
	tests := []struct {
		name      string
		request   dto.VideoTaskCreateRequest
		wantParam string
	}{
		{
			name: "generation",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "generate"}},
		},
		{
			name: "reference generation",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "generate", ReferenceImages: []dto.VideoTaskSource{{Provider: "xai", FileID: "file_1"}}}},
		},
		{
			name: "edit output is provider validated",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "edit",
				Input:  dto.VideoTaskInputRequest{Prompt: "edit", Video: &validURL},
				Output: dto.VideoTaskOutputRequest{Duration: common.GetPointer(5)}},
		},
		{
			name: "extension requires video",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "extension",
				Input: dto.VideoTaskInputRequest{Prompt: "extend"}},
			wantParam: "input.video",
		},
		{
			name: "generation image combinations are provider validated",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "generate", Image: &dto.VideoTaskSource{URL: "data:image/png;base64,AA"}, ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}}}},
		},
		{
			name: "edit images are provider validated",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "edit",
				Input: dto.VideoTaskInputRequest{Prompt: "edit", Video: &validURL, ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}}}},
		},
		{
			name: "empty reference image array",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "generate", ReferenceImages: []dto.VideoTaskSource{}}},
			wantParam: "input.reference_images",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalizeVideoTaskCreateRequest(&test.request)
			param, message := validateVideoTaskCreateRequest(&test.request)
			assert.Equal(t, test.wantParam, param)
			if test.wantParam == "" {
				assert.Empty(t, message)
			} else {
				assert.NotEmpty(t, message)
			}
		})
	}
}

func TestValidateVideoTaskSourceRejectsAmbiguousSource(t *testing.T) {
	assert.NotEmpty(t, validateVideoTaskSource(&dto.VideoTaskSource{
		URL: "https://example.com/video.mp4", Provider: "xai", FileID: "file_1",
	}))
	assert.NotEmpty(t, validateVideoTaskSource(&dto.VideoTaskSource{Provider: "xai"}))
	assert.Empty(t, validateVideoTaskSource(&dto.VideoTaskSource{URL: "data:video/mp4;base64,AA"}))
}

func TestNormalizeVideoTaskProviderOptionsNamespace(t *testing.T) {
	request := dto.VideoTaskCreateRequest{
		ProviderOptions: map[string]map[string]any{" XAI ": {"seed": float64(7)}},
	}
	normalizeVideoTaskCreateRequest(&request)

	require.Contains(t, request.ProviderOptions, "xai")
	assert.Equal(t, float64(7), request.ProviderOptions["xai"]["seed"])
}

func TestNormalizeVideoTaskRejectsDuplicateProviderNamespace(t *testing.T) {
	request := dto.VideoTaskCreateRequest{
		Model: "video-model", Operation: "generation",
		Input: dto.VideoTaskInputRequest{Prompt: "generate"},
		ProviderOptions: map[string]map[string]any{
			"xai": {"seed": float64(7)},
			"XAI": {"seed": float64(8)},
		},
	}
	normalizeVideoTaskCreateRequest(&request)
	param, message := validateVideoTaskCreateRequest(&request)

	assert.Equal(t, "provider_options", param)
	assert.NotEmpty(t, message)
}

func TestQueryVideoTasksPreservesOrderAndUserIsolation(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.VideoTaskRequest{}, &model.Asset{}))
	now := time.Now().Unix()
	create := func(userID int, taskID string) {
		task := &model.Task{
			TaskID: taskID, UserId: userID, Platform: "48", Action: "videoGeneration",
			Status: model.TaskStatusQueued, Progress: "20%", SubmitTime: now,
			Properties: model.Properties{OriginModelName: "video-model", AssetType: "video", Operation: "generation"},
		}
		require.NoError(t, db.Create(task).Error)
		requestJSON := []byte(`{"model":"video-model","operation":"generation","input":{"prompt":"generate"}}`)
		require.NoError(t, db.Create(model.NewVideoTaskRequest(task, userID, nil, "fingerprint-"+taskID, "", requestJSON)).Error)
	}
	create(1, "task_first")
	create(1, "task_second")
	create(2, "task_other")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/tasks/query", bytes.NewBufferString(`{"task_ids":["task_second","task_other","task_first"]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	QueryVideoTasks(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response dto.VideoTaskListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 2)
	assert.Equal(t, "task_second", response.Data[0].ID)
	assert.Equal(t, "task_first", response.Data[1].ID)
	assert.Equal(t, []string{"task_other"}, response.Missing)
}

func TestPublicAssetMetadataRemovesInternalVideoKeys(t *testing.T) {
	metadata := publicAssetMetadata(model.AssetMetadata{
		"source": "task_info.video_outputs", "resolver": "channel",
		"provider_reference": "private-ref", "internal_token": "secret",
		"format": "mp4",
	})
	assert.Equal(t, map[string]any{"format": "mp4"}, metadata)
}

func TestPublicAssetDTOOmitsInternalRoutingFields(t *testing.T) {
	item := assetToAPIItem(&model.Asset{
		AssetID: "asset_public", TaskID: "task_public", AssetType: model.AssetTypeImage,
		URL: "https://cdn.example.com/image.png", Platform: "48", Action: "videoGeneration",
		Status: model.AssetStatusAvailable,
	})
	payload, err := common.Marshal(item)
	require.NoError(t, err)

	assert.NotContains(t, string(payload), "platform")
	assert.NotContains(t, string(payload), "videoGeneration")
}
