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
