package common

import (
	"fmt"
	"strconv"
	"strings"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type openAIFunctionNamePath struct {
	path string
	name string
}

func ShouldApplyOpenAIReservedFunctionNameCompat(info *RelayInfo) bool {
	if info == nil || !model_setting.GetGlobalSettings().OpenAIReservedFunctionNameCompatEnabled {
		return false
	}
	return info.RelayMode == relayconstant.RelayModeChatCompletions &&
		info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI
}

func RewriteOpenAIReservedFunctionNamesJSON(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if !ShouldApplyOpenAIReservedFunctionNameCompat(info) {
		return jsonData, nil
	}
	if !gjson.ValidBytes(jsonData) {
		return nil, fmt.Errorf("invalid OpenAI Chat Completions request JSON")
	}

	reservedNames := model_setting.GetOpenAIReservedFunctionNames()
	if len(reservedNames) == 0 {
		return jsonData, nil
	}
	reserved := make(map[string]struct{}, len(reservedNames))
	occupied := make(map[string]struct{}, len(reservedNames))
	for _, name := range reservedNames {
		reserved[name] = struct{}{}
		occupied[name] = struct{}{}
	}

	paths := collectOpenAIRequestFunctionNamePaths(jsonData)
	for _, item := range paths {
		occupied[item.name] = struct{}{}
	}
	for _, alias := range info.OpenAIReservedFunctionNameAliases {
		occupied[alias] = struct{}{}
	}

	result := string(jsonData)
	for _, item := range paths {
		if _, ok := reserved[item.name]; !ok {
			continue
		}
		alias := ensureOpenAIFunctionNameAlias(info, item.name, occupied)
		var err error
		result, err = sjson.Set(result, item.path, alias)
		if err != nil {
			return nil, err
		}
	}
	return []byte(result), nil
}

func RestoreOpenAIReservedFunctionNamesJSON(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if info == nil || len(info.OpenAIReservedFunctionNameRestores) == 0 || !gjson.ValidBytes(jsonData) {
		return jsonData, nil
	}

	result := string(jsonData)
	for _, item := range collectOpenAIResponseFunctionNamePaths(jsonData) {
		original, ok := info.OpenAIReservedFunctionNameRestores[item.name]
		if !ok {
			continue
		}
		var err error
		result, err = sjson.Set(result, item.path, original)
		if err != nil {
			return nil, err
		}
	}
	return []byte(result), nil
}

func ensureOpenAIFunctionNameAlias(info *RelayInfo, original string, occupied map[string]struct{}) string {
	if alias, ok := info.OpenAIReservedFunctionNameAliases[original]; ok {
		return alias
	}
	if info.OpenAIReservedFunctionNameAliases == nil {
		info.OpenAIReservedFunctionNameAliases = make(map[string]string)
	}
	if info.OpenAIReservedFunctionNameRestores == nil {
		info.OpenAIReservedFunctionNameRestores = make(map[string]string)
	}

	base := fitOpenAIFunctionName("run_" + original)
	alias := base
	for suffix := 2; ; suffix++ {
		if _, exists := occupied[alias]; !exists {
			break
		}
		suffixText := "_" + strconv.Itoa(suffix)
		alias = fitOpenAIFunctionName(base, suffixText)
	}
	info.OpenAIReservedFunctionNameAliases[original] = alias
	info.OpenAIReservedFunctionNameRestores[alias] = original
	occupied[alias] = struct{}{}
	return alias
}

func fitOpenAIFunctionName(parts ...string) string {
	suffix := ""
	base := strings.Join(parts, "")
	if len(parts) > 1 {
		suffix = parts[len(parts)-1]
		base = strings.Join(parts[:len(parts)-1], "")
	}
	maxBaseLength := model_setting.OpenAIFunctionNameMaxLength - len(suffix)
	if len(base) > maxBaseLength {
		base = base[:maxBaseLength]
	}
	return base + suffix
}

func collectOpenAIRequestFunctionNamePaths(jsonData []byte) []openAIFunctionNamePath {
	paths := make([]openAIFunctionNamePath, 0)
	paths = appendArrayFunctionNamePaths(paths, jsonData, "tools", "function.name")
	paths = appendArrayFunctionNamePaths(paths, jsonData, "functions", "name")
	paths = appendSingleFunctionNamePath(paths, jsonData, "tool_choice.function.name")
	paths = appendSingleFunctionNamePath(paths, jsonData, "tool_choice.name")
	paths = appendSingleFunctionNamePath(paths, jsonData, "function_call.name")

	for messageIndex, message := range gjson.GetBytes(jsonData, "messages").Array() {
		prefix := "messages." + strconv.Itoa(messageIndex)
		for toolCallIndex, toolCall := range message.Get("tool_calls").Array() {
			paths = appendResultFunctionNamePath(paths, toolCall.Get("function.name"),
				prefix+".tool_calls."+strconv.Itoa(toolCallIndex)+".function.name")
		}
		paths = appendResultFunctionNamePath(paths, message.Get("function_call.name"), prefix+".function_call.name")
		if message.Get("role").String() == "function" {
			paths = appendResultFunctionNamePath(paths, message.Get("name"), prefix+".name")
		}
	}
	return paths
}

func collectOpenAIResponseFunctionNamePaths(jsonData []byte) []openAIFunctionNamePath {
	paths := make([]openAIFunctionNamePath, 0)
	for choiceIndex, choice := range gjson.GetBytes(jsonData, "choices").Array() {
		prefix := "choices." + strconv.Itoa(choiceIndex)
		paths = appendMessageFunctionNamePaths(paths, choice.Get("message"), prefix+".message")
		paths = appendMessageFunctionNamePaths(paths, choice.Get("delta"), prefix+".delta")
	}
	return paths
}

func appendMessageFunctionNamePaths(paths []openAIFunctionNamePath, message gjson.Result, prefix string) []openAIFunctionNamePath {
	for toolCallIndex, toolCall := range message.Get("tool_calls").Array() {
		paths = appendResultFunctionNamePath(paths, toolCall.Get("function.name"),
			prefix+".tool_calls."+strconv.Itoa(toolCallIndex)+".function.name")
	}
	return appendResultFunctionNamePath(paths, message.Get("function_call.name"), prefix+".function_call.name")
}

func appendArrayFunctionNamePaths(paths []openAIFunctionNamePath, jsonData []byte, arrayPath string, namePath string) []openAIFunctionNamePath {
	for index, item := range gjson.GetBytes(jsonData, arrayPath).Array() {
		paths = appendResultFunctionNamePath(paths, item.Get(namePath), arrayPath+"."+strconv.Itoa(index)+"."+namePath)
	}
	return paths
}

func appendSingleFunctionNamePath(paths []openAIFunctionNamePath, jsonData []byte, path string) []openAIFunctionNamePath {
	return appendResultFunctionNamePath(paths, gjson.GetBytes(jsonData, path), path)
}

func appendResultFunctionNamePath(paths []openAIFunctionNamePath, result gjson.Result, path string) []openAIFunctionNamePath {
	if result.Type != gjson.String || result.String() == "" {
		return paths
	}
	return append(paths, openAIFunctionNamePath{path: path, name: result.String()})
}
