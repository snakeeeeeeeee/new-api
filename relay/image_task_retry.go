package relay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AsyncImageAttemptFailureOutcome struct {
	Ignored     bool
	Retrying    bool
	Exhausted   bool
	NextAttempt *model.ImageTaskAttempt
}

type preparedAsyncImageAttempt struct {
	context *gin.Context
	attempt *model.ImageTaskAttempt
	lease   *model.ImageCredentialLease
	body    []byte
}

func buildAsyncImageRetryContext(parent *model.Task, state *model.ImageTaskRetryState, selection *service.AsyncImageRouteSelection, clientTaskID string, attemptNumber int) (*gin.Context, *relaycommon.RelayInfo, channel.TaskAdaptor, dto.ImageTaskCreateRequest, error) {
	var requestRecord model.ImageTaskRequest
	if err := model.DB.Where("task_record_id = ?", parent.ID).First(&requestRecord).Error; err != nil {
		return nil, nil, nil, dto.ImageTaskCreateRequest{}, err
	}
	var normalized dto.ImageTaskCreateRequest
	if err := common.UnmarshalJsonStr(requestRecord.RequestJSON, &normalized); err != nil {
		return nil, nil, nil, normalized, err
	}
	legacy := relaycommon.NormalizedImageTaskToLegacy(normalized)
	legacy.ClientTaskID = clientTaskID
	legacyBody, err := common.Marshal(legacy)
	if err != nil {
		return nil, nil, nil, normalized, err
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader(legacyBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("task_request", legacy)
	c.Set(relaycommon.ImageTaskPublicRequestContextKey, normalized)
	c.Set(relaycommon.ImageTaskPublicRequestJSONContextKey, []byte(requestRecord.RequestJSON))
	requestID := ""
	if parent.PrivateData.BillingContext != nil {
		requestID = strings.TrimSpace(parent.PrivateData.BillingContext.RequestId)
	}
	if requestID == "" {
		requestID = parent.TaskID
	}
	requestID = fmt.Sprintf("%s:attempt:%d", requestID, attemptNumber)
	common.SetContextKey(c, common.RequestIdKey, requestID)
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	common.SetContextKey(c, constant.ContextKeyUserId, parent.UserId)
	if user, userErr := model.GetUserCache(parent.UserId); userErr == nil && user != nil {
		user.WriteContext(c)
	}
	common.SetContextKey(c, constant.ContextKeyUserGroup, state.UserGroup)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, selection.RouteGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, state.TokenGroup)
	common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, state.CrossGroupRetry)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, state.OriginalModel)
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, state.AggregateGroup)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutingMode, state.RoutingMode)
	common.SetContextKey(c, constant.ContextKeyRouteGroup, selection.RouteGroup)
	common.SetContextKey(c, constant.ContextKeyRouteGroupIndex, selection.RouteIndex)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutePool, selection.RoutePool)
	if token, tokenErr := model.GetTokenById(parent.PrivateData.TokenId); tokenErr == nil && token != nil {
		if setupErr := middleware.SetupContextForToken(c, token); setupErr != nil {
			return nil, nil, nil, normalized, setupErr
		}
		common.SetContextKey(c, constant.ContextKeyTokenGroup, state.TokenGroup)
		common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, state.CrossGroupRetry)
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, selection.Channel, state.OriginalModel); setupErr != nil {
		return nil, nil, nil, normalized, setupErr
	}

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		return nil, nil, nil, normalized, err
	}
	info.InitChannelMeta(c)
	info.PublicTaskID = clientTaskID
	platform := constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeImageHandle))
	adaptor := GetTaskAdaptor(platform)
	if adaptor == nil {
		return nil, nil, nil, normalized, fmt.Errorf("image-handle task adaptor not found")
	}
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		return nil, nil, nil, normalized, taskErr.Error
	}
	info.OriginModelName = state.OriginalModel
	if err := prepareAsyncImageRetryPricing(c, info, adaptor); err != nil {
		return nil, nil, nil, normalized, err
	}
	return c, info, adaptor, normalized, nil
}

