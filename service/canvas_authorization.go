package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CanvasClientId                  = "infinite-canvas"
	canvasOptionEnabled             = "canvas.authorization.enabled"
	canvasOptionImageGroup          = "canvas.authorization.image_group"
	canvasOptionVideoGroup          = "canvas.authorization.video_group"
	canvasOptionRedirectUris        = "canvas.authorization.redirect_uris"
	canvasAuthorizationCodeLifetime = 2 * time.Minute
)

var (
	pkceVerifierPattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	pkceChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

type CanvasAuthorizationError struct {
	Code          string   `json:"code"`
	Message       string   `json:"message"`
	MissingGroups []string `json:"missing_groups,omitempty"`
}

func (e *CanvasAuthorizationError) Error() string { return e.Message }

type CanvasConfig struct {
	Enabled      bool     `json:"enabled"`
	ImageGroup   string   `json:"image_group"`
	VideoGroup   string   `json:"video_group"`
	RedirectUris []string `json:"redirect_uris"`
	ImageModels  []string `json:"image_models"`
	VideoModels  []string `json:"video_models"`
}

type CanvasGroupOption struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	ImageModels []string `json:"image_models"`
	VideoModels []string `json:"video_models"`
}

type CanvasAdminConfig struct {
	Config CanvasConfig        `json:"config"`
	Groups []CanvasGroupOption `json:"groups"`
}

type CanvasAuthorizationRequest struct {
	ClientId            string `json:"client_id"`
	RedirectUri         string `json:"redirect_uri"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

type CanvasAuthorizationContext struct {
	ClientId        string   `json:"client_id"`
	ApplicationName string   `json:"application_name"`
	Username        string   `json:"username"`
	DisplayName     string   `json:"display_name"`
	ImageGroup      string   `json:"image_group"`
	ImageGroupName  string   `json:"image_group_name"`
	VideoGroup      string   `json:"video_group"`
	VideoGroupName  string   `json:"video_group_name"`
	ImageModels     []string `json:"image_models"`
	VideoModels     []string `json:"video_models"`
}

type CanvasAuthorizationCodeResult struct {
	Code        string `json:"code"`
	State       string `json:"state"`
	RedirectUri string `json:"redirect_uri"`
}

type CanvasTokenExchangeRequest struct {
	GrantType    string `json:"grant_type"`
	ClientId     string `json:"client_id"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RedirectUri  string `json:"redirect_uri"`
}

type CanvasTokenExchangeResult struct {
	TokenType      string   `json:"token_type"`
	ImageApiKey    string   `json:"image_api_key"`
	VideoApiKey    string   `json:"video_api_key"`
	ResourceApiKey string   `json:"resource_api_key"`
	ImageModels    []string `json:"image_models"`
	VideoModels    []string `json:"video_models"`
	AuthorizedAt   int64    `json:"authorized_at"`
}

type CanvasModelSyncRequest struct {
	ClientId    string `json:"client_id"`
	ImageApiKey string `json:"image_api_key"`
	VideoApiKey string `json:"video_api_key"`
}

type CanvasModelSyncResult struct {
	ImageModels []string `json:"image_models"`
	VideoModels []string `json:"video_models"`
	SyncedAt    int64    `json:"synced_at"`
}

func canvasError(code, message string) error {
	return &CanvasAuthorizationError{Code: code, Message: message}
}

func getCanvasOption(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key]
}

func GetCanvasConfig() CanvasConfig {
	config := CanvasConfig{
		Enabled:    getCanvasOption(canvasOptionEnabled) == "true",
		ImageGroup: strings.TrimSpace(getCanvasOption(canvasOptionImageGroup)),
		VideoGroup: strings.TrimSpace(getCanvasOption(canvasOptionVideoGroup)),
	}
	_ = common.UnmarshalJsonStr(getCanvasOption(canvasOptionRedirectUris), &config.RedirectUris)
	if config.RedirectUris == nil {
		config.RedirectUris = []string{}
	}
	config.ImageModels = canvasModelsForGroup(config.ImageGroup, constant.EndpointTypeImageGeneration)
	config.VideoModels = canvasModelsForGroup(config.VideoGroup, constant.EndpointTypeVideoTask, constant.EndpointTypeOpenAIVideo)
	return config
}

