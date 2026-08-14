package skillpackage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillDir returns the absolute path to a skill package directory.
func SkillDir(skillsRoot, skillID string) string {
	return filepath.Join(skillsRoot, skillID)
}

// ResolveSKILLPath returns SKILL.md path or error if missing.
func ResolveSKILLPath(skillPath string) (string, error) {
	md := filepath.Join(skillPath, "SKILL.md")
	if st, err := os.Stat(md); err != nil || st.IsDir() {
		return "", fmt.Errorf("missing SKILL.md in %q (Agent Skills standard)", filepath.Base(skillPath))
	}
	return md, nil
}

// SkillsRootFromConfig resolves cfg.SkillsDir relative to the config file directory.
func SkillsRootFromConfig(skillsDir string, configPath string) string {
	if skillsDir == "" {
		skillsDir = "skills"
	}
	configDir := filepath.Dir(configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}
	return skillsDir
}

// DirLister lists skill package directory names under SkillsRoot.
type DirLister struct {
	SkillsRoot string
}

// ListSkills returns skill package directory names that contain SKILL.md.
func (d DirLister) ListSkills() ([]string, error) {
	return ListSkillDirNames(d.SkillsRoot)
}

// ListSkillDirNames returns subdirectory names under skillsRoot that contain SKILL.md.
func ListSkillDirNames(skillsRoot string) ([]string, error) {
	if _, err := os.Stat(skillsRoot); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, fmt.Errorf("read skills directory: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillPath := filepath.Join(skillsRoot, entry.Name())
		if _, err := ResolveSKILLPath(skillPath); err == nil {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
