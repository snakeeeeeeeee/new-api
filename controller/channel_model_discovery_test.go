package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelModelDiscoveryRegistry(t *testing.T) {
	tests := []struct {
		channelType int
		strategy    string
		supported   bool
	}{
		{constant.ChannelTypeOpenAI, modelDiscoveryOpenAI, true},
		{constant.ChannelTypeAnthropic, modelDiscoveryAnthropic, true},
		{constant.ChannelTypeGemini, modelDiscoveryGemini, true},
		{constant.ChannelTypeOllama, modelDiscoveryOllama, true},
		{constant.ChannelTypeVolcEngine, modelDiscoveryOpenAI, true},
		{constant.ChannelTypeAdobeVideo, modelDiscoveryOpenAI, true},
		{constant.ChannelTypeHiggsfieldVideo, modelDiscoveryOpenAI, true},
		{constant.ChannelTypeMidjourney, "", false},
	}
	for _, test := range tests {
		capability := getChannelModelDiscoveryCapability(test.channelType)
		assert.Equal(t, test.supported, capability.Supported)
		assert.Equal(t, test.strategy, capability.Strategy)
		if test.supported {
			assert.Empty(t, capability.Reason)
		} else {
			assert.NotEmpty(t, capability.Reason)
		}
	}
}

func TestSelfHostedVideoChannelsDiscoverStandardModelList(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"kling_3.0_720p"},{"id":"veo_3.1_fast_720p"},{"id":"kling_3.0_720p"}]}`))
	}))
	defer server.Close()

	for _, channelType := range []int{constant.ChannelTypeAdobeVideo, constant.ChannelTypeHiggsfieldVideo} {
		baseURL := server.URL
		models, err := fetchChannelUpstreamModelIDs(&model.Channel{
			Type: channelType, Key: "provider-key", BaseURL: &baseURL,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"kling_3.0_720p", "veo_3.1_fast_720p"}, models)
		assert.Equal(t, "Bearer provider-key", authorization)
	}
}

func TestUnsupportedChannelModelDiscoveryReturnsReasonWithoutRequest(t *testing.T) {
	_, err := fetchChannelUpstreamModelIDs(&model.Channel{Type: constant.ChannelTypeMidjourney})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有已适配的模型发现接口")
}

func TestChannelResponseIncludesDiscoveryCapability(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeAdobeVideo}
	clearChannelForResponse(channel)
	assert.True(t, channel.ModelDiscoverySupported)
	assert.Equal(t, modelDiscoveryOpenAI, channel.ModelDiscoveryStrategy)
	assert.Empty(t, channel.ModelDiscoveryReason)
}
