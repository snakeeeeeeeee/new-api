package service

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

const (
	VideoURLAuthNone           = "none"
	VideoURLAuthResourceAPIKey = "resource_api_key"
)

func BuildPublicVideoTasks(tasks []*model.Task) ([]*dto.VideoTaskPublic, error) {
	if len(tasks) == 0 {
		return []*dto.VideoTaskPublic{}, nil
	}
	userID := tasks[0].UserId
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			taskIDs = append(taskIDs, task.TaskID)
		}
	}
	requests, err := model.GetVideoTaskRequestsByTaskIDs(userID, taskIDs)
	if err != nil {
		return nil, err
	}
	assets, err := model.GetUserAssetsByTaskIDs(userID, taskIDs)
	if err != nil {
		return nil, err
	}
	requestByTask := make(map[string]*model.VideoTaskRequest, len(requests))
	for _, request := range requests {
		requestByTask[request.TaskID] = request
	}
	assetsByTask := make(map[string][]*model.Asset)
	for _, asset := range assets {
		assetsByTask[asset.TaskID] = append(assetsByTask[asset.TaskID], asset)
	}
	result := make([]*dto.VideoTaskPublic, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			result = append(result, buildPublicVideoTask(task, requestByTask[task.TaskID], assetsByTask[task.TaskID]))
		}
	}
	return result, nil
}

