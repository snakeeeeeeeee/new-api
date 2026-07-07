package common

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	ClaudeCacheTTL5m = "5m"
	ClaudeCacheTTL1h = "1h"
)

type claudeCacheTTLScanResult struct {
	hasExplicit5m bool
	hasExplicit1h bool
}

func CaptureClaudeCacheTTLBillingCompat(info *RelayInfo, jsonData []byte) {
	if info == nil || info.ChannelMeta == nil || len(jsonData) == 0 {
		return
	}
	if !info.ChannelOtherSettings.ClaudeCacheTTLBillingCompatEnabled {
		return
	}
	if info.GetFinalRequestRelayFormat() != types.RelayFormatClaude {
		return
	}

	scan, ok := inspectClaudeCacheTTLRequestJSON(jsonData)
	if !ok || !scan.hasExplicit5m || scan.hasExplicit1h {
		info.ClaudeCacheTTLBillingCompat = nil
		return
	}

	info.ClaudeCacheTTLBillingCompat = &ClaudeCacheTTLBillingCompatInfo{
		RequestedTTL:        ClaudeCacheTTL5m,
		UpstreamReportedTTL: ClaudeCacheTTL1h,
	}
}

func inspectClaudeCacheTTLRequestJSON(jsonData []byte) (claudeCacheTTLScanResult, bool) {
	var payload map[string]any
	if err := common.Unmarshal(jsonData, &payload); err != nil {
		return claudeCacheTTLScanResult{}, false
	}
	var result claudeCacheTTLScanResult
	recordClaudeCacheTTL(&result, claudeRawCacheControlTTL(payload["cache_control"]))
	collectClaudeCacheTTLFromBlocks(&result, payload["system"])
	if messages, ok := payload["messages"].([]any); ok {
		for _, messageValue := range messages {
			message, ok := messageValue.(map[string]any)
			if !ok {
				continue
			}
			collectClaudeCacheTTLFromBlocks(&result, message["content"])
		}
	}
	return result, true
}

func collectClaudeCacheTTLFromBlocks(result *claudeCacheTTLScanResult, value any) {
	blocks, ok := value.([]any)
	if !ok {
		return
	}
	for _, blockValue := range blocks {
		block, ok := blockValue.(map[string]any)
		if !ok {
			continue
		}
		recordClaudeCacheTTL(result, claudeRawCacheControlTTL(block["cache_control"]))
	}
}

func recordClaudeCacheTTL(result *claudeCacheTTLScanResult, ttl string) {
	switch strings.TrimSpace(ttl) {
	case ClaudeCacheTTL5m:
		result.hasExplicit5m = true
	case ClaudeCacheTTL1h:
		result.hasExplicit1h = true
	}
}
