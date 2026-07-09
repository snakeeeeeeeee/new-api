package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			service.DumpRelayErrorIfNeeded(c, newAPIError)
			if common.GetContextKeyString(c, constant.ContextKeyAggregateGroup) != "" {
				logger.LogError(c, buildAggregateRelayErrorLog(c, newAPIError))
			} else {
				logger.LogError(c, fmt.Sprintf("relay error: %s", newAPIError.Error()))
			}
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				if shouldWrapClientFacingRelayError(newAPIError) {
					c.JSON(http.StatusInternalServerError, gin.H{
						"type":  "error",
						"error": buildClientFacingClaudeError(newAPIError),
					})
				} else {
					c.JSON(newAPIError.StatusCode, gin.H{
						"type":  "error",
						"error": buildClientFacingRelayClaudeError(newAPIError),
					})
				}
			default:
				if shouldWrapClientFacingRelayError(newAPIError) {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": buildClientFacingOpenAIError(newAPIError),
					})
				} else {
					c.JSON(newAPIError.StatusCode, gin.H{
						"error": buildClientFacingRelayOpenAIError(newAPIError),
					})
				}
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needViolationDetect := service.IsViolationDetectionEnabled()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken || needViolationDetect {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needViolationDetect && meta != nil {
		newAPIError = service.CheckViolationAndHandle(c, relayInfo, meta.CombineText)
		if newAPIError != nil {
			return
		}
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError)
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			relaycommon.LogClaudeToolSchemaCompatOriginalSchemasOnError(relayInfo, newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	_, isAggregateGroupRequest := service.GetAggregateGroup(relayInfo.TokenGroup, true)
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for {
		if !isAggregateGroupRequest && retryParam.GetRetry() > common.RetryTimes {
			break
		}
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}

		service.FillViolationLogRouteContextIfNeeded(c, relayInfo)
		addUsedChannel(c, channel.Id)
		service.DumpRawRequestIfNeeded(c)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError
		relaycommon.LogClaudeToolSchemaCompatOriginalSchemasOnError(relayInfo, newAPIError)
		service.RecordRelayTimingContext(c, relayInfo)

		shouldRetryRequest := shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry())
		isInternalRetryLog := shouldRetryRequest
		if isAggregateGroupRequest {
			if transition := service.PrepareAggregateGroupRetry(c, retryParam.GetRetry(), relayInfo.OriginModelName, common.RetryTimes); transition != nil {
				groupRetryable := shouldRetry(c, newAPIError, 1)
				shouldRetryRequest = groupRetryable
				isInternalRetryLog = groupRetryable && transition.HasNext
				if groupRetryable && !transition.WithinCurrentGroup {
					service.RecordAggregateRouteSmartFailure(c, relayInfo.OriginModelName, transition.FailedGroup, newAPIError.StatusCode)
				}
				if transition.HasNext {
					if transition.WithinCurrentGroup {
						logger.LogWarn(c, fmt.Sprintf(
							"aggregate group internal retry: aggregate_group=%s, model=%s, route_group=%s(index=%d), status_code=%d, retry=%d/%d",
							transition.AggregateGroup,
							relayInfo.OriginModelName,
							transition.FailedGroup,
							transition.FailedIndex,
							newAPIError.StatusCode,
							retryParam.GetRetry()+1,
							common.RetryTimes,
						))
					} else {
						logger.LogWarn(c, fmt.Sprintf(
							"aggregate fallback retry: aggregate_group=%s, model=%s, failed_group=%s(index=%d), next_group=%s(index=%d), status_code=%d",
							transition.AggregateGroup,
							relayInfo.OriginModelName,
							transition.FailedGroup,
							transition.FailedIndex,
							transition.NextGroup,
							transition.NextIndex,
							newAPIError.StatusCode,
						))
					}
				} else {
					logger.LogWarn(c, fmt.Sprintf(
						"aggregate fallback exhausted: aggregate_group=%s, model=%s, failed_group=%s(index=%d), status_code=%d, no next route group",
						transition.AggregateGroup,
						relayInfo.OriginModelName,
						transition.FailedGroup,
						transition.FailedIndex,
						newAPIError.StatusCode,
					))
					shouldRetryRequest = false
				}
			}
		}
		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError, isInternalRetryLog)

		if !shouldRetryRequest {
			break
		}
		retryParam.IncreaseRetry()
	}

	logUsedChannelTrace(c)
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

