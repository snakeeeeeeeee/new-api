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
)

type AggregateGroup struct {
	Id                      int                    `json:"id"`
	Name                    string                 `json:"name" gorm:"size:64;not null;uniqueIndex:uk_aggregate_group_name,where:deleted_at IS NULL"`
	DisplayName             string                 `json:"display_name" gorm:"size:128;not null"`
	Description             string                 `json:"description,omitempty" gorm:"type:varchar(255)"`
	Status                  int                    `json:"status" gorm:"default:1;index"`
	GroupRatio              float64                `json:"group_ratio" gorm:"default:1"`
	SmartRoutingEnabled     bool                   `json:"smart_routing_enabled" gorm:"default:false"`
	RecoveryEnabled         bool                   `json:"recovery_enabled" gorm:"default:true"`
	RecoveryIntervalSeconds int                    `json:"recovery_interval_seconds" gorm:"default:300"`
	RetryStatusCodes        string                 `json:"retry_status_codes" gorm:"type:text"`
	VisibleUserGroups       string                 `json:"-" gorm:"type:text"`
	CreatedTime             int64                  `json:"created_time" gorm:"bigint"`
	UpdatedTime             int64                  `json:"updated_time" gorm:"bigint"`
	DeletedAt               gorm.DeletedAt         `json:"-" gorm:"index"`
	Targets                 []AggregateGroupTarget `json:"targets,omitempty" gorm:"foreignKey:AggregateGroupId"`
}

type AggregateGroupTarget struct {
	Id               int    `json:"id"`
	AggregateGroupId int    `json:"aggregate_group_id" gorm:"index;uniqueIndex:uk_aggregate_group_target"`
	RealGroup        string `json:"real_group" gorm:"size:64;not null;uniqueIndex:uk_aggregate_group_target"`
	OrderIndex       int    `json:"order_index" gorm:"index"`
}

func (g *AggregateGroup) IsEnabled() bool {
	return g != nil && g.Status == AggregateGroupStatusEnabled
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
			"name":                      g.Name,
			"display_name":              g.DisplayName,
			"description":               g.Description,
			"status":                    g.Status,
			"group_ratio":               g.GroupRatio,
			"smart_routing_enabled":     g.SmartRoutingEnabled,
			"recovery_enabled":          g.RecoveryEnabled,
			"recovery_interval_seconds": g.RecoveryIntervalSeconds,
			"retry_status_codes":        g.RetryStatusCodes,
			"visible_user_groups":       g.VisibleUserGroups,
			"updated_time":              g.UpdatedTime,
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
