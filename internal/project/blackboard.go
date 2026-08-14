package project

import (
	"fmt"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
)

// AppendSystemPromptBlock 将附加块追加到 system prompt。
func AppendSystemPromptBlock(base, block string) string {
	base = strings.TrimSpace(base)
	block = strings.TrimSpace(block)
	if block == "" {
		return base
	}
	if base == "" {
		return block
	}
	return base + "\n\n" + block
}

const (
	factIndexFooterGetDetail = "需要完整内容（攻击链、POC、请求响应等）时必须调用 get_project_fact(fact_key)，禁止凭摘要臆造细节。"
	factIndexFooterWriteHint = "写入事实 links 时用 from（来源 fact_key → 当前 fact），如 finding 上 {from:target/*, type:discovered_on}；body 写可复现全流程（发现/利用类 fact_key 建议 finding|chain|exploit|poc/ 前缀）。"
	factIndexFooterEmpty     = "需要写入请使用 upsert_project_fact；需要详情请调用 get_project_fact(fact_key)。"
)

// BuildFactIndexBlock 为 Agent 系统提示生成项目黑板索引（key + summary + 关系边 + 攻击路径，不含 body）。
func BuildFactIndexBlock(db *database.DB, projectID string, cfg config.ProjectConfig) (string, error) {
	if db == nil || !cfg.Enabled {
		return "", nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil
	}

	proj, err := db.GetProject(projectID)
	if err != nil {
		return "", err
	}

	facts, err := db.ListProjectFactsForIndex(projectID, cfg.DefaultInjectDeprecated)
	if err != nil {
		return "", err
	}
	allEdges, _ := db.ListProjectFactEdgesByProject(projectID)
	_, incomingByTarget := indexEdgeGroupMaps(allEdges)

	if len(facts) == 0 {
		return wrapFactIndexBlock(fmt.Sprintf("## 项目黑板索引（project: %s, id: %s）\n（暂无事实）\n%s", proj.Name, proj.ID, factIndexFooterEmpty)), nil
	}

	sortFactsForIndex(facts)

	maxRunes := cfg.FactIndexMaxRunesEffective()
	pathMaxRunes := cfg.FactIndexPathMaxRunesEffective()
	footer := factIndexFooterGetDetail + "\n" + factIndexFooterWriteHint
	footerRunes := len([]rune(footer))
	factsBudget := maxRunes - pathMaxRunes - footerRunes
	if factsBudget < 800 {
		factsBudget = maxRunes - footerRunes
		pathMaxRunes = 0
	}

	indexedKeys := make(map[string]struct{}, len(facts))
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 项目黑板索引（project: %s, id: %s）\n", proj.Name, proj.ID))
	used := len([]rune(b.String()))
	omitted := 0

	for _, f := range facts {
		indexedKeys[f.FactKey] = struct{}{}
		line := fmt.Sprintf("- [%s] %s — %s (%s)", f.FactKey, f.Category, strings.TrimSpace(f.Summary), f.Confidence)
		line += FormatFactIndexLinksHint(f.FactKey, incomingByTarget[f.FactKey])
		line += "\n"
		lineRunes := len([]rune(line))
		if used+lineRunes > factsBudget {
			omitted++
			continue
		}
		b.WriteString(line)
		used += lineRunes
	}

	if omitted > 0 {
		b.WriteString(fmt.Sprintf("\n（另有 %d 条未列入索引，请使用 list_project_facts 或 search_project_facts 查询。）\n", omitted))
	}

	if pathSection := BuildFactPathOverviewSection(allEdges, indexedKeys, pathMaxRunes); pathSection != "" {
		b.WriteString("\n")
		b.WriteString(pathSection)
	}

	b.WriteString(footer)
	return wrapFactIndexBlock(b.String()), nil
}
