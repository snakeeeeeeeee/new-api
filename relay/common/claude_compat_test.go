package common

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	commonpkg "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func withClaudeSettings(t *testing.T, mutate func(*model_setting.ClaudeSettings)) {
	t.Helper()
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	mutate(model_setting.GetClaudeSettings())
}

func baseClaudeCompatRequest(model string) *dto.ClaudeRequest {
	return &dto.ClaudeRequest{
		Model: model,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
}

func TestNormalizeClaudeRequestCompatFixesImageMediaTypeMismatch(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.AutoFixImageMediaTypeEnabled = true
	})
	cases := []struct {
		name      string
		declared  string
		bytes     []byte
		wantMedia string
	}{
		{name: "jpeg", declared: "image/png", bytes: []byte{0xff, 0xd8, 0xff, 0x00}, wantMedia: "image/jpeg"},
		{name: "png", declared: "image/jpeg", bytes: []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, wantMedia: "image/png"},
		{name: "gif", declared: "image/png", bytes: []byte("GIF89a"), wantMedia: "image/gif"},
		{name: "webp", declared: "image/jpeg", bytes: append([]byte("RIFFxxxxWEBP"), 0x00), wantMedia: "image/webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseClaudeCompatRequest("claude-sonnet-4-6")
			req.Messages[0].Content = []dto.ClaudeMediaMessage{
				{
					Type: "image",
					Source: &dto.ClaudeMessageSource{
						Type:      "base64",
						MediaType: tc.declared,
						Data:      base64.StdEncoding.EncodeToString(tc.bytes),
					},
				},
			}

			err := NormalizeClaudeRequestCompat(req, nil)
			require.Nil(t, err)
			contents, parseErr := req.Messages[0].ParseContent()
			require.NoError(t, parseErr)
			require.Equal(t, tc.wantMedia, contents[0].Source.MediaType)
		})
	}
}

func TestNormalizeClaudeRequestCompatImageMediaTypeDisabledKeepsMismatch(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.AutoFixImageMediaTypeEnabled = false
	})
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Messages[0].Content = []dto.ClaudeMediaMessage{
		{
			Type: "image",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: "image/jpeg",
				Data:      base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}),
			},
		},
	}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.Nil(t, err)
	contents, parseErr := req.Messages[0].ParseContent()
	require.NoError(t, parseErr)
	require.Equal(t, "image/jpeg", contents[0].Source.MediaType)
}

func TestNormalizeClaudeRequestCompatFixesNonStandardDeclaredImageMediaType(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.AutoFixImageMediaTypeEnabled = true
	})
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Messages[0].Content = []dto.ClaudeMediaMessage{
		{
			Type: "image",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: "image/jpg",
				Data:      base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0x00}),
			},
		},
	}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.Nil(t, err)
	contents, parseErr := req.Messages[0].ParseContent()
	require.NoError(t, parseErr)
	require.Equal(t, "image/jpeg", contents[0].Source.MediaType)
}

func TestNormalizeClaudeRequestCompatInvalidImageDataReturns400(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.AutoFixImageMediaTypeEnabled = true
	})
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Messages[0].Content = []dto.ClaudeMediaMessage{
		{
			Type: "image",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "not-base64",
			},
		},
	}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, err.StatusCode)
	openAIError := err.ToOpenAIError()
	require.Equal(t, "invalid_request_error", openAIError.Type)
	require.Equal(t, "messages.0.content.0.source.data", openAIError.Param)
	require.Equal(t, ClaudeCompatCodeInvalidImageBase64, openAIError.Code)
	require.Contains(t, openAIError.Message, "messages.0.content.0.source.data")
}

func TestNormalizeClaudeRequestCompatMaxTokensDefaultAndZero(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.DefaultMaxTokens = map[string]int{"default": 8192, "claude-sonnet-4-6": 4096}
		settings.PreserveZeroMaxTokensEnabled = true
	})

	nilReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	require.Nil(t, NormalizeClaudeRequestCompat(nilReq, nil))
	require.NotNil(t, nilReq.MaxTokens)
	require.Equal(t, uint(4096), *nilReq.MaxTokens)

	zero := uint(0)
	zeroReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	zeroReq.MaxTokens = &zero
	require.Nil(t, NormalizeClaudeRequestCompat(zeroReq, nil))
	require.Equal(t, uint(0), *zeroReq.MaxTokens)

	positive := uint(32)
	positiveReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	positiveReq.MaxTokens = &positive
	require.Nil(t, NormalizeClaudeRequestCompat(positiveReq, nil))
	require.Equal(t, uint(32), *positiveReq.MaxTokens)
}