func BuildPublicVideoTask(task *model.Task) (*dto.VideoTaskPublic, error) {
	items, err := BuildPublicVideoTasks([]*model.Task{task})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func BuildPublicVideoTaskFromRequest(task *model.Task, request *dto.VideoTaskCreateRequest) *dto.VideoTaskPublic {
	public := buildPublicVideoTask(task, nil, nil)
	if request != nil {
		public.Model = strings.TrimSpace(request.Model)
		public.Operation = request.Operation
		public.ClientReferenceID = request.ClientReferenceID
		public.Metadata = request.Metadata
	}
	return public
}

func BuildPublicVideoTaskTx(tx *gorm.DB, task *model.Task) (*dto.VideoTaskPublic, bool, error) {
	if tx == nil || task == nil {
		return nil, false, nil
	}
	var request model.VideoTaskRequest
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
	return buildPublicVideoTask(task, &request, assets), true, nil
}

func buildPublicVideoTask(task *model.Task, requestRecord *model.VideoTaskRequest, assets []*model.Asset) *dto.VideoTaskPublic {
	public := &dto.VideoTaskPublic{
		ID: task.TaskID, Object: "video.task",
		Model:     firstNonEmptyString(task.Properties.OriginModelName, task.Properties.UpstreamModelName),
		Operation: publicVideoOperation(task), Status: PublicVideoTaskStatus(task.Status),
		Progress:       publicVideoProgress(task),
		ProgressKnown:  task.PrivateData.ProgressMetadataSet && task.PrivateData.ProgressKnown,
		ProgressSource: publicTaskProgressSource(task),
		Stage:          publicVideoProgressStage(task),
		CreatedAt:      task.CreatedAt, UpdatedAt: task.UpdatedAt,
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
		var request dto.VideoTaskCreateRequest
		if common.UnmarshalJsonStr(requestRecord.RequestJSON, &request) == nil {
			public.Metadata = request.Metadata
			public.Model = firstNonEmptyString(request.Model, public.Model)
			if request.Operation != "" {
				public.Operation = request.Operation
			}
		}
	}
	if len(assets) > 0 {
		videos := make([]dto.VideoTaskResultVideo, 0, len(assets))
		for _, asset := range assets {
			if asset.AssetType != model.AssetTypeVideo {
				continue
			}
			publicURL, urlAuth := PublicVideoAssetURL(asset)
			videos = append(videos, dto.VideoTaskResultVideo{
				AssetID: asset.AssetID, Index: asset.AssetIndex, URL: publicURL,
				MimeType: asset.MimeType, Filename: asset.Filename, Width: asset.Width,
				Height: asset.Height, DurationMS: asset.DurationMS, Temporary: true, URLAuth: urlAuth,
			})
		}
		if len(videos) > 0 {
			public.Result = &dto.VideoTaskResult{Videos: videos}
		}
	}
	if public.Status == "failed" {
		public.Error = buildPublicVideoTaskError(task)
	}
	return public
}

func publicVideoProgressStage(task *model.Task) string {
	if task != nil {
		if stage := strings.TrimSpace(task.PrivateData.ProgressStage); stage != "" {
			return stage
		}
		return PublicVideoTaskStatus(task.Status)
	}
	return "queued"
}

var legacyVideoHTTPStatusPattern = regexp.MustCompile(`(?i)(?:poll|submit)[^:\n]{0,40}failed:\s*([1-5][0-9]{2})\b`)

type videoTaskErrorDiagnostic struct {
	Code           string
	UpstreamStatus int
}

func buildPublicVideoTaskError(task *model.Task) *dto.VideoTaskPublicError {
	diagnostic := extractVideoTaskErrorDiagnostic(task)
	code := publicVideoTaskErrorCode(diagnostic)
	publicError := &dto.VideoTaskPublicError{
		Code:              code,
		Message:           publicVideoTaskErrorMessage(task, code),
		Retryable:         publicVideoTaskErrorRetryable(task, code),
		UpstreamStatus:    diagnostic.UpstreamStatus,
		UpstreamErrorCode: publicVideoUpstreamErrorCode(diagnostic.Code),
	}
	if task != nil && task.PrivateData.BillingContext != nil {
		publicError.RequestID = strings.TrimSpace(task.PrivateData.BillingContext.RequestId)
	}
	return publicError
}

func extractVideoTaskErrorDiagnostic(task *model.Task) videoTaskErrorDiagnostic {
	if task == nil {
		return videoTaskErrorDiagnostic{}
	}
	var response struct {
		Code           string `json:"code"`
		ErrorCode      string `json:"error_code"`
		StatusCode     int    `json:"status_code"`
		UpstreamStatus int    `json:"upstream_status"`
		Error          *struct {
			Code           string `json:"code"`
			ErrorCode      string `json:"error_code"`
			UpstreamStatus int    `json:"upstream_status"`
		} `json:"error"`
	}
	if len(task.Data) > 0 {
		_ = common.Unmarshal(task.Data, &response)
	}
	diagnostic := videoTaskErrorDiagnostic{
		Code:           firstNonEmptyString(response.ErrorCode, response.Code),
		UpstreamStatus: task.PrivateData.LastUpstreamStatus,
	}
	if response.UpstreamStatus > 0 {
		diagnostic.UpstreamStatus = response.UpstreamStatus
	} else if diagnostic.UpstreamStatus == 0 {
		diagnostic.UpstreamStatus = response.StatusCode
	}
	if response.Error != nil {
		diagnostic.Code = firstNonEmptyString(response.Error.Code, response.Error.ErrorCode, diagnostic.Code)
		if response.Error.UpstreamStatus > 0 {
			diagnostic.UpstreamStatus = response.Error.UpstreamStatus
		}
	}
	if diagnostic.UpstreamStatus == 0 {
		matches := legacyVideoHTTPStatusPattern.FindStringSubmatch(task.FailReason)
		if len(matches) == 2 {
			diagnostic.UpstreamStatus, _ = strconv.Atoi(matches[1])
		}
	}
	return diagnostic
}

func publicVideoTaskErrorCode(diagnostic videoTaskErrorDiagnostic) string {
	rawCode := strings.ToLower(strings.TrimSpace(diagnostic.Code))
	switch rawCode {
	case "content_moderated":
		return "content_moderated"
	case "invalid_reference_media_duration":
		return "invalid_reference_media_duration"
	case "reference_media_duration_exceeded":
		return "reference_media_duration_exceeded"
	case "cancelled_by_admin", "cancelled":
		return "cancelled"
	case "upstream_status_unavailable", "upstream_unavailable":
		return "upstream_unavailable"
	case "retry_exhausted", "model_overload":
		return "upstream_unavailable"
	case "timeout", "timeout_error", "request_timeout", "upstream_timeout":
		return "upstream_timeout"
	case "rate_limit", "rate_limited", "upstream_rate_limited":
		return "upstream_rate_limited"
	case "upstream_authentication_error", "authentication_error", "auth_invalid":
		return "upstream_authentication_error"
	case "video_task_failed":
		// Keep evaluating the HTTP status below so legacy generic errors can
		// still be projected to a more useful provider-neutral code.
	}
	switch diagnostic.UpstreamStatus {
	case 401, 403:
		return "upstream_authentication_error"
	case 408:
		return "upstream_timeout"
	case 429:
		return "upstream_rate_limited"
	}
	if diagnostic.UpstreamStatus >= 500 {
		return "upstream_unavailable"
	}
	return "video_task_failed"
}

func publicVideoTaskErrorMessage(task *model.Task, code string) string {
	reason := ""
	if task != nil {
		reason = strings.TrimSpace(task.FailReason)
	}
	switch reason {
	case "Adobe submission result is unknown":
		return "Submission result could not be confirmed"
	case "Adobe submission connection ended after it may have been accepted":
		return "Submission connection ended before the result was confirmed"
	case "Adobe submission timed out after it may have been accepted":
		return "Submission timed out before the result was confirmed"
	case "AdobeVideo task failed":
		return "Video task failed"
	case "AdobeVideo completed without a task reference":
		return "Video task completed without a task reference"
	}
	switch code {
	case "content_moderated", "invalid_reference_media_duration", "reference_media_duration_exceeded":
		if reason != "" {
			return reason
		}
	case "cancelled":
		return "Video task was cancelled"
	case "upstream_timeout":
		return "Generation status was temporarily unavailable for too long"
	case "upstream_rate_limited":
		return "Generation service remained rate limited for too long"
	case "upstream_unavailable":
		return "Generation service remained unavailable for too long"
	case "upstream_authentication_error":
		return "Generation service authentication failed"
	}
	if strings.Contains(strings.ToLower(reason), "submission") &&
		(strings.Contains(strings.ToLower(reason), "unknown") || strings.Contains(strings.ToLower(reason), "confirm")) {
		return "Submission result could not be confirmed"
	}
	if safePublicVideoFailureReason(reason) {
		return reason
	}
	return "Video task failed"
}

func safePublicVideoFailureReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 240 {
		return false
	}
	lower := strings.ToLower(reason)
	for _, marker := range []string{
		"adobe", "provider", "fal-ai", "fal_ai", "seedance", "kling", "veo",
		"leonardo", "higgsfield", "gemini", "google", "xai", "http://", "https://", "www.",
		"authorization", "cookie", "access_token", "refresh_token",
		"traceback", "stack trace", "{\"", "{'",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func publicVideoTaskErrorRetryable(task *model.Task, code string) bool {
	switch code {
	case "content_moderated":
		return true
	case "upstream_rate_limited", "upstream_unavailable":
		if task != nil {
			return strings.Contains(strings.ToLower(task.FailReason), "submit")
		}
	}
	return false
}

func publicVideoUpstreamErrorCode(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "timeout", "timeout_error", "request_timeout",
		"rate_limit", "rate_limited", "model_overload",
		"upstream_status_unavailable", "retry_exhausted",
		"submission_unknown", "content_moderated",
		"invalid_reference_media_duration", "reference_media_duration_exceeded",
		"cancelled_by_admin", "upstream_authentication_error", "authentication_error", "auth_invalid":
		return strings.ToLower(strings.TrimSpace(code))
	default:
		return ""
	}
}

func PublicVideoTaskStatus(status model.TaskStatus) string {
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

func publicVideoProgress(task *model.Task) int {
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

func publicVideoOperation(task *model.Task) string {
	if operation := strings.TrimSpace(task.Properties.Operation); operation != "" {
		return operation
	}
	switch task.Action {
	case constant.TaskActionVideoEdit:
		return "edit"
	case constant.TaskActionVideoExtension:
		return "extension"
	case constant.TaskActionRemix:
		return "remix"
	default:
		return "generation"
	}
}

func PublicVideoAssetURL(asset *model.Asset) (string, string) {
	if asset != nil && isPublicCrossOriginVideoURL(asset) {
		return strings.TrimSpace(asset.URL), VideoURLAuthNone
	}
	assetID := ""
	if asset != nil {
		assetID = asset.AssetID
	}
	path := "/v1/assets/" + assetID + "/content"
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		return path, VideoURLAuthResourceAPIKey
	}
	return base + path, VideoURLAuthResourceAPIKey
}

func isPublicCrossOriginVideoURL(asset *model.Asset) bool {
	if asset == nil || videoAssetRequiresProxy(asset) {
		return false
	}
	target, err := url.Parse(strings.TrimSpace(asset.URL))
	if err != nil || target.User != nil || !target.IsAbs() || !strings.EqualFold(target.Scheme, "https") || target.Hostname() == "" {
		return false
	}
	for key := range target.Query() {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "key", "api_key", "apikey", "access_token", "authorization":
			return false
		}
	}
	channel, err := model.CacheGetChannel(asset.ChannelID)
	if err != nil || channel == nil {
		return false
	}
	switch channel.Type {
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi, constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		return false
	}
	base, err := url.Parse(strings.TrimSpace(channel.GetBaseURL()))
	if err != nil || base.Hostname() == "" {
		return false
	}
	return !samePublicVideoOrigin(base, target)
}

func videoAssetRequiresProxy(asset *model.Asset) bool {
	if asset == nil || len(asset.Metadata) == 0 {
		return false
	}
	for _, key := range []string{"resolver", "provider_reference"} {
		if value, ok := asset.Metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func samePublicVideoOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		publicVideoPort(left) == publicVideoPort(right)
}

func publicVideoPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	if strings.EqualFold(value.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(value.Scheme, "http") {
		return "80"
	}
	return ""
}
