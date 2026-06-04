package aws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoAwsClientRequest_AppliesRuntimeHeaderOverrideToAnthropicBeta(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName:           "claude-3-5-sonnet-20240620",
		IsStream:                  false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"anthropic-beta": "computer-use-2025-01-24",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "access-key|secret-key|us-east-1",
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}

	requestBody := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
	adaptor := &Adaptor{}

	_, err := doAwsClientRequest(ctx, info, adaptor, requestBody)
	require.NoError(t, err)

	awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	require.True(t, ok)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(awsReq.Body, &payload))

	anthropicBeta, exists := payload["anthropic_beta"]
	require.True(t, exists)

	values, ok := anthropicBeta.([]any)
	require.True(t, ok)
	require.Equal(t, []any{"computer-use-2025-01-24"}, values)
}

func TestBuildAwsRequestBodyToolSchemaCompatNonPassThrough(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
	})

	info := awsRelayInfoWithToolSchemaCompat(false, false)
	req := &AwsClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
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

	body, err := buildAwsRequestBody(nil, info, req)
	require.NoError(t, err)
	schema := decodeFirstToolSchema(t, body)
	require.Nil(t, schema["required"])
	require.Nil(t, schema["properties"])

	info = awsRelayInfoWithToolSchemaCompat(true, false)
	req = &AwsClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
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
	body, err = buildAwsRequestBody(nil, info, req)
	require.NoError(t, err)
	schema = decodeFirstToolSchema(t, body)
	require.NotContains(t, schema, "required")
	require.Equal(t, map[string]any{}, schema["properties"])
}

func TestBuildAwsRequestBodyToolSchemaCompatPassThrough(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
	})

	rawBody := `{"model":"claude-opus-4-7","stream":true,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"custom","input_schema":{"type":"","properties":null,"required":null}}]}`

	ctx := awsTestContext(rawBody)
	body, err := buildAwsRequestBody(ctx, awsRelayInfoWithToolSchemaCompat(false, true), &AwsClaudeRequest{})
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.NotContains(t, payload, "model")
	require.NotContains(t, payload, "stream")
	schema := decodeFirstToolSchema(t, body)
	require.Nil(t, schema["required"])
	require.Nil(t, schema["properties"])

	ctx = awsTestContext(rawBody)
	body, err = buildAwsRequestBody(ctx, awsRelayInfoWithToolSchemaCompat(true, true), &AwsClaudeRequest{})
	require.NoError(t, err)
	schema = decodeFirstToolSchema(t, body)
	require.Equal(t, "object", schema["type"])
	require.Equal(t, map[string]any{}, schema["properties"])
	require.NotContains(t, schema, "required")
}

func TestBuildAwsRequestBodyToolSchemaCompatFixesArrayItems(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
	})

	req := &AwsClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
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
	body, err := buildAwsRequestBody(nil, awsRelayInfoWithToolSchemaCompat(true, false), req)
	require.NoError(t, err)
	items := decodeFirstToolArrayItemsSchema(t, body, "pages")
	require.Equal(t, map[string]any{}, items["properties"])
	require.NotContains(t, items, "required")

	rawBody := `{"model":"claude-opus-4-7","stream":true,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"read_pdf_pages","input_schema":{"type":"object","properties":{"pages":{"type":"array","items":{"type":"object","properties":null,"required":null}}}}}]}`
	ctx := awsTestContext(rawBody)
	body, err = buildAwsRequestBody(ctx, awsRelayInfoWithToolSchemaCompat(true, true), &AwsClaudeRequest{})
	require.NoError(t, err)
	items = decodeFirstToolArrayItemsSchema(t, body, "pages")
	require.Equal(t, map[string]any{}, items["properties"])
	require.NotContains(t, items, "required")
}

func TestBuildAwsRequestBodyToolSchemaCompatFixesTypelessDescriptionOnlyLeaf(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
	})

	req := &AwsClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
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
	body, err := buildAwsRequestBody(nil, awsRelayInfoWithToolSchemaCompat(true, false), req)
	require.NoError(t, err)
	sql := decodeFirstToolPropertySchema(t, body, "sql")
	require.Equal(t, "string", sql["type"])

	rawBody := `{"model":"claude-opus-4-7","stream":true,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"algo_exec","input_schema":{"type":"object","properties":{"sql":{"description":"sql text"}}}}]}`
	ctx := awsTestContext(rawBody)
	body, err = buildAwsRequestBody(ctx, awsRelayInfoWithToolSchemaCompat(true, true), &AwsClaudeRequest{})
	require.NoError(t, err)
	sql = decodeFirstToolPropertySchema(t, body, "sql")
	require.Equal(t, "string", sql["type"])
}

func awsRelayInfoWithToolSchemaCompat(enabled bool, passThrough bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 789,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: passThrough,
			},
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ClaudeToolSchemaCompatEnabled: enabled,
			},
		},
	}
}

func awsTestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func decodeFirstToolSchema(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, tools)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	schema, ok := tool["input_schema"].(map[string]any)
	require.True(t, ok)
	return schema
}

func decodeFirstToolArrayItemsSchema(t *testing.T, body []byte, propertyName string) map[string]any {
	t.Helper()

	property := decodeFirstToolPropertySchema(t, body, propertyName)
	items, ok := property["items"].(map[string]any)
	require.True(t, ok)
	return items
}

func decodeFirstToolPropertySchema(t *testing.T, body []byte, propertyName string) map[string]any {
	t.Helper()

	schema := decodeFirstToolSchema(t, body)
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	property, ok := properties[propertyName].(map[string]any)
	require.True(t, ok)
	return property
}
