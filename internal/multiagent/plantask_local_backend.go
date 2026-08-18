package multiagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk/middlewares/plantask"
)

// localPlantaskBackend adapts eino-ext local filesystem backend for Eino plantask.
//
// plantask TaskCreate/TaskList list a directory via LsInfo, then Read using each entry's Path.
// local.LsInfo returns basenames only (e.g. ".highwatermark"), while local.Read expects a
// resolvable path — causing "file not found: .highwatermark" on the second TaskCreate.
type localPlantaskBackend struct {
	*localbk.Local
}

func newLocalPlantaskBackend(loc *localbk.Local) *localPlantaskBackend {
	if loc == nil {
		return nil
	}
	return &localPlantaskBackend{Local: loc}
}

// LsInfo lists files under req.Path and returns absolute paths suitable for subsequent Read calls.
func (l *localPlantaskBackend) LsInfo(ctx context.Context, req *plantask.LsInfoRequest) ([]plantask.FileInfo, error) {
	if l == nil || l.Local == nil {
		return nil, fmt.Errorf("plantask backend: local nil")
	}
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, fmt.Errorf("plantask backend: list path empty")
	}
	files, err := l.Local.LsInfo(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return files, nil
	}
	base := filepath.Clean(req.Path)
	out := make([]plantask.FileInfo, len(files))
	for i, f := range files {
		out[i] = f
		name := strings.TrimSpace(f.Path)
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) {
			out[i].Path = filepath.Clean(name)
			continue
		}
		out[i].Path = filepath.Join(base, name)
	}
	return out, nil
}

func (l *localPlantaskBackend) Delete(ctx context.Context, req *plantask.DeleteRequest) error {
	if l == nil || l.Local == nil || req == nil {
		return nil
	}
	p := strings.TrimSpace(req.FilePath)
	if p == "" {
		return nil
	}
	return os.Remove(p)
}
