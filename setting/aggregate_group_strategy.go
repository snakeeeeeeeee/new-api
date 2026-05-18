package setting

const (
	DefaultAggregateGroupSmartStrategyEnabled       = false
	DefaultAggregateGroupConsecutiveFailureLimit    = 2
	DefaultAggregateGroupDegradeDurationSeconds     = 600
	DefaultAggregateGroupClusterDegradedWeightPct   = 50
	DefaultAggregateGroupSlowRequestThreshold       = 30
	DefaultAggregateGroupSlowFirstResponseThreshold = 0
	DefaultAggregateGroupConsecutiveSlowThreshold   = 3
	DefaultAggregateGroupFailureRateWindowSeconds   = 60
	DefaultAggregateGroupFailureRateMinRequests     = 100
	DefaultAggregateGroupFailureRateThresholdPct    = 5
	DefaultAggregateGroupSlowRateWindowSeconds      = 60
	DefaultAggregateGroupSlowRateMinRequests        = 100
	DefaultAggregateGroupSlowRateThresholdPct       = 30
	MaxAggregateGroupRateWindowSeconds              = 3600
)

var (
	AggregateGroupSmartStrategyEnabled       = DefaultAggregateGroupSmartStrategyEnabled
	AggregateGroupFailureThreshold           = DefaultAggregateGroupConsecutiveFailureLimit
	AggregateGroupDegradeDurationSeconds     = DefaultAggregateGroupDegradeDurationSeconds
	AggregateGroupClusterDegradedWeightPct   = DefaultAggregateGroupClusterDegradedWeightPct
	AggregateGroupSlowRequestThreshold       = DefaultAggregateGroupSlowRequestThreshold
	AggregateGroupSlowFirstResponseThreshold = DefaultAggregateGroupSlowFirstResponseThreshold
	AggregateGroupConsecutiveSlowLimit       = DefaultAggregateGroupConsecutiveSlowThreshold
	AggregateGroupFailureRateWindowSeconds   = DefaultAggregateGroupFailureRateWindowSeconds
	AggregateGroupFailureRateMinRequests     = DefaultAggregateGroupFailureRateMinRequests
	AggregateGroupFailureRateThresholdPct    = DefaultAggregateGroupFailureRateThresholdPct
	AggregateGroupSlowRateWindowSeconds      = DefaultAggregateGroupSlowRateWindowSeconds
	AggregateGroupSlowRateMinRequests        = DefaultAggregateGroupSlowRateMinRequests
	AggregateGroupSlowRateThresholdPct       = DefaultAggregateGroupSlowRateThresholdPct
)

func NormalizeAggregateGroupFailureThreshold(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupConsecutiveFailureLimit
	}
	return value
}

func NormalizeAggregateGroupDegradeDurationSeconds(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupDegradeDurationSeconds
	}
	return value
}

func NormalizeAggregateGroupClusterDegradedWeightPercent(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupClusterDegradedWeightPct
	}
	if value > 100 {
		return 100
	}
	return value
}

func NormalizeAggregateGroupSlowRequestThreshold(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupSlowRequestThreshold
	}
	return value
}

func NormalizeAggregateGroupSlowFirstResponseThreshold(value int) int {
	if value < 0 {
		return DefaultAggregateGroupSlowFirstResponseThreshold
	}
	return value
}

func NormalizeAggregateGroupConsecutiveSlowThreshold(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupConsecutiveSlowThreshold
	}
	return value
}

func NormalizeAggregateGroupRateWindowSeconds(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupFailureRateWindowSeconds
	}
	if value > MaxAggregateGroupRateWindowSeconds {
		return MaxAggregateGroupRateWindowSeconds
	}
	return value
}

func NormalizeAggregateGroupRateMinRequests(value int, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}

func NormalizeAggregateGroupRateThresholdPercent(value int, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value > 100 {
		return 100
	}
	return value
}
