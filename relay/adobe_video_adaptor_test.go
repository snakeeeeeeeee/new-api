package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdobeVideoTaskAdaptorRegistration(t *testing.T) {
	assert.Equal(t, "AdobeVideo", constant.GetChannelTypeName(constant.ChannelTypeAdobeVideo))
	require.Greater(t, len(constant.ChannelBaseURLs), constant.ChannelTypeAdobeVideo)

	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeAdobeVideo)))
	require.NotNil(t, adaptor)
	assert.Equal(t, "adobe-video", adaptor.GetChannelName())
	_, normalized := adaptor.(channel.NormalizedVideoTaskAdaptor)
	assert.True(t, normalized)
	_, billed := adaptor.(channel.VideoBillingEstimator)
	assert.True(t, billed)
	_, content := adaptor.(channel.VideoContentResolver)
	assert.True(t, content)
}

func TestHiggsfieldVideoTaskAdaptorRegistration(t *testing.T) {
	assert.Equal(t, "HiggsfieldVideo", constant.GetChannelTypeName(constant.ChannelTypeHiggsfieldVideo))
	require.Greater(t, len(constant.ChannelBaseURLs), constant.ChannelTypeHiggsfieldVideo)

	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeHiggsfieldVideo)))
	require.NotNil(t, adaptor)
	assert.Equal(t, "higgsfield-video", adaptor.GetChannelName())
	assert.Equal(t, []string{"seedance-2.0-480p", "seedance-2.0-720p"}, adaptor.GetModelList())
	_, normalized := adaptor.(channel.NormalizedVideoTaskAdaptor)
	assert.True(t, normalized)
	_, billed := adaptor.(channel.VideoBillingEstimator)
	assert.True(t, billed)
	_, content := adaptor.(channel.VideoContentResolver)
	assert.True(t, content)
}

func TestLeonardoVideoTaskAdaptorRegistration(t *testing.T) {
	assert.Equal(t, "LeonardoVideo", constant.GetChannelTypeName(constant.ChannelTypeLeonardoVideo))
	require.Greater(t, len(constant.ChannelBaseURLs), constant.ChannelTypeLeonardoVideo)

	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeLeonardoVideo)))
	require.NotNil(t, adaptor)
	assert.Equal(t, "leonardo-video", adaptor.GetChannelName())
	assert.Equal(t, []string{
		"seedance-2.0-fast-480p", "seedance-2.0-fast-720p",
		"seedance-2.0-480p", "seedance-2.0-720p", "seedance-2.0-1080p",
	}, adaptor.GetModelList())
	_, normalized := adaptor.(channel.NormalizedVideoTaskAdaptor)
	assert.True(t, normalized)
	_, billed := adaptor.(channel.VideoBillingEstimator)
	assert.True(t, billed)
	_, content := adaptor.(channel.VideoContentResolver)
	assert.True(t, content)
}
