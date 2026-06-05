package model_setting

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

//var claudeHeadersSettings = map[string][]string{}
//
//var ClaudeThinkingAdapterEnabled = true
//var ClaudeThinkingAdapterMaxTokens = 8192
//var ClaudeThinkingAdapterBudgetTokensPercentage = 0.8

// ClaudeSettings 定义Claude模型的配置
type ClaudeSettings struct {
	HeadersSettings                       map[string]map[string][]string `json:"model_headers_settings"`
	DefaultMaxTokens                      map[string]int                 `json:"default_max_tokens"`
	ThinkingAdapterEnabled                bool                           `json:"thinking_adapter_enabled"`
	ThinkingAdapterBudgetTokensPercentage float64                        `json:"thinking_adapter_budget_tokens_percentage"`
	AutoFixImageMediaTypeEnabled          bool                           `json:"auto_fix_image_media_type_enabled"`
	PreserveZeroMaxTokensEnabled          bool                           `json:"preserve_zero_max_tokens_enabled"`
	DropDefaultSamplingForOpusEnabled     bool                           `json:"drop_default_sampling_for_opus_enabled"`
	ValidateOutputEffortEnabled           bool                           `json:"validate_output_effort_enabled"`
	PromoteLeadingSystemRoleEnabled       bool                           `json:"promote_leading_system_role_enabled"`
	MergeAdjacentSameRoleEnabled          bool                           `json:"merge_adjacent_same_role_enabled"`
	ReorderToolResultBlocksEnabled        bool                           `json:"reorder_tool_result_blocks_enabled"`
	ApplyCompatInPassthroughEnabled       bool                           `json:"apply_compat_in_passthrough_enabled"`
	RequestSchemaValidationMode           string                         `json:"request_schema_validation_mode"`
	ToolProtocolValidationMode            string                         `json:"tool_protocol_validation_mode"`
	ToolSchemaValidationMode              string                         `json:"tool_schema_validation_mode"`
	ToolChoiceValidationMode              string                         `json:"tool_choice_validation_mode"`
	ThinkingValidationMode                string                         `json:"thinking_validation_mode"`
	ImageLimitsValidationMode             string                         `json:"image_limits_validation_mode"`
	PromptCacheValidationMode             string                         `json:"prompt_cache_validation_mode"`
	StopSequencesValidationMode           string                         `json:"stop_sequences_validation_mode"`
	ServiceTierValidationMode             string                         `json:"service_tier_validation_mode"`
	MetadataUserIDValidationMode          string                         `json:"metadata_user_id_validation_mode"`
	RequestSizeLimitBytes                 int64                          `json:"request_size_limit_bytes"`
}

// 默认配置
var defaultClaudeSettings = ClaudeSettings{
	HeadersSettings:                   map[string]map[string][]string{},
	ThinkingAdapterEnabled:            true,
	AutoFixImageMediaTypeEnabled:      true,
	PreserveZeroMaxTokensEnabled:      true,
	DropDefaultSamplingForOpusEnabled: true,
	ValidateOutputEffortEnabled:       true,
	PromoteLeadingSystemRoleEnabled:   true,
	MergeAdjacentSameRoleEnabled:      true,
	ReorderToolResultBlocksEnabled:    false,
	ApplyCompatInPassthroughEnabled:   false,
	RequestSchemaValidationMode:       "reject",
	ToolProtocolValidationMode:        "reject",
	ToolSchemaValidationMode:          "log",
	ToolChoiceValidationMode:          "log",
	ThinkingValidationMode:            "log",
	ImageLimitsValidationMode:         "log",
	PromptCacheValidationMode:         "log",
	StopSequencesValidationMode:       "reject",
	ServiceTierValidationMode:         "reject",
	MetadataUserIDValidationMode:      "log",
	RequestSizeLimitBytes:             32 << 20,
	DefaultMaxTokens: map[string]int{
		"default": 8192,
	},
	ThinkingAdapterBudgetTokensPercentage: 0.8,
}

// 全局实例
var claudeSettings = defaultClaudeSettings

