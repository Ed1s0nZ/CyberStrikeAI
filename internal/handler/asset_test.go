package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestAssetListPaginatesWithinProject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "asset-list-pagination.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	project, err := db.CreateProject(&database.Project{Name: "Paged Project", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	otherProject, err := db.CreateProject(&database.Project{Name: "Other Project", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}

	assets := make([]*database.Asset, 0, 8)
	for i := 1; i <= 7; i++ {
		assets = append(assets, &database.Asset{
			ProjectID: project.ID,
			IP:        fmt.Sprintf("192.0.2.%d", i),
			Port:      80,
			Protocol:  "http",
		})
	}
	assets = append(assets, &database.Asset{
		ProjectID: otherProject.ID,
		IP:        "198.51.100.1",
		Port:      443,
		Protocol:  "https",
	})
	if _, err := db.UpsertAssets(assets, "", true); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/api/assets", NewAssetHandler(db, zap.NewNop()).List)
	request := httptest.NewRequest(http.MethodGet, "/api/assets?project_id="+project.ID+"&page=2&page_size=3", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Assets     []*database.Asset `json:"assets"`
		Total      int               `json:"total"`
		Page       int               `json:"page"`
		PageSize   int               `json:"page_size"`
		TotalPages int               `json:"total_pages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 7 || payload.Page != 2 || payload.PageSize != 3 || payload.TotalPages != 3 {
		t.Fatalf("unexpected pagination: total=%d page=%d page_size=%d total_pages=%d",
			payload.Total, payload.Page, payload.PageSize, payload.TotalPages)
	}
	if len(payload.Assets) != 3 {
		t.Fatalf("expected 3 assets on page 2, got %d", len(payload.Assets))
	}
	for _, asset := range payload.Assets {
		if asset.ProjectID != project.ID {
			t.Fatalf("asset from another project leaked into page: %#v", asset)
		}
	}
}