func prepareAsyncImageRetryPricing(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.TaskAdaptor) error {
	groupRatioInfo := helper.HandleGroupRatio(c, info)
	imagePricingSnapshot, imagePricingBound, err := helper.ResolveTaskImagePricing(c, info.OriginModelName, groupRatioInfo.GroupRatio)
	if err != nil {
		return err
	}
	info.UpstreamModelName = info.OriginModelName
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return err
	}
	if imageHandleSyncChannelType(info) == constant.ChannelTypeGemini {
		if !service.IsGeminiImageModel(info.OriginModelName) {
			return fmt.Errorf("unsupported Gemini image model: %s", info.OriginModelName)
		}
		if taskErr := validateAndNormalizeGeminiAsyncImageRequest(c, info.OriginModelName); taskErr != nil {
			return taskErr.Error
		}
	}
	if imagePricingBound {
		info.PriceData = helper.ImagePricingPriceData(imagePricingSnapshot, groupRatioInfo, true)
	} else {
		priceData, priceErr := helper.ModelPriceHelperPerCall(c, info)
		if priceErr != nil {
			return priceErr
		}
		info.PriceData = priceData
	}
	applyAsyncImageUsageBillingSnapshot(c, info)
	if estimatedRatios := adaptor.EstimateBilling(c, info); len(estimatedRatios) > 0 {
		for key, ratio := range estimatedRatios {
			info.PriceData.AddOtherRatio(key, ratio)
		}
	}
	if info.PriceData.ImagePricing == nil && !common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		quotaWithRatios := float64(info.PriceData.Quota)
		for _, ratio := range info.PriceData.OtherRatios {
			if ratio != 1 {
				quotaWithRatios *= ratio
			}
		}
		info.PriceData.Quota = common.QuotaFromFloat(quotaWithRatios)
	}
	applyAsyncImageUsagePrecharge(c, info)
	return nil
}

func prepareNextAsyncImageAttempt(parent *model.Task, state *model.ImageTaskRetryState, selection *service.AsyncImageRouteSelection) (*preparedAsyncImageAttempt, error) {
	attemptNumber := state.AttemptCount + 1
	clientTaskID := model.GenerateImageTaskAttemptClientID()
	c, info, adaptor, _, err := buildAsyncImageRetryContext(parent, state, selection, clientTaskID, attemptNumber)
	if err != nil {
		return nil, err
	}
	snapshotTask, operation, err := newAsyncImageTask(c, info, parent.Platform, info.PriceData.Quota)
	if err != nil {
		return nil, err
	}
	attempt := model.NewImageTaskAttempt(
		parent, attemptNumber, clientTaskID, selection.Channel.Id,
		selection.RouteGroup, selection.RouteIndex, selection.RoutePool,
		info.OriginModelName, info.UpstreamModelName, info.PriceData.Quota,
		snapshotTask.PrivateData.BillingContext, c.GetString(common.RequestIdKey),
	)
	attempt.ExecutionDriver = snapshotTask.PrivateData.ImageHandleExecutionDriver
	lease := model.NewImageCredentialLease(parent, operation, info.UpstreamModelName, imageCredentialLeaseTTL(info, parent))
	lease.ChannelID = attempt.ChannelID
	c.Set("image_credential_lease_id", lease.LeaseID)
	bodyReader, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, err
	}
	return &preparedAsyncImageAttempt{context: c, attempt: attempt, lease: lease, body: body}, nil
}

func cloneAsyncImageRetryState(state *model.ImageTaskRetryState) *model.ImageTaskRetryState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.FailedChannelIDs = append([]int(nil), state.FailedChannelIDs...)
	cloned.AttemptedRouteKeys = append([]string(nil), state.AttemptedRouteKeys...)
	cloned.RouteTrace = append([]model.ImageTaskRouteTraceEntry(nil), state.RouteTrace...)
	return &cloned
}

