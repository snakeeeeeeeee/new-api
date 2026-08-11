package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdobeVideoUsesNormalizedVideoTaskEndpoint(t *testing.T) {
	assert.Equal(
		t,
		[]constant.EndpointType{constant.EndpointTypeVideoTask},
		GetEndpointTypesByChannelType(constant.ChannelTypeAdobeVideo, "seedance-2.0-fast-480p"),
	)
	endpoint, ok := GetDefaultEndpointInfo(constant.EndpointTypeVideoTask)
	require.True(t, ok)
	assert.Equal(t, "/v1/video/tasks", endpoint.Path)
	assert.Equal(t, "POST", endpoint.Method)
}

func TestLeonardoVideoUsesNormalizedVideoTaskEndpoint(t *testing.T) {
	assert.Equal(
		t,
		[]constant.EndpointType{constant.EndpointTypeVideoTask},
		GetEndpointTypesByChannelType(constant.ChannelTypeLeonardoVideo, "leonardo-seedance-2.0-fast-480p"),
	)
}

func TestGrok720pVideoModelsIncludeNormalizedVideoTaskEndpoint(t *testing.T) {
	want := []constant.EndpointType{
		constant.EndpointTypeVideoTask,
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	for _, modelName := range []string{
		"grok-imagine-video-720p",
		"grok-imagine-video-1.5-preview-720p",
	} {
		assert.Equal(t, want, GetEndpointTypesByChannelType(constant.ChannelTypeXai, modelName))
	}
}

func TestXAIImageAndTextEndpointInferenceRemainsUnchanged(t *testing.T) {
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeImageGeneration,
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, GetEndpointTypesByChannelType(constant.ChannelTypeXai, "grok-imagine-image"))
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, GetEndpointTypesByChannelType(constant.ChannelTypeXai, "grok-4-0709"))
}
