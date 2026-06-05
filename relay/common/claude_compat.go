package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
)

const (
	ClaudeCompatCodeImageMediaTypeMismatch       = "claude_image_media_type_mismatch"
	ClaudeCompatCodeInvalidImageBase64           = "claude_invalid_image_base64"
	ClaudeCompatCodeUnknownImageMediaType        = "claude_unknown_image_media_type"
	ClaudeCompatCodeZeroMaxTokensIncompatible    = "claude_zero_max_tokens_incompatible"
	ClaudeCompatCodeUnsupportedSamplingParameter = "claude_unsupported_sampling_parameter"
	ClaudeCompatCodeInvalidOutputEffort          = "claude_invalid_output_effort"
	ClaudeCompatCodeInvalidToolResultOrder       = "claude_invalid_tool_result_order"
	ClaudeCompatCodeToolResultMismatch           = "claude_tool_result_mismatch"
	ClaudeCompatCodeInvalidNamePattern           = "claude_invalid_name_pattern"
	ClaudeCompatCodeInvalidRequestSchema         = "claude_invalid_request_schema"
	ClaudeCompatCodeRequestTooLarge              = "claude_request_too_large"
	ClaudeCompatCodeDuplicateToolUseID           = "claude_duplicate_tool_use_id"
	ClaudeCompatCodeMissingToolResult            = "claude_missing_tool_result"
	ClaudeCompatCodeInvalidToolSchema            = "claude_invalid_tool_schema"
	ClaudeCompatCodeInvalidToolChoice            = "claude_invalid_tool_choice"
	ClaudeCompatCodeInvalidThinking              = "claude_invalid_thinking"
	ClaudeCompatCodeImageLimitExceeded           = "claude_image_limit_exceeded"
	ClaudeCompatCodeInvalidPromptCache           = "claude_invalid_prompt_cache"
	ClaudeCompatCodeInvalidStopSequences         = "claude_invalid_stop_sequences"
	ClaudeCompatCodeInvalidServiceTier           = "claude_invalid_service_tier"
	ClaudeCompatCodeMetadataUserIDPII            = "claude_metadata_user_id_pii"
)

var claudeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var obviousPIIPattern = regexp.MustCompile(`(?i)([a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}|\+?\d[\d\s\-()]{8,}\d)`)

const (
	claudeCompatRiskLow    = "low"
	claudeCompatRiskMedium = "medium"
	claudeCompatRiskHigh   = "high"

	claudeCompatMaxMessages            = 100000
	claudeCompatClaudeImageMaxBytes    = 10 * 1024 * 1024
	claudeCompatBedrockImageMaxBytes   = 5 * 1024 * 1024
	claudeCompatImageMaxPixelsPerSide  = 8000
	claudeCompatImageManyThreshold     = 20
	claudeCompatImageManyMaxPixelsSide = 2000
	claudeCompatContextModelImageLimit = 100
	claudeCompatDefaultImageLimit      = 600
)

type claudeCompatViolation struct {
	param     string
	code      string
	message   string
	status    int
	riskLevel string
}

func (v claudeCompatViolation) Error() string {
	return v.message
}

func NewClaudeCompatAPIError(param, code, message string) *types.NewAPIError {
	return newClaudeCompatAPIErrorWithStatus(param, code, message, http.StatusBadRequest)
}

func newClaudeCompatAPIErrorWithStatus(param, code, message string, status int) *types.NewAPIError {
	if status == 0 {
		status = http.StatusBadRequest
	}
	openAIError := types.OpenAIError{
		Message: message,
		Type:    "invalid_request_error",
		Param:   param,
		Code:    code,
	}
	if status == http.StatusRequestEntityTooLarge {
		openAIError.Type = "request_too_large"
	}
	return types.WithOpenAIError(openAIError, status, types.ErrOptionWithSkipRetry())
}

func NormalizeClaudeRequestCompat(request *dto.ClaudeRequest, info *RelayInfo) *types.NewAPIError {
	if request == nil {
		return NewClaudeCompatAPIError("", string(types.ErrorCodeInvalidRequest), "Invalid request for Claude: request body is empty.")
	}
	if err := normalizeClaudeRequestCompat(request, info); err != nil {
		return newClaudeCompatAPIErrorWithStatus(err.param, err.code, err.message, err.status)
	}
	return nil
}