func applyPlannedAsyncImageRoute(state *model.ImageTaskRetryState, planned *model.ImageTaskRetryState) {
	state.CurrentRouteGroup = planned.CurrentRouteGroup
	state.CurrentRouteIndex = planned.CurrentRouteIndex
	state.CurrentRoutePool = planned.CurrentRoutePool
	state.CurrentGroupAttempts = planned.CurrentGroupAttempts
	state.AttemptedRouteKeys = append([]string(nil), planned.AttemptedRouteKeys...)
}

func createPreparedAsyncImageAttemptTx(tx *gorm.DB, parent *model.Task, state *model.ImageTaskRetryState, planned *model.ImageTaskRetryState, prepared *preparedAsyncImageAttempt) error {
	if prepared == nil || prepared.attempt == nil || prepared.lease == nil {
		return fmt.Errorf("prepared image task attempt is incomplete")
	}
	attempt := prepared.attempt
	if err := tx.Create(attempt).Error; err != nil {
		return err
	}
	prepared.lease.AttemptRecordID = &attempt.ID
	if err := tx.Create(prepared.lease).Error; err != nil {
		return err
	}
	if err := tx.Create(model.NewImageTaskDispatchForAttempt(parent, attempt, prepared.body)).Error; err != nil {
		return err
	}
	applyPlannedAsyncImageRoute(state, planned)
	state.ActiveAttemptRecordID = attempt.ID
	state.AttemptCount = attempt.AttemptNumber
	state.AppendTrace(attempt, model.ImageTaskAttemptPending, "")
	return nil
}

func isRetryableAsyncImageTransitionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "deadlock") ||
		strings.Contains(message, "could not serialize") ||
		strings.Contains(message, "serialization failure")
}

