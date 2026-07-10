package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperSnapshotsCacheCreationConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	originalModelRatio := ratio_setting.ModelRatio2JSONString()
	originalCreateCacheRatio := ratio_setting.CreateCacheRatio2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatio))
		require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(originalCreateCacheRatio))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"cache-write-missing":1,"cache-write-zero":1,"cache-write-one":1,"cache-write-one-quarter":1}`))
	require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(`{"cache-write-zero":0,"cache-write-one":1,"cache-write-one-quarter":1.25}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	tests := []struct {
		model          string
		wantRatio      float64
		wantConfigured bool
	}{
		{model: "cache-write-missing", wantRatio: 1.25, wantConfigured: false},
		{model: "cache-write-zero", wantRatio: 0, wantConfigured: true},
		{model: "cache-write-one", wantRatio: 1, wantConfigured: true},
		{model: "cache-write-one-quarter", wantRatio: 1.25, wantConfigured: true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.model,
				UserGroup:       "default",
				UsingGroup:      "default",
			}

			priceData, err := ModelPriceHelper(ctx, info, 100, &types.TokenCountMeta{})

			require.NoError(t, err)
			require.Equal(t, tt.wantRatio, priceData.CacheCreationRatio)
			require.Equal(t, tt.wantConfigured, priceData.CacheCreationRatioConfigured)
			require.Equal(t, priceData, info.PriceData)
		})
	}
}
