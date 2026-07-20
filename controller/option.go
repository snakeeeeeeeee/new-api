package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func buildTokenTierPricingRulesMetaValue() string {
	jsonBytes, err := common.Marshal(ratio_setting.GetTokenTierPricingRulesMeta())
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		if k == "ExternalRegisterAuthKey" {
			continue
		}
		value := common.Interface2String(v)
		if strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key") {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	options = append(options, &model.Option{
		Key:   "TokenTierPricingRulesMeta",
		Value: buildTokenTierPricingRulesMetaValue(),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

func parseExternalRegisterAuthKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var authKeys []string
	if err := common.UnmarshalJsonStr(raw, &authKeys); err == nil {
		return normalizeExternalRegisterAuthKeys(authKeys)
	}

	return normalizeExternalRegisterAuthKeys([]string{raw})
}

func normalizeExternalRegisterAuthKeys(authKeys []string) []string {
	normalized := make([]string, 0, len(authKeys))
	seen := make(map[string]struct{}, len(authKeys))
	for _, authKey := range authKeys {
		authKey = strings.TrimSpace(authKey)
		if authKey == "" {
			continue
		}
		if _, ok := seen[authKey]; ok {
			continue
		}
		seen[authKey] = struct{}{}
		normalized = append(normalized, authKey)
	}
	return normalized
}

func encodeExternalRegisterAuthKeys(authKeys []string) (string, error) {
	authKeys = normalizeExternalRegisterAuthKeys(authKeys)
	if len(authKeys) == 0 {
		return "", nil
	}
	data, err := common.Marshal(authKeys)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseExternalTopupAuthKeys(raw string) []string {
	return parseExternalRegisterAuthKeys(raw)
}

func encodeExternalTopupAuthKeys(authKeys []string) (string, error) {
	return encodeExternalRegisterAuthKeys(authKeys)
}

func externalRegisterAuthCodeResponse(enabled bool, authKeys []string) gin.H {
	authKeys = normalizeExternalRegisterAuthKeys(authKeys)
	authKey := ""
	if len(authKeys) > 0 {
		authKey = authKeys[len(authKeys)-1]
	}
	return gin.H{
		"enabled":    enabled,
		"configured": len(authKeys) > 0,
		"auth_key":   authKey,
		"auth_keys":  authKeys,
	}
}

func externalTopupAuthCodeResponse(enabled bool, authKeys []string) gin.H {
	authKeys = normalizeExternalRegisterAuthKeys(authKeys)
	authKey := ""
	if len(authKeys) > 0 {
		authKey = authKeys[len(authKeys)-1]
	}
	return gin.H{
		"enabled":         enabled,
		"configured":      len(authKeys) > 0,
		"auth_key":        authKey,
		"auth_keys":       authKeys,
		"callback_secret": common.ExternalTopupCallbackSecret,
	}
}

type deleteExternalRegisterAuthCodeRequest struct {
	AuthKey string `json:"auth_key"`
}

func GetExternalRegisterAuthCode(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalRegisterAuthKeys(common.OptionMap["ExternalRegisterAuthKey"])
	enabled := common.OptionMap["ExternalRegisterEnabled"] == "true"
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalRegisterAuthCodeResponse(enabled, authKeys),
	})
}

func GenerateExternalRegisterAuthCode(c *gin.Context) {
	authKey, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		common.SysLog("failed to generate external register auth key: " + err.Error())
		return
	}
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalRegisterAuthKeys(common.OptionMap["ExternalRegisterAuthKey"])
	common.OptionMapRWMutex.RUnlock()
	authKeys = append(authKeys, authKey)
	authKeysValue, err := encodeExternalRegisterAuthKeys(authKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalRegisterAuthKey", authKeysValue); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalRegisterEnabled", "true"); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalRegisterAuthCodeResponse(true, authKeys),
	})
}

func DeleteExternalRegisterAuthCode(c *gin.Context) {
	targetAuthKey := strings.TrimSpace(c.Query("auth_key"))
	if targetAuthKey == "" && c.Request.Body != nil {
		var req deleteExternalRegisterAuthCodeRequest
		if err := common.DecodeJson(c.Request.Body, &req); err == nil {
			targetAuthKey = strings.TrimSpace(req.AuthKey)
		}
	}
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalRegisterAuthKeys(common.OptionMap["ExternalRegisterAuthKey"])
	common.OptionMapRWMutex.RUnlock()
	if targetAuthKey == "" {
		authKeys = nil
	} else {
		filtered := make([]string, 0, len(authKeys))
		for _, authKey := range authKeys {
			if authKey != targetAuthKey {
				filtered = append(filtered, authKey)
			}
		}
		authKeys = filtered
	}

	authKeysValue, err := encodeExternalRegisterAuthKeys(authKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalRegisterAuthKey", authKeysValue); err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := len(authKeys) > 0
	if !enabled {
		if err := model.UpdateOption("ExternalRegisterEnabled", "false"); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if !enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    externalRegisterAuthCodeResponse(false, authKeys),
		})
		return
	}
	common.OptionMapRWMutex.RLock()
	enabled = common.OptionMap["ExternalRegisterEnabled"] == "true"
	common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalRegisterAuthCodeResponse(enabled, authKeys),
	})
}

