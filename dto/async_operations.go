package dto

type AdminImageTaskDispatch struct {
	DispatchID     string `json:"dispatch_id"`
	Status         string `json:"status"`
	Attempts       int    `json:"attempts"`
	NextAttemptAt  int64  `json:"next_attempt_at"`
	LockedUntil    int64  `json:"locked_until"`
	LastHTTPStatus int    `json:"last_http_status"`
	LastError      string `json:"last_error"`
	DeliveredAt    int64  `json:"delivered_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type AdminAsyncTaskItem struct {
	Task     *TaskDto                `json:"task"`
	Dispatch *AdminImageTaskDispatch `json:"dispatch,omitempty"`
}

type AdminWebhookDelivery struct {
	DeliveryID     string `json:"delivery_id"`
	EventID        string `json:"event_id"`
	EventType      string `json:"event_type"`
	ObjectID       string `json:"object_id"`
	EndpointID     string `json:"endpoint_id"`
	EndpointURL    string `json:"endpoint_url"`
	EndpointStatus string `json:"endpoint_status"`
	UserID         int    `json:"user_id"`
	Username       string `json:"username,omitempty"`
	Status         string `json:"status"`
	Attempts       int    `json:"attempts"`
	NextAttemptAt  int64  `json:"next_attempt_at"`
	LockedUntil    int64  `json:"locked_until"`
	RetryDeadline  int64  `json:"retry_deadline"`
	LastHTTPStatus int    `json:"last_http_status"`
	LastError      string `json:"last_error"`
	DeliveredAt    int64  `json:"delivered_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type AdminWebhookDeliveryAttempt struct {
	AttemptID     string `json:"attempt_id"`
	AttemptNumber int    `json:"attempt_number"`
	HTTPStatus    int    `json:"http_status"`
	Error         string `json:"error"`
	ResponseBody  string `json:"response_body,omitempty"`
	DurationMS    int64  `json:"duration_ms"`
	CreatedAt     int64  `json:"created_at"`
}

type AdminWebhookDeliveryDetail struct {
	Delivery *AdminWebhookDelivery          `json:"delivery"`
	Payload  any                            `json:"payload"`
	Attempts []*AdminWebhookDeliveryAttempt `json:"attempts"`
}
