package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTaskPollingRelayInfoRestoresXAI2KENVariant(t *testing.T) {
	settings, err := common.Marshal(dto.ChannelOtherSettings{XAIAPIVariant: dto.XAIAPIVariant2KEN})
	require.NoError(t, err)
	channel := &model.Channel{
		Id:            42,
		Type:          constant.ChannelTypeXai,
		Key:           "poll-key",
		BaseURL:       common.GetPointer("https://apis.2ken.com"),
		OtherSettings: string(settings),
	}

	info := taskPollingRelayInfo(channel)
	require.NotNil(t, info.ChannelMeta)
	require.Equal(t, constant.ChannelTypeXai, info.ChannelType)
	require.Equal(t, 42, info.ChannelId)
	require.Equal(t, "poll-key", info.ApiKey)
	require.Equal(t, "https://apis.2ken.com", info.ChannelBaseUrl)
	require.True(t, info.ChannelOtherSettings.IsXAI2KEN())
}
