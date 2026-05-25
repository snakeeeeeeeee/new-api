package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	goahocorasick "github.com/anknown/ahocorasick"
	"github.com/gin-gonic/gin"
)

type violationMatchSnapshot struct {
	enabled       bool
	caseSensitive bool
	keywordsRaw   string
	keywords      []string
	lowerKeywords []string
	caseMachine   *goahocorasick.Machine
}

type violationMatcherCache struct {
	mu       sync.RWMutex
	snapshot violationMatchSnapshot
}

var violationMatcher violationMatcherCache

const violationLogContextKey = "violation_log_id"

func IsViolationDetectionEnabled() bool {
	return operation_setting.GetViolationSetting().Enabled
}

func CheckViolationAndHandle(c *gin.Context, relayInfo *relaycommon.RelayInfo, combineText string) *types.NewAPIError {
	if relayInfo == nil || combineText == "" {
		return nil
	}
	setting := operation_setting.NormalizeViolationSetting(*operation_setting.GetViolationSetting())
	if !setting.Enabled {
		return nil
	}

	contains, words, firstKeyword := matchViolationKeywords(combineText, setting)
	if !contains {
		return nil
	}

	words = RemoveDuplicate(words)
	matchedBytes, err := common.Marshal(words)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to marshal violation matched words: %v", err))
		matchedBytes = []byte("[]")
	}

	action := operation_setting.NormalizeViolationAction(setting.Action)
	statusCode := setting.HTTPStatusCode
	errorCode := setting.ErrorCode
	if action == operation_setting.ViolationActionLogOnly {
		statusCode = 0
		errorCode = ""
	}

	requestPath := relayInfo.RequestURLPath
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}

	log := &model.ViolationLog{
		UserId:         relayInfo.UserId,
		Username:       c.GetString("username"),
		TokenId:        relayInfo.TokenId,
		TokenName:      c.GetString("token_name"),
		ModelName:      relayInfo.OriginModelName,
		UserGroup:      relayInfo.UserGroup,
		UsingGroup:     relayInfo.UsingGroup,
		AggregateGroup: common.GetContextKeyString(c, constant.ContextKeyAggregateGroup),
		RouteGroup:     common.GetContextKeyString(c, constant.ContextKeyRouteGroup),
		RequestId:      relayInfo.RequestId,
		RequestPath:    requestPath,
		MatchedWords:   string(matchedBytes),
		TextExcerpt:    buildViolationExcerpt(combineText, firstKeyword, setting),
		Action:         action,
		HTTPStatusCode: statusCode,
		ErrorCode:      errorCode,
		IsStream:       relayInfo.IsStream,
	}

	if err := model.InsertViolationLog(log); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to record violation log: %v", err))
	}

	if action == operation_setting.ViolationActionBanAfterThreshold {
		if banned := enforceViolationBanThreshold(c, relayInfo.UserId, setting.BanThreshold, log.Id); banned {
			log.Banned = true
		}
	}

	logger.LogWarn(c, fmt.Sprintf("violation keywords detected: user_id=%d, token_id=%d, model=%s, action=%s, words=%s",
		relayInfo.UserId, relayInfo.TokenId, relayInfo.OriginModelName, action, strings.Join(words, ", ")))

	if action == operation_setting.ViolationActionLogOnly {
		if log.Id > 0 {
			c.Set(violationLogContextKey, log.Id)
		}
		return nil
	}

	return types.NewErrorWithStatusCode(
		errors.New(setting.ErrorMessage),
		types.ErrorCode(setting.ErrorCode),
		setting.HTTPStatusCode,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func FillViolationLogRouteContextIfNeeded(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	if c == nil || relayInfo == nil {
		return
	}
	logID := c.GetInt(violationLogContextKey)
	if logID <= 0 {
		return
	}
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	usingGroup := relayInfo.UsingGroup
	if usingGroup == "" {
		usingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if routeGroup == "" && usingGroup == "" {
		return
	}
	if err := model.UpdateViolationLogRouteContext(logID, usingGroup, routeGroup); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to update violation route context: id=%d, err=%v", logID, err))
		return
	}
	c.Set(violationLogContextKey, 0)
}

func enforceViolationBanThreshold(c *gin.Context, userID int, threshold int, violationLogID int) bool {
	if userID <= 0 {
		return false
	}
	count, err := model.CountViolationLogsByUserID(userID)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to count violation logs: user_id=%d, err=%v", userID, err))
		return false
	}
	if count < int64(threshold) {
		return false
	}
	banned, err := model.DisableUserByViolation(userID)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to disable user by violation threshold: user_id=%d, err=%v", userID, err))
		return false
	}
	if err := model.MarkViolationLogBanned(violationLogID); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to mark violation log banned: id=%d, err=%v", violationLogID, err))
	}
	if banned {
		logger.LogWarn(c, fmt.Sprintf("user disabled by violation threshold: user_id=%d, threshold=%d, count=%d", userID, threshold, count))
	}
	return true
}

