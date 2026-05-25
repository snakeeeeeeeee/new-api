package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupViolationControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSetting := *operation_setting.GetViolationSetting()
	originalOptionMap := common.OptionMap

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.OptionMap = make(map[string]string)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}, &model.ViolationLog{}))

	t.Cleanup(func() {
		*operation_setting.GetViolationSetting() = originalSetting
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.OptionMap = originalOptionMap
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func violationControllerCtx(method string, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, recorder
}

func TestViolationControllerStatusAndSetting(t *testing.T) {
	setupViolationControllerTestDB(t)

	updateCtx, updateRecorder := violationControllerCtx(http.MethodPut, "/api/violation/setting", `{
		"enabled": true,
		"keywords": "reverse\njailbreak",
		"case_sensitive": true,
		"action": "ban_after_threshold",
		"http_status_code": 451,
		"error_code": "risk",
		"error_message": "blocked",
		"max_excerpt_length": 128,
		"ban_threshold": 4
	}`)
	UpdateViolationSetting(updateCtx)
	var updateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)
	require.Contains(t, string(updateResp.Data), `"http_status_code":451`)

	statusCtx, statusRecorder := violationControllerCtx(http.MethodGet, "/api/violation/status", "")
	GetViolationStatus(statusCtx)
	var statusResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &statusResp))
	require.True(t, statusResp.Success, statusResp.Message)
	require.Contains(t, string(statusResp.Data), `"keyword_count":2`)
	require.Contains(t, string(statusResp.Data), `"ban_after_threshold"`)
}

func TestViolationControllerLogsAndDelete(t *testing.T) {
	setupViolationControllerTestDB(t)
	require.NoError(t, model.InsertViolationLog(&model.ViolationLog{
		CreatedAt:      100,
		UserId:         10,
		Username:       "risk-user",
		TokenId:        20,
		TokenName:      "risk-token",
		ModelName:      "gpt-risk",
		AggregateGroup: "ag-risk",
		RouteGroup:     "route-a",
		RequestId:      "req-risk",
		MatchedWords:   `["reverse"]`,
		Action:         "block",
	}))
	require.NoError(t, model.InsertViolationLog(&model.ViolationLog{
		CreatedAt:    200,
		UserId:       11,
		Username:     "other-user",
		MatchedWords: `["jailbreak"]`,
		Action:       "log_only",
	}))

	listCtx, listRecorder := violationControllerCtx(http.MethodGet, "/api/violation/logs?username=risk-user&p=1&page_size=10", "")
	GetViolationLogs(listCtx)
	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"total":1`)
	require.Contains(t, string(listResp.Data), `"request_id":"req-risk"`)

	deleteCtx, deleteRecorder := violationControllerCtx(http.MethodDelete, "/api/violation/logs?target_timestamp=150", "")
	DeleteViolationLogs(deleteCtx)
	var deleteResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(deleteRecorder.Body.Bytes(), &deleteResp))
	require.True(t, deleteResp.Success, deleteResp.Message)
	require.Equal(t, "1", string(deleteResp.Data))
}

func TestViolationControllerRejectsInvalidSetting(t *testing.T) {
	setupViolationControllerTestDB(t)
	ctx, recorder := violationControllerCtx(http.MethodPut, "/api/violation/setting", `{
		"enabled": true,
		"keywords": "reverse",
		"action": "delete_everything",
		"http_status_code": 200,
		"error_code": "",
		"error_message": "",
		"max_excerpt_length": 0,
		"ban_threshold": 0
	}`)
	UpdateViolationSetting(ctx)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}
