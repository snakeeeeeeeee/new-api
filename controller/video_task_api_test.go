package controller

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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
		"output":{"duration":0,"generate_audio":false}
	}`), &request))
	require.NotNil(t, request.Output.Duration)
	assert.Zero(t, *request.Output.Duration)
	require.NotNil(t, request.Output.GenerateAudio)
	assert.False(t, *request.Output.GenerateAudio)
	canonical, err := common.Marshal(request)
	require.NoError(t, err)
	assert.Contains(t, string(canonical), `"duration":0`)
	assert.Contains(t, string(canonical), `"generate_audio":false`)
}

func TestVideoTaskGenerateAudioStrictTypes(t *testing.T) {
	valid := []string{
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":true}}`,
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":false}}`,
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{}}`,
	}
	for _, body := range valid {
		var request dto.VideoTaskCreateRequest
		require.NoError(t, common.UnmarshalStrict([]byte(body), &request))
		assert.Nil(t, validateVideoTaskGenerateAudioJSON([]byte(body)))
	}

	invalid := []string{
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":"true"}}`,
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":1}}`,
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":{}}}`,
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":[]}}`,
	}
	for _, body := range invalid {
		var request dto.VideoTaskCreateRequest
		assert.Error(t, common.UnmarshalStrict([]byte(body), &request))
	}

	problem := validateVideoTaskGenerateAudioJSON([]byte(
		`{"model":"video","operation":"generation","input":{"prompt":"cat"},"output":{"generate_audio":null}}`,
	))
	require.NotNil(t, problem)
	assert.Equal(t, http.StatusBadRequest, problem.status)
	assert.Equal(t, "invalid_request", problem.code)
	assert.Equal(t, "output.generate_audio", problem.param)
}

func TestValidateReservedVideoProviderOptionsRejectsPublicDuplicates(t *testing.T) {
	tests := []struct {
		key         string
		replacement string
	}{
		{key: " Generate_Audio ", replacement: "output.generate_audio"},
		{key: " Reference_Mode ", replacement: "input.reference_mode"},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			request := dto.VideoTaskCreateRequest{ProviderOptions: map[string]map[string]any{
				" Leonardo_Video ": {test.key: false},
			}}
			normalizeVideoTaskCreateRequest(&request)
			problem := validateReservedVideoProviderOptions(&request)
			require.NotNil(t, problem)
			assert.Equal(t, "invalid_provider_options", problem.code)
			assert.Contains(t, problem.message, test.replacement)
		})
	}

	request := dto.VideoTaskCreateRequest{ProviderOptions: map[string]map[string]any{
		"xai": {"user": "customer-1"},
	}}
	assert.Nil(t, validateReservedVideoProviderOptions(&request))
}

func TestVideoTaskGenerateAudioIdempotencyConflicts(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.VideoTaskRequest{}))

	key := "video-audio-idempotency"
	trueValue := true
	falseValue := false
	base := dto.VideoTaskCreateRequest{
		Model: "seedance-2.0-fast-480p", Operation: "generation",
		Input:  dto.VideoTaskInputRequest{Prompt: "cinematic sunrise"},
		Output: dto.VideoTaskOutputRequest{GenerateAudio: &trueValue},
	}
	fingerprint := func(request dto.VideoTaskCreateRequest) string {
		canonical, err := common.Marshal(request)
		require.NoError(t, err)
		return fmt.Sprintf("%x", sha256.Sum256(canonical))
	}

	task := &model.Task{
		TaskID: "task_video_audio_idempotency", UserId: 81, Platform: "61",
		Action: "videoGeneration", Status: model.TaskStatusQueued, SubmitTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: base.Model, AssetType: "video", Operation: "generation"},
	}
	require.NoError(t, db.Create(task).Error)
	requestJSON, err := videoTaskRequestSnapshot(base)
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewVideoTaskRequest(task, 81, &key, fingerprint(base), "", requestJSON)).Error)

	tests := []struct {
		name       string
		request    dto.VideoTaskCreateRequest
		statusCode int
		code       string
	}{
		{name: "same explicit true replays", request: base, statusCode: http.StatusAccepted},
		{name: "explicit false conflicts", request: func() dto.VideoTaskCreateRequest {
			request := base
			request.Output.GenerateAudio = &falseValue
			return request
		}(), statusCode: http.StatusConflict, code: "idempotency_key_conflict"},
		{name: "omitted conflicts with explicit true", request: func() dto.VideoTaskCreateRequest {
			request := base
			request.Output.GenerateAudio = nil
			return request
		}(), statusCode: http.StatusConflict, code: "idempotency_key_conflict"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 81)
			assert.True(t, replayVideoTaskRequest(ctx, key, fingerprint(test.request)))
			assert.Equal(t, test.statusCode, recorder.Code, recorder.Body.String())
			if test.code != "" {
				assert.Contains(t, recorder.Body.String(), `"code":"`+test.code+`"`)
			} else {
				assert.Equal(t, "true", recorder.Header().Get("Idempotent-Replayed"))
			}
		})
	}
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
			wantParam: "input.reference_images[0]",
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
			name: "generation data URL is rejected",
			request: dto.VideoTaskCreateRequest{Model: "video-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "generate", Image: &dto.VideoTaskSource{URL: "data:image/png;base64,AA"}, ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}}}},
			wantParam: "input.image",
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
	assert.NotEmpty(t, validateVideoReferenceSource(&dto.VideoTaskSource{URL: "data:video/mp4;base64,AA"}))
}

