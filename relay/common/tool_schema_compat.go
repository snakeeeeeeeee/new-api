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
	ToolName    string
	Fixes       []string
	SchemaShape string
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
	if reason := claudeRootSchemaSkipReason(tool.InputSchema); reason != "" {
		logToolSchemaCompatSkipped(info, tool.Name, reason, tool.InputSchema)
		return false
	}
	schema, report, changed := normalizeClaudeInputSchemaMap(tool.InputSchema, tool.Name)
	if changed {
		tool.InputSchema = schema
		report.SchemaShape = claudeSchemaShape(schema)
		logToolSchemaCompat(info, report)
	} else {
		logToolSchemaCompatChecked(info, tool.Name, schema)
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
		defaultSchema := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		tool["input_schema"] = defaultSchema
		logToolSchemaCompat(info, toolSchemaCompatReport{
			ToolName:    toolName,
			Fixes:       []string{"input_schema_defaulted"},
			SchemaShape: claudeSchemaShape(defaultSchema),
		})
		return true
	}

	schema, ok := schemaValue.(map[string]any)
	if !ok {
		tool["input_schema"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		logToolSchemaCompat(info, toolSchemaCompatReport{
			ToolName:    toolName,
			Fixes:       []string{"input_schema_defaulted"},
			SchemaShape: claudeSchemaShape(schemaValue),
		})
		return true
	}
	if reason := claudeRootSchemaSkipReason(schema); reason != "" {
		logToolSchemaCompatSkipped(info, toolName, reason, schema)
		return false
	}

	normalized, report, changed := normalizeClaudeInputSchemaMap(schema, toolName)
	if changed {
		tool["input_schema"] = normalized
		report.SchemaShape = claudeSchemaShape(normalized)
		logToolSchemaCompat(info, report)
	} else {
		logToolSchemaCompatChecked(info, toolName, normalized)
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
	} else if _, ok := properties.(map[string]any); !ok {
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

	normalizeClaudeAdditionalProperties(schema, &report, "additional_properties_removed")

	if normalizeClaudeNestedPropertySchemas(schema["properties"], &report) {
		return schema, report, true
	}

	return schema, report, len(report.Fixes) > 0
}

func normalizeClaudeNestedPropertySchemas(properties any, report *toolSchemaCompatReport) bool {
	propertiesMap, ok := properties.(map[string]any)
	if !ok {
		return false
	}

	changed := false
	for propertyName, propertySchemaValue := range propertiesMap {
		switch propertySchema := propertySchemaValue.(type) {
		case nil:
			propertiesMap[propertyName] = map[string]any{}
			report.Fixes = append(report.Fixes, "nested_property_schema_defaulted")
			changed = true
		case bool:
			continue
		case map[string]any:
			if normalizeClaudeNestedSchemaMap(propertySchema, report) {
				changed = true
			}
		default:
			propertiesMap[propertyName] = map[string]any{}
			report.Fixes = append(report.Fixes, "nested_property_schema_defaulted")
			changed = true
		}
	}
	return changed
}

func normalizeClaudeNestedSchemaMap(schema map[string]any, report *toolSchemaCompatReport) bool {
	if schema == nil || hasClaudeComplexSchemaKeyword(schema) {
		return false
	}

	changed := normalizeClaudeItemsSchema(schema, report)
	if isExplicitClaudeNonObjectSchema(schema) {
		return changed
	}

	changed = normalizeClaudeObjectLikeType(schema, report) || changed
	if typ, exists := schema["type"]; exists {
		typeString, ok := typ.(string)
		if !ok || strings.TrimSpace(typeString) == "" {
			schema["type"] = "object"
			report.Fixes = append(report.Fixes, "nested_type_defaulted")
			changed = true
		}
	}

	if properties, exists := schema["properties"]; exists {
		if properties == nil {
			schema["properties"] = map[string]any{}
			report.Fixes = append(report.Fixes, "nested_properties_defaulted")
			changed = true
		} else if _, ok := properties.(map[string]any); !ok {
			schema["properties"] = map[string]any{}
			report.Fixes = append(report.Fixes, "nested_properties_defaulted")
			changed = true
		} else if normalizeClaudeNestedPropertySchemas(properties, report) {
			changed = true
		}
	}

	if required, exists := schema["required"]; exists {
		requiredList, ok := required.([]any)
		if !ok {
			delete(schema, "required")
			report.Fixes = append(report.Fixes, "nested_required_removed")
			changed = true
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
				report.Fixes = append(report.Fixes, "nested_required_empty_removed")
				changed = true
			} else if removedInvalid || len(normalizedRequired) != len(requiredList) {
				schema["required"] = normalizedRequired
				report.Fixes = append(report.Fixes, "nested_required_filtered")
				changed = true
			}
		}
	}

	if normalizeClaudeAdditionalProperties(schema, report, "nested_additional_properties_removed") {
		changed = true
	}

	return changed
}

