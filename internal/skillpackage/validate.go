package skillpackage

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var agentSkillsSpecFrontMatterKeys = map[string]struct{}{
	"name": {}, "description": {}, "license": {}, "compatibility": {},
	"metadata": {}, "allowed-tools": {},
}

// ValidateAgentSkillManifest enforces Agent Skills rules for name and description.
func ValidateAgentSkillManifest(m *SkillManifest) error {
	if m == nil {
		return fmt.Errorf("skill manifest is nil")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("SKILL.md front matter: name is required")
	}
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("SKILL.md front matter: description is required")
	}
	if utf8.RuneCountInString(m.Name) > 64 {
		return fmt.Errorf("name exceeds 64 characters (Agent Skills limit)")
	}
	if utf8.RuneCountInString(m.Description) > 1024 {
		return fmt.Errorf("description exceeds 1024 characters (Agent Skills limit)")
	}
	if m.Name != strings.ToLower(m.Name) {
		return fmt.Errorf("name must be lowercase (Agent Skills)")
	}
	for _, r := range m.Name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("name must contain only lowercase letters, numbers, hyphens (Agent Skills)")
		}
	}
	if strings.HasPrefix(m.Name, "-") || strings.HasSuffix(m.Name, "-") {
		return fmt.Errorf("name must not start or end with a hyphen (Agent Skills spec)")
	}
	if strings.Contains(m.Name, "--") {
		return fmt.Errorf("name must not contain consecutive hyphens (Agent Skills spec)")
	}
	lname := strings.ToLower(m.Name)
	if strings.Contains(lname, "anthropic") || strings.Contains(lname, "claude") {
		return fmt.Errorf("name must not contain reserved words anthropic or claude")
	}
	return nil
}

// ValidateAgentSkillManifestInPackage checks manifest and that name matches package directory.
func ValidateAgentSkillManifestInPackage(m *SkillManifest, packageDirName string) error {
	if err := ValidateAgentSkillManifest(m); err != nil {
		return err
	}
	if strings.TrimSpace(packageDirName) == "" {
		return nil
	}
	if m.Name != packageDirName {
		return fmt.Errorf("SKILL.md name %q must match directory name %q (Agent Skills spec)", m.Name, packageDirName)
	}
	return nil
}

// ValidateOfficialFrontMatterTopLevelKeys rejects keys not in the open spec.
func ValidateOfficialFrontMatterTopLevelKeys(fmYAML string) error {
	var top map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmYAML), &top); err != nil {
		return fmt.Errorf("SKILL.md front matter: %w", err)
	}
	for k := range top {
		if _, ok := agentSkillsSpecFrontMatterKeys[k]; !ok {
			return fmt.Errorf("SKILL.md front matter: unsupported key %q (allowed: name, description, license, compatibility, metadata, allowed-tools — see https://agentskills.io/specification.md)", k)
		}
	}
	return nil
}

// ValidateSkillMDPackage validates SKILL.md bytes for writes.
func ValidateSkillMDPackage(raw []byte, packageDirName string) error {
	fmYAML, body, err := ExtractSkillMDFrontMatterYAML(raw)
	if err != nil {
		return err
	}
	if err := ValidateOfficialFrontMatterTopLevelKeys(fmYAML); err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("SKILL.md: markdown body after front matter must not be empty")
	}
	var fm SkillManifest
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		return fmt.Errorf("SKILL.md front matter: %w", err)
	}
	if c := strings.TrimSpace(fm.Compatibility); c != "" && utf8.RuneCountInString(c) > 500 {
		return fmt.Errorf("compatibility exceeds 500 characters (Agent Skills spec)")
	}
	return ValidateAgentSkillManifestInPackage(&fm, packageDirName)
}