func GetCanvasAdminConfig() (CanvasAdminConfig, error) {
	groups, err := model.GetAllAggregateGroups(true)
	if err != nil {
		return CanvasAdminConfig{}, err
	}
	model.GetPricing()
	options := make([]CanvasGroupOption, 0, len(groups))
	for _, group := range groups {
		options = append(options, CanvasGroupOption{
			Name: group.Name, DisplayName: group.DisplayName, Description: group.Description,
			ImageModels: canvasModelsForGroup(group.Name, constant.EndpointTypeImageGeneration),
			VideoModels: canvasModelsForGroup(group.Name, constant.EndpointTypeVideoTask, constant.EndpointTypeOpenAIVideo),
		})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].DisplayName < options[j].DisplayName })
	return CanvasAdminConfig{Config: GetCanvasConfig(), Groups: options}, nil
}

func SaveCanvasConfig(config CanvasConfig) (CanvasAdminConfig, error) {
	config.ImageGroup = strings.TrimSpace(config.ImageGroup)
	config.VideoGroup = strings.TrimSpace(config.VideoGroup)
	redirectUris, err := normalizeCanvasRedirectUris(config.RedirectUris)
	if err != nil {
		return CanvasAdminConfig{}, err
	}
	config.RedirectUris = redirectUris
	if config.Enabled {
		if config.ImageGroup == "" || config.VideoGroup == "" || len(config.RedirectUris) == 0 {
			return CanvasAdminConfig{}, errors.New("启用 Canvas 授权前必须配置图片分组、视频分组和回调地址")
		}
		if _, err := model.GetAggregateGroupByName(config.ImageGroup, true); err != nil {
			return CanvasAdminConfig{}, errors.New("Canvas 图片聚合分组不存在或未启用")
		}
		if _, err := model.GetAggregateGroupByName(config.VideoGroup, true); err != nil {
			return CanvasAdminConfig{}, errors.New("Canvas 视频聚合分组不存在或未启用")
		}
		if len(canvasModelsForGroup(config.ImageGroup, constant.EndpointTypeImageGeneration)) == 0 {
			return CanvasAdminConfig{}, errors.New("Canvas 图片聚合分组没有可用的生图模型")
		}
		if len(canvasModelsForGroup(config.VideoGroup, constant.EndpointTypeVideoTask, constant.EndpointTypeOpenAIVideo)) == 0 {
			return CanvasAdminConfig{}, errors.New("Canvas 视频聚合分组没有可用的视频模型")
		}
	}
	redirectBytes, err := common.Marshal(config.RedirectUris)
	if err != nil {
		return CanvasAdminConfig{}, err
	}
	err = model.UpdateOptions(map[string]string{
		canvasOptionEnabled:      boolString(config.Enabled),
		canvasOptionImageGroup:   config.ImageGroup,
		canvasOptionVideoGroup:   config.VideoGroup,
		canvasOptionRedirectUris: string(redirectBytes),
	})
	if err != nil {
		return CanvasAdminConfig{}, err
	}
	return GetCanvasAdminConfig()
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func normalizeCanvasRedirectUris(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, errors.New("Canvas 回调地址必须是无查询参数和片段的完整 URL")
		}
		hostname := strings.ToLower(parsed.Hostname())
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (hostname == "localhost" || hostname == "127.0.0.1")) {
			return nil, errors.New("Canvas 回调地址必须使用 HTTPS，本地 localhost 或 127.0.0.1 可使用 HTTP")
		}
		if strings.Contains(value, "*") {
			return nil, errors.New("Canvas 回调地址不支持通配符")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func canvasModelsForGroup(group string, endpointTypes ...constant.EndpointType) []string {
	if strings.TrimSpace(group) == "" {
		return []string{}
	}
	model.GetPricing()
	wanted := make(map[constant.EndpointType]struct{}, len(endpointTypes))
	for _, endpointType := range endpointTypes {
		wanted[endpointType] = struct{}{}
	}
	models := make([]string, 0)
	for _, modelName := range GetModelsForGroup(group) {
		for _, endpointType := range model.GetModelSupportEndpointTypes(modelName) {
			if _, ok := wanted[endpointType]; ok {
				models = append(models, modelName)
				break
			}
		}
	}
	sort.Strings(models)
	return slices.Compact(models)
}

func validateCanvasAuthorizationRequest(request CanvasAuthorizationRequest) (CanvasConfig, error) {
	if request.ClientId != CanvasClientId {
		return CanvasConfig{}, canvasError("invalid_client", "不支持的 Canvas 客户端")
	}
	if request.State == "" || len(request.State) > 256 {
		return CanvasConfig{}, canvasError("invalid_state", "授权状态参数无效")
	}
	if request.CodeChallengeMethod != "S256" || !pkceChallengePattern.MatchString(request.CodeChallenge) {
		return CanvasConfig{}, canvasError("invalid_pkce", "Canvas 授权必须使用 PKCE S256")
	}
	config := GetCanvasConfig()
	if !config.Enabled {
		return CanvasConfig{}, canvasError("canvas_disabled", "管理员尚未启用 Canvas 授权")
	}
	if !slices.Contains(config.RedirectUris, request.RedirectUri) {
		return CanvasConfig{}, canvasError("invalid_redirect_uri", "Canvas 回调地址未被管理员允许")
	}
	if len(config.ImageModels) == 0 || len(config.VideoModels) == 0 {
		return CanvasConfig{}, canvasError("invalid_canvas_config", "Canvas 授权分组没有可用模型，请联系管理员")
	}
	return config, nil
}

func validateCanvasUser(user *model.User, config CanvasConfig) error {
	if user == nil || user.Id == 0 || user.Status != common.UserStatusEnabled {
		return canvasError("user_unavailable", "当前账号不可用")
	}
	usable := GetUserUsableGroupsWithSetting(user.Group, user.GetSetting())
	missing := make([]string, 0, 2)
	if _, ok := usable[config.ImageGroup]; !ok {
		missing = append(missing, config.ImageGroup)
	}
	if _, ok := usable[config.VideoGroup]; !ok && config.VideoGroup != config.ImageGroup {
		missing = append(missing, config.VideoGroup)
	}
	if len(missing) > 0 {
		return &CanvasAuthorizationError{Code: "group_forbidden", Message: "当前账号缺少 Canvas 所需分组权限", MissingGroups: missing}
	}
	return nil
}

func GetCanvasAuthorizationContext(userId int, request CanvasAuthorizationRequest) (CanvasAuthorizationContext, error) {
	config, err := validateCanvasAuthorizationRequest(request)
	if err != nil {
		return CanvasAuthorizationContext{}, err
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return CanvasAuthorizationContext{}, err
	}
	if err := validateCanvasUser(user, config); err != nil {
		return CanvasAuthorizationContext{}, err
	}
	imageGroup, _ := model.GetAggregateGroupByName(config.ImageGroup, true)
	videoGroup, _ := model.GetAggregateGroupByName(config.VideoGroup, true)
	return CanvasAuthorizationContext{
		ClientId: CanvasClientId, ApplicationName: "Infinite Canvas", Username: user.Username, DisplayName: user.DisplayName,
		ImageGroup: config.ImageGroup, ImageGroupName: canvasGroupDisplayName(imageGroup, config.ImageGroup),
		VideoGroup: config.VideoGroup, VideoGroupName: canvasGroupDisplayName(videoGroup, config.VideoGroup),
		ImageModels: config.ImageModels, VideoModels: config.VideoModels,
	}, nil
}

func canvasGroupDisplayName(group *model.AggregateGroup, fallback string) string {
	if group != nil && strings.TrimSpace(group.DisplayName) != "" {
		return group.DisplayName
	}
	return fallback
}

func IssueCanvasAuthorizationCode(userId int, request CanvasAuthorizationRequest) (CanvasAuthorizationCodeResult, error) {
	config, err := validateCanvasAuthorizationRequest(request)
	if err != nil {
		return CanvasAuthorizationCodeResult{}, err
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return CanvasAuthorizationCodeResult{}, err
	}
	if err := validateCanvasUser(user, config); err != nil {
		return CanvasAuthorizationCodeResult{}, err
	}
	code, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return CanvasAuthorizationCodeResult{}, err
	}
	now := time.Now().Unix()
	row := model.CanvasAuthorizationCode{
		CodeHash: canvasCodeHash(code), UserId: userId, ClientId: request.ClientId,
		RedirectUri: request.RedirectUri, CodeChallenge: request.CodeChallenge, ConfigHash: canvasConfigHash(config),
		ExpiredAt: now + int64(canvasAuthorizationCodeLifetime/time.Second), CreatedAt: now,
	}
	if err := model.DB.Create(&row).Error; err != nil {
		return CanvasAuthorizationCodeResult{}, err
	}
	model.DB.Where("expired_at < ? OR (consumed_at > ? AND consumed_at < ?)", now-86400, 0, now-86400).Delete(&model.CanvasAuthorizationCode{})
	return CanvasAuthorizationCodeResult{Code: code, State: request.State, RedirectUri: request.RedirectUri}, nil
}

