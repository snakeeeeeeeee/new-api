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
