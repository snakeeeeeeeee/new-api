package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdobeVideoChannelRejectsGenericChannelTest(t *testing.T) {
	result := testChannel(
		&model.Channel{Type: constant.ChannelTypeAdobeVideo},
		"seedance-2.0-fast-480p",
		string(constant.EndpointTypeVideoTask),
		false,
	)

	require.Error(t, result.localErr)
	assert.Contains(t, result.localErr.Error(), "AdobeVideo channel test is not supported")
	assert.Nil(t, result.newAPIError)
}
