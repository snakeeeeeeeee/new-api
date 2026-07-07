package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func relayInfoWithClaudeCacheTTLBillingCompat(enabled bool) *RelayInfo {
	return &RelayInfo{
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		ChannelMeta: &ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ClaudeCacheTTLBillingCompatEnabled: enabled,
			},
		},
	}
}

func TestCaptureClaudeCacheTTLBillingCompatExplicit5mOnly(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "system block ttl",
			body: `{
				"model":"claude-sonnet-4-6",
				"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"5m"}}],
				"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
			}`,
		},
		{
			name: "top level ttl",
			body: `{
				"model":"claude-sonnet-4-6",
				"cache_control":{"type":"ephemeral","ttl":"5m"},
				"messages":[{"role":"user","content":"hello"}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := relayInfoWithClaudeCacheTTLBillingCompat(true)

			CaptureClaudeCacheTTLBillingCompat(info, []byte(tt.body))

			require.NotNil(t, info.ClaudeCacheTTLBillingCompat)
			require.Equal(t, ClaudeCacheTTL5m, info.ClaudeCacheTTLBillingCompat.RequestedTTL)
			require.Equal(t, ClaudeCacheTTL1h, info.ClaudeCacheTTLBillingCompat.UpstreamReportedTTL)
		})
	}
}

func TestCaptureClaudeCacheTTLBillingCompatRequiresSwitch(t *testing.T) {
	info := relayInfoWithClaudeCacheTTLBillingCompat(false)
	body := []byte(`{"cache_control":{"type":"ephemeral","ttl":"5m"},"messages":[{"role":"user","content":"hello"}]}`)

	CaptureClaudeCacheTTLBillingCompat(info, body)

	require.Nil(t, info.ClaudeCacheTTLBillingCompat)
}

func TestCaptureClaudeCacheTTLBillingCompatRejectsMissingOrMixedTTL(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing explicit ttl",
			body: `{"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hello"}]}`,
		},
		{
			name: "mixed explicit ttl",
			body: `{"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"5m"}}],"messages":[{"role":"user","content":[{"type":"text","text":"later","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`,
		},
		{
			name: "top level one hour",
			body: `{"cache_control":{"type":"ephemeral","ttl":"1h"},"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"5m"}}],"messages":[{"role":"user","content":"hello"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := relayInfoWithClaudeCacheTTLBillingCompat(true)

			CaptureClaudeCacheTTLBillingCompat(info, []byte(tt.body))

			require.Nil(t, info.ClaudeCacheTTLBillingCompat)
		})
	}
}
