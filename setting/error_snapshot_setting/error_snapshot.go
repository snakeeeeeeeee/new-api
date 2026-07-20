package error_snapshot_setting

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	DefaultTTLMinutes    = 60
	MinTTLMinutes        = 5
	MaxTTLMinutes        = 10080
	DefaultMaxStorageMiB = 256
	MinMaxStorageMiB     = 16
	MaxMaxStorageMiB     = 10240
	DefaultMaxFiles      = 1000
	MinMaxFiles          = 10
	MaxMaxFiles          = 100000
)

type Settings struct {
	Enabled            bool   `json:"enabled"`
	TTLMinutes         int    `json:"ttl_minutes"`
	MaxStorageMiB      int    `json:"max_storage_mib"`
	MaxFiles           int    `json:"max_files"`
	PriorityUserIDs    string `json:"priority_user_ids"`
	PriorityChannelIDs string `json:"priority_channel_ids"`
}

type Snapshot struct {
	Enabled            bool
	TTLMinutes         int
	MaxStorageMiB      int
	MaxFiles           int
	PriorityUserIDs    map[int]struct{}
	PriorityChannelIDs map[int]struct{}
}

var settings = Settings{
	Enabled:       false,
	TTLMinutes:    DefaultTTLMinutes,
	MaxStorageMiB: DefaultMaxStorageMiB,
	MaxFiles:      DefaultMaxFiles,
}

var currentSnapshot atomic.Value

func init() {
	config.GlobalConfig.Register("error_snapshot", &settings)
	RefreshSnapshot()
}

func GetSettings() Settings {
	snapshot := GetSnapshot()
	return Settings{
		Enabled:            snapshot.Enabled,
		TTLMinutes:         snapshot.TTLMinutes,
		MaxStorageMiB:      snapshot.MaxStorageMiB,
		MaxFiles:           snapshot.MaxFiles,
		PriorityUserIDs:    formatIDs(snapshot.PriorityUserIDs),
		PriorityChannelIDs: formatIDs(snapshot.PriorityChannelIDs),
	}
}

func GetSnapshot() Snapshot {
	if value := currentSnapshot.Load(); value != nil {
		return value.(Snapshot)
	}
	return RefreshSnapshot()
}

func RefreshSnapshot() Snapshot {
	next := Snapshot{
		Enabled:            settings.Enabled,
		TTLMinutes:         normalizeRange(settings.TTLMinutes, MinTTLMinutes, MaxTTLMinutes, DefaultTTLMinutes),
		MaxStorageMiB:      normalizeRange(settings.MaxStorageMiB, MinMaxStorageMiB, MaxMaxStorageMiB, DefaultMaxStorageMiB),
		MaxFiles:           normalizeRange(settings.MaxFiles, MinMaxFiles, MaxMaxFiles, DefaultMaxFiles),
		PriorityUserIDs:    parseIDs(settings.PriorityUserIDs),
		PriorityChannelIDs: parseIDs(settings.PriorityChannelIDs),
	}
	settings.TTLMinutes = next.TTLMinutes
	settings.MaxStorageMiB = next.MaxStorageMiB
	settings.MaxFiles = next.MaxFiles
	settings.PriorityUserIDs = formatIDs(next.PriorityUserIDs)
	settings.PriorityChannelIDs = formatIDs(next.PriorityChannelIDs)
	currentSnapshot.Store(next)
	return next
}

func IsPriority(userID, channelID int) bool {
	snapshot := GetSnapshot()
	if userID > 0 {
		if _, ok := snapshot.PriorityUserIDs[userID]; ok {
			return true
		}
	}
	if channelID > 0 {
		_, ok := snapshot.PriorityChannelIDs[channelID]
		return ok
	}
	return false
}

func NormalizeOptionValue(key, value string) (string, error) {
	value = strings.TrimSpace(value)
	switch key {
	case "enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("error snapshot enabled must be true or false")
		}
		return strconv.FormatBool(parsed), nil
	case "ttl_minutes":
		return normalizeIntegerOption(value, MinTTLMinutes, MaxTTLMinutes, "error snapshot TTL minutes")
	case "max_storage_mib":
		return normalizeIntegerOption(value, MinMaxStorageMiB, MaxMaxStorageMiB, "error snapshot storage MiB")
	case "max_files":
		return normalizeIntegerOption(value, MinMaxFiles, MaxMaxFiles, "error snapshot max files")
	case "priority_user_ids", "priority_channel_ids":
		ids, err := parseIDList(value)
		if err != nil {
			return "", err
		}
		return formatIDs(ids), nil
	default:
		return "", fmt.Errorf("unknown error snapshot option %q", key)
	}
}

func normalizeIntegerOption(value string, minValue, maxValue int, label string) (string, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue || parsed > maxValue {
		return "", fmt.Errorf("%s must be between %d and %d", label, minValue, maxValue)
	}
	return strconv.Itoa(parsed), nil
}

func parseIDList(value string) (map[int]struct{}, error) {
	result := make(map[int]struct{})
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("ID list must contain positive integers")
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func parseIDs(value string) map[int]struct{} {
	ids, err := parseIDList(value)
	if err != nil {
		return map[int]struct{}{}
	}
	return ids
}

func formatIDs(ids map[int]struct{}) string {
	values := make([]int, 0, len(ids))
	for id := range ids {
		values = append(values, id)
	}
	sort.Ints(values)
	parts := make([]string, 0, len(values))
	for _, id := range values {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

func normalizeRange(value, minValue, maxValue, fallback int) int {
	if value < minValue || value > maxValue {
		return fallback
	}
	return value
}
