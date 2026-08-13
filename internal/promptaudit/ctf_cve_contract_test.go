package promptaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCTFCVEIntelligenceContract(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	rolePath := filepath.Join(repoRoot, "roles", "CTF.yaml")
	roleData, err := os.ReadFile(rolePath)
	if err != nil {
		t.Fatal(err)
	}

	var role struct {
		UserPrompt string   `yaml:"user_prompt"`
		Tools      []string `yaml:"tools"`
	}
	if err := yaml.Unmarshal(roleData, &role); err != nil {
		t.Fatal(err)
	}

	requiredTools := map[string]bool{
		"cve-search::vul_cve_search":         true,
		"cve-search::vul_last_cves":          true,
		"cve-search::vul_vendor_product_cve": true,
		"cve-search::vul_db_update_status":   true,
		"cve-search::vul_vendor_products":    true,
		"cve-search::vul_vendors":            true,
	}
	actualTools := make(map[string]bool, len(requiredTools))
	for _, tool := range role.Tools {
		if strings.HasPrefix(tool, "cve-search::") {
			actualTools[tool] = true
		}
	}
	if len(actualTools) != len(requiredTools) {
		t.Fatalf("CTF role must expose exactly %d cve-search tools, got %v", len(requiredTools), actualTools)
	}
	for tool := range requiredTools {
		if !actualTools[tool] {
			t.Errorf("CTF role missing required tool %q", tool)
		}
	}

	for _, marker := range []string{
		"cve_lookup_state=pending",
		"cve-search__vul_vendor_product_cve",
		"精确版本不是开始检索的前置条件",
		"最多 3 次非控制面工具调用",
		"upsert_project_fact",
		"根组件加最多 3 个",
		"ComfyUI-Manager",
		"candidate",
		"observed_installed",
	} {
		if !strings.Contains(role.UserPrompt, marker) {
			t.Errorf("CTF role missing CVE intelligence marker %q", marker)
		}
	}

	skillPath := filepath.Join(repoRoot, "skills", "component-vuln-intel", "SKILL.md")
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	skill := string(skillData)
	for _, marker := range []string{
		"cve_lookup_state=pending",
		"根组件与组件家族必须同批检索",
		"最多 3 个家族候选",
		"ComfyUI-Manager",
		"family_of/addon_of",
		"cve_lookup_state=queried",
	} {
		if !strings.Contains(skill, marker) {
			t.Errorf("component-vuln-intel skill missing marker %q", marker)
		}
	}
}