func DeleteAllExternalRegisterAuthCodes(c *gin.Context) {
	if err := model.UpdateOption("ExternalRegisterAuthKey", ""); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalRegisterEnabled", "false"); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalRegisterAuthCodeResponse(false, nil),
	})
}

func GetExternalTopupAuthCode(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalTopupAuthKeys(common.OptionMap["ExternalTopupAuthKey"])
	enabled := common.OptionMap["ExternalTopupEnabled"] == "true"
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalTopupAuthCodeResponse(enabled, authKeys),
	})
}

func GenerateExternalTopupAuthCode(c *gin.Context) {
	authKey, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		common.SysLog("failed to generate external topup auth key: " + err.Error())
		return
	}
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalTopupAuthKeys(common.OptionMap["ExternalTopupAuthKey"])
	callbackSecret := strings.TrimSpace(common.OptionMap["ExternalTopupCallbackSecret"])
	common.OptionMapRWMutex.RUnlock()
	authKeys = append(authKeys, authKey)
	authKeysValue, err := encodeExternalTopupAuthKeys(authKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if callbackSecret == "" {
		callbackSecret, err = common.GenerateRandomCharsKey(48)
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
			common.SysLog("failed to generate external topup callback secret: " + err.Error())
			return
		}
		if err := model.UpdateOption("ExternalTopupCallbackSecret", callbackSecret); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := model.UpdateOption("ExternalTopupAuthKey", authKeysValue); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalTopupEnabled", "true"); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalTopupAuthCodeResponse(true, authKeys),
	})
}

