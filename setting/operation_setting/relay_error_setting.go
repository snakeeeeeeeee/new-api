package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type RelayErrorSetting struct {
	PassthroughEnabled     bool   `json:"passthrough_enabled"`
	PassthroughStatusCodes string `json:"passthrough_status_codes"`
	MaskSensitive          bool   `json:"mask_sensitive"`
}

var relayErrorSetting = RelayErrorSetting{
	PassthroughEnabled:     false,
	PassthroughStatusCodes: "400,422",
	MaskSensitive:          true,
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
