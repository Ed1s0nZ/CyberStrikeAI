// Package promptaudit provides a repository-level static audit for agent prompts,
// role overlays, and Agent Skills packages.
package promptaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cyberstrike-ai/internal/skillpackage"
	"gopkg.in/yaml.v3"
)

type Limits struct {
	AgentMaxLines int
	RoleMaxLines  int
	SkillMaxLines int
	SkillMaxBytes int64
}

func DefaultLimits() Limits {
	return Limits{
		AgentMaxLines: 120,
		RoleMaxLines:  80,
		SkillMaxLines: 200,
		SkillMaxBytes: 24 * 1024,
	}
}

type Finding struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

type Summary struct {
	Agents               int            `json:"agents"`
	Roles                int            `json:"roles"`
	Skills               int            `json:"skills"`
	InvalidAgents        int            `json:"invalid_agents"`
	InvalidRoles         int            `json:"invalid_roles"`
	InvalidSkills        int            `json:"invalid_skills"`
	OversizedAgents      int            `json:"oversized_agents"`
	OversizedRoles       int            `json:"oversized_roles"`
	OversizedSkills      int            `json:"oversized_skills"`
	BrokenReferences     int            `json:"broken_references"`
	InternalNameMismatch int            `json:"internal_name_mismatch"`
	MissingSections      map[string]int `json:"missing_skill_sections"`
}

type Report struct {
	Root     string    `json:"root"`
	Limits   Limits    `json:"limits"`
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
}

var (
	markdownReferenceRE = regexp.MustCompile("`((?:references|scripts|assets)/[^`\\s]+)`")
	internalSkillNameRE = regexp.MustCompile(`(?im)^\s*(?:[-*]\s*)?(?:\*\*)?Skill Name(?:\*\*)?\s*:\s*([a-z0-9-]+)\s*$`)
	requiredSections    = []struct {
		name     string
		patterns []string
	}{
		{name: "when_to_use", patterns: []string{"when to use", "适用", "触发"}},
		{name: "preconditions", patterns: []string{"precondition", "前置"}},
		{name: "procedure", patterns: []string{"procedure", "workflow", "instructions", "步骤", "流程", "操作"}},
		{name: "stop_conditions", patterns: []string{"stop condition", "停止", "退出"}},
		{name: "output", patterns: []string{"output", "输出", "交付"}},
	}
)

func Audit(root string, limits Limits) (Report, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Root:   absRoot,
		Limits: limits,
		Summary: Summary{
			MissingSections: make(map[string]int),
		},
	}

	auditAgents(absRoot, &report)
	auditRoles(absRoot, &report)
	if err := auditSkills(absRoot, &report); err != nil {
		return Report{}, err
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Path != report.Findings[j].Path {
			return report.Findings[i].Path < report.Findings[j].Path
		}
		return report.Findings[i].Kind < report.Findings[j].Kind
	})
	return report, nil
}

func auditAgents(root string, report *Report) {
	entries, err := os.ReadDir(filepath.Join(root, "agents"))
	if err != nil {
		addFinding(report, "error", "agents_dir", "agents", err.Error())
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}
		report.Summary.Agents++
		rel := filepath.ToSlash(filepath.Join("agents", entry.Name()))
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			report.Summary.InvalidAgents++
			addFinding(report, "error", "agent_read", rel, err.Error())
			continue
		}
		if _, _, err := skillpackage.ExtractSkillMDFrontMatterYAML(raw); err != nil {
			report.Summary.InvalidAgents++
			addFinding(report, "error", "agent_frontmatter", rel, err.Error())
		}
		if lines := lineCount(raw); lines > report.Limits.AgentMaxLines {
			report.Summary.OversizedAgents++
			addFinding(report, "warning", "agent_size", rel, fmt.Sprintf("%d lines exceeds advisory limit %d", lines, report.Limits.AgentMaxLines))
		}
	}
}

