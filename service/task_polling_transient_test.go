package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestVideoPollNotFoundGrace(t *testing.T) {
	const now int64 = 1_800_000_000
	normalized := &model.Task{
		SubmitTime: now - 1,
		Properties: model.Properties{
			AssetType: constant.TaskAssetTypeVideo,
			Operation: "generation",
		},
	}

	assert.True(t, isTransientTaskPollHTTPStatus(normalized, http.StatusNotFound, now))

	normalized.SubmitTime = now - videoTaskNotFoundGraceSeconds
	assert.False(t, isTransientTaskPollHTTPStatus(normalized, http.StatusNotFound, now))

	compatible := &model.Task{
		SubmitTime: now - 1,
		PrivateData: model.TaskPrivateData{OpenAIVideoCompatibility: &dto.OpenAIVideoCompatibilityMetadata{
			Version: dto.OpenAIVideoCompatibilityVersion,
		}},
	}

	assert.True(t, isTransientTaskPollHTTPStatus(compatible, http.StatusNotFound, now))

	compatible.SubmitTime = now - videoTaskNotFoundGraceSeconds
	assert.False(t, isTransientTaskPollHTTPStatus(compatible, http.StatusNotFound, now))

	legacy := &model.Task{SubmitTime: now - 1}
	assert.False(t, isTransientTaskPollHTTPStatus(legacy, http.StatusNotFound, now))
	assert.True(t, isTransientTaskPollHTTPStatus(legacy, http.StatusServiceUnavailable, now))
}
