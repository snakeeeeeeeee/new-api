package operation_setting

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	ViolationActionLogOnly           = "log_only"
	ViolationActionBlock             = "block"
	ViolationActionBanAfterThreshold = "ban_after_threshold"

	DefaultViolationHTTPStatusCode = http.StatusForbidden
	DefaultViolationErrorCode      = "policy_violation"
	DefaultViolationErrorMessage   = "Request blocked by policy."
	DefaultViolationMaxExcerptLen  = 300
	DefaultViolationBanThreshold   = 3
	maxViolationExcerptLen         = 2000
	maxViolationBanThreshold       = 1000000
)

type ViolationSetting struct {
	Enabled          bool   `json:"enabled"`
	Keywords         string `json:"keywords"`
	CaseSensitive    bool   `json:"case_sensitive"`
	Action           string `json:"action"`
	HTTPStatusCode   int    `json:"http_status_code"`
	ErrorCode        string `json:"error_code"`
	ErrorMessage     string `json:"error_message"`
	MaxExcerptLength int    `json:"max_excerpt_length"`
	BanThreshold     int    `json:"ban_threshold"`
}

var violationSetting = ViolationSetting{
	Enabled:          false,
	Keywords:         "",
	CaseSensitive:    false,
	Action:           ViolationActionBlock,
	HTTPStatusCode:   DefaultViolationHTTPStatusCode,
	ErrorCode:        DefaultViolationErrorCode,
	ErrorMessage:     DefaultViolationErrorMessage,
	MaxExcerptLength: DefaultViolationMaxExcerptLen,
	BanThreshold:     DefaultViolationBanThreshold,
}

func init() {
	config.GlobalConfig.Register("violation_setting", &violationSetting)
}

func GetViolationSetting() *ViolationSetting {
	return &violationSetting
}

func NormalizeViolationSetting(setting ViolationSetting) ViolationSetting {
	setting.Action = NormalizeViolationAction(setting.Action)
	if setting.HTTPStatusCode < 400 || setting.HTTPStatusCode > 599 {
		setting.HTTPStatusCode = DefaultViolationHTTPStatusCode
	}
	setting.ErrorCode = strings.TrimSpace(setting.ErrorCode)
	if setting.ErrorCode == "" {
		setting.ErrorCode = DefaultViolationErrorCode
	}
	setting.ErrorMessage = strings.TrimSpace(setting.ErrorMessage)
	if setting.ErrorMessage == "" {
		setting.ErrorMessage = DefaultViolationErrorMessage
	}
	if setting.MaxExcerptLength <= 0 {
		setting.MaxExcerptLength = DefaultViolationMaxExcerptLen
	}
	if setting.MaxExcerptLength > maxViolationExcerptLen {
		setting.MaxExcerptLength = maxViolationExcerptLen
	}
	if setting.BanThreshold <= 0 {
		setting.BanThreshold = DefaultViolationBanThreshold
	}
	if setting.BanThreshold > maxViolationBanThreshold {
		setting.BanThreshold = maxViolationBanThreshold
	}
	return setting
}

func NormalizeViolationAction(action string) string {
	switch strings.TrimSpace(action) {
	case ViolationActionLogOnly:
		return ViolationActionLogOnly
	case ViolationActionBanAfterThreshold:
		return ViolationActionBanAfterThreshold
	default:
		return ViolationActionBlock
	}
}

func ValidateViolationSetting(setting ViolationSetting) error {
	if setting.HTTPStatusCode < 400 || setting.HTTPStatusCode > 599 {
		return errors.New("http_status_code must be between 400 and 599")
	}
	if strings.TrimSpace(setting.ErrorCode) == "" {
		return errors.New("error_code is required")
	}
	if strings.TrimSpace(setting.ErrorMessage) == "" {
		return errors.New("error_message is required")
	}
	if NormalizeViolationAction(setting.Action) != strings.TrimSpace(setting.Action) {
		return errors.New("action must be log_only, block, or ban_after_threshold")
	}
	if setting.MaxExcerptLength <= 0 || setting.MaxExcerptLength > maxViolationExcerptLen {
		return errors.New("max_excerpt_length must be between 1 and 2000")
	}
	if setting.BanThreshold <= 0 || setting.BanThreshold > maxViolationBanThreshold {
		return errors.New("ban_threshold must be between 1 and 1000000")
	}
	return nil
}

func ViolationKeywords() []string {
	return splitViolationKeywords(violationSetting.Keywords)
}

func ViolationKeywordsFromString(raw string) []string {
	return splitViolationKeywords(raw)
}

func splitViolationKeywords(raw string) []string {
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	keywords := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		keyword := strings.TrimSpace(line)
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return keywords
}
