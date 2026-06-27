package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAssetCreateInputsMultipleImages(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_images",
		Status:    model.TaskStatusSuccess,
		Action:    constant.TaskActionImageGeneration,
		Platform:  constant.TaskPlatform("58"),
		ChannelId: 1,
		UserId:    1,
		Properties: model.Properties{
			OriginModelName: "gpt-image-2",
		},
	}
	task.SetData(map[string]any{
		"result": map[string]any{
			"images": []map[string]any{
				{"url": "https://cdn.example.com/1.webp"},
				{"url": "https://cdn.example.com/2.webp"},
			},
		},
	})

	inputs := BuildAssetCreateInputs(task)

	require.Len(t, inputs, 2)
	assert.Equal(t, 0, inputs[0].AssetIndex)
	assert.Equal(t, 1, inputs[1].AssetIndex)
	assert.EqualValues(t, model.AssetTypeImage, inputs[0].AssetType)
	assert.Equal(t, "https://cdn.example.com/1.webp", inputs[0].URL)
	assert.Equal(t, "https://cdn.example.com/2.webp", inputs[1].URL)
}

func TestBuildAssetCreateInputsKeepsImageHandleMetadata(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_image_metadata",
		Status:    model.TaskStatusSuccess,
		Action:    constant.TaskActionImageGeneration,
		Platform:  constant.TaskPlatform("58"),
		ChannelId: 1,
		UserId:    1,
		Properties: model.Properties{
			OriginModelName: "gpt-image-2",
		},
	}
	task.SetData(map[string]any{
		"result": map[string]any{
			"images": []map[string]any{{
				"url":            "https://cdn.example.com/1.png",
				"mime_type":      "image/png",
				"format":         "png",
				"filename":       "1.png",
				"size_bytes":     123456,
				"width":          1024,
				"height":         768,
				"revised_prompt": "revised",
			}},
			"output": map[string]any{
				"quality":       "high",
				"output_format": "png",
				"size":          "1024x768",
			},
			"metadata": map[string]any{
				"image_count":       1,
				"input_image_count": 1,
				"mask_used":         true,
			},
		},
	})

	inputs := BuildAssetCreateInputs(task)

	require.Len(t, inputs, 1)
	assert.Equal(t, "https://cdn.example.com/1.png", inputs[0].URL)
	assert.Equal(t, "image/png", inputs[0].MimeType)
	assert.Equal(t, "1.png", inputs[0].Filename)
	assert.EqualValues(t, 123456, inputs[0].SizeBytes)
	assert.Equal(t, 1024, inputs[0].Width)
	assert.Equal(t, 768, inputs[0].Height)
	assert.Equal(t, "png", inputs[0].Metadata["format"])
	assert.Equal(t, "revised", inputs[0].Metadata["revised_prompt"])
	assert.Equal(t, map[string]any{"quality": "high", "output_format": "png", "size": "1024x768"}, inputs[0].Metadata["output"])
	assert.Equal(t, map[string]any{"image_count": float64(1), "input_image_count": float64(1), "mask_used": true}, inputs[0].Metadata["execution"])
}

func TestApplyTaskResultCreatesAssetsOnce(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Asset{}))
	task := &model.Task{
		TaskID:     "task_asset_once",
		UserId:     1,
		ChannelId:  1,
		Group:      "default",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "gpt-image-2",
		},
	}
	task.SetData(map[string]any{
		"result": map[string]any{
			"images": []map[string]any{
				{"url": "https://cdn.example.com/a.webp"},
			},
		},
	})
	require.NoError(t, model.DB.Create(task).Error)

	taskResult := &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess,
		Url:    "https://cdn.example.com/a.webp",
	}
	updated, _ := ApplyTaskResult(context.Background(), &mockAdaptor{}, task, taskResult)
	require.True(t, updated)

	var count int64
	require.NoError(t, model.DB.Model(&model.Asset{}).Where("task_id = ?", "task_asset_once").Count(&count).Error)
	assert.EqualValues(t, 1, count)

	var saved model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_asset_once").First(&saved).Error)
	updated, _ = ApplyTaskResult(context.Background(), &mockAdaptor{}, &saved, taskResult)
	assert.False(t, updated)
	require.NoError(t, model.DB.Model(&model.Asset{}).Where("task_id = ?", "task_asset_once").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestApplyTaskResultFailureDoesNotCreateAssets(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Asset{}))
	task := &model.Task{
		TaskID:    "task_asset_fail",
		UserId:    1,
		ChannelId: 1,
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		Status:    model.TaskStatusQueued,
		Progress:  "20%",
	}
	require.NoError(t, model.DB.Create(task).Error)

	ApplyTaskResult(context.Background(), &mockAdaptor{}, task, &relaycommon.TaskInfo{
		Status: model.TaskStatusFailure,
		Reason: "failed",
	})

	var count int64
	require.NoError(t, model.DB.Model(&model.Asset{}).Where("task_id = ?", "task_asset_fail").Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestApplyTaskResultRollsBackSuccessWhenAssetInsertFails(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Asset{}))
	task := &model.Task{
		TaskID:    "task_asset_rollback",
		UserId:    1,
		ChannelId: 1,
		Group:     "default",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		Status:    model.TaskStatusQueued,
		Progress:  "20%",
		Properties: model.Properties{
			OriginModelName: "gpt-image-2",
		},
	}
	task.SetData(map[string]any{
		"result": map[string]any{
			"images": []map[string]any{
				{"url": "https://cdn.example.com/rollback.webp"},
			},
		},
	})
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, model.DB.Migrator().DropTable(&model.Asset{}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.AutoMigrate(&model.Asset{}))
	})

	updated, billed := ApplyTaskResult(context.Background(), &mockAdaptor{}, task, &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess,
		Url:    "https://cdn.example.com/rollback.webp",
	})

	assert.False(t, updated)
	assert.False(t, billed)
	var saved model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_asset_rollback").First(&saved).Error)
	assert.EqualValues(t, model.TaskStatusQueued, saved.Status)
	assert.Empty(t, saved.PrivateData.ResultURL)
}
