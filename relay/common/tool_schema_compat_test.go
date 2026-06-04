package common

import (
	"bytes"
	"testing"

	commonpkg "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
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

func TestNormalizeClaudeRequestToolSchemasFixesNestedObjectSchemaIssues(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type":       "object",
							"properties": nil,
							"required":   nil,
						},
						"nested": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path": map[string]any{
									"required": []any{"value", nil, "value"},
								},
							},
						},
						"list": map[string]any{
							"type":     "array",
							"items":    map[string]any{"type": "string"},
							"required": nil,
						},
						"ref": map[string]any{
							"$ref":     "#/$defs/ref",
							"required": nil,
						},
					},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	config := properties["config"].(map[string]any)
	require.Equal(t, map[string]any{}, config["properties"])
	require.NotContains(t, config, "required")
	nested := properties["nested"].(map[string]any)
	nestedProperties := nested["properties"].(map[string]any)
	path := nestedProperties["path"].(map[string]any)
	require.Equal(t, []any{"value"}, path["required"])
	list := properties["list"].(map[string]any)
	require.Nil(t, list["required"])
	ref := properties["ref"].(map[string]any)
	require.Nil(t, ref["required"])
}

func TestNormalizeClaudeRequestToolSchemasFixesArrayItemsObjectSchemaIssues(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pages": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":       "object",
								"properties": nil,
								"required":   nil,
							},
						},
						"data_sources": map[string]any{
							"type": "array",
							"items": map[string]any{
								"properties": map[string]any{
									"name": map[string]any{
										"required": []any{"value", float64(123), nil, "value"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	pages := properties["pages"].(map[string]any)
	pagesItems := pages["items"].(map[string]any)
	require.Equal(t, "object", pagesItems["type"])
	require.Equal(t, map[string]any{}, pagesItems["properties"])
	require.NotContains(t, pagesItems, "required")

	dataSources := properties["data_sources"].(map[string]any)
	dataSourceItems := dataSources["items"].(map[string]any)
	require.Equal(t, "object", dataSourceItems["type"])
	dataSourceProperties := dataSourceItems["properties"].(map[string]any)
	name := dataSourceProperties["name"].(map[string]any)
	require.Equal(t, []any{"value"}, name["required"])
}

func TestNormalizeClaudeRequestToolSchemasFixesTypelessDescriptionOnlyLeaf(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "algo_exec",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sql": map[string]any{
							"description": "sql text",
						},
						"title_only": map[string]any{
							"title": "title is not enough",
						},
						"bad_description": map[string]any{
							"description": []any{"not", "string"},
							"type":        "string",
						},
					},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	sql := properties["sql"].(map[string]any)
	require.Equal(t, "string", sql["type"])
	titleOnly := properties["title_only"].(map[string]any)
	require.NotContains(t, titleOnly, "type")
	badDescription := properties["bad_description"].(map[string]any)
	require.NotContains(t, badDescription, "description")
	require.Equal(t, "string", badDescription["type"])
}

func TestNormalizeClaudeRequestToolSchemasDisabledKeepsTypelessDescriptionOnlyLeaf(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "algo_exec",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sql": map[string]any{
							"description": "sql text",
						},
					},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(false))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	sql := properties["sql"].(map[string]any)
	require.NotContains(t, sql, "type")
	require.Equal(t, "sql text", sql["description"])
}

