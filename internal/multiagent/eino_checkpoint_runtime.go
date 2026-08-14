package multiagent

import (
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type einoCheckpointRuntime struct {
	Store        *fileCheckPointStore
	CheckPointID string
}

func newEinoCheckpointRuntime(checkpointDir, conversationID, orchMode string, logger *zap.Logger) *einoCheckpointRuntime {
	checkpointDir = strings.TrimSpace(checkpointDir)
	if checkpointDir == "" {
		return nil
	}
	cpDir := filepath.Join(checkpointDir, sanitizeEinoPathSegment(conversationID))
	store, err := newFileCheckPointStore(cpDir)
	if err != nil {
		if logger != nil {
			logger.Warn("eino checkpoint store disabled", zap.String("dir", cpDir), zap.Error(err))
		}
		return nil
	}
	checkPointID := buildEinoCheckpointID(orchMode)
	if logger != nil {
		logger.Info("eino runner: checkpoint store enabled",
			zap.String("dir", cpDir),
			zap.String("checkPointID", checkPointID))
	}
	return &einoCheckpointRuntime{
		Store:        store,
		CheckPointID: checkPointID,
	}
}
