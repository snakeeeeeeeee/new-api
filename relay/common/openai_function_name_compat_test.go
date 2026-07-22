package common

import (
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func withOpenAIReservedFunctionNameSettings(t *testing.T, enabled bool, names string) {
	t.Helper()
	settings := model_setting.GetGlobalSettings()
	originalEnabled := settings.OpenAIReservedFunctionNameCompatEnabled
	originalNames := settings.OpenAIReservedFunctionNames
	settings.OpenAIReservedFunctionNameCompatEnabled = enabled
	settings.OpenAIReservedFunctionNames = names
	t.Cleanup(func() {
		settings.OpenAIReservedFunctionNameCompatEnabled = originalEnabled
		settings.OpenAIReservedFunctionNames = originalNames
	})
}

func newOpenAIReservedFunctionNameTestInfo() *RelayInfo {
	return &RelayInfo{
		RelayMode:               relayconstant.RelayModeChatCompletions,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}
}

func TestRewriteOpenAIReservedFunctionNamesJSONCoversKnownRequestPaths(t *testing.T) {
	withOpenAIReservedFunctionNameSettings(t, true, "python, xxx")
	info := newOpenAIReservedFunctionNameTestInfo()
	body := []byte(`{
  "model":"gpt-any-model",
  "tools":[
    {"type":"function","function":{"name":"python","description":"python","parameters":{"type":"object"}}},
    {"type":"function","function":{"name":"run_python","parameters":{}}},
    {"type":"function","function":{"name":"run_python_2","parameters":{}}}
  ],
  "functions":[{"name":"xxx","description":"xxx"}],
  "tool_choice":{"type":"function","function":{"name":"python"},"name":"xxx"},
  "function_call":{"name":"xxx"},
  "messages":[
    {"role":"assistant","content":"python run_python_3","tool_calls":[{"id":"call_1","type":"function","function":{"name":"python","arguments":"{\"code\":\"run_python_3()\"}"}}],"function_call":{"name":"xxx","arguments":"{\"name\":\"python\"}"}},
    {"role":"function","name":"python","content":"run_python_3"},
    {"role":"user","name":"python","content":"python"}
  ]
}`)

	rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, info)
	require.NoError(t, err)
	require.Equal(t, "run_python_3", info.OpenAIReservedFunctionNameAliases["python"])
	require.Equal(t, "run_xxx", info.OpenAIReservedFunctionNameAliases["xxx"])

	assertJSONPathEquals(t, rewritten, "tools.0.function.name", "run_python_3")
	assertJSONPathEquals(t, rewritten, "tools.1.function.name", "run_python")
	assertJSONPathEquals(t, rewritten, "tools.2.function.name", "run_python_2")
	assertJSONPathEquals(t, rewritten, "functions.0.name", "run_xxx")
	assertJSONPathEquals(t, rewritten, "tool_choice.function.name", "run_python_3")
	assertJSONPathEquals(t, rewritten, "tool_choice.name", "run_xxx")
	assertJSONPathEquals(t, rewritten, "function_call.name", "run_xxx")
	assertJSONPathEquals(t, rewritten, "messages.0.tool_calls.0.function.name", "run_python_3")
	assertJSONPathEquals(t, rewritten, "messages.0.function_call.name", "run_xxx")
	assertJSONPathEquals(t, rewritten, "messages.1.name", "run_python_3")
	assertJSONPathEquals(t, rewritten, "messages.2.name", "python")
	assertJSONPathEquals(t, rewritten, "messages.0.content", "python run_python_3")
	assertJSONPathEquals(t, rewritten, "messages.0.tool_calls.0.function.arguments", `{"code":"run_python_3()"}`)
	assertJSONPathEquals(t, rewritten, "messages.0.function_call.arguments", `{"name":"python"}`)
}

