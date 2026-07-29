package vertex

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

func TestResolveVideoBillingKeepsVertexDurationSecondsInUpstreamBody(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "cat", Duration: 6})
	info := &relaycommon.RelayInfo{}
	adaptor := &TaskAdaptor{}

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, 6, estimate.Seconds)
	require.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	parameters := payload["parameters"].(map[string]interface{})
	require.Equal(t, float64(6), parameters["durationSeconds"])
}