func TestValidateVideoTaskMediaReferences(t *testing.T) {
	request := dto.VideoTaskCreateRequest{
		Model: "seedance-2.0-720p", Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt: "combine references", ReferenceMode: "media",
			ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/a.png", Name: "character"}},
			ReferenceVideos: []dto.VideoTaskSource{{URL: "https://example.com/a.mp4", Name: "motion"}},
			ReferenceAudios: []dto.VideoTaskSource{{URL: "https://example.com/a.m4a", Name: "music"}},
		},
	}
	normalizeVideoTaskCreateRequest(&request)
	param, message := validateVideoTaskCreateRequest(&request)
	assert.Empty(t, param)
	assert.Empty(t, message)

	request.Input.ReferenceAudios[0].Name = "motion"
	param, message = validateVideoTaskCreateRequest(&request)
	assert.Equal(t, "input", param)
	assert.Contains(t, message, "unique")
}

func TestValidateVideoTaskImagesReferences(t *testing.T) {
	request := dto.VideoTaskCreateRequest{
		Model: "adobe-kling-3.0-omni-720p", Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt: "combine references", ReferenceMode: "images",
			ReferenceImages: []dto.VideoTaskSource{
				{URL: "https://example.com/a.png"},
				{URL: "https://example.com/b.png"},
				{URL: "https://example.com/c.png"},
			},
		},
	}
	normalizeVideoTaskCreateRequest(&request)
	param, message := validateVideoTaskCreateRequest(&request)
	assert.Empty(t, param)
	assert.Empty(t, message)

	request.Input.ReferenceImages = append(request.Input.ReferenceImages,
		dto.VideoTaskSource{URL: "https://example.com/d.png"},
		dto.VideoTaskSource{URL: "https://example.com/e.png"},
	)
	param, message = validateVideoTaskCreateRequest(&request)
	assert.Empty(t, param)
	assert.Empty(t, message)

	request.Input.ReferenceImages = append(request.Input.ReferenceImages,
		dto.VideoTaskSource{URL: "https://example.com/f.png"},
	)
	param, message = validateVideoTaskCreateRequest(&request)
	assert.Equal(t, "input.reference_images", param)
	assert.Contains(t, message, "at most 5")

	request.Input.ReferenceImages = request.Input.ReferenceImages[:3]
	request.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "https://example.com/a.mp3"}}
	param, message = validateVideoTaskCreateRequest(&request)
	assert.Equal(t, "input.reference_mode", param)
	assert.Contains(t, message, "image references only")
}

func TestVideoTaskRequestSnapshotHashesReferenceURLs(t *testing.T) {
	generateAudio := false
	request := dto.VideoTaskCreateRequest{
		Model: "seedance-2.0-720p", Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt: "generate", ReferenceMode: "media",
			ReferenceVideos: []dto.VideoTaskSource{{
				URL: "https://media.example.com/clip.mp4?signature=secret", Name: "clip",
			}},
		},
		Output: dto.VideoTaskOutputRequest{GenerateAudio: &generateAudio},
	}
	snapshot, err := videoTaskRequestSnapshot(request)
	require.NoError(t, err)
	assert.NotContains(t, string(snapshot), "signature")
	assert.NotContains(t, string(snapshot), "secret")
	assert.Contains(t, string(snapshot), `"url":"sha256:`)
	assert.Contains(t, string(snapshot), `"generate_audio":false`)
	assert.Equal(t, "https://media.example.com/clip.mp4?signature=secret", request.Input.ReferenceVideos[0].URL)
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