func TestNormalizeClaudeRequestCompatZeroMaxTokensConflict(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PreserveZeroMaxTokensEnabled = true
	})
	zero := uint(0)
	stream := true
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.MaxTokens = &zero
	req.Stream = &stream
	req.Thinking = &dto.Thinking{Type: "enabled"}
	req.OutputFormat = []byte(`{"type":"json_schema"}`)
	req.ToolChoice = dto.ClaudeToolChoice{Type: "tool", Name: "call_me"}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.NotNil(t, err)
	openAIError := err.ToOpenAIError()
	require.Equal(t, "max_tokens", openAIError.Param)
	require.Equal(t, ClaudeCompatCodeZeroMaxTokensIncompatible, openAIError.Code)
	require.Contains(t, openAIError.Message, "stream")
	require.Contains(t, openAIError.Message, "thinking")
	require.Contains(t, openAIError.Message, "output_format")
	require.Contains(t, openAIError.Message, "tool_choice")
}

func TestNormalizeClaudeRequestCompatZeroMaxTokensUsesRelayInfoStream(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PreserveZeroMaxTokensEnabled = true
	})
	zero := uint(0)
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.MaxTokens = &zero

	err := NormalizeClaudeRequestCompat(req, &RelayInfo{IsStream: true})
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeZeroMaxTokensIncompatible, err.ToOpenAIError().Code)
	require.Contains(t, err.ToOpenAIError().Message, "stream")
}

func TestNormalizeClaudeRequestCompatOpusSampling(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.DropDefaultSamplingForOpusEnabled = true
	})
	defaultReq := baseClaudeCompatRequest("claude-opus-4-7")
	defaultReq.Temperature = commonpkg.GetPointer(1.0)
	defaultReq.TopP = commonpkg.GetPointer(1.0)
	require.Nil(t, NormalizeClaudeRequestCompat(defaultReq, nil))
	require.Nil(t, defaultReq.Temperature)
	require.Nil(t, defaultReq.TopP)

	compatibleTopPReq := baseClaudeCompatRequest("claude-opus-4-7")
	compatibleTopPReq.TopP = commonpkg.GetPointer(0.99)
	require.Nil(t, NormalizeClaudeRequestCompat(compatibleTopPReq, nil))
	require.Nil(t, compatibleTopPReq.TopP)

	nonDefaultReq := baseClaudeCompatRequest("claude-opus-4-7")
	nonDefaultReq.Temperature = commonpkg.GetPointer(0.2)
	require.Nil(t, NormalizeClaudeRequestCompat(nonDefaultReq, nil))
	require.Nil(t, nonDefaultReq.Temperature)

	topKReq := baseClaudeCompatRequest("claude-opus-4-7")
	topKReq.TopK = commonpkg.GetPointer(250)
	require.Nil(t, NormalizeClaudeRequestCompat(topKReq, nil))
	require.Nil(t, topKReq.TopK)

	sonnetReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	sonnetReq.Temperature = commonpkg.GetPointer(1.0)
	require.Nil(t, NormalizeClaudeRequestCompat(sonnetReq, nil))
	require.NotNil(t, sonnetReq.Temperature)
}

