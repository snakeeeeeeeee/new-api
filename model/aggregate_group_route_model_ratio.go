package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AggregateGroupRouteModelRatio overrides the final group ratio after an
// aggregate route has selected a concrete child group for an exact model.
type AggregateGroupRouteModelRatio struct {
	Id               int     `json:"id"`
	AggregateGroupId int     `json:"aggregate_group_id" gorm:"not null;index;uniqueIndex:uk_aggregate_route_model_ratio,priority:1"`
	RealGroup        string  `json:"real_group" gorm:"type:varchar(64);not null;index"`
	ModelName        string  `json:"model_name" gorm:"type:varchar(255);not null"`
	GroupRatio       float64 `json:"group_ratio" gorm:"not null"`
	Enabled          bool    `json:"enabled" gorm:"not null;index"`
	RuleKey          string  `json:"-" gorm:"type:char(64);not null;uniqueIndex:uk_aggregate_route_model_ratio,priority:2"`
}

func AggregateGroupRouteModelRatioRuleKey(realGroup string, modelName string) string {
	realGroup = strings.TrimSpace(realGroup)
	modelName = strings.TrimSpace(modelName)
	canonical := fmt.Sprintf("%d:%s%d:%s", len(realGroup), realGroup, len(modelName), modelName)
	return fmt.Sprintf("%x", common.Sha256Raw([]byte(canonical)))
}

func (r *AggregateGroupRouteModelRatio) normalize() {
	if r == nil {
		return
	}
	r.RealGroup = strings.TrimSpace(r.RealGroup)
	r.ModelName = strings.TrimSpace(r.ModelName)
	r.RuleKey = AggregateGroupRouteModelRatioRuleKey(r.RealGroup, r.ModelName)
}

func (r *AggregateGroupRouteModelRatio) BeforeCreate(_ *gorm.DB) error {
	r.normalize()
	return nil
}

func (r *AggregateGroupRouteModelRatio) BeforeSave(_ *gorm.DB) error {
	r.normalize()
	return nil
}

func prepareAggregateGroupRouteModelRatios(aggregateGroupId int, rules []AggregateGroupRouteModelRatio) []AggregateGroupRouteModelRatio {
	prepared := make([]AggregateGroupRouteModelRatio, 0, len(rules))
	for _, rule := range rules {
		rule.Id = 0
		rule.AggregateGroupId = aggregateGroupId
		rule.normalize()
		prepared = append(prepared, rule)
	}
	return prepared
}

func GetAggregateGroupRouteModelRatios(aggregateGroupId int) ([]AggregateGroupRouteModelRatio, error) {
	rules := make([]AggregateGroupRouteModelRatio, 0)
	err := DB.Where("aggregate_group_id = ?", aggregateGroupId).
		Order("real_group ASC, model_name ASC").
		Find(&rules).Error
	return rules, err
}

func GetAggregateGroupRouteModelRatiosByGroupIDs(aggregateGroupIds []int) (map[int][]AggregateGroupRouteModelRatio, error) {
	result := make(map[int][]AggregateGroupRouteModelRatio, len(aggregateGroupIds))
	if len(aggregateGroupIds) == 0 {
		return result, nil
	}
	rules := make([]AggregateGroupRouteModelRatio, 0)
	if err := DB.Where("aggregate_group_id IN ?", aggregateGroupIds).
		Order("aggregate_group_id ASC, real_group ASC, model_name ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, rule := range rules {
		result[rule.AggregateGroupId] = append(result[rule.AggregateGroupId], rule)
	}
	return result, nil
}

func GetEnabledAggregateGroupRouteModelRatio(aggregateGroupId int, realGroup string, modelName string) (*AggregateGroupRouteModelRatio, bool, error) {
	if aggregateGroupId == 0 || strings.TrimSpace(realGroup) == "" || strings.TrimSpace(modelName) == "" {
		return nil, false, nil
	}
	var rule AggregateGroupRouteModelRatio
	tx := DB.Where(
		"aggregate_group_id = ? AND rule_key = ? AND enabled = ?",
		aggregateGroupId,
		AggregateGroupRouteModelRatioRuleKey(realGroup, modelName),
		true,
	).Limit(1).Find(&rule)
	if tx.Error != nil {
		return nil, false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, false, nil
	}
	return &rule, true, nil
}
