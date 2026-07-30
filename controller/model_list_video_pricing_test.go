package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const modelListVideoPricingConfig = `{
  "version": 1,
  "profiles": {
    "model-list-video": {
      "name": "model list video",
      "billing_mode": "per_second",
      "unit_price": 0.03
    }
  },
  "model_bindings": {
    "model-list-video-only": {
      "profile": "model-list-video",
      "subscription_enabled": false
    }
  }
}`

func listModelIDsForTest(t *testing.T, configureContext func(*gin.Context)) []string {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 1)
	configureContext(c)

	ListModels(c, 0)
	require.Equal(t, 200, recorder.Code)

	var response struct {
		Success bool               `json:"success"`
		Data    []dto.OpenAIModels `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	ids := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		ids = append(ids, item.Id)
	}
	return ids
}

func TestListModelsIncludesVideoPricingOnlyModel(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalVideoPricing := ratio_setting.VideoPricing2JSONString()
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(originalVideoPricing))
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})
	operation_setting.SelfUseModeEnabled = false
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(modelListVideoPricingConfig))

	user := &model.User{
		Id:       1,
		Username: "model-list-video-user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Group:    "video-group",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "video-group",
		Model:     "model-list-video-only",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	t.Run("token model limit", func(t *testing.T) {
		ids := listModelIDsForTest(t, func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{
				"model-list-video-only": true,
			})
		})
		require.Equal(t, []string{"model-list-video-only"}, ids)
	})

	t.Run("group model list", func(t *testing.T) {
		ids := listModelIDsForTest(t, func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "video-group")
		})
		require.Equal(t, []string{"model-list-video-only"}, ids)
	})
}
