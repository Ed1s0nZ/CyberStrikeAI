package audit

import (
	"time"

	"go.uber.org/zap"
)

// auditRetentionPurgeInterval is how often PurgeExpired runs while the process is up (startup also purges once).
const auditRetentionPurgeInterval = time.Hour

// StartRetentionLoop periodically purges expired audit rows.
func StartRetentionLoop(s *Service, logger *zap.Logger) {
	if s == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(auditRetentionPurgeInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.PurgeExpired()
			if logger != nil {
				logger.Debug("audit retention tick completed")
			}
		}
	}()
}
