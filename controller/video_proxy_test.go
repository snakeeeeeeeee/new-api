package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeVideoContentResolver struct {
	output  relaycommon.VideoOutput
	headers http.Header
}

func (f *fakeVideoContentResolver) ResolveVideoContent(_ context.Context, _ *model.Channel, _ *model.Task, output relaycommon.VideoOutput, headers http.Header) (*http.Response, error) {
	f.output = output
	f.headers = headers.Clone()
	return &http.Response{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Content-Type":  []string{"video/mp4"},
			"Content-Range": []string{"bytes 0-3/8"},
		},
		Body: io.NopCloser(strings.NewReader("data")),
	}, nil
}

func TestResolveVideoProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		baseURL  string
		wantURL  string
		wantAuth bool
		wantErr  bool
	}{
		{
			name:     "relative path uses channel origin",
			rawURL:   "/v1/videos/upstream-id/content",
			baseURL:  "https://upstream.example/api",
			wantURL:  "https://upstream.example/v1/videos/upstream-id/content",
			wantAuth: true,
		},
		{
			name:     "relative segment uses channel base path",
			rawURL:   "videos/upstream-id/content",
			baseURL:  "https://upstream.example/api",
			wantURL:  "https://upstream.example/api/videos/upstream-id/content",
			wantAuth: true,
		},
		{
			name:     "same origin absolute URL gets auth",
			rawURL:   "https://upstream.example:443/content.mp4",
			baseURL:  "https://upstream.example",
			wantURL:  "https://upstream.example:443/content.mp4",
			wantAuth: true,
		},
		{
			name:     "absolute CDN URL does not get auth",
			rawURL:   "https://cdn.example/video.mp4",
			baseURL:  "https://upstream.example",
			wantURL:  "https://cdn.example/video.mp4",
			wantAuth: false,
		},
		{
			name:    "scheme relative URL is rejected",
			rawURL:  "//cdn.example/video.mp4",
			baseURL: "https://upstream.example",
			wantErr: true,
		},
		{
			name:    "non HTTP URL is rejected",
			rawURL:  "file:///tmp/video.mp4",
			baseURL: "https://upstream.example",
			wantErr: true,
		},
		{
			name:    "embedded credentials are rejected",
			rawURL:  "https://key@upstream.example/video.mp4",
			baseURL: "https://upstream.example",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, withAuth, err := resolveVideoProxyURL(tt.rawURL, tt.baseURL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, resolved.String())
			assert.Equal(t, tt.wantAuth, withAuth)
		})
	}
}

func TestVideoProxyClientStripsAuthorizationOnCrossOriginRedirect(t *testing.T) {
	client := &http.Client{}
	authorizedOrigin, err := url.Parse("https://upstream.example/video.mp4")
	require.NoError(t, err)
	redirectClient := videoProxyClientForOrigin(client, authorizedOrigin)

	req := httptest.NewRequest(http.MethodGet, "https://cdn.example/video.mp4", nil)
	req.Header.Set("Authorization", "Bearer channel-secret")
	require.NoError(t, redirectClient.CheckRedirect(req, nil))

	assert.Empty(t, req.Header.Get("Authorization"))
}

func TestStreamVideoContentForwardsRangeAndSameOriginAuth(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	var gotRange, gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 301, Type: 48, Key: "channel-secret", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled, Name: "video-test",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset_test/content", nil)
	ctx.Request.Header.Set("Range", "bytes=0-3")
	task := &model.Task{TaskID: "task_stream", ChannelId: 301, Status: model.TaskStatusSuccess}
	streamVideoContent(ctx, task, upstream.URL+"/video.mp4")

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "bytes=0-3", gotRange)
	assert.Equal(t, "Bearer channel-secret", gotAuthorization)
	assert.Equal(t, "bytes 0-3/10", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "private, max-age=600", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "data", recorder.Body.String())
}