func TestNormalizeClaudeRequestCompatEffortValidation(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.ValidateOutputEffortEnabled = true
	})
	xhighReq := baseClaudeCompatRequest("claude-opus-4-7")
	xhighReq.OutputConfig = []byte(`{"effort":"xhigh"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(xhighReq, nil))

	sonnetReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	sonnetReq.OutputConfig = []byte(`{"effort":"xhigh"}`)
	err := NormalizeClaudeRequestCompat(sonnetReq, nil)
	require.NotNil(t, err)
	require.Equal(t, "output_config.effort", err.ToOpenAIError().Param)

	sonnetMaxReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	sonnetMaxReq.OutputConfig = []byte(`{"effort":"max"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(sonnetMaxReq, nil))

	maxReq := baseClaudeCompatRequest("claude-opus-4-6")
	maxReq.OutputConfig = []byte(`{"effort":"max"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(maxReq, nil))

	opus46XHighReq := baseClaudeCompatRequest("claude-opus-4-6")
	opus46XHighReq.OutputConfig = []byte(`{"effort":"xhigh"}`)
	err = NormalizeClaudeRequestCompat(opus46XHighReq, nil)
	require.NotNil(t, err)
	require.Equal(t, "output_config.effort", err.ToOpenAIError().Param)

	opus48MaxReq := baseClaudeCompatRequest("claude-opus-4-8")
	opus48MaxReq.OutputConfig = []byte(`{"effort":"max"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(opus48MaxReq, nil))

	fableMaxReq := baseClaudeCompatRequest("claude-fable-5")
	fableMaxReq.OutputConfig = []byte(`{"effort":"max"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(fableMaxReq, nil))

	fableXHighReq := baseClaudeCompatRequest("claude-fable-5")
	fableXHighReq.OutputConfig = []byte(`{"effort":"xhigh"}`)
	require.Nil(t, NormalizeClaudeRequestCompat(fableXHighReq, nil))

	unknownReq := baseClaudeCompatRequest("claude-opus-4-7")
	unknownReq.OutputConfig = []byte(`{"effort":"extreme"}`)
	err = NormalizeClaudeRequestCompat(unknownReq, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidOutputEffort, err.ToOpenAIError().Code)
}

func TestNormalizeClaudeRequestCompatToolResultReorderAndMismatch(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.ReorderToolResultBlocksEnabled = true
	})
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Messages = []dto.ClaudeMessage{
		{
			Role: "assistant",
			Content: []dto.ClaudeMediaMessage{
				{Type: "tool_use", Id: "call_1", Name: "lookup", Input: map[string]any{"q": "x"}},
			},
		},
		{
			Role: "user",
			Content: []dto.ClaudeMediaMessage{
				{Type: "text", Text: commonpkg.GetPointer("done")},
				{Type: "tool_result", ToolUseId: "call_1", Content: "ok"},
			},
		},
	}

	require.Nil(t, NormalizeClaudeRequestCompat(req, nil))
	contents, parseErr := req.Messages[1].ParseContent()
	require.NoError(t, parseErr)
	require.Equal(t, "tool_result", contents[0].Type)
	require.Equal(t, "text", contents[1].Type)

	badReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	badReq.Messages = []dto.ClaudeMessage{
		{Role: "assistant", Content: []dto.ClaudeMediaMessage{{Type: "tool_use", Id: "call_1", Name: "lookup"}}},
		{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: "missing", Content: "ok"}}},
	}
	err := NormalizeClaudeRequestCompat(badReq, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeToolResultMismatch, err.ToOpenAIError().Code)
	require.Equal(t, "messages.1.content.0.tool_use_id", err.ToOpenAIError().Param)
}

func TestNormalizeClaudeRequestCompatNamePatternAndClaudeEnvelope(t *testing.T) {
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Tools = []any{map[string]any{"name": "bad.name", "input_schema": map[string]any{"type": "object"}}}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.NotNil(t, err)
	openAIError := err.ToOpenAIError()
	require.Equal(t, "invalid_request_error", openAIError.Type)
	require.Equal(t, "tools.0.name", openAIError.Param)
	require.Equal(t, ClaudeCompatCodeInvalidNamePattern, openAIError.Code)

	claudeError := err.ToClaudeError()
	require.Equal(t, "invalid_request_error", claudeError.Type)
	require.Contains(t, claudeError.Message, "tools.0.name")
}

func TestNormalizeClaudeRequestCompatJSONPreservesUnknownFields(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.AutoFixImageMediaTypeEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","unknown_beta":{"keep":true},"messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"` + base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) + `"}}]}]}`)

	out, err := NormalizeClaudeRequestCompatJSON(body, nil)
	require.Nil(t, err)
	require.Contains(t, string(out), `"unknown_beta"`)
	require.Contains(t, string(out), `"image/png"`)
}

func TestNormalizeClaudeRequestCompatJSONUsesRelayInfoModelWhenBodyOmitsModel(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.DropDefaultSamplingForOpusEnabled = true
		settings.ValidateOutputEffortEnabled = true
	})
	body := []byte(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":32,"temperature":1,"output_config":{"effort":"xhigh"}}`)
	info := &RelayInfo{ChannelMeta: &ChannelMeta{UpstreamModelName: "claude-opus-4-7"}}

	out, err := NormalizeClaudeRequestCompatJSON(body, info)
	require.Nil(t, err)
	require.NotContains(t, string(out), `"temperature"`)
	require.Contains(t, string(out), `"output_config"`)

	sonnetInfo := &RelayInfo{ChannelMeta: &ChannelMeta{UpstreamModelName: "claude-sonnet-4-6"}}
	_, err = NormalizeClaudeRequestCompatJSON(body, sonnetInfo)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidOutputEffort, err.ToOpenAIError().Code)
}

