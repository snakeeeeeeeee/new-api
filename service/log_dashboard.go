package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	LogDashboardWindow1h  = "1h"
	LogDashboardWindow6h  = "6h"
	LogDashboardWindow24h = "24h"
)

type LogDashboardSummary struct {
	TotalRequests                int     `json:"total_requests"`
	SuccessfulRequests           int     `json:"successful_requests"`
	FailedRequests               int     `json:"failed_requests"`
	SuccessRate                  float64 `json:"success_rate"`
	ErrorRate                    float64 `json:"error_rate"`
	AverageSuccessUseTimeSeconds float64 `json:"average_success_use_time_seconds"`
}

type LogDashboardTrendPoint struct {
	BucketStart        int64   `json:"bucket_start"`
	Label              string  `json:"label"`
	TotalRequests      int     `json:"total_requests"`
	SuccessfulRequests int     `json:"successful_requests"`
	FailedRequests     int     `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"`
	ErrorRate          float64 `json:"error_rate"`
}

type LogDashboardDimensionTrendPoint struct {
	BucketStart      int64  `json:"bucket_start"`
	Label            string `json:"label"`
	Series           string `json:"series"`
	Count            int    `json:"count"`
	SuccessCount     int    `json:"success_count"`
	FailureCount     int    `json:"failure_count"`
	IsAggregateGroup bool   `json:"is_aggregate_group,omitempty"`
}

type LogDashboardErrorMessageStat struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type LogDashboardStatusCodeStat struct {
	StatusCode int `json:"status_code"`
	Count      int `json:"count"`
}

type LogDashboardChannelStat struct {
	ChannelId             int     `json:"channel_id"`
	ChannelName           string  `json:"channel_name"`
	AttemptCount          int     `json:"attempt_count"`
	SuccessCount          int     `json:"success_count"`
	FailureCount          int     `json:"failure_count"`
	SuccessRate           float64 `json:"success_rate"`
	ErrorRate             float64 `json:"error_rate"`
	AverageUseTimeSeconds float64 `json:"average_use_time_seconds"`
	TopStatusCode         int     `json:"top_status_code"`
	TopStatusCodeCount    int     `json:"top_status_code_count"`
	TopErrorMessage       string  `json:"top_error_message"`
	TopErrorMessageCount  int     `json:"top_error_message_count"`
}

type LogDashboardGroupStat struct {
	GroupName                    string  `json:"group_name"`
	IsAggregateGroup             bool    `json:"is_aggregate_group"`
	TotalRequests                int     `json:"total_requests"`
	SuccessCount                 int     `json:"success_count"`
	FailureCount                 int     `json:"failure_count"`
	SuccessRate                  float64 `json:"success_rate"`
	ErrorRate                    float64 `json:"error_rate"`
	AverageSuccessUseTimeSeconds float64 `json:"average_success_use_time_seconds"`
	TopStatusCode                int     `json:"top_status_code"`
	TopStatusCodeCount           int     `json:"top_status_code_count"`
	TopErrorMessage              string  `json:"top_error_message"`
	TopErrorMessageCount         int     `json:"top_error_message_count"`
}

type LogDashboardData struct {
	Window           string                            `json:"window"`
	GeneratedAt      int64                             `json:"generated_at"`
	Summary          LogDashboardSummary               `json:"summary"`
	Trend            []LogDashboardTrendPoint          `json:"trend"`
	GroupTrend       []LogDashboardDimensionTrendPoint `json:"group_trend"`
	ChannelTrend     []LogDashboardDimensionTrendPoint `json:"channel_trend"`
	Channels         []LogDashboardChannelStat         `json:"channels"`
	Groups           []LogDashboardGroupStat           `json:"groups"`
	TopErrorMessages []LogDashboardErrorMessageStat    `json:"top_error_messages"`
	TopStatusCodes   []LogDashboardStatusCodeStat      `json:"top_status_codes"`
}

type logDashboardWindowConfig struct {
	WindowKey      string
	Duration       time.Duration
	BucketDuration time.Duration
	LabelLayout    string
}

type requestOutcomeState struct {
	RequestID       string
	SuccessAt       int64
	SuccessUseTime  int
	SuccessGroup    string
	LastErrorAt     int64
	LastErrorStatus int
	LastErrorMsg    string
	LastErrorGroup  string
	HasSuccess      bool
	HasError        bool
}

