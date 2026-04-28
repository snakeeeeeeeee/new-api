package setting

const (
	DefaultAggregateGroupSmartStrategyEnabled     = false
	DefaultAggregateGroupConsecutiveFailureLimit  = 2
	DefaultAggregateGroupDegradeDurationSeconds   = 600
	DefaultAggregateGroupClusterDegradedWeightPct = 20
	DefaultAggregateGroupSlowRequestThreshold     = 30
	DefaultAggregateGroupConsecutiveSlowThreshold = 3
)

var (
	AggregateGroupSmartStrategyEnabled     = DefaultAggregateGroupSmartStrategyEnabled
	AggregateGroupFailureThreshold         = DefaultAggregateGroupConsecutiveFailureLimit
	AggregateGroupDegradeDurationSeconds   = DefaultAggregateGroupDegradeDurationSeconds
	AggregateGroupClusterDegradedWeightPct = DefaultAggregateGroupClusterDegradedWeightPct
	AggregateGroupSlowRequestThreshold     = DefaultAggregateGroupSlowRequestThreshold
	AggregateGroupConsecutiveSlowLimit     = DefaultAggregateGroupConsecutiveSlowThreshold
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

func NormalizeAggregateGroupConsecutiveSlowThreshold(value int) int {
	if value <= 0 {
		return DefaultAggregateGroupConsecutiveSlowThreshold
	}
	return value
}