func TestNormalizeClaudeRequestCompatJSONNormalizesSimpleInvalidContent(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
		settings.NormalizeSimpleMessageContentEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":null},{"role":"assistant","content":[]},{"role":"user","content":123},{"role":"assistant","content":true}]}`)

	out, err := NormalizeClaudeRequestCompatJSON(body, nil)

	require.Nil(t, err)
	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	messages := payload["messages"].([]any)
	require.Equal(t, claudeCompatEmptyContentText, messages[0].(map[string]any)["content"])
	require.Equal(t, claudeCompatEmptyContentText, messages[1].(map[string]any)["content"])
	require.Equal(t, "123", messages[2].(map[string]any)["content"])
	require.Equal(t, "true", messages[3].(map[string]any)["content"])
}

func TestNormalizeClaudeRequestCompatJSONSimpleContentDisabledRejects(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
		settings.NormalizeSimpleMessageContentEnabled = false
	})
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":null}]}`)

	_, err := NormalizeClaudeRequestCompatJSON(body, nil)

	require.NotNil(t, err)
	require.Equal(t, "messages.0.content", err.ToOpenAIError().Param)
	require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)
}

func TestNormalizeClaudeRequestCompatJSONRejectsObjectContent(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	})
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":{}}]}`)

	_, err := NormalizeClaudeRequestCompatJSON(body, nil)

	require.NotNil(t, err)
	require.Equal(t, "messages.0.content", err.ToOpenAIError().Param)
	require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)
}

func TestNormalizeClaudeRequestCompatJSONPromotesOpenAIStyleLeadingSystemAndDeveloper(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"leading system"},{"role":"developer","content":"leading developer"},{"role":"user","content":"a"},{"role":"user","content":"b"},{"role":"assistant","content":"c"},{"role":"assistant","content":"d"},{"role":"user","content":"finish"}]}`)
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	out, err := NormalizeClaudeRequestCompatJSON(body, info)
	require.Nil(t, err)
	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	system, ok := payload["system"].([]any)
	require.True(t, ok)
	require.Len(t, system, 2)
	require.Equal(t, "leading system", system[0].(map[string]any)["text"])
	require.Equal(t, "leading developer", system[1].(map[string]any)["text"])
	messages := payload["messages"].([]any)
	require.Len(t, messages, 3)
	require.Equal(t, "a\nb", messages[0].(map[string]any)["content"])
	require.Equal(t, "c\nd", messages[1].(map[string]any)["content"])
	require.Equal(t, "finish", messages[2].(map[string]any)["content"])
}

func TestNormalizeClaudeOpenAIStyleMessagesJSONOnlyNormalizesRoles(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.AutoFixImageMediaTypeEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"sys"},{"role":"developer","content":"dev"},{"role":"user","content":"first"},{"role":"user","content":"second"}],"temperature":0.7}`)

	out, err := NormalizeClaudeOpenAIStyleMessagesJSON(body)
	require.Nil(t, err)

	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	require.Equal(t, 0.7, payload["temperature"])
	require.NotNil(t, payload["system"])
	messages := payload["messages"].([]any)
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].(map[string]any)["role"])
}

func TestNormalizeClaudeRequestCompatJSONPromotesOpenAIStyleWhenFinalFormatIsClaude(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"leading system"},{"role":"developer","content":"leading developer"},{"role":"user","content":"finish"}]}`)
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI},
		FinalRequestRelayFormat: types.RelayFormatClaude,
	}

	out, err := NormalizeClaudeRequestCompatJSON(body, info)
	require.Nil(t, err)
	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	system, ok := payload["system"].([]any)
	require.True(t, ok)
	require.Len(t, system, 2)
	require.Equal(t, "leading system", system[0].(map[string]any)["text"])
	require.Equal(t, "leading developer", system[1].(map[string]any)["text"])
	messages := payload["messages"].([]any)
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].(map[string]any)["role"])
}

func TestNormalizeClaudeRequestCompatJSONPromotesOpenAIStyleAfterFinalFormatMarked(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"leading system"},{"role":"developer","content":"leading developer"},{"role":"user","content":"finish"}]}`)
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI},
	}
	info.MarkFinalRequestRelayFormat(types.RelayFormatClaude)

	out, err := NormalizeClaudeRequestCompatJSON(body, info)
	require.Nil(t, err)
	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	require.NotContains(t, string(out), `"developer"`)
	system, ok := payload["system"].([]any)
	require.True(t, ok)
	require.Len(t, system, 2)
}