func matchViolationKeywords(text string, setting operation_setting.ViolationSetting) (bool, []string, string) {
	snapshot := getViolationMatchSnapshot(setting)
	if !snapshot.enabled || len(snapshot.keywords) == 0 {
		return false, nil, ""
	}
	if snapshot.caseSensitive {
		if snapshot.caseMachine == nil {
			return false, nil, ""
		}
		hits := snapshot.caseMachine.MultiPatternSearch([]rune(text), false)
		if len(hits) == 0 {
			return false, nil, ""
		}
		words := make([]string, 0, len(hits))
		firstKeyword := ""
		firstIndex := -1
		for _, hit := range hits {
			word := string(hit.Word)
			words = append(words, word)
			idx := strings.Index(text, word)
			if idx < 0 {
				continue
			}
			if firstIndex < 0 || idx < firstIndex {
				firstIndex = idx
				firstKeyword = word
			}
		}
		return true, words, firstKeyword
	}

	lowerText := strings.ToLower(text)
	contains, words := AcSearch(lowerText, snapshot.lowerKeywords, false)
	if !contains {
		return false, nil, ""
	}
	firstKeyword := ""
	firstIndex := -1
	for _, keyword := range words {
		idx := strings.Index(lowerText, strings.ToLower(keyword))
		if idx < 0 {
			continue
		}
		if firstIndex < 0 || idx < firstIndex {
			firstIndex = idx
			firstKeyword = keyword
		}
	}
	return true, words, firstKeyword
}

func getViolationMatchSnapshot(setting operation_setting.ViolationSetting) violationMatchSnapshot {
	violationMatcher.mu.RLock()
	current := violationMatcher.snapshot
	if current.enabled == setting.Enabled && current.caseSensitive == setting.CaseSensitive && current.keywordsRaw == setting.Keywords {
		violationMatcher.mu.RUnlock()
		return current
	}
	violationMatcher.mu.RUnlock()

	violationMatcher.mu.Lock()
	defer violationMatcher.mu.Unlock()
	current = violationMatcher.snapshot
	if current.enabled == setting.Enabled && current.caseSensitive == setting.CaseSensitive && current.keywordsRaw == setting.Keywords {
		return current
	}
	keywords := operation_setting.ViolationKeywordsFromString(setting.Keywords)
	lowerKeywords := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		lowerKeywords = append(lowerKeywords, strings.ToLower(keyword))
	}
	var caseMachine *goahocorasick.Machine
	if setting.CaseSensitive {
		caseMachine = buildCaseSensitiveViolationMachine(keywords)
	}
	next := violationMatchSnapshot{
		enabled:       setting.Enabled,
		caseSensitive: setting.CaseSensitive,
		keywordsRaw:   setting.Keywords,
		keywords:      keywords,
		lowerKeywords: lowerKeywords,
		caseMachine:   caseMachine,
	}
	violationMatcher.snapshot = next
	return next
}

func buildCaseSensitiveViolationMachine(keywords []string) *goahocorasick.Machine {
	if len(keywords) == 0 {
		return nil
	}
	runes := make([][]rune, 0, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		runes = append(runes, []rune(keyword))
	}
	if len(runes) == 0 {
		return nil
	}
	m := new(goahocorasick.Machine)
	if err := m.Build(runes); err != nil {
		return nil
	}
	return m
}

func buildViolationExcerpt(text string, firstKeyword string, setting operation_setting.ViolationSetting) string {
	limit := setting.MaxExcerptLength
	if limit <= 0 {
		limit = operation_setting.DefaultViolationMaxExcerptLen
	}
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	centerByte := 0
	if firstKeyword != "" {
		haystack := text
		needle := firstKeyword
		if !setting.CaseSensitive {
			haystack = strings.ToLower(text)
			needle = strings.ToLower(firstKeyword)
		}
		if idx := strings.Index(haystack, needle); idx >= 0 {
			centerByte = idx
		}
	}

	runes := []rune(text)
	centerRune := utf8.RuneCountInString(text[:safeUTF8PrefixLen(text, centerByte)])
	start := centerRune - limit/2
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(runes) {
		end = len(runes)
		start = end - limit
		if start < 0 {
			start = 0
		}
	}
	excerpt := string(runes[start:end])
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(runes) {
		excerpt += "..."
	}
	return excerpt
}

func safeUTF8PrefixLen(s string, end int) int {
	if end <= 0 {
		return 0
	}
	if end >= len(s) {
		return len(s)
	}
	for end > 0 && end < len(s) && !utf8.RuneStart(s[end]) {
		end--
	}
	return end
}

func UpdateViolationSetting(setting operation_setting.ViolationSetting) (operation_setting.ViolationSetting, error) {
	setting.Action = strings.TrimSpace(setting.Action)
	setting.ErrorCode = strings.TrimSpace(setting.ErrorCode)
	setting.ErrorMessage = strings.TrimSpace(setting.ErrorMessage)
	if err := operation_setting.ValidateViolationSetting(setting); err != nil {
		return setting, err
	}
	setting = operation_setting.NormalizeViolationSetting(setting)
	updates := map[string]string{
		"enabled":            fmt.Sprintf("%t", setting.Enabled),
		"keywords":           setting.Keywords,
		"case_sensitive":     fmt.Sprintf("%t", setting.CaseSensitive),
		"action":             setting.Action,
		"http_status_code":   fmt.Sprintf("%d", setting.HTTPStatusCode),
		"error_code":         setting.ErrorCode,
		"error_message":      setting.ErrorMessage,
		"max_excerpt_length": fmt.Sprintf("%d", setting.MaxExcerptLength),
		"ban_threshold":      fmt.Sprintf("%d", setting.BanThreshold),
	}
	for key, value := range updates {
		if err := model.UpdateOption("violation_setting."+key, value); err != nil {
			return setting, err
		}
	}
	_ = getViolationMatchSnapshot(setting)
	return setting, nil
}

func CurrentViolationSetting() operation_setting.ViolationSetting {
	return operation_setting.NormalizeViolationSetting(*operation_setting.GetViolationSetting())
}

func ValidateViolationLogDeleteTimestamp(targetTimestamp int64) error {
	if targetTimestamp <= 0 {
		return errors.New("target_timestamp is required")
	}
	return nil
}
