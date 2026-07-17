package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AggregateGroupCategoryOtherID       = 0
	AggregateGroupCategoryNameMaxLength = 32
)

type AggregateGroupCategory struct {
	Id             int    `json:"id"`
	Name           string `json:"name" gorm:"type:varchar(64);not null"`
	NormalizedName string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	OrderIndex     int    `json:"order_index" gorm:"not null;default:0;index"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`
}

type AggregateGroupCategoryCount struct {
	CategoryId int   `json:"category_id"`
	Count      int64 `json:"count"`
}

func normalizeAggregateGroupCategoryName(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", errors.New("业务分类名称不能为空")
	}
	if utf8.RuneCountInString(name) > AggregateGroupCategoryNameMaxLength {
		return "", "", fmt.Errorf("业务分类名称不能超过 %d 个字符", AggregateGroupCategoryNameMaxLength)
	}
	normalized := strings.ToLower(name)
	if normalized == "other" || normalized == "其他" {
		return "", "", errors.New("业务分类名称不能使用系统保留名称“其他”")
	}
	return name, normalized, nil
}

func GetAllAggregateGroupCategories() ([]AggregateGroupCategory, error) {
	categories := make([]AggregateGroupCategory, 0)
	err := DB.Order("order_index ASC, id ASC").Find(&categories).Error
	return categories, err
}

func GetAggregateGroupCategoriesByID() (map[int]AggregateGroupCategory, error) {
	categories, err := GetAllAggregateGroupCategories()
	if err != nil {
		return nil, err
	}
	result := make(map[int]AggregateGroupCategory, len(categories))
	for _, category := range categories {
		result[category.Id] = category
	}
	return result, nil
}

func GetAggregateGroupCategoryByID(id int) (*AggregateGroupCategory, error) {
	if id <= AggregateGroupCategoryOtherID {
		return nil, gorm.ErrRecordNotFound
	}
	var category AggregateGroupCategory
	tx := DB.Where("id = ?", id).Limit(1).Find(&category)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &category, nil
}

func ValidateAggregateGroupCategoryID(id int) error {
	if id == AggregateGroupCategoryOtherID {
		return nil
	}
	if id < AggregateGroupCategoryOtherID {
		return errors.New("业务分类无效")
	}
	if _, err := GetAggregateGroupCategoryByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("业务分类不存在")
		}
		return err
	}
	return nil
}

func InsertAggregateGroupCategory(category *AggregateGroupCategory) error {
	if category == nil {
		return errors.New("业务分类不能为空")
	}
	name, normalized, err := normalizeAggregateGroupCategoryName(category.Name)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	category.Name = name
	category.NormalizedName = normalized
	category.CreatedTime = now
	category.UpdatedTime = now
	return DB.Transaction(func(tx *gorm.DB) error {
		var duplicateCount int64
		if err := tx.Model(&AggregateGroupCategory{}).
			Where("normalized_name = ?", normalized).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return errors.New("业务分类名称已存在")
		}
		var last AggregateGroupCategory
		result := tx.Order("order_index DESC, id DESC").Limit(1).Find(&last)
		if result.Error != nil {
			return result.Error
		}
		category.OrderIndex = 0
		if result.RowsAffected > 0 {
			category.OrderIndex = last.OrderIndex + 1
		}
		return tx.Create(category).Error
	})
}

func UpdateAggregateGroupCategoryName(id int, name string) error {
	if id <= AggregateGroupCategoryOtherID {
		return errors.New("业务分类不存在")
	}
	name, normalized, err := normalizeAggregateGroupCategoryName(name)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var existingCount int64
		if err := tx.Model(&AggregateGroupCategory{}).Where("id = ?", id).Count(&existingCount).Error; err != nil {
			return err
		}
		if existingCount == 0 {
			return errors.New("业务分类不存在")
		}
		var duplicateCount int64
		if err := tx.Model(&AggregateGroupCategory{}).
			Where("normalized_name = ? AND id <> ?", normalized, id).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return errors.New("业务分类名称已存在")
		}
		return tx.Model(&AggregateGroupCategory{}).Where("id = ?", id).Updates(map[string]interface{}{
			"name":            name,
			"normalized_name": normalized,
			"updated_time":    common.GetTimestamp(),
		}).Error
	})
}