const (
	clientFacingRelayErrorMessage = "Service temporarily unavailable, please try again later."
	clientFacingRelayErrorType    = "new_api_error"
	clientFacingRelayErrorCode    = "service_unavailable"
)

func shouldWrapClientFacingRelayError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if isLocalClaudeCompatError(err) {
		return false
	}
	switch err.GetErrorType() {
	case types.ErrorTypeOpenAIError, types.ErrorTypeClaudeError:
		return !shouldPassthroughClientFacingRelayError(err)
	default:
		return false
	}
}

func shouldPassthroughClientFacingRelayError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if !operation_setting.ShouldPassthroughRelayErrorStatusCode(err.StatusCode) {
		return false
	}
	return !operation_setting.ShouldBlockRelayErrorPassthrough(err.Error())
}

func buildClientFacingRelayOpenAIError(err *types.NewAPIError) types.OpenAIError {
	if !isUpstreamClientFacingRelayError(err) {
		return err.ToOpenAIError()
	}
	openaiErr := err.ToOpenAIError()
	openaiErr.Message = relayErrorMessageForClient(err)
	if openaiErr.Message == "" {
		openaiErr.Message = string(err.GetErrorType())
	}
	return openaiErr
}

func buildClientFacingRelayClaudeError(err *types.NewAPIError) types.ClaudeError {
	if !isUpstreamClientFacingRelayError(err) {
		return err.ToClaudeError()
	}
	claudeErr := err.ToClaudeError()
	if openaiErr, ok := err.RelayError.(types.OpenAIError); ok && openaiErr.Type != "" {
		claudeErr.Type = openaiErr.Type
	}
	claudeErr.Message = relayErrorMessageForClient(err)
	if claudeErr.Message == "" {
		claudeErr.Message = string(err.GetErrorType())
	}
	return claudeErr
}

func relayErrorMessageForClient(err *types.NewAPIError) string {
	if err == nil {
		return ""
	}
	if operation_setting.GetRelayErrorSetting().MaskSensitive && !isLocalClaudeCompatError(err) {
		return err.MaskSensitiveError()
	}
	return err.Error()
}

func isLocalClaudeCompatError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if strings.HasPrefix(string(err.GetErrorCode()), "claude_") {
		return true
	}
	if claudeErr, ok := err.RelayError.(types.ClaudeError); ok {
		if code, ok := claudeErr.Code.(string); ok {
			return strings.HasPrefix(code, "claude_")
		}
	}
	return false
}

func isUpstreamClientFacingRelayError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if isLocalClaudeCompatError(err) {
		return false
	}
	switch err.GetErrorType() {
	case types.ErrorTypeOpenAIError, types.ErrorTypeClaudeError:
		return true
	default:
		return false
	}
}

func buildClientFacingOpenAIError(_ *types.NewAPIError) types.OpenAIError {
	return types.OpenAIError{
		Message: clientFacingRelayErrorMessage,
		Type:    clientFacingRelayErrorType,
		Code:    clientFacingRelayErrorCode,
	}
}

func buildClientFacingClaudeError(_ *types.NewAPIError) types.ClaudeError {
	return types.ClaudeError{
		Message: clientFacingRelayErrorMessage,
		Type:    clientFacingRelayErrorType,
	}
}

func relayClientResponseLogFields(err *types.NewAPIError) map[string]interface{} {
	fields := make(map[string]interface{})
	if err == nil {
		return fields
	}
	wrapped := shouldWrapClientFacingRelayError(err)
	fields["client_response_wrapped"] = wrapped
	if wrapped {
		fields["client_response_status_code"] = http.StatusInternalServerError
		fields["client_response_message"] = clientFacingRelayErrorMessage
		fields["client_response_error_type"] = clientFacingRelayErrorType
		fields["client_response_error_code"] = clientFacingRelayErrorCode
		return fields
	}
	fields["client_response_status_code"] = err.StatusCode
	fields["client_response_message"] = relayErrorMessageForClient(err)
	if openAIError := err.ToOpenAIError(); openAIError.Type != "" || openAIError.Code != nil {
		fields["client_response_error_type"] = openAIError.Type
		fields["client_response_error_code"] = common.Interface2String(openAIError.Code)
	}
	return fields
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)

	useChannelTrace := c.GetStringSlice("use_channel_trace")
	useChannelTrace = append(useChannelTrace, formatUsedChannelTrace(c, channelId))
	c.Set("use_channel_trace", useChannelTrace)
}

