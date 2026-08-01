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
	assert.Equal(t, constant.ChannelTypeLeonardoVideo+1, constant.ChannelTypeDummy)

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
		{constant.ChannelTypeLeonardoVideo, modelDiscoveryOpenAI, true},
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

func TestLeonardoVideoDiscoveryFiltersToSupportedSKUs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
            {"id":"seedance-2.0"},
            {"id":"seedance-2.0-fast-480p"},
            {"id":"seedance-2.0-fast-720p"},
            {"id":"seedance-2.0-480p"},
            {"id":"seedance-2.0-720p"},
            {"id":"seedance-2.0-1080p"},
            {"id":"seedance-2.0-2160p"},
            {"id":"future-model"}
        ]}`))
	}))
	defer server.Close()
	baseURL := server.URL
	models, err := fetchChannelUpstreamModelIDs(&model.Channel{
		Type: constant.ChannelTypeLeonardoVideo, Key: "provider-key", BaseURL: &baseURL,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"seedance-2.0-fast-480p", "seedance-2.0-fast-720p",
		"seedance-2.0-480p", "seedance-2.0-720p", "seedance-2.0-1080p",
	}, models)
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
