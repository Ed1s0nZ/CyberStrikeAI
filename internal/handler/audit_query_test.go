package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuditFilterFromQueryIncludesActorAndMemberFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/audit/logs?actor=operator-user&action=assign_resource&resource_type=conversation&related_user_id=user-1", nil)

	filter := auditFilterFromQuery(context)
	if filter.Actor != "operator-user" || filter.Action != "assign_resource" || filter.ResourceType != "conversation" || filter.RelatedUserID != "user-1" {
		t.Fatalf("filter = %#v", filter)
	}
}
