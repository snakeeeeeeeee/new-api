package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefundTokenQuotaPreservesUnlimitedSemantics(t *testing.T) {
	tests := []struct {
		name           string
		batch          bool
		unlimited      bool
		expectedRemain int
		expectedUsed   int
	}{
		{name: "normal immediate", batch: false, unlimited: false, expectedRemain: 800, expectedUsed: 200},
		{name: "unlimited immediate", batch: false, unlimited: true, expectedRemain: 700, expectedUsed: 200},
		{name: "normal batch", batch: true, unlimited: false, expectedRemain: 800, expectedUsed: 200},
		{name: "unlimited batch", batch: true, unlimited: true, expectedRemain: 700, expectedUsed: 200},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateTables(t)
			batchUpdate()
			previousBatchSetting := common.BatchUpdateEnabled
			common.BatchUpdateEnabled = tt.batch
			t.Cleanup(func() {
				common.BatchUpdateEnabled = previousBatchSetting
				batchUpdate()
			})

			token := Token{
				Id:             9200 + index,
				UserId:         9200 + index,
				Key:            "token-refund-test-" + tt.name,
				Status:         common.TokenStatusEnabled,
				RemainQuota:    700,
				UsedQuota:      300,
				UnlimitedQuota: tt.unlimited,
			}
			require.NoError(t, DB.Create(&token).Error)
			require.NoError(t, RefundTokenQuota(token.Id, token.Key, 100))

			if tt.batch {
				var beforeFlush Token
				require.NoError(t, DB.Select("remain_quota", "used_quota").First(&beforeFlush, token.Id).Error)
				assert.Equal(t, 700, beforeFlush.RemainQuota)
				assert.Equal(t, 300, beforeFlush.UsedQuota)

				// The queued refund must retain the token mode observed when it was
				// enqueued, even if an administrator changes that mode before flush.
				require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).
					Update("unlimited_quota", !tt.unlimited).Error)
				batchUpdate()
			}

			var actual Token
			require.NoError(t, DB.Select("remain_quota", "used_quota").First(&actual, token.Id).Error)
			assert.Equal(t, tt.expectedRemain, actual.RemainQuota)
			assert.Equal(t, tt.expectedUsed, actual.UsedQuota)
		})
	}
}

func TestRefundTokenQuotaRejectsNegativeQuota(t *testing.T) {
	require.Error(t, RefundTokenQuota(1, "unused", -1))
}
