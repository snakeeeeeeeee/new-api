package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
