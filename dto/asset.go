package dto

type AssetDto struct {
	ID           int64          `json:"id"`
	AssetID      string         `json:"asset_id"`
	TaskID       string         `json:"task_id"`
	TaskRecordID int64          `json:"task_record_id"`
	AssetIndex   int            `json:"asset_index"`
	UserID       int            `json:"user_id"`
	Group        string         `json:"group"`
	ChannelID    int            `json:"channel_id"`
	Platform     string         `json:"platform"`
	Action       string         `json:"action"`
	Model        string         `json:"model"`
	AssetType    string         `json:"asset_type"`
	URL          string         `json:"url"`
	ThumbnailURL string         `json:"thumbnail_url"`
	MimeType     string         `json:"mime_type"`
	Filename     string         `json:"filename"`
	SizeBytes    int64          `json:"size_bytes"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	DurationMS   int64          `json:"duration_ms"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
	DeletedAt    int64          `json:"deleted_at"`
	Username     string         `json:"username,omitempty"`
}

type AssetBatchURLRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

type AssetURLItem struct {
	AssetID string `json:"asset_id"`
	TaskID  string `json:"task_id"`
	Type    string `json:"asset_type"`
	URL     string `json:"url"`
}

type AssetBlockRequest struct {
	IsBlocked bool `json:"is_blocked"`
}

type AssetKeyDto struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Key        string `json:"key"`
	Status     int    `json:"status"`
	Scopes     string `json:"scopes"`
	AllowIPs   string `json:"allow_ips"`
	ExpiredAt  int64  `json:"expired_at"`
	LastUsedAt int64  `json:"last_used_at"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

type AssetKeyCreateRequest struct {
	Name      string `json:"name"`
	AllowIPs  string `json:"allow_ips"`
	ExpiredAt int64  `json:"expired_at"`
}

type AssetKeyStatusRequest struct {
	Status int `json:"status"`
}

type AssetAPIItem struct {
	Object       string         `json:"object"`
	ID           string         `json:"id"`
	TaskID       string         `json:"task_id"`
	Index        int            `json:"index"`
	Type         string         `json:"type"`
	URL          string         `json:"url"`
	ThumbnailURL string         `json:"thumbnail_url,omitempty"`
	MimeType     string         `json:"mime_type,omitempty"`
	Filename     string         `json:"filename,omitempty"`
	SizeBytes    int64          `json:"size_bytes,omitempty"`
	Width        int            `json:"width,omitempty"`
	Height       int            `json:"height,omitempty"`
	DurationMS   int64          `json:"duration_ms,omitempty"`
	Model        string         `json:"model,omitempty"`
	Platform     string         `json:"platform,omitempty"`
	Action       string         `json:"action,omitempty"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
}

type AssetAPIListResponse struct {
	Object   string          `json:"object"`
	Data     []*AssetAPIItem `json:"data"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
	HasMore  bool            `json:"has_more"`
}

type AssetAPIQueryRequest struct {
	AssetType      string   `json:"asset_type"`
	TaskID         string   `json:"task_id"`
	Model          string   `json:"model"`
	Platform       string   `json:"platform"`
	Action         string   `json:"action"`
	StartTimestamp int64    `json:"start_timestamp"`
	EndTimestamp   int64    `json:"end_timestamp"`
	Page           int      `json:"page"`
	PageSize       int      `json:"page_size"`
	AssetIDs       []string `json:"asset_ids"`
}
