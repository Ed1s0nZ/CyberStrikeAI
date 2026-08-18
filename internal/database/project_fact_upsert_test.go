package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestUpsertProjectFact_preservesBodyOnEmptyUpdate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "facts.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	proj, err := db.CreateProject(&Project{Name: "test-facts"})
	if err != nil {
		t.Fatal(err)
	}

	const body = "## 攻击链\n1. step\n```http\nGET / HTTP/1.1\n```\n"
	_, err = db.UpsertProjectFact(&ProjectFact{
		ProjectID: proj.ID,
		FactKey:   "finding/sqli-login",
		Category:  "finding",
		Summary:   "SQLi on /login",
		Body:      body,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := db.UpsertProjectFact(&ProjectFact{
		ProjectID: proj.ID,
		FactKey:   "finding/sqli-login",
		Summary:   "SQLi on /login (confirmed)",
		Body:      "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Summary != "SQLi on /login (confirmed)" {
		t.Fatalf("summary=%q", updated.Summary)
	}
	if updated.Body != body {
		t.Fatalf("returned body=%q want preserved attack chain", updated.Body)
	}

	fromDB, err := db.GetProjectFactByKey(proj.ID, "finding/sqli-login")
	if err != nil {
		t.Fatal(err)
	}
	if fromDB.Body != body {
		t.Fatalf("stored body=%q want preserved", fromDB.Body)
	}
}

func TestUpsertProjectFact_replacesBodyWhenProvided(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "facts.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	proj, err := db.CreateProject(&Project{Name: "test-facts"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.UpsertProjectFact(&ProjectFact{
		ProjectID: proj.ID,
		FactKey:   "target/primary",
		Summary:   "v1",
		Body:      "old body",
	})
	if err != nil {
		t.Fatal(err)
	}

	const newBody = "new body with evidence"
	updated, err := db.UpsertProjectFact(&ProjectFact{
		ProjectID: proj.ID,
		FactKey:   "target/primary",
		Summary:   "v2",
		Body:      newBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Body != newBody {
		t.Fatalf("body=%q want %q", updated.Body, newBody)
	}
}

func TestRestoreProjectFact(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "facts.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	proj, err := db.CreateProject(&Project{Name: "restore-test"})
	if err != nil {
		t.Fatal(err)
	}
	key := "target/restore-me"
	_, err = db.UpsertProjectFact(&ProjectFact{
		ProjectID:  proj.ID,
		FactKey:    key,
		Summary:    "s",
		Confidence: "confirmed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeprecateProjectFact(proj.ID, key); err != nil {
		t.Fatal(err)
	}
	if err := db.RestoreProjectFact(proj.ID, key, "confirmed"); err != nil {
		t.Fatal(err)
	}
	f, err := db.GetProjectFactByKey(proj.ID, key)
	if err != nil {
		t.Fatal(err)
	}
	if f.Confidence != "confirmed" {
		t.Fatalf("confidence=%q want confirmed", f.Confidence)
	}
	if err := db.RestoreProjectFact(proj.ID, key, ""); err == nil {
		t.Fatal("expected error when not deprecated")
	}
}

func TestMergeFactBodyOnUpdate(t *testing.T) {
	if got := mergeFactBodyOnUpdate("", "keep"); got != "keep" {
		t.Fatalf("empty incoming: got %q", got)
	}
	if got := mergeFactBodyOnUpdate("  ", "keep"); got != "keep" {
		t.Fatalf("whitespace incoming: got %q", got)
	}
	if got := mergeFactBodyOnUpdate("new", "old"); got != "new" {
		t.Fatalf("non-empty incoming: got %q", got)
	}
}
