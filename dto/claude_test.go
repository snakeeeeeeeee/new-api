package dto

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeResponseGetClaudeErrorPreservesCompatFields(t *testing.T) {
	response := ClaudeResponse{
		Error: map[string]interface{}{
			"type":    "invalid_request_error",
			"message": "Invalid request for Claude: messages.0.content.0.source.data is not valid base64 image data.",
			"param":   "messages.0.content.0.source.data",
			"code":    "claude_invalid_image_base64",
			"status":  float64(http.StatusBadRequest),
		},
	}

	got := response.GetClaudeError()

	require.NotNil(t, got)
	require.Equal(t, "invalid_request_error", got.Type)
	require.Equal(t, "messages.0.content.0.source.data", got.Param)
	require.Equal(t, "claude_invalid_image_base64", got.Code)
	require.Equal(t, http.StatusBadRequest, got.Status)
	require.Contains(t, got.Message, "messages.0.content.0.source.data")
}
