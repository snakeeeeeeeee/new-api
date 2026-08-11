package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func new2KENChannel(baseURL string, models string) *model.Channel {
	settings, _ := common.Marshal(dto.ChannelOtherSettings{XAIAPIVariant: dto.XAIAPIVariant2KEN})
	return &model.Channel{
		Type: constant.ChannelTypeXai, Key: "test-key", Models: models,
		BaseURL: common.GetPointer(baseURL), OtherSettings: string(settings),
	}
}

func TestValidateChannelRequires2KENConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		channel   *model.Channel
		wantError string
	}{
		{name: "valid", channel: new2KENChannel("https://apis.2ken.com", "grok-imagine-image")},
		{name: "missing base URL", channel: new2KENChannel("", "grok-imagine-image"), wantError: "API 地址不能为空"},
		{name: "missing models", channel: new2KENChannel("https://apis.2ken.com", ""), wantError: "至少需要配置一个模型"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateChannel(test.channel, true)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestValidateChannelRequiresEffective2KENKey(t *testing.T) {
	channel := new2KENChannel("https://apis.2ken.com", "grok-imagine-image")
	channel.Key = ""

	err := validateChannel(channel, true)
	require.ErrorContains(t, err, "API 密钥不能为空")

	require.NoError(t, validateChannel(channel, false))
	require.NoError(t, validateXAI2KENEffectiveKey(channel, "stored-key"))
	require.ErrorContains(t, validateXAI2KENEffectiveKey(channel, ""), "API 密钥不能为空")
}

func TestValidateChannelKeepsOfficialXAIBaseURLOptional(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeXai, Key: "test-key", Models: "grok-imagine-image"}
	require.NoError(t, validateChannel(channel, true))
}

func TestResolveChannelBaseURLAddsV1OnlyFor2KENXAI(t *testing.T) {
	channel := new2KENChannel("https://apis.2ken.com", "grok-imagine-image")
	assert.Equal(t, "https://apis.2ken.com/v1", resolveChannelBaseURL(channel))

	channel.BaseURL = common.GetPointer("https://apis.2ken.com/v1/images/generations")
	assert.Equal(t, "https://apis.2ken.com/v1", resolveChannelBaseURL(channel))

	official := &model.Channel{
		Type:    constant.ChannelTypeXai,
		BaseURL: common.GetPointer("https://official-xai-compatible.example"),
	}
	assert.Equal(t, "https://official-xai-compatible.example", resolveChannelBaseURL(official))
}