// AdoptLegacyAsyncImageTaskForRetry lazily attaches attempt state to a
// normalized image task created before attempt persistence was introduced.
// Historical administrator requests are locked because the old task row
// cannot prove whether an administrator selected a specific channel.
func AdoptLegacyAsyncImageTaskForRetry(parent *model.Task, providerTaskID string) (*model.ImageTaskAttempt, bool, error) {
	if parent == nil || parent.ID <= 0 {
		return nil, false, fmt.Errorf("legacy image task is invalid")
	}
	var requestRecord model.ImageTaskRequest
	requestResult := model.DB.Where("task_record_id = ?", parent.ID).Limit(1).Find(&requestRecord)
	if requestResult.Error != nil {
		return nil, false, requestResult.Error
	}
	if requestResult.RowsAffected == 0 {
		return nil, false, nil
	}

	originalModel := strings.TrimSpace(parent.Properties.OriginModelName)
	if parent.PrivateData.BillingContext != nil && strings.TrimSpace(parent.PrivateData.BillingContext.OriginModelName) != "" {
		originalModel = strings.TrimSpace(parent.PrivateData.BillingContext.OriginModelName)
	}
	if originalModel == "" {
		originalModel = strings.TrimSpace(parent.Properties.UpstreamModelName)
	}
	userGroup, err := model.GetUserGroup(parent.UserId, true)
	if err != nil {
		return nil, false, err
	}
	tokenGroup := userGroup
	crossGroupRetry := false
	if parent.PrivateData.TokenId > 0 {
		if token, tokenErr := model.GetTokenById(parent.PrivateData.TokenId); tokenErr == nil && token != nil {
			if strings.TrimSpace(token.Group) != "" && strings.TrimSpace(token.Group) != "auto" {
				tokenGroup = strings.TrimSpace(token.Group)
			}
			crossGroupRetry = token.CrossGroupRetry
		}
	}

	state := model.NewImageTaskRetryState(parent, common.RetryTimes, tokenGroup, userGroup, originalModel)
	state.CrossGroupRetry = crossGroupRetry
	state.LockedChannel = model.IsAdmin(parent.UserId)
	state.CurrentRouteGroup = parent.Group
	state.CurrentRouteIndex = -1
	state.CurrentGroupAttempts = 1
	if aggregate, ok := service.GetAggregateGroup(tokenGroup, true); ok && aggregate != nil {
		for index, target := range aggregate.Targets {
			if target.RealGroup != parent.Group {
				continue
			}
			state.AggregateGroup = aggregate.Name
			state.RoutingMode = aggregate.GetRoutingMode()
			state.CurrentRouteIndex = index
			break
		}
	}

	requestID := parent.TaskID
	if parent.PrivateData.BillingContext != nil && strings.TrimSpace(parent.PrivateData.BillingContext.RequestId) != "" {
		requestID = strings.TrimSpace(parent.PrivateData.BillingContext.RequestId)
	}
	attempt := model.NewImageTaskAttempt(
		parent, 1, parent.TaskID, parent.ChannelId, parent.Group,
		state.CurrentRouteIndex, state.CurrentRoutePool, originalModel,
		parent.Properties.UpstreamModelName, parent.Quota,
		parent.PrivateData.BillingContext, requestID,
	)
	attempt.ProviderTaskID = firstNonEmptyImageAttemptValue(providerTaskID, parent.PrivateData.UpstreamTaskID)
	attempt.Status = asyncImageAttemptStatusFromTaskInfo(string(parent.Status))
	attempt.ProgressSequence = parent.PrivateData.ProgressSequence
	attempt.StartedAt = parent.StartTime

	adopted := false
	transition := func(tx *gorm.DB) error {
		var lockedParent model.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedParent, parent.ID).Error; err != nil {
			return err
		}
		if lockedParent.Status == model.TaskStatusSuccess || lockedParent.Status == model.TaskStatusFailure {
			return nil
		}
		var existingState model.ImageTaskRetryState
		stateResult := tx.Where("task_record_id = ?", parent.ID).Limit(1).Find(&existingState)
		if stateResult.Error != nil {
			return stateResult.Error
		}
		if stateResult.RowsAffected > 0 {
			return nil
		}
		var normalizedRequest model.ImageTaskRequest
		requestResult := tx.Where("task_record_id = ?", parent.ID).Limit(1).Find(&normalizedRequest)
		if requestResult.Error != nil {
			return requestResult.Error
		}
		if requestResult.RowsAffected == 0 {
			return nil
		}
		if err := tx.Create(state).Error; err != nil {
			return err
		}
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		state.ActiveAttemptRecordID = attempt.ID
		state.AttemptCount = 1
		state.AppendTrace(attempt, attempt.Status, "")
		state.UpdatedAt = time.Now().Unix()
		if err := tx.Save(state).Error; err != nil {
			return err
		}
		attemptID := attempt.ID
		if err := tx.Model(&model.ImageTaskDispatch{}).
			Where("task_record_id = ? AND attempt_record_id IS NULL", parent.ID).
			Update("attempt_record_id", attemptID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ImageCredentialLease{}).
			Where("task_record_id = ? AND attempt_record_id IS NULL", parent.ID).
			Update("attempt_record_id", attemptID).Error; err != nil {
			return err
		}
		adopted = true
		return nil
	}
	err = model.DB.Transaction(transition)
	if err != nil {
		return nil, false, err
	}
	if adopted {
		logger.LogInfo(context.Background(), fmt.Sprintf(
			"legacy async image task adopted: task_id=%s attempt=%d channel_id=%d retry_limit=%d locked_channel=%t",
			parent.TaskID, attempt.AttemptNumber, attempt.ChannelID, state.RetryLimit, state.LockedChannel,
		))
		return attempt, true, nil
	}
	existing, exists, lookupErr := model.GetImageTaskAttemptByClientTaskID(parent.TaskID)
	if lookupErr != nil || !exists {
		return nil, false, lookupErr
	}
	return existing, false, nil
}