func TestNormalizeClaudeRequestCompatJSONKeepsMiddleSystemInHistory(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
	})
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"user","content":"first"},{"role":"system","content":"middle system"},{"role":"user","content":"finish"}]}`)
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	out, err := NormalizeClaudeRequestCompatJSON(body, info)
	require.Nil(t, err)
	var payload map[string]any
	require.NoError(t, commonpkg.Unmarshal(out, &payload))
	require.NotContains(t, payload, "system")
	messages := payload["messages"].([]any)
	require.Len(t, messages, 3)
	middle := messages[1].(map[string]any)
	require.Equal(t, "user", middle["role"])
	require.Equal(t, "system: middle system", middle["content"])
}

func TestMergeClaudeAdjacentMessagesSkipsToolResults(t *testing.T) {
	messages := []dto.ClaudeMessage{
		{Role: "user", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "c"},
		{Role: "assistant", Content: "d"},
		{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: "call_1"}}},
		{Role: "user", Content: "after"},
	}
	merged := MergeClaudeAdjacentMessages(messages)
	require.Len(t, merged, 4)
	require.Equal(t, "a\nb", merged[0].Content)
	require.Equal(t, "c\nd", merged[1].Content)
	require.Equal(t, "user", merged[2].Role)
	require.Equal(t, "after", merged[3].Content)
}

func TestClaudeCompatErrorsDoNotExposeBase64(t *testing.T) {
	longData := strings.Repeat("a", 128)
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.Messages[0].Content = []dto.ClaudeMediaMessage{{Type: "image", Source: &dto.ClaudeMessageSource{Type: "base64", MediaType: "image/png", Data: longData}}}

	err := NormalizeClaudeRequestCompat(req, nil)
	require.NotNil(t, err)
	require.NotContains(t, err.ToOpenAIError().Message, longData)
}

func TestNormalizeClaudeRequestCompatJSONSchemaRejectsHardErrors(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	})
	cases := []struct {
		name      string
		body      string
		wantParam string
	}{
		{name: "top level array", body: `[]`, wantParam: ""},
		{name: "missing messages", body: `{"model":"claude-sonnet-4-6"}`, wantParam: "messages"},
		{name: "messages not array", body: `{"model":"claude-sonnet-4-6","messages":"hello"}`, wantParam: "messages"},
		{name: "message not object", body: `{"model":"claude-sonnet-4-6","messages":["bad"]}`, wantParam: "messages.0"},
		{name: "bad role", body: `{"model":"claude-sonnet-4-6","messages":[{"role":"system","content":"hello"}]}`, wantParam: "messages.0.role"},
		{name: "bad content", body: `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":{}}]}`, wantParam: "messages.0.content"},
		{name: "negative max tokens", body: `{"model":"claude-sonnet-4-6","max_tokens":-1,"messages":[{"role":"user","content":"hello"}]}`, wantParam: "max_tokens"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeClaudeRequestCompatJSON([]byte(tc.body), nil)
			require.NotNil(t, err)
			require.Equal(t, tc.wantParam, err.ToOpenAIError().Param)
			require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)
		})
	}
}

func TestValidateClaudeRequestSchemaJSONRespectsMode(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	})
	err := ValidateClaudeRequestSchemaJSON([]byte(``), nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)

	err = ValidateClaudeRequestSchemaJSON([]byte(`   `), nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)

	err = ValidateClaudeRequestSchemaJSON([]byte(`{"model":"claude-sonnet-4-6","messages":"bad"}`), nil)
	require.NotNil(t, err)
	require.Equal(t, "messages", err.ToOpenAIError().Param)
	require.Equal(t, ClaudeCompatCodeInvalidRequestSchema, err.ToOpenAIError().Code)

	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeOff
	})
	err = ValidateClaudeRequestSchemaJSON([]byte(`{"model":"claude-sonnet-4-6","messages":"bad"}`), nil)
	require.Nil(t, err)
}

func TestNormalizeClaudeRequestCompatJSONRequestSizeRejects413(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.RequestSizeLimitBytes = 8
	})
	_, err := NormalizeClaudeRequestCompatJSON([]byte(`{"messages":[]}`), nil)
	require.NotNil(t, err)
	require.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode)
	require.Equal(t, ClaudeCompatCodeRequestTooLarge, err.ToOpenAIError().Code)
	require.Equal(t, "request_too_large", err.ToOpenAIError().Type)
}

