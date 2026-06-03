package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func compatRelayInfo(enabled bool) *RelayInfo {
	return &RelayInfo{
		ChannelMeta: &ChannelMeta{
			ChannelId: 123,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ClaudeToolSchemaCompatEnabled: enabled,
			},
		},
	}
}

func TestNormalizeClaudeRequestToolSchemasDisabledKeepsRequiredNull(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": nil,
					"required":   nil,
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(false))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	require.Nil(t, schema["required"])
	require.Nil(t, schema["properties"])
}

func TestNormalizeClaudeRequestToolSchemasEnabledFixesLowRiskSchemaIssues(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name":         "custom_null_schema",
				"input_schema": nil,
			},
			map[string]any{
				"name": "custom_missing_type",
				"input_schema": map[string]any{
					"properties": nil,
					"required":   nil,
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	require.Equal(t, "object", schema["type"])
	require.Equal(t, map[string]any{}, schema["properties"])
	require.NotContains(t, schema, "required")

	tool = req.Tools.([]any)[1].(map[string]any)
	schema = tool["input_schema"].(map[string]any)
	require.Equal(t, "object", schema["type"])
	require.Equal(t, map[string]any{}, schema["properties"])
	require.NotContains(t, schema, "required")
}

func TestNormalizeClaudeRequestToolSchemasFiltersRequiredArray(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"path": map[string]any{"type": "string"}},
					"required":   []any{"path", float64(123), nil, "path"},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	require.Equal(t, []any{"path"}, schema["required"])
}

func TestNormalizeClaudeRequestToolSchemasLeavesBuiltInToolsUntouched(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"type": "web_search_20250305",
				"name": "web_search",
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	require.NotContains(t, tool, "input_schema")
}

func TestNormalizeClaudeRequestToolSchemasDoesNotRewriteComplexSchemaKeywords(t *testing.T) {
	t.Parallel()

	oneOf := []any{
		map[string]any{"type": "string"},
		map[string]any{"type": "number"},
	}
	items := map[string]any{"type": "string"}
	enumValues := []any{"a", "b"}
	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
					"$ref":       "#/$defs/custom",
					"oneOf":      oneOf,
					"items":      items,
					"enum":       enumValues,
					"required":   []any{"missing"},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	require.Equal(t, "#/$defs/custom", schema["$ref"])
	require.Equal(t, oneOf, schema["oneOf"])
	require.Equal(t, items, schema["items"])
	require.Equal(t, enumValues, schema["enum"])
	require.Equal(t, []any{"missing"}, schema["required"])
}

func TestNormalizeClaudeRequestToolSchemasLeavesExplicitNonObjectSchemaUntouched(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "string"},
					"required": nil,
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	require.Equal(t, "array", schema["type"])
	require.NotContains(t, schema, "properties")
	require.Nil(t, schema["required"])
}