func TransitionAsyncImageAttemptFailure(c *gin.Context, attemptRecordID int64, errorCode, errorMessage string, retryable bool, callbackData []byte) (AsyncImageAttemptFailureOutcome, error) {
	outcome := AsyncImageAttemptFailureOutcome{}
	attempt, state, parent, active, err := model.GetActiveImageTaskAttemptByID(attemptRecordID)
	if err != nil {
		return outcome, err
	}
	if !active {
		outcome.Ignored = true
		return outcome, nil
	}

	plannedState := cloneAsyncImageRetryState(state)
	plannedState.AddFailedChannel(attempt.ChannelID)
	var selection *service.AsyncImageRouteSelection
	var prepared *preparedAsyncImageAttempt
	if retryable && !plannedState.LockedChannel {
		selection, err = service.SelectNextAsyncImageRoute(c, plannedState)
		if err != nil {
			return outcome, err
		}
		if selection != nil && selection.Channel != nil {
			prepared, err = prepareNextAsyncImageAttempt(parent, plannedState, selection)
			if err != nil {
				return outcome, err
			}
		}
	}
	transition := func(tx *gorm.DB) error {
		var attempt model.ImageTaskAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, attemptRecordID).Error; err != nil {
			return err
		}
		var state model.ImageTaskRetryState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_record_id = ?", attempt.TaskRecordID).First(&state).Error; err != nil {
			return err
		}
		var parent model.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent, attempt.TaskRecordID).Error; err != nil {
			return err
		}
		if model.ImageTaskAttemptIsTerminal(attempt.Status) || state.ActiveAttemptRecordID != attempt.ID ||
			parent.Status == model.TaskStatusSuccess || parent.Status == model.TaskStatusFailure {
			outcome.Ignored = true
			return nil
		}
		now := time.Now().Unix()
		attempt.Status = model.ImageTaskAttemptFailed
		attempt.ErrorCode = strings.TrimSpace(errorCode)
		attempt.ErrorMessage = strings.TrimSpace(errorMessage)
		attempt.ErrorRetryable = retryable
		attempt.CallbackData = string(callbackData)
		attempt.FinishedAt = now
		attempt.UpdatedAt = now
		if err := tx.Save(&attempt).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ImageCredentialLease{}).
			Where("attempt_record_id = ? AND status IN ?", attempt.ID, []string{
				model.ImageCredentialLeaseStatusActive,
				model.ImageCredentialLeaseStatusResolved,
			}).
			Updates(map[string]any{
				"status":     model.ImageCredentialLeaseStatusFailed,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		state.AddFailedChannel(attempt.ChannelID)
		state.AppendTrace(&attempt, model.ImageTaskAttemptFailed, attempt.ErrorCode)
		state.ActiveAttemptRecordID = 0
		state.Version++
		state.UpdatedAt = now
		if !retryable || state.LockedChannel {
			state.Status = model.ImageTaskRetryStateFailed
			outcome.Exhausted = true
			return tx.Save(&state).Error
		}

		if selection == nil || selection.Channel == nil {
			state.Status = model.ImageTaskRetryStateExhausted
			outcome.Exhausted = true
			return tx.Save(&state).Error
		}
		if err := createPreparedAsyncImageAttemptTx(tx, &parent, &state, plannedState, prepared); err != nil {
			return err
		}
		state.Version++
		state.UpdatedAt = now
		if err := tx.Save(&state).Error; err != nil {
			return err
		}
		outcome.Retrying = true
		outcome.NextAttempt = prepared.attempt
		privateData := parent.PrivateData
		privateData.ProgressMetadataSet = false
		privateData.ProgressKnown = false
		privateData.ProgressSource = ""
		privateData.ProgressStage = ""
		privateData.ProgressSequence = 0
		privateData.LastUpstreamStatus = 0
		return tx.Model(&model.Task{}).Where("id = ? AND status NOT IN ?", parent.ID, []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure}).Updates(map[string]any{
			"status":       model.TaskStatusQueued,
			"progress":     taskcommon.ProgressQueued,
			"fail_reason":  "",
			"private_data": privateData,
			"updated_at":   now,
		}).Error
	}
	for transactionAttempt := 0; transactionAttempt < 3; transactionAttempt++ {
		outcome = AsyncImageAttemptFailureOutcome{}
		err = model.DB.Transaction(transition)
		if err == nil || !isRetryableAsyncImageTransitionError(err) {
			break
		}
		if prepared != nil {
			prepared.attempt.ID = 0
			prepared.lease.ID = 0
			prepared.lease.AttemptRecordID = nil
		}
		time.Sleep(time.Duration(transactionAttempt+1) * 5 * time.Millisecond)
	}
	if err == nil && outcome.Retrying {
		service.RecordAsyncImageRouteAttempt(c, plannedState, selection)
		logger.LogInfo(prepared.context, fmt.Sprintf("async image attempt scheduled: task_id=%s attempt=%d client_task_id=%s channel_id=%d route_group=%s route_index=%d route_pool=%s",
			parent.TaskID, prepared.attempt.AttemptNumber, prepared.attempt.ClientTaskID, prepared.attempt.ChannelID,
			prepared.attempt.RouteGroup, prepared.attempt.RouteIndex, prepared.attempt.RoutePool))
	}
	return outcome, err
}

