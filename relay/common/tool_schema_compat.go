package common

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
)

type toolSchemaCompatReport struct {
	ToolName string
	Fixes    []string
}

func ShouldApplyClaudeToolSchemaCompat(info *RelayInfo) bool {
	return info != nil && info.ChannelOtherSettings.ClaudeToolSchemaCompatEnabled
}

func NormalizeClaudeRequestToolSchemas(request *dto.ClaudeRequest, info *RelayInfo) {
	if request == nil || !ShouldApplyClaudeToolSchemaCompat(info) {
		return
	}
	NormalizeClaudeToolsValue(request.Tools, info)
}

func NormalizeClaudeRequestToolSchemasInJSON(jsonData []byte, info *RelayInfo) ([]byte, error) {
	if len(jsonData) == 0 || !ShouldApplyClaudeToolSchemaCompat(info) {
		return jsonData, nil
	}

	var payload map[string]any
	if err := common.Unmarshal(jsonData, &payload); err != nil {
		return nil, err
	}
	if normalizeClaudeToolsInMap(payload, info) {
		return common.Marshal(payload)
	}
	return jsonData, nil
}

func NormalizeClaudeToolsValue(tools any, info *RelayInfo) {
	if tools == nil || !ShouldApplyClaudeToolSchemaCompat(info) {
		return
	}

	switch typedTools := tools.(type) {
	case []any:
		for _, tool := range typedTools {
			normalizeClaudeToolValue(tool, info)
		}
	case []*dto.Tool:
		for _, tool := range typedTools {
			normalizeClaudeDTOInputSchema(tool, info)
		}
	case []dto.Tool:
		for i := range typedTools {
			normalizeClaudeDTOInputSchema(&typedTools[i], info)
		}
	case []map[string]any:
		for _, tool := range typedTools {
			normalizeClaudeToolMap(tool, info)
		}
	}
}

func normalizeClaudeToolsInMap(payload map[string]any, info *RelayInfo) bool {
	tools, ok := payload["tools"]
	if !ok || tools == nil {
		return false
	}
	return normalizeClaudeToolsJSONValue(tools, info)
}

func normalizeClaudeToolsJSONValue(tools any, info *RelayInfo) bool {
	switch typedTools := tools.(type) {
	case []any:
		changed := false
		for _, tool := range typedTools {
			if normalizeClaudeToolValue(tool, info) {
				changed = true
			}
		}
		return changed
	case []map[string]any:
		changed := false
		for _, tool := range typedTools {
			if normalizeClaudeToolMap(tool, info) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func normalizeClaudeToolValue(tool any, info *RelayInfo) bool {
	switch typedTool := tool.(type) {
	case map[string]any:
		return normalizeClaudeToolMap(typedTool, info)
	case *dto.Tool:
		return normalizeClaudeDTOInputSchema(typedTool, info)
	case dto.Tool:
		return false
	default:
		return false
	}
}

func normalizeClaudeDTOInputSchema(tool *dto.Tool, info *RelayInfo) bool {
	if tool == nil {
		return false
	}
	schema, report, changed := normalizeClaudeInputSchemaMap(tool.InputSchema, tool.Name)
	if changed {
		tool.InputSchema = schema
		logToolSchemaCompat(info, report)
	}
	return changed
}

func normalizeClaudeToolMap(tool map[string]any, info *RelayInfo) bool {
	if tool == nil || isClaudeBuiltInToolMap(tool) {
		return false
	}

	toolName := common.Interface2String(tool["name"])
	schemaValue, exists := tool["input_schema"]
	if !exists || schemaValue == nil {
		tool["input_schema"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		logToolSchemaCompat(info, toolSchemaCompatReport{ToolName: toolName, Fixes: []string{"input_schema_defaulted"}})
		return true
	}

	schema, ok := schemaValue.(map[string]any)
	if !ok {
		tool["input_schema"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		logToolSchemaCompat(info, toolSchemaCompatReport{ToolName: toolName, Fixes: []string{"input_schema_defaulted"}})
		return true
	}

	normalized, report, changed := normalizeClaudeInputSchemaMap(schema, toolName)
	if changed {
		tool["input_schema"] = normalized
		logToolSchemaCompat(info, report)
	}
	return changed
}

func normalizeClaudeInputSchemaMap(schema map[string]any, toolName string) (map[string]any, toolSchemaCompatReport, bool) {
	report := toolSchemaCompatReport{ToolName: toolName}
	if schema == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, toolSchemaCompatReport{ToolName: toolName, Fixes: []string{"input_schema_defaulted"}}, true
	}

	typeValue, hasStringType := schema["type"].(string)
	if !hasStringType || strings.TrimSpace(typeValue) == "" {
		schema["type"] = "object"
		report.Fixes = append(report.Fixes, "type_defaulted")
	} else if typeValue != "object" {
		return schema, report, false
	}

	if properties, exists := schema["properties"]; !exists || properties == nil {
		schema["properties"] = map[string]any{}
		report.Fixes = append(report.Fixes, "properties_defaulted")
	}

	if required, exists := schema["required"]; exists {
		requiredList, ok := required.([]any)
		if !ok {
			delete(schema, "required")
			report.Fixes = append(report.Fixes, "required_removed")
		} else {
			seen := make(map[string]struct{}, len(requiredList))
			normalizedRequired := make([]any, 0, len(requiredList))
			removedInvalid := false
			for _, item := range requiredList {
				value, ok := item.(string)
				if !ok {
					removedInvalid = true
					continue
				}
				if _, exists := seen[value]; exists {
					removedInvalid = true
					continue
				}
				seen[value] = struct{}{}
				normalizedRequired = append(normalizedRequired, value)
			}
			if len(normalizedRequired) == 0 {
				delete(schema, "required")
				report.Fixes = append(report.Fixes, "required_empty_removed")
			} else if removedInvalid || len(normalizedRequired) != len(requiredList) {
				schema["required"] = normalizedRequired
				report.Fixes = append(report.Fixes, "required_filtered")
			}
		}
	}

	return schema, report, len(report.Fixes) > 0
}

func isClaudeBuiltInToolMap(tool map[string]any) bool {
	toolType, ok := tool["type"].(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(toolType, "web_search_")
}

func logToolSchemaCompat(info *RelayInfo, report toolSchemaCompatReport) {
	if len(report.Fixes) == 0 {
		return
	}
	fixes := append([]string(nil), report.Fixes...)
	sort.Strings(fixes)
	channelId := 0
	if info != nil {
		channelId = info.ChannelId
	}
	logger.LogInfo(context.Background(), fmt.Sprintf("tool_schema_compat_applied channel=%d tool=%q fixes=%s", channelId, report.ToolName, strings.Join(fixes, ",")))
}