func TestRewriteOpenAIReservedFunctionNamesJSONRespectsScopeAndEmptyConfig(t *testing.T) {
	body := []byte(`{"tools":[{"type":"function","function":{"name":"python"}}]}`)

	t.Run("disabled", func(t *testing.T) {
		withOpenAIReservedFunctionNameSettings(t, false, "python")
		rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, newOpenAIReservedFunctionNameTestInfo())
		require.NoError(t, err)
		require.Equal(t, body, rewritten)
	})

	t.Run("empty list", func(t *testing.T) {
		withOpenAIReservedFunctionNameSettings(t, true, "")
		rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, newOpenAIReservedFunctionNameTestInfo())
		require.NoError(t, err)
		require.Equal(t, body, rewritten)
	})

	t.Run("non OpenAI final format", func(t *testing.T) {
		withOpenAIReservedFunctionNameSettings(t, true, "python")
		info := newOpenAIReservedFunctionNameTestInfo()
		info.FinalRequestRelayFormat = types.RelayFormatClaude
		rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, info)
		require.NoError(t, err)
		require.Equal(t, body, rewritten)
	})
}

func TestRewriteOpenAIReservedFunctionNamesJSONBoundsCollisionAliases(t *testing.T) {
	original := strings.Repeat("a", model_setting.OpenAIFunctionNameMaxLength)
	candidate := "run_" + original[:model_setting.OpenAIFunctionNameMaxLength-4]
	withOpenAIReservedFunctionNameSettings(t, true, original)
	info := newOpenAIReservedFunctionNameTestInfo()
	body := []byte(`{"tools":[{"type":"function","function":{"name":"` + original + `"}},{"type":"function","function":{"name":"` + candidate + `"}}]}`)

	rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, info)
	require.NoError(t, err)
	alias := gjson.GetBytes(rewritten, "tools.0.function.name").String()
	require.Len(t, alias, model_setting.OpenAIFunctionNameMaxLength)
	require.True(t, strings.HasSuffix(alias, "_2"))
	require.NotEqual(t, candidate, alias)
}

func TestRewriteOpenAIReservedFunctionNamesJSONAvoidsConfiguredReservedAlias(t *testing.T) {
	withOpenAIReservedFunctionNameSettings(t, true, "python,run_python")
	info := newOpenAIReservedFunctionNameTestInfo()
	body := []byte(`{"tools":[{"type":"function","function":{"name":"python"}}]}`)

	rewritten, err := RewriteOpenAIReservedFunctionNamesJSON(body, info)

	require.NoError(t, err)
	assertJSONPathEquals(t, rewritten, "tools.0.function.name", "run_python_2")
}

func TestRestoreOpenAIReservedFunctionNamesJSONCoversStreamingAndNormalResponses(t *testing.T) {
	info := newOpenAIReservedFunctionNameTestInfo()
	info.OpenAIReservedFunctionNameRestores = map[string]string{
		"run_python": "python",
		"run_xxx":    "xxx",
	}
	body := []byte(`{
  "choices":[
    {"message":{"content":"run_python","tool_calls":[{"function":{"name":"run_python","arguments":"{\"name\":\"run_python\"}"}}],"function_call":{"name":"run_xxx","arguments":"run_xxx"}}},
    {"delta":{"content":"run_python","tool_calls":[{"function":{"name":"run_python","arguments":"run_python"}}],"function_call":{"name":"run_xxx","arguments":"run_xxx"}}}
  ]
}`)

	restored, err := RestoreOpenAIReservedFunctionNamesJSON(body, info)
	require.NoError(t, err)
	assertJSONPathEquals(t, restored, "choices.0.message.tool_calls.0.function.name", "python")
	assertJSONPathEquals(t, restored, "choices.0.message.function_call.name", "xxx")
	assertJSONPathEquals(t, restored, "choices.1.delta.tool_calls.0.function.name", "python")
	assertJSONPathEquals(t, restored, "choices.1.delta.function_call.name", "xxx")
	assertJSONPathEquals(t, restored, "choices.0.message.content", "run_python")
	assertJSONPathEquals(t, restored, "choices.0.message.tool_calls.0.function.arguments", `{"name":"run_python"}`)
	assertJSONPathEquals(t, restored, "choices.1.delta.tool_calls.0.function.arguments", "run_python")
}

func assertJSONPathEquals(t *testing.T, body []byte, path string, expected string) {
	t.Helper()
	require.Equal(t, expected, gjson.GetBytes(body, path).String(), path)
}