func normalizeClaudeItemsSchema(schema map[string]any, report *toolSchemaCompatReport) bool {
	itemsValue, exists := schema["items"]
	if !exists {
		return false
	}
	itemsSchema, ok := itemsValue.(map[string]any)
	if !ok || itemsSchema == nil || hasClaudeComplexSchemaKeyword(itemsSchema) {
		return false
	}

	if normalizeClaudeNestedSchemaMap(itemsSchema, report) {
		report.Fixes = append(report.Fixes, "nested_items_schema_fixed")
		return true
	}
	return false
}

func normalizeClaudeObjectLikeType(schema map[string]any, report *toolSchemaCompatReport) bool {
	if _, exists := schema["type"]; exists {
		return false
	}
	if _, exists := schema["properties"]; !exists {
		if _, exists := schema["additionalProperties"]; !exists {
			return false
		}
	}
	schema["type"] = "object"
	report.Fixes = append(report.Fixes, "nested_type_defaulted")
	return true
}

func normalizeClaudeAdditionalProperties(schema map[string]any, report *toolSchemaCompatReport, fixName string) bool {
	value, exists := schema["additionalProperties"]
	if !exists {
		return false
	}
	switch typedValue := value.(type) {
	case bool:
		return false
	case map[string]any:
		return normalizeClaudeNestedSchemaMap(typedValue, report)
	case nil:
		delete(schema, "additionalProperties")
		report.Fixes = append(report.Fixes, fixName)
		return true
	default:
		delete(schema, "additionalProperties")
		report.Fixes = append(report.Fixes, fixName)
		return true
	}
}

func claudeRootSchemaSkipReason(schema map[string]any) string {
	typeString, ok := schema["type"].(string)
	if !ok || strings.TrimSpace(typeString) == "" || typeString == "object" {
		return ""
	}
	return "explicit_non_object_schema:" + typeString
}

func hasClaudeComplexSchemaKeyword(schema map[string]any) bool {
	for _, key := range []string{"$ref", "oneOf", "anyOf", "allOf", "enum"} {
		if _, exists := schema[key]; exists {
			return true
		}
	}
	return false
}

func isExplicitClaudeNonObjectSchema(schema map[string]any) bool {
	typeString, ok := schema["type"].(string)
	return ok && strings.TrimSpace(typeString) != "" && typeString != "object"
}

func isClaudeBuiltInToolMap(tool map[string]any) bool {
	toolType, ok := tool["type"].(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(toolType, "web_search_")
}

const (
	claudeSchemaShapeMaxDepth      = 3
	claudeSchemaShapeMaxProperties = 16
	claudeSchemaShapeMaxLength     = 2000
)

func claudeSchemaShape(value any) string {
	var builder strings.Builder
	appendClaudeSchemaShape(&builder, value, 0)
	shape := builder.String()
	if len(shape) > claudeSchemaShapeMaxLength {
		return shape[:claudeSchemaShapeMaxLength] + "...truncated"
	}
	return shape
}

func appendClaudeSchemaShape(builder *strings.Builder, value any, depth int) {
	if builder.Len() > claudeSchemaShapeMaxLength {
		return
	}
	if depth > claudeSchemaShapeMaxDepth {
		builder.WriteString("...")
		return
	}

	switch typedValue := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool:
		builder.WriteString(fmt.Sprintf("bool(%t)", typedValue))
	case string:
		builder.WriteString("string")
	case []any:
		builder.WriteString(fmt.Sprintf("array(len=%d)", len(typedValue)))
	case map[string]any:
		appendClaudeSchemaMapShape(builder, typedValue, depth)
	default:
		builder.WriteString(fmt.Sprintf("%T", value))
	}
}

