package dto

const OpenAIVideoCompatibilityVersion = "openai.videos.v1"

type OpenAIVideoCompatibilityMetadata struct {
	Version          string `json:"version"`
	Seconds          int    `json:"seconds,omitempty"`
	Size             string `json:"size,omitempty"`
	ResolutionName   string `json:"resolution_name,omitempty"`
	Preset           string `json:"preset,omitempty"`
	SourceVideoID    string `json:"source_video_id,omitempty"`
	RemixedFromVideo string `json:"remixed_from_video_id,omitempty"`
	DeletedAt        int64  `json:"deleted_at,omitempty"`
}

type OpenAIVideoDeleted struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

type OpenAIVideoList struct {
	Object  string         `json:"object"`
	Data    []*OpenAIVideo `json:"data"`
	FirstID string         `json:"first_id,omitempty"`
	LastID  string         `json:"last_id,omitempty"`
	HasMore bool           `json:"has_more"`
}
