package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AggregateGroupStatusEnabled  = 1
	AggregateGroupStatusDisabled = 2

	AggregateGroupRoutingModeFailover = "failover"
	AggregateGroupRoutingModeCluster  = "cluster"

	AggregateGroupRouteAffinityStrategyPlatformUser = "platform_user"
	AggregateGroupRouteAffinityStrategyRequestFirst = "request_first"
	AggregateGroupRouteAffinityStrategyRequestOnly  = "request_only"
	AggregateGroupRouteAffinityStrategyOff          = "off"

	AggregateGroupTargetDefaultWeight              = 100
	AggregateGroupClusterAffinityTTLDefaultSeconds = 300

	AggregateGroupClientTypeClaudeCodeCLI      = "claude_code_cli"
	AggregateGroupClientRoutePoolClaudeCodeCLI = "claude_code_cli"
)

type AggregateGroup struct {
	Id                        int                    `json:"id"`
	Name                      string                 `json:"name" gorm:"size:64;not null;uniqueIndex:uk_aggregate_group_name,where:deleted_at IS NULL"`
	DisplayName               string                 `json:"display_name" gorm:"size:128;not null"`
	Description               string                 `json:"description,omitempty" gorm:"type:varchar(255)"`
	Status                    int                    `json:"status" gorm:"default:1;index"`
	GroupRatio                float64                `json:"group_ratio" gorm:"default:1"`
	RoutingMode               string                 `json:"routing_mode" gorm:"size:32;default:failover"`
	SmartRoutingEnabled       bool                   `json:"smart_routing_enabled" gorm:"default:false"`
	RecoveryEnabled           bool                   `json:"recovery_enabled" gorm:"default:true"`
	RecoveryIntervalSeconds   int                    `json:"recovery_interval_seconds" gorm:"default:300"`
	ClusterAffinityTTLSeconds int                    `json:"cluster_affinity_ttl_seconds" gorm:"default:300"`
	RouteAffinityStrategy     string                 `json:"route_affinity_strategy" gorm:"size:32;default:platform_user"`
	RouteAffinityKeySources   string                 `json:"-" gorm:"type:text"`
	RetryStatusCodes          string                 `json:"retry_status_codes" gorm:"type:text"`
	VisibleUserGroups         string                 `json:"-" gorm:"type:text"`
	ClientRoutePools          string                 `json:"-" gorm:"type:text"`
	SmartStrategyConfig       string                 `json:"-" gorm:"type:text"`
	CreatedTime               int64                  `json:"created_time" gorm:"bigint"`
	UpdatedTime               int64                  `json:"updated_time" gorm:"bigint"`
	DeletedAt                 gorm.DeletedAt         `json:"-" gorm:"index"`
	Targets                   []AggregateGroupTarget `json:"targets,omitempty" gorm:"foreignKey:AggregateGroupId"`
}

type AggregateGroupTarget struct {
	Id               int    `json:"id"`
	AggregateGroupId int    `json:"aggregate_group_id" gorm:"index;uniqueIndex:uk_aggregate_group_target"`
	RealGroup        string `json:"real_group" gorm:"size:64;not null;uniqueIndex:uk_aggregate_group_target"`
	OrderIndex       int    `json:"order_index" gorm:"index"`
	Weight           *int   `json:"weight" gorm:"default:100"`
	RPMLimit         int    `json:"rpm_limit" gorm:"default:0"`
}

type AggregateGroupClientRoutePools struct {
	Enabled       bool                          `json:"enabled"`
	ClaudeCodeCLI AggregateGroupClientRoutePool `json:"claude_code_cli"`
}

type AggregateGroupClientRoutePool struct {
	Enabled           bool                                  `json:"enabled"`
	FallbackToDefault *bool                                 `json:"fallback_to_default"`
	Targets           []AggregateGroupClientRoutePoolTarget `json:"targets"`
}

type AggregateGroupClientRoutePoolTarget struct {
	RealGroup string `json:"real_group"`
	Weight    *int   `json:"weight"`
	RPMLimit  int    `json:"rpm_limit"`
}

type AggregateGroupRouteAffinityKeySource struct {
	Type string `json:"type"`
	Key  string `json:"key,omitempty"`
	Path string `json:"path,omitempty"`
}

