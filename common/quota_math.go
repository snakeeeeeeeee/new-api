package common

import (
	"math"

	"github.com/shopspring/decimal"
)

const MaxQuota = math.MaxInt32

func QuotaFromFloat(value float64) int {
	switch {
	case math.IsNaN(value), value <= 0:
		return 0
	case math.IsInf(value, 1), value >= MaxQuota:
		return MaxQuota
	default:
		return int(value)
	}
}

func QuotaFromDecimalRound(value decimal.Decimal) int {
	f, _ := value.Round(0).Float64()
	return QuotaFromFloat(f)
}

func QuotaFromDecimalTrunc(value decimal.Decimal) int {
	f, _ := value.Float64()
	return QuotaFromFloat(f)
}
