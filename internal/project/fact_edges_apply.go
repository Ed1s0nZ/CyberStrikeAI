package project

import (
	"cyberstrike-ai/internal/database"
)

// ApplyFactOutgoingLinks 替换某事实的出边（links 为 nil 时不修改）。
func ApplyFactOutgoingLinks(db *database.DB, projectID, sourceFactKey, sourceConversationID string, links []database.ProjectFactEdgeInput) error {
	if links == nil {
		return nil
	}
	return db.ReplaceOutgoingProjectFactEdges(projectID, sourceFactKey, sourceConversationID, links)
}

// ResolveFactLinkInputs 合并 links 数组与 links_text 文本（数组优先）。
func ResolveFactLinkInputs(links []database.ProjectFactEdgeFromInput, linksText string) ([]database.ProjectFactEdgeFromInput, error) {
	if len(links) > 0 {
		return links, nil
	}
	return ParseFactLinksText(linksText)
}

// ApplyFactIncomingLinks 替换某事实的入边（links 为 nil 时不修改）。
func ApplyFactIncomingLinks(db *database.DB, projectID, targetFactKey string, links []database.ProjectFactEdgeFromInput) error {
	if links == nil {
		return nil
	}
	return db.ReplaceIncomingProjectFactEdges(projectID, targetFactKey, links)
}

// PersistFactIncomingLinks 写入入边并可选同步当前事实 body「关联」段。
func PersistFactIncomingLinks(db *database.DB, projectID, targetFactKey string, links []database.ProjectFactEdgeFromInput, syncBody bool) error {
	if links == nil {
		return nil
	}
	if err := ApplyFactIncomingLinks(db, projectID, targetFactKey, links); err != nil {
		return err
	}
	if !syncBody {
		return nil
	}
	f, err := db.GetProjectFactByKey(projectID, targetFactKey)
	if err != nil {
		return nil
	}
	in, err := db.ListIncomingProjectFactEdges(projectID, targetFactKey)
	if err != nil {
		return err
	}
	f.Body = SyncBodyLinksSection(f.Body, in)
	_, err = db.UpsertProjectFact(f)
	return err
}

// PersistFactLinksFromParsed 写入解析后的 links（parsed 为 nil 表示不修改）。
func PersistFactLinksFromParsed(db *database.DB, projectID, factKey, sourceConversationID string, parsed *ParsedFactLinks, syncBody bool) error {
	if parsed == nil || parsed.Incoming == nil {
		return nil
	}
	return PersistFactIncomingLinks(db, projectID, factKey, parsed.Incoming, syncBody)
}

// PersistFactOutgoingLinks 写入出边（图连线等低层 API；body 同步请用 PersistFactIncomingLinks）。
func PersistFactOutgoingLinks(db *database.DB, projectID, sourceFactKey, sourceConversationID string, links []database.ProjectFactEdgeInput, syncBody bool) error {
	if links == nil {
		return nil
	}
	return ApplyFactOutgoingLinks(db, projectID, sourceFactKey, sourceConversationID, links)
}

// LinkCountMap 项目内各 fact 的入/出边计数。
type LinkCountMap map[string]LinkCounts

// LinkCounts 单 fact 的入/出边数。
type LinkCounts struct {
	Outgoing int `json:"outgoing"`
	Incoming int `json:"incoming"`
}

// LoadProjectFactLinkCounts 批量加载边计数。
func LoadProjectFactLinkCounts(db *database.DB, projectID string) (LinkCountMap, error) {
	edges, err := db.ListProjectFactEdgesByProject(projectID)
	if err != nil {
		return nil, err
	}
	m := LinkCountMap{}
	for _, e := range edges {
		c := m[e.SourceFactKey]
		c.Outgoing++
		m[e.SourceFactKey] = c
		c = m[e.TargetFactKey]
		c.Incoming++
		m[e.TargetFactKey] = c
	}
	return m, nil
}
