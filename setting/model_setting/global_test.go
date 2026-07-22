package model_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIReservedFunctionNames(t *testing.T) {
	normalized, names, err := NormalizeOpenAIReservedFunctionNames(" python,xxx\npython\r\nfoo_bar-2 ")

	require.NoError(t, err)
	require.Equal(t, "python\nxxx\nfoo_bar-2", normalized)
	require.Equal(t, []string{"python", "xxx", "foo_bar-2"}, names)
}

func TestNormalizeOpenAIReservedFunctionNamesRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "invalid character", value: "python.tool"},
		{name: "too long", value: strings.Repeat("a", OpenAIFunctionNameMaxLength+1)},
		{name: "too many", value: buildOpenAIReservedFunctionNames(OpenAIReservedFunctionNamesMaxCount + 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := NormalizeOpenAIReservedFunctionNames(tt.value)
			require.Error(t, err)
		})
	}
}

func TestOpenAIToolSchemaNullRequiredCompatibilityDefaultsDisabled(t *testing.T) {
	require.False(t, defaultOpenaiSettings.OpenAIToolSchemaNullRequiredCompatEnabled)
}

func buildOpenAIReservedFunctionNames(count int) string {
	names := make([]string, count)
	for i := range names {
		names[i] = "function_" + strings.Repeat("x", i%8) + string(rune('A'+i%26)) + string(rune('0'+i%10))
	}
	return strings.Join(names, ",")
}