type AggregateGroupSmartStrategyConfig struct {
	FailureRateWindowSeconds   *int `json:"failure_rate_window_seconds,omitempty"`
	FailureRateMinRequests     *int `json:"failure_rate_min_requests,omitempty"`
	FailureRateThresholdPct    *int `json:"failure_rate_threshold_percent,omitempty"`
	SlowRateWindowSeconds      *int `json:"slow_rate_window_seconds,omitempty"`
	SlowRateMinRequests        *int `json:"slow_rate_min_requests,omitempty"`
	SlowRateThresholdPct       *int `json:"slow_rate_threshold_percent,omitempty"`
	DegradeDurationSeconds     *int `json:"degrade_duration_seconds,omitempty"`
	ClusterDegradedWeightPct   *int `json:"cluster_degraded_weight_percent,omitempty"`
	SlowRequestThreshold       *int `json:"slow_request_threshold_seconds,omitempty"`
	SlowFirstResponseThreshold *int `json:"slow_first_response_threshold_seconds,omitempty"`
}

func (g *AggregateGroup) IsEnabled() bool {
	return g != nil && g.Status == AggregateGroupStatusEnabled
}

func NormalizeAggregateGroupRoutingMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case AggregateGroupRoutingModeCluster:
		return AggregateGroupRoutingModeCluster
	default:
		return AggregateGroupRoutingModeFailover
	}
}

func IsValidAggregateGroupRoutingMode(mode string) bool {
	mode = strings.TrimSpace(mode)
	return mode == "" || mode == AggregateGroupRoutingModeFailover || mode == AggregateGroupRoutingModeCluster
}

func (g *AggregateGroup) GetRoutingMode() string {
	if g == nil {
		return AggregateGroupRoutingModeFailover
	}
	return NormalizeAggregateGroupRoutingMode(g.RoutingMode)
}

func NormalizeAggregateGroupRouteAffinityStrategy(strategy string) string {
	switch strings.TrimSpace(strategy) {
	case AggregateGroupRouteAffinityStrategyRequestFirst:
		return AggregateGroupRouteAffinityStrategyRequestFirst
	case AggregateGroupRouteAffinityStrategyRequestOnly:
		return AggregateGroupRouteAffinityStrategyRequestOnly
	case AggregateGroupRouteAffinityStrategyOff:
		return AggregateGroupRouteAffinityStrategyOff
	default:
		return AggregateGroupRouteAffinityStrategyPlatformUser
	}
}

func IsValidAggregateGroupRouteAffinityStrategy(strategy string) bool {
	strategy = strings.TrimSpace(strategy)
	return strategy == "" ||
		strategy == AggregateGroupRouteAffinityStrategyPlatformUser ||
		strategy == AggregateGroupRouteAffinityStrategyRequestFirst ||
		strategy == AggregateGroupRouteAffinityStrategyRequestOnly ||
		strategy == AggregateGroupRouteAffinityStrategyOff
}

func (g *AggregateGroup) GetRouteAffinityStrategy() string {
	if g == nil {
		return AggregateGroupRouteAffinityStrategyPlatformUser
	}
	return NormalizeAggregateGroupRouteAffinityStrategy(g.RouteAffinityStrategy)
}

func (t *AggregateGroupTarget) GetWeight() int {
	if t == nil || t.Weight == nil {
		return AggregateGroupTargetDefaultWeight
	}
	return *t.Weight
}

func (t *AggregateGroupTarget) GetRPMLimit() int {
	if t == nil || t.RPMLimit < 0 {
		return 0
	}
	return t.RPMLimit
}

func (t *AggregateGroupClientRoutePoolTarget) GetWeight() int {
	if t == nil || t.Weight == nil {
		return AggregateGroupTargetDefaultWeight
	}
	return *t.Weight
}

func (t *AggregateGroupClientRoutePoolTarget) GetRPMLimit() int {
	if t == nil || t.RPMLimit < 0 {
		return 0
	}
	return t.RPMLimit
}

func (p *AggregateGroupClientRoutePool) GetFallbackToDefault() bool {
	if p == nil || p.FallbackToDefault == nil {
		return true
	}
	return *p.FallbackToDefault
}

func NormalizeAggregateGroupClusterAffinityTTLSeconds(seconds int) int {
	if seconds <= 0 {
		return AggregateGroupClusterAffinityTTLDefaultSeconds
	}
	return seconds
}

