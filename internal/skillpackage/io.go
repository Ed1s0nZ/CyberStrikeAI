package skillpackage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxPackageFiles = 4000
	maxPackageDepth = 24
	maxScriptsDepth = 24
	defaultMaxRead  = 10 << 20
)

// SafeRelPath resolves rel inside root (no ..).
func SafeRelPath(root, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("empty resource path")
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	cleanRoot := filepath.Clean(root)
	cleanAbs := filepath.Clean(abs)
	relOut, err := filepath.Rel(cleanRoot, cleanAbs)
	if err != nil || relOut == ".." || strings.HasPrefix(relOut, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes skill directory: %q", rel)
	}
	return cleanAbs, nil
}

// ListPackageFiles lists files under a skill directory.
func ListPackageFiles(skillsRoot, skillID string) ([]PackageFileInfo, error) {
	root := SkillDir(skillsRoot, skillID)
	if _, err := ResolveSKILLPath(root); err != nil {
		return nil, fmt.Errorf("skill %q: %w", skillID, err)
	}
	var out []PackageFileInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > maxPackageDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= maxPackageFiles {
			return fmt.Errorf("skill package exceeds %d files", maxPackageFiles)
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, PackageFileInfo{
			Path:  filepath.ToSlash(rel),
			Size:  fi.Size(),
			IsDir: d.IsDir(),
		})
		return nil
	})
	return out, err
}

// ReadPackageFile reads a file relative to the skill package.
func ReadPackageFile(skillsRoot, skillID, relPath string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxRead
	}
	root := SkillDir(skillsRoot, skillID)
	abs, err := SafeRelPath(root, relPath)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}
	if fi.Size() > maxBytes {
		return readFileHead(abs, maxBytes)
	}
	return os.ReadFile(abs)
}

// WritePackageFile writes a file inside the skill package.
func WritePackageFile(skillsRoot, skillID, relPath string, content []byte) error {
	root := SkillDir(skillsRoot, skillID)
	if _, err := ResolveSKILLPath(root); err != nil {
		return fmt.Errorf("skill %q: %w", skillID, err)
	}
	abs, err := SafeRelPath(root, relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	return os.WriteFile(abs, content, 0644)
}

func readFileHead(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, max)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

func listScripts(skillsRoot, skillID string) ([]SkillScriptInfo, error) {
	root := filepath.Join(SkillDir(skillsRoot, skillID), "scripts")
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}
	var out []SkillScriptInfo
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if strings.Count(rel, string(os.PathSeparator)) >= maxScriptsDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		relSkill := filepath.Join("scripts", rel)
		full := filepath.Join(root, rel)
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			return nil
		}
		out = append(out, SkillScriptInfo{
			Name:    filepath.Base(rel),
			RelPath: filepath.ToSlash(relSkill),
			Size:    fi.Size(),
		})
		return nil
	})
	return out, err
}

func countNonDirFiles(files []PackageFileInfo) int {
	n := 0
	for _, f := range files {
		if !f.IsDir && f.Path != "SKILL.md" {
			n++
		}
	}
	return n
}
