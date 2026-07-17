package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type aggregateGroupCategoryNameRequest struct {
	Name string `json:"name"`
}

type aggregateGroupCategoryOrderRequest struct {
	CategoryIds []int `json:"category_ids"`
}

type aggregateGroupCategoryAssignRequest struct {
	AggregateGroupIds []int `json:"aggregate_group_ids"`
	CategoryId        int   `json:"category_id"`
}

type aggregateGroupCategoryResponse struct {
	Id                  int    `json:"id"`
	Name                string `json:"name"`
	OrderIndex          int    `json:"order_index"`
	AggregateGroupCount int64  `json:"aggregate_group_count"`
}

func buildAggregateGroupCategoryResponses() ([]aggregateGroupCategoryResponse, error) {
	categories, err := model.GetAllAggregateGroupCategories()
	if err != nil {
		return nil, err
	}
	counts, err := model.GetAggregateGroupCategoryCounts()
	if err != nil {
		return nil, err
	}
	responses := make([]aggregateGroupCategoryResponse, 0, len(categories))
	for _, category := range categories {
		responses = append(responses, aggregateGroupCategoryResponse{
			Id:                  category.Id,
			Name:                category.Name,
			OrderIndex:          category.OrderIndex,
			AggregateGroupCount: counts[category.Id],
		})
	}
	return responses, nil
}

func GetAggregateGroupCategories(c *gin.Context) {
	categories, err := buildAggregateGroupCategoryResponses()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, categories)
}

func CreateAggregateGroupCategory(c *gin.Context) {
	var req aggregateGroupCategoryNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	category := &model.AggregateGroupCategory{Name: req.Name}
	if err := model.InsertAggregateGroupCategory(category); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, aggregateGroupCategoryResponse{
		Id:         category.Id,
		Name:       category.Name,
		OrderIndex: category.OrderIndex,
	})
}

func UpdateAggregateGroupCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req aggregateGroupCategoryNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateAggregateGroupCategoryName(id, req.Name); err != nil {
		common.ApiError(c, err)
		return
	}
	category, err := model.GetAggregateGroupCategoryByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, aggregateGroupCategoryResponse{
		Id:         category.Id,
		Name:       category.Name,
		OrderIndex: category.OrderIndex,
	})
}

func ReorderAggregateGroupCategories(c *gin.Context) {
	var req aggregateGroupCategoryOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ReorderAggregateGroupCategories(req.CategoryIds); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteAggregateGroupCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteAggregateGroupCategory(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AssignAggregateGroupCategories(c *gin.Context) {
	var req aggregateGroupCategoryAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.AssignAggregateGroupsToCategory(req.AggregateGroupIds, req.CategoryId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