func formatUsedChannelTrace(c *gin.Context, channelId int) string {
	if c == nil {
		return fmt.Sprintf("channel#%d", channelId)
	}
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	channelName := c.GetString("channel_name")
	switch {
	case routeGroup != "" && channelName != "":
		return fmt.Sprintf("%s(channel#%d:%s)", routeGroup, channelId, channelName)
	case routeGroup != "":
		return fmt.Sprintf("%s(channel#%d)", routeGroup, channelId)
	case channelName != "":
		return fmt.Sprintf("channel#%d(%s)", channelId, channelName)
	default:
		return fmt.Sprintf("channel#%d", channelId)
	}
}

func logUsedChannelTrace(c *gin.Context) {
	useChannelTrace := c.GetStringSlice("use_channel_trace")
	if len(useChannelTrace) <= 1 {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("聚合链路尝试：%s", strings.Join(useChannelTrace, " -> ")))
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil && (retryParam == nil || retryParam.GetRetry() == 0) {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	if aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup); aggregateGroup != "" {
		if decision, configured := service.ShouldRetryStatusCodeByAggregateGroup(aggregateGroup, code); configured {
			return decision
		}
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func buildAggregateChannelErrorLog(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) string {
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	modelName := c.GetString("original_model")
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	routeGroupIndex := common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex)
	channelName := c.GetString("channel_name")
	userId := c.GetInt("id")
	return fmt.Sprintf(
		"aggregate channel error: aggregate_group=%s, model=%s, route_group=%s(index=%d), user_id=%d, channel#%d(%s), status_code=%d, error=%s",
		aggregateGroup,
		modelName,
		routeGroup,
		routeGroupIndex,
		userId,
		channelError.ChannelId,
		channelName,
		err.StatusCode,
		err.Error(),
	)
}

func buildAggregateRelayErrorLog(c *gin.Context, err *types.NewAPIError) string {
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	modelName := c.GetString("original_model")
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	routeGroupIndex := common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex)
	userId := c.GetInt("id")
	return fmt.Sprintf(
		"aggregate relay error: aggregate_group=%s, model=%s, route_group=%s(index=%d), user_id=%d, status_code=%d, error=%s",
		aggregateGroup,
		modelName,
		routeGroup,
		routeGroupIndex,
		userId,
		err.StatusCode,
		err.Error(),
	)
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, internalRetry ...bool) {
	service.DumpRelayErrorIfNeeded(c, err)
	if common.GetContextKeyString(c, constant.ContextKeyAggregateGroup) != "" {
		logger.LogError(c, buildAggregateChannelErrorLog(c, channelError, err))
		service.RecordAggregateRouteRPMFailure(c, c.GetString("original_model"))
	} else {
		logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, err.Error()))
	}
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	if service.ShouldDisableChannel(channelError.ChannelType, err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	isInternalRetry := len(internalRetry) > 0 && internalRetry[0]
	recordRelayErrorLog(c, err, isInternalRetry)
}

