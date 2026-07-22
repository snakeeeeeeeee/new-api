package dto

type VideoTaskSource struct {
	URL      string `json:"url,omitempty"`
	Provider string `json:"provider,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type VideoTaskInputRequest struct {
	Prompt          string            `json:"prompt"`
	Image           *VideoTaskSource  `json:"image,omitempty"`
	ReferenceImages []VideoTaskSource `json:"reference_images,omitempty"`
	Video           *VideoTaskSource  `json:"video,omitempty"`
}

type VideoTaskOutputRequest struct {
	Duration    *int    `json:"duration,omitempty"`
	AspectRatio *string `json:"aspect_ratio,omitempty"`
	Resolution  *string `json:"resolution,omitempty"`
}

type VideoTaskCreateRequest struct {
	Model             string                    `json:"model"`
	Operation         string                    `json:"operation"`
	Input             VideoTaskInputRequest     `json:"input"`
	Output            VideoTaskOutputRequest    `json:"output,omitempty"`
	ClientReferenceID string                    `json:"client_reference_id,omitempty"`
	Metadata          map[string]any            `json:"metadata,omitempty"`
	ProviderOptions   map[string]map[string]any `json:"provider_options,omitempty"`
}

type VideoTaskBatchQueryRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type VideoTaskAPIError struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type VideoTaskAPIErrorResponse struct {
	Error VideoTaskAPIError `json:"error"`
}

type VideoTaskResultVideo struct {
	AssetID    string `json:"asset_id"`
	Index      int    `json:"index"`
	URL        string `json:"url"`
	MimeType   string `json:"mime_type,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Temporary  bool   `json:"temporary"`
	URLAuth    string `json:"url_auth"`
}

type VideoTaskResult struct {
	Videos []VideoTaskResultVideo `json:"videos"`
}

type VideoTaskPublicError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type VideoTaskPublic struct {
	ID                string                `json:"id"`
	Object            string                `json:"object"`
	Model             string                `json:"model"`
	Operation         string                `json:"operation"`
	Status            string                `json:"status"`
	Progress          int                   `json:"progress"`
	Result            *VideoTaskResult      `json:"result"`
	Error             *VideoTaskPublicError `json:"error"`
	ClientReferenceID string                `json:"client_reference_id,omitempty"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
	CreatedAt         int64                 `json:"created_at"`
	StartedAt         *int64                `json:"started_at"`
	CompletedAt       *int64                `json:"completed_at"`
	UpdatedAt         int64                 `json:"updated_at"`
}

type VideoTaskListResponse struct {
	Object  string             `json:"object"`
	Data    []*VideoTaskPublic `json:"data"`
	FirstID string             `json:"first_id,omitempty"`
	LastID  string             `json:"last_id,omitempty"`
	HasMore bool               `json:"has_more"`
	Missing []string           `json:"missing,omitempty"`
}
