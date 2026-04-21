package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Manager Skills manager
type Manager struct {
	skillsDir string
	logger    *zap.Logger
	skills map[string]*cachedSkill // loadskills(status)
	mu        sync.RWMutex            // protects skills map concurrent access
}

type cachedSkill struct {
	skill    *Skill
	filePath string
	modTime  int64
}

// Skill Skill definition
type Skill struct {
	Name        string // Skill name
	Description string // Skill description
	Content     string // Skill content (extracted from SKILL.md)
	Path        string // Skill path
}

// NewManager creates a new Skills manager
func NewManager(skillsDir string, logger *zap.Logger) *Manager {
	return &Manager{
		skillsDir: skillsDir,
		logger:    logger,
		skills:    make(map[string]*cachedSkill),
	}
}

// LoadSkill loadskill
func (m *Manager) LoadSkill(skillName string) (*Skill, error) {
	// build skill path
	skillPath := filepath.Join(m.skillsDir, skillName)

	// check if directory exists
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		m.InvalidateSkill(skillName)
		return nil, fmt.Errorf("skill %s not found", skillName)
	}

	// find skill filestatus
	skillFile, err := m.resolveSkillFile(skillPath)
	if err != nil {
		m.InvalidateSkill(skillName)
		return nil, err
	}
	fileInfo, err := os.Stat(skillFile)
	if err != nil {
		m.InvalidateSkill(skillName)
		return nil, fmt.Errorf("failed to stat skill file: %w", err)
	}
	modTime := fileInfo.ModTime().UnixNano()

	// first try read lock cache hit(file path)
	m.mu.RLock()
	if cached, exists := m.skills[skillName]; exists &&
		cached.filePath == skillFile &&
		cached.modTime == modTime {
		m.mu.RUnlock()
		return cached.skill, nil
	}
	m.mu.RUnlock()

	// read skill file
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	// parseskill
	skill := m.parseSkillContent(string(content), skillName, skillPath)

	// use write lock to update cache
	m.mu.Lock()
	m.skills[skillName] = &cachedSkill{
		skill:    skill,
		filePath: skillFile,
		modTime:  modTime,
	}
	m.mu.Unlock()

	return skill, nil
}

// LoadSkills loadskills
func (m *Manager) LoadSkills(skillNames []string) ([]*Skill, error) {
	var skills []*Skill
	var errors []string

	for _, name := range skillNames {
		skill, err := m.LoadSkill(name)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to load skill %s: %v", name, err))
			m.logger.Warn("loadskill", zap.String("skill", name), zap.Error(err))
			continue
		}
		skills = append(skills, skill)
	}

	if len(errors) > 0 && len(skills) == 0 {
		return nil, fmt.Errorf("failed to load any skills: %s", strings.Join(errors, "; "))
	}

	return skills, nil
}

// ListSkills list all available skills
func (m *Manager) ListSkills() ([]string, error) {
	if _, err := os.Stat(m.skillsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		// check if SKILL.md file exists
		skillFile := filepath.Join(m.skillsDir, skillName, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			skills = append(skills, skillName)
			continue
		}

		// filename
		alternatives := []string{
			filepath.Join(m.skillsDir, skillName, "skill.md"),
			filepath.Join(m.skillsDir, skillName, "README.md"),
			filepath.Join(m.skillsDir, skillName, "readme.md"),
		}
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); err == nil {
				skills = append(skills, skillName)
				break
			}
		}
	}

	return skills, nil
}

func (m *Manager) resolveSkillFile(skillPath string) (string, error) {
	// filename
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil {
		return skillFile, nil
	}

	// filename
	alternatives := []string{
		filepath.Join(skillPath, "skill.md"),
		filepath.Join(skillPath, "README.md"),
		filepath.Join(skillPath, "readme.md"),
	}
	for _, alt := range alternatives {
		if _, err := os.Stat(alt); err == nil {
			return alt, nil
		}
	}

	return "", fmt.Errorf("skill file not found for %s", filepath.Base(skillPath))
}

// InvalidateSkill invalidate specified skill cache
func (m *Manager) InvalidateSkill(skillName string) {
	m.mu.Lock()
	delete(m.skills, skillName)
	m.mu.Unlock()
}

// InvalidateAll clearskill
func (m *Manager) InvalidateAll() {
	m.mu.Lock()
	m.skills = make(map[string]*cachedSkill)
	m.mu.Unlock()
}

// parseSkillContent parseskill
// YAML front matterformat,goskills
func (m *Manager) parseSkillContent(content, skillName, skillPath string) *Skill {
	skill := &Skill{
		Name: skillName,
		Path: skillPath,
	}

	// check if has YAML front matter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			// parsefront matter(simple implementation, only extract name and description)
			frontMatter := parts[1]
			lines := strings.Split(frontMatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					name = strings.Trim(name, `"'"`)
					if name != "" {
						skill.Name = name
					}
				} else if strings.HasPrefix(line, "description:") {
					desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					desc = strings.Trim(desc, `"'"`)
					skill.Description = desc
				}
			}
			// remaining part is content
			if len(parts) == 3 {
				skill.Content = strings.TrimSpace(parts[2])
			}
		} else {
			// no front matter, entire content is skill content
			skill.Content = content
		}
	} else {
		// no front matter, entire content is skill content
		skill.Content = content
	}

	// if content empty, use description as content
	if skill.Content == "" {
		skill.Content = skill.Description
	}

	return skill
}

// GetSkillContent get complete skill content (for injection into system prompt)
func (m *Manager) GetSkillContent(skillNames []string) (string, error) {
	skills, err := m.LoadSkills(skillNames)
	if err != nil {
		return "", err
	}

	if len(skills) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## available Skills\n\n")
	builder.WriteString("before executing tasks, please read the following skills content, which contains relevant professional knowledge and methods:\n\n")

	for _, skill := range skills {
		builder.WriteString(fmt.Sprintf("### Skill: %s\n", skill.Name))
		if skill.Description != "" {
			builder.WriteString(fmt.Sprintf("**description**: %s\n\n", skill.Description))
		}
		builder.WriteString(skill.Content)
		builder.WriteString("\n\n---\n\n")
	}

	return builder.String(), nil
}