func ValidateClaudeRequestSchemaJSON(jsonData []byte, info *RelayInfo) *types.NewAPIError {
	if len(bytes.TrimSpace(jsonData)) == 0 {
		return NewClaudeCompatAPIError("", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: request body must be a JSON object.")
	}
	payload, apiErr := parseClaudeRawPayload(jsonData)
	if apiErr != nil {
		return apiErr
	}
	settings := model_setting.GetClaudeSettings()
	if err := applyClaudeCompatValidation(settings.RequestSchemaValidationMode, "request_schema", info, validateClaudeRawRequestSchema(payload, info)); err != nil {
		return newClaudeCompatAPIErrorWithStatus(err.param, err.code, err.message, err.status)
	}
	return nil
}

func NormalizeClaudeRequestCompatJSON(jsonData []byte, info *RelayInfo) ([]byte, *types.NewAPIError) {
	if len(jsonData) == 0 {
		return jsonData, nil
	}
	payload, apiErr := parseClaudeRawPayload(jsonData)
	if apiErr != nil {
		return nil, apiErr
	}
	if normalizeOpenAIStyleClaudeRawMessages(payload, info) {
		normalizedJSON, err := common.Marshal(payload)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData = normalizedJSON
	}
	if err := applyRawClaudeCompatValidations(payload, info); err != nil {
		return nil, newClaudeCompatAPIErrorWithStatus(err.param, err.code, err.message, err.status)
	}
	var request dto.ClaudeRequest
	if err := common.Unmarshal(jsonData, &request); err != nil {
		return nil, NewClaudeCompatAPIError("", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: body does not match Claude Messages API schema: %s", err.Error()))
	}
	if apiErr := NormalizeClaudeRequestCompat(&request, info); apiErr != nil {
		return nil, apiErr
	}
	normalizedData, err := common.Marshal(request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	var normalizedPayload map[string]any
	if err := common.Unmarshal(normalizedData, &normalizedPayload); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadRequestBody, types.ErrOptionWithSkipRetry())
	}
	copyClaudeCompatJSONField(payload, normalizedPayload, "max_tokens")
	copyClaudeCompatJSONField(payload, normalizedPayload, "messages")
	copyClaudeCompatJSONField(payload, normalizedPayload, "temperature")
	copyClaudeCompatJSONField(payload, normalizedPayload, "top_p")
	copyClaudeCompatJSONField(payload, normalizedPayload, "top_k")
	copyClaudeCompatJSONField(payload, normalizedPayload, "output_config")
	copyClaudeCompatJSONField(payload, normalizedPayload, "tools")
	copyClaudeCompatJSONField(payload, normalizedPayload, "tool_choice")
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	return data, nil
}

func parseClaudeRawPayload(jsonData []byte) (map[string]any, *types.NewAPIError) {
	if err := validateClaudeRequestSize(len(jsonData)); err != nil {
		return nil, newClaudeCompatAPIErrorWithStatus(err.param, err.code, err.message, err.status)
	}
	var rawPayload any
	if err := common.Unmarshal(jsonData, &rawPayload); err != nil {
		return nil, NewClaudeCompatAPIError("", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: body is not valid JSON: %s", err.Error()))
	}
	payload, ok := rawPayload.(map[string]any)
	if !ok {
		return nil, NewClaudeCompatAPIError("", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: request body must be a JSON object.")
	}
	return payload, nil
}

func copyClaudeCompatJSONField(payload map[string]any, normalizedPayload map[string]any, field string) {
	if value, ok := normalizedPayload[field]; ok {
		payload[field] = value
		return
	}
	delete(payload, field)
}

func applyRawClaudeCompatValidations(payload map[string]any, info *RelayInfo) *claudeCompatViolation {
	settings := model_setting.GetClaudeSettings()
	if err := applyClaudeCompatValidation(settings.RequestSchemaValidationMode, "request_schema", info, validateClaudeRawRequestSchema(payload, info)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.StopSequencesValidationMode, "stop_sequences", info, validateClaudeRawStopSequences(payload)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.ServiceTierValidationMode, "service_tier", info, validateClaudeRawServiceTier(payload)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.ToolSchemaValidationMode, "tool_schema", info, validateClaudeRawToolSchemas(payload)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.ToolChoiceValidationMode, "tool_choice", info, validateClaudeRawToolChoice(payload)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.PromptCacheValidationMode, "prompt_cache", info, validateClaudeRawPromptCache(payload)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.MetadataUserIDValidationMode, "metadata_user_id", info, validateClaudeRawMetadataUserID(payload)); err != nil {
		return err
	}
	return nil
}

func applyClaudeCompatValidation(mode, group string, info *RelayInfo, violation *claudeCompatViolation) *claudeCompatViolation {
	if violation == nil {
		return nil
	}
	if violation.status == 0 {
		violation.status = http.StatusBadRequest
	}
	if violation.riskLevel == "" {
		violation.riskLevel = claudeCompatRiskLow
	}
	normalizedMode := strings.TrimSpace(strings.ToLower(mode))
	switch normalizedMode {
	case model_setting.ClaudeValidationModeOff:
		return nil
	case model_setting.ClaudeValidationModeLog:
		logClaudeCompatViolation(group, info, violation)
		return nil
	default:
		return violation
	}
}

func logClaudeCompatViolation(group string, info *RelayInfo, violation *claudeCompatViolation) {
	if violation == nil {
		return
	}
	detail := map[string]any{
		"group":      group,
		"status":     violation.status,
		"param":      violation.param,
		"code":       violation.code,
		"message":    common.MaskSensitiveInfo(limitClaudeCompatLogMessage(violation.message)),
		"risk_level": violation.riskLevel,
	}
	if info != nil {
		detail["channel_id"] = info.ChannelId
		detail["channel_type"] = info.ChannelType
		detail["user_id"] = info.UserId
		if info.UpstreamModelName != "" {
			detail["model"] = info.UpstreamModelName
		} else if info.OriginModelName != "" {
			detail["model"] = info.OriginModelName
		}
	}
	body, err := common.Marshal(detail)
	if err != nil {
		return
	}
	logger.LogWarn(context.Background(), "claude compat validation: "+string(body))
}

func limitClaudeCompatLogMessage(message string) string {
	const maxLen = 512
	if len(message) <= maxLen {
		return message
	}
	return message[:maxLen] + "..."
}

func newClaudeCompatViolation(param, code, message string) *claudeCompatViolation {
	return &claudeCompatViolation{
		param:     param,
		code:      code,
		message:   message,
		status:    http.StatusBadRequest,
		riskLevel: claudeCompatRiskLow,
	}
}

func validateClaudeRequestSize(size int) *claudeCompatViolation {
	settings := model_setting.GetClaudeSettings()
	limit := settings.RequestSizeLimitBytes
	if limit <= 0 || int64(size) <= limit {
		return nil
	}
	return &claudeCompatViolation{
		param:     "",
		code:      ClaudeCompatCodeRequestTooLarge,
		message:   fmt.Sprintf("Invalid request for Claude: request body exceeds the %d byte Messages API limit.", limit),
		status:    http.StatusRequestEntityTooLarge,
		riskLevel: claudeCompatRiskLow,
	}
}

func validateClaudeRawRequestSchema(payload map[string]any, info *RelayInfo) *claudeCompatViolation {
	if strings.TrimSpace(common.Interface2String(payload["model"])) == "" && effectiveClaudeModelName(nil, info) == "" {
		return newClaudeCompatViolation("model", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: model is required.")
	}
	messagesValue, ok := payload["messages"]
	if !ok {
		return newClaudeCompatViolation("messages", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: messages is required and must be an array.")
	}
	messages, ok := messagesValue.([]any)
	if !ok {
		return newClaudeCompatViolation("messages", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: messages is required and must be an array.")
	}
	if len(messages) > claudeCompatMaxMessages {
		return newClaudeCompatViolation("messages", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: messages may contain at most %d items.", claudeCompatMaxMessages))
	}
	if maxTokensValue, ok := payload["max_tokens"]; ok {
		if err := validateClaudeRawNonNegativeInteger("max_tokens", maxTokensValue); err != nil {
			return err
		}
	}
	if systemValue, ok := payload["system"]; ok {
		if err := validateClaudeRawSystem(systemValue); err != nil {
			return err
		}
	}
	for i, messageValue := range messages {
		message, ok := messageValue.(map[string]any)
		if !ok {
			param := fmt.Sprintf("messages.%d", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be an object.", param))
		}
		role := strings.TrimSpace(common.Interface2String(message["role"]))
		if role != "user" && role != "assistant" {
			param := fmt.Sprintf("messages.%d.role", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be \"user\" or \"assistant\".", param))
		}
		content, ok := message["content"]
		if !ok {
			param := fmt.Sprintf("messages.%d.content", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a string or an array of content blocks.", param))
		}
		if err := validateClaudeRawContent(content, fmt.Sprintf("messages.%d.content", i)); err != nil {
			return err
		}
	}
	return nil
}

func validateClaudeRawNonNegativeInteger(param string, value any) *claudeCompatViolation {
	switch typed := value.(type) {
	case float64:
		if typed < 0 || math.Trunc(typed) != typed {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a non-negative integer.", param))
		}
	case int:
		if typed < 0 {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a non-negative integer.", param))
		}
	case int64:
		if typed < 0 {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a non-negative integer.", param))
		}
	case uint, uint64:
		return nil
	default:
		return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a non-negative integer.", param))
	}
	return nil
}

func validateClaudeRawSystem(value any) *claudeCompatViolation {
	if _, ok := value.(string); ok {
		return nil
	}
	blocks, ok := value.([]any)
	if !ok {
		return newClaudeCompatViolation("system", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: system must be a string or an array of text blocks.")
	}
	for i, blockValue := range blocks {
		block, ok := blockValue.(map[string]any)
		param := fmt.Sprintf("system.%d", i)
		if !ok {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a text block object.", param))
		}
		if common.Interface2String(block["type"]) != "text" {
			return newClaudeCompatViolation(param+".type", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.type must be \"text\".", param))
		}
		if _, ok := block["text"].(string); !ok {
			return newClaudeCompatViolation(param+".text", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.text is required for text blocks.", param))
		}
	}
	return nil
}

func validateClaudeRawContent(value any, param string) *claudeCompatViolation {
	if _, ok := value.(string); ok {
		return nil
	}
	blocks, ok := value.([]any)
	if !ok {
		return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a string or an array of content blocks.", param))
	}
	if len(blocks) == 0 {
		return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must not be an empty array.", param))
	}
	for i, blockValue := range blocks {
		block, ok := blockValue.(map[string]any)
		blockParam := fmt.Sprintf("%s.%d", param, i)
		if !ok {
			return newClaudeCompatViolation(blockParam, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be an object.", blockParam))
		}
		blockType := strings.TrimSpace(common.Interface2String(block["type"]))
		if !isSupportedClaudeContentBlockType(blockType) {
			return newClaudeCompatViolation(blockParam+".type", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.type is not supported by Claude Messages API.", blockParam))
		}
		if blockType == "text" {
			if _, ok := block["text"].(string); !ok {
				return newClaudeCompatViolation(blockParam+".text", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.text is required for text blocks.", blockParam))
			}
		}
	}
	return nil
}

func isSupportedClaudeContentBlockType(blockType string) bool {
	switch blockType {
	case "text", "image", "document", "search_result", "thinking", "redacted_thinking", "tool_use", "tool_result", "server_tool_use", "web_search_tool_result", "code_execution_tool_result", "mcp_tool_use", "mcp_tool_result", "container_upload":
		return true
	default:
		return false
	}
}

func validateClaudeRawStopSequences(payload map[string]any) *claudeCompatViolation {
	value, ok := payload["stop_sequences"]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return newClaudeCompatViolation("stop_sequences", ClaudeCompatCodeInvalidStopSequences, "Invalid request for Claude: stop_sequences must be an array of non-empty strings.")
	}
	for i, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			param := fmt.Sprintf("stop_sequences.%d", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidStopSequences, fmt.Sprintf("Invalid request for Claude: %s must be a non-empty string.", param))
		}
	}
	return nil
}

func validateClaudeRawServiceTier(payload map[string]any) *claudeCompatViolation {
	value, ok := payload["service_tier"]
	if !ok || value == nil {
		return nil
	}
	serviceTier, ok := value.(string)
	if !ok {
		return newClaudeCompatViolation("service_tier", ClaudeCompatCodeInvalidServiceTier, "Invalid request for Claude: service_tier must be auto or standard_only.")
	}
	switch serviceTier {
	case "auto", "standard_only":
		return nil
	default:
		return newClaudeCompatViolation("service_tier", ClaudeCompatCodeInvalidServiceTier, "Invalid request for Claude: service_tier must be auto or standard_only.")
	}
}

func validateClaudeRawToolSchemas(payload map[string]any) *claudeCompatViolation {
	value, ok := payload["tools"]
	if !ok || value == nil {
		return nil
	}
	tools, ok := value.([]any)
	if !ok {
		return mediumRiskClaudeViolation("tools", ClaudeCompatCodeInvalidToolSchema, "Invalid request for Claude: tools must be an array of tool definitions.")
	}
	for i, toolValue := range tools {
		tool, ok := toolValue.(map[string]any)
		param := fmt.Sprintf("tools.%d", i)
		if !ok {
			return mediumRiskClaudeViolation(param, ClaudeCompatCodeInvalidToolSchema, fmt.Sprintf("Invalid request for Claude: %s must be an object.", param))
		}
		if isClaudeBuiltInToolName(common.Interface2String(tool["name"])) {
			continue
		}
		inputSchemaValue, ok := tool["input_schema"]
		if !ok {
			return mediumRiskClaudeViolation(param+".input_schema", ClaudeCompatCodeInvalidToolSchema, fmt.Sprintf("Invalid request for Claude: %s.input_schema must be a JSON Schema object.", param))
		}
		inputSchema, ok := inputSchemaValue.(map[string]any)
		if !ok {
			return mediumRiskClaudeViolation(param+".input_schema", ClaudeCompatCodeInvalidToolSchema, fmt.Sprintf("Invalid request for Claude: %s.input_schema must be a JSON Schema object.", param))
		}
		schemaType := common.Interface2String(inputSchema["type"])
		if schemaType != "" && schemaType != "object" {
			return mediumRiskClaudeViolation(param+".input_schema.type", ClaudeCompatCodeInvalidToolSchema, fmt.Sprintf("Invalid request for Claude: %s.input_schema.type must be \"object\" for user-defined tools.", param))
		}
		if examples, ok := tool["input_examples"]; ok && examples != nil {
			if _, ok := examples.([]any); !ok {
				return mediumRiskClaudeViolation(param+".input_examples", ClaudeCompatCodeInvalidToolSchema, fmt.Sprintf("Invalid request for Claude: %s.input_examples must be an array when provided.", param))
			}
		}
	}
	return nil
}

func validateClaudeRawToolChoice(payload map[string]any) *claudeCompatViolation {
	value, ok := payload["tool_choice"]
	if !ok || value == nil {
		return nil
	}
	choice, ok := value.(map[string]any)
	if !ok {
		return mediumRiskClaudeViolation("tool_choice", ClaudeCompatCodeInvalidToolChoice, "Invalid request for Claude: tool_choice must be an object.")
	}
	choiceType := common.Interface2String(choice["type"])
	switch choiceType {
	case "auto", "any", "tool", "none":
	default:
		return mediumRiskClaudeViolation("tool_choice.type", ClaudeCompatCodeInvalidToolChoice, "Invalid request for Claude: tool_choice.type must be one of auto, any, tool, none.")
	}
	if choiceType == "tool" {
		name := strings.TrimSpace(common.Interface2String(choice["name"]))
		if name == "" {
			return mediumRiskClaudeViolation("tool_choice.name", ClaudeCompatCodeInvalidToolChoice, "Invalid request for Claude: tool_choice.name is required when tool_choice.type is tool.")
		}
		toolNames := claudeRawToolNames(payload["tools"])
		if len(toolNames) > 0 {
			if _, ok := toolNames[name]; !ok {
				return mediumRiskClaudeViolation("tool_choice.name", ClaudeCompatCodeInvalidToolChoice, "Invalid request for Claude: tool_choice.name must reference one of the provided tools.")
			}
		}
	}
	thinkingEnabled := claudeRawThinkingEnabled(payload["thinking"])
	if thinkingEnabled && isForcedClaudeToolChoice(choiceType) {
		return mediumRiskClaudeViolation("tool_choice.type", ClaudeCompatCodeInvalidToolChoice, "Invalid request for Claude: forced tool use is not supported when thinking is enabled; use auto or none.")
	}
	return nil
}

func claudeRawToolNames(value any) map[string]struct{} {
	tools, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make(map[string]struct{}, len(tools))
	for _, toolValue := range tools {
		tool, ok := toolValue.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(common.Interface2String(tool["name"]))
		if name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

func validateClaudeRawPromptCache(payload map[string]any) *claudeCompatViolation {
	totalBreakpoints := 0
	lastBlockTTL := ""
	if systemValue, ok := payload["system"]; ok {
		count, lastTTL, violation := countClaudeRawCacheControls(systemValue, "system")
		if violation != nil {
			return violation
		}
		totalBreakpoints += count
		if lastTTL != "" {
			lastBlockTTL = lastTTL
		}
	}
	messages, _ := payload["messages"].([]any)
	for i, messageValue := range messages {
		message, ok := messageValue.(map[string]any)
		if !ok {
			continue
		}
		count, lastTTL, violation := countClaudeRawCacheControls(message["content"], fmt.Sprintf("messages.%d.content", i))
		if violation != nil {
			return violation
		}
		totalBreakpoints += count
		if lastTTL != "" {
			lastBlockTTL = lastTTL
		}
	}
	if totalBreakpoints > 4 {
		return mediumRiskClaudeViolation("cache_control", ClaudeCompatCodeInvalidPromptCache, "Invalid request for Claude: at most 4 cache_control breakpoints are allowed.")
	}
	topLevelTTL := claudeRawCacheControlTTL(payload["cache_control"])
	if topLevelTTL != "" {
		if !isAllowedClaudeCacheTTL(topLevelTTL) {
			return mediumRiskClaudeViolation("cache_control.ttl", ClaudeCompatCodeInvalidPromptCache, "Invalid request for Claude: cache_control.ttl must be \"5m\" or \"1h\".")
		}
		if lastBlockTTL != "" && lastBlockTTL != topLevelTTL {
			return mediumRiskClaudeViolation("cache_control.ttl", ClaudeCompatCodeInvalidPromptCache, "Invalid request for Claude: top-level cache_control conflicts with the last block cache_control ttl.")
		}
		if totalBreakpoints >= 4 {
			return mediumRiskClaudeViolation("cache_control", ClaudeCompatCodeInvalidPromptCache, "Invalid request for Claude: no cache breakpoint slots left for top-level cache_control.")
		}
	}
	return nil
}

func countClaudeRawCacheControls(value any, param string) (int, string, *claudeCompatViolation) {
	blocks, ok := value.([]any)
	if !ok {
		return 0, "", nil
	}
	count := 0
	lastTTL := ""
	for i, blockValue := range blocks {
		block, ok := blockValue.(map[string]any)
		if !ok {
			continue
		}
		cacheControl, ok := block["cache_control"]
		if !ok || cacheControl == nil {
			continue
		}
		count++
		ttl := claudeRawCacheControlTTL(cacheControl)
		cacheParam := fmt.Sprintf("%s.%d.cache_control.ttl", param, i)
		if ttl != "" && !isAllowedClaudeCacheTTL(ttl) {
			return count, lastTTL, mediumRiskClaudeViolation(cacheParam, ClaudeCompatCodeInvalidPromptCache, "Invalid request for Claude: cache_control.ttl must be \"5m\" or \"1h\".")
		}
		if ttl != "" {
			lastTTL = ttl
		} else {
			lastTTL = "5m"
		}
	}
	return count, lastTTL, nil
}

func claudeRawCacheControlTTL(value any) string {
	if value == nil {
		return ""
	}
	cacheControl, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(common.Interface2String(cacheControl["ttl"]))
}

func isAllowedClaudeCacheTTL(ttl string) bool {
	return ttl == "5m" || ttl == "1h"
}

func validateClaudeRawMetadataUserID(payload map[string]any) *claudeCompatViolation {
	value, ok := payload["metadata"]
	if !ok || value == nil {
		return nil
	}
	metadata, ok := value.(map[string]any)
	if !ok {
		return mediumRiskClaudeViolation("metadata", ClaudeCompatCodeMetadataUserIDPII, "Invalid request for Claude: metadata must be an object.")
	}
	userIDValue, ok := metadata["user_id"]
	if !ok || userIDValue == nil {
		return nil
	}
	userID, ok := userIDValue.(string)
	if !ok {
		return mediumRiskClaudeViolation("metadata.user_id", ClaudeCompatCodeMetadataUserIDPII, "Invalid request for Claude: metadata.user_id must be an opaque string identifier.")
	}
	if obviousPIIPattern.MatchString(userID) {
		return mediumRiskClaudeViolation("metadata.user_id", ClaudeCompatCodeMetadataUserIDPII, "Invalid request for Claude: metadata.user_id should be an opaque identifier and must not contain obvious PII.")
	}
	return nil
}

func mediumRiskClaudeViolation(param, code, message string) *claudeCompatViolation {
	return &claudeCompatViolation{
		param:     param,
		code:      code,
		message:   message,
		status:    http.StatusBadRequest,
		riskLevel: claudeCompatRiskMedium,
	}
}

func normalizeClaudeRequestCompat(request *dto.ClaudeRequest, info *RelayInfo) *claudeCompatViolation {
	settings := model_setting.GetClaudeSettings()
	modelName := effectiveClaudeModelName(request, info)
	if err := applyClaudeCompatValidation(settings.RequestSchemaValidationMode, "request_schema", info, validateClaudeRequestSchema(request, info)); err != nil {
		return err
	}
	if request.MaxTokens == nil {
		defaultMaxTokens := uint(settings.GetDefaultMaxTokens(modelName))
		request.MaxTokens = &defaultMaxTokens
	} else if !settings.PreserveZeroMaxTokensEnabled && *request.MaxTokens == 0 {
		defaultMaxTokens := uint(settings.GetDefaultMaxTokens(modelName))
		request.MaxTokens = &defaultMaxTokens
	}
	if settings.PreserveZeroMaxTokensEnabled && request.MaxTokens != nil && *request.MaxTokens == 0 {
		if err := validateClaudeZeroMaxTokensCompat(request, info); err != nil {
			return err
		}
	}
	if settings.AutoFixImageMediaTypeEnabled {
		if err := normalizeClaudeImageMediaTypes(request); err != nil {
			return err
		}
	}
	if settings.DropDefaultSamplingForOpusEnabled {
		if err := normalizeClaudeOpusSampling(request, modelName); err != nil {
			return err
		}
	}
	if settings.ValidateOutputEffortEnabled {
		if err := validateClaudeOutputEffort(request, modelName); err != nil {
			return err
		}
	}
	if err := applyClaudeCompatValidation(settings.ThinkingValidationMode, "thinking", info, validateClaudeThinking(request, modelName)); err != nil {
		return err
	}
	if err := applyClaudeCompatValidation(settings.ImageLimitsValidationMode, "image_limits", info, validateClaudeImageLimits(request, info)); err != nil {
		return err
	}
	if err := validateClaudeNames(request); err != nil {
		return err
	}
	if settings.ReorderToolResultBlocksEnabled {
		reorderClaudeToolResults(request)
	}
	if err := applyClaudeCompatValidation(settings.ToolProtocolValidationMode, "tool_protocol", info, validateClaudeToolResults(request, info)); err != nil {
		return err
	}
	return nil
}

func validateClaudeRequestSchema(request *dto.ClaudeRequest, info *RelayInfo) *claudeCompatViolation {
	if strings.TrimSpace(request.Model) == "" && effectiveClaudeModelName(request, info) == "" {
		return newClaudeCompatViolation("model", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: model is required.")
	}
	if request.Messages == nil {
		return newClaudeCompatViolation("messages", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: messages is required and must be an array.")
	}
	if len(request.Messages) > claudeCompatMaxMessages {
		return newClaudeCompatViolation("messages", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: messages may contain at most %d items.", claudeCompatMaxMessages))
	}
	if request.System != nil {
		if _, ok := request.System.(string); !ok {
			systemBlocks := request.ParseSystem()
			if systemBlocks == nil {
				return newClaudeCompatViolation("system", ClaudeCompatCodeInvalidRequestSchema, "Invalid request for Claude: system must be a string or an array of text blocks.")
			}
			for i, block := range systemBlocks {
				if block.Type != "text" {
					param := fmt.Sprintf("system.%d.type", i)
					return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be \"text\".", param))
				}
				if block.Text == nil {
					param := fmt.Sprintf("system.%d.text", i)
					return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s is required for text blocks.", param))
				}
			}
		}
	}
	for i, message := range request.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			param := fmt.Sprintf("messages.%d.role", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be \"user\" or \"assistant\".", param))
		}
		if message.Content == nil {
			param := fmt.Sprintf("messages.%d.content", i)
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a string or an array of content blocks.", param))
		}
		if _, ok := message.Content.(string); ok {
			continue
		}
		contents, ok, err := claudeMessageContents(message.Content)
		param := fmt.Sprintf("messages.%d.content", i)
		if err != nil || !ok {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must be a string or an array of content blocks.", param))
		}
		if len(contents) == 0 {
			return newClaudeCompatViolation(param, ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s must not be an empty array.", param))
		}
		for j, content := range contents {
			blockParam := fmt.Sprintf("%s.%d", param, j)
			if !isSupportedClaudeContentBlockType(content.Type) {
				return newClaudeCompatViolation(blockParam+".type", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.type is not supported by Claude Messages API.", blockParam))
			}
			if content.Type == "text" && content.Text == nil {
				return newClaudeCompatViolation(blockParam+".text", ClaudeCompatCodeInvalidRequestSchema, fmt.Sprintf("Invalid request for Claude: %s.text is required for text blocks.", blockParam))
			}
		}
	}
	return nil
}

func effectiveClaudeModelName(request *dto.ClaudeRequest, info *RelayInfo) string {
	if request != nil && strings.TrimSpace(request.Model) != "" {
		return request.Model
	}
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		return info.UpstreamModelName
	}
	if info != nil && info.ChannelMeta != nil && strings.TrimSpace(info.ChannelMeta.UpstreamModelName) != "" {
		return info.ChannelMeta.UpstreamModelName
	}
	return ""
}

func validateClaudeZeroMaxTokensCompat(request *dto.ClaudeRequest, info *RelayInfo) *claudeCompatViolation {
	conflicts := make([]string, 0, 4)
	if (request.Stream != nil && *request.Stream) || (info != nil && info.IsStream) {
		conflicts = append(conflicts, "stream")
	}
	if request.Thinking != nil && request.Thinking.Type != "" && request.Thinking.Type != "disabled" {
		conflicts = append(conflicts, "thinking")
	}
	if len(request.OutputFormat) > 0 && common.GetJsonType(request.OutputFormat) != "null" {
		conflicts = append(conflicts, "output_format")
	}
	if hasForcedClaudeToolChoice(request.ToolChoice) {
		conflicts = append(conflicts, "tool_choice")
	}
	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	return &claudeCompatViolation{
		param: "max_tokens",
		code:  ClaudeCompatCodeZeroMaxTokensIncompatible,
		message: fmt.Sprintf(
			"Invalid request for Claude: max_tokens=0 is only supported for cache pre-warming and cannot be combined with %s.",
			strings.Join(conflicts, ", "),
		),
	}
}

func validateClaudeThinking(request *dto.ClaudeRequest, modelName string) *claudeCompatViolation {
	if request.Thinking == nil || request.Thinking.Type == "" {
		return nil
	}
	thinkingType := strings.TrimSpace(strings.ToLower(request.Thinking.Type))
	switch thinkingType {
	case "enabled", "disabled", "adaptive":
	default:
		return mediumRiskClaudeViolation("thinking.type", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: thinking.type must be enabled, disabled, or adaptive.")
	}
	if thinkingType == "disabled" {
		return nil
	}
	if thinkingType == "enabled" {
		if request.Thinking.BudgetTokens == nil {
			return mediumRiskClaudeViolation("thinking.budget_tokens", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: thinking.budget_tokens is required when thinking.type is enabled.")
		}
		budgetTokens := request.Thinking.GetBudgetTokens()
		if budgetTokens < 1024 {
			return mediumRiskClaudeViolation("thinking.budget_tokens", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: thinking.budget_tokens must be >= 1024.")
		}
		if request.MaxTokens != nil && budgetTokens >= int(*request.MaxTokens) {
			return mediumRiskClaudeViolation("thinking.budget_tokens", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: thinking.budget_tokens must be less than max_tokens.")
		}
		if isClaudeOpus47Or48(modelName) {
			return mediumRiskClaudeViolation("thinking.type", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: manual extended thinking is not supported for this model; use adaptive thinking.")
		}
	}
	if hasForcedClaudeToolChoice(request.ToolChoice) {
		return mediumRiskClaudeViolation("tool_choice.type", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: forced tool use is not supported when thinking is enabled; use auto or none.")
	}
	if request.Temperature != nil && !isFloatDefault(*request.Temperature, 1.0) {
		return mediumRiskClaudeViolation("temperature", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: temperature is not compatible with thinking.")
	}
	if request.TopK != nil && *request.TopK != 0 && *request.TopK != 250 {
		return mediumRiskClaudeViolation("top_k", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: top_k is not compatible with thinking.")
	}
	if request.TopP != nil && (*request.TopP < 0.95 || *request.TopP > 1.0) {
		return mediumRiskClaudeViolation("top_p", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: top_p must be between 0.95 and 1 when thinking is enabled.")
	}
	if hasClaudeAssistantPrefill(request) {
		return mediumRiskClaudeViolation("messages", ClaudeCompatCodeInvalidThinking, "Invalid request for Claude: prefilling assistant responses is not supported when thinking is enabled.")
	}
	return nil
}

func hasClaudeAssistantPrefill(request *dto.ClaudeRequest) bool {
	if request == nil || len(request.Messages) == 0 {
		return false
	}
	last := request.Messages[len(request.Messages)-1]
	if last.Role != "assistant" {
		return false
	}
	if text, ok := last.Content.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	contents := contentToClaudeMediaMessages(last.Content)
	return len(contents) > 0
}

func claudeRawThinkingEnabled(value any) bool {
	if value == nil {
		return false
	}
	thinking, ok := value.(map[string]any)
	if !ok {
		return false
	}
	thinkingType := strings.TrimSpace(strings.ToLower(common.Interface2String(thinking["type"])))
	return thinkingType != "" && thinkingType != "disabled"
}

func validateClaudeImageLimits(request *dto.ClaudeRequest, info *RelayInfo) *claudeCompatViolation {
	images := collectClaudeBase64Images(request)
	if len(images) == 0 {
		return nil
	}
	imageLimit := claudeImageCountLimit(effectiveClaudeModelName(request, info))
	if len(images) > imageLimit {
		return mediumRiskClaudeViolation("messages", ClaudeCompatCodeImageLimitExceeded, fmt.Sprintf("Invalid request for Claude: too many images; this model supports at most %d images per request.", imageLimit))
	}
	maxBytes := claudeImageMaxBytesForChannel(info)
	for _, item := range images {
		decoded, err := decodeClaudeBase64ImageData(item.data)
		if err != nil {
			continue
		}
		if len(decoded) > maxBytes {
			return mediumRiskClaudeViolation(item.param+".source.data", ClaudeCompatCodeImageLimitExceeded, fmt.Sprintf("Invalid request for Claude: %s exceeds the %d byte image limit.", item.param, maxBytes))
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(decoded))
		if err != nil {
			continue
		}
		if config.Width <= 0 || config.Height <= 0 {
			continue
		}
		sideLimit := claudeCompatImageMaxPixelsPerSide
		if len(images) > claudeCompatImageManyThreshold {
			sideLimit = claudeCompatImageManyMaxPixelsSide
		}
		if config.Width > sideLimit || config.Height > sideLimit {
			return mediumRiskClaudeViolation(item.param+".source.data", ClaudeCompatCodeImageLimitExceeded, fmt.Sprintf("Invalid request for Claude: %s dimensions exceed %dx%d.", item.param, sideLimit, sideLimit))
		}
	}
	return nil
}

type claudeBase64ImageRef struct {
	param string
	data  string
}

func collectClaudeBase64Images(request *dto.ClaudeRequest) []claudeBase64ImageRef {
	if request == nil {
		return nil
	}
	var images []claudeBase64ImageRef
	for messageIndex, message := range request.Messages {
		contents, ok, err := claudeMessageContents(message.Content)
		if err != nil || !ok {
			continue
		}
		for contentIndex, content := range contents {
			if content.Type != "image" || content.Source == nil || content.Source.Type != "base64" {
				continue
			}
			images = append(images, claudeBase64ImageRef{
				param: fmt.Sprintf("messages.%d.content.%d", messageIndex, contentIndex),
				data:  common.Interface2String(content.Source.Data),
			})
		}
	}
	return images
}

func decodeClaudeBase64ImageData(data string) ([]byte, error) {
	data = strings.TrimSpace(data)
	if comma := strings.IndexByte(data, ','); comma >= 0 && strings.Contains(strings.ToLower(data[:comma]), "base64") {
		data = data[comma+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(data)
	}
	return decoded, err
}

func claudeImageMaxBytesForChannel(info *RelayInfo) int {
	if info != nil && info.ChannelMeta != nil {
		switch info.ChannelType {
		case constant.ChannelTypeAws, constant.ChannelTypeVertexAi:
			return claudeCompatBedrockImageMaxBytes
		}
	}
	return claudeCompatClaudeImageMaxBytes
}

func claudeImageCountLimit(modelName string) int {
	modelName = strings.ToLower(modelName)
	if strings.Contains(modelName, "200k") {
		return claudeCompatContextModelImageLimit
	}
	return claudeCompatDefaultImageLimit
}

func hasForcedClaudeToolChoice(toolChoice any) bool {
	if toolChoice == nil {
		return false
	}
	switch choice := toolChoice.(type) {
	case dto.ClaudeToolChoice:
		return isForcedClaudeToolChoice(choice.Type)
	case *dto.ClaudeToolChoice:
		return choice != nil && isForcedClaudeToolChoice(choice.Type)
	case map[string]any:
		return isForcedClaudeToolChoice(common.Interface2String(choice["type"]))
	default:
		var parsedChoice dto.ClaudeToolChoice
		if err := anyToType(toolChoice, &parsedChoice); err != nil {
			return false
		}
		return isForcedClaudeToolChoice(parsedChoice.Type)
	}
}

func isForcedClaudeToolChoice(choiceType string) bool {
	switch strings.ToLower(strings.TrimSpace(choiceType)) {
	case "tool", "any":
		return true
	default:
		return false
	}
}

func normalizeClaudeImageMediaTypes(request *dto.ClaudeRequest) *claudeCompatViolation {
	for messageIndex := range request.Messages {
		message := &request.Messages[messageIndex]
		contents, ok, err := claudeMessageContents(message.Content)
		if err != nil || !ok {
			continue
		}
		changed := false
		for contentIndex := range contents {
			content := &contents[contentIndex]
			if content.Type != "image" || content.Source == nil || content.Source.Type != "base64" {
				continue
			}
			dataParam := fmt.Sprintf("messages.%d.content.%d.source.data", messageIndex, contentIndex)
			data := common.Interface2String(content.Source.Data)
			mediaType, violation := sniffClaudeImageMediaType(data, dataParam)
			if violation != nil {
				return violation
			}
			declared := strings.TrimSpace(strings.ToLower(content.Source.MediaType))
			if declared == "" || declared == mediaType {
				if declared == "" {
					content.Source.MediaType = mediaType
					changed = true
				}
				continue
			}
			content.Source.MediaType = mediaType
			changed = true
		}
		if changed {
			message.SetContent(contents)
		}
	}
	return nil
}

func sniffClaudeImageMediaType(data string, param string) (string, *claudeCompatViolation) {
	data = strings.TrimSpace(data)
	if comma := strings.IndexByte(data, ','); comma >= 0 && strings.Contains(strings.ToLower(data[:comma]), "base64") {
		data = data[comma+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(data)
	}
	if err != nil {
		return "", &claudeCompatViolation{
			param:   param,
			code:    ClaudeCompatCodeInvalidImageBase64,
			message: fmt.Sprintf("Invalid request for Claude: %s is not valid base64 image data.", param),
		}
	}
	mediaType := sniffImageMagicBytes(decoded)
	if mediaType == "" {
		return "", &claudeCompatViolation{
			param:   param,
			code:    ClaudeCompatCodeUnknownImageMediaType,
			message: fmt.Sprintf("Invalid request for Claude: %s is not a supported image format; supported formats are JPEG, PNG, GIF, and WebP.", param),
		}
	}
	return mediaType, nil
}

func sniffImageMagicBytes(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 &&
		data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a {
		return "image/png"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

func normalizeClaudeOpusSampling(request *dto.ClaudeRequest, modelName string) *claudeCompatViolation {
	if !isClaudeOpus47OrLater(modelName) {
		return nil
	}
	if request.Temperature != nil {
		if isFloatDefault(*request.Temperature, 1.0) {
			request.Temperature = nil
		} else {
			return unsupportedSamplingViolation("temperature", *request.Temperature)
		}
	}
	if request.TopP != nil {
		if isClaudeOpusCompatibleTopP(*request.TopP) {
			request.TopP = nil
		} else {
			return unsupportedSamplingViolation("top_p", *request.TopP)
		}
	}
	if request.TopK != nil {
		return unsupportedSamplingViolation("top_k", *request.TopK)
	}
	return nil
}

func unsupportedSamplingViolation(param string, value any) *claudeCompatViolation {
	return &claudeCompatViolation{
		param: param,
		code:  ClaudeCompatCodeUnsupportedSamplingParameter,
		message: fmt.Sprintf(
			"Invalid request for Claude: %s=%v is not supported for this Opus model; remove %s or use the provider default.",
			param,
			value,
			param,
		),
	}
}

func isFloatDefault(value float64, expected float64) bool {
	return math.Abs(value-expected) < 0.0000001
}

func isClaudeOpusCompatibleTopP(value float64) bool {
	return value >= 0.99 && value <= 1.0
}

func isClaudeOpus47OrLater(model string) bool {
	model = strings.ToLower(model)
	if !strings.Contains(model, "claude-opus-4") {
		return false
	}
	return strings.Contains(model, "claude-opus-4-7") || strings.Contains(model, "claude-opus-4-8")
}

func validateClaudeOutputEffort(request *dto.ClaudeRequest, modelName string) *claudeCompatViolation {
	effort := strings.TrimSpace(strings.ToLower(request.GetEfforts()))
	if effort == "" {
		return nil
	}
	switch effort {
	case "low", "medium", "high":
		return nil
	case "xhigh":
		if isClaudeOpus47Or48(modelName) {
			return nil
		}
	case "max":
		if supportsClaudeMaxEffort(modelName) {
			return nil
		}
	default:
		return &claudeCompatViolation{
			param: "output_config.effort",
			code:  ClaudeCompatCodeInvalidOutputEffort,
			message: fmt.Sprintf(
				"Invalid request for Claude: output_config.effort=%q is unsupported; allowed values are low, medium, high, xhigh, and max.",
				effort,
			),
		}
	}
	return &claudeCompatViolation{
		param: "output_config.effort",
		code:  ClaudeCompatCodeInvalidOutputEffort,
		message: fmt.Sprintf(
			"Invalid request for Claude: output_config.effort=%q is not supported by model %q.",
			effort,
			modelName,
		),
	}
}

func isClaudeOpus47Or48(model string) bool {
	model = strings.ToLower(model)
	return strings.Contains(model, "claude-opus-4-7") || strings.Contains(model, "claude-opus-4-8")
}

func supportsClaudeMaxEffort(model string) bool {
	model = strings.ToLower(model)
	return strings.Contains(model, "claude-opus-4-6") ||
		strings.Contains(model, "claude-opus-4-7") ||
		strings.Contains(model, "claude-opus-4-8") ||
		strings.Contains(model, "claude-sonnet-4-7")
}

func validateClaudeNames(request *dto.ClaudeRequest) *claudeCompatViolation {
	for i, tool := range claudeToolsAsMaps(request.Tools) {
		name := common.Interface2String(tool["name"])
		if name == "" || isClaudeBuiltInToolName(name) {
			continue
		}
		if !claudeNamePattern.MatchString(name) {
			param := fmt.Sprintf("tools.%d.name", i)
			return &claudeCompatViolation{
				param:   param,
				code:    ClaudeCompatCodeInvalidNamePattern,
				message: fmt.Sprintf("Invalid request for Claude: %s must match ^[a-zA-Z0-9_-]{1,64}$.", param),
			}
		}
	}
	for messageIndex, message := range request.Messages {
		contents, ok, err := claudeMessageContents(message.Content)
		if err != nil || !ok {
			continue
		}
		for contentIndex, content := range contents {
			if content.Type != "tool_use" || content.Name == "" {
				continue
			}
			if !claudeNamePattern.MatchString(content.Name) {
				param := fmt.Sprintf("messages.%d.content.%d.name", messageIndex, contentIndex)
				return &claudeCompatViolation{
					param:   param,
					code:    ClaudeCompatCodeInvalidNamePattern,
					message: fmt.Sprintf("Invalid request for Claude: %s must match ^[a-zA-Z0-9_-]{1,64}$.", param),
				}
			}
		}
	}
	return nil
}

func isClaudeBuiltInToolName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "web_search") || strings.HasPrefix(name, "computer_") || strings.HasPrefix(name, "text_editor_")
}

func reorderClaudeToolResults(request *dto.ClaudeRequest) {
	for messageIndex := range request.Messages {
		message := &request.Messages[messageIndex]
		if message.Role != "user" {
			continue
		}
		contents, ok, err := claudeMessageContents(message.Content)
		if err != nil || !ok || len(contents) < 2 {
			continue
		}
		toolResults := make([]dto.ClaudeMediaMessage, 0, len(contents))
		others := make([]dto.ClaudeMediaMessage, 0, len(contents))
		changed := false
		seenOther := false
		for _, content := range contents {
			if content.Type == "tool_result" {
				if seenOther {
					changed = true
				}
				toolResults = append(toolResults, content)
			} else {
				seenOther = true
				others = append(others, content)
			}
		}
		if changed {
			reordered := append(toolResults, others...)
			message.SetContent(reordered)
		}
	}
}

func validateClaudeToolResults(request *dto.ClaudeRequest, info *RelayInfo) *claudeCompatViolation {
	for messageIndex, message := range request.Messages {
		contents, ok, err := claudeMessageContents(message.Content)
		previousToolUseIDs := map[string]struct{}{}
		if messageIndex > 0 {
			previousToolUseIDs = claudeToolUseIDsInMessage(request.Messages[messageIndex-1])
		}
		if err != nil || !ok {
			if len(previousToolUseIDs) > 0 {
				return missingClaudeToolResultViolation(messageIndex, firstClaudeToolUseID(previousToolUseIDs))
			}
			continue
		}
		if message.Role == "assistant" {
			seenToolUseIDs := map[string]struct{}{}
			for contentIndex, content := range contents {
				if content.Type != "tool_use" || strings.TrimSpace(content.Id) == "" {
					continue
				}
				if _, exists := seenToolUseIDs[content.Id]; exists {
					param := fmt.Sprintf("messages.%d.content.%d.id", messageIndex, contentIndex)
					return &claudeCompatViolation{
						param:     param,
						code:      ClaudeCompatCodeDuplicateToolUseID,
						message:   fmt.Sprintf("Invalid request for Claude: %s duplicates tool_use id %q within the message history.", param, content.Id),
						status:    http.StatusBadRequest,
						riskLevel: claudeCompatRiskLow,
					}
				}
				seenToolUseIDs[content.Id] = struct{}{}
			}
		}
		matchedToolUseIDs := map[string]struct{}{}
		seenNonToolResult := false
		for contentIndex, content := range contents {
			switch content.Type {
			case "tool_use":
				continue
			case "tool_result":
				param := fmt.Sprintf("messages.%d.content.%d.tool_use_id", messageIndex, contentIndex)
				if strings.TrimSpace(content.ToolUseId) == "" {
					return &claudeCompatViolation{
						param:     param,
						code:      ClaudeCompatCodeInvalidToolResultOrder,
						message:   fmt.Sprintf("Invalid request for Claude: %s is required for tool_result blocks.", param),
						status:    http.StatusBadRequest,
						riskLevel: claudeCompatRiskLow,
					}
				}
				if message.Role != "user" || seenNonToolResult {
					return &claudeCompatViolation{
						param:     fmt.Sprintf("messages.%d.content.%d", messageIndex, contentIndex),
						code:      ClaudeCompatCodeInvalidToolResultOrder,
						message:   fmt.Sprintf("Invalid request for Claude: messages.%d.content.%d must place tool_result blocks at the start of the following user message.", messageIndex, contentIndex),
						status:    http.StatusBadRequest,
						riskLevel: claudeCompatRiskLow,
					}
				}
				if _, ok := previousToolUseIDs[content.ToolUseId]; !ok {
					return &claudeCompatViolation{
						param:     param,
						code:      ClaudeCompatCodeToolResultMismatch,
						message:   fmt.Sprintf("Invalid request for Claude: %s references %q, but the immediately preceding assistant message does not contain a matching tool_use id.", param, content.ToolUseId),
						status:    http.StatusBadRequest,
						riskLevel: claudeCompatRiskLow,
					}
				}
				matchedToolUseIDs[content.ToolUseId] = struct{}{}
			default:
				seenNonToolResult = true
			}
		}
		if len(previousToolUseIDs) > 0 {
			for toolUseID := range previousToolUseIDs {
				if _, ok := matchedToolUseIDs[toolUseID]; !ok {
					return missingClaudeToolResultViolation(messageIndex, toolUseID)
				}
			}
		}
	}
	if len(request.Messages) > 0 {
		lastIndex := len(request.Messages) - 1
		lastToolUseIDs := claudeToolUseIDsInMessage(request.Messages[lastIndex])
		if len(lastToolUseIDs) > 0 {
			return missingClaudeToolResultViolation(lastIndex+1, firstClaudeToolUseID(lastToolUseIDs))
		}
	}
	return nil
}

func missingClaudeToolResultViolation(messageIndex int, toolUseID string) *claudeCompatViolation {
	return &claudeCompatViolation{
		param:     fmt.Sprintf("messages.%d.content", messageIndex),
		code:      ClaudeCompatCodeMissingToolResult,
		message:   fmt.Sprintf("Invalid request for Claude: tool_use id %q must have a corresponding tool_result block immediately after the assistant message.", toolUseID),
		status:    http.StatusBadRequest,
		riskLevel: claudeCompatRiskLow,
	}
}

func firstClaudeToolUseID(ids map[string]struct{}) string {
	for id := range ids {
		return id
	}
	return ""
}

func claudeToolUseIDsInMessage(message dto.ClaudeMessage) map[string]struct{} {
	result := make(map[string]struct{})
	if message.Role != "assistant" {
		return result
	}
	contents, ok, err := claudeMessageContents(message.Content)
	if err != nil || !ok {
		return result
	}
	for _, content := range contents {
		if content.Type == "tool_use" && content.Id != "" {
			result[content.Id] = struct{}{}
		}
	}
	return result
}

func claudeMessageContents(content any) ([]dto.ClaudeMediaMessage, bool, error) {
	if content == nil {
		return nil, false, nil
	}
	switch typed := content.(type) {
	case string:
		return nil, false, nil
	case []dto.ClaudeMediaMessage:
		return typed, true, nil
	case []any:
		var contents []dto.ClaudeMediaMessage
		if err := anyToType(typed, &contents); err != nil {
			return nil, false, err
		}
		return contents, true, nil
	default:
		var contents []dto.ClaudeMediaMessage
		if err := anyToType(content, &contents); err != nil {
			return nil, false, err
		}
		return contents, true, nil
	}
}

func claudeToolsAsMaps(tools any) []map[string]any {
	if tools == nil {
		return nil
	}
	switch typed := tools.(type) {
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, tool := range typed {
			if m, ok := tool.(map[string]any); ok {
				result = append(result, m)
				continue
			}
			var m map[string]any
			if err := anyToType(tool, &m); err == nil {
				result = append(result, m)
			}
		}
		return result
	case []*dto.Tool:
		result := make([]map[string]any, 0, len(typed))
		for _, tool := range typed {
			if tool == nil {
				continue
			}
			result = append(result, map[string]any{"name": tool.Name})
		}
		return result
	case []dto.Tool:
		result := make([]map[string]any, 0, len(typed))
		for _, tool := range typed {
			result = append(result, map[string]any{"name": tool.Name})
		}
		return result
	default:
		var result []map[string]any
		if err := anyToType(tools, &result); err == nil {
			return result
		}
	}
	return nil
}

func anyToType(data any, target any) error {
	if data == nil {
		return errors.New("nil data")
	}
	body, err := common.Marshal(data)
	if err != nil {
		return err
	}
	return common.Unmarshal(body, target)
}

func normalizeOpenAIStyleClaudeRawMessages(payload map[string]any, info *RelayInfo) bool {
	if !shouldNormalizeOpenAIStyleClaudeRawMessages(info) {
		return false
	}
	settings := model_setting.GetClaudeSettings()
	if !settings.PromoteLeadingSystemRoleEnabled && !settings.MergeAdjacentSameRoleEnabled {
		return false
	}
	if payload == nil {
		return false
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}
	hasOpenAIOnlyRole := false
	for _, value := range messages {
		message, ok := value.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(common.Interface2String(message["role"]))
		if role == "developer" || role == "system" {
			hasOpenAIOnlyRole = true
			break
		}
	}
	if !hasOpenAIOnlyRole {
		return false
	}

	normalized := make([]any, 0, len(messages))
	mergeBarriers := make([]bool, 0, len(messages))
	systemBlocks := make([]any, 0)
	leadingSystemDone := !settings.PromoteLeadingSystemRoleEnabled
	changed := false
	for _, value := range messages {
		message, ok := value.(map[string]any)
		if !ok {
			normalized = append(normalized, value)
			mergeBarriers = append(mergeBarriers, true)
			leadingSystemDone = true
			continue
		}
		role := strings.TrimSpace(common.Interface2String(message["role"]))
		if role == "developer" {
			message = cloneClaudeRawMessageMap(message)
			message["role"] = "system"
			role = "system"
			changed = true
		}
		if !leadingSystemDone && role == "system" {
			systemBlocks = append(systemBlocks, claudeRawSystemBlocksFromMessage(message)...)
			changed = true
			continue
		}
		leadingSystemDone = true
		mergeBarrier := false
		if role == "system" {
			message = cloneClaudeRawMessageMap(message)
			message["role"] = "user"
			message["content"] = prefixClaudeRawSystemContent(message["content"])
			mergeBarrier = true
			changed = true
		}
		normalized = append(normalized, message)
		mergeBarriers = append(mergeBarriers, mergeBarrier)
	}
	if len(systemBlocks) > 0 {
		payload["system"] = appendClaudeRawSystemBlocks(payload["system"], systemBlocks)
	}
	if settings.MergeAdjacentSameRoleEnabled {
		merged, mergedChanged := mergeClaudeRawAdjacentMessages(normalized, mergeBarriers)
		if mergedChanged {
			normalized = merged
			changed = true
		}
	}
	if changed {
		payload["messages"] = normalized
	}
	return changed
}

func shouldNormalizeOpenAIStyleClaudeRawMessages(info *RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayFormat != types.RelayFormatOpenAI {
		return false
	}
	for _, format := range info.RequestConversionChain {
		if format == types.RelayFormatClaude {
			return true
		}
	}
	return info.FinalRequestRelayFormat == types.RelayFormatClaude
}

func cloneClaudeRawMessageMap(message map[string]any) map[string]any {
	cloned := make(map[string]any, len(message))
	for key, value := range message {
		cloned[key] = value
	}
	return cloned
}

func claudeRawSystemBlocksFromMessage(message map[string]any) []any {
	content := message["content"]
	if text, ok := content.(string); ok {
		return []any{map[string]any{"type": "text", "text": text}}
	}
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}
	result := make([]any, 0, len(blocks))
	for _, value := range blocks {
		block, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(common.Interface2String(block["type"])) != "text" {
			continue
		}
		result = append(result, block)
	}
	return result
}

func appendClaudeRawSystemBlocks(existing any, blocks []any) any {
	if len(blocks) == 0 {
		return existing
	}
	if existing == nil {
		return blocks
	}
	if text, ok := existing.(string); ok {
		result := []any{map[string]any{"type": "text", "text": text}}
		return append(result, blocks...)
	}
	if existingBlocks, ok := existing.([]any); ok {
		result := make([]any, 0, len(existingBlocks)+len(blocks))
		result = append(result, existingBlocks...)
		result = append(result, blocks...)
		return result
	}
	return existing
}

func prefixClaudeRawSystemContent(content any) any {
	if text, ok := content.(string); ok {
		return "system: " + text
	}
	blocks, ok := content.([]any)
	if !ok {
		return content
	}
	result := make([]any, 0, len(blocks)+1)
	for _, value := range blocks {
		block, ok := value.(map[string]any)
		if !ok || strings.TrimSpace(common.Interface2String(block["type"])) != "text" {
			result = append(result, value)
			continue
		}
		cloned := make(map[string]any, len(block))
		for key, blockValue := range block {
			cloned[key] = blockValue
		}
		cloned["text"] = "system: " + common.Interface2String(block["text"])
		result = append(result, cloned)
	}
	return result
}

func mergeClaudeRawAdjacentMessages(messages []any, barriers []bool) ([]any, bool) {
	if len(messages) < 2 {
		return messages, false
	}
	merged := make([]any, 0, len(messages))
	mergedBarriers := make([]bool, 0, len(messages))
	changed := false
	for index, value := range messages {
		barrier := index < len(barriers) && barriers[index]
		message, ok := value.(map[string]any)
		if !ok || len(merged) == 0 {
			merged = append(merged, value)
			mergedBarriers = append(mergedBarriers, barrier)
			continue
		}
		last, ok := merged[len(merged)-1].(map[string]any)
		if !ok {
			merged = append(merged, value)
			mergedBarriers = append(mergedBarriers, barrier)
			continue
		}
		role := strings.TrimSpace(common.Interface2String(message["role"]))
		if barrier || mergedBarriers[len(mergedBarriers)-1] || !canMergeClaudeAdjacentRole(strings.TrimSpace(common.Interface2String(last["role"])), role) {
			merged = append(merged, value)
			mergedBarriers = append(mergedBarriers, barrier)
			continue
		}
		last["content"] = mergeClaudeRawMessageContent(last["content"], message["content"])
		changed = true
	}
	return merged, changed
}

func mergeClaudeRawMessageContent(left any, right any) any {
	leftText, leftIsText := left.(string)
	rightText, rightIsText := right.(string)
	if leftIsText && rightIsText {
		return strings.TrimSpace(leftText + "\n" + rightText)
	}
	leftBlocks := claudeRawContentBlocks(left)
	rightBlocks := claudeRawContentBlocks(right)
	return append(leftBlocks, rightBlocks...)
}

func claudeRawContentBlocks(content any) []any {
	if content == nil {
		return nil
	}
	if text, ok := content.(string); ok {
		return []any{map[string]any{"type": "text", "text": text}}
	}
	if blocks, ok := content.([]any); ok {
		return blocks
	}
	return []any{map[string]any{"type": "text", "text": common.Interface2String(content)}}
}

func MergeClaudeAdjacentMessages(messages []dto.ClaudeMessage) []dto.ClaudeMessage {
	if len(messages) < 2 {
		return messages
	}
	merged := make([]dto.ClaudeMessage, 0, len(messages))
	for _, message := range messages {
		if len(merged) == 0 {
			merged = append(merged, message)
			continue
		}
		last := &merged[len(merged)-1]
		if !canMergeClaudeAdjacentRole(last.Role, message.Role) {
			merged = append(merged, message)
			continue
		}
		lastContents := contentToClaudeMediaMessages(last.Content)
		currentContents := contentToClaudeMediaMessages(message.Content)
		if containsClaudeToolResult(lastContents) || containsClaudeToolResult(currentContents) {
			merged = append(merged, message)
			continue
		}
		last.Content = mergeClaudeMessageContent(last.Content, message.Content)
	}
	return merged
}

func canMergeClaudeAdjacentRole(left, right string) bool {
	if left != right {
		return false
	}
	return left == "user" || left == "assistant"
}

func mergeClaudeMessageContent(left, right any) any {
	leftText, leftIsText := left.(string)
	rightText, rightIsText := right.(string)
	if leftIsText && rightIsText {
		return strings.TrimSpace(leftText + "\n" + rightText)
	}
	leftContents := contentToClaudeMediaMessages(left)
	rightContents := contentToClaudeMediaMessages(right)
	return append(leftContents, rightContents...)
}

func containsClaudeToolResult(contents []dto.ClaudeMediaMessage) bool {
	for _, content := range contents {
		if content.Type == "tool_result" {
			return true
		}
	}
	return false
}

func contentToClaudeMediaMessages(content any) []dto.ClaudeMediaMessage {
	if content == nil {
		return nil
	}
	if text, ok := content.(string); ok {
		return []dto.ClaudeMediaMessage{{
			Type: "text",
			Text: common.GetPointer(text),
		}}
	}
	contents, ok, err := claudeMessageContents(content)
	if err != nil || !ok {
		return nil
	}
	return contents
}
