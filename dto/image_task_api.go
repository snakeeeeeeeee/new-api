package dto

type ImageTaskSource struct {
	URL string `json:"url"`
}

type ImageTaskInputRequest struct {
	Prompt string            `json:"prompt"`
	Images []ImageTaskSource `json:"images,omitempty"`
	Mask   *ImageTaskSource  `json:"mask,omitempty"`
}

type ImageTaskOutputRequest struct {
	Count       *int    `json:"count,omitempty"`
	Size        *string `json:"size,omitempty"`
	Quality     *string `json:"quality,omitempty"`
	Format      *string `json:"format,omitempty"`
	Compression *int    `json:"compression,omitempty"`
	Background  *string `json:"background,omitempty"`
}

type ImageTaskCreateRequest struct {
	Model             string                 `json:"model"`
	Operation         string                 `json:"operation"`
	Input             ImageTaskInputRequest  `json:"input"`
	Output            ImageTaskOutputRequest `json:"output,omitempty"`
	ClientReferenceID string                 `json:"client_reference_id,omitempty"`
	Metadata          map[string]any         `json:"metadata,omitempty"`
}

type ImageTaskBatchQueryRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type ImageTaskAPIError struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type ImageTaskAPIErrorResponse struct {
	Error ImageTaskAPIError `json:"error"`
}

type ImageTaskResultImage struct {
	AssetID       string `json:"asset_id"`
	URL           string `json:"url"`
	MimeType      string `json:"mime_type,omitempty"`
	Format        string `json:"format,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	SizeBytes     int64  `json:"size_bytes,omitempty"`
	Filename      string `json:"filename,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageTaskResult struct {
	Images []ImageTaskResultImage `json:"images"`
	Output map[string]any         `json:"output,omitempty"`
}

type ImageTaskPublicError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type ImageTaskPublic struct {
	ID                string                `json:"id"`
	Object            string                `json:"object"`
	Model             string                `json:"model"`
	Operation         string                `json:"operation"`
	Status            string                `json:"status"`
	Progress          int                   `json:"progress"`
	Result            *ImageTaskResult      `json:"result"`
	Usage             map[string]any        `json:"usage"`
	Error             *ImageTaskPublicError `json:"error"`
	ClientReferenceID string                `json:"client_reference_id,omitempty"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
	CreatedAt         int64                 `json:"created_at"`
	StartedAt         *int64                `json:"started_at"`
	CompletedAt       *int64                `json:"completed_at"`
	UpdatedAt         int64                 `json:"updated_at"`
}

type ImageTaskListResponse struct {
	Object  string             `json:"object"`
	Data    []*ImageTaskPublic `json:"data"`
	FirstID string             `json:"first_id,omitempty"`
	LastID  string             `json:"last_id,omitempty"`
	HasMore bool               `json:"has_more"`
	Missing []string           `json:"missing,omitempty"`
}

type ImageUploadPublic struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Field     string `json:"field"`
	URL       string `json:"url"`
	MimeType  string `json:"mime_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Format    string `json:"format,omitempty"`
	Temporary bool   `json:"temporary"`
}

type ImageUploadListResponse struct {
	Object string              `json:"object"`
	Data   []ImageUploadPublic `json:"data"`
	Images []string            `json:"images"`
	Mask   *string             `json:"mask"`
}