func ReorderAggregateGroupCategories(categoryIDs []int) error {
	if len(categoryIDs) == 0 {
		var count int64
		if err := DB.Model(&AggregateGroupCategory{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		return errors.New("业务分类排序列表不完整")
	}
	seen := make(map[int]struct{}, len(categoryIDs))
	for _, id := range categoryIDs {
		if id <= AggregateGroupCategoryOtherID {
			return errors.New("业务分类排序包含无效分类")
		}
		if _, exists := seen[id]; exists {
			return errors.New("业务分类排序包含重复分类")
		}
		seen[id] = struct{}{}
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var categories []AggregateGroupCategory
		if err := tx.Select("id").Find(&categories).Error; err != nil {
			return err
		}
		if len(categories) != len(categoryIDs) {
			return errors.New("业务分类排序列表不完整")
		}
		for _, category := range categories {
			if _, exists := seen[category.Id]; !exists {
				return errors.New("业务分类排序列表不完整")
			}
		}
		now := common.GetTimestamp()
		for orderIndex, id := range categoryIDs {
			if err := tx.Model(&AggregateGroupCategory{}).Where("id = ?", id).Updates(map[string]interface{}{
				"order_index":  orderIndex,
				"updated_time": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteAggregateGroupCategory(id int) error {
	if id <= AggregateGroupCategoryOtherID {
		return errors.New("系统分类“其他”不能删除")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&AggregateGroupCategory{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("业务分类不存在")
		}
		now := common.GetTimestamp()
		if err := tx.Model(&AggregateGroup{}).Where("category_id = ?", id).Updates(map[string]interface{}{
			"category_id":  AggregateGroupCategoryOtherID,
			"updated_time": now,
		}).Error; err != nil {
			return err
		}
		return tx.Delete(&AggregateGroupCategory{}, id).Error
	})
}

func AssignAggregateGroupsToCategory(aggregateGroupIDs []int, categoryID int) error {
	if len(aggregateGroupIDs) == 0 {
		return errors.New("请选择至少一个聚合分组")
	}
	seen := make(map[int]struct{}, len(aggregateGroupIDs))
	normalizedIDs := make([]int, 0, len(aggregateGroupIDs))
	for _, id := range aggregateGroupIDs {
		if id <= 0 {
			return errors.New("聚合分组 ID 无效")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalizedIDs = append(normalizedIDs, id)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if categoryID != AggregateGroupCategoryOtherID {
			var categoryCount int64
			if err := tx.Model(&AggregateGroupCategory{}).Where("id = ?", categoryID).Count(&categoryCount).Error; err != nil {
				return err
			}
			if categoryCount == 0 {
				return errors.New("业务分类不存在")
			}
		}
		var groupCount int64
		if err := tx.Model(&AggregateGroup{}).Where("id IN ?", normalizedIDs).Count(&groupCount).Error; err != nil {
			return err
		}
		if groupCount != int64(len(normalizedIDs)) {
			return errors.New("部分聚合分组不存在")
		}
		return tx.Model(&AggregateGroup{}).Where("id IN ?", normalizedIDs).Updates(map[string]interface{}{
			"category_id":  categoryID,
			"updated_time": common.GetTimestamp(),
		}).Error
	})
}

func GetAggregateGroupCategoryCounts() (map[int]int64, error) {
	rows := make([]AggregateGroupCategoryCount, 0)
	if err := DB.Model(&AggregateGroup{}).
		Select("category_id, COUNT(*) AS count").
		Group("category_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int64, len(rows))
	for _, row := range rows {
		counts[row.CategoryId] = row.Count
	}
	return counts, nil
}
