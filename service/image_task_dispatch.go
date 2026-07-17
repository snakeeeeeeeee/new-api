package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

var imageTaskDispatchWorkerOnce sync.Once

func StartImageTaskDispatchWorker() {
	if !common.IsMasterNode {
		return
	}
	imageTaskDispatchWorkerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				processDueImageTaskDispatches(context.Background())
				<-ticker.C
			}
		}()
	})
}

func processDueImageTaskDispatches(ctx context.Context) {
	dispatches, err := model.ClaimDueImageTaskDispatches(20, 60)
	if err != nil {
		logger.LogError(ctx, "claim image task dispatches failed: "+err.Error())
		return
	}
	for _, dispatch := range dispatches {
		processImageTaskDispatch(ctx, dispatch)
	}
}

func processImageTaskDispatch(ctx context.Context, dispatch *model.ImageTaskDispatch) {
	if dispatch == nil {
		return
	}
	task, err := model.GetTaskByRecordID(dispatch.TaskRecordID)
	if err != nil {
		_ = model.MarkImageTaskDispatchFailed(dispatch.ID, dispatch.LockToken, 0, err.Error())
		return
	}
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		_ = model.MarkImageTaskDispatchDelivered(dispatch.ID, dispatch.LockToken, 0)
		return
	}
	configErr := ValidateImageHandleSubmitConfig()
	if configErr != nil {
		rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, 0, configErr.Error(), true)
		return
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(GetImageHandleSubmitBaseURL(), "/")+"/v1/image/tasks", bytes.NewBufferString(dispatch.RequestBody))
	if err != nil {
		rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, 0, err.Error(), false)
		return
	}
	request.Header.Set("Authorization", "Bearer "+GetImageHandleSubmitAPIKey())
	request.Header.Set("Content-Type", "application/json")
	response, err := GetHttpClient().Do(request)
	if err != nil {
		rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, 0, err.Error(), true)
		return
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if readErr != nil {
		rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, response.StatusCode, readErr.Error(), true)
		return
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		var submit struct {
			ProviderTaskID string `json:"provider_task_id"`
			ClientTaskID   string `json:"client_task_id"`
		}
		if err := common.Unmarshal(body, &submit); err != nil || strings.TrimSpace(submit.ProviderTaskID) == "" {
			rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, response.StatusCode, "image-handle returned an invalid submit response", true)
			return
		}
		if submit.ClientTaskID != "" && submit.ClientTaskID != task.TaskID {
			failImageTaskDispatch(ctx, dispatch, task, response.StatusCode, "image-handle returned a mismatched client_task_id")
			return
		}
		if err := model.PersistTaskSubmitResult(task.ID, submit.ProviderTaskID, body, 0); err != nil {
			rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, response.StatusCode, err.Error(), true)
			return
		}
		_ = model.MarkImageTaskDispatchDelivered(dispatch.ID, dispatch.LockToken, response.StatusCode)
		return
	}
	reason := fmt.Sprintf("image-handle submit failed with status %d", response.StatusCode)
	if len(body) > 0 {
		var errorBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if common.Unmarshal(body, &errorBody) == nil && strings.TrimSpace(errorBody.Error.Message) != "" {
			reason = errorBody.Error.Message
		}
	}
	retryable := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
	rescheduleOrFailImageTaskDispatch(ctx, dispatch, task, response.StatusCode, reason, retryable)
}

func rescheduleOrFailImageTaskDispatch(ctx context.Context, dispatch *model.ImageTaskDispatch, task *model.Task, status int, reason string, retryable bool) {
	delays := []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute}
	if retryable && dispatch.Attempts <= len(delays) && time.Now().Unix()-dispatch.CreatedAt < int64(time.Hour/time.Second) {
		delay := delays[dispatch.Attempts-1]
		if err := model.RescheduleImageTaskDispatch(dispatch.ID, dispatch.LockToken, status, reason, delay); err != nil {
			logger.LogError(ctx, "reschedule image task dispatch failed: "+err.Error())
		}
		return
	}
	failImageTaskDispatch(ctx, dispatch, task, status, reason)
}

func failImageTaskDispatch(ctx context.Context, dispatch *model.ImageTaskDispatch, task *model.Task, status int, reason string) {
	ApplyTaskResult(ctx, nil, task, relaycommon.FailTaskInfo(reason))
	if task.Status != model.TaskStatusFailure {
		if err := model.RescheduleImageTaskDispatch(dispatch.ID, dispatch.LockToken, status, reason, 30*time.Second); err != nil {
			logger.LogError(ctx, "reschedule image task dispatch after terminal transition failure: "+err.Error())
		}
		return
	}
	if err := model.MarkImageTaskDispatchFailed(dispatch.ID, dispatch.LockToken, status, reason); err != nil {
		logger.LogError(ctx, "mark image task dispatch failed: "+err.Error())
	}
}
