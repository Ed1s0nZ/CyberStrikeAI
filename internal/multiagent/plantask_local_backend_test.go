package multiagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/plantask"
)

func TestLocalPlantaskBackendLsInfoReturnsFullPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	baseDir := t.TempDir()

	loc, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	be := newLocalPlantaskBackend(loc)

	hwPath := filepath.Join(baseDir, ".highwatermark")
	if err := os.WriteFile(hwPath, []byte("1"), 0o600); err != nil {
		t.Fatalf("write highwatermark: %v", err)
	}

	files, err := be.LsInfo(ctx, &plantask.LsInfoRequest{Path: baseDir})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != hwPath {
		t.Fatalf("expected full path %q, got %q", hwPath, files[0].Path)
	}

	content, err := be.Read(ctx, &plantask.ReadRequest{FilePath: files[0].Path})
	if err != nil {
		t.Fatalf("Read via LsInfo path: %v", err)
	}
	if content.Content != "1" {
		t.Fatalf("unexpected content: %q", content.Content)
	}
}

func TestLocalPlantaskBackendSecondTaskCreateScenario(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	baseDir := t.TempDir()

	loc, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	be := newLocalPlantaskBackend(loc)

	hwPath := filepath.Join(baseDir, ".highwatermark")
	if err := loc.Write(ctx, &filesystem.WriteRequest{FilePath: hwPath, Content: "1"}); err != nil {
		t.Fatalf("seed highwatermark: %v", err)
	}

	files, err := be.LsInfo(ctx, &plantask.LsInfoRequest{Path: baseDir})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	var hwFile string
	for _, f := range files {
		if filepath.Base(f.Path) == ".highwatermark" {
			hwFile = f.Path
			break
		}
	}
	if hwFile == "" {
		t.Fatal("highwatermark not listed")
	}
	if _, err := be.Read(ctx, &plantask.ReadRequest{FilePath: hwFile}); err != nil {
		t.Fatalf("Read highwatermark (second TaskCreate path): %v", err)
	}
}
