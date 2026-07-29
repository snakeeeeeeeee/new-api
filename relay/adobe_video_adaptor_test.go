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