func auditRoles(root string, report *Report) {
	entries, err := os.ReadDir(filepath.Join(root, "roles"))
	if err != nil {
		addFinding(report, "error", "roles_dir", "roles", err.Error())
		return
	}
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		report.Summary.Roles++
		rel := filepath.ToSlash(filepath.Join("roles", entry.Name()))
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			report.Summary.InvalidRoles++
			addFinding(report, "error", "role_read", rel, err.Error())
			continue
		}
		var role struct {
			Name       string `yaml:"name"`
			UserPrompt string `yaml:"user_prompt"`
		}
		if err := yaml.Unmarshal(raw, &role); err != nil {
			report.Summary.InvalidRoles++
			addFinding(report, "error", "role_yaml", rel, err.Error())
			continue
		}
		if strings.TrimSpace(role.Name) == "" {
			report.Summary.InvalidRoles++
			addFinding(report, "error", "role_name", rel, "name is empty")
		}
		if lines := lineCount([]byte(role.UserPrompt)); lines > report.Limits.RoleMaxLines {
			report.Summary.OversizedRoles++
			addFinding(report, "warning", "role_prompt_size", rel, fmt.Sprintf("user_prompt has %d lines; advisory limit is %d", lines, report.Limits.RoleMaxLines))
		}
	}
}

func auditSkills(root string, report *Report) error {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillPath := filepath.Join(root, "skills", entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}
		report.Summary.Skills++
		rel := filepath.ToSlash(filepath.Join("skills", entry.Name(), "SKILL.md"))
		raw, err := os.ReadFile(skillPath)
		if err != nil {
			report.Summary.InvalidSkills++
			addFinding(report, "error", "skill_read", rel, err.Error())
			continue
		}
		if err := skillpackage.ValidateSkillMDPackage(raw, entry.Name()); err != nil {
			report.Summary.InvalidSkills++
			addFinding(report, "error", "skill_manifest", rel, err.Error())
			continue
		}
		manifest, body, err := skillpackage.ParseSkillMD(raw)
		if err != nil {
			report.Summary.InvalidSkills++
			addFinding(report, "error", "skill_parse", rel, err.Error())
			continue
		}
		if int64(len(raw)) > report.Limits.SkillMaxBytes || lineCount(raw) > report.Limits.SkillMaxLines {
			report.Summary.OversizedSkills++
			addFinding(report, "warning", "skill_size", rel, fmt.Sprintf("%d bytes/%d lines exceeds advisory limit %d bytes/%d lines", len(raw), lineCount(raw), report.Limits.SkillMaxBytes, report.Limits.SkillMaxLines))
		}
		headings := markdownHeadings(body)
		for _, section := range requiredSections {
			if !headingMatches(headings, section.patterns) {
				report.Summary.MissingSections[section.name]++
			}
		}
		if match := internalSkillNameRE.FindStringSubmatch(body); len(match) == 2 && match[1] != manifest.Name {
			report.Summary.InternalNameMismatch++
			addFinding(report, "warning", "skill_internal_name", rel, fmt.Sprintf("body Skill Name %q differs from manifest %q", match[1], manifest.Name))
		}
		for _, match := range markdownReferenceRE.FindAllStringSubmatch(body, -1) {
			ref := strings.TrimRight(match[1], ".,;:)")
			if _, err := os.Stat(filepath.Join(filepath.Dir(skillPath), filepath.FromSlash(ref))); err != nil {
				report.Summary.BrokenReferences++
				addFinding(report, "warning", "skill_reference", rel, fmt.Sprintf("referenced path does not exist: %s", ref))
			}
		}
	}
	return nil
}

func addFinding(report *Report, severity, kind, path, message string) {
	report.Findings = append(report.Findings, Finding{Severity: severity, Kind: kind, Path: path, Message: message})
}

func lineCount(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	return strings.Count(strings.TrimSuffix(string(raw), "\n"), "\n") + 1
}

func markdownHeadings(body string) []string {
	var headings []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "##") {
			continue
		}
		headings = append(headings, strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#"))))
	}
	return headings
}

func headingMatches(headings, patterns []string) bool {
	for _, heading := range headings {
		for _, pattern := range patterns {
			if strings.Contains(heading, pattern) {
				return true
			}
		}
	}
	return false
}
