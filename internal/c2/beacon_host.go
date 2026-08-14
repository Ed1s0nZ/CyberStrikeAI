package c2

import (
	"strings"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// ResolveBeaconDialHost 决定植入端应连接的主机名（不含端口）。
// 优先级：explicitOverride > 监听器 config_json 中的 callback_host > bind_host（0.0.0.0/::/空 时 detectExternalIP，失败则 127.0.0.1）。
func ResolveBeaconDialHost(listener *database.C2Listener, explicitOverride string, logger *zap.Logger, listenerID string) string {
	if h := strings.TrimSpace(explicitOverride); h != "" {
		return h
	}
	cfg := &ListenerConfig{}
	if listener != nil && listener.ConfigJSON != "" {
		_ = parseJSON(listener.ConfigJSON, cfg)
	}
	if h := strings.TrimSpace(cfg.CallbackHost); h != "" {
		return h
	}
	if listener == nil {
		return "127.0.0.1"
	}
	host := strings.TrimSpace(listener.BindHost)
	if host == "0.0.0.0" || host == "" || host == "::" {
		host = detectExternalIP()
		if host == "" {
			if logger != nil {
				logger.Warn("listener binds 0.0.0.0 but no external IP detected, falling back to 127.0.0.1; set callback_host or pass explicit host",
					zap.String("listener_id", listenerID))
			}
			return "127.0.0.1"
		}
	}
	return host
}
