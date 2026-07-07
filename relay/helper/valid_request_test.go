package helper

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateClaudeRequestReturnsCompatErrorBeforeDTOUnmarshal(t *testing.T) {
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	model_setting.GetClaudeSettings().RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":"bad"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateClaudeRequest(c)
	require.Nil(t, request)
	require.Error(t, err)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "messages", apiErr.ToOpenAIError().Param)
	require.Equal(t, relaycommon.ClaudeCompatCodeInvalidRequestSchema, apiErr.ToOpenAIError().Code)
}

func TestGetAndValidateClaudeRequestNormalizesSimpleInvalidContentBeforeDTOUnmarshal(t *testing.T) {
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	settings := model_setting.GetClaudeSettings()
	settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	settings.NormalizeSimpleMessageContentEnabled = true

	body := `{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"user","content":null},{"role":"assistant","content":[]},{"role":"user","content":123},{"role":"assistant","content":true}]}`

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateClaudeRequest(c)
	require.NoError(t, err)
	require.NotNil(t, request)
	require.Len(t, request.Messages, 4)
	require.Equal(t, " ", request.Messages[0].Content)
	require.Equal(t, " ", request.Messages[1].Content)
	require.Equal(t, "123", request.Messages[2].Content)
	require.Equal(t, "true", request.Messages[3].Content)

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	normalizedBody, err := storage.Bytes()
	require.NoError(t, err)
	normalized := string(normalizedBody)
	require.NotContains(t, normalized, `"content":null`)
	require.NotContains(t, normalized, `"content":[]`)
	require.Contains(t, normalized, `"content":" "`)
	require.Contains(t, normalized, `"content":"123"`)
	require.Contains(t, normalized, `"content":"true"`)
}

func TestGetAndValidateClaudeRequestSimpleContentDisabledRejects(t *testing.T) {
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	settings := model_setting.GetClaudeSettings()
	settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	settings.NormalizeSimpleMessageContentEnabled = false

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"user","content":null}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateClaudeRequest(c)
	require.Nil(t, request)
	require.Error(t, err)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "messages.0.content", apiErr.ToOpenAIError().Param)
	require.Equal(t, relaycommon.ClaudeCompatCodeInvalidRequestSchema, apiErr.ToOpenAIError().Code)
}

func TestGetAndValidateClaudeRequestNormalizesOpenAIStyleRolesBeforeSchemaValidation(t *testing.T) {
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	settings := model_setting.GetClaudeSettings()
	settings.ApplyCompatInPassthroughEnabled = true
	settings.RequestSchemaValidationMode = model_setting.ClaudeValidationModeReject
	settings.PromoteLeadingSystemRoleEnabled = true
	settings.MergeAdjacentSameRoleEnabled = true

	body := `{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"sys"},{"role":"developer","content":"dev"},{"role":"user","content":"first"},{"role":"user","content":"second"}]}`

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateClaudeRequest(c)
	require.NoError(t, err)
	require.NotNil(t, request)
	require.Len(t, request.Messages, 1)
	require.Equal(t, "user", request.Messages[0].Role)
	require.NotNil(t, request.System)

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	normalizedBody, err := storage.Bytes()
	require.NoError(t, err)
	normalized := string(normalizedBody)
	require.NotContains(t, normalized, `"role":"developer"`)
	require.NotContains(t, normalized, `"role":"system"`)
	require.Contains(t, normalized, `"system"`)
	require.Contains(t, normalized, `"text":"sys"`)
	require.Contains(t, normalized, `"text":"dev"`)
}

func newJSONValidationContext(path string, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestMaxTokensBounds(t *testing.T) {
	limit := maxTokensLimit
	tests := []struct {
		name    string
		body    string
		call    func(*gin.Context) (any, error)
		wantErr string
	}{
		{
			name: "openai max_tokens above limit rejected",
			body: fmt.Sprintf(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}],"max_tokens":%d}`, limit+1),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
			},
			wantErr: "max_tokens is invalid",
		},
		{
			name: "openai max_completion_tokens above limit rejected",
			body: fmt.Sprintf(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":%d}`, limit+1),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
			},
			wantErr: "max_tokens is invalid",
		},
		{
			name: "responses max_output_tokens above limit rejected",
			body: fmt.Sprintf(`{"model":"gpt-test","input":"hi","max_output_tokens":%d}`, limit+1),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateResponsesRequest(c)
			},
			wantErr: "max_output_tokens is invalid",
		},
		{
			name: "gemini maxOutputTokens above limit rejected",
			body: fmt.Sprintf(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":%d}}`, limit+1),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateGeminiRequest(c)
			},
			wantErr: "maxOutputTokens is invalid",
		},
		{
			name: "gemini max_output_tokens above limit rejected",
			body: fmt.Sprintf(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"max_output_tokens":%d}}`, limit+1),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateGeminiRequest(c)
			},
			wantErr: "maxOutputTokens is invalid",
		},
		{
			name: "openai max_tokens at limit accepted",
			body: fmt.Sprintf(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}],"max_tokens":%d}`, limit),
			call: func(c *gin.Context) (any, error) {
				return GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call(newJSONValidationContext("/v1/chat/completions", tc.body))
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestClaudeMaxTokensBounds(t *testing.T) {
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	model_setting.GetClaudeSettings().RequestSchemaValidationMode = model_setting.ClaudeValidationModeOff

	body := fmt.Sprintf(`{"model":"claude-sonnet-4-6","max_tokens_to_sample":%d,"messages":[{"role":"user","content":"hi"}]}`, maxTokensLimit+1)
	_, err := GetAndValidateClaudeRequest(newJSONValidationContext("/v1/messages", body))
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_tokens is invalid")
}

func TestGetAndValidOpenAIImageRequestNBounds(t *testing.T) {
	boundErr := fmt.Sprintf("n must be an integer between 1 and %d", dto.MaxImageN)
	tests := []struct {
		name    string
		body    string
		wantErr string
		wantN   uint
	}{
		{
			name:    "json n above max rejected",
			body:    fmt.Sprintf(`{"model":"gpt-image-1","prompt":"cat","n":%d}`, dto.MaxImageN+1),
			wantErr: boundErr,
		},
		{
			name:  "json n at max accepted",
			body:  fmt.Sprintf(`{"model":"gpt-image-1","prompt":"cat","n":%d}`, dto.MaxImageN),
			wantN: dto.MaxImageN,
		},
		{
			name:  "missing n defaults to one",
			body:  `{"model":"gpt-image-1","prompt":"cat"}`,
			wantN: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := GetAndValidOpenAIImageRequest(newJSONValidationContext("/v1/images/generations", tc.body), relayconstant.RelayModeImagesGenerations)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, req.N)
			require.Equal(t, tc.wantN, *req.N)
		})
	}

	t.Run("multipart n above max rejected", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "gpt-image-1"))
		require.NoError(t, writer.WriteField("prompt", "cat"))
		require.NoError(t, writer.WriteField("n", fmt.Sprintf("%d", dto.MaxImageN+1)))
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())

		_, err := GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesEdits)
		require.Error(t, err)
		require.Contains(t, err.Error(), boundErr)
	})
}
