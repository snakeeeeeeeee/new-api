package common

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var LogConsumeExcludedUserIDs string

var (
	logConsumeExcludedUserIDsMu  sync.RWMutex
	logConsumeExcludedUserIDsSet = make(map[int]struct{})
)

func NormalizeLogConsumeExcludedUserIDs(value string) (string, []int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil, nil
	}

	parts := strings.Split(trimmed, ",")
	seen := make(map[int]struct{}, len(parts))
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", nil, fmt.Errorf("排除消费日志的用户 ID 配置格式无效：存在空白项")
		}
		userID, err := strconv.Atoi(part)
		if err != nil || userID <= 0 {
			return "", nil, fmt.Errorf("排除消费日志的用户 ID 配置格式无效：%q 不是正整数", part)
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		ids = append(ids, userID)
	}

	sort.Ints(ids)
	normalizedParts := make([]string, 0, len(ids))
	for _, userID := range ids {
		normalizedParts = append(normalizedParts, strconv.Itoa(userID))
	}
	return strings.Join(normalizedParts, ","), ids, nil
}

func SetLogConsumeExcludedUserIDs(value string) (string, error) {
	normalized, ids, err := NormalizeLogConsumeExcludedUserIDs(value)
	if err != nil {
		return "", err
	}

	next := make(map[int]struct{}, len(ids))
	for _, userID := range ids {
		next[userID] = struct{}{}
	}

	logConsumeExcludedUserIDsMu.Lock()
	logConsumeExcludedUserIDsSet = next
	LogConsumeExcludedUserIDs = normalized
	logConsumeExcludedUserIDsMu.Unlock()
	return normalized, nil
}

func IsLogConsumeExcludedUserID(userID int) bool {
	if userID <= 0 {
		return false
	}
	logConsumeExcludedUserIDsMu.RLock()
	_, ok := logConsumeExcludedUserIDsSet[userID]
	logConsumeExcludedUserIDsMu.RUnlock()
	return ok
}
