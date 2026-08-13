package promptaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditFindsSkillQualityWarnings(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "agents", "orchestrator.md"), "---\nname: test\n---\n\n# Agent\n")
	mustWrite(t, filepath.Join(root, "roles", "test.yaml"), "name: test\nuser_prompt: ok\n")
	mustWrite(t, filepath.Join(root, "skills", "demo", "SKILL.md"), `---
name: demo
description: demo skill
---

# Demo

Skill Name: other-name

Read `+"`references/missing.md`"+`.
`)

	report, err := Audit(root, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Skills != 1 || report.Summary.InvalidSkills != 0 {
		t.Fatalf("unexpected skill summary: %+v", report.Summary)
	}
	if report.Summary.InternalNameMismatch != 1 {
		t.Fatalf("expected one internal name mismatch: %+v", report.Summary)
	}
	if report.Summary.BrokenReferences != 1 {
		t.Fatalf("expected one broken reference: %+v", report.Summary)
	}
	for _, section := range []string{"when_to_use", "preconditions", "procedure", "stop_conditions", "output"} {
		if report.Summary.MissingSections[section] != 1 {
			t.Fatalf("expected missing %s: %+v", section, report.Summary.MissingSections)
		}
	}
}

func TestAuditAcceptsCompleteSkill(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "agents", "orchestrator.md"), "---\nname: test\n---\n\n# Agent\n")
	mustWrite(t, filepath.Join(root, "roles", "test.yaml"), "name: test\nuser_prompt: ok\n")
	mustWrite(t, filepath.Join(root, "skills", "demo", "SKILL.md"), `---
name: demo
description: complete demo skill
---

# Demo

## When to use
x
## Preconditions
x
## Procedure
x
## Stop conditions
x
## Output
x
`)

	report, err := Audit(root, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.InvalidSkills != 0 || report.Summary.BrokenReferences != 0 || report.Summary.InternalNameMismatch != 0 {
		t.Fatalf("unexpected warnings: %+v", report.Summary)
	}
	for name, count := range report.Summary.MissingSections {
		if count != 0 {
			t.Fatalf("unexpected missing section %s=%d", name, count)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
