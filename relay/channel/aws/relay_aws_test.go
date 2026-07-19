package aws

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockruntimeTypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeAwsClaudeIntegrityResponseStream struct {
	events chan bedrockruntimeTypes.ResponseStream
	err    error
}

func (s *fakeAwsClaudeIntegrityResponseStream) Events() <-chan bedrockruntimeTypes.ResponseStream {
	return s.events
}

func (s *fakeAwsClaudeIntegrityResponseStream) Err() error {
	return s.err
}

func TestAwsClaudeIntegrityStreamBodyPreservesChunksAndPropagatesStreamError(t *testing.T) {
	events := make(chan bedrockruntimeTypes.ResponseStream, 2)
	events <- &bedrockruntimeTypes.ResponseStreamMemberChunk{Value: bedrockruntimeTypes.PayloadPart{Bytes: []byte(`{"type":"message_start"}`)}}
	events <- &bedrockruntimeTypes.ResponseStreamMemberChunk{Value: bedrockruntimeTypes.PayloadPart{Bytes: []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`)}}
	close(events)
	body := awsClaudeIntegrityStreamBody(&fakeAwsClaudeIntegrityResponseStream{events: events}, make(chan struct{}))
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t,
		"data: {\"type\":\"message_start\"}\n\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n",
		string(data),
	)

	sentinel := errors.New("synthetic Bedrock stream failure")
	errorEvents := make(chan bedrockruntimeTypes.ResponseStream)
	close(errorEvents)
	errorBody := awsClaudeIntegrityStreamBody(&fakeAwsClaudeIntegrityResponseStream{events: errorEvents, err: sentinel}, make(chan struct{}))
	_, err = io.ReadAll(errorBody)
	require.ErrorIs(t, err, sentinel)
}

func TestAwsClaudeIntegrityInvokeErrorPreservesFirstBlockTimeoutClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta:                              &relaycommon.ChannelMeta{},
		IsStream:                                 true,
		ClaudeResponseIntegrityEnabled:           true,
		ClaudeResponseIntegrityFirstBlockTimeout: time.Millisecond,
	}

	attemptCtx := info.BeginClaudeResponseIntegrityAttempt(context.Background())
	select {
	case <-attemptCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("Claude integrity attempt did not reach the first-block timeout")
	}

	apiErr := awsClaudeIntegrityInvokeError(c, info, "InvokeModelWithResponseStream", attemptCtx.Err())
	info.EndClaudeResponseIntegrityAttempt()

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
}

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
	require.Len(t, info.ClaudeToolSchemaCompatFinalSchemas, 1)
	require.Equal(t, "custom", info.ClaudeToolSchemaCompatFinalSchemas[0].ToolName)
	finalSchema := info.ClaudeToolSchemaCompatFinalSchemas[0].InputSchema.(map[string]any)
	require.NotContains(t, finalSchema, "required")
	require.Equal(t, map[string]any{}, finalSchema["properties"])
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

func TestBuildAwsRequestBodyPassThroughFixesToolSchemaBeforeCompatReject(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	oldClaudeSettings := *model_setting.GetClaudeSettings()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled = true
	model_setting.GetClaudeSettings().ToolSchemaValidationMode = model_setting.ClaudeValidationModeReject
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
		*model_setting.GetClaudeSettings() = oldClaudeSettings
	})

	rawBody := `{"model":"claude-opus-4-7","stream":true,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"custom","input_schema":null}]}`
	ctx := awsTestContext(rawBody)
	info := awsRelayInfoWithToolSchemaCompat(true, true)
	info.UpstreamModelName = "claude-opus-4-7"
	body, err := buildAwsRequestBody(ctx, info, &AwsClaudeRequest{})
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.NotContains(t, payload, "model")
	schema := decodeFirstToolSchema(t, body)
	require.Equal(t, "object", schema["type"])
	require.Equal(t, map[string]any{}, schema["properties"])
}

func TestBuildAwsRequestBodyPassThroughConvertsOpenAIToolCallsToClaude(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	oldClaudeSettings := *model_setting.GetClaudeSettings()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	model_setting.GetClaudeSettings().OpenAIToolCallCompatEnabled = true
	model_setting.GetClaudeSettings().ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
		*model_setting.GetClaudeSettings() = oldClaudeSettings
	})

	rawBody := `{
		"model":"claude-opus-4-7",
		"stream":true,
		"messages":[
			{"role":"user","content":"check"},
			{"role":"assistant","content":"Let me check."},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"code\" title=\"Open page\": \"bad json\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"ok"},
			{"role":"user","content":"done"}
		],
		"tools":[{"type":"function","function":{"name":"exec","description":"run","parameters":{"type":"object","properties":{"code":{"type":"string"}}}}}]
	}`
	ctx := awsTestContext(rawBody)
	info := awsRelayInfoWithToolSchemaCompat(false, true)
	info.RelayFormat = "openai"
	info.UpstreamModelName = "claude-opus-4-7"

	body, err := buildAwsRequestBody(ctx, info, &AwsClaudeRequest{AnthropicVersion: "bedrock-2023-05-31"})
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.NotContains(t, payload, "model")
	require.NotContains(t, payload, "stream")
	require.Equal(t, "bedrock-2023-05-31", payload["anthropic_version"])
	require.NotContains(t, string(body), `"tool_calls"`)
	require.NotContains(t, string(body), `"tool_call_id"`)

	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 4)
	assistantMessage, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assistantContents, ok := assistantMessage["content"].([]any)
	require.True(t, ok)
	require.Len(t, assistantContents, 2)
	require.Equal(t, "text", assistantContents[0].(map[string]any)["type"])
	toolUse := assistantContents[1].(map[string]any)
	require.Equal(t, "tool_use", toolUse["type"])
	require.Equal(t, "call_1", toolUse["id"])
	require.Equal(t, map[string]any{"_raw_arguments": `{"code" title="Open page": "bad json"}`}, toolUse["input"])

	resultMessage, ok := messages[2].(map[string]any)
	require.True(t, ok)
	resultContents, ok := resultMessage["content"].([]any)
	require.True(t, ok)
	require.Len(t, resultContents, 1)
	require.Equal(t, "tool_result", resultContents[0].(map[string]any)["type"])
	require.Equal(t, "call_1", resultContents[0].(map[string]any)["tool_use_id"])
}

func TestBuildAwsRequestBodyToolSchemaCompatWhitelistMissKeepsSchema(t *testing.T) {
	oldPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPassThrough
	})

	info := awsRelayInfoWithToolSchemaCompat(true, false)
	info.UserId = 256
	info.ChannelOtherSettings.ClaudeToolSchemaCompatUserIDs = []int{1001}
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
	require.Empty(t, info.ClaudeToolSchemaCompatFinalSchemas)

	passThroughInfo := awsRelayInfoWithToolSchemaCompat(true, true)
	passThroughInfo.UserId = 256
	passThroughInfo.ChannelOtherSettings.ClaudeToolSchemaCompatUserIDs = []int{1001}
	rawBody := `{"model":"claude-opus-4-7","stream":true,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"custom","input_schema":{"type":"","properties":null,"required":null}}]}`
	ctx := awsTestContext(rawBody)
	body, err = buildAwsRequestBody(ctx, passThroughInfo, &AwsClaudeRequest{})
	require.NoError(t, err)
	schema = decodeFirstToolSchema(t, body)
	require.Equal(t, "", schema["type"])
	require.Nil(t, schema["required"])
	require.Nil(t, schema["properties"])
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