func TestNormalizeClaudeRequestCompatToolProtocolRejectsMissingAndDuplicate(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})
	missingReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	missingReq.Messages = []dto.ClaudeMessage{
		{Role: "assistant", Content: []dto.ClaudeMediaMessage{{Type: "tool_use", Id: "call_1", Name: "lookup"}}},
		{Role: "user", Content: "next"},
	}
	err := NormalizeClaudeRequestCompat(missingReq, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeMissingToolResult, err.ToOpenAIError().Code)
	require.Equal(t, "messages.1.content", err.ToOpenAIError().Param)

	duplicateReq := baseClaudeCompatRequest("claude-sonnet-4-6")
	duplicateReq.Messages = []dto.ClaudeMessage{
		{
			Role: "assistant",
			Content: []dto.ClaudeMediaMessage{
				{Type: "tool_use", Id: "call_1", Name: "lookup"},
				{Type: "tool_use", Id: "call_1", Name: "lookup"},
			},
		},
	}
	err = NormalizeClaudeRequestCompat(duplicateReq, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeDuplicateToolUseID, err.ToOpenAIError().Code)
	require.Equal(t, "messages.0.content.1.id", err.ToOpenAIError().Param)
}

func TestNormalizeClaudeRequestCompatLogOnlyModesDoNotReject(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.ToolSchemaValidationMode = model_setting.ClaudeValidationModeLog
		settings.ToolChoiceValidationMode = model_setting.ClaudeValidationModeLog
		settings.ThinkingValidationMode = model_setting.ClaudeValidationModeLog
		settings.PromptCacheValidationMode = model_setting.ClaudeValidationModeLog
		settings.MetadataUserIDValidationMode = model_setting.ClaudeValidationModeLog
		settings.ImageLimitsValidationMode = model_setting.ClaudeValidationModeLog
	})
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":4096,
		"metadata":{"user_id":"customer@example.com"},
		"cache_control":{"ttl":"bad"},
		"thinking":{"type":"enabled","budget_tokens":999},
		"tool_choice":{"type":"tool","name":"missing"},
		"tools":[{"name":"lookup","input_schema":"bad"}],
		"messages":[{"role":"user","content":"hello"}]
	}`)
	_, err := NormalizeClaudeRequestCompatJSON(body, nil)
	require.Nil(t, err)

	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.MaxTokens = commonpkg.GetPointer(uint(4096))
	req.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: commonpkg.GetPointer(999)}
	err = NormalizeClaudeRequestCompat(req, nil)
	require.Nil(t, err)
}

func TestNormalizeClaudeRequestCompatRejectModesCanRejectP1P2(t *testing.T) {
	withClaudeSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.ThinkingValidationMode = model_setting.ClaudeValidationModeReject
		settings.ToolChoiceValidationMode = model_setting.ClaudeValidationModeReject
	})
	req := baseClaudeCompatRequest("claude-sonnet-4-6")
	req.MaxTokens = commonpkg.GetPointer(uint(4096))
	req.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: commonpkg.GetPointer(999)}
	err := NormalizeClaudeRequestCompat(req, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidThinking, err.ToOpenAIError().Code)

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"tool_choice":{"type":"never"}}`)
	_, err = NormalizeClaudeRequestCompatJSON(body, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidToolChoice, err.ToOpenAIError().Code)

	fableReq := baseClaudeCompatRequest("claude-fable-5")
	fableReq.MaxTokens = commonpkg.GetPointer(uint(4096))
	fableReq.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: commonpkg.GetPointer(2048)}
	err = NormalizeClaudeRequestCompat(fableReq, nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidThinking, err.ToOpenAIError().Code)
	require.Equal(t, "thinking.type", err.ToOpenAIError().Param)
}

func TestNormalizeClaudeRequestCompatRejectsStopSequencesAndServiceTierByDefault(t *testing.T) {
	_, err := NormalizeClaudeRequestCompatJSON([]byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"stop_sequences":[""]}`), nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidStopSequences, err.ToOpenAIError().Code)

	_, err = NormalizeClaudeRequestCompatJSON([]byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"service_tier":"priority"}`), nil)
	require.NotNil(t, err)
	require.Equal(t, ClaudeCompatCodeInvalidServiceTier, err.ToOpenAIError().Code)
}
