package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

type taskAssetImage struct {
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
	MimeType     string `json:"mime_type"`
	Filename     string `json:"filename"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type taskAssetVideo struct {
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
	PreviewURL   string `json:"preview_url"`
	MimeType     string `json:"mime_type"`
	Filename     string `json:"filename"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	DurationMS   int64  `json:"duration_ms"`
}

type taskAssetData struct {
	Result struct {
		Images []taskAssetImage `json:"images"`
		Videos []taskAssetVideo `json:"videos"`
		URL    string           `json:"url"`
	} `json:"result"`
	Images []taskAssetImage `json:"images"`
	Videos []taskAssetVideo `json:"videos"`
	URL    string           `json:"url"`
}

func CreateAssetsFromTask(ctx context.Context, task *model.Task) error {
	inputs := BuildAssetCreateInputs(task)
	if len(inputs) == 0 {
		return nil
	}
	if err := model.CreateAssetsForTask(inputs); err != nil {
		logger.LogError(ctx, fmt.Sprintf("create assets for task %s failed: %s", task.TaskID, err.Error()))
		return err
	}
	return nil
}

func BuildAssetCreateInputs(task *model.Task) []model.AssetCreateInput {
	if task == nil || task.Status != model.TaskStatusSuccess {
		return nil
	}
	assetType := assetTypeForTaskAction(task.Action)
	if assetType == "" {
		return nil
	}
	inputs := make([]model.AssetCreateInput, 0)
	seen := make(map[string]bool)
	appendURL := func(url string, thumbnailURL string, mimeType string, filename string, width int, height int, durationMS int64, metadata model.AssetMetadata) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		if metadata == nil {
			metadata = model.AssetMetadata{}
		}
		inputs = append(inputs, model.AssetCreateInput{
			Task:         task,
			AssetIndex:   len(inputs),
			AssetType:    assetType,
			URL:          url,
			ThumbnailURL: strings.TrimSpace(thumbnailURL),
			MimeType:     strings.TrimSpace(mimeType),
			Filename:     strings.TrimSpace(filename),
			Width:        width,
			Height:       height,
			DurationMS:   durationMS,
			Metadata:     metadata,
		})
	}

	data := taskAssetData{}
	if len(task.Data) != 0 {
		_ = common.Unmarshal(task.Data, &data)
	}

	if assetType == model.AssetTypeImage {
		for _, image := range data.Result.Images {
			appendURL(image.URL, image.ThumbnailURL, image.MimeType, image.Filename, image.Width, image.Height, 0, model.AssetMetadata{"source": "data.result.images"})
		}
		for _, image := range data.Images {
			appendURL(image.URL, image.ThumbnailURL, image.MimeType, image.Filename, image.Width, image.Height, 0, model.AssetMetadata{"source": "data.images"})
		}
	}

	if assetType == model.AssetTypeVideo {
		for _, video := range data.Result.Videos {
			thumbnailURL := video.ThumbnailURL
			if thumbnailURL == "" {
				thumbnailURL = video.PreviewURL
			}
			appendURL(video.URL, thumbnailURL, video.MimeType, video.Filename, video.Width, video.Height, video.DurationMS, model.AssetMetadata{"source": "data.result.videos"})
		}
		for _, video := range data.Videos {
			thumbnailURL := video.ThumbnailURL
			if thumbnailURL == "" {
				thumbnailURL = video.PreviewURL
			}
			appendURL(video.URL, thumbnailURL, video.MimeType, video.Filename, video.Width, video.Height, video.DurationMS, model.AssetMetadata{"source": "data.videos"})
		}
	}

	appendURL(data.Result.URL, "", "", "", 0, 0, 0, model.AssetMetadata{"source": "data.result.url"})
	appendURL(data.URL, "", "", "", 0, 0, 0, model.AssetMetadata{"source": "data.url"})
	appendURL(task.GetResultURL(), "", "", "", 0, 0, 0, model.AssetMetadata{"source": "private_data.result_url"})

	return inputs
}

func assetTypeForTaskAction(action string) model.AssetType {
	switch constant.TaskActionAssetType(action) {
	case constant.TaskAssetTypeImage:
		return model.AssetTypeImage
	case constant.TaskAssetTypeVideo:
		return model.AssetTypeVideo
	case constant.TaskAssetTypeAudio:
		return model.AssetTypeAudio
	default:
		return ""
	}
}