func TestNormalizeClaudeRequestToolSchemasLeavesComplexArrayItemsUntouched(t *testing.T) {
	t.Parallel()

	items := map[string]any{
		"$ref":     "#/$defs/page",
		"oneOf":    []any{map[string]any{"type": "string"}},
		"enum":     []any{"a", "b"},
		"required": nil,
	}
	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pages": map[string]any{
							"type":  "array",
							"items": items,
						},
					},
				},
			},
		},
	}

	NormalizeClaudeRequestToolSchemas(req, compatRelayInfo(true))

	tool := req.Tools.([]any)[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	pages := properties["pages"].(map[string]any)
	require.Equal(t, items, pages["items"])
	require.Nil(t, items["required"])
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

func TestLogToolSchemaCompatIncludesUserAndEndpoint(t *testing.T) {
	var buf bytes.Buffer
	commonpkg.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	commonpkg.LogWriterMu.Unlock()
	t.Cleanup(func() {
		commonpkg.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		commonpkg.LogWriterMu.Unlock()
	})

	logToolSchemaCompat(&RelayInfo{
		UserId:         256,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &ChannelMeta{
			ChannelId: 77,
		},
	}, toolSchemaCompatReport{
		ToolName:    "Workflow",
		Fixes:       []string{"required_removed"},
		SchemaShape: "{keys=[properties,type] type=object properties={}}",
	})

	logText := buf.String()
	require.Contains(t, logText, "tool_schema_compat_applied")
	require.Contains(t, logText, "channel=77")
	require.Contains(t, logText, "user_id=256")
	require.Contains(t, logText, `endpoint="/v1/chat/completions"`)
	require.Contains(t, logText, `tool="Workflow"`)
	require.Contains(t, logText, `schema_shape="{keys=[properties,type] type=object properties={}}"`)
}

func TestNormalizeClaudeRequestToolSchemasLogsCheckedSchemaShape(t *testing.T) {
	var buf bytes.Buffer
	commonpkg.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	commonpkg.LogWriterMu.Unlock()
	t.Cleanup(func() {
		commonpkg.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		commonpkg.LogWriterMu.Unlock()
	})

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "custom",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	info := compatRelayInfo(true)
	info.UserId = 256
	info.RequestURLPath = "/v1/chat/completions"

	NormalizeClaudeRequestToolSchemas(req, info)

	logText := buf.String()
	require.Contains(t, logText, "tool_schema_compat_checked")
	require.Contains(t, logText, "channel=123")
	require.Contains(t, logText, "user_id=256")
	require.Contains(t, logText, `tool="custom"`)
	require.Contains(t, logText, "schema_shape=")
	require.Contains(t, logText, "properties={path:")
}

func TestNormalizeClaudeRequestToolSchemasLogsArrayItemsShape(t *testing.T) {
	var buf bytes.Buffer
	commonpkg.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	commonpkg.LogWriterMu.Unlock()
	t.Cleanup(func() {
		commonpkg.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		commonpkg.LogWriterMu.Unlock()
	})

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "read_pdf_pages",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pages": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":       "object",
								"properties": nil,
								"required":   nil,
							},
						},
					},
				},
			},
		},
	}
	info := compatRelayInfo(true)
	info.UserId = 256
	info.RequestURLPath = "/v1/chat/completions"

	NormalizeClaudeRequestToolSchemas(req, info)

	logText := buf.String()
	require.Contains(t, logText, "tool_schema_compat_applied")
	require.Contains(t, logText, `tool="read_pdf_pages"`)
	require.Contains(t, logText, "nested_items_schema_fixed")
	require.Contains(t, logText, "pages:{keys=[items,type] type=array items={keys=[properties,type] type=object properties={}}}")
}

func TestNormalizeClaudeRequestToolSchemasLogsTypelessLeafFix(t *testing.T) {
	var buf bytes.Buffer
	commonpkg.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &buf
	commonpkg.LogWriterMu.Unlock()
	t.Cleanup(func() {
		commonpkg.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		commonpkg.LogWriterMu.Unlock()
	})

	req := &dto.ClaudeRequest{
		Tools: []any{
			map[string]any{
				"name": "algo_exec",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sql": map[string]any{
							"description": "sql text",
						},
					},
				},
			},
		},
	}
	info := compatRelayInfo(true)
	info.UserId = 256
	info.RequestURLPath = "/v1/chat/completions"

	NormalizeClaudeRequestToolSchemas(req, info)

	logText := buf.String()
	require.Contains(t, logText, "tool_schema_compat_applied")
	require.Contains(t, logText, `tool="algo_exec"`)
	require.Contains(t, logText, "nested_leaf_type_defaulted")
	require.Contains(t, logText, "sql:{keys=[description,type] type=string}")
}