func ExchangeCanvasAuthorizationCode(request CanvasTokenExchangeRequest) (CanvasTokenExchangeResult, error) {
	if request.GrantType != "authorization_code" || request.ClientId != CanvasClientId {
		return CanvasTokenExchangeResult{}, canvasError("invalid_request", "授权兑换参数无效")
	}
	if !pkceVerifierPattern.MatchString(request.CodeVerifier) || request.Code == "" {
		return CanvasTokenExchangeResult{}, canvasError("invalid_grant", "授权码或 PKCE verifier 无效")
	}
	config := GetCanvasConfig()
	if !config.Enabled || !slices.Contains(config.RedirectUris, request.RedirectUri) || len(config.ImageModels) == 0 || len(config.VideoModels) == 0 {
		return CanvasTokenExchangeResult{}, canvasError("invalid_canvas_config", "Canvas 授权配置已变化，请重新授权")
	}
	var result CanvasTokenExchangeResult
	var imageTokenId, videoTokenId int
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		var authorizationCode model.CanvasAuthorizationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ?", canvasCodeHash(request.Code)).First(&authorizationCode).Error; err != nil {
			return canvasError("invalid_grant", "授权码无效或已失效")
		}
		if authorizationCode.ConsumedAt != 0 || authorizationCode.ExpiredAt < now || authorizationCode.ClientId != request.ClientId || authorizationCode.RedirectUri != request.RedirectUri {
			return canvasError("invalid_grant", "授权码已过期、已使用或与客户端不匹配")
		}
		if authorizationCode.ConfigHash != canvasConfigHash(config) {
			return canvasError("invalid_canvas_config", "Canvas 授权配置已变化，请重新授权")
		}
		if !validatePkce(request.CodeVerifier, authorizationCode.CodeChallenge) {
			return canvasError("invalid_grant", "PKCE 校验失败")
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, authorizationCode.UserId).Error; err != nil {
			return err
		}
		if err := validateCanvasUser(&user, config); err != nil {
			return err
		}
		var grant model.CanvasGrant
		grantErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND client_id = ?", user.Id, CanvasClientId).First(&grant).Error
		if grantErr != nil && !errors.Is(grantErr, gorm.ErrRecordNotFound) {
			return grantErr
		}
		imageToken, imageExists, err := getCanvasGrantToken(tx, grant.ImageTokenId, user.Id)
		if err != nil {
			return err
		}
		videoToken, videoExists, err := getCanvasGrantToken(tx, grant.VideoTokenId, user.Id)
		if err != nil {
			return err
		}
		if imageExists && videoExists && imageToken.Id == videoToken.Id {
			videoToken, videoExists = nil, false
		}
		needed := 0
		if !imageExists {
			needed++
		}
		if !videoExists {
			needed++
		}
		if needed > 0 {
			var count int64
			if err := tx.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
				return err
			}
			if count+int64(needed) > int64(operation_setting.GetMaxUserTokens()) {
				return canvasError("token_limit_reached", "账号令牌数量已达上限，无法创建 Canvas 专用令牌")
			}
		}
		imageToken, err = upsertCanvasToken(tx, imageToken, imageExists, user.Id, "canvas-images", config.ImageGroup, config.ImageModels, now)
		if err != nil {
			return err
		}
		videoToken, err = upsertCanvasToken(tx, videoToken, videoExists, user.Id, "canvas-videos", config.VideoGroup, config.VideoModels, now)
		if err != nil {
			return err
		}
		if errors.Is(grantErr, gorm.ErrRecordNotFound) {
			grant = model.CanvasGrant{UserId: user.Id, ClientId: CanvasClientId, CreatedTime: now}
		}
		grant.ImageTokenId = imageToken.Id
		grant.VideoTokenId = videoToken.Id
		grant.UpdatedTime = now
		if grant.Id == 0 {
			if err := tx.Create(&grant).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&grant).Updates(map[string]any{"image_token_id": grant.ImageTokenId, "video_token_id": grant.VideoTokenId, "updated_time": now}).Error; err != nil {
			return err
		}
		assetKey, found, err := model.GetActiveUserAssetKeyTx(tx, user.Id)
		if err != nil {
			return err
		}
		if !found {
			assetKey, err = model.CreateAssetKeyWithScopesTx(tx, user.Id, "canvas-resources", -1, "", []string{model.AssetKeyScopeRead})
			if err != nil {
				return err
			}
		}
		consume := tx.Model(&model.CanvasAuthorizationCode{}).Where("id = ? AND consumed_at = ?", authorizationCode.Id, 0).Update("consumed_at", now)
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return canvasError("invalid_grant", "授权码已经被使用")
		}
		imageTokenId, videoTokenId = imageToken.Id, videoToken.Id
		result = CanvasTokenExchangeResult{
			TokenType: "Bearer", ImageApiKey: "sk-" + imageToken.Key, VideoApiKey: "sk-" + videoToken.Key,
			ResourceApiKey: assetKey.Key, ImageModels: config.ImageModels, VideoModels: config.VideoModels, AuthorizedAt: now,
		}
		return nil
	})
	if err != nil {
		return CanvasTokenExchangeResult{}, err
	}
	model.RefreshTokenCacheById(imageTokenId)
	model.RefreshTokenCacheById(videoTokenId)
	return result, nil
}

