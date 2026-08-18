package codex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexDoRequestStartsOpenAIIntegrityAttemptForResponsesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	for _, tt := range []struct {
		name   string
		mode   int
		active bool
	}{
		{name: "responses", mode: relayconstant.RelayModeResponses, active: true},
		{name: "compact excluded", mode: relayconstant.RelayModeResponsesCompact, active: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{
				RelayMode:                                 tt.mode,
				OpenAIResponseIntegrityEnabled:            true,
				OpenAIResponseIntegrityFirstOutputTimeout: time.Second,
				StartTime:                                 time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    constant.ChannelTypeCodex,
					ChannelBaseUrl: server.URL,
					ApiKey:         `{"access_token":"token","account_id":"account"}`,
				},
			}
			defer info.EndOpenAIResponseIntegrityAttempt()

			response, err := (&Adaptor{}).DoRequest(c, info, strings.NewReader(`{}`))

			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tt.active, info.OpenAIResponseIntegrityAttemptContext() != nil)
			if httpResponse, ok := response.(*http.Response); ok {
				require.NoError(t, httpResponse.Body.Close())
			}
		})
	}
}
