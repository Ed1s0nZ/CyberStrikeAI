package security

import (
	"net/http"
	"testing"
)

func TestWorkflowPackageRoutesHaveExplicitWorkflowPermissions(t *testing.T) {
	if got := permissionForRequest(http.MethodGet, "/api/workflows/:id/package"); got != "workflow:read" {
		t.Fatalf("export permission=%q", got)
	}
	for _, path := range []string{"/api/workflow-package-inspections", "/api/workflow-package-inspections/:inspectionId", "/api/workflow-package-imports", "/api/workflow-package-imports/:importId"} {
		if got := permissionForRequest(http.MethodGet, path); got != "workflow:write" {
			t.Fatalf("%s permission=%q", path, got)
		}
	}
	if !isProcessGlobalMutationPath("/workflow-package-imports") || !isProcessGlobalMutationPath("/workflow-package-inspections") {
		t.Fatal("package mutations must require all-resource scope")
	}
}