func SyncCanvasModels(userId int, assetKeyId int64, request CanvasModelSyncRequest) (CanvasModelSyncResult, error) {
	if userId == 0 || assetKeyId == 0 || request.ClientId != CanvasClientId {
		return CanvasModelSyncResult{}, canvasError("invalid_client", "不支持的 Canvas 客户端")
	}
	imageKey := canvasTokenKey(request.ImageApiKey)
	videoKey := canvasTokenKey(request.VideoApiKey)
	if imageKey == "" || videoKey == "" || imageKey == videoKey {
		return CanvasModelSyncResult{}, canvasError("invalid_credentials", "Canvas 授权凭证无效，请重新授权")
	}
	imageToken, imageErr := model.GetTokenByKey(imageKey, true)
	videoToken, videoErr := model.GetTokenByKey(videoKey, true)
	if imageErr != nil || videoErr != nil || imageToken.UserId != userId || videoToken.UserId != userId {
		return CanvasModelSyncResult{}, canvasError("invalid_credentials", "Canvas 授权凭证无效，请重新授权")
	}
	config := GetCanvasConfig()
	if !config.Enabled || len(config.ImageModels) == 0 || len(config.VideoModels) == 0 {
		return CanvasModelSyncResult{}, canvasError("invalid_canvas_config", "Canvas 授权配置不可用，请联系管理员")
	}

	var result CanvasModelSyncResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userId).Error; err != nil {
			return err
		}
		if err := validateCanvasUser(&user, config); err != nil {
			return err
		}
		var assetKey model.AssetKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", assetKeyId, userId).First(&assetKey).Error; err != nil {
			return canvasError("invalid_credentials", "Canvas Resource Key 不可用，请重新授权")
		}
		if assetKey.Status != model.AssetKeyStatusEnabled || assetKey.IsExpired(now) || !model.AssetKeyHasScope(assetKey.Scopes, model.AssetKeyScopeRead) {
			return canvasError("invalid_credentials", "Canvas Resource Key 不可用，请重新授权")
		}
		var currentAssetKey model.AssetKey
		if err := tx.Where("user_id = ?", userId).Order("id DESC").First(&currentAssetKey).Error; err != nil || currentAssetKey.ID != assetKey.ID {
			return canvasError("invalid_credentials", "Canvas Resource Key 不可用，请重新授权")
		}

		var grant model.CanvasGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND client_id = ?", userId, CanvasClientId).First(&grant).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return canvasError("invalid_credentials", "Canvas 授权凭证无效，请重新授权")
			}
			return err
		}
		if grant.ImageTokenId != imageToken.Id || grant.VideoTokenId != videoToken.Id || grant.ImageTokenId == grant.VideoTokenId {
			return canvasError("invalid_credentials", "Canvas 授权凭证无效，请重新授权")
		}

		var lockedImageToken, lockedVideoToken model.Token
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", grant.ImageTokenId, userId).First(&lockedImageToken).Error; err != nil {
			return canvasError("invalid_credentials", "Canvas 图片凭证不可用，请重新授权")
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", grant.VideoTokenId, userId).First(&lockedVideoToken).Error; err != nil {
			return canvasError("invalid_credentials", "Canvas 视频凭证不可用，请重新授权")
		}
		if !canvasTokenMatches(&lockedImageToken, imageKey, now) || !canvasTokenMatches(&lockedVideoToken, videoKey, now) {
			return canvasError("invalid_credentials", "Canvas 授权凭证不可用，请重新授权")
		}
		if err := syncCanvasToken(tx, &lockedImageToken, config.ImageGroup, config.ImageModels); err != nil {
			return err
		}
		if err := syncCanvasToken(tx, &lockedVideoToken, config.VideoGroup, config.VideoModels); err != nil {
			return err
		}
		result = CanvasModelSyncResult{ImageModels: config.ImageModels, VideoModels: config.VideoModels, SyncedAt: now}
		return nil
	})
	if err != nil {
		return CanvasModelSyncResult{}, err
	}
	model.RefreshTokenCacheById(imageToken.Id)
	model.RefreshTokenCacheById(videoToken.Id)
	return result, nil
}