type channelAggregateState struct {
	ChannelId    int
	AttemptCount int
	SuccessCount int
	FailureCount int
	UseTimeTotal int64
	UseTimeCount int
	StatusCounts map[int]int
	ErrorCounts  map[string]int
}

type groupAggregateState struct {
	GroupName     string
	TotalRequests int
	SuccessCount  int
	FailureCount  int
	UseTimeTotal  int64
	UseTimeCount  int
	StatusCounts  map[int]int
	ErrorCounts   map[string]int
}

type logDashboardTrendMetric struct {
	Count        int
	SuccessCount int
	FailureCount int
}

var (
	logDashboardNow               = time.Now
	requestIDSuffixPattern        = regexp.MustCompile(`\s*\(\s*request id:\s*[^)]+\)`)
	logDashboardWhitespacePattern = regexp.MustCompile(`\s+`)
)

func resolveLogDashboardWindow(window string) (logDashboardWindowConfig, error) {
	switch window {
	case "", LogDashboardWindow1h:
		return logDashboardWindowConfig{
			WindowKey:      LogDashboardWindow1h,
			Duration:       time.Hour,
			BucketDuration: 5 * time.Minute,
			LabelLayout:    "15:04",
		}, nil
	case LogDashboardWindow6h:
		return logDashboardWindowConfig{
			WindowKey:      LogDashboardWindow6h,
			Duration:       6 * time.Hour,
			BucketDuration: 15 * time.Minute,
			LabelLayout:    "15:04",
		}, nil
	case LogDashboardWindow24h:
		return logDashboardWindowConfig{
			WindowKey:      LogDashboardWindow24h,
			Duration:       24 * time.Hour,
			BucketDuration: time.Hour,
			LabelLayout:    "01-02 15:04",
		}, nil
	default:
		return logDashboardWindowConfig{}, fmt.Errorf("invalid window: %s", window)
	}
}

func calculateLogDashboardRate(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func normalizeLogDashboardErrorMessage(message string) string {
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return ""
	}
	normalized = requestIDSuffixPattern.ReplaceAllString(normalized, "")
	normalized = logDashboardWhitespacePattern.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func extractDashboardStatusCode(other string) int {
	if strings.TrimSpace(other) == "" {
		return 0
	}
	otherMap, err := common.StrToMap(other)
	if err != nil || otherMap == nil {
		return 0
	}
	raw, ok := otherMap["status_code"]
	if !ok || raw == nil {
		return 0
	}
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		statusCode, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			return statusCode
		}
	}
	return 0
}