func (g *AggregateGroup) GetClusterAffinityTTLSeconds() int {
	if g == nil {
		return AggregateGroupClusterAffinityTTLDefaultSeconds
	}
	return NormalizeAggregateGroupClusterAffinityTTLSeconds(g.ClusterAffinityTTLSeconds)
}

func (g *AggregateGroup) GetVisibleUserGroups() []string {
	if g == nil || strings.TrimSpace(g.VisibleUserGroups) == "" {
		return []string{}
	}
	var groups []string
	if err := common.UnmarshalJsonStr(g.VisibleUserGroups, &groups); err != nil {
		common.SysError("failed to unmarshal aggregate group visible user groups: " + err.Error())
		return []string{}
	}
	return groups
}

func (g *AggregateGroup) GetRouteAffinityKeySources() []AggregateGroupRouteAffinityKeySource {
	if g == nil || strings.TrimSpace(g.RouteAffinityKeySources) == "" {
		return []AggregateGroupRouteAffinityKeySource{}
	}
	var sources []AggregateGroupRouteAffinityKeySource
	if err := common.UnmarshalJsonStr(g.RouteAffinityKeySources, &sources); err != nil {
		common.SysError("failed to unmarshal aggregate group route affinity key sources: " + err.Error())
		return []AggregateGroupRouteAffinityKeySource{}
	}
	return sources
}

func NormalizeAggregateGroupClientRoutePools(config AggregateGroupClientRoutePools) AggregateGroupClientRoutePools {
	if config.ClaudeCodeCLI.FallbackToDefault == nil {
		config.ClaudeCodeCLI.FallbackToDefault = common.GetPointer(true)
	}
	if config.ClaudeCodeCLI.Targets == nil {
		config.ClaudeCodeCLI.Targets = []AggregateGroupClientRoutePoolTarget{}
	}
	return config
}

func (g *AggregateGroup) GetClientRoutePools() AggregateGroupClientRoutePools {
	if g == nil || strings.TrimSpace(g.ClientRoutePools) == "" {
		return NormalizeAggregateGroupClientRoutePools(AggregateGroupClientRoutePools{})
	}
	var config AggregateGroupClientRoutePools
	if err := common.UnmarshalJsonStr(g.ClientRoutePools, &config); err != nil {
		common.SysError("failed to unmarshal aggregate group client route pools: " + err.Error())
		return NormalizeAggregateGroupClientRoutePools(AggregateGroupClientRoutePools{})
	}
	return NormalizeAggregateGroupClientRoutePools(config)
}

func (g *AggregateGroup) SetClientRoutePools(config AggregateGroupClientRoutePools) error {
	config = NormalizeAggregateGroupClientRoutePools(config)
	jsonBytes, err := common.Marshal(config)
	if err != nil {
		return err
	}
	g.ClientRoutePools = string(jsonBytes)
	return nil
}

func (g *AggregateGroup) SetRouteAffinityKeySources(sources []AggregateGroupRouteAffinityKeySource) error {
	if len(sources) == 0 {
		g.RouteAffinityKeySources = ""
		return nil
	}
	jsonBytes, err := common.Marshal(sources)
	if err != nil {
		return err
	}
	g.RouteAffinityKeySources = string(jsonBytes)
	return nil
}

func (g *AggregateGroup) GetSmartStrategyConfig() *AggregateGroupSmartStrategyConfig {
	if g == nil || strings.TrimSpace(g.SmartStrategyConfig) == "" {
		return nil
	}
	var config AggregateGroupSmartStrategyConfig
	if err := common.UnmarshalJsonStr(g.SmartStrategyConfig, &config); err != nil {
		common.SysError("failed to unmarshal aggregate group smart strategy config: " + err.Error())
		return nil
	}
	return &config
}

func (g *AggregateGroup) SetSmartStrategyConfig(config *AggregateGroupSmartStrategyConfig) error {
	if config == nil {
		g.SmartStrategyConfig = ""
		return nil
	}
	jsonBytes, err := common.Marshal(config)
	if err != nil {
		return err
	}
	g.SmartStrategyConfig = string(jsonBytes)
	return nil
}