func canvasTokenKey(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "sk-")
}

func canvasTokenMatches(token *model.Token, key string, now int64) bool {
	if token == nil || token.Status != common.TokenStatusEnabled || (token.ExpiredTime != -1 && token.ExpiredTime < now) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token.Key), []byte(key)) == 1
}

func syncCanvasToken(tx *gorm.DB, token *model.Token, group string, models []string) error {
	modelLimits := strings.Join(models, ",")
	if err := tx.Model(token).Updates(map[string]any{
		"model_limits_enabled": true,
		"model_limits":         modelLimits,
		"group":                group,
		"cross_group_retry":    false,
	}).Error; err != nil {
		return err
	}
	token.ModelLimitsEnabled, token.ModelLimits, token.Group, token.CrossGroupRetry = true, modelLimits, group, false
	return nil
}

func getCanvasGrantToken(tx *gorm.DB, tokenId, userId int) (*model.Token, bool, error) {
	if tokenId == 0 {
		return nil, false, nil
	}
	var token model.Token
	err := tx.Unscoped().Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || token.DeletedAt.Valid {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &token, true, nil
}

func upsertCanvasToken(tx *gorm.DB, token *model.Token, exists bool, userId int, name, group string, models []string, now int64) (*model.Token, error) {
	modelLimits := strings.Join(models, ",")
	if !exists {
		key, err := common.GenerateKey()
		if err != nil {
			return nil, err
		}
		emptyIps := ""
		token = &model.Token{
			UserId: userId, Key: key, Status: common.TokenStatusEnabled, Name: name,
			CreatedTime: now, AccessedTime: now, ExpiredTime: -1, UnlimitedQuota: true,
			ModelLimitsEnabled: true, ModelLimits: modelLimits, AllowIps: &emptyIps, Group: group,
		}
		if err := tx.Create(token).Error; err != nil {
			return nil, err
		}
		return token, nil
	}
	updates := map[string]any{
		"name": name, "status": common.TokenStatusEnabled, "expired_time": -1, "unlimited_quota": true,
		"model_limits_enabled": true, "model_limits": modelLimits, "allow_ips": "", "group": group, "cross_group_retry": false,
	}
	if err := tx.Model(token).Updates(updates).Error; err != nil {
		return nil, err
	}
	token.Name, token.Status, token.ExpiredTime, token.UnlimitedQuota = name, common.TokenStatusEnabled, -1, true
	token.ModelLimitsEnabled, token.ModelLimits, token.Group, token.CrossGroupRetry = true, modelLimits, group, false
	return token, nil
}

func canvasCodeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func canvasConfigHash(config CanvasConfig) string {
	value := strings.Join([]string{
		config.ImageGroup,
		config.VideoGroup,
		strings.Join(config.ImageModels, "\x1f"),
		strings.Join(config.VideoModels, "\x1f"),
	}, "\x1e")
	return canvasCodeHash(value)
}

func validatePkce(verifier, expectedChallenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expectedChallenge)) == 1
}
