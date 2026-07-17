package controller

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetUserAssets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID := c.GetInt("id")
	queryParams := parseAssetQuery(c, false)
	items, err := model.AssetGetAllByUser(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total := model.AssetCountAllByUser(userID, queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assetsToDto(items, false))
	common.ApiSuccess(c, pageInfo)
}

func GetUserAssetKeys(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keys, err := model.GetUserAssetKeys(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total := model.CountUserAssetKeys(c.GetInt("id"))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assetKeysToDto(keys))
	common.ApiSuccess(c, pageInfo)
}

func CreateUserAssetKey(c *gin.Context) {
	req := dto.AssetKeyCreateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	key, err := model.CreateAssetKeyWithScopes(c.GetInt("id"), req.Name, req.ExpiredAt, req.AllowIPs, req.Scopes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, assetKeyToDto(key))
}

func UpdateUserAssetKeyStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	req := dto.AssetKeyStatusRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	key, err := model.UpdateUserAssetKeyStatus(id, c.GetInt("id"), req.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, assetKeyToDto(key))
}

func DeleteUserAssetKey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteUserAssetKey(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetAllAssets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	queryParams := parseAssetQuery(c, true)
	items, err := model.AssetGetAll(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total := model.AssetCountAll(queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assetsToDto(items, true))
	common.ApiSuccess(c, pageInfo)
}

func GetUserAsset(c *gin.Context) {
	asset, exists, err := model.GetUserAssetByAssetID(c.GetInt("id"), c.Param("asset_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiError(c, errors.New("asset_not_exist"))
		return
	}
	common.ApiSuccess(c, assetToDto(asset))
}

func GetAsset(c *gin.Context) {
	asset, exists, err := model.GetAssetByAssetID(c.Param("asset_id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiError(c, errors.New("asset_not_exist"))
		return
	}
	fillAssetUsernames([]*model.Asset{asset})
	common.ApiSuccess(c, assetToDto(asset))
}

func GetUserAssetBatchURLs(c *gin.Context) {
	req := dto.AssetBatchURLRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	assetIDs := normalizeAssetIDs(req.AssetIDs, 100)
	assets, err := model.GetUserAssetsByAssetIDs(c.GetInt("id"), assetIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, assetsToURLItems(assets))
}

func GetAssetBatchURLs(c *gin.Context) {
	req := dto.AssetBatchURLRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	assetIDs := normalizeAssetIDs(req.AssetIDs, 100)
	assets, err := model.GetAssetsByAssetIDs(assetIDs, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, assetsToURLItems(assets))
}

func ExportUserAssets(c *gin.Context) {
	userID := c.GetInt("id")
	queryParams := parseAssetQuery(c, false)
	items, err := model.AssetGetAllByUser(userID, 0, 10000, queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	writeAssetCSV(c, items)
}

func ExportAssets(c *gin.Context) {
	queryParams := parseAssetQuery(c, true)
	items, err := model.AssetGetAll(0, 10000, queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	writeAssetCSV(c, items)
}

func UpdateAssetBlockStatus(c *gin.Context) {
	req := dto.AssetBlockRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	asset, exists, err := model.UpdateAssetBlocked(c.Param("asset_id"), req.IsBlocked)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiError(c, errors.New("asset_not_exist"))
		return
	}
	action := "解除屏蔽"
	if req.IsBlocked {
		action = "屏蔽"
	}
	model.RecordLog(asset.UserID, model.LogTypeManage, "管理员"+action+"资源，管理员ID："+strconv.Itoa(c.GetInt("id"))+"，资源ID："+asset.AssetID)
	common.ApiSuccess(c, assetToDto(asset))
}

func ListAssetsByAPIKey(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	page := pageInfo.GetPage()
	if apiPage, err := strconv.Atoi(c.Query("page")); err == nil && apiPage > 0 {
		page = apiPage
	}
	queryParams := parseAssetQuery(c, false)
	writeAssetAPIList(c, c.GetInt("id"), page, pageInfo.GetPageSize(), queryParams)
}

func QueryAssetsByAPIKey(c *gin.Context) {
	req := dto.AssetAPIQueryRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAssetAPIError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if len(req.AssetIDs) > 0 {
		assetIDs := normalizeAssetIDs(req.AssetIDs, 100)
		assets, err := model.GetUserAssetsByAssetIDs(c.GetInt("id"), assetIDs)
		if err != nil {
			writeAssetAPIError(c, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, dto.AssetAPIListResponse{
			Object:   "list",
			Data:     assetsToAPIItems(assets),
			Page:     1,
			PageSize: len(assetIDs),
			Total:    int64(len(assets)),
			HasMore:  false,
		})
		return
	}
	page, pageSize := normalizeAssetAPIPage(req.Page, req.PageSize)
	queryParams := model.AssetQueryParams{
		AssetType:      model.AssetType(req.AssetType),
		TaskID:         req.TaskID,
		Platform:       constant.TaskPlatform(req.Platform),
		Action:         req.Action,
		Model:          req.Model,
		StartTimestamp: req.StartTimestamp,
		EndTimestamp:   req.EndTimestamp,
	}
	if queryParams.AssetType != "" {
		switch queryParams.AssetType {
		case model.AssetTypeImage, model.AssetTypeVideo, model.AssetTypeAudio, model.AssetTypeFile:
		default:
			writeAssetAPIError(c, http.StatusBadRequest, "invalid_request", "invalid asset_type")
			return
		}
	}
	writeAssetAPIList(c, c.GetInt("id"), page, pageSize, queryParams)
}

func GetAssetByAPIKey(c *gin.Context) {
	asset, exists, err := model.GetUserAssetByAssetID(c.GetInt("id"), c.Param("asset_id"))
	if err != nil {
		writeAssetAPIError(c, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if !exists {
		writeAssetAPIError(c, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	c.JSON(http.StatusOK, assetToAPIItem(asset))
}

func GetAssetBatchURLsByAPIKey(c *gin.Context) {
	req := dto.AssetBatchURLRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAssetAPIError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	assetIDs := normalizeAssetIDs(req.AssetIDs, 100)
	assets, err := model.GetUserAssetsByAssetIDs(c.GetInt("id"), assetIDs)
	if err != nil {
		writeAssetAPIError(c, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   assetsToURLItems(assets),
	})
}

func ExportAssetsByAPIKey(c *gin.Context) {
	queryParams := parseAssetQuery(c, false)
	items, err := model.AssetGetAllByUser(c.GetInt("id"), 0, 10000, queryParams)
	if err != nil {
		writeAssetAPIError(c, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeAssetCSV(c, items)
}

func parseAssetQuery(c *gin.Context, admin bool) model.AssetQueryParams {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	status := model.AssetStatus(strings.TrimSpace(c.Query("status")))
	if status != "" && status != model.AssetStatusAvailable && status != model.AssetStatusBlocked && status != model.AssetStatusDeleted && status != model.AssetStatusUnavailable {
		status = ""
	}
	assetType := model.AssetType(strings.TrimSpace(c.Query("asset_type")))
	if assetType != "" && assetType != model.AssetTypeImage && assetType != model.AssetTypeVideo && assetType != model.AssetTypeAudio && assetType != model.AssetTypeFile {
		assetType = ""
	}
	userID := ""
	if admin {
		userID = c.Query("user_id")
	}
	return model.AssetQueryParams{
		AssetType:      assetType,
		Status:         status,
		TaskID:         c.Query("task_id"),
		Platform:       constant.TaskPlatform(c.Query("platform")),
		Action:         c.Query("action"),
		Model:          c.Query("model"),
		ChannelID:      c.Query("channel_id"),
		UserID:         userID,
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Keyword:        c.Query("keyword"),
		IncludeHidden:  admin,
	}
}

func assetToDto(asset *model.Asset) *dto.AssetDto {
	if asset == nil {
		return nil
	}
	return &dto.AssetDto{
		ID:           asset.ID,
		AssetID:      asset.AssetID,
		TaskID:       asset.TaskID,
		TaskRecordID: asset.TaskRecordID,
		AssetIndex:   asset.AssetIndex,
		UserID:       asset.UserID,
		Group:        asset.Group,
		ChannelID:    asset.ChannelID,
		Platform:     string(asset.Platform),
		Action:       asset.Action,
		Model:        asset.Model,
		AssetType:    string(asset.AssetType),
		URL:          asset.URL,
		ThumbnailURL: asset.ThumbnailURL,
		MimeType:     asset.MimeType,
		Filename:     asset.Filename,
		SizeBytes:    asset.SizeBytes,
		Width:        asset.Width,
		Height:       asset.Height,
		DurationMS:   asset.DurationMS,
		Status:       string(asset.Status),
		Metadata:     asset.Metadata,
		CreatedAt:    asset.CreatedAt,
		UpdatedAt:    asset.UpdatedAt,
		DeletedAt:    asset.DeletedAt,
		Username:     asset.Username,
	}
}

func assetToAPIItem(asset *model.Asset) *dto.AssetAPIItem {
	if asset == nil {
		return nil
	}
	return &dto.AssetAPIItem{
		Object:       "asset",
		ID:           asset.AssetID,
		TaskID:       asset.TaskID,
		Index:        asset.AssetIndex,
		Type:         string(asset.AssetType),
		URL:          asset.URL,
		ThumbnailURL: asset.ThumbnailURL,
		MimeType:     asset.MimeType,
		Filename:     asset.Filename,
		SizeBytes:    asset.SizeBytes,
		Width:        asset.Width,
		Height:       asset.Height,
		DurationMS:   asset.DurationMS,
		Model:        asset.Model,
		Platform:     string(asset.Platform),
		Action:       asset.Action,
		Status:       string(asset.Status),
		Metadata:     asset.Metadata,
		CreatedAt:    asset.CreatedAt,
		UpdatedAt:    asset.UpdatedAt,
	}
}

func assetsToAPIItems(assets []*model.Asset) []*dto.AssetAPIItem {
	items := make([]*dto.AssetAPIItem, 0, len(assets))
	for _, asset := range assets {
		items = append(items, assetToAPIItem(asset))
	}
	return items
}

func writeAssetAPIList(c *gin.Context, userID int, page int, pageSize int, queryParams model.AssetQueryParams) {
	page, pageSize = normalizeAssetAPIPage(page, pageSize)
	startIdx := (page - 1) * pageSize
	items, err := model.AssetGetAllByUser(userID, startIdx, pageSize, queryParams)
	if err != nil {
		writeAssetAPIError(c, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	total := model.AssetCountAllByUser(userID, queryParams)
	c.JSON(http.StatusOK, dto.AssetAPIListResponse{
		Object:   "list",
		Data:     assetsToAPIItems(items),
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasMore:  int64(page*pageSize) < total,
	})
}

func normalizeAssetAPIPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func writeAssetAPIError(c *gin.Context, statusCode int, code string, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}

func assetKeyToDto(key *model.AssetKey) *dto.AssetKeyDto {
	if key == nil {
		return nil
	}
	return &dto.AssetKeyDto{
		ID:         key.ID,
		Name:       key.Name,
		Key:        key.Key,
		Status:     key.Status,
		Scopes:     key.Scopes,
		AllowIPs:   key.AllowIPs,
		ExpiredAt:  key.ExpiredAt,
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
	}
}

func assetKeysToDto(keys []*model.AssetKey) []*dto.AssetKeyDto {
	result := make([]*dto.AssetKeyDto, 0, len(keys))
	for _, key := range keys {
		result = append(result, assetKeyToDto(key))
	}
	return result
}

func assetsToDto(assets []*model.Asset, fillUser bool) []*dto.AssetDto {
	if fillUser {
		fillAssetUsernames(assets)
	}
	result := make([]*dto.AssetDto, len(assets))
	for i, asset := range assets {
		result[i] = assetToDto(asset)
	}
	return result
}

func fillAssetUsernames(assets []*model.Asset) {
	userIdMap := make(map[int]*model.UserBase)
	for _, asset := range assets {
		if asset.UserID <= 0 {
			continue
		}
		if _, ok := userIdMap[asset.UserID]; ok {
			continue
		}
		user, err := model.GetUserCache(asset.UserID)
		if err == nil {
			userIdMap[asset.UserID] = user
		}
	}
	for _, asset := range assets {
		if user, ok := userIdMap[asset.UserID]; ok {
			asset.Username = user.Username
		}
	}
}

func normalizeAssetIDs(assetIDs []string, limit int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		assetID = strings.TrimSpace(assetID)
		if assetID == "" || seen[assetID] {
			continue
		}
		seen[assetID] = true
		result = append(result, assetID)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func assetsToURLItems(assets []*model.Asset) []dto.AssetURLItem {
	items := make([]dto.AssetURLItem, 0, len(assets))
	for _, asset := range assets {
		items = append(items, dto.AssetURLItem{
			AssetID: asset.AssetID,
			TaskID:  asset.TaskID,
			Type:    string(asset.AssetType),
			URL:     asset.URL,
		})
	}
	return items
}

func writeAssetCSV(c *gin.Context, assets []*model.Asset) {
	var buffer bytes.Buffer
	buffer.WriteString("asset_id,task_id,asset_type,url,filename,model,platform,action,created_at\n")
	for _, asset := range assets {
		row := []string{
			asset.AssetID,
			asset.TaskID,
			string(asset.AssetType),
			asset.URL,
			asset.Filename,
			asset.Model,
			string(asset.Platform),
			asset.Action,
			strconv.FormatInt(asset.CreatedAt, 10),
		}
		buffer.WriteString(csvLine(row))
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=assets.csv")
	c.String(200, buffer.String())
}

func csvLine(fields []string) string {
	escaped := make([]string, len(fields))
	for i, field := range fields {
		field = strings.ReplaceAll(field, `"`, `""`)
		escaped[i] = `"` + field + `"`
	}
	return strings.Join(escaped, ",") + "\n"
}