func TestStreamVideoContentMapsUpstreamExpiration(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 302, Type: 48, Key: "channel-secret", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled, Name: "video-expired",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset_expired/content", nil)
	task := &model.Task{TaskID: "task_expired", ChannelId: 302, Status: model.TaskStatusSuccess}
	streamVideoContent(ctx, task, upstream.URL+"/video.mp4")

	assert.Equal(t, http.StatusGone, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"type":"resource_expired"`)
}

func TestStreamVideoContentUsesProviderResolver(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	baseURL := "https://provider.example"
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 303, Type: 48, Key: "channel-secret", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled, Name: "video-resolver",
	}).Error)

	fake := &fakeVideoContentResolver{}
	originalResolver := getVideoContentResolver
	getVideoContentResolver = func(_ constant.TaskPlatform) channel.VideoContentResolver { return fake }
	t.Cleanup(func() { getVideoContentResolver = originalResolver })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset_resolver/content", nil)
	ctx.Request.Header.Set("Range", "bytes=0-3")
	task := &model.Task{TaskID: "task_resolver", ChannelId: 303, Platform: "48", Status: model.TaskStatusSuccess}
	asset := &model.Asset{
		AssetIndex: 1, URL: "opaque-provider-reference",
		Metadata: model.AssetMetadata{"provider_reference": "provider-video-1", "resolver": "content-api"},
	}

	streamVideoContent(ctx, task, asset.URL, asset)

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "data", recorder.Body.String())
	assert.Equal(t, "bytes=0-3", fake.headers.Get("Range"))
	assert.Equal(t, "provider-video-1", fake.output.ProviderReference)
	assert.Equal(t, "content-api", fake.output.Resolver)
}

func TestWriteVideoDataURLSupportsRange(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset_data/content", nil)
	ctx.Request.Header.Set("Range", "bytes=2-4")

	require.NoError(t, writeVideoDataURL(ctx, "data:video/mp4;base64,YWJjZGVm"))

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "cde", recorder.Body.String())
	assert.Equal(t, "bytes 2-4/6", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "private, max-age=600", recorder.Header().Get("Cache-Control"))
}

func newVideoProxyTaskContext(t *testing.T, role int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctxRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(ctxRecorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_other/content", nil)
	ctx.Set("role", role)
	ctx.Set("id", 1)
	return ctx
}

func TestGetVideoProxyTaskAdminCanReadAnyUserTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_other",
		UserId:    2,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleAdminUser)
	task, exists, err := getVideoProxyTask(ctx, 1, "task_other")

	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, 2, task.UserId)
}

func TestGetVideoProxyTaskCommonUserCannotReadOtherUserTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_other",
		UserId:    2,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleCommonUser)
	_, exists, err := getVideoProxyTask(ctx, 1, "task_other")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetVideoProxyTaskCommonUserCannotReadBlockedTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_blocked",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		IsBlocked: true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleCommonUser)
	_, exists, err := getVideoProxyTask(ctx, 1, "task_blocked")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetVideoProxyTaskAdminCanReadBlockedTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_blocked",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		IsBlocked: true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleAdminUser)
	task, exists, err := getVideoProxyTask(ctx, 1, "task_blocked")

	require.NoError(t, err)
	require.True(t, exists)
	assert.True(t, task.IsBlocked)
}

func TestUpdateTaskBlockStatusTogglesTaskAndRecordsLog(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_toggle",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/task/task_toggle/block", bytes.NewReader([]byte(`{"is_blocked":true}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_toggle"}}
	ctx.Set("id", 99)

	UpdateTaskBlockStatus(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_toggle").First(&task).Error)
	assert.True(t, task.IsBlocked)

	var log model.Log
	require.NoError(t, db.Where("user_id = ? and type = ?", 1, model.LogTypeManage).First(&log).Error)
	assert.Contains(t, log.Content, "屏蔽任务记录")
	assert.Contains(t, log.Content, "任务ID：task_toggle")
}
