package service

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	publicImageTaskUpstreamQuotaCode    = "524"
	publicImageTaskUpstreamQuotaMessage = "Image generation service is temporarily unavailable. Please try again later."
)

type imageTaskCallbackError struct {
	Code                 string `json:"code"`
	Message              string `json:"message"`
	Retryable            bool   `json:"retryable"`
	UpstreamStatus       int    `json:"upstream_status,omitempty"`
	ProviderErrorCode    string `json:"provider_error_code,omitempty"`
	ProviderErrorType    string `json:"provider_error_type,omitempty"`
	ProviderErrorMessage string `json:"provider_error_message,omitempty"`
}

type imageTaskCallbackData struct {
	Result *struct {
		Output map[string]any `json:"output"`
	} `json:"result"`
	Usage map[string]any          `json:"usage"`
	Error *imageTaskCallbackError `json:"error"`
}

func BuildPublicImageTasks(tasks []*model.Task) ([]*dto.ImageTaskPublic, error) {
	if len(tasks) == 0 {
		return []*dto.ImageTaskPublic{}, nil
	}
	userID := tasks[0].UserId
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			taskIDs = append(taskIDs, task.TaskID)
		}
	}
	requests, err := model.GetImageTaskRequestsByTaskIDs(userID, taskIDs)
	if err != nil {
		return nil, err
	}
	assets, err := model.GetUserAssetsByTaskIDs(userID, taskIDs)
	if err != nil {
		return nil, err
	}
	requestByTask := make(map[string]*model.ImageTaskRequest, len(requests))
	for _, request := range requests {
		requestByTask[request.TaskID] = request
	}
	assetsByTask := make(map[string][]*model.Asset)
	for _, asset := range assets {
		assetsByTask[asset.TaskID] = append(assetsByTask[asset.TaskID], asset)
	}
	result := make([]*dto.ImageTaskPublic, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		result = append(result, buildPublicImageTask(task, requestByTask[task.TaskID], assetsByTask[task.TaskID]))
	}
	return result, nil
}