func buildTopStatusCodeStats(counts map[int]int, limit int) []LogDashboardStatusCodeStat {
	if len(counts) == 0 {
		return []LogDashboardStatusCodeStat{}
	}
	items := make([]LogDashboardStatusCodeStat, 0, len(counts))
	for statusCode, count := range counts {
		if statusCode == 0 || count <= 0 {
			continue
		}
		items = append(items, LogDashboardStatusCodeStat{
			StatusCode: statusCode,
			Count:      count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].StatusCode < items[j].StatusCode
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func buildTopErrorMessageStats(counts map[string]int, limit int) []LogDashboardErrorMessageStat {
	if len(counts) == 0 {
		return []LogDashboardErrorMessageStat{}
	}
	items := make([]LogDashboardErrorMessageStat, 0, len(counts))
	for message, count := range counts {
		if message == "" || count <= 0 {
			continue
		}
		items = append(items, LogDashboardErrorMessageStat{
			Message: message,
			Count:   count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Message < items[j].Message
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func buildLogDashboardTrend(config logDashboardWindowConfig, now time.Time, successBuckets map[int64]int, failureBuckets map[int64]int) []LogDashboardTrendPoint {
	bucketStarts := buildLogDashboardBucketStarts(config, now)
	if len(bucketStarts) == 0 {
		return []LogDashboardTrendPoint{}
	}
	points := make([]LogDashboardTrendPoint, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		successCount := successBuckets[bucketStart]
		failureCount := failureBuckets[bucketStart]
		totalCount := successCount + failureCount
		points = append(points, LogDashboardTrendPoint{
			BucketStart:        bucketStart,
			Label:              time.Unix(bucketStart, 0).In(time.Local).Format(config.LabelLayout),
			TotalRequests:      totalCount,
			SuccessfulRequests: successCount,
			FailedRequests:     failureCount,
			SuccessRate:        calculateLogDashboardRate(successCount, totalCount),
			ErrorRate:          calculateLogDashboardRate(failureCount, totalCount),
		})
	}
	return points
}

func buildLogDashboardBucketStarts(config logDashboardWindowConfig, now time.Time) []int64 {
	windowEnd := now.Unix()
	windowStart := now.Add(-config.Duration).Unix()
	bucketSeconds := int64(config.BucketDuration.Seconds())
	if bucketSeconds <= 0 {
		return []int64{}
	}

	firstBucketStart := (windowStart / bucketSeconds) * bucketSeconds
	lastBucketStart := (windowEnd / bucketSeconds) * bucketSeconds
	if lastBucketStart < firstBucketStart {
		lastBucketStart = firstBucketStart
	}

	bucketStarts := make([]int64, 0, int((lastBucketStart-firstBucketStart)/bucketSeconds)+1)
	for bucketStart := firstBucketStart; bucketStart <= lastBucketStart; bucketStart += bucketSeconds {
		bucketStarts = append(bucketStarts, bucketStart)
	}
	return bucketStarts
}

func buildDimensionTrendPoints(
	bucketStarts []int64,
	labelLayout string,
	seriesOrder []string,
	seriesMetrics map[string]map[int64]*logDashboardTrendMetric,
	aggregateSeriesSet map[string]struct{},
) []LogDashboardDimensionTrendPoint {
	if len(bucketStarts) == 0 || len(seriesOrder) == 0 {
		return []LogDashboardDimensionTrendPoint{}
	}
	points := make([]LogDashboardDimensionTrendPoint, 0, len(bucketStarts)*len(seriesOrder))
	for _, series := range seriesOrder {
		bucketMetrics := seriesMetrics[series]
		for _, bucketStart := range bucketStarts {
			metric := &logDashboardTrendMetric{}
			if bucketMetrics != nil {
				if existing, ok := bucketMetrics[bucketStart]; ok && existing != nil {
					metric = existing
				}
			}
			point := LogDashboardDimensionTrendPoint{
				BucketStart:  bucketStart,
				Label:        time.Unix(bucketStart, 0).In(time.Local).Format(labelLayout),
				Series:       series,
				Count:        metric.Count,
				SuccessCount: metric.SuccessCount,
				FailureCount: metric.FailureCount,
			}
			if aggregateSeriesSet != nil {
				if _, ok := aggregateSeriesSet[series]; ok {
					point.IsAggregateGroup = true
				}
			}
			points = append(points, point)
		}
	}
	return points
}

func normalizeLogDashboardGroupName(groupName string) string {
	normalized := strings.TrimSpace(groupName)
	if normalized == "" {
		return "-"
	}
	return normalized
}

func getAggregateGroupNameSet() map[string]struct{} {
	nameSet := make(map[string]struct{})
	groups, err := model.GetAllAggregateGroups(false)
	if err != nil {
		return nameSet
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		name := strings.TrimSpace(group.Name)
		if name == "" {
			continue
		}
		nameSet[name] = struct{}{}
	}
	return nameSet
}

func SetLogDashboardNowForTest(nowFn func() time.Time) func() {
	previous := logDashboardNow
	if nowFn == nil {
		logDashboardNow = time.Now
	} else {
		logDashboardNow = nowFn
	}
	return func() {
		logDashboardNow = previous
	}
}

func GetLogDashboard(ctx context.Context, window string) (*LogDashboardData, error) {
	config, err := resolveLogDashboardWindow(window)
	if err != nil {
		return nil, err
	}

	now := logDashboardNow().In(time.Local)
	endTimestamp := now.Unix()
	startTimestamp := now.Add(-config.Duration).Unix()

	logs, err := model.GetLogsForDashboard(ctx, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}

	requestStates := make(map[string]*requestOutcomeState)
	channelStates := make(map[int]*channelAggregateState)
	groupStates := make(map[string]*groupAggregateState)
	overallStatusCounts := make(map[int]int)
	overallErrorCounts := make(map[string]int)
	channelIDs := make([]int, 0)
	channelSeen := make(map[int]struct{})
	successBuckets := make(map[int64]int)
	failureBuckets := make(map[int64]int)
	bucketSeconds := int64(config.BucketDuration.Seconds())
	groupTrendMetrics := make(map[string]map[int64]*logDashboardTrendMetric)
	channelTrendMetrics := make(map[int]map[int64]*logDashboardTrendMetric)

	for _, logItem := range logs {
		if logItem == nil || common.IsLogConsumeExcludedUserID(logItem.UserId) {
			continue
		}

		normalizedErrorMessage := normalizeLogDashboardErrorMessage(logItem.Content)
		statusCode := extractDashboardStatusCode(logItem.Other)

		if logItem.ChannelId > 0 {
			channelState, ok := channelStates[logItem.ChannelId]
			if !ok {
				channelState = &channelAggregateState{
					ChannelId:    logItem.ChannelId,
					StatusCounts: make(map[int]int),
					ErrorCounts:  make(map[string]int),
				}
				channelStates[logItem.ChannelId] = channelState
			}
			channelState.AttemptCount++
			channelState.UseTimeTotal += int64(logItem.UseTime)
			channelState.UseTimeCount++
			if logItem.Type == model.LogTypeConsume {
				channelState.SuccessCount++
			}
			if logItem.Type == model.LogTypeError {
				channelState.FailureCount++
				if statusCode > 0 {
					channelState.StatusCounts[statusCode]++
				}
				if normalizedErrorMessage != "" {
					channelState.ErrorCounts[normalizedErrorMessage]++
				}
			}
			if bucketSeconds > 0 {
				bucketStart := (logItem.CreatedAt / bucketSeconds) * bucketSeconds
				bucketMetrics, ok := channelTrendMetrics[logItem.ChannelId]
				if !ok {
					bucketMetrics = make(map[int64]*logDashboardTrendMetric)
					channelTrendMetrics[logItem.ChannelId] = bucketMetrics
				}
				metric, ok := bucketMetrics[bucketStart]
				if !ok {
					metric = &logDashboardTrendMetric{}
					bucketMetrics[bucketStart] = metric
				}
				metric.Count++
				if logItem.Type == model.LogTypeConsume {
					metric.SuccessCount++
				}
				if logItem.Type == model.LogTypeError {
					metric.FailureCount++
				}
			}
			if _, ok := channelSeen[logItem.ChannelId]; !ok {
				channelSeen[logItem.ChannelId] = struct{}{}
				channelIDs = append(channelIDs, logItem.ChannelId)
			}
		}

		if strings.TrimSpace(logItem.RequestId) == "" {
			continue
		}

		requestState, ok := requestStates[logItem.RequestId]
		if !ok {
			requestState = &requestOutcomeState{
				RequestID: logItem.RequestId,
			}
			requestStates[logItem.RequestId] = requestState
		}

		if logItem.Type == model.LogTypeConsume {
			if !requestState.HasSuccess || logItem.CreatedAt >= requestState.SuccessAt {
				requestState.HasSuccess = true
				requestState.SuccessAt = logItem.CreatedAt
				requestState.SuccessUseTime = logItem.UseTime
				requestState.SuccessGroup = normalizeLogDashboardGroupName(logItem.Group)
			}
			continue
		}

		if logItem.Type == model.LogTypeError {
			if !requestState.HasError || logItem.CreatedAt >= requestState.LastErrorAt {
				requestState.HasError = true
				requestState.LastErrorAt = logItem.CreatedAt
				requestState.LastErrorStatus = statusCode
				requestState.LastErrorMsg = normalizedErrorMessage
				requestState.LastErrorGroup = normalizeLogDashboardGroupName(logItem.Group)
			}
		}
	}

	summary := LogDashboardSummary{}
	successUseTimeTotal := 0
	for _, requestState := range requestStates {
		if requestState == nil {
			continue
		}
		if requestState.HasSuccess {
			summary.TotalRequests++
			summary.SuccessfulRequests++
			successUseTimeTotal += requestState.SuccessUseTime
			groupName := normalizeLogDashboardGroupName(requestState.SuccessGroup)
			groupState, ok := groupStates[groupName]
			if !ok {
				groupState = &groupAggregateState{
					GroupName:    groupName,
					StatusCounts: make(map[int]int),
					ErrorCounts:  make(map[string]int),
				}
				groupStates[groupName] = groupState
			}
			groupState.TotalRequests++
			groupState.SuccessCount++
			groupState.UseTimeTotal += int64(requestState.SuccessUseTime)
			groupState.UseTimeCount++
			if bucketSeconds > 0 {
				bucketStart := (requestState.SuccessAt / bucketSeconds) * bucketSeconds
				successBuckets[bucketStart]++
				bucketMetrics, ok := groupTrendMetrics[groupName]
				if !ok {
					bucketMetrics = make(map[int64]*logDashboardTrendMetric)
					groupTrendMetrics[groupName] = bucketMetrics
				}
				metric, ok := bucketMetrics[bucketStart]
				if !ok {
					metric = &logDashboardTrendMetric{}
					bucketMetrics[bucketStart] = metric
				}
				metric.Count++
				metric.SuccessCount++
			}
			continue
		}
		if requestState.HasError {
			summary.TotalRequests++
			summary.FailedRequests++
			groupName := normalizeLogDashboardGroupName(requestState.LastErrorGroup)
			groupState, ok := groupStates[groupName]
			if !ok {
				groupState = &groupAggregateState{
					GroupName:    groupName,
					StatusCounts: make(map[int]int),
					ErrorCounts:  make(map[string]int),
				}
				groupStates[groupName] = groupState
			}
			groupState.TotalRequests++
			groupState.FailureCount++
			if bucketSeconds > 0 {
				bucketStart := (requestState.LastErrorAt / bucketSeconds) * bucketSeconds
				failureBuckets[bucketStart]++
				bucketMetrics, ok := groupTrendMetrics[groupName]
				if !ok {
					bucketMetrics = make(map[int64]*logDashboardTrendMetric)
					groupTrendMetrics[groupName] = bucketMetrics
				}
				metric, ok := bucketMetrics[bucketStart]
				if !ok {
					metric = &logDashboardTrendMetric{}
					bucketMetrics[bucketStart] = metric
				}
				metric.Count++
				metric.FailureCount++
			}
			if requestState.LastErrorStatus > 0 {
				overallStatusCounts[requestState.LastErrorStatus]++
				groupState.StatusCounts[requestState.LastErrorStatus]++
			}
			if requestState.LastErrorMsg != "" {
				overallErrorCounts[requestState.LastErrorMsg]++
				groupState.ErrorCounts[requestState.LastErrorMsg]++
			}
		}
	}

	summary.SuccessRate = calculateLogDashboardRate(summary.SuccessfulRequests, summary.TotalRequests)
	summary.ErrorRate = calculateLogDashboardRate(summary.FailedRequests, summary.TotalRequests)
	if summary.SuccessfulRequests > 0 {
		summary.AverageSuccessUseTimeSeconds = float64(successUseTimeTotal) / float64(summary.SuccessfulRequests)
	}

	channelNameMap, err := model.GetChannelNameMap(channelIDs)
	if err != nil {
		return nil, err
	}

	channels := make([]LogDashboardChannelStat, 0, len(channelStates))
	for _, channelState := range channelStates {
		if channelState == nil || channelState.AttemptCount <= 0 {
			continue
		}
		channelName := channelNameMap[channelState.ChannelId]
		if channelName == "" {
			channelName = fmt.Sprintf("channel#%d", channelState.ChannelId)
		}
		channelStat := LogDashboardChannelStat{
			ChannelId:    channelState.ChannelId,
			ChannelName:  channelName,
			AttemptCount: channelState.AttemptCount,
			SuccessCount: channelState.SuccessCount,
			FailureCount: channelState.FailureCount,
			SuccessRate:  calculateLogDashboardRate(channelState.SuccessCount, channelState.AttemptCount),
			ErrorRate:    calculateLogDashboardRate(channelState.FailureCount, channelState.AttemptCount),
		}
		if channelState.UseTimeCount > 0 {
			channelStat.AverageUseTimeSeconds = float64(channelState.UseTimeTotal) / float64(channelState.UseTimeCount)
		}
		topStatusCodes := buildTopStatusCodeStats(channelState.StatusCounts, 1)
		if len(topStatusCodes) > 0 {
			channelStat.TopStatusCode = topStatusCodes[0].StatusCode
			channelStat.TopStatusCodeCount = topStatusCodes[0].Count
		}
		topErrorMessages := buildTopErrorMessageStats(channelState.ErrorCounts, 1)
		if len(topErrorMessages) > 0 {
			channelStat.TopErrorMessage = topErrorMessages[0].Message
			channelStat.TopErrorMessageCount = topErrorMessages[0].Count
		}
		channels = append(channels, channelStat)
	}

	sort.Slice(channels, func(i, j int) bool {
		if channels[i].AttemptCount == channels[j].AttemptCount {
			return channels[i].ChannelId < channels[j].ChannelId
		}
		return channels[i].AttemptCount > channels[j].AttemptCount
	})

	aggregateGroupNameSet := getAggregateGroupNameSet()
	groups := make([]LogDashboardGroupStat, 0, len(groupStates))
	for _, groupState := range groupStates {
		if groupState == nil || groupState.TotalRequests <= 0 {
			continue
		}
		groupStat := LogDashboardGroupStat{
			GroupName:     groupState.GroupName,
			TotalRequests: groupState.TotalRequests,
			SuccessCount:  groupState.SuccessCount,
			FailureCount:  groupState.FailureCount,
			SuccessRate:   calculateLogDashboardRate(groupState.SuccessCount, groupState.TotalRequests),
			ErrorRate:     calculateLogDashboardRate(groupState.FailureCount, groupState.TotalRequests),
		}
		if _, ok := aggregateGroupNameSet[groupState.GroupName]; ok {
			groupStat.IsAggregateGroup = true
		}
		if groupState.UseTimeCount > 0 {
			groupStat.AverageSuccessUseTimeSeconds = float64(groupState.UseTimeTotal) / float64(groupState.UseTimeCount)
		}
		topStatusCodes := buildTopStatusCodeStats(groupState.StatusCounts, 1)
		if len(topStatusCodes) > 0 {
			groupStat.TopStatusCode = topStatusCodes[0].StatusCode
			groupStat.TopStatusCodeCount = topStatusCodes[0].Count
		}
		topErrorMessages := buildTopErrorMessageStats(groupState.ErrorCounts, 1)
		if len(topErrorMessages) > 0 {
			groupStat.TopErrorMessage = topErrorMessages[0].Message
			groupStat.TopErrorMessageCount = topErrorMessages[0].Count
		}
		groups = append(groups, groupStat)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalRequests == groups[j].TotalRequests {
			return groups[i].GroupName < groups[j].GroupName
		}
		return groups[i].TotalRequests > groups[j].TotalRequests
	})

	bucketStarts := buildLogDashboardBucketStarts(config, now)
	groupSeriesOrder := make([]string, 0, len(groups))
	for _, groupStat := range groups {
		groupSeriesOrder = append(groupSeriesOrder, groupStat.GroupName)
	}
	channelSeriesOrder := make([]string, 0, len(channels))
	channelSeriesMetrics := make(map[string]map[int64]*logDashboardTrendMetric, len(channels))
	for _, channelStat := range channels {
		channelSeriesOrder = append(channelSeriesOrder, channelStat.ChannelName)
		channelSeriesMetrics[channelStat.ChannelName] = channelTrendMetrics[channelStat.ChannelId]
	}
	aggregateGroupSeriesSet := make(map[string]struct{}, len(aggregateGroupNameSet))
	for name := range aggregateGroupNameSet {
		aggregateGroupSeriesSet[name] = struct{}{}
	}

	return &LogDashboardData{
		Window:           config.WindowKey,
		GeneratedAt:      endTimestamp,
		Summary:          summary,
		Trend:            buildLogDashboardTrend(config, now, successBuckets, failureBuckets),
		GroupTrend:       buildDimensionTrendPoints(bucketStarts, config.LabelLayout, groupSeriesOrder, groupTrendMetrics, aggregateGroupSeriesSet),
		ChannelTrend:     buildDimensionTrendPoints(bucketStarts, config.LabelLayout, channelSeriesOrder, channelSeriesMetrics, nil),
		Channels:         channels,
		Groups:           groups,
		TopErrorMessages: buildTopErrorMessageStats(overallErrorCounts, 10),
		TopStatusCodes:   buildTopStatusCodeStats(overallStatusCounts, 10),
	}, nil
}
