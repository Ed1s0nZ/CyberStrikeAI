package handler

import (
	"strings"

	"cyberstrike-ai/internal/config"
)

// effectiveProjectID 请求/队列显式项目优先，否则使用 config.project.default_project_id。
func effectiveProjectID(cfg *config.Config, explicit string) string {
	if pid := strings.TrimSpace(explicit); pid != "" {
		return pid
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.Project.DefaultProjectID)
	}
	return ""
}
