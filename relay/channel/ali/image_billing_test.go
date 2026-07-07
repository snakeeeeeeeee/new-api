package ali

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestAliImageParametersNBound(t *testing.T) {
	parameters, err := common.Marshal(AliImageParameters{N: dto.MaxImageN + 1})
	require.NoError(t, err)

	_, err = oaiImage2AliImageRequest(&relaycommon.RelayInfo{}, dto.ImageRequest{
		Model: "wanx2.1-t2i-plus",
		Extra: map[string]json.RawMessage{
			"parameters": parameters,
		},
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parameters.n")
}