const (
	ClaudeValidationModeOff    = "off"
	ClaudeValidationModeLog    = "log"
	ClaudeValidationModeReject = "reject"
)

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("claude", &claudeSettings)
}

// GetClaudeSettings 获取Claude配置
func GetClaudeSettings() *ClaudeSettings {
	// check default max tokens must have default key
	if _, ok := claudeSettings.DefaultMaxTokens["default"]; !ok {
		claudeSettings.DefaultMaxTokens["default"] = 8192
	}
	claudeSettings.NormalizeValidationModes()
	return &claudeSettings
}

func ValidateClaudeValidationMode(mode string) error {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case ClaudeValidationModeOff, ClaudeValidationModeLog, ClaudeValidationModeReject:
		return nil
	default:
		return fmt.Errorf("Claude 校验模式必须是 off、log 或 reject")
	}
}

func normalizeClaudeValidationMode(mode string, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	if ValidateClaudeValidationMode(normalized) == nil {
		return normalized
	}
	return fallback
}

func (c *ClaudeSettings) NormalizeValidationModes() {
	c.RequestSchemaValidationMode = normalizeClaudeValidationMode(c.RequestSchemaValidationMode, defaultClaudeSettings.RequestSchemaValidationMode)
	c.ToolProtocolValidationMode = normalizeClaudeValidationMode(c.ToolProtocolValidationMode, defaultClaudeSettings.ToolProtocolValidationMode)
	c.ToolSchemaValidationMode = normalizeClaudeValidationMode(c.ToolSchemaValidationMode, defaultClaudeSettings.ToolSchemaValidationMode)
	c.ToolChoiceValidationMode = normalizeClaudeValidationMode(c.ToolChoiceValidationMode, defaultClaudeSettings.ToolChoiceValidationMode)
	c.ThinkingValidationMode = normalizeClaudeValidationMode(c.ThinkingValidationMode, defaultClaudeSettings.ThinkingValidationMode)
	c.ImageLimitsValidationMode = normalizeClaudeValidationMode(c.ImageLimitsValidationMode, defaultClaudeSettings.ImageLimitsValidationMode)
	c.PromptCacheValidationMode = normalizeClaudeValidationMode(c.PromptCacheValidationMode, defaultClaudeSettings.PromptCacheValidationMode)
	c.StopSequencesValidationMode = normalizeClaudeValidationMode(c.StopSequencesValidationMode, defaultClaudeSettings.StopSequencesValidationMode)
	c.ServiceTierValidationMode = normalizeClaudeValidationMode(c.ServiceTierValidationMode, defaultClaudeSettings.ServiceTierValidationMode)
	c.MetadataUserIDValidationMode = normalizeClaudeValidationMode(c.MetadataUserIDValidationMode, defaultClaudeSettings.MetadataUserIDValidationMode)
	if c.RequestSizeLimitBytes <= 0 {
		c.RequestSizeLimitBytes = defaultClaudeSettings.RequestSizeLimitBytes
	}
}

func (c *ClaudeSettings) WriteHeaders(originModel string, httpHeader *http.Header) {
	if headers, ok := c.HeadersSettings[originModel]; ok {
		for headerKey, headerValues := range headers {
			mergedValues := normalizeHeaderListValues(
				append(append([]string(nil), httpHeader.Values(headerKey)...), headerValues...),
			)
			if len(mergedValues) == 0 {
				continue
			}
			httpHeader.Set(headerKey, strings.Join(mergedValues, ","))
		}
	}
}

func normalizeHeaderListValues(values []string) []string {
	normalizedValues := make([]string, 0, len(values))
	seenValues := make(map[string]struct{}, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			normalizedItem := strings.TrimSpace(item)
			if normalizedItem == "" {
				continue
			}
			if _, exists := seenValues[normalizedItem]; exists {
				continue
			}
			seenValues[normalizedItem] = struct{}{}
			normalizedValues = append(normalizedValues, normalizedItem)
		}
	}
	return normalizedValues
}

func (c *ClaudeSettings) GetDefaultMaxTokens(model string) int {
	if maxTokens, ok := c.DefaultMaxTokens[model]; ok {
		return maxTokens
	}
	return c.DefaultMaxTokens["default"]
}
