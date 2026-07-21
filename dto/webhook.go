package dto

type AccountWebhookUpdateRequest struct {
	URL           string `json:"url"`
	RegenerateKey bool   `json:"regenerate_key,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`
}

type AccountWebhookPublic struct {
	Configured    bool   `json:"configured"`
	URL           string `json:"url,omitempty"`
	KeyConfigured bool   `json:"key_configured"`
	Status        string `json:"status"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
	Key           string `json:"key,omitempty"`
}

type AccountWebhookTestResponse struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

type WebhookEventEnvelope struct {
	ID         string           `json:"id"`
	Object     string           `json:"object"`
	APIVersion string           `json:"api_version"`
	Type       string           `json:"type"`
	CreatedAt  int64            `json:"created_at"`
	Data       WebhookEventData `json:"data"`
}

type WebhookEventData struct {
	Object any `json:"object"`
}
