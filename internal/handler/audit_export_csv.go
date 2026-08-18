package handler

import (
	"encoding/csv"
	"fmt"
	"time"

	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
)

func writeAuditLogsCSV(c *gin.Context, logs []*database.AuditLog) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-logs-%s.csv"`, time.Now().Format("20060102")))

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{
		"id", "created_at", "level", "category", "action", "result", "actor",
		"session_hint", "client_ip", "resource_type", "resource_id", "message",
	})
	for _, row := range logs {
		if row == nil {
			continue
		}
		_ = w.Write([]string{
			row.ID,
			row.CreatedAt.UTC().Format(time.RFC3339),
			row.Level,
			row.Category,
			row.Action,
			row.Result,
			row.Actor,
			row.SessionHint,
			row.ClientIP,
			row.ResourceType,
			row.ResourceID,
			row.Message,
		})
	}
	w.Flush()
}