func recordRelayErrorLog(c *gin.Context, err *types.NewAPIError, internalRetry bool) {
	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["upstream_status_code"] = err.StatusCode
		other["upstream_error_message"] = err.MaskSensitiveError()
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		for key, value := range relayClientResponseLogFields(err) {
			other[key] = value
		}
		if executionMode := common.GetContextKeyString(c, constant.ContextKeyExecutionMode); executionMode != "" {
			other["execution_mode"] = executionMode
		}
		if detail, ok := common.GetContextKey(c, constant.ContextKeyImageHandleSyncErrorDetail); ok && detail != nil {
			other["image_handle_sync_error"] = detail
		}
		if len(err.Metadata) > 0 {
			var metadata map[string]interface{}
			if unmarshalErr := common.Unmarshal(err.Metadata, &metadata); unmarshalErr == nil && len(metadata) > 0 {
				other["error_metadata"] = metadata
			}
		}
		if internalRetry {
			other["internal_retry"] = true
		} else {
			other["user_safe"] = true
		}
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		service.AppendAggregateGroupAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		isStream := common.GetContextKeyBool(c, constant.ContextKeyRelayIsStream)
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, isStream, userGroup, other)
	}
}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	_, isAggregateGroupRequest := service.GetAggregateGroup(relayInfo.TokenGroup, true)

	for {
		if !isAggregateGroupRequest && retryParam.GetRetry() > common.RetryTimes {
			break
		}
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if c.Request != nil && c.Request.URL != nil && c.Request.URL.Path == "/v1/image/tasks" {
			break
		}
		shouldRetryTask := shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry())
		isInternalRetryLog := shouldRetryTask
		if isAggregateGroupRequest {
			if transition := service.PrepareAggregateGroupRetry(c, retryParam.GetRetry(), relayInfo.OriginModelName, common.RetryTimes); transition != nil {
				groupRetryable := shouldRetryTaskRelay(c, channel.Id, taskErr, 1)
				shouldRetryTask = groupRetryable
				isInternalRetryLog = groupRetryable && transition.HasNext
				if groupRetryable && !transition.WithinCurrentGroup {
					service.RecordAggregateRouteSmartFailure(c, relayInfo.OriginModelName, transition.FailedGroup, taskErr.StatusCode)
				}
				if transition.HasNext {
					if transition.WithinCurrentGroup {
						logger.LogWarn(c, fmt.Sprintf(
							"aggregate task internal retry: aggregate_group=%s, model=%s, route_group=%s(index=%d), status_code=%d, retry=%d/%d",
							transition.AggregateGroup,
							relayInfo.OriginModelName,
							transition.FailedGroup,
							transition.FailedIndex,
							taskErr.StatusCode,
							retryParam.GetRetry()+1,
							common.RetryTimes,
						))
					} else {
						logger.LogWarn(c, fmt.Sprintf(
							"aggregate task fallback retry: aggregate_group=%s, model=%s, failed_group=%s(index=%d), next_group=%s(index=%d), status_code=%d",
							transition.AggregateGroup,
							relayInfo.OriginModelName,
							transition.FailedGroup,
							transition.FailedIndex,
							transition.NextGroup,
							transition.NextIndex,
							taskErr.StatusCode,
						))
					}
				} else {
					logger.LogWarn(c, fmt.Sprintf(
						"aggregate task fallback exhausted: aggregate_group=%s, model=%s, failed_group=%s(index=%d), status_code=%d, no next route group",
						transition.AggregateGroup,
						relayInfo.OriginModelName,
						transition.FailedGroup,
						transition.FailedIndex,
						taskErr.StatusCode,
					))
					shouldRetryTask = false
				}
			}
		}
		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode),
				isInternalRetryLog)
		}
		if !shouldRetryTask {
			break
		}
		retryParam.IncreaseRetry()
	}

	logUsedChannelTrace(c)

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if result != nil && result.ExistingTask != nil {
			return
		}
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)
		if result != nil && result.CreatedTask != nil {
			result.CreatedTask.PrivateData.UpstreamTaskID = result.UpstreamTaskID
			result.CreatedTask.Data = result.TaskData
			if updateErr := model.DB.Model(&model.Task{}).
				Where("id = ?", result.CreatedTask.ID).
				Updates(map[string]any{
					"private_data": result.CreatedTask.PrivateData,
					"data":         result.CreatedTask.Data,
					"updated_at":   time.Now().Unix(),
				}).Error; updateErr != nil {
				common.SysError("update pre-created task error: " + updateErr.Error())
			}
			return
		}

		task := model.InitTask(result.Platform, relayInfo)
		perCallBilling := common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) ||
			relayInfo.ChannelType == constant.ChannelTypeXai
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		if imageSnapshot, err := buildAsyncImageRequestSnapshot(c, relayInfo); err == nil && imageSnapshot != nil && len(imageSnapshot.request) > 0 {
			task.PrivateData.ImageRequest = imageSnapshot.request
			task.PrivateData.ImageInputURLs = imageSnapshot.images
			task.PrivateData.ImageMaskURL = imageSnapshot.mask
		} else if err != nil {
			common.SysError("build async image request snapshot error: " + err.Error())
		}
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:               relayInfo.PriceData.ModelPrice,
			GroupRatio:               relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			OriginalGroupRatio:       relayInfo.PriceData.GroupRatioInfo.OriginalGroupRatio,
			RatioOverride:            relayInfo.PriceData.GroupRatioInfo.RatioOverride,
			GroupSpecialRatio:        relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio,
			HasSpecialRatio:          relayInfo.PriceData.GroupRatioInfo.HasSpecialRatio,
			HasRatioOverride:         relayInfo.PriceData.GroupRatioInfo.HasRatioOverride,
			RatioOverrideApplied:     relayInfo.PriceData.GroupRatioInfo.RatioOverrideApplied,
			RouteModelGroupRatio:     relayInfo.PriceData.GroupRatioInfo.RouteModelGroupRatio,
			HasRouteModelGroupRatio:  relayInfo.PriceData.GroupRatioInfo.HasRouteModelGroupRatio,
			RouteModelAggregateGroup: relayInfo.PriceData.GroupRatioInfo.RouteModelRatioAggregateGroup,
			RouteModelRealGroup:      relayInfo.PriceData.GroupRatioInfo.RouteModelRatioRealGroup,
			RouteModelName:           relayInfo.PriceData.GroupRatioInfo.RouteModelRatioModelName,
			ModelRatio:               relayInfo.PriceData.ModelRatio,
			OtherRatios:              relayInfo.PriceData.OtherRatios,
			OriginModelName:          relayInfo.OriginModelName,
			PerCallBilling:           perCallBilling,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