func PrepareAsyncImageAttemptSuccess(attemptRecordID int64, providerTaskID string, taskInfo *relaycommon.TaskInfo, callbackData []byte) (*model.Task, bool, error) {
	var result *model.Task
	ignored := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var attempt model.ImageTaskAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, attemptRecordID).Error; err != nil {
			return err
		}
		var state model.ImageTaskRetryState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_record_id = ?", attempt.TaskRecordID).First(&state).Error; err != nil {
			return err
		}
		var parent model.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent, attempt.TaskRecordID).Error; err != nil {
			return err
		}
		if model.ImageTaskAttemptIsTerminal(attempt.Status) || state.ActiveAttemptRecordID != attempt.ID ||
			parent.Status == model.TaskStatusSuccess || parent.Status == model.TaskStatusFailure {
			ignored = true
			return nil
		}
		taskInfoJSON, err := common.Marshal(taskInfo)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		attempt.Status = model.ImageTaskAttemptSucceeded
		attempt.ProviderTaskID = firstNonEmptyImageAttemptValue(attempt.ProviderTaskID, providerTaskID)
		attempt.CallbackData = string(callbackData)
		attempt.TaskInfoJSON = string(taskInfoJSON)
		attempt.FinishedAt = now
		attempt.UpdatedAt = now
		if err := tx.Save(&attempt).Error; err != nil {
			return err
		}
		state.Status = model.ImageTaskRetryStateSucceeded
		state.AppendTrace(&attempt, model.ImageTaskAttemptSucceeded, "")
		state.Version++
		state.UpdatedAt = now
		if err := tx.Save(&state).Error; err != nil {
			return err
		}

		parent.ChannelId = attempt.ChannelID
		parent.Group = attempt.RouteGroup
		parent.Properties.OriginModelName = attempt.OriginModel
		parent.Properties.UpstreamModelName = attempt.UpstreamModel
		parent.PrivateData.UpstreamTaskID = attempt.ProviderTaskID
		parent.PrivateData.BillingContext = cloneImageAttemptBillingContext(attempt.BillingContext, parent.PrivateData.BillingContext)
		if err := tx.Model(&model.Task{}).Where("id = ? AND status NOT IN ?", parent.ID, []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure}).Updates(map[string]any{
			"channel_id":   parent.ChannelId,
			"group":        parent.Group,
			"properties":   parent.Properties,
			"private_data": parent.PrivateData,
			"updated_at":   now,
		}).Error; err != nil {
			return err
		}
		result = &parent
		return nil
	})
	return result, ignored, err
}

