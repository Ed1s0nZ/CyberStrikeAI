package workflow

import (
	"fmt"
	"strconv"
	"strings"
)

var allowedWorkflowNodeTypes = map[string]bool{
	"start":     true,
	"tool":      true,
	"agent":     true,
	"condition": true,
	"hitl":      true,
	"output":    true,
	"end":       true,
}

func validateGraphDefinition(g *graphDef, idx *graphIndex) error {
	if g == nil || idx == nil {
		return fmt.Errorf("工作流图为空")
	}
	if err := validateNodeIDsAndTypes(g); err != nil {
		return err
	}
	if err := validateEdges(g, idx); err != nil {
		return err
	}
	if err := validateNodeTopology(idx); err != nil {
		return err
	}
	if err := validateNodeConfigs(idx); err != nil {
		return err
	}
	if err := validateDAG(idx); err != nil {
		return err
	}
	if err := validateReachability(idx); err != nil {
		return err
	}
	return nil
}

func validateNodeIDsAndTypes(g *graphDef) error {
	seen := make(map[string]bool, len(g.Nodes))
	for _, node := range g.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return fmt.Errorf("工作流存在空节点 ID")
		}
		if seen[id] {
			return fmt.Errorf("工作流存在重复节点 ID: %s", id)
		}
		seen[id] = true
		nodeType := strings.ToLower(strings.TrimSpace(node.Type))
		if nodeType == "" {
			return fmt.Errorf("节点「%s」缺少节点类型", id)
		}
		if !allowedWorkflowNodeTypes[nodeType] {
			return fmt.Errorf("节点「%s」使用了未知节点类型: %s", id, node.Type)
		}
	}
	return nil
}

func validateEdges(g *graphDef, idx *graphIndex) error {
	seen := make(map[string]bool, len(g.Edges))
	for _, edge := range g.Edges {
		if id := strings.TrimSpace(edge.ID); id != "" {
			if seen[id] {
				return fmt.Errorf("工作流存在重复连线 ID: %s", id)
			}
			seen[id] = true
		}
		source := strings.TrimSpace(edge.Source)
		target := strings.TrimSpace(edge.Target)
		if source == "" || target == "" {
			return fmt.Errorf("工作流存在源或目标为空的连线")
		}
		if source == target {
			return fmt.Errorf("连线「%s」不能自环", firstNonEmpty(edge.ID, source))
		}
		if _, ok := idx.nodes[source]; !ok {
			return fmt.Errorf("连线「%s」引用了不存在的源节点: %s", firstNonEmpty(edge.ID, source), source)
		}
		if _, ok := idx.nodes[target]; !ok {
			return fmt.Errorf("连线「%s」引用了不存在的目标节点: %s", firstNonEmpty(edge.ID, target), target)
		}
	}
	return nil
}

func validateNodeTopology(idx *graphIndex) error {
	starts := explicitStartNodeIDs(idx)
	if len(starts) == 0 {
		return fmt.Errorf("工作流至少需要一个开始节点")
	}
	outputs := outputNodeIDs(idx)
	if len(outputs) == 0 {
		return fmt.Errorf("工作流至少需要一个输出节点")
	}
	for id, node := range idx.nodes {
		inDegree := len(idx.incoming[id])
		outDegree := len(idx.outgoing[id])
		nodeType := strings.ToLower(strings.TrimSpace(node.Type))
		switch nodeType {
		case "start":
			if inDegree > 0 {
				return fmt.Errorf("开始节点「%s」不能有入边", firstNonEmpty(node.Label, id))
			}
			if outDegree == 0 {
				return fmt.Errorf("开始节点「%s」至少需要一条出边", firstNonEmpty(node.Label, id))
			}
		case "output", "end":
			if outDegree > 0 {
				return fmt.Errorf("%s 节点「%s」不能有出边", displayNodeType(nodeType), firstNonEmpty(node.Label, id))
			}
			if inDegree == 0 {
				return fmt.Errorf("%s 节点「%s」至少需要一条入边", displayNodeType(nodeType), firstNonEmpty(node.Label, id))
			}
		default:
			if inDegree == 0 {
				return fmt.Errorf("节点「%s」不可达：非开始节点必须有入边", firstNonEmpty(node.Label, id))
			}
			if outDegree == 0 {
				return fmt.Errorf("节点「%s」没有出边；请连接到 output/end 节点", firstNonEmpty(node.Label, id))
			}
		}
	}
	return nil
}

