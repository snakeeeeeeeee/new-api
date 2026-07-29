package gemini

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExplicitVeoDurationDoesNotUseProviderDefault(t *testing.T) {
	seconds, exists, err := ExplicitVeoDuration(nil, 0, "")
	require.NoError(t, err)
	require.False(t, exists)
	require.Zero(t, seconds)

	seconds, exists, err = ExplicitVeoDuration(map[string]any{"durationSeconds": float64(7)}, 4, "5")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, 7, seconds)

	_, exists, err = ExplicitVeoDuration(map[string]any{"durationSeconds": 7.5}, 0, "")
	require.True(t, exists)
	require.Error(t, err)
}

func TestResolveVideoBillingKeepsGeminiDurationSecondsInUpstreamBody(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "cat", Metadata: map[string]interface{}{"durationSeconds": float64(7)}})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "veo-3"}}
	adaptor := &TaskAdaptor{}

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, 7, estimate.Seconds)
	require.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	parameters := payload["parameters"].(map[string]interface{})
	require.Equal(t, float64(7), parameters["durationSeconds"])
}

func TestResolveVideoBillingRejectsMissingGeminiDuration(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "cat"})

	_, taskErr := (&TaskAdaptor{}).ResolveVideoBilling(c, &relaycommon.RelayInfo{})
	require.NotNil(t, taskErr)
	require.Equal(t, "video_duration_required", taskErr.Code)
}
