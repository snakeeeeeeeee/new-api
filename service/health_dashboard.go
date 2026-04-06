package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	healthDashboardCacheTTL = 30 * time.Second
)

type HealthDashboardSummary struct {
	TotalChecks        int     `json:"totalChecks"`
	PendingChecks      int     `json:"pendingChecks"`
	HealthyChecks      int     `json:"healthyChecks"`
	DegradedChecks     int     `json:"degradedChecks"`
	FailedChecks       int     `json:"failedChecks"`
	OverallHealthScore float64 `json:"overallHealthScore"`
	AverageLatencyMs   int     `json:"averageLatencyMs"`
	LastUpdatedAt      string  `json:"lastUpdatedAt"`
}

type HealthDashboardProvider struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type HealthDashboardGroup struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProviderID string `json:"providerId"`
	Count      int    `json:"count"`
}

type HealthDashboardHistoryItem struct {
	Status        string `json:"status"`
	LatencyMs     int    `json:"latencyMs"`
	PingLatencyMs int    `json:"pingLatencyMs"`
	CheckedAt     string `json:"checkedAt"`
	Message       string `json:"message"`
}

type HealthDashboardCheck struct {
	ID               string                       `json:"id"`
	Name             string                       `json:"name"`
	ProviderID       string                       `json:"providerId"`
	ProviderName     string                       `json:"providerName"`
	GroupID          string                       `json:"groupId"`
	GroupName        string                       `json:"groupName"`
	Endpoint         string                       `json:"endpoint"`
	Model            string                       `json:"model"`
	Description      string                       `json:"description"`
	Status           string                       `json:"status"`
	StatusLabel      string                       `json:"statusLabel"`
	LatencyMs        int                          `json:"latencyMs"`
	PingLatencyMs    int                          `json:"pingLatencyMs"`
	AverageLatencyMs int                          `json:"averageLatencyMs"`
	HealthScore      float64                      `json:"healthScore"`
	CheckedAt        string                       `json:"checkedAt"`
	Message          string                       `json:"message"`
	History          []HealthDashboardHistoryItem `json:"history"`
}

type HealthDashboardData struct {
	Summary   HealthDashboardSummary    `json:"summary"`
	Providers []HealthDashboardProvider `json:"providers"`
	Groups    []HealthDashboardGroup    `json:"groups"`
	Checks    []HealthDashboardCheck    `json:"checks"`
}

type healthDashboardCacheEntry struct {
	Data      HealthDashboardData
	URL       string
	ExpiresAt time.Time
}

var (
	healthDashboardCacheMu     sync.Mutex
	healthDashboardCacheEntryV *healthDashboardCacheEntry
	healthDashboardURLOverride string
)

func getHealthDashboardURL() string {
	if healthDashboardURLOverride != "" {
		return healthDashboardURLOverride
	}
	return system_setting.ResolveHealthDashboardURL(system_setting.HealthDashboardURL)
}

func cloneHealthDashboardData(data HealthDashboardData) (HealthDashboardData, error) {
	raw, err := common.Marshal(data)
	if err != nil {
		return HealthDashboardData{}, err
	}
	var copied HealthDashboardData
	if err := common.Unmarshal(raw, &copied); err != nil {
		return HealthDashboardData{}, err
	}
	return copied, nil
}

func FetchHealthDashboard(ctx context.Context) (*HealthDashboardData, bool, error) {
	currentURL := getHealthDashboardURL()
	if currentURL == "" {
		return nil, false, fmt.Errorf("health dashboard url is not configured")
	}
	healthDashboardCacheMu.Lock()
	if healthDashboardCacheEntryV != nil &&
		healthDashboardCacheEntryV.URL == currentURL &&
		time.Now().Before(healthDashboardCacheEntryV.ExpiresAt) {
		copied, err := cloneHealthDashboardData(healthDashboardCacheEntryV.Data)
		healthDashboardCacheMu.Unlock()
		if err != nil {
			return nil, false, err
		}
		return &copied, true, nil
	}
	healthDashboardCacheMu.Unlock()

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(requestCtx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, currentURL, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("health dashboard request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	var data HealthDashboardData
	if err := common.Unmarshal(body, &data); err != nil {
		return nil, false, err
	}

	copied, err := cloneHealthDashboardData(data)
	if err != nil {
		return nil, false, err
	}

	healthDashboardCacheMu.Lock()
	healthDashboardCacheEntryV = &healthDashboardCacheEntry{
		Data:      copied,
		URL:       currentURL,
		ExpiresAt: time.Now().Add(healthDashboardCacheTTL),
	}
	healthDashboardCacheMu.Unlock()

	return &copied, false, nil
}

func SetHealthDashboardURLForTest(url string) func() {
	healthDashboardCacheMu.Lock()
	prevURL := healthDashboardURLOverride
	prevCache := healthDashboardCacheEntryV
	healthDashboardURLOverride = url
	healthDashboardCacheEntryV = nil
	healthDashboardCacheMu.Unlock()
	return func() {
		healthDashboardCacheMu.Lock()
		healthDashboardURLOverride = prevURL
		healthDashboardCacheEntryV = prevCache
		healthDashboardCacheMu.Unlock()
	}
}