func validateNodeConfigs(idx *graphIndex) error {
	for id, node := range idx.nodes {
		label := firstNonEmpty(node.Label, id)
		switch strings.ToLower(strings.TrimSpace(node.Type)) {
		case "tool":
			if cfgString(node.Config, "tool_name") == "" {
				return fmt.Errorf("工具节点「%s」必须选择 MCP 工具", label)
			}
			if err := validateToolConfig(node); err != nil {
				return err
			}
		case "agent":
			if cfgString(node.Config, "instruction") == "" {
				if _, ok := parseFieldBinding(node.Config, "input_binding"); !ok {
					return fmt.Errorf("Agent 节点「%s」必须填写节点指令或输入绑定", label)
				}
			}
			if cfgString(node.Config, "output_key") == "" {
				return fmt.Errorf("Agent 节点「%s」必须填写输出变量名", label)
			}
		case "condition":
			if cfgString(node.Config, "expression") == "" {
				return fmt.Errorf("条件节点「%s」必须填写表达式", label)
			}
			if err := validateConditionExpression(cfgString(node.Config, "expression")); err != nil {
				return fmt.Errorf("条件节点「%s」表达式非法: %w", label, err)
			}
			if n := len(idx.outgoing[id]); n < 1 || n > 2 {
				return fmt.Errorf("条件节点「%s」需要 1 到 2 条出边（是/否）", label)
			}
			if err := validateConditionBranchLabels(idx, id, node); err != nil {
				return err
			}
		case "output":
			if cfgString(node.Config, "output_key") == "" {
				return fmt.Errorf("输出节点「%s」必须填写输出变量名", label)
			}
		}
		if err := validateJoinConfig(idx, id, node); err != nil {
			return err
		}
		if hasConditionalOutgoingEdges(idx, id) {
			if err := validateConditionalOutgoingEdges(idx, id, node); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateConditionalOutgoingEdges(idx *graphIndex, nodeID string, node graphNode) error {
	unconditional := 0
	for _, edge := range idx.outgoing[nodeID] {
		cond := firstNonEmpty(cfgString(edge.Config, "condition"), cfgString(edge.Config, "expression"))
		if cond != "" {
			if err := validateConditionExpression(cond); err != nil {
				return fmt.Errorf("节点「%s」的连线条件非法: %w", firstNonEmpty(node.Label, nodeID), err)
			}
		}
		if cond == "" {
			unconditional++
		}
	}
	if unconditional > 1 {
		return fmt.Errorf("节点「%s」的条件出边最多只能有一条默认分支", firstNonEmpty(node.Label, nodeID))
	}
	return nil
}

func validateToolConfig(node graphNode) error {
	rawArgs := cfgString(node.Config, "arguments")
	if rawArgs != "" {
		if _, err := resolveToolArguments(node.Config, &WorkflowLocalState{}); err != nil {
			return fmt.Errorf("工具节点「%s」参数 JSON 非法: %w", firstNonEmpty(node.Label, node.ID), err)
		}
	}
	if timeout := cfgString(node.Config, "timeout_seconds"); timeout != "" {
		if _, err := parsePositiveInt(timeout); err != nil {
			return fmt.Errorf("工具节点「%s」超时时间必须是正整数", firstNonEmpty(node.Label, node.ID))
		}
	}
	return nil
}

func validateJoinConfig(idx *graphIndex, nodeID string, node graphNode) error {
	strategy := joinStrategy(node)
	if !allowedJoinStrategies[strategy] {
		return fmt.Errorf("节点「%s」使用了未知汇聚策略: %s", firstNonEmpty(node.Label, nodeID), strategy)
	}
	if len(idx.incoming[nodeID]) > 1 && strategy == "" {
		return fmt.Errorf("节点「%s」有多个上游时必须声明汇聚策略", firstNonEmpty(node.Label, nodeID))
	}
	return nil
}

func validateConditionBranchLabels(idx *graphIndex, nodeID string, node graphNode) error {
	seen := map[string]bool{}
	for _, edge := range idx.outgoing[nodeID] {
		hint := conditionBranchHint(edge)
		if hint == "" {
			return fmt.Errorf("条件节点「%s」的出边必须标记为是/否或 true/false", firstNonEmpty(node.Label, nodeID))
		}
		if seen[hint] {
			return fmt.Errorf("条件节点「%s」存在重复分支标签: %s", firstNonEmpty(node.Label, nodeID), hint)
		}
		seen[hint] = true
	}
	return nil
}

func validateDAG(idx *graphIndex) error {
	color := make(map[string]int, len(idx.nodes))
	var visit func(string) error
	visit = func(id string) error {
		switch color[id] {
		case 1:
			return fmt.Errorf("工作流存在环路，Workflow 编排必须是 DAG: %s", id)
		case 2:
			return nil
		}
		color[id] = 1
		for _, edge := range idx.outgoing[id] {
			if err := visit(edge.Target); err != nil {
				return err
			}
		}
		color[id] = 2
		return nil
	}
	for id := range idx.nodes {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func validateReachability(idx *graphIndex) error {
	starts := explicitStartNodeIDs(idx)
	reached := make(map[string]bool, len(idx.nodes))
	queue := append([]string(nil), starts...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if reached[id] {
			continue
		}
		reached[id] = true
		for _, edge := range idx.outgoing[id] {
			queue = append(queue, edge.Target)
		}
	}
	for id, node := range idx.nodes {
		if !reached[id] {
			return fmt.Errorf("节点「%s」不可达：没有从开始节点连通到该节点", firstNonEmpty(node.Label, id))
		}
	}

	canReachTerminal := make(map[string]bool, len(idx.nodes))
	visiting := make(map[string]bool, len(idx.nodes))
	var reachesTerminal func(string) bool
	reachesTerminal = func(id string) bool {
		if canReachTerminal[id] {
			return true
		}
		if visiting[id] {
			return false
		}
		visiting[id] = true
		node := idx.nodes[id]
		nodeType := strings.ToLower(strings.TrimSpace(node.Type))
		if nodeType == "output" || nodeType == "end" {
			canReachTerminal[id] = true
			visiting[id] = false
			return true
		}
		for _, edge := range idx.outgoing[id] {
			if reachesTerminal(edge.Target) {
				canReachTerminal[id] = true
				visiting[id] = false
				return true
			}
		}
		visiting[id] = false
		return false
	}
	for id, node := range idx.nodes {
		if !reachesTerminal(id) {
			return fmt.Errorf("节点「%s」无法到达 output/end 终点", firstNonEmpty(node.Label, id))
		}
	}
	return nil
}

func explicitStartNodeIDs(idx *graphIndex) []string {
	var ids []string
	for id, node := range idx.nodes {
		if strings.EqualFold(node.Type, "start") {
			ids = append(ids, id)
		}
	}
	sortNodeIDsByCanvas(ids, idx.nodes)
	return ids
}

func outputNodeIDs(idx *graphIndex) []string {
	var ids []string
	for id, node := range idx.nodes {
		if strings.EqualFold(node.Type, "output") {
			ids = append(ids, id)
		}
	}
	sortNodeIDsByCanvas(ids, idx.nodes)
	return ids
}

func displayNodeType(nodeType string) string {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "output":
		return "输出"
	case "end":
		return "结束"
	default:
		return nodeType
	}
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not positive integer")
	}
	return n, nil
}