func BuildPublicImageTask(task *model.Task) (*dto.ImageTaskPublic, error) {
	items, err := BuildPublicImageTasks([]*model.Task{task})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

// BuildPublicImageTaskFromRequest builds the initial queued response from the
// same normalized request that was committed with the task. It deliberately
// avoids a post-commit database read that could turn an accepted task into an
// apparent submit failure.
func BuildPublicImageTaskFromRequest(task *model.Task, request *dto.ImageTaskCreateRequest) *dto.ImageTaskPublic {
	public := buildPublicImageTask(task, nil, nil)
	if request == nil {
		return public
	}
	public.Model = strings.TrimSpace(request.Model)
	public.Operation = request.Operation
	public.ClientReferenceID = request.ClientReferenceID
	public.Metadata = request.Metadata
	return public
}

func BuildPublicImageTaskTx(tx *gorm.DB, task *model.Task) (*dto.ImageTaskPublic, bool, error) {
	if tx == nil || task == nil {
		return nil, false, nil
	}
	var request model.ImageTaskRequest
	if err := tx.Where("user_id = ? AND task_id = ?", task.UserId, task.TaskID).First(&request).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	var assets []*model.Asset
	if err := tx.Where("user_id = ? AND task_id = ? AND status = ? AND deleted_at = ?", task.UserId, task.TaskID, model.AssetStatusAvailable, 0).
		Order("asset_index ASC").Find(&assets).Error; err != nil {
		return nil, false, err
	}
	return buildPublicImageTask(task, &request, assets), true, nil
}

func buildPublicImageTask(task *model.Task, requestRecord *model.ImageTaskRequest, assets []*model.Asset) *dto.ImageTaskPublic {
	public := &dto.ImageTaskPublic{
		ID:             task.TaskID,
		Object:         "image.task",
		Model:          firstNonEmptyString(task.Properties.OriginModelName, task.Properties.UpstreamModelName),
		Operation:      publicImageOperation(task.Action),
		Status:         PublicImageTaskStatus(task.Status),
		Progress:       publicImageProgress(task),
		ProgressKnown:  task.PrivateData.ProgressMetadataSet && task.PrivateData.ProgressKnown,
		ProgressSource: publicTaskProgressSource(task),
		Stage:          publicImageProgressStage(task),
		Result:         nil,
		Usage:          nil,
		Error:          nil,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
	if public.CreatedAt == 0 {
		public.CreatedAt = task.SubmitTime
	}
	if public.UpdatedAt == 0 {
		public.UpdatedAt = public.CreatedAt
	}
	if task.StartTime > 0 {
		value := task.StartTime
		public.StartedAt = &value
	}
	if task.FinishTime > 0 {
		value := task.FinishTime
		public.CompletedAt = &value
	}
	if requestRecord != nil {
		public.ClientReferenceID = requestRecord.ClientReferenceID
		var request dto.ImageTaskCreateRequest
		if common.UnmarshalJsonStr(requestRecord.RequestJSON, &request) == nil {
			public.Metadata = request.Metadata
			if strings.TrimSpace(request.Model) != "" {
				public.Model = strings.TrimSpace(request.Model)
			}
			if request.Operation != "" {
				public.Operation = request.Operation
			}
		}
	}
	var callback imageTaskCallbackData
	if len(task.Data) > 0 {
		_ = common.Unmarshal(task.Data, &callback)
	}
	if len(callback.Usage) > 0 {
		delete(callback.Usage, "actual_quota")
		public.Usage = callback.Usage
	}
	if len(assets) > 0 {
		images := make([]dto.ImageTaskResultImage, 0, len(assets))
		for _, asset := range assets {
			if asset.AssetType != model.AssetTypeImage {
				continue
			}
			images = append(images, dto.ImageTaskResultImage{
				AssetID:       asset.AssetID,
				URL:           asset.URL,
				MimeType:      asset.MimeType,
				Format:        metadataStringValue(asset.Metadata, "format"),
				Width:         asset.Width,
				Height:        asset.Height,
				SizeBytes:     asset.SizeBytes,
				Filename:      asset.Filename,
				RevisedPrompt: metadataStringValue(asset.Metadata, "revised_prompt"),
			})
		}
		public.Result = &dto.ImageTaskResult{Images: images}
		if callback.Result != nil {
			public.Result.Output = callback.Result.Output
		}
	}
	if public.Status == "failed" {
		public.Error = buildPublicImageTaskError(callback.Error, task.FailReason)
	}
	return public
}

func publicTaskProgressSource(task *model.Task) string {
	if task != nil {
		if source := strings.TrimSpace(task.PrivateData.ProgressSource); source != "" {
			return source
		}
	}
	return "local_status"
}

func publicImageProgressStage(task *model.Task) string {
	if task != nil {
		if stage := strings.TrimSpace(task.PrivateData.ProgressStage); stage != "" {
			return stage
		}
		return PublicImageTaskStatus(task.Status)
	}
	return "queued"
}

func buildPublicImageTaskError(callbackError *imageTaskCallbackError, failReason string) *dto.ImageTaskPublicError {
	if isInternalUpstreamQuotaError(callbackError, failReason) {
		return &dto.ImageTaskPublicError{
			Code:      publicImageTaskUpstreamQuotaCode,
			Message:   publicImageTaskUpstreamQuotaMessage,
			Retryable: true,
		}
	}

	publicError := &dto.ImageTaskPublicError{Code: "image_task_failed", Message: failReason}
	if callbackError != nil {
		publicError.Code = firstNonEmptyString(callbackError.Code, publicError.Code)
		publicError.Message = firstNonEmptyString(callbackError.Message, publicError.Message)
		publicError.Retryable = callbackError.Retryable
	}
	if strings.TrimSpace(publicError.Message) == "" {
		publicError.Message = "Image task failed"
	}
	return publicError
}

func isInternalUpstreamQuotaError(callbackError *imageTaskCallbackError, failReason string) bool {
	if callbackError != nil {
		for _, code := range []string{
			callbackError.Code,
			callbackError.ProviderErrorCode,
			callbackError.ProviderErrorType,
		} {
			switch strings.ToLower(strings.TrimSpace(code)) {
			case "insufficient_user_quota",
				"insufficient_quota",
				"pre_consume_token_quota_failed",
				"billing_not_active",
				"arrearage":
				return true
			}
		}
	}

	messages := []string{failReason}
	if callbackError != nil {
		messages = append(messages, callbackError.Message, callbackError.ProviderErrorMessage)
	}
	for _, message := range messages {
		normalized := strings.ToLower(strings.TrimSpace(message))
		if normalized == "" {
			continue
		}
		if strings.Contains(normalized, "本地用户") &&
			(strings.Contains(normalized, "额度") || strings.Contains(normalized, "预扣费")) {
			return true
		}
		for _, marker := range []string{
			"insufficient quota",
			"quota insufficient",
			"pre-consume token quota failed",
			"preconsume token quota failed",
			"billing not active",
			"arrearage",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func PublicImageTaskStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	case model.TaskStatusInProgress:
		return "in_progress"
	default:
		return "queued"
	}
}

func publicImageProgress(task *model.Task) int {
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		return 100
	}
	value := strings.TrimSpace(strings.TrimSuffix(task.Progress, "%"))
	progress, err := strconv.Atoi(value)
	if err != nil || progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func publicImageOperation(action string) string {
	if strings.Contains(strings.ToLower(action), "edit") {
		return "edit"
	}
	return "generation"
}

func metadataStringValue(metadata model.AssetMetadata, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
