package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	basecommon "github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const openAIToolSchemaMaxCompatDepth = 256

var openAIToolSchemaMapKeywords = []string{
	"properties",
	"patternProperties",
	"$defs",
	"definitions",
	"dependentSchemas",
}

var openAIToolSchemaSingleKeywords = []string{
	"additionalProperties",
	"unevaluatedProperties",
	"propertyNames",
	"contains",
	"not",
	"if",
	"then",
	"else",
}

var openAIToolSchemaArrayKeywords = []string{
	"prefixItems",
	"allOf",
	"anyOf",
	"oneOf",
}

type openAIToolSchemaUpdate struct {
	path   string
	schema json.RawMessage
}

func ShouldApplyOpenAIToolSchemaNullRequiredCompat(info *RelayInfo) bool {
	if info == nil || !model_setting.GetGlobalSettings().OpenAIToolSchemaNullRequiredCompatEnabled {
		return false
	}
	return info.RelayMode == relayconstant.RelayModeChatCompletions &&
		info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI
}

func CleanOpenAIToolSchemaNullRequiredJSON(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if !ShouldApplyOpenAIToolSchemaNullRequiredCompat(info) {
		return jsonData, nil
	}
	if !gjson.ValidBytes(jsonData) {
		return nil, fmt.Errorf("invalid OpenAI Chat Completions request JSON")
	}

	updates, err := collectOpenAIToolSchemaNullRequiredUpdates(jsonData)
	if err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return jsonData, nil
	}

	result := jsonData
	for _, update := range updates {
		var err error
		result, err = sjson.SetRawBytes(result, update.path, update.schema)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func collectOpenAIToolSchemaNullRequiredUpdates(jsonData []byte) ([]openAIToolSchemaUpdate, error) {
	updates := make([]openAIToolSchemaUpdate, 0)
	for index, tool := range gjson.GetBytes(jsonData, "tools").Array() {
		var err error
		updates, err = appendOpenAIToolSchemaNullRequiredUpdate(
			updates,
			tool.Get("function.parameters"),
			"tools."+strconv.Itoa(index)+".function.parameters",
		)
		if err != nil {
			return nil, err
		}
	}
	for index, function := range gjson.GetBytes(jsonData, "functions").Array() {
		var err error
		updates, err = appendOpenAIToolSchemaNullRequiredUpdate(
			updates,
			function.Get("parameters"),
			"functions."+strconv.Itoa(index)+".parameters",
		)
		if err != nil {
			return nil, err
		}
	}
	return updates, nil
}

func appendOpenAIToolSchemaNullRequiredUpdate(updates []openAIToolSchemaUpdate, result gjson.Result, path string) ([]openAIToolSchemaUpdate, error) {
	if !result.IsObject() {
		return updates, nil
	}
	cleaned, changed, err := cleanOpenAIToolSchemaNullRequired(json.RawMessage(result.Raw), 0)
	if err != nil {
		return nil, err
	}
	if !changed {
		return updates, nil
	}
	return append(updates, openAIToolSchemaUpdate{path: path, schema: cleaned}), nil
}

func cleanOpenAIToolSchemaNullRequired(schema json.RawMessage, depth int) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(schema)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return schema, false, nil
	}
	if depth > openAIToolSchemaMaxCompatDepth {
		return nil, false, fmt.Errorf("OpenAI tool schema exceeds compatibility nesting limit")
	}

	var object map[string]json.RawMessage
	if err := basecommon.Unmarshal(trimmed, &object); err != nil {
		return nil, false, err
	}
	changed := false
	if required, exists := object["required"]; exists && bytes.Equal(bytes.TrimSpace(required), []byte("null")) {
		delete(object, "required")
		changed = true
	}

	for _, keyword := range openAIToolSchemaMapKeywords {
		cleaned, childChanged, err := cleanOpenAIToolSchemaMap(object[keyword], depth+1)
		if err != nil {
			return nil, false, err
		}
		if childChanged {
			object[keyword] = cleaned
			changed = true
		}
	}
	for _, keyword := range openAIToolSchemaSingleKeywords {
		cleaned, childChanged, err := cleanOpenAIToolSchemaNullRequired(object[keyword], depth+1)
		if err != nil {
			return nil, false, err
		}
		if childChanged {
			object[keyword] = cleaned
			changed = true
		}
	}
	for _, keyword := range openAIToolSchemaArrayKeywords {
		cleaned, childChanged, err := cleanOpenAIToolSchemaArray(object[keyword], depth+1)
		if err != nil {
			return nil, false, err
		}
		if childChanged {
			object[keyword] = cleaned
			changed = true
		}
	}

	items := bytes.TrimSpace(object["items"])
	if len(items) > 0 {
		var cleaned json.RawMessage
		var itemsChanged bool
		var err error
		if items[0] == '[' {
			cleaned, itemsChanged, err = cleanOpenAIToolSchemaArray(items, depth+1)
		} else {
			cleaned, itemsChanged, err = cleanOpenAIToolSchemaNullRequired(items, depth+1)
		}
		if err != nil {
			return nil, false, err
		}
		if itemsChanged {
			object["items"] = cleaned
			changed = true
		}
	}

	if !changed {
		return schema, false, nil
	}
	cleaned, err := basecommon.Marshal(object)
	return cleaned, true, err
}

func cleanOpenAIToolSchemaMap(raw json.RawMessage, depth int) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw, false, nil
	}
	var schemas map[string]json.RawMessage
	if err := basecommon.Unmarshal(trimmed, &schemas); err != nil {
		return nil, false, err
	}
	changed := false
	for name, schema := range schemas {
		cleaned, childChanged, err := cleanOpenAIToolSchemaNullRequired(schema, depth)
		if err != nil {
			return nil, false, err
		}
		if childChanged {
			schemas[name] = cleaned
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	cleaned, err := basecommon.Marshal(schemas)
	return cleaned, true, err
}

func cleanOpenAIToolSchemaArray(raw json.RawMessage, depth int) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return raw, false, nil
	}
	var schemas []json.RawMessage
	if err := basecommon.Unmarshal(trimmed, &schemas); err != nil {
		return nil, false, err
	}
	changed := false
	for index, schema := range schemas {
		cleaned, childChanged, err := cleanOpenAIToolSchemaNullRequired(schema, depth)
		if err != nil {
			return nil, false, err
		}
		if childChanged {
			schemas[index] = cleaned
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	cleaned, err := basecommon.Marshal(schemas)
	return cleaned, true, err
}
