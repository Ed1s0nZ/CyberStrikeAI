package robot

import (
	"context"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

// StartDing starts DingTalk (钉钉) robot connection.
// Stub — DingTalk integration removed from this fork.
func StartDing(ctx context.Context, cfg config.RobotDingtalkConfig, handler interface{}, logger *zap.Logger) {
	logger.Info("DingTalk robot disabled in this build")
	<-ctx.Done()
}
