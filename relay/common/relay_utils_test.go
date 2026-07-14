package common

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTaskValidationContext(body string) (*gin.Context, *RelayInfo) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
}

func newMultipartTaskValidationContext(t *testing.T, fields map[string]string) (*gin.Context, *RelayInfo) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
}

func TestTaskQuantityBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "huge duration rejected",
			body:    fmt.Sprintf(`{"model":"sora-2","prompt":"cat","duration":%d}`, MaxTaskDurationSeconds+1),
			wantErr: "invalid_seconds",
		},
		{
			name:    "huge seconds rejected",
			body:    fmt.Sprintf(`{"model":"sora-2","prompt":"cat","seconds":"%d"}`, MaxTaskDurationSeconds+1),
			wantErr: "invalid_seconds",
		},
		{
			name:    "bad seconds rejected",
			body:    `{"model":"sora-2","prompt":"cat","seconds":"bad"}`,
			wantErr: "invalid_seconds",
		},
		{
			name:    "negative duration rejected",
			body:    `{"model":"sora-2","prompt":"cat","duration":-1}`,
			wantErr: "invalid_seconds",
		},
		{
			name:    "metadata duration rejected",
			body:    fmt.Sprintf(`{"model":"veo","prompt":"cat","metadata":{"durationSeconds":%d}}`, MaxTaskDurationSeconds+1),
			wantErr: "invalid_seconds",
		},
		{
			name:    "async image n rejected",
			body:    `{"model":"gpt-image-2","prompt":"cat","metadata":{"n":129}}`,
			wantErr: "invalid_n",
		},
		{
			name: "max duration accepted",
			body: fmt.Sprintf(`{"model":"sora-2","prompt":"cat","duration":%d}`, MaxTaskDurationSeconds),
		},
		{
			name: "default duration accepted",
			body: `{"model":"sora-2","prompt":"cat"}`,
		},
		{
			name: "max async image n accepted",
			body: `{"model":"gpt-image-2","prompt":"cat","metadata":{"n":128}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, info := newTaskValidationContext(tc.body)
			err := ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
			if tc.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErr, err.Code)
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestMultipartTaskQuantityBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		fields  map[string]string
		wantErr string
	}{
		{
			name: "bad seconds rejected",
			fields: map[string]string{
				"model":   "sora-2",
				"prompt":  "cat",
				"seconds": "bad",
			},
			wantErr: "invalid_seconds",
		},
		{
			name: "bad duration rejected",
			fields: map[string]string{
				"model":    "sora-2",
				"prompt":   "cat",
				"duration": "bad",
			},
			wantErr: "invalid_seconds",
		},
		{
			name: "max seconds accepted",
			fields: map[string]string{
				"model":   "sora-2",
				"prompt":  "cat",
				"seconds": fmt.Sprintf("%d", MaxTaskDurationSeconds),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, info := newMultipartTaskValidationContext(t, tc.fields)
			err := ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
			if tc.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErr, err.Code)
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestMultipartTaskPreservesResponseFormat(t *testing.T) {
	c, info := newMultipartTaskValidationContext(t, map[string]string{
		"model":           "gpt-image-2",
		"prompt":          "edit test",
		"quality":         "high",
		"resolution":      "2k",
		"response_format": "b64_json",
	})

	taskErr := ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
	require.Nil(t, taskErr)
	req, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.NotNil(t, req.Quality)
	require.Equal(t, "high", *req.Quality)
	require.NotNil(t, req.Resolution)
	require.Equal(t, "2k", *req.Resolution)
	require.NotNil(t, req.ResponseFormat)
	require.Equal(t, "b64_json", *req.ResponseFormat)
	require.NotContains(t, req.Metadata, "response_format")
}