func appendClaudeSchemaMapShape(builder *strings.Builder, schema map[string]any, depth int) {
	builder.WriteString("{")
	keys := sortedMapKeys(schema)
	builder.WriteString("keys=[")
	builder.WriteString(strings.Join(keys, ","))
	builder.WriteString("]")
	if typeValue, exists := schema["type"]; exists {
		builder.WriteString(" type=")
		if typeString, ok := typeValue.(string); ok {
			builder.WriteString(typeString)
		} else {
			builder.WriteString(valueKind(typeValue))
		}
	}
	if required, exists := schema["required"]; exists {
		builder.WriteString(" required=")
		builder.WriteString(valueKind(required))
	}
	if additionalProperties, exists := schema["additionalProperties"]; exists {
		builder.WriteString(" additionalProperties=")
		builder.WriteString(valueKind(additionalProperties))
	}
	for _, keyword := range []string{"$ref", "oneOf", "anyOf", "allOf", "items", "enum"} {
		if keywordValue, exists := schema[keyword]; exists {
			builder.WriteString(" ")
			builder.WriteString(keyword)
			builder.WriteString("=")
			if keyword == "items" {
				appendClaudeSchemaShape(builder, keywordValue, depth+1)
			} else {
				builder.WriteString(valueKind(keywordValue))
			}
		}
	}
	if properties, exists := schema["properties"]; exists {
		builder.WriteString(" properties=")
		appendClaudePropertiesShape(builder, properties, depth+1)
	}
	builder.WriteString("}")
}

func appendClaudePropertiesShape(builder *strings.Builder, properties any, depth int) {
	propertiesMap, ok := properties.(map[string]any)
	if !ok {
		builder.WriteString(valueKind(properties))
		return
	}
	builder.WriteString("{")
	propertyNames := sortedMapKeys(propertiesMap)
	limit := len(propertyNames)
	if limit > claudeSchemaShapeMaxProperties {
		limit = claudeSchemaShapeMaxProperties
	}
	for i := 0; i < limit; i++ {
		if i > 0 {
			builder.WriteString(",")
		}
		propertyName := propertyNames[i]
		builder.WriteString(propertyName)
		builder.WriteString(":")
		appendClaudeSchemaShape(builder, propertiesMap[propertyName], depth)
	}
	if len(propertyNames) > limit {
		builder.WriteString(fmt.Sprintf(",...+%d", len(propertyNames)-limit))
	}
	builder.WriteString("}")
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func valueKind(value any) string {
	switch typedValue := value.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case []any:
		return fmt.Sprintf("array(len=%d)", len(typedValue))
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func logToolSchemaCompat(info *RelayInfo, report toolSchemaCompatReport) {
	if len(report.Fixes) == 0 {
		return
	}
	fixes := uniqueStrings(report.Fixes)
	sort.Strings(fixes)
	channelId := 0
	userId := 0
	endpoint := ""
	if info != nil {
		channelId = info.ChannelId
		userId = info.UserId
		endpoint = info.RequestURLPath
	}
	logger.LogInfo(context.Background(), fmt.Sprintf("tool_schema_compat_applied channel=%d user_id=%d endpoint=%q tool=%q fixes=%s schema_shape=%q", channelId, userId, endpoint, report.ToolName, strings.Join(fixes, ","), report.SchemaShape))
}

func logToolSchemaCompatSkipped(info *RelayInfo, toolName string, reason string, schema any) {
	channelId := 0
	userId := 0
	endpoint := ""
	if info != nil {
		channelId = info.ChannelId
		userId = info.UserId
		endpoint = info.RequestURLPath
	}
	logger.LogInfo(context.Background(), fmt.Sprintf("tool_schema_compat_skipped channel=%d user_id=%d endpoint=%q tool=%q reason=%q schema_shape=%q", channelId, userId, endpoint, toolName, reason, claudeSchemaShape(schema)))
}

func logToolSchemaCompatChecked(info *RelayInfo, toolName string, schema any) {
	channelId := 0
	userId := 0
	endpoint := ""
	if info != nil {
		channelId = info.ChannelId
		userId = info.UserId
		endpoint = info.RequestURLPath
	}
	logger.LogInfo(context.Background(), fmt.Sprintf("tool_schema_compat_checked channel=%d user_id=%d endpoint=%q tool=%q schema_shape=%q", channelId, userId, endpoint, toolName, claudeSchemaShape(schema)))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
