package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type AggregateClientDetection struct {
	Matched                 bool   `json:"matched"`
	ClientType              string `json:"client_type,omitempty"`
	UserAgentClaudeCLI      bool   `json:"user_agent_claude_cli"`
	XAppCLI                 bool   `json:"x_app_cli"`
	AnthropicBetaClaudeCode bool   `json:"anthropic_beta_claude_code"`
	HasMetadataUserID       bool   `json:"has_metadata_user_id"`
	RequestPath             string `json:"request_path,omitempty"`
	ModelName               string `json:"model_name,omitempty"`
}

func DetectAggregateClientType(c *gin.Context, modelName string) AggregateClientDetection {
	detection := AggregateClientDetection{ModelName: strings.TrimSpace(modelName)}
	if c == nil || c.Request == nil {
		return detection
	}
	if c.Request.URL != nil {
		detection.RequestPath = c.Request.URL.Path
	}
	userAgent := strings.ToLower(c.Request.Header.Get("User-Agent"))
	xApp := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("X-App")))
	anthropicBeta := strings.ToLower(c.Request.Header.Get("Anthropic-Beta"))
	modelName = strings.ToLower(strings.TrimSpace(modelName))

	detection.UserAgentClaudeCLI = strings.Contains(userAgent, "claude-cli/")
	detection.XAppCLI = xApp == "cli"
	detection.AnthropicBetaClaudeCode = strings.Contains(anthropicBeta, "claude-code")
	detection.HasMetadataUserID = requestBodyHasMetadataUserID(c)
	detection.Matched = detection.RequestPath == "/v1/messages" &&
		strings.HasPrefix(modelName, "claude-") &&
		detection.UserAgentClaudeCLI
	if detection.Matched {
		detection.ClientType = model.AggregateGroupClientTypeClaudeCodeCLI
	}
	return detection
}

func requestBodyHasMetadataUserID(c *gin.Context) bool {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false
	}
	body, err := storage.Bytes()
	if err != nil || len(body) == 0 {
		return false
	}
	value := gjson.GetBytes(body, "metadata.user_id")
	return value.Exists() && strings.TrimSpace(value.String()) != ""
}
