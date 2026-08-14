package multiagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fileCheckPointStore implements adk.CheckPointStore with one file per checkpoint id.
type fileCheckPointStore struct {
	dir string
}

func newFileCheckPointStore(baseDir string) (*fileCheckPointStore, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, fmt.Errorf("checkpoint base dir empty")
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &fileCheckPointStore{dir: abs}, nil
}

func (s *fileCheckPointStore) path(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("checkpoint id empty")
	}
	if strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid checkpoint id")
	}
	return filepath.Join(s.dir, id+".ckpt"), nil
}

func (s *fileCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	_ = ctx
	p, err := s.path(checkPointID)
	if err != nil {
		return nil, false, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

func (s *fileCheckPointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	_ = ctx
	p, err := s.path(checkPointID)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, checkPoint, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