type asyncImageRequestSnapshot struct {
	request json.RawMessage
	images  []string
	mask    string
}

func buildAsyncImageRequestSnapshot(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*asyncImageRequestSnapshot, error) {
	if c == nil || c.Request == nil || c.Request.URL == nil || c.Request.URL.Path != "/v1/image/tasks" {
		return nil, nil
	}
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	imgReq := dto.ImageRequest{
		Model:  relayInfo.UpstreamModelName,
		Prompt: taskReq.Prompt,
		Size:   taskReq.Size,
	}
	if imgReq.Model == "" {
		imgReq.Model = relayInfo.OriginModelName
	}
	if taskReq.Image != "" {
		imgReq.Image, _ = common.Marshal(taskReq.Image)
	}
	images := append([]string{}, taskReq.Images...)
	if len(images) == 0 && strings.TrimSpace(taskReq.Image) != "" {
		images = []string{strings.TrimSpace(taskReq.Image)}
	}
	mask := ""
	if taskReq.Metadata != nil {
		if v, ok := taskReq.Metadata["quality"].(string); ok {
			imgReq.Quality = v
		}
		if v, ok := taskReq.Metadata["response_format"].(string); ok {
			imgReq.ResponseFormat = v
		}
		if n, ok := numericMetadataToUint(taskReq.Metadata["n"]); ok {
			imgReq.N = &n
		}
		if raw, ok := rawMetadataValue(taskReq.Metadata["output_format"]); ok {
			imgReq.OutputFormat = raw
		}
		if raw, ok := rawMetadataValue(taskReq.Metadata["output_compression"]); ok {
			imgReq.OutputCompression = raw
		}
		if raw, ok := rawMetadataValue(taskReq.Metadata["background"]); ok {
			imgReq.Background = raw
		}
		if raw, ok := rawMetadataValue(taskReq.Metadata["moderation"]); ok {
			imgReq.Moderation = raw
		}
		if v, ok := taskReq.Metadata["mask"].(string); ok {
			mask = strings.TrimSpace(v)
		}
	}
	data, err := common.Marshal(imgReq)
	if err != nil {
		return nil, err
	}
	return &asyncImageRequestSnapshot{
		request: json.RawMessage(data),
		images:  images,
		mask:    mask,
	}, nil
}

func numericMetadataToUint(value any) (uint, bool) {
	switch v := value.(type) {
	case int:
		if v > 0 {
			return uint(v), true
		}
	case int64:
		if v > 0 {
			return uint(v), true
		}
	case float64:
		if v > 0 {
			return uint(v), true
		}
	case json.Number:
		i, err := v.Int64()
		if err == nil && i > 0 {
			return uint(i), true
		}
	}
	return 0, false
}

func rawMetadataValue(value any) (json.RawMessage, bool) {
	if value == nil {
		return nil, false
	}
	data, err := common.Marshal(value)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return json.RawMessage(data), true
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}