func DeleteExternalTopupAuthCode(c *gin.Context) {
	targetAuthKey := strings.TrimSpace(c.Query("auth_key"))
	if targetAuthKey == "" && c.Request.Body != nil {
		var req deleteExternalRegisterAuthCodeRequest
		if err := common.DecodeJson(c.Request.Body, &req); err == nil {
			targetAuthKey = strings.TrimSpace(req.AuthKey)
		}
	}
	common.OptionMapRWMutex.RLock()
	authKeys := parseExternalTopupAuthKeys(common.OptionMap["ExternalTopupAuthKey"])
	common.OptionMapRWMutex.RUnlock()
	if targetAuthKey == "" {
		authKeys = nil
	} else {
		filtered := make([]string, 0, len(authKeys))
		for _, authKey := range authKeys {
			if authKey != targetAuthKey {
				filtered = append(filtered, authKey)
			}
		}
		authKeys = filtered
	}

	authKeysValue, err := encodeExternalTopupAuthKeys(authKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalTopupAuthKey", authKeysValue); err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := len(authKeys) > 0
	if !enabled {
		if err := model.UpdateOption("ExternalTopupEnabled", "false"); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if enabled {
		common.OptionMapRWMutex.RLock()
		enabled = common.OptionMap["ExternalTopupEnabled"] == "true"
		common.OptionMapRWMutex.RUnlock()
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalTopupAuthCodeResponse(enabled, authKeys),
	})
}

func DeleteAllExternalTopupAuthCodes(c *gin.Context) {
	if err := model.UpdateOption("ExternalTopupAuthKey", ""); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("ExternalTopupEnabled", "false"); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    externalTopupAuthCodeResponse(false, nil),
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	if option.Key == "ExternalRegisterAuthKey" || option.Key == "ExternalTopupAuthKey" {
		common.ApiErrorMsg(c, "请使用专用接口管理外部鉴权码")
		return
	}
	if strings.HasPrefix(option.Key, "violation_setting.") {
		if err := validateViolationOptionUpdate(option.Key, option.Value.(string)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if strings.HasPrefix(option.Key, "async_task_setting.") {
		normalized, err := validateAndNormalizeAsyncTaskOptionUpdate(option.Key, option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		option.Value = normalized
	}
	if strings.HasPrefix(option.Key, "error_snapshot.") {
		configKey := strings.TrimPrefix(option.Key, "error_snapshot.")
		normalized, normalizeErr := error_snapshot_setting.NormalizeOptionValue(configKey, option.Value.(string))
		if normalizeErr != nil {
			common.ApiErrorMsg(c, normalizeErr.Error())
			return
		}
		option.Value = normalized
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "relay_error_setting.passthrough_status_codes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "claude.request_schema_validation_mode",
		"claude.tool_protocol_validation_mode",
		"claude.tool_schema_validation_mode",
		"claude.tool_choice_validation_mode",
		"claude.thinking_validation_mode",
		"claude.image_limits_validation_mode",
		"claude.prompt_cache_validation_mode",
		"claude.stop_sequences_validation_mode",
		"claude.service_tier_validation_mode",
		"claude.metadata_user_id_validation_mode",
		"claude.assistant_prefill_validation_mode":
		if err = model_setting.ValidateClaudeValidationMode(option.Value.(string)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "claude.request_size_limit_bytes":
		intValue, parseErr := strconv.ParseInt(option.Value.(string), 10, 64)
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Claude 请求体限制必须大于 0",
			})
			return
		}
	case "claude.response_integrity_first_block_timeout_seconds":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue < 1 || intValue > 300 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Claude 首内容块超时必须在 1 到 300 秒之间",
			})
			return
		}
	case "aggregate_group.smart_strategy_enabled":
		_, err = strconv.ParseBool(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "聚合分组智能策略开关格式无效",
			})
			return
		}
	case "aggregate_group.consecutive_failure_threshold":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "连续失败阈值必须大于 0",
			})
			return
		}
	case "aggregate_group.degrade_duration_seconds":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "临时降级时长必须大于 0",
			})
			return
		}
	case "aggregate_group.cluster_degraded_weight_percent":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 || intValue > 100 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cluster 降级有效权重比例必须在 1 到 100 之间",
			})
			return
		}
	case "aggregate_group.slow_request_threshold_seconds":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "慢请求阈值必须大于 0",
			})
			return
		}
	case "aggregate_group.slow_first_response_threshold_seconds":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "首字慢阈值必须大于等于 0",
			})
			return
		}
	case "aggregate_group.consecutive_slow_threshold":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "连续慢请求阈值必须大于 0",
			})
			return
		}
	case "aggregate_group.failure_rate_window_seconds", "aggregate_group.slow_rate_window_seconds":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 || intValue > setting.MaxAggregateGroupRateWindowSeconds {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "统计窗口必须在 1 到 3600 秒之间",
			})
			return
		}
	case "aggregate_group.failure_rate_min_requests", "aggregate_group.slow_rate_min_requests":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "最小样本数必须大于 0",
			})
			return
		}
	case "aggregate_group.failure_rate_threshold_percent", "aggregate_group.slow_rate_threshold_percent":
		intValue, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || intValue <= 0 || intValue > 100 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "百分比阈值必须在 1 到 100 之间",
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func validateViolationOptionUpdate(key string, value string) error {
	configKey := strings.TrimPrefix(key, "violation_setting.")
	next := *operation_setting.GetViolationSetting()
	switch configKey {
	case "enabled":
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		next.Enabled = boolValue
	case "keywords":
		next.Keywords = value
	case "case_sensitive":
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		next.CaseSensitive = boolValue
	case "action":
		next.Action = value
	case "http_status_code":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		next.HTTPStatusCode = intValue
	case "error_code":
		next.ErrorCode = value
	case "error_message":
		next.ErrorMessage = value
	case "max_excerpt_length":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		next.MaxExcerptLength = intValue
	case "ban_threshold":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		next.BanThreshold = intValue
	default:
		return fmt.Errorf("unknown violation setting key")
	}
	return operation_setting.ValidateViolationSetting(next)
}

func validateAndNormalizeAsyncTaskOptionUpdate(key string, value string) (string, error) {
	configKey := strings.TrimPrefix(key, "async_task_setting.")
	switch configKey {
	case "default_timeout_minutes":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(async_task_setting.NormalizeDefaultTimeoutMinutes(intValue)), nil
	case "query_limit":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(async_task_setting.NormalizeQueryLimit(intValue)), nil
	case "webhook_max_attempts":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(async_task_setting.NormalizeWebhookMaxAttempts(intValue)), nil
	case "webhook_retry_interval_seconds":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(async_task_setting.NormalizeWebhookRetryIntervalSeconds(intValue)), nil
	case "timeout_overrides":
		var overrides []async_task_setting.TimeoutOverride
		if strings.TrimSpace(value) == "" {
			value = "[]"
		}
		if err := common.UnmarshalJsonStr(value, &overrides); err != nil {
			return "", err
		}
		normalized := async_task_setting.NormalizeSetting(async_task_setting.AsyncTaskSetting{
			DefaultTimeoutMinutes: async_task_setting.GetAsyncTaskSetting().DefaultTimeoutMinutes,
			QueryLimit:            async_task_setting.GetAsyncTaskSetting().QueryLimit,
			TimeoutOverrides:      overrides,
		}).TimeoutOverrides
		data, err := common.Marshal(normalized)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return value, nil
	}
}
