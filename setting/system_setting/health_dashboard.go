package system_setting

import (
	"os"
	"strings"
)

func ResolveHealthDashboardURL(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	if envValue := strings.TrimSpace(os.Getenv("HEALTH_DASHBOARD_URL")); envValue != "" {
		return envValue
	}
	return ""
}

var HealthDashboardURL = ResolveHealthDashboardURL("")
