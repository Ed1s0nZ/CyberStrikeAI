package skillpackage

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// ListSkillSummaries scans skillsRoot and returns index rows for the admin API.
func ListSkillSummaries(skillsRoot string) ([]SkillSummary, error) {
	names, err := ListSkillDirNames(skillsRoot)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]SkillSummary, 0, len(names))
	for _, dirName := range names {
		su, err := loadSummary(skillsRoot, dirName)
		if err != nil {
			continue
		}
		out = append(out, su)
	}
	return out, nil
}

func loadSummary(skillsRoot, dirName string) (SkillSummary, error) {
	skillPath := SkillDir(skillsRoot, dirName)
	mdPath, err := ResolveSKILLPath(skillPath)
	if err != nil {
		return SkillSummary{}, err
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return SkillSummary{}, err
	}
	man, _, err := ParseSkillMD(raw)
	if err != nil {
		return SkillSummary{}, err
	}
	if err := ValidateAgentSkillManifestInPackage(man, dirName); err != nil {
		return SkillSummary{}, err
	}
	fi, err := os.Stat(mdPath)
	if err != nil {
		return SkillSummary{}, err
	}
	pfiles, err := ListPackageFiles(skillsRoot, dirName)
	if err != nil {
		return SkillSummary{}, err
	}
	nFiles := 0
	for _, p := range pfiles {
		if !p.IsDir {
			nFiles++
		}
	}
	scripts, err := listScripts(skillsRoot, dirName)
	if err != nil {
		return SkillSummary{}, err
	}
	ver := versionFromMetadata(man)
	return SkillSummary{
		ID:          dirName,
		DirName:     dirName,
		Name:        man.Name,
		Description: man.Description,
		Version:     ver,
		Path:        skillPath,
		Tags:        manifestTags(man),
		ScriptCount: len(scripts),
		FileCount:   nFiles,
		FileSize:    fi.Size(),
		ModTime:     fi.ModTime().Format("2006-01-02 15:04:05"),
		Progressive: true,
	}, nil
}

// LoadOptions mirrors legacy API query params for the web admin.
type LoadOptions struct {
	Depth   string // summary | full
	Section string
}

// LoadSkill returns manifest + body + package listing for admin.
func LoadSkill(skillsRoot, skillID string, opt LoadOptions) (*SkillView, error) {
	skillPath := SkillDir(skillsRoot, skillID)
	mdPath, err := ResolveSKILLPath(skillPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	man, body, err := ParseSkillMD(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateAgentSkillManifestInPackage(man, skillID); err != nil {
		return nil, err
	}
	pfiles, err := ListPackageFiles(skillsRoot, skillID)
	if err != nil {
		return nil, err
	}
	scripts, err := listScripts(skillsRoot, skillID)
	if err != nil {
		return nil, err
	}
	sort.Slice(scripts, func(i, j int) bool { return scripts[i].RelPath < scripts[j].RelPath })
	sections := deriveSections(body)
	ver := versionFromMetadata(man)
	v := &SkillView{
		DirName:      skillID,
		Name:         man.Name,
		Description:  man.Description,
		Content:      body,
		Path:         skillPath,
		Version:      ver,
		Tags:         manifestTags(man),
		Scripts:      scripts,
		Sections:     sections,
		PackageFiles: pfiles,
	}
	depth := strings.ToLower(strings.TrimSpace(opt.Depth))
	if depth == "" {
		depth = "full"
	}
	sec := strings.TrimSpace(opt.Section)
	if sec != "" {
		mds := splitMarkdownSections(body)
		chunk := findSectionContent(mds, sec)
		if chunk == "" {
			v.Content = fmt.Sprintf("_(section %q not found in SKILL.md for skill %s)_", sec, skillID)
		} else {
			v.Content = chunk
		}
		return v, nil
	}
	if depth == "summary" {
		v.Content = buildSummaryMarkdown(man.Name, man.Description, v.Tags, scripts, sections, body)
	}
	return v, nil
}

// ReadScriptText returns file content as string (for HTTP resource_path).
func ReadScriptText(skillsRoot, skillID, relPath string, maxBytes int64) (string, error) {
	b, err := ReadPackageFile(skillsRoot, skillID, relPath, maxBytes)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
