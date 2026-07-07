package common

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestQuotaFromFloatSaturatesUnsafeValues(t *testing.T) {
	require.Equal(t, 123, QuotaFromFloat(123.7))
	require.Equal(t, 0, QuotaFromFloat(-1))
	require.Equal(t, 0, QuotaFromFloat(math.NaN()))
	require.Equal(t, MaxQuota, QuotaFromFloat(math.Inf(1)))
	require.Equal(t, MaxQuota, QuotaFromFloat(math.MaxFloat64))
}

func TestQuotaFromDecimalConversionsSaturate(t *testing.T) {
	require.Equal(t, 124, QuotaFromDecimalRound(decimal.NewFromFloat(123.7)))
	require.Equal(t, 123, QuotaFromDecimalTrunc(decimal.NewFromFloat(123.7)))
	require.Equal(t, 0, QuotaFromDecimalRound(decimal.NewFromInt(-1)))

	huge := decimal.NewFromInt(1).Shift(100)
	require.Equal(t, MaxQuota, QuotaFromDecimalRound(huge))
	require.Equal(t, MaxQuota, QuotaFromDecimalTrunc(huge))
}
