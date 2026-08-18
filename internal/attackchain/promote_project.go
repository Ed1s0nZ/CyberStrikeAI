package attackchain

import (
	"fmt"
	"regexp"
	"strings"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/project"

	"github.com/google/uuid"
)

var promoteSlugSanitizer = regexp.MustCompile(`[^a-z0-9._/-]+`)

// PromoteToProjectResult 攻击链沉淀结果。
type PromoteToProjectResult struct {
	FactsCreated int                         `json:"facts_created"`
	FactsUpdated int                         `json:"facts_updated"`
	EdgesCreated int                         `json:"edges_created"`
	FactKeys     []string                    `json:"fact_keys"`
	Graph        *database.ProjectFactGraph  `json:"graph,omitempty"`
}

// PromoteToProject 将对话攻击链沉淀为项目事实与边。
func PromoteToProject(db *database.DB, projectID, conversationID string) (*PromoteToProjectResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database 未初始化")
	}
	projectID = strings.TrimSpace(projectID)
	conversationID = strings.TrimSpace(conversationID)
	if projectID == "" || conversationID == "" {
		return nil, fmt.Errorf("project_id 与 conversation_id 必填")
	}
	if _, err := db.GetProject(projectID); err != nil {
		return nil, fmt.Errorf("项目不存在")
	}
	conv, err := db.GetConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	if pid := strings.TrimSpace(conv.ProjectID); pid != "" && pid != projectID {
		return nil, fmt.Errorf("对话已绑定其他项目")
	}

	nodes, err := db.LoadAttackChainNodes(conversationID)
	if err != nil {
		return nil, err
	}
	edges, err := db.LoadAttackChainEdges(conversationID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("该对话尚无攻击链，请先在对话中生成攻击链")
	}

	res := &PromoteToProjectResult{}
	nodeToKey := make(map[string]string, len(nodes))
	usedKeys := map[string]int{}

	for _, node := range nodes {
		key := allocatePromoteFactKey(node, usedKeys)
		nodeToKey[node.ID] = key
		category := mapPromoteNodeCategory(node.Type)
		existing, getErr := db.GetProjectFactByKey(projectID, key)
		f := &database.ProjectFact{
			ProjectID:            projectID,
			FactKey:              key,
			Category:             category,
			Summary:              strings.TrimSpace(node.Label),
			Body:                 formatPromotedFactBody(node, conversationID),
			Confidence:           "tentative",
			SourceConversationID: conversationID,
		}
		if getErr == nil && existing != nil {
			f.ID = existing.ID
			f.CreatedAt = existing.CreatedAt
			if strings.TrimSpace(f.Summary) == "" {
				f.Summary = existing.Summary
			}
			if _, err := db.UpsertProjectFact(f); err != nil {
				return nil, err
			}
			res.FactsUpdated++
		} else {
			if _, err := db.UpsertProjectFact(f); err != nil {
				return nil, err
			}
			res.FactsCreated++
		}
		res.FactKeys = append(res.FactKeys, key)
	}

	for _, edge := range edges {
		srcKey, ok1 := nodeToKey[edge.Source]
		tgtKey, ok2 := nodeToKey[edge.Target]
		if !ok1 || !ok2 || srcKey == tgtKey {
			continue
		}
		edgeType := mapPromoteEdgeType(edge.Type)
		incoming, _ := db.ListIncomingProjectFactEdges(projectID, tgtKey)
		merged := project.MergeLinkFromInputsUnique(promoteFromEdgeInputsFromDB(incoming), []database.ProjectFactEdgeFromInput{{From: srcKey, Type: edgeType}})
		if err := db.ReplaceIncomingProjectFactEdges(projectID, tgtKey, merged); err != nil {
			return nil, err
		}
		res.EdgesCreated++
		if fact, err := db.GetProjectFactByKey(projectID, tgtKey); err == nil {
			in, _ := db.ListIncomingProjectFactEdges(projectID, tgtKey)
			fact.Body = project.SyncBodyLinksSection(fact.Body, in)
			_, _ = db.UpsertProjectFact(fact)
		}
	}

	graph, _ := project.BuildProjectFactGraph(db, projectID, "full", true)
	res.Graph = graph
	return res, nil
}

func promoteFromEdgeInputsFromDB(edges []*database.ProjectFactEdge) []database.ProjectFactEdgeFromInput {
	out := make([]database.ProjectFactEdgeFromInput, 0, len(edges))
	for _, e := range edges {
		out = append(out, database.ProjectFactEdgeFromInput{From: e.SourceFactKey, Type: e.EdgeType, Confidence: e.Confidence})
	}
	return out
}

func mapPromoteNodeCategory(nodeType string) string {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "target":
		return project.FactCategoryTarget
	case "vulnerability":
		return project.FactCategoryFinding
	case "action":
		return project.FactCategoryChain
	default:
		return project.FactCategoryNote
	}
}

func mapPromoteEdgeType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "discovers", "discovered_on", "targets":
		return "discovered_on"
	case "exploits":
		return "exploits"
	case "enables":
		return "enables"
	case "depends_on":
		return "depends_on"
	default:
		return "leads_to"
	}
}

func allocatePromoteFactKey(node Node, used map[string]int) string {
	prefix := "chain/"
	switch strings.ToLower(strings.TrimSpace(node.Type)) {
	case "target":
		prefix = "target/"
	case "vulnerability":
		prefix = "finding/"
	case "action":
		prefix = "chain/"
	}
	base := promoteSlugify(node.Label)
	if base == "" {
		base = promoteSlugify(node.ID)
	}
	if base == "" {
		base = uuid.New().String()[:8]
	}
	key := prefix + base
	if n, ok := used[key]; ok {
		n++
		used[key] = n
		key = fmt.Sprintf("%s-%d", key, n)
	} else {
		used[key] = 1
	}
	return key
}

func promoteSlugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "-", "—", "-", "–", "-", "/", "-").Replace(s)
	s = promoteSlugSanitizer.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

func formatPromotedFactBody(node Node, conversationID string) string {
	var b strings.Builder
	b.WriteString("## 来源\n")
	b.WriteString(fmt.Sprintf("- 对话攻击链沉淀\n- source_conversation_id: %s\n- node_id: %s\n- node_type: %s\n\n", conversationID, node.ID, node.Type))
	b.WriteString("## 摘要\n")
	b.WriteString(strings.TrimSpace(node.Label))
	b.WriteString("\n\n## 关联\n- 结构化关系边（自动同步）:\n  （见项目攻击路径图）\n")
	return b.String()
}
