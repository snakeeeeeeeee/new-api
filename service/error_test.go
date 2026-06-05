package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerPreservesClaudeCompatErrorFields(t *testing.T) {
	body := `{"type":"error","error":{"type":"invalid_request_error","message":"Invalid request for Claude: messages.0.content.0.source.data is not valid base64 image data.","param":"messages.0.content.0.source.data","code":"claude_invalid_image_base64","status":400}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	got := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, got)
	require.Equal(t, http.StatusBadRequest, got.StatusCode)
	openAIError := got.ToOpenAIError()
	require.Equal(t, "invalid_request_error", openAIError.Type)
	require.Equal(t, "messages.0.content.0.source.data", openAIError.Param)
	require.Equal(t, "claude_invalid_image_base64", openAIError.Code)
	require.Contains(t, openAIError.Message, "messages.0.content.0.source.data")
	require.NotContains(t, openAIError.Message, "***.***.***.***.***.data")
}
