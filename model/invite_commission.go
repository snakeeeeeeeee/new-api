package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const InviteCommissionSettingsOptionKey = "InviteCommissionSettings"

type InviteCommissionUserConfig struct {
	Id            int    `json:"id"`
	UserID        int    `json:"user_id" gorm:"column:user_id;uniqueIndex"`
	Enabled       bool   `json:"enabled"`
	Level1RateBps int    `json:"level1_rate_bps" gorm:"type:int"`
	Level2RateBps int    `json:"level2_rate_bps" gorm:"type:int"`
	Remark        string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`

	Username    string `json:"username,omitempty" gorm:"-"`
	DisplayName string `json:"display_name,omitempty" gorm:"-"`
}

func (c *InviteCommissionUserConfig) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	c.CreatedAt = now
	c.UpdatedAt = now
	return nil
}

func (c *InviteCommissionUserConfig) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = common.GetTimestamp()
	return nil
}

type InviteCommissionSubscriptionTier struct {
	StartPercent float64 `json:"start_percent"`
	EndPercent   float64 `json:"end_percent"`
	RateBps      int     `json:"rate_bps"`
}

type InviteCommissionServiceCategory struct {
	Service string `json:"service"`
	Label   string `json:"label"`
	Remark  string `json:"remark"`
}

type InviteCommissionGroupProfitRule struct {
	Group                   string `json:"group"`
	Service                 string `json:"service"`
	ProfitRateBps           int    `json:"profit_rate_bps"`
	MaxCommissionRateBps    int    `json:"max_commission_rate_bps"`
	ProfitShareRateBps      int    `json:"profit_share_rate_bps"`
	ProfitProtectionEnabled bool   `json:"profit_protection_enabled"`
	Remark                  string `json:"remark"`
}

type InviteCommissionGroupProfitRuleRow struct {
	Group                   string `json:"group"`
	Type                    string `json:"type"`
	Configured              bool   `json:"configured"`
	Service                 string `json:"service"`
	ProfitRateBps           int    `json:"profit_rate_bps"`
	MaxCommissionRateBps    int    `json:"max_commission_rate_bps"`
	ProfitShareRateBps      int    `json:"profit_share_rate_bps"`
	ProfitProtectionEnabled bool   `json:"profit_protection_enabled"`
	Remark                  string `json:"remark"`
}

type InviteCommissionSettings struct {
	DefaultLevel1RateBps int                                `json:"default_level1_rate_bps"`
	DefaultLevel2RateBps int                                `json:"default_level2_rate_bps"`
	ServiceCategories    []InviteCommissionServiceCategory  `json:"service_categories"`
	SubscriptionTiers    []InviteCommissionSubscriptionTier `json:"subscription_tiers"`
	GroupProfitRules     []InviteCommissionGroupProfitRule  `json:"group_profit_rules"`
}

type InviteCommissionEffectiveConfig struct {
	Enabled       bool `json:"enabled"`
	UserConfigID  int  `json:"user_config_id,omitempty"`
	UseUserConfig bool `json:"use_user_config"`
	Level1RateBps int  `json:"level1_rate_bps"`
	Level2RateBps int  `json:"level2_rate_bps"`
}

type InviteCommissionReportRequest struct {
	OwnerUserID    int
	StartTimestamp int64
	EndTimestamp   int64
	IncludeDetails bool
}

type InviteCommissionReport struct {
	OwnerUserID    int                             `json:"owner_user_id"`
	OwnerUsername  string                          `json:"owner_username"`
	StartTimestamp int64                           `json:"start_timestamp"`
	EndTimestamp   int64                           `json:"end_timestamp"`
	Settings       InviteCommissionSettings        `json:"settings"`
	Effective      InviteCommissionEffectiveConfig `json:"effective"`
	Summary        InviteCommissionSummary         `json:"summary"`
	Levels         []InviteCommissionLevelSummary  `json:"levels"`
	Services       []InviteCommissionServiceStat   `json:"services"`
	Groups         []InviteCommissionGroupStat     `json:"groups,omitempty"`
	Invitees       []InviteCommissionInviteeStat   `json:"invitees,omitempty"`
	Models         []InviteCommissionModelStat     `json:"models,omitempty"`
}

type InviteCommissionSummary struct {
	InviteeCount                 int     `json:"invitee_count"`
	Level1InviteeCount           int     `json:"level1_invitee_count"`
	Level2InviteeCount           int     `json:"level2_invitee_count"`
	WalletRechargeAmount         int64   `json:"wallet_recharge_amount"`
	WalletRechargeMoney          float64 `json:"wallet_recharge_money"`
	RedemptionQuota              int64   `json:"redemption_quota"`
	SubscriptionPurchaseMoney    float64 `json:"subscription_purchase_money"`
	WalletConsumptionQuota       int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota        int64   `json:"total_consumption_quota"`
	EstimatedCommissionQuota     float64 `json:"estimated_commission_quota"`
	WalletCommissionQuota        float64 `json:"wallet_commission_quota"`
	SubscriptionCommissionQuota  float64 `json:"subscription_commission_quota"`
	UpstreamCostQuota            int64   `json:"upstream_cost_quota,omitempty"`
	GrossProfitQuota             int64   `json:"gross_profit_quota,omitempty"`
	TheoreticalCommissionQuota   float64 `json:"theoretical_commission_quota,omitempty"`
	ProfitCapCommissionQuota     float64 `json:"profit_cap_commission_quota,omitempty"`
	ProfitProtectionReducedQuota float64 `json:"profit_protection_reduced_quota,omitempty"`
	MissingProfitSnapshotQuota   int64   `json:"missing_profit_snapshot_quota,omitempty"`
	AdminSubscriptionConsumption int64   `json:"admin_subscription_consumption_quota"`
	AdminSubscriptionCommission  float64 `json:"admin_subscription_commission_quota"`
	OrderSubscriptionConsumption int64   `json:"order_subscription_consumption_quota"`
	OrderSubscriptionCommission  float64 `json:"order_subscription_commission_quota"`
}

type InviteCommissionLevelSummary struct {
	Level                        int     `json:"level"`
	InviteeCount                 int     `json:"invitee_count"`
	RateBps                      int     `json:"rate_bps"`
	WalletRechargeAmount         int64   `json:"wallet_recharge_amount"`
	WalletRechargeMoney          float64 `json:"wallet_recharge_money"`
	RedemptionQuota              int64   `json:"redemption_quota"`
	SubscriptionPurchaseMoney    float64 `json:"subscription_purchase_money"`
	WalletConsumptionQuota       int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota        int64   `json:"total_consumption_quota"`
	EstimatedCommissionQuota     float64 `json:"estimated_commission_quota"`
	WalletCommissionQuota        float64 `json:"wallet_commission_quota"`
	SubscriptionCommissionQuota  float64 `json:"subscription_commission_quota"`
	UpstreamCostQuota            int64   `json:"upstream_cost_quota,omitempty"`
	GrossProfitQuota             int64   `json:"gross_profit_quota,omitempty"`
	TheoreticalCommissionQuota   float64 `json:"theoretical_commission_quota,omitempty"`
	ProfitCapCommissionQuota     float64 `json:"profit_cap_commission_quota,omitempty"`
	ProfitProtectionReducedQuota float64 `json:"profit_protection_reduced_quota,omitempty"`
	MissingProfitSnapshotQuota   int64   `json:"missing_profit_snapshot_quota,omitempty"`
	AdminSubscriptionConsumption int64   `json:"admin_subscription_consumption_quota"`
	OrderSubscriptionConsumption int64   `json:"order_subscription_consumption_quota"`
}

type InviteCommissionServiceStat struct {
	Service                      string  `json:"service"`
	Label                        string  `json:"label"`
	RequestCount                 int64   `json:"request_count"`
	PromptTokens                 int64   `json:"prompt_tokens"`
	CompletionTokens             int64   `json:"completion_tokens"`
	WalletConsumptionQuota       int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota        int64   `json:"total_consumption_quota"`
	EstimatedCommissionQuota     float64 `json:"estimated_commission_quota"`
	WalletCommissionQuota        float64 `json:"wallet_commission_quota"`
	SubscriptionCommissionQuota  float64 `json:"subscription_commission_quota"`
	UpstreamCostQuota            int64   `json:"upstream_cost_quota,omitempty"`
	GrossProfitQuota             int64   `json:"gross_profit_quota,omitempty"`
	TheoreticalCommissionQuota   float64 `json:"theoretical_commission_quota,omitempty"`
	ProfitCapCommissionQuota     float64 `json:"profit_cap_commission_quota,omitempty"`
	ProfitProtectionReducedQuota float64 `json:"profit_protection_reduced_quota,omitempty"`
	MissingProfitSnapshotQuota   int64   `json:"missing_profit_snapshot_quota,omitempty"`
	AdminSubscriptionConsumption int64   `json:"admin_subscription_consumption_quota"`
	OrderSubscriptionConsumption int64   `json:"order_subscription_consumption_quota"`
}

type InviteCommissionInviteeStat struct {
	UserID                       int     `json:"user_id"`
	Username                     string  `json:"username"`
	Level                        int     `json:"level"`
	DirectOwnerUserID            int     `json:"direct_owner_user_id"`
	DirectOwnerUsername          string  `json:"direct_owner_username,omitempty"`
	WalletRechargeAmount         int64   `json:"wallet_recharge_amount"`
	WalletRechargeMoney          float64 `json:"wallet_recharge_money"`
	RedemptionQuota              int64   `json:"redemption_quota"`
	SubscriptionPurchaseMoney    float64 `json:"subscription_purchase_money"`
	WalletConsumptionQuota       int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota        int64   `json:"total_consumption_quota"`
	EstimatedCommissionQuota     float64 `json:"estimated_commission_quota"`
	WalletCommissionQuota        float64 `json:"wallet_commission_quota"`
	SubscriptionCommissionQuota  float64 `json:"subscription_commission_quota"`
	UpstreamCostQuota            int64   `json:"upstream_cost_quota,omitempty"`
	GrossProfitQuota             int64   `json:"gross_profit_quota,omitempty"`
	TheoreticalCommissionQuota   float64 `json:"theoretical_commission_quota,omitempty"`
	ProfitCapCommissionQuota     float64 `json:"profit_cap_commission_quota,omitempty"`
	ProfitProtectionReducedQuota float64 `json:"profit_protection_reduced_quota,omitempty"`
	MissingProfitSnapshotQuota   int64   `json:"missing_profit_snapshot_quota,omitempty"`
	AdminSubscriptionConsumption int64   `json:"admin_subscription_consumption_quota"`
	OrderSubscriptionConsumption int64   `json:"order_subscription_consumption_quota"`
}

type InviteCommissionGroupStat struct {
	Group                         string  `json:"group"`
	Type                          string  `json:"type"`
	Service                       string  `json:"service"`
	ServiceLabel                  string  `json:"service_label"`
	RequestCount                  int64   `json:"request_count"`
	WalletConsumptionQuota        int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota  int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota         int64   `json:"total_consumption_quota"`
	UpstreamCostQuota             int64   `json:"upstream_cost_quota"`
	GrossProfitQuota              int64   `json:"gross_profit_quota"`
	TheoreticalCommissionQuota    float64 `json:"theoretical_commission_quota"`
	ProfitCapCommissionQuota      float64 `json:"profit_cap_commission_quota"`
	EstimatedCommissionQuota      float64 `json:"estimated_commission_quota"`
	ProfitProtectionReducedQuota  float64 `json:"profit_protection_reduced_quota"`
	MissingProfitSnapshotQuota    int64   `json:"missing_profit_snapshot_quota"`
	ConfiguredProfitSnapshotCount int64   `json:"configured_profit_snapshot_count"`
	MissingProfitSnapshotCount    int64   `json:"missing_profit_snapshot_count"`
}

type InviteCommissionModelStat struct {
	ModelName                    string  `json:"model_name"`
	Service                      string  `json:"service"`
	ServiceLabel                 string  `json:"service_label"`
	RequestCount                 int64   `json:"request_count"`
	PromptTokens                 int64   `json:"prompt_tokens"`
	CompletionTokens             int64   `json:"completion_tokens"`
	WalletConsumptionQuota       int64   `json:"wallet_consumption_quota"`
	SubscriptionConsumptionQuota int64   `json:"subscription_consumption_quota"`
	TotalConsumptionQuota        int64   `json:"total_consumption_quota"`
	EstimatedCommissionQuota     float64 `json:"estimated_commission_quota"`
	WalletCommissionQuota        float64 `json:"wallet_commission_quota"`
	SubscriptionCommissionQuota  float64 `json:"subscription_commission_quota"`
	UpstreamCostQuota            int64   `json:"upstream_cost_quota,omitempty"`
	GrossProfitQuota             int64   `json:"gross_profit_quota,omitempty"`
	TheoreticalCommissionQuota   float64 `json:"theoretical_commission_quota,omitempty"`
	ProfitCapCommissionQuota     float64 `json:"profit_cap_commission_quota,omitempty"`
	ProfitProtectionReducedQuota float64 `json:"profit_protection_reduced_quota,omitempty"`
	MissingProfitSnapshotQuota   int64   `json:"missing_profit_snapshot_quota,omitempty"`
	AdminSubscriptionConsumption int64   `json:"admin_subscription_consumption_quota"`
	OrderSubscriptionConsumption int64   `json:"order_subscription_consumption_quota"`
}

type inviteCommissionUserBrief struct {
	UserID            int
	Username          string
	Level             int
	DirectOwnerUserID int
}

type inviteCommissionLogOther struct {
	BillingSource          string `json:"billing_source"`
	UpstreamModelName      string `json:"upstream_model_name"`
	SubscriptionID         int    `json:"subscription_id"`
	SubscriptionTotal      int64  `json:"subscription_total"`
	SubscriptionUsed       int64  `json:"subscription_used"`
	SubscriptionConsumed   int64  `json:"subscription_consumed"`
	SubscriptionPreConsume int64  `json:"subscription_pre_consumed"`
	SubscriptionPostDelta  int64  `json:"subscription_post_delta"`
	CommissionSnapshot     InviteCommissionLogCommissionSnapshot
	HasCommissionSnapshot  bool
}

type InviteCommissionLogCommissionSnapshot struct {
	Group                   string `json:"group"`
	RouteGroup              string `json:"route_group,omitempty"`
	Service                 string `json:"service"`
	Configured              bool   `json:"configured"`
	ProfitRateBps           int    `json:"profit_rate_bps"`
	CostRateBps             int    `json:"cost_rate_bps"`
	MaxCommissionRateBps    int    `json:"max_commission_rate_bps"`
	ProfitShareRateBps      int    `json:"profit_share_rate_bps"`
	ProfitProtectionEnabled bool   `json:"profit_protection_enabled"`
	RevenueQuota            int64  `json:"revenue_quota"`
	UpstreamCostQuota       int64  `json:"upstream_cost_quota"`
	GrossProfitQuota        int64  `json:"gross_profit_quota"`
	Source                  string `json:"source"`
}

func DefaultInviteCommissionSettings() InviteCommissionSettings {
	return InviteCommissionSettings{
		DefaultLevel1RateBps: 500,
		DefaultLevel2RateBps: 150,
		ServiceCategories: []InviteCommissionServiceCategory{
			{Service: "gpt", Label: "GPT"},
			{Service: "claude", Label: "Claude"},
			{Service: "gemini", Label: "Gemini"},
			{Service: "other", Label: "Other"},
		},
		SubscriptionTiers: []InviteCommissionSubscriptionTier{
			{StartPercent: 0, EndPercent: 33, RateBps: 1500},
			{StartPercent: 33, EndPercent: 66, RateBps: 750},
			{StartPercent: 66, EndPercent: 100, RateBps: 0},
		},
	}
}

func DefaultInviteCommissionSettingsJSONString() string {
	raw, err := common.Marshal(DefaultInviteCommissionSettings())
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func normalizeInviteCommissionSettings(settings InviteCommissionSettings) InviteCommissionSettings {
	defaultSettings := DefaultInviteCommissionSettings()
	if settings.DefaultLevel1RateBps < 0 {
		settings.DefaultLevel1RateBps = 0
	}
	if settings.DefaultLevel2RateBps < 0 {
		settings.DefaultLevel2RateBps = 0
	}
	if len(settings.ServiceCategories) == 0 {
		settings.ServiceCategories = defaultSettings.ServiceCategories
	}
	settings.ServiceCategories = normalizeInviteCommissionServiceCategories(settings.ServiceCategories)
	if len(settings.ServiceCategories) == 0 {
		settings.ServiceCategories = defaultSettings.ServiceCategories
	}
	if len(settings.SubscriptionTiers) == 0 {
		settings.SubscriptionTiers = defaultSettings.SubscriptionTiers
	}
	for i := range settings.SubscriptionTiers {
		if settings.SubscriptionTiers[i].StartPercent < 0 {
			settings.SubscriptionTiers[i].StartPercent = 0
		}
		if settings.SubscriptionTiers[i].EndPercent < settings.SubscriptionTiers[i].StartPercent {
			settings.SubscriptionTiers[i].EndPercent = settings.SubscriptionTiers[i].StartPercent
		}
		if settings.SubscriptionTiers[i].RateBps < 0 {
			settings.SubscriptionTiers[i].RateBps = 0
		}
	}
	sort.SliceStable(settings.SubscriptionTiers, func(i, j int) bool {
		return settings.SubscriptionTiers[i].StartPercent < settings.SubscriptionTiers[j].StartPercent
	})
	settings.GroupProfitRules = normalizeInviteCommissionGroupProfitRules(settings.GroupProfitRules)
	return settings
}

func normalizeInviteCommissionServiceCategories(categories []InviteCommissionServiceCategory) []InviteCommissionServiceCategory {
	normalized := make([]InviteCommissionServiceCategory, 0, len(categories))
	seen := make(map[string]int)
	for _, category := range categories {
		category.Service = normalizeInviteCommissionService(category.Service)
		if category.Service == "" {
			continue
		}
		category.Label = strings.TrimSpace(category.Label)
		if category.Label == "" {
			category.Label = inviteCommissionDefaultServiceLabel(category.Service)
		}
		category.Remark = strings.TrimSpace(category.Remark)
		if idx, ok := seen[category.Service]; ok {
			normalized[idx] = category
			continue
		}
		seen[category.Service] = len(normalized)
		normalized = append(normalized, category)
	}
	return normalized
}

func normalizeInviteCommissionGroupProfitRules(rules []InviteCommissionGroupProfitRule) []InviteCommissionGroupProfitRule {
	normalized := make([]InviteCommissionGroupProfitRule, 0, len(rules))
	seen := make(map[string]int)
	for _, rule := range rules {
		rule.Group = normalizeInviteCommissionGroupName(rule.Group)
		if rule.Group == "" || shouldFilterInviteCommissionGroup(rule.Group) {
			continue
		}
		rule.Service = normalizeInviteCommissionService(rule.Service)
		if rule.Service == "" {
			rule.Service = "other"
		}
		rule.ProfitRateBps = clampInviteCommissionBps(rule.ProfitRateBps)
		rule.MaxCommissionRateBps = clampInviteCommissionBps(rule.MaxCommissionRateBps)
		rule.ProfitShareRateBps = clampInviteCommissionBps(rule.ProfitShareRateBps)
		rule.Remark = strings.TrimSpace(rule.Remark)
		if idx, ok := seen[rule.Group]; ok {
			normalized[idx] = rule
			continue
		}
		seen[rule.Group] = len(normalized)
		normalized = append(normalized, rule)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].Group < normalized[j].Group
	})
	return normalized
}

func clampInviteCommissionBps(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10000 {
		return 10000
	}
	return value
}

func GetInviteCommissionSettings() InviteCommissionSettings {
	raw := ""
	common.OptionMapRWMutex.RLock()
	if common.OptionMap != nil {
		raw = strings.TrimSpace(common.OptionMap[InviteCommissionSettingsOptionKey])
	}
	common.OptionMapRWMutex.RUnlock()

	if raw == "" && DB != nil {
		ensureCommonColumnsInitialized()
		var option Option
		if err := DB.First(&option, commonKeyCol+" = ?", InviteCommissionSettingsOptionKey).Error; err == nil {
			raw = strings.TrimSpace(option.Value)
		}
	}
	if raw == "" {
		return DefaultInviteCommissionSettings()
	}

	var settings InviteCommissionSettings
	if err := common.UnmarshalJsonStr(raw, &settings); err != nil {
		common.SysLog("failed to parse invite commission settings: " + err.Error())
		return DefaultInviteCommissionSettings()
	}
	return normalizeInviteCommissionSettings(settings)
}

func UpdateInviteCommissionSettings(settings InviteCommissionSettings) (InviteCommissionSettings, error) {
	settings = normalizeInviteCommissionSettings(settings)
	raw, err := common.Marshal(settings)
	if err != nil {
		return settings, err
	}
	if err := UpdateOption(InviteCommissionSettingsOptionKey, string(raw)); err != nil {
		return settings, err
	}
	return settings, nil
}

func ListInviteCommissionGroupProfitRuleRows(keyword string) ([]InviteCommissionGroupProfitRuleRow, error) {
	groupTypes, err := loadInviteCommissionAvailableGroups()
	if err != nil {
		return nil, err
	}
	settings := GetInviteCommissionSettings()
	ruleMap := inviteCommissionGroupProfitRuleMap(settings)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	groups := make([]string, 0, len(groupTypes))
	for group := range groupTypes {
		if keyword != "" && !strings.Contains(strings.ToLower(group), keyword) {
			continue
		}
		groups = append(groups, group)
	}
	sort.Strings(groups)
	rows := make([]InviteCommissionGroupProfitRuleRow, 0, len(groups))
	for _, group := range groups {
		row := InviteCommissionGroupProfitRuleRow{
			Group:   group,
			Type:    groupTypes[group],
			Service: "other",
		}
		if rule, ok := ruleMap[group]; ok {
			row.Configured = true
			row.Service = rule.Service
			row.ProfitRateBps = rule.ProfitRateBps
			row.MaxCommissionRateBps = rule.MaxCommissionRateBps
			row.ProfitShareRateBps = rule.ProfitShareRateBps
			row.ProfitProtectionEnabled = rule.ProfitProtectionEnabled
			row.Remark = rule.Remark
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func UpsertInviteCommissionGroupProfitRule(rule InviteCommissionGroupProfitRule) (InviteCommissionGroupProfitRule, error) {
	rules := normalizeInviteCommissionGroupProfitRules([]InviteCommissionGroupProfitRule{rule})
	if len(rules) == 0 {
		return InviteCommissionGroupProfitRule{}, errors.New("无效的分组利润规则")
	}
	rule = rules[0]
	groupTypes, err := loadInviteCommissionAvailableGroups()
	if err != nil {
		return InviteCommissionGroupProfitRule{}, err
	}
	if _, ok := groupTypes[rule.Group]; !ok {
		return InviteCommissionGroupProfitRule{}, errors.New("分组不存在或不允许配置")
	}
	settings := GetInviteCommissionSettings()
	replaced := false
	for i := range settings.GroupProfitRules {
		if normalizeInviteCommissionGroupName(settings.GroupProfitRules[i].Group) == rule.Group {
			settings.GroupProfitRules[i] = rule
			replaced = true
			break
		}
	}
	if !replaced {
		settings.GroupProfitRules = append(settings.GroupProfitRules, rule)
	}
	if _, err := UpdateInviteCommissionSettings(settings); err != nil {
		return InviteCommissionGroupProfitRule{}, err
	}
	return rule, nil
}

func DeleteInviteCommissionGroupProfitRule(group string) error {
	group = normalizeInviteCommissionGroupName(group)
	if group == "" {
		return errors.New("无效的分组")
	}
	settings := GetInviteCommissionSettings()
	rules := make([]InviteCommissionGroupProfitRule, 0, len(settings.GroupProfitRules))
	for _, rule := range settings.GroupProfitRules {
		if normalizeInviteCommissionGroupName(rule.Group) != group {
			rules = append(rules, rule)
		}
	}
	settings.GroupProfitRules = rules
	_, err := UpdateInviteCommissionSettings(settings)
	return err
}

func loadInviteCommissionAvailableGroups() (map[string]string, error) {
	ensureCommonColumnsInitialized()
	groups := make(map[string]string)
	for raw := range ratio_setting.GetGroupRatioCopy() {
		group := normalizeInviteCommissionGroupName(raw)
		if group == "" || shouldFilterInviteCommissionGroup(group) {
			continue
		}
		groups[group] = "ratio"
	}
	var channelGroups []string
	if err := DB.Model(&Channel{}).Pluck("group", &channelGroups).Error; err != nil {
		return nil, err
	}
	for _, raw := range channelGroups {
		for _, part := range strings.Split(raw, ",") {
			group := normalizeInviteCommissionGroupName(part)
			if group == "" || shouldFilterInviteCommissionGroup(group) {
				continue
			}
			groups[group] = "normal"
		}
	}
	var aggregateGroups []AggregateGroup
	if err := DB.Model(&AggregateGroup{}).Select("name").Find(&aggregateGroups).Error; err != nil {
		return nil, err
	}
	for _, aggregateGroup := range aggregateGroups {
		group := normalizeInviteCommissionGroupName(aggregateGroup.Name)
		if group == "" || shouldFilterInviteCommissionGroup(group) {
			continue
		}
		groups[group] = "aggregate"
	}
	return groups, nil
}

func inviteCommissionGroupProfitRuleMap(settings InviteCommissionSettings) map[string]InviteCommissionGroupProfitRule {
	ruleMap := make(map[string]InviteCommissionGroupProfitRule, len(settings.GroupProfitRules))
	for _, rule := range settings.GroupProfitRules {
		group := normalizeInviteCommissionGroupName(rule.Group)
		if group != "" {
			ruleMap[group] = rule
		}
	}
	return ruleMap
}

func normalizeInviteCommissionGroupName(group string) string {
	return strings.TrimSpace(strings.Trim(group, ","))
}

func shouldFilterInviteCommissionGroup(group string) bool {
	group = normalizeInviteCommissionGroupName(group)
	return group == "" || group == "default" || strings.HasPrefix(group, "UserGroup-")
}

func AttachInviteCommissionSnapshotToLogOther(other map[string]interface{}, group string, quota int) map[string]interface{} {
	if other == nil {
		other = make(map[string]interface{})
	}
	if quota <= 0 {
		return other
	}
	snapshot := buildInviteCommissionLogCommissionSnapshot(other, group, quota)
	adminInfo := ensureInviteCommissionAdminInfo(other)
	adminInfo["commission"] = snapshot
	other["admin_info"] = adminInfo
	return other
}

func buildInviteCommissionLogCommissionSnapshot(other map[string]interface{}, group string, quota int) InviteCommissionLogCommissionSnapshot {
	adminInfo := inviteCommissionMapMap(other, "admin_info")
	commissionGroup := normalizeInviteCommissionGroupName(group)
	routeGroup := inviteCommissionMapString(adminInfo, "route_group")
	if aggregateGroup := inviteCommissionMapString(adminInfo, "aggregate_group"); aggregateGroup != "" {
		commissionGroup = aggregateGroup
	}
	settings := GetInviteCommissionSettings()
	rule, configured := inviteCommissionGroupProfitRuleMap(settings)[commissionGroup]
	if !configured {
		rule = InviteCommissionGroupProfitRule{
			Group:   commissionGroup,
			Service: "other",
		}
	}
	profitRate := clampInviteCommissionBps(rule.ProfitRateBps)
	grossProfit := quotaByBps(int64(quota), profitRate)
	upstreamCost := int64(quota) - grossProfit
	if upstreamCost < 0 {
		upstreamCost = 0
	}
	return InviteCommissionLogCommissionSnapshot{
		Group:                   commissionGroup,
		RouteGroup:              routeGroup,
		Service:                 normalizeInviteCommissionService(rule.Service),
		Configured:              configured,
		ProfitRateBps:           profitRate,
		CostRateBps:             10000 - profitRate,
		MaxCommissionRateBps:    clampInviteCommissionBps(rule.MaxCommissionRateBps),
		ProfitShareRateBps:      clampInviteCommissionBps(rule.ProfitShareRateBps),
		ProfitProtectionEnabled: rule.ProfitProtectionEnabled && configured,
		RevenueQuota:            int64(quota),
		UpstreamCostQuota:       upstreamCost,
		GrossProfitQuota:        grossProfit,
		Source:                  "snapshot",
	}
}

func ensureInviteCommissionAdminInfo(other map[string]interface{}) map[string]interface{} {
	if adminInfo := inviteCommissionMapMap(other, "admin_info"); adminInfo != nil {
		return adminInfo
	}
	adminInfo := make(map[string]interface{})
	other["admin_info"] = adminInfo
	return adminInfo
}

func quotaByBps(quota int64, bps int) int64 {
	if quota <= 0 || bps <= 0 {
		return 0
	}
	return int64(math.Round(float64(quota) * float64(bps) / 10000))
}

func GetInviteCommissionUserConfig(userID int) (*InviteCommissionUserConfig, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	var config InviteCommissionUserConfig
	if err := DB.First(&config, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func UpsertInviteCommissionUserConfig(config *InviteCommissionUserConfig) (*InviteCommissionUserConfig, error) {
	if config == nil || config.UserID <= 0 {
		return nil, errors.New("invalid user config")
	}
	config.Remark = strings.TrimSpace(config.Remark)
	if config.Level1RateBps < 0 || config.Level2RateBps < 0 {
		return nil, errors.New("返佣比例不能为负数")
	}
	var user User
	if err := DB.Select("id").First(&user, "id = ?", config.UserID).Error; err != nil {
		return nil, err
	}

	var existing InviteCommissionUserConfig
	err := DB.First(&existing, "user_id = ?", config.UserID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := DB.Create(config).Error; err != nil {
			return nil, err
		}
		return config, nil
	}

	existing.Enabled = config.Enabled
	existing.Level1RateBps = config.Level1RateBps
	existing.Level2RateBps = config.Level2RateBps
	existing.Remark = config.Remark
	if err := DB.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func ListInviteCommissionUserConfigs(keyword string, startIdx int, limit int) ([]*InviteCommissionUserConfig, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	query := DB.Model(&InviteCommissionUserConfig{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		var userIDs []int
		userQuery := DB.Model(&User{})
		if id, err := parsePositiveInt(keyword); err == nil {
			userQuery = userQuery.Where("id = ? OR username LIKE ? OR display_name LIKE ?", id, "%"+keyword+"%", "%"+keyword+"%")
		} else {
			userQuery = userQuery.Where("username LIKE ? OR display_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
		if err := userQuery.Pluck("id", &userIDs).Error; err != nil {
			return nil, 0, err
		}
		if len(userIDs) == 0 {
			return []*InviteCommissionUserConfig{}, 0, nil
		}
		query = query.Where("user_id IN ?", userIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var configs []*InviteCommissionUserConfig
	if err := query.Order("id desc").Limit(limit).Offset(startIdx).Find(&configs).Error; err != nil {
		return nil, 0, err
	}
	if err := populateInviteCommissionUserConfigUsers(configs); err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}

func populateInviteCommissionUserConfigUsers(configs []*InviteCommissionUserConfig) error {
	if len(configs) == 0 {
		return nil
	}
	userIDs := make([]int, 0, len(configs))
	for _, config := range configs {
		userIDs = append(userIDs, config.UserID)
	}
	var users []User
	if err := DB.Select("id, username, display_name").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return err
	}
	userMap := make(map[int]User, len(users))
	for _, user := range users {
		userMap[user.Id] = user
	}
	for _, config := range configs {
		if user, ok := userMap[config.UserID]; ok {
			config.Username = user.Username
			config.DisplayName = user.DisplayName
		}
	}
	return nil
}

func BuildInviteCommissionReport(req InviteCommissionReportRequest) (*InviteCommissionReport, error) {
	if req.OwnerUserID <= 0 {
		return nil, errors.New("invalid owner user id")
	}
	if req.EndTimestamp <= 0 || req.StartTimestamp <= 0 || req.EndTimestamp < req.StartTimestamp {
		return nil, errors.New("invalid time range")
	}

	var owner User
	if err := DB.Select("id, username").First(&owner, "id = ?", req.OwnerUserID).Error; err != nil {
		return nil, err
	}

	settings := GetInviteCommissionSettings()
	effective := getInviteCommissionEffectiveConfig(req.OwnerUserID, settings)
	level1ReportRateBps := 0
	level2ReportRateBps := 0
	if effective.Enabled {
		level1ReportRateBps = effective.Level1RateBps
		level2ReportRateBps = effective.Level2RateBps
	}
	invitees, err := loadInviteCommissionInvitees(req.OwnerUserID)
	if err != nil {
		return nil, err
	}

	report := &InviteCommissionReport{
		OwnerUserID:    owner.Id,
		OwnerUsername:  owner.Username,
		StartTimestamp: req.StartTimestamp,
		EndTimestamp:   req.EndTimestamp,
		Settings:       settings,
		Effective:      effective,
		Summary: InviteCommissionSummary{
			InviteeCount: len(invitees),
		},
		Levels: []InviteCommissionLevelSummary{
			{Level: 1, RateBps: level1ReportRateBps},
			{Level: 2, RateBps: level2ReportRateBps},
		},
		Services: buildInitialInviteCommissionServiceStats(settings),
	}

	inviteeStats := make(map[int]*InviteCommissionInviteeStat, len(invitees))
	inviteeIDs := make([]int, 0, len(invitees))
	for _, invitee := range invitees {
		stat := &InviteCommissionInviteeStat{
			UserID:              invitee.UserID,
			Username:            invitee.Username,
			Level:               invitee.Level,
			DirectOwnerUserID:   invitee.DirectOwnerUserID,
			DirectOwnerUsername: "",
		}
		if invitee.Level == 2 {
			if direct, ok := inviteeStats[invitee.DirectOwnerUserID]; ok {
				stat.DirectOwnerUsername = direct.Username
			}
		}
		inviteeStats[invitee.UserID] = stat
		inviteeIDs = append(inviteeIDs, invitee.UserID)
		if invitee.Level == 1 {
			report.Summary.Level1InviteeCount++
			report.Levels[0].InviteeCount++
		} else if invitee.Level == 2 {
			report.Summary.Level2InviteeCount++
			report.Levels[1].InviteeCount++
		}
	}

	if len(inviteeIDs) == 0 {
		if req.IncludeDetails {
			report.Invitees = []InviteCommissionInviteeStat{}
			report.Models = []InviteCommissionModelStat{}
			report.Groups = []InviteCommissionGroupStat{}
		}
		return report, nil
	}

	if err := aggregateInviteCommissionWalletRecharges(report, inviteeStats, inviteeIDs, req); err != nil {
		return nil, err
	}
	if err := aggregateInviteCommissionRedemptions(report, inviteeStats, inviteeIDs, req); err != nil {
		return nil, err
	}
	if err := aggregateInviteCommissionSubscriptionPurchases(report, inviteeStats, inviteeIDs, req); err != nil {
		return nil, err
	}
	if err := aggregateInviteCommissionConsumption(report, inviteeStats, inviteeIDs, req, settings, effective); err != nil {
		return nil, err
	}

	if req.IncludeDetails {
		report.Invitees = inviteCommissionInviteeStatsSorted(inviteeStats)
		report.Models = inviteCommissionModelStatsSorted(report.Models)
		report.Groups = inviteCommissionGroupStatsSorted(report.Groups)
	} else {
		report.Models = nil
		report.Invitees = nil
		report.Groups = nil
	}
	report.Services = inviteCommissionServiceStatsSorted(report.Services)
	return report, nil
}

func StripInviteCommissionProfitDetailsForUser(report *InviteCommissionReport) {
	if report == nil {
		return
	}
	report.Groups = nil
	report.Models = nil
	report.Settings.GroupProfitRules = nil
	report.Summary.UpstreamCostQuota = 0
	report.Summary.GrossProfitQuota = 0
	report.Summary.TheoreticalCommissionQuota = 0
	report.Summary.ProfitCapCommissionQuota = 0
	report.Summary.ProfitProtectionReducedQuota = 0
	report.Summary.MissingProfitSnapshotQuota = 0
	for i := range report.Levels {
		report.Levels[i].UpstreamCostQuota = 0
		report.Levels[i].GrossProfitQuota = 0
		report.Levels[i].TheoreticalCommissionQuota = 0
		report.Levels[i].ProfitCapCommissionQuota = 0
		report.Levels[i].ProfitProtectionReducedQuota = 0
		report.Levels[i].MissingProfitSnapshotQuota = 0
	}
	for i := range report.Services {
		report.Services[i].UpstreamCostQuota = 0
		report.Services[i].GrossProfitQuota = 0
		report.Services[i].TheoreticalCommissionQuota = 0
		report.Services[i].ProfitCapCommissionQuota = 0
		report.Services[i].ProfitProtectionReducedQuota = 0
		report.Services[i].MissingProfitSnapshotQuota = 0
	}
}

func getInviteCommissionEffectiveConfig(ownerUserID int, settings InviteCommissionSettings) InviteCommissionEffectiveConfig {
	effective := InviteCommissionEffectiveConfig{
		Enabled:       false,
		UseUserConfig: false,
		Level1RateBps: settings.DefaultLevel1RateBps,
		Level2RateBps: settings.DefaultLevel2RateBps,
	}
	config, err := GetInviteCommissionUserConfig(ownerUserID)
	if err != nil {
		return effective
	}
	effective.Enabled = config.Enabled
	effective.UseUserConfig = true
	effective.UserConfigID = config.Id
	effective.Level1RateBps = config.Level1RateBps
	effective.Level2RateBps = config.Level2RateBps
	return effective
}

func loadInviteCommissionInvitees(ownerUserID int) ([]inviteCommissionUserBrief, error) {
	var level1 []User
	if err := DB.Select("id, username, invite_code_owner_id").
		Where("invite_code_owner_id = ? AND invite_code_id > 0", ownerUserID).
		Find(&level1).Error; err != nil {
		return nil, err
	}
	invitees := make([]inviteCommissionUserBrief, 0, len(level1))
	level1IDs := make([]int, 0, len(level1))
	for _, user := range level1 {
		invitees = append(invitees, inviteCommissionUserBrief{
			UserID:            user.Id,
			Username:          user.Username,
			Level:             1,
			DirectOwnerUserID: ownerUserID,
		})
		level1IDs = append(level1IDs, user.Id)
	}
	if len(level1IDs) == 0 {
		return invitees, nil
	}
	var level2 []User
	if err := DB.Select("id, username, invite_code_owner_id").
		Where("invite_code_owner_id IN ? AND invite_code_id > 0", level1IDs).
		Find(&level2).Error; err != nil {
		return nil, err
	}
	for _, user := range level2 {
		invitees = append(invitees, inviteCommissionUserBrief{
			UserID:            user.Id,
			Username:          user.Username,
			Level:             2,
			DirectOwnerUserID: user.InviteCodeOwnerId,
		})
	}
	return invitees, nil
}

func aggregateInviteCommissionWalletRecharges(report *InviteCommissionReport, inviteeStats map[int]*InviteCommissionInviteeStat, inviteeIDs []int, req InviteCommissionReportRequest) error {
	subscriptionTradeNos, err := loadInviteCommissionSubscriptionTradeNos(inviteeIDs)
	if err != nil {
		return err
	}
	query := DB.Model(&TopUp{}).
		Where("user_id IN ? AND status = ? AND complete_time >= ? AND complete_time <= ?", inviteeIDs, common.TopUpStatusSuccess, req.StartTimestamp, req.EndTimestamp)
	if len(subscriptionTradeNos) > 0 {
		query = query.Not("trade_no IN ?", subscriptionTradeNos)
	}
	var topUps []TopUp
	if err := query.Find(&topUps).Error; err != nil {
		return err
	}
	for _, topUp := range topUps {
		stat := inviteeStats[topUp.UserId]
		if stat == nil {
			continue
		}
		stat.WalletRechargeAmount += topUp.Amount
		stat.WalletRechargeMoney += topUp.Money
		level := report.levelSummary(stat.Level)
		level.WalletRechargeAmount += topUp.Amount
		level.WalletRechargeMoney += topUp.Money
		report.Summary.WalletRechargeAmount += topUp.Amount
		report.Summary.WalletRechargeMoney += topUp.Money
	}
	return nil
}

func loadInviteCommissionSubscriptionTradeNos(userIDs []int) ([]string, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var tradeNos []string
	err := DB.Model(&SubscriptionOrder{}).
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess).
		Pluck("trade_no", &tradeNos).Error
	return tradeNos, err
}

func aggregateInviteCommissionRedemptions(report *InviteCommissionReport, inviteeStats map[int]*InviteCommissionInviteeStat, inviteeIDs []int, req InviteCommissionReportRequest) error {
	var redemptions []Redemption
	if err := DB.Model(&Redemption{}).
		Where("used_user_id IN ? AND status = ? AND redeemed_time >= ? AND redeemed_time <= ?", inviteeIDs, common.RedemptionCodeStatusUsed, req.StartTimestamp, req.EndTimestamp).
		Find(&redemptions).Error; err != nil {
		return err
	}
	for _, redemption := range redemptions {
		stat := inviteeStats[redemption.UsedUserId]
		if stat == nil {
			continue
		}
		quota := int64(redemption.Quota)
		stat.RedemptionQuota += quota
		level := report.levelSummary(stat.Level)
		level.RedemptionQuota += quota
		report.Summary.RedemptionQuota += quota
	}
	return nil
}

func aggregateInviteCommissionSubscriptionPurchases(report *InviteCommissionReport, inviteeStats map[int]*InviteCommissionInviteeStat, inviteeIDs []int, req InviteCommissionReportRequest) error {
	var orders []SubscriptionOrder
	if err := DB.Model(&SubscriptionOrder{}).
		Where("user_id IN ? AND status = ? AND complete_time >= ? AND complete_time <= ?", inviteeIDs, common.TopUpStatusSuccess, req.StartTimestamp, req.EndTimestamp).
		Find(&orders).Error; err != nil {
		return err
	}
	for _, order := range orders {
		stat := inviteeStats[order.UserId]
		if stat == nil {
			continue
		}
		stat.SubscriptionPurchaseMoney += order.Money
		level := report.levelSummary(stat.Level)
		level.SubscriptionPurchaseMoney += order.Money
		report.Summary.SubscriptionPurchaseMoney += order.Money
	}
	return nil
}

func aggregateInviteCommissionConsumption(report *InviteCommissionReport, inviteeStats map[int]*InviteCommissionInviteeStat, inviteeIDs []int, req InviteCommissionReportRequest, settings InviteCommissionSettings, effective InviteCommissionEffectiveConfig) error {
	var logs []Log
	if err := LOG_DB.Model(&Log{}).
		Where("user_id IN ? AND type = ? AND quota > 0 AND created_at >= ? AND created_at <= ?", inviteeIDs, LogTypeConsume, req.StartTimestamp, req.EndTimestamp).
		Find(&logs).Error; err != nil {
		return err
	}
	subscriptionSourceCache := make(map[int]string)
	modelStats := make(map[string]*InviteCommissionModelStat)
	groupStats := make(map[string]*InviteCommissionGroupStat)
	groupTypes, _ := loadInviteCommissionAvailableGroups()
	for _, logItem := range logs {
		stat := inviteeStats[logItem.UserId]
		if stat == nil {
			continue
		}
		other := parseInviteCommissionLogOther(logItem.Other)
		modelName := strings.TrimSpace(logItem.ModelName)
		if strings.TrimSpace(other.UpstreamModelName) != "" {
			modelName = strings.TrimSpace(other.UpstreamModelName)
		}
		service := "other"
		if other.HasCommissionSnapshot && other.CommissionSnapshot.Configured && other.CommissionSnapshot.Service != "" {
			service = other.CommissionSnapshot.Service
		}
		serviceStat := report.serviceStat(service, settings)
		modelStat := getInviteCommissionModelStat(modelStats, modelName, service, serviceStat.Label)
		groupStat := getInviteCommissionGroupStat(groupStats, other, logItem.Group, service, serviceStat.Label, groupTypes)
		quota := int64(logItem.Quota)
		levelRate := effective.levelRate(stat.Level)
		if !effective.Enabled {
			levelRate = 0
		}
		commissionEnabled := effective.Enabled && levelRate > 0

		serviceStat.RequestCount++
		serviceStat.PromptTokens += int64(logItem.PromptTokens)
		serviceStat.CompletionTokens += int64(logItem.CompletionTokens)
		modelStat.RequestCount++
		modelStat.PromptTokens += int64(logItem.PromptTokens)
		modelStat.CompletionTokens += int64(logItem.CompletionTokens)
		groupStat.RequestCount++

		if other.BillingSource == "subscription" {
			source := getInviteCommissionSubscriptionSource(other.SubscriptionID, subscriptionSourceCache)
			tierRate := subscriptionTierRateBps(other, settings)
			metrics := calculateInviteCommissionProfitMetrics(quota, tierRate, levelRate, other, commissionEnabled)
			if source != "order" {
				metrics.zeroCommission()
				stat.AdminSubscriptionConsumption += quota
				report.Summary.AdminSubscriptionConsumption += quota
				report.levelSummary(stat.Level).AdminSubscriptionConsumption += quota
				serviceStat.AdminSubscriptionConsumption += quota
				modelStat.AdminSubscriptionConsumption += quota
			} else {
				stat.OrderSubscriptionConsumption += quota
				report.Summary.OrderSubscriptionConsumption += quota
				report.Summary.OrderSubscriptionCommission += metrics.EstimatedCommission
				report.levelSummary(stat.Level).OrderSubscriptionConsumption += quota
				serviceStat.OrderSubscriptionConsumption += quota
				modelStat.OrderSubscriptionConsumption += quota
			}
			stat.SubscriptionConsumptionQuota += quota
			stat.SubscriptionCommissionQuota += metrics.EstimatedCommission
			stat.EstimatedCommissionQuota += metrics.EstimatedCommission
			report.Summary.SubscriptionConsumptionQuota += quota
			report.Summary.SubscriptionCommissionQuota += metrics.EstimatedCommission
			report.Summary.EstimatedCommissionQuota += metrics.EstimatedCommission
			level := report.levelSummary(stat.Level)
			level.SubscriptionConsumptionQuota += quota
			level.SubscriptionCommissionQuota += metrics.EstimatedCommission
			level.EstimatedCommissionQuota += metrics.EstimatedCommission
			serviceStat.SubscriptionConsumptionQuota += quota
			serviceStat.SubscriptionCommissionQuota += metrics.EstimatedCommission
			serviceStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			modelStat.SubscriptionConsumptionQuota += quota
			modelStat.SubscriptionCommissionQuota += metrics.EstimatedCommission
			modelStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			groupStat.SubscriptionConsumptionQuota += quota
			groupStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			applyInviteCommissionProfitMetrics(report, stat, level, serviceStat, modelStat, groupStat, metrics)
		} else {
			theoreticalRate := 0
			if other.HasCommissionSnapshot {
				theoreticalRate = other.CommissionSnapshot.MaxCommissionRateBps
			}
			metrics := calculateInviteCommissionProfitMetrics(quota, theoreticalRate, levelRate, other, commissionEnabled)
			stat.WalletConsumptionQuota += quota
			stat.WalletCommissionQuota += metrics.EstimatedCommission
			stat.EstimatedCommissionQuota += metrics.EstimatedCommission
			report.Summary.WalletConsumptionQuota += quota
			report.Summary.WalletCommissionQuota += metrics.EstimatedCommission
			report.Summary.EstimatedCommissionQuota += metrics.EstimatedCommission
			level := report.levelSummary(stat.Level)
			level.WalletConsumptionQuota += quota
			level.WalletCommissionQuota += metrics.EstimatedCommission
			level.EstimatedCommissionQuota += metrics.EstimatedCommission
			serviceStat.WalletConsumptionQuota += quota
			serviceStat.WalletCommissionQuota += metrics.EstimatedCommission
			serviceStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			modelStat.WalletConsumptionQuota += quota
			modelStat.WalletCommissionQuota += metrics.EstimatedCommission
			modelStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			groupStat.WalletConsumptionQuota += quota
			groupStat.EstimatedCommissionQuota += metrics.EstimatedCommission
			applyInviteCommissionProfitMetrics(report, stat, level, serviceStat, modelStat, groupStat, metrics)
		}

		stat.TotalConsumptionQuota += quota
		report.Summary.TotalConsumptionQuota += quota
		report.levelSummary(stat.Level).TotalConsumptionQuota += quota
		serviceStat.TotalConsumptionQuota += quota
		modelStat.TotalConsumptionQuota += quota
		groupStat.TotalConsumptionQuota += quota
	}
	report.Models = make([]InviteCommissionModelStat, 0, len(modelStats))
	for _, stat := range modelStats {
		report.Models = append(report.Models, *stat)
	}
	report.Groups = make([]InviteCommissionGroupStat, 0, len(groupStats))
	for _, stat := range groupStats {
		report.Groups = append(report.Groups, *stat)
	}
	return nil
}

func parseInviteCommissionLogOther(raw string) inviteCommissionLogOther {
	var other inviteCommissionLogOther
	if strings.TrimSpace(raw) == "" {
		return other
	}
	_ = common.UnmarshalJsonStr(raw, &other)
	var generic map[string]interface{}
	if err := common.UnmarshalJsonStr(raw, &generic); err == nil {
		if other.BillingSource == "" {
			other.BillingSource = inviteCommissionMapString(generic, "billing_source")
		}
		if other.UpstreamModelName == "" {
			other.UpstreamModelName = inviteCommissionMapString(generic, "upstream_model_name")
		}
		if other.SubscriptionID == 0 {
			other.SubscriptionID = int(inviteCommissionMapInt64(generic, "subscription_id"))
		}
		if other.SubscriptionTotal == 0 {
			other.SubscriptionTotal = inviteCommissionMapInt64(generic, "subscription_total")
		}
		if other.SubscriptionUsed == 0 {
			other.SubscriptionUsed = inviteCommissionMapInt64(generic, "subscription_used")
		}
		if other.SubscriptionConsumed == 0 {
			other.SubscriptionConsumed = inviteCommissionMapInt64(generic, "subscription_consumed")
		}
		if other.SubscriptionPreConsume == 0 {
			other.SubscriptionPreConsume = inviteCommissionMapInt64(generic, "subscription_pre_consumed")
		}
		if other.SubscriptionPostDelta == 0 {
			other.SubscriptionPostDelta = inviteCommissionMapInt64(generic, "subscription_post_delta")
		}
		if snapshot, ok := parseInviteCommissionSnapshot(generic); ok {
			other.CommissionSnapshot = snapshot
			other.HasCommissionSnapshot = true
		}
	}
	return other
}

type inviteCommissionProfitMetrics struct {
	HasSnapshot                 bool
	ConfiguredSnapshot          bool
	UpstreamCostQuota           int64
	GrossProfitQuota            int64
	TheoreticalCommission       float64
	ProfitCapCommission         float64
	EstimatedCommission         float64
	ProfitProtectionReduced     float64
	MissingProfitSnapshotQuota  int64
	ConfiguredProfitSnapshotHit int64
	MissingProfitSnapshotHit    int64
}

func (m *inviteCommissionProfitMetrics) zeroCommission() {
	m.TheoreticalCommission = 0
	m.ProfitCapCommission = 0
	m.EstimatedCommission = 0
	m.ProfitProtectionReduced = 0
}

func calculateInviteCommissionProfitMetrics(quota int64, theoreticalRateBps int, levelRateBps int, other inviteCommissionLogOther, enabled bool) inviteCommissionProfitMetrics {
	metrics := inviteCommissionProfitMetrics{}
	if quota <= 0 {
		return metrics
	}
	if !other.HasCommissionSnapshot {
		metrics.MissingProfitSnapshotQuota = quota
		metrics.MissingProfitSnapshotHit = 1
		return metrics
	}
	snapshot := other.CommissionSnapshot
	metrics.HasSnapshot = true
	metrics.ConfiguredSnapshot = snapshot.Configured
	metrics.UpstreamCostQuota = snapshot.UpstreamCostQuota
	metrics.GrossProfitQuota = snapshot.GrossProfitQuota
	if snapshot.Configured {
		metrics.ConfiguredProfitSnapshotHit = 1
	}
	if !enabled || !snapshot.Configured || theoreticalRateBps <= 0 || levelRateBps <= 0 {
		return metrics
	}
	metrics.TheoreticalCommission = commissionByBps(float64(quota), theoreticalRateBps, levelRateBps)
	if snapshot.ProfitProtectionEnabled {
		metrics.ProfitCapCommission = commissionByBps(float64(snapshot.GrossProfitQuota), snapshot.ProfitShareRateBps, levelRateBps)
		if metrics.ProfitCapCommission < metrics.TheoreticalCommission {
			metrics.EstimatedCommission = metrics.ProfitCapCommission
			metrics.ProfitProtectionReduced = roundInviteCommissionFloat(metrics.TheoreticalCommission - metrics.EstimatedCommission)
			return metrics
		}
	}
	metrics.ProfitCapCommission = metrics.TheoreticalCommission
	metrics.EstimatedCommission = metrics.TheoreticalCommission
	return metrics
}

func applyInviteCommissionProfitMetrics(report *InviteCommissionReport, invitee *InviteCommissionInviteeStat, level *InviteCommissionLevelSummary, service *InviteCommissionServiceStat, modelStat *InviteCommissionModelStat, group *InviteCommissionGroupStat, metrics inviteCommissionProfitMetrics) {
	report.Summary.UpstreamCostQuota += metrics.UpstreamCostQuota
	report.Summary.GrossProfitQuota += metrics.GrossProfitQuota
	report.Summary.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	report.Summary.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	report.Summary.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	report.Summary.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota

	invitee.UpstreamCostQuota += metrics.UpstreamCostQuota
	invitee.GrossProfitQuota += metrics.GrossProfitQuota
	invitee.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	invitee.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	invitee.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	invitee.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota

	level.UpstreamCostQuota += metrics.UpstreamCostQuota
	level.GrossProfitQuota += metrics.GrossProfitQuota
	level.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	level.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	level.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	level.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota

	service.UpstreamCostQuota += metrics.UpstreamCostQuota
	service.GrossProfitQuota += metrics.GrossProfitQuota
	service.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	service.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	service.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	service.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota

	modelStat.UpstreamCostQuota += metrics.UpstreamCostQuota
	modelStat.GrossProfitQuota += metrics.GrossProfitQuota
	modelStat.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	modelStat.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	modelStat.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	modelStat.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota

	group.UpstreamCostQuota += metrics.UpstreamCostQuota
	group.GrossProfitQuota += metrics.GrossProfitQuota
	group.TheoreticalCommissionQuota += metrics.TheoreticalCommission
	group.ProfitCapCommissionQuota += metrics.ProfitCapCommission
	group.ProfitProtectionReducedQuota += metrics.ProfitProtectionReduced
	group.MissingProfitSnapshotQuota += metrics.MissingProfitSnapshotQuota
	group.ConfiguredProfitSnapshotCount += metrics.ConfiguredProfitSnapshotHit
	group.MissingProfitSnapshotCount += metrics.MissingProfitSnapshotHit
}

func inviteCommissionMapString(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
	return ""
}

func inviteCommissionMapInt64(data map[string]interface{}, key string) int64 {
	value, ok := data[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsed, _ := parsePositiveInt(v)
		return int64(parsed)
	default:
		return 0
	}
}

func inviteCommissionMapBool(data map[string]interface{}, key string) bool {
	value, ok := data[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	default:
		return false
	}
}

func inviteCommissionMapMap(data map[string]interface{}, key string) map[string]interface{} {
	if data == nil {
		return nil
	}
	value, ok := data[key]
	if !ok {
		return nil
	}
	if result, ok := value.(map[string]interface{}); ok {
		return result
	}
	return nil
}

func parseInviteCommissionSnapshot(data map[string]interface{}) (InviteCommissionLogCommissionSnapshot, bool) {
	adminInfo := inviteCommissionMapMap(data, "admin_info")
	commission := inviteCommissionMapMap(adminInfo, "commission")
	if len(commission) == 0 {
		return InviteCommissionLogCommissionSnapshot{}, false
	}
	snapshot := InviteCommissionLogCommissionSnapshot{
		Group:                   inviteCommissionMapString(commission, "group"),
		RouteGroup:              inviteCommissionMapString(commission, "route_group"),
		Service:                 normalizeInviteCommissionService(inviteCommissionMapString(commission, "service")),
		Configured:              inviteCommissionMapBool(commission, "configured"),
		ProfitRateBps:           int(inviteCommissionMapInt64(commission, "profit_rate_bps")),
		CostRateBps:             int(inviteCommissionMapInt64(commission, "cost_rate_bps")),
		MaxCommissionRateBps:    int(inviteCommissionMapInt64(commission, "max_commission_rate_bps")),
		ProfitShareRateBps:      int(inviteCommissionMapInt64(commission, "profit_share_rate_bps")),
		ProfitProtectionEnabled: inviteCommissionMapBool(commission, "profit_protection_enabled"),
		RevenueQuota:            inviteCommissionMapInt64(commission, "revenue_quota"),
		UpstreamCostQuota:       inviteCommissionMapInt64(commission, "upstream_cost_quota"),
		GrossProfitQuota:        inviteCommissionMapInt64(commission, "gross_profit_quota"),
		Source:                  inviteCommissionMapString(commission, "source"),
	}
	if snapshot.Service == "" {
		snapshot.Service = "other"
	}
	return snapshot, true
}

func getInviteCommissionSubscriptionSource(subscriptionID int, cache map[int]string) string {
	if subscriptionID <= 0 {
		return ""
	}
	if source, ok := cache[subscriptionID]; ok {
		return source
	}
	var sub UserSubscription
	if err := DB.Select("id, source").First(&sub, "id = ?", subscriptionID).Error; err != nil {
		cache[subscriptionID] = ""
		return ""
	}
	source := strings.TrimSpace(sub.Source)
	cache[subscriptionID] = source
	return source
}

func subscriptionTierRateBps(other inviteCommissionLogOther, settings InviteCommissionSettings) int {
	if other.SubscriptionTotal <= 0 {
		return firstInviteCommissionSubscriptionTierRate(settings)
	}
	used := other.SubscriptionUsed
	if used <= 0 {
		used = other.SubscriptionPreConsume + other.SubscriptionPostDelta
	}
	if used < 0 {
		used = 0
	}
	percent := float64(used) / float64(other.SubscriptionTotal) * 100
	for _, tier := range settings.SubscriptionTiers {
		if percent >= tier.StartPercent && percent < tier.EndPercent {
			return tier.RateBps
		}
	}
	if percent >= 100 && len(settings.SubscriptionTiers) > 0 {
		return settings.SubscriptionTiers[len(settings.SubscriptionTiers)-1].RateBps
	}
	return 0
}

func firstInviteCommissionSubscriptionTierRate(settings InviteCommissionSettings) int {
	if len(settings.SubscriptionTiers) == 0 {
		return 0
	}
	return settings.SubscriptionTiers[0].RateBps
}

func normalizeInviteCommissionService(service string) string {
	return strings.TrimSpace(strings.ToLower(service))
}

func buildInitialInviteCommissionServiceStats(settings InviteCommissionSettings) []InviteCommissionServiceStat {
	stats := make([]InviteCommissionServiceStat, 0, len(settings.ServiceCategories))
	seen := make(map[string]bool)
	for _, category := range settings.ServiceCategories {
		service := normalizeInviteCommissionService(category.Service)
		if service == "" || seen[service] {
			continue
		}
		seen[service] = true
		stats = append(stats, InviteCommissionServiceStat{
			Service: service,
			Label:   inviteCommissionServiceLabel(service, settings),
		})
	}
	if !seen["other"] {
		stats = append(stats, InviteCommissionServiceStat{
			Service: "other",
			Label:   inviteCommissionServiceLabel("other", settings),
		})
	}
	return stats
}

func (r *InviteCommissionReport) levelSummary(level int) *InviteCommissionLevelSummary {
	for i := range r.Levels {
		if r.Levels[i].Level == level {
			return &r.Levels[i]
		}
	}
	r.Levels = append(r.Levels, InviteCommissionLevelSummary{Level: level})
	return &r.Levels[len(r.Levels)-1]
}

func (r *InviteCommissionReport) serviceStat(service string, settings InviteCommissionSettings) *InviteCommissionServiceStat {
	service = normalizeInviteCommissionService(service)
	if service == "" {
		service = "other"
	}
	for i := range r.Services {
		if r.Services[i].Service == service {
			return &r.Services[i]
		}
	}
	r.Services = append(r.Services, InviteCommissionServiceStat{
		Service: service,
		Label:   inviteCommissionServiceLabel(service, settings),
	})
	return &r.Services[len(r.Services)-1]
}

func inviteCommissionServiceLabel(service string, settings InviteCommissionSettings) string {
	service = normalizeInviteCommissionService(service)
	for _, category := range settings.ServiceCategories {
		if normalizeInviteCommissionService(category.Service) == service && strings.TrimSpace(category.Label) != "" {
			return strings.TrimSpace(category.Label)
		}
	}
	return inviteCommissionDefaultServiceLabel(service)
}

func inviteCommissionDefaultServiceLabel(service string) string {
	service = normalizeInviteCommissionService(service)
	switch service {
	case "gpt":
		return "GPT"
	case "claude":
		return "Claude"
	case "gemini":
		return "Gemini"
	case "other":
		return "Other"
	case "":
		return "Other"
	default:
		return strings.ToUpper(service)
	}
}

func getInviteCommissionModelStat(stats map[string]*InviteCommissionModelStat, modelName string, service string, serviceLabel string) *InviteCommissionModelStat {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = "unknown"
	}
	key := service + "\x00" + modelName
	if stat, ok := stats[key]; ok {
		return stat
	}
	stat := &InviteCommissionModelStat{
		ModelName:    modelName,
		Service:      service,
		ServiceLabel: serviceLabel,
	}
	stats[key] = stat
	return stat
}

func getInviteCommissionGroupStat(stats map[string]*InviteCommissionGroupStat, other inviteCommissionLogOther, logGroup string, service string, serviceLabel string, groupTypes map[string]string) *InviteCommissionGroupStat {
	group := normalizeInviteCommissionGroupName(logGroup)
	if other.HasCommissionSnapshot && other.CommissionSnapshot.Group != "" {
		group = normalizeInviteCommissionGroupName(other.CommissionSnapshot.Group)
	}
	if group == "" {
		group = "unknown"
	}
	groupType := "unknown"
	if value, ok := groupTypes[group]; ok {
		groupType = value
	}
	if stat, ok := stats[group]; ok {
		return stat
	}
	stat := &InviteCommissionGroupStat{
		Group:        group,
		Type:         groupType,
		Service:      service,
		ServiceLabel: serviceLabel,
	}
	stats[group] = stat
	return stat
}

func (e InviteCommissionEffectiveConfig) levelRate(level int) int {
	if level == 1 {
		return e.Level1RateBps
	}
	if level == 2 {
		return e.Level2RateBps
	}
	return 0
}

func commissionByBps(quota float64, rateBps int, levelBps int) float64 {
	if quota <= 0 || rateBps <= 0 || levelBps <= 0 {
		return 0
	}
	value := quota * float64(rateBps) / 10000 * float64(levelBps) / 10000
	return math.Round(value*10000) / 10000
}

func roundInviteCommissionFloat(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func inviteCommissionInviteeStatsSorted(stats map[int]*InviteCommissionInviteeStat) []InviteCommissionInviteeStat {
	result := make([]InviteCommissionInviteeStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, *stat)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Level != result[j].Level {
			return result[i].Level < result[j].Level
		}
		return result[i].UserID > result[j].UserID
	})
	return result
}

func inviteCommissionServiceStatsSorted(stats []InviteCommissionServiceStat) []InviteCommissionServiceStat {
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Service == "other" {
			return false
		}
		if stats[j].Service == "other" {
			return true
		}
		return stats[i].Service < stats[j].Service
	})
	return stats
}

func inviteCommissionModelStatsSorted(stats []InviteCommissionModelStat) []InviteCommissionModelStat {
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].TotalConsumptionQuota != stats[j].TotalConsumptionQuota {
			return stats[i].TotalConsumptionQuota > stats[j].TotalConsumptionQuota
		}
		return stats[i].ModelName < stats[j].ModelName
	})
	return stats
}

func inviteCommissionGroupStatsSorted(stats []InviteCommissionGroupStat) []InviteCommissionGroupStat {
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].TotalConsumptionQuota != stats[j].TotalConsumptionQuota {
			return stats[i].TotalConsumptionQuota > stats[j].TotalConsumptionQuota
		}
		return stats[i].Group < stats[j].Group
	})
	return stats
}

func parsePositiveInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty int")
	}
	var result int
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid int")
		}
		result = result*10 + int(ch-'0')
	}
	return result, nil
}
