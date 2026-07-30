package dto

type MediaUploadFileRequest struct {
	ClientID  string `json:"client_id,omitempty"`
	Kind      string `json:"kind"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

type MediaUploadCreateRequest struct {
	Files []MediaUploadFileRequest `json:"files"`
}

type MediaUploadCompleteRequest struct {
	UploadIDs []string `json:"upload_ids"`
}

type MediaUploadSession struct {
	ID        string            `json:"id"`
	ClientID  string            `json:"client_id,omitempty"`
	Kind      string            `json:"kind"`
	Method    string            `json:"method"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expires_at"`
}

type MediaUploadSessionListResponse struct {
	Object string               `json:"object"`
	Data   []MediaUploadSession `json:"data"`
}

type MediaUploadPublic struct {
	ID        string `json:"id"`
	ClientID  string `json:"client_id,omitempty"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	Temporary bool   `json:"temporary"`
	ExpiresAt int64  `json:"expires_at"`
}

type MediaUploadListResponse struct {
	Object string              `json:"object"`
	Data   []MediaUploadPublic `json:"data"`
}
