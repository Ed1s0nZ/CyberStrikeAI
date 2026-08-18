package audit

import "github.com/gin-gonic/gin"

// RecordAction writes a platform audit row with common defaults.
func (s *Service) RecordAction(c *gin.Context, category, action, result, message, resourceType, resourceID string, detail map[string]interface{}) {
	if s == nil {
		return
	}
	s.Record(c, Entry{
		Category:     category,
		Action:       action,
		Result:       result,
		Message:      message,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Detail:       detail,
	})
}

// RecordOK is a shorthand for successful operations.
func (s *Service) RecordOK(c *gin.Context, category, action, message, resourceType, resourceID string, detail map[string]interface{}) {
	s.RecordAction(c, category, action, "success", message, resourceType, resourceID, detail)
}

// RecordFail is a shorthand for failed operations.
func (s *Service) RecordFail(c *gin.Context, category, action, message string, detail map[string]interface{}) {
	s.RecordAction(c, category, action, "failure", message, "", "", detail)
}
