package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskSensitiveInfoPreservesJSONFieldPaths(t *testing.T) {
	input := "Invalid request for Claude: messages.0.content.0.source.data is not valid base64 image data."

	result := MaskSensitiveInfo(input)

	require.Contains(t, result, "messages.0.content.0.source.data")
	require.NotContains(t, result, "***.***.***.***.***.data")
}

func TestMaskSensitiveInfoStillMasksDomainsAndIPs(t *testing.T) {
	input := "call https://api.openai.com/v1/chat?key=secret failed from 192.168.1.1 and fallback openai.com"

	result := MaskSensitiveInfo(input)

	require.NotContains(t, result, "api.openai.com")
	require.NotContains(t, result, "192.168.1.1")
	require.NotContains(t, result, "openai.com")
	require.Contains(t, result, "https://***.com/***")
	require.Contains(t, result, "***.***.***.***")
}

func TestMaskSensitiveInfoFullyMasksURLIPsAndPorts(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "ipv4",
			input: `Post "http://192.0.2.236:28787/v1/image/tasks/sync": context canceled`,
		},
		{
			name:  "ipv6",
			input: `Post "http://[2001:db8::236]:28787/v1/image/tasks/sync": context canceled`,
		},
		{
			name:  "domain with port",
			input: `Post "https://internal.example.com:28787/v1/image/tasks/sync": context canceled`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := MaskSensitiveInfo(test.input)
			require.NotContains(t, result, "192.0.2.236")
			require.NotContains(t, result, "2001:db8::236")
			require.NotContains(t, result, "internal.example.com")
			require.NotContains(t, result, "28787")
			require.NotContains(t, result, "/v1/image/tasks/sync")
			require.Contains(t, result, "context canceled")
		})
	}
}

func TestMaskSensitiveValueRecursivelyMasksNestedErrorDetails(t *testing.T) {
	input := map[string]any{
		"message": `Post "http://192.0.2.236:28787/v1/image/tasks/sync": context canceled`,
		"nested": []any{
			map[string]any{"url": "https://internal.example.com:9443/private/result"},
			42,
		},
	}

	masked := MaskSensitiveValue(input).(map[string]any)
	serialized, err := Marshal(masked)
	require.NoError(t, err)
	result := string(serialized)
	require.NotContains(t, result, "192.0.2.236")
	require.NotContains(t, result, "28787")
	require.NotContains(t, result, "internal.example.com")
	require.NotContains(t, result, "9443")
	require.NotContains(t, result, "private")
	require.Contains(t, result, "context canceled")
	require.Contains(t, result, "42")
}

func TestMaskSensitiveInfoPreservesCommonNestedParams(t *testing.T) {
	fields := []string{
		"tools.0.input_schema.properties.path.type",
		"messages.12.content.3.tool_use_id",
		"metadata.user_id",
		"output_config.effort",
	}

	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			result := MaskSensitiveInfo("param " + field + " failed")
			require.Contains(t, result, field)
			require.False(t, strings.Contains(result, "***."+strings.Split(field, ".")[len(strings.Split(field, "."))-1]))
		})
	}
}
