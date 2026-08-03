package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestOpenAIVideoPollNotFoundGrace(t *testing.T) {
	const now int64 = 1_800_000_000
	compatible := &model.Task{
		SubmitTime: now - 1,
		PrivateData: model.TaskPrivateData{OpenAIVideoCompatibility: &dto.OpenAIVideoCompatibilityMetadata{
			Version: dto.OpenAIVideoCompatibilityVersion,
		}},
	}

	assert.True(t, isTransientTaskPollHTTPStatus(compatible, http.StatusNotFound, now))

	compatible.SubmitTime = now - openAIVideoNotFoundGraceSeconds
	assert.False(t, isTransientTaskPollHTTPStatus(compatible, http.StatusNotFound, now))

	legacy := &model.Task{SubmitTime: now - 1}
	assert.False(t, isTransientTaskPollHTTPStatus(legacy, http.StatusNotFound, now))
	assert.True(t, isTransientTaskPollHTTPStatus(legacy, http.StatusServiceUnavailable, now))
}