func (g *AggregateGroup) SetVisibleUserGroups(groups []string) error {
	jsonBytes, err := common.Marshal(groups)
	if err != nil {
		return err
	}
	g.VisibleUserGroups = string(jsonBytes)
	return nil
}

func (g *AggregateGroup) GetLabel() string {
	if g == nil {
		return ""
	}
	if strings.TrimSpace(g.DisplayName) != "" {
		return g.DisplayName
	}
	if strings.TrimSpace(g.Description) != "" {
		return g.Description
	}
	return g.Name
}

func (g *AggregateGroup) GetTargetGroups() []string {
	if g == nil || len(g.Targets) == 0 {
		return []string{}
	}
	groups := make([]string, 0, len(g.Targets))
	for _, target := range g.Targets {
		if strings.TrimSpace(target.RealGroup) == "" {
			continue
		}
		groups = append(groups, target.RealGroup)
	}
	return groups
}

func (g *AggregateGroup) InsertWithTargets(targets []AggregateGroupTarget) error {
	now := common.GetTimestamp()
	g.CreatedTime = now
	g.UpdatedTime = now
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(g).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		for i := range targets {
			targets[i].Id = 0
			targets[i].AggregateGroupId = g.Id
		}
		return tx.Create(&targets).Error
	})
}

func (g *AggregateGroup) UpdateWithTargets(targets []AggregateGroupTarget) error {
	if g == nil || g.Id == 0 {
		return errors.New("aggregate group id is required")
	}
	g.UpdatedTime = common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AggregateGroup{}).Where("id = ?", g.Id).Updates(map[string]interface{}{
			"name":                         g.Name,
			"display_name":                 g.DisplayName,
			"description":                  g.Description,
			"status":                       g.Status,
			"group_ratio":                  g.GroupRatio,
			"routing_mode":                 g.RoutingMode,
			"smart_routing_enabled":        g.SmartRoutingEnabled,
			"recovery_enabled":             g.RecoveryEnabled,
			"recovery_interval_seconds":    g.RecoveryIntervalSeconds,
			"cluster_affinity_ttl_seconds": g.ClusterAffinityTTLSeconds,
			"route_affinity_strategy":      g.RouteAffinityStrategy,
			"route_affinity_key_sources":   g.RouteAffinityKeySources,
			"retry_status_codes":           g.RetryStatusCodes,
			"visible_user_groups":          g.VisibleUserGroups,
			"client_route_pools":           g.ClientRoutePools,
			"smart_strategy_config":        g.SmartStrategyConfig,
			"updated_time":                 g.UpdatedTime,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("aggregate_group_id = ?", g.Id).Delete(&AggregateGroupTarget{}).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		for i := range targets {
			targets[i].Id = 0
			targets[i].AggregateGroupId = g.Id
		}
		return tx.Create(&targets).Error
	})
}

func DeleteAggregateGroupByID(id int) error {
	if id == 0 {
		return errors.New("aggregate group id is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("aggregate_group_id = ?", id).Delete(&AggregateGroupTarget{}).Error; err != nil {
			return err
		}
		return tx.Delete(&AggregateGroup{}, id).Error
	})
}

func IsAggregateGroupNameDuplicated(id int, name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&AggregateGroup{}).
		Where("name = ? AND id <> ?", name, id).
		Count(&count).Error
	return count > 0, err
}

func GetAggregateGroupByID(id int) (*AggregateGroup, error) {
	if id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var group AggregateGroup
	tx := DB.Preload("Targets", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_index ASC")
	}).Where("id = ?", id).Limit(1).Find(&group)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &group, nil
}

func GetAggregateGroupByName(name string, enabledOnly bool) (*AggregateGroup, error) {
	if strings.TrimSpace(name) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var group AggregateGroup
	query := DB.Preload("Targets", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_index ASC")
	}).Where("name = ?", name)
	if enabledOnly {
		query = query.Where("status = ?", AggregateGroupStatusEnabled)
	}
	tx := query.Limit(1).Find(&group)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &group, nil
}

func GetAllAggregateGroups(enabledOnly bool) ([]*AggregateGroup, error) {
	var groups []*AggregateGroup
	query := DB.Preload("Targets", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_index ASC")
	}).Order("updated_time DESC")
	if enabledOnly {
		query = query.Where("status = ?", AggregateGroupStatusEnabled)
	}
	if err := query.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