func ApplyAsyncImageAttemptProgress(attemptRecordID int64, providerTaskID string, taskInfo *relaycommon.TaskInfo, callbackData []byte) (bool, error) {
	ignored := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var attempt model.ImageTaskAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, attemptRecordID).Error; err != nil {
			return err
		}
		var state model.ImageTaskRetryState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_record_id = ?", attempt.TaskRecordID).First(&state).Error; err != nil {
			return err
		}
		var parent model.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent, attempt.TaskRecordID).Error; err != nil {
			return err
		}
		if model.ImageTaskAttemptIsTerminal(attempt.Status) || state.ActiveAttemptRecordID != attempt.ID ||
			parent.Status == model.TaskStatusSuccess || parent.Status == model.TaskStatusFailure ||
			(taskInfo.Sequence > 0 && taskInfo.Sequence <= attempt.ProgressSequence) {
			ignored = true
			return nil
		}
		now := time.Now().Unix()
		attemptStatus := asyncImageAttemptStatusFromTaskInfo(taskInfo.Status)
		taskInfoJSON, err := common.Marshal(taskInfo)
		if err != nil {
			return err
		}
		attempt.ProviderTaskID = firstNonEmptyImageAttemptValue(attempt.ProviderTaskID, providerTaskID)
		attempt.Status = attemptStatus
		attempt.ProgressSequence = taskInfo.Sequence
		attempt.CallbackData = string(callbackData)
		attempt.TaskInfoJSON = string(taskInfoJSON)
		if attempt.StartedAt == 0 && attemptStatus == model.ImageTaskAttemptProcessing {
			attempt.StartedAt = now
		}
		attempt.UpdatedAt = now
		if err := tx.Save(&attempt).Error; err != nil {
			return err
		}

		parentStatus := model.TaskStatus(taskInfo.Status)
		progress := taskInfo.Progress
		switch parentStatus {
		case model.TaskStatusSubmitted:
			if progress == "" {
				progress = taskcommon.ProgressSubmitted
			}
		case model.TaskStatusQueued:
			if progress == "" {
				progress = taskcommon.ProgressQueued
			}
		case model.TaskStatusInProgress:
			if progress == "" {
				progress = taskcommon.ProgressInProgress
			}
			if parent.StartTime == 0 {
				parent.StartTime = now
			}
		default:
			return fmt.Errorf("invalid non-terminal image attempt status %s", taskInfo.Status)
		}
		privateData := parent.PrivateData
		if taskInfo.ProgressMetadataSet {
			privateData.ProgressMetadataSet = true
			privateData.ProgressKnown = taskInfo.ProgressKnown
			privateData.ProgressSource = taskInfo.ProgressSource
			privateData.ProgressStage = taskInfo.Stage
			privateData.ProgressSequence = taskInfo.Sequence
		}
		updates := map[string]any{
			"status":       parentStatus,
			"progress":     progress,
			"start_time":   parent.StartTime,
			"private_data": privateData,
			"updated_at":   now,
		}
		if len(callbackData) > 0 {
			updates["data"] = callbackData
		}
		result := tx.Model(&model.Task{}).
			Where("id = ? AND status NOT IN ?", parent.ID, []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure}).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			ignored = true
		}
		return nil
	})
	return ignored, err
}

func asyncImageAttemptStatusFromTaskInfo(status string) string {
	switch model.TaskStatus(status) {
	case model.TaskStatusSubmitted:
		return model.ImageTaskAttemptSubmitted
	case model.TaskStatusQueued:
		return model.ImageTaskAttemptQueued
	case model.TaskStatusInProgress:
		return model.ImageTaskAttemptProcessing
	case model.TaskStatusSuccess:
		return model.ImageTaskAttemptSucceeded
	case model.TaskStatusFailure:
		return model.ImageTaskAttemptFailed
	default:
		return model.ImageTaskAttemptPending
	}
}

func cloneImageAttemptBillingContext(winner model.TaskBillingContext, existing *model.TaskBillingContext) *model.TaskBillingContext {
	consumeLogID := 0
	finalLog := (*model.TaskFinalConsumeLogSnapshot)(nil)
	if existing != nil {
		consumeLogID = existing.ConsumeLogId
		finalLog = existing.FinalConsumeLog
	}
	winner.ConsumeLogId = consumeLogID
	winner.FinalConsumeLog = finalLog
	return &winner
}

func firstNonEmptyImageAttemptValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func GetAsyncImageAttemptParent(attempt *model.ImageTaskAttempt) (*model.Task, error) {
	if attempt == nil {
		return nil, errors.New("image task attempt is nil")
	}
	return model.GetTaskByRecordID(attempt.TaskRecordID)
}

func LogAsyncImageAttemptIgnored(ctx context.Context, taskID string, attemptNumber int, reason string) {
	logger.LogInfo(ctx, fmt.Sprintf("async image attempt callback ignored: task_id=%s attempt=%d reason=%s", taskID, attemptNumber, reason))
}
