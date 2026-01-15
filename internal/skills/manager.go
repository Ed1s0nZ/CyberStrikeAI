package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Manager Skills管理器
type Manager struct {
	skillsDir string
	logger    *zap.Logger
	skills    map[string]*Skill // 缓存已加载的skills
	mu        sync.RWMutex      // 保护skills map的并发访问
}

// Skill Skill定义
type Skill struct {
	Name        string // Skill名称
	Description string // Skill描述
	Content     string // Skill内容（从SKILL.md中提取）
	Path        string // Skill路径
}

// NewManager 创建新的Skills管理器
func NewManager(skillsDir string, logger *zap.Logger) *Manager {
	return &Manager{
		skillsDir: skillsDir,
		logger:    logger,
		skills:    make(map[string]*Skill),
	}
}

// LoadSkill 加载单个skill
func (m *Manager) LoadSkill(skillName string) (*Skill, error) {
	// 先尝试读锁检查缓存
	m.mu.RLock()
	if skill, exists := m.skills[skillName]; exists {
		m.mu.RUnlock()
		return skill, nil
	}
	m.mu.RUnlock()

	// 构建skill路径
	skillPath := filepath.Join(m.skillsDir, skillName)
	
	// 检查目录是否存在
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("skill %s not found", skillName)
	}

	// 查找SKILL.md文件
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		// 尝试其他可能的文件名
		alternatives := []string{
			filepath.Join(skillPath, "skill.md"),
			filepath.Join(skillPath, "README.md"),
			filepath.Join(skillPath, "readme.md"),
		}
		found := false
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); err == nil {
				skillFile = alt
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("skill file not found for %s", skillName)
		}
	}

	// 读取skill文件
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	// 解析skill内容
	skill := m.parseSkillContent(string(content), skillName, skillPath)
	
	// 使用写锁缓存skill（双重检查，避免重复加载）
	m.mu.Lock()
	// 再次检查，可能其他goroutine已经加载了
	if existing, exists := m.skills[skillName]; exists {
		m.mu.Unlock()
		return existing, nil
	}
	m.skills[skillName] = skill
	m.mu.Unlock()

	return skill, nil
}

// LoadSkills 批量加载skills
func (m *Manager) LoadSkills(skillNames []string) ([]*Skill, error) {
	var skills []*Skill
	var errors []string

	for _, name := range skillNames {
		skill, err := m.LoadSkill(name)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to load skill %s: %v", name, err))
			m.logger.Warn("加载skill失败", zap.String("skill", name), zap.Error(err))
			continue
		}
		skills = append(skills, skill)
	}

	if len(errors) > 0 && len(skills) == 0 {
		return nil, fmt.Errorf("failed to load any skills: %s", strings.Join(errors, "; "))
	}

	return skills, nil
}

// ListSkills 列出所有可用的skills
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
		// 检查是否有SKILL.md文件
		skillFile := filepath.Join(m.skillsDir, skillName, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			skills = append(skills, skillName)
			continue
		}

		// 尝试其他可能的文件名
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

// parseSkillContent 解析skill内容
// 支持YAML front matter格式，类似goskills
func (m *Manager) parseSkillContent(content, skillName, skillPath string) *Skill {
	skill := &Skill{
		Name: skillName,
		Path: skillPath,
	}

	// 检查是否有YAML front matter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			// 解析front matter（简单实现，只提取name和description）
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
			// 剩余部分是内容
			if len(parts) == 3 {
				skill.Content = strings.TrimSpace(parts[2])
			}
		} else {
			// 没有front matter，整个内容就是skill内容
			skill.Content = content
		}
	} else {
		// 没有front matter，整个内容就是skill内容
		skill.Content = content
	}

	// 如果内容为空，使用描述作为内容
	if skill.Content == "" {
		skill.Content = skill.Description
	}

	return skill
}

// GetSkillContent 获取skill的完整内容（用于注入到系统提示词）
func (m *Manager) GetSkillContent(skillNames []string) (string, error) {
	skills, err := m.LoadSkills(skillNames)
	if err != nil {
		return "", err
	}

	if len(skills) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## 可用Skills\n\n")
	builder.WriteString("在执行任务前，请仔细阅读以下skills内容，这些内容包含了相关的专业知识和方法：\n\n")

	for _, skill := range skills {
		builder.WriteString(fmt.Sprintf("### Skill: %s\n", skill.Name))
		if skill.Description != "" {
			builder.WriteString(fmt.Sprintf("**描述**: %s\n\n", skill.Description))
		}
		builder.WriteString(skill.Content)
		builder.WriteString("\n\n---\n\n")
	}

	return builder.String(), nil
}
