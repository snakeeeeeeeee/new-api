package common

import (
	"testing"

	basecommon "github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func withOpenAIToolSchemaNullRequiredCompat(t *testing.T, enabled bool) {
	t.Helper()
	settings := model_setting.GetGlobalSettings()
	original := settings.OpenAIToolSchemaNullRequiredCompatEnabled
	settings.OpenAIToolSchemaNullRequiredCompatEnabled = enabled
	t.Cleanup(func() {
		settings.OpenAIToolSchemaNullRequiredCompatEnabled = original
	})
}

func newOpenAIToolSchemaNullRequiredTestInfo() *RelayInfo {
	return &RelayInfo{
		RelayMode:               relayconstant.RelayModeChatCompletions,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}
}

func TestCleanOpenAIToolSchemaNullRequiredJSONCoversSchemaLocations(t *testing.T) {
	withOpenAIToolSchemaNullRequiredCompat(t, true)
	body := []byte(`{
  "model":"gpt-test",
  "messages":[{"role":"user","content":{"required":null}}],
  "metadata":{"required":null},
  "tools":[{
    "type":"function",
    "function":{
      "name":"knowledge_list_documents",
      "parameters":{
        "type":"object",
        "required":null,
        "properties":{
          "required":{"type":"string"},
          "nested":{"type":"object","required":null},
          "data":{
            "default":{"required":null},
            "const":{"required":null},
            "enum":[{"required":null}],
            "examples":[{"required":null}]
          }
        },
        "patternProperties":{"^x":{"required":null}},
        "$defs":{"child":{"required":null}},
        "definitions":{"child":{"required":null}},
        "dependentSchemas":{"child":{"required":null}},
        "items":[{"required":null}],
        "prefixItems":[{"required":null}],
        "allOf":[{"required":null}],
        "anyOf":[{"required":null}],
        "oneOf":[{"required":null}],
        "additionalProperties":{"required":null},
        "unevaluatedProperties":{"required":null},
        "propertyNames":{"required":null},
        "contains":{"required":null},
        "not":{"required":null},
        "if":{"required":null},
        "then":{"required":null},
        "else":{"required":null}
      }
    }
  }],
  "functions":[{"name":"legacy","parameters":{"properties":{"nested":{"required":null}}}}]
}`)

	cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, newOpenAIToolSchemaNullRequiredTestInfo())
	require.NoError(t, err)

	removedPaths := []string{
		"tools.0.function.parameters.required",
		"tools.0.function.parameters.properties.nested.required",
		"tools.0.function.parameters.patternProperties.^x.required",
		"tools.0.function.parameters.$defs.child.required",
		"tools.0.function.parameters.definitions.child.required",
		"tools.0.function.parameters.dependentSchemas.child.required",
		"tools.0.function.parameters.items.0.required",
		"tools.0.function.parameters.prefixItems.0.required",
		"tools.0.function.parameters.allOf.0.required",
		"tools.0.function.parameters.anyOf.0.required",
		"tools.0.function.parameters.oneOf.0.required",
		"tools.0.function.parameters.additionalProperties.required",
		"tools.0.function.parameters.unevaluatedProperties.required",
		"tools.0.function.parameters.propertyNames.required",
		"tools.0.function.parameters.contains.required",
		"tools.0.function.parameters.not.required",
		"tools.0.function.parameters.if.required",
		"tools.0.function.parameters.then.required",
		"tools.0.function.parameters.else.required",
		"functions.0.parameters.properties.nested.required",
	}
	for _, path := range removedPaths {
		require.False(t, gjson.GetBytes(cleaned, path).Exists(), path)
	}

	preservedNullPaths := []string{
		"messages.0.content.required",
		"metadata.required",
		"tools.0.function.parameters.properties.data.default.required",
		"tools.0.function.parameters.properties.data.const.required",
		"tools.0.function.parameters.properties.data.enum.0.required",
		"tools.0.function.parameters.properties.data.examples.0.required",
	}
	for _, path := range preservedNullPaths {
		result := gjson.GetBytes(cleaned, path)
		require.True(t, result.Exists(), path)
		require.Equal(t, "null", result.Raw, path)
	}
	require.Equal(t, "string", gjson.GetBytes(cleaned, "tools.0.function.parameters.properties.required.type").String())
}

func TestCleanOpenAIToolSchemaNullRequiredJSONPreservesOtherInvalidValues(t *testing.T) {
	withOpenAIToolSchemaNullRequiredCompat(t, true)
	body := []byte(`{"tools":[{"type":"function","function":{"name":"x","parameters":{"required":"name","properties":{"child":{"required":false}}}}}],"functions":[{"name":"legacy","parameters":{"required":{}}}]}`)

	cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, newOpenAIToolSchemaNullRequiredTestInfo())
	require.NoError(t, err)
	require.Equal(t, body, cleaned)
}

func TestCleanOpenAIToolSchemaNullRequiredJSONRespectsSwitchAndFormatScope(t *testing.T) {
	body := []byte(" {\n  \"tools\": [{\"type\":\"function\",\"function\":{\"name\":\"x\",\"parameters\":{\"required\":null}}}]\n} ")

	t.Run("disabled", func(t *testing.T) {
		withOpenAIToolSchemaNullRequiredCompat(t, false)
		cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, newOpenAIToolSchemaNullRequiredTestInfo())
		require.NoError(t, err)
		require.Equal(t, body, cleaned)
	})

	t.Run("non OpenAI final format", func(t *testing.T) {
		withOpenAIToolSchemaNullRequiredCompat(t, true)
		info := newOpenAIToolSchemaNullRequiredTestInfo()
		info.FinalRequestRelayFormat = types.RelayFormatClaude
		cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, info)
		require.NoError(t, err)
		require.Equal(t, body, cleaned)
	})

	t.Run("non Chat Completions relay", func(t *testing.T) {
		withOpenAIToolSchemaNullRequiredCompat(t, true)
		info := newOpenAIToolSchemaNullRequiredTestInfo()
		info.RelayMode = relayconstant.RelayModeResponses
		cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, info)
		require.NoError(t, err)
		require.Equal(t, body, cleaned)
	})
}

func TestCleanOpenAIToolSchemaNullRequiredJSONRejectsMalformedJSONOnlyWhenEnabled(t *testing.T) {
	withOpenAIToolSchemaNullRequiredCompat(t, true)
	_, err := CleanOpenAIToolSchemaNullRequiredJSON([]byte(`{"tools":`), newOpenAIToolSchemaNullRequiredTestInfo())
	require.ErrorContains(t, err, "invalid OpenAI Chat Completions request JSON")
}

func TestCleanOpenAIToolSchemaNullRequiredDoesNotMutateInputMap(t *testing.T) {
	withOpenAIToolSchemaNullRequiredCompat(t, true)
	body := []byte(`{"tools":[{"type":"function","function":{"name":"x","parameters":{"required":null}}}]}`)
	original := append([]byte(nil), body...)

	cleaned, err := CleanOpenAIToolSchemaNullRequiredJSON(body, newOpenAIToolSchemaNullRequiredTestInfo())
	require.NoError(t, err)
	require.Equal(t, original, body)
	require.NotEqual(t, original, cleaned)

	var decoded map[string]any
	require.NoError(t, basecommon.Unmarshal(cleaned, &decoded))
}
