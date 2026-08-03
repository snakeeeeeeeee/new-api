package common

import (
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

const (
	VideoTaskPublicRequestContextKey     = "video_task_public_request"
	VideoTaskPublicRequestJSONContextKey = "video_task_public_request_json"
	VideoTaskFingerprintContextKey       = "video_task_request_fingerprint"
	VideoTaskIdempotencyKeyContextKey    = "video_task_idempotency_key"
	OpenAIVideoCompatibilityContextKey   = "openai_video_compatibility"
	OpenAIVideoModelContextKey           = "openai_video_model"
)

func GetVideoTaskPublicRequest(c *gin.Context) (dto.VideoTaskCreateRequest, error) {
	if c == nil {
		return dto.VideoTaskCreateRequest{}, fmt.Errorf("video task request context is nil")
	}
	value, exists := c.Get(VideoTaskPublicRequestContextKey)
	if !exists {
		return dto.VideoTaskCreateRequest{}, fmt.Errorf("normalized video task request is missing")
	}
	request, ok := value.(dto.VideoTaskCreateRequest)
	if !ok {
		return dto.VideoTaskCreateRequest{}, fmt.Errorf("normalized video task request is invalid")
	}
	return request, nil
}

func GetOpenAIVideoCompatibility(c *gin.Context) (dto.OpenAIVideoCompatibilityMetadata, bool) {
	if c == nil {
		return dto.OpenAIVideoCompatibilityMetadata{}, false
	}
	value, exists := c.Get(OpenAIVideoCompatibilityContextKey)
	if !exists {
		return dto.OpenAIVideoCompatibilityMetadata{}, false
	}
	metadata, ok := value.(dto.OpenAIVideoCompatibilityMetadata)
	return metadata, ok && metadata.Version != ""
}
