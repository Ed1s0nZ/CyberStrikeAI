package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestListProjectsSearchCaseInsensitive(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "projects-search.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	p1, err := db.CreateProject(&Project{Name: "Alpha Security Review", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := db.CreateProject(&Project{Name: "beta-scan", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProject(&Project{Name: "Other", Status: "archived"}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		search string
		status string
		want   []string
	}{
		{name: "case insensitive name", search: "alpha", status: "active", want: []string{p1.ID}},
		{name: "upper query", search: "BETA", status: "active", want: []string{p2.ID}},
		{name: "search by id substring", search: p1.ID[:8], status: "", want: []string{p1.ID}},
		{name: "status filter", search: "alpha", status: "archived", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list, err := db.ListProjects(tc.status, tc.search, 50, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(list))
			for _, p := range list {
				got = append(got, p.ID)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestProjectListSearchPatternEscapesWildcards(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "projects-like.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	p, err := db.CreateProject(&Project{Name: "100% coverage", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.ListProjects("active", "100%", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("expected exact match for literal %% query, got %#v", list)
	}
}
