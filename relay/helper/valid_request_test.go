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
