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
