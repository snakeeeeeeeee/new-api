package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
