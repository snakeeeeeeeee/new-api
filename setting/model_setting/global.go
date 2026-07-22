package model_setting

import (
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	OpenAIFunctionNameMaxLength              = 64
	OpenAIReservedFunctionNamesMaxCount      = 128
	OpenAIReservedFunctionNamesMaxInputBytes = 8192
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled               bool                             `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist                  []string                         `json:"thinking_model_blacklist"`
	ChatCompletionsToResponsesPolicy        ChatCompletionsToResponsesPolicy `json:"chat_completions_to_responses_policy"`
	OpenAIReservedFunctionNameCompatEnabled bool                             `json:"openai_reserved_function_name_compat_enabled"`
	OpenAIReservedFunctionNames             string                           `json:"openai_reserved_function_names"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
	OpenAIReservedFunctionNameCompatEnabled: true,
	OpenAIReservedFunctionNames:             "python",
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

func NormalizeOpenAIReservedFunctionNames(value string) (string, []string, error) {
	if len(value) > OpenAIReservedFunctionNamesMaxInputBytes {
		return "", nil, fmt.Errorf("OpenAI 保留函数名配置不能超过 %d 字节", OpenAIReservedFunctionNamesMaxInputBytes)
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	names := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if err := validateOpenAIFunctionName(name); err != nil {
			return "", nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		if len(names) >= OpenAIReservedFunctionNamesMaxCount {
			return "", nil, fmt.Errorf("OpenAI 保留函数名最多配置 %d 个", OpenAIReservedFunctionNamesMaxCount)
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	return strings.Join(names, "\n"), names, nil
}

func GetOpenAIReservedFunctionNames() []string {
	_, names, err := NormalizeOpenAIReservedFunctionNames(globalSettings.OpenAIReservedFunctionNames)
	if err != nil {
		return nil
	}
	return names
}

func validateOpenAIFunctionName(name string) error {
	if len(name) > OpenAIFunctionNameMaxLength {
		return fmt.Errorf("OpenAI 函数名 %q 不能超过 %d 个字符", name, OpenAIFunctionNameMaxLength)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("OpenAI 函数名 %q 只能包含字母、数字、下划线和连字符", name)
	}
	return nil
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}
