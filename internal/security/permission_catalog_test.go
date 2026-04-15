package security

import (
	"slices"
	"testing"
)

func TestCanonicalPermissionCatalogIncludesPlatformDomains(t *testing.T) {
	permissions := CanonicalWebPermissions()

	expected := []string{
		"intel.fofa_query.execute",
		"task.batch_queue.start",
		"vulnerability.record.update",
		"webshell.command.execute",
		"file.workspace_content.update",
		"mcp.external_server.stop",
		"knowledge.search.execute",
		"skill.definition.delete",
		"agent.multi_run.execute",
		"role.agent_role.update",
		"system.super_admin.grant",
	}

	for _, permission := range expected {
		if !slices.Contains(permissions, permission) {
			t.Fatalf("expected canonical catalog to contain %q", permission)
		}
	}
}

func TestNormalizeWebPermissionsExpandsLegacyValues(t *testing.T) {
	input := []string{
		" system.config.write ",
		"security.users.manage",
		"system.super_admin",
		"",
		"knowledge.search.execute",
		"knowledge.search.execute",
	}

	got := NormalizeWebPermissions(input)
	want := []string{
		"knowledge.search.execute",
		"system.config_settings.update",
		"system.model_connectivity.test",
		"system.runtime_config.apply",
		"system.super_admin.grant",
		"system.web_user.create",
		"system.web_user.delete",
		"system.web_user.read",
		"system.web_user.update",
		"system.web_user_credential.reset",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeWebPermissions() = %#v, want %#v", got, want)
	}
}

func TestIsCanonicalWebPermissionRejectsRetiredPermission(t *testing.T) {
	if IsCanonicalWebPermission("system.super_admin") {
		t.Fatal("expected retired legacy permission to be rejected")
	}

	if IsCanonicalWebPermission("security.users.manage") {
		t.Fatal("expected retired coarse-grained permission to be rejected")
	}

	if !IsCanonicalWebPermission("system.super_admin.grant") {
		t.Fatal("expected canonical privileged permission to be accepted")
	}
}
