package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type RelayErrorSetting struct {
	PassthroughEnabled       bool   `json:"passthrough_enabled"`
	PassthroughStatusCodes   string `json:"passthrough_status_codes"`
	PassthroughBlockKeywords string `json:"passthrough_block_keywords"`
	MaskSensitive            bool   `json:"mask_sensitive"`
}

var relayErrorSetting = RelayErrorSetting{
	PassthroughEnabled:       false,
	PassthroughStatusCodes:   "400,422",
	PassthroughBlockKeywords: "",
	MaskSensitive:            true,
}

func init() {
	config.GlobalConfig.Register("relay_error_setting", &relayErrorSetting)
}

func GetRelayErrorSetting() *RelayErrorSetting {
	return &relayErrorSetting
}

func ShouldPassthroughRelayErrorStatusCode(code int) bool {
	if !relayErrorSetting.PassthroughEnabled {
		return false
	}
	ranges, err := ParseHTTPStatusCodeRanges(relayErrorSetting.PassthroughStatusCodes)
	if err != nil {
		return false
	}
	return shouldMatchStatusCodeRanges(ranges, code)
}

func ShouldBlockRelayErrorPassthrough(message string) bool {
	message = strings.ToLower(message)
	if strings.TrimSpace(message) == "" {
		return false
	}
	for _, keyword := range strings.Split(relayErrorSetting.PassthroughBlockKeywords, "\n") {
		keyword = strings.TrimSpace(strings.ToLower(keyword))
		if keyword == "" {
			continue
		}
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
