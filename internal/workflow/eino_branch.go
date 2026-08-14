package workflow

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

func hasConditionalOutgoingEdges(idx *graphIndex, nodeID string) bool {
	for _, edge := range idx.outgoing[nodeID] {
		cond := firstNonEmpty(cfgString(edge.Config, "condition"), cfgString(edge.Config, "expression"))
		if cond != "" {
			return true
		}
	}
	return false
}

func wireConditionBranch(
	wf *compose.Workflow[WorkflowInput, WorkflowOutput],
	nodeRefs map[string]*compose.WorkflowNode,
	idx *graphIndex,
	condID string,
	condNode graphNode,
) error {
	edges := idx.outgoing[condID]
	if len(edges) == 0 {
		return nil
	}
	branchID := branchNodeID(condID)
	wf.AddPassthroughNode(branchID).AddInput(condID)

	endNodes := map[string]bool{compose.END: true}
	for _, edge := range edges {
		endNodes[edge.Target] = true
	}

	sortedEdges := append([]graphEdge(nil), edges...)
	sortEdgesByCanvas(sortedEdges, idx.nodes)

	branch := compose.NewGraphBranch(func(runCtx context.Context, _ map[string]any) (string, error) {
		rt := workflowRuntimeFrom(runCtx)
		if rt == nil {
			return compose.END, fmt.Errorf("workflow runtime missing in context")
		}
		emitConditionBranchProgress(rt.args, rt.runID, condNode, sortedEdges, idx.nodes, rt.state)
		for edgeIdx, edge := range sortedEdges {
			if conditionBranchAllowed(edge, edgeIdx, rt.state) {
				return edge.Target, nil
			}
		}
		return compose.END, nil
	}, endNodes)
	wf.AddBranch(branchID, branch)

	for _, edge := range edges {
		if target, ok := nodeRefs[edge.Target]; ok {
			target.AddInput(branchID)
		}
	}
	return nil
}

func wireEdgeConditionBranch(
	wf *compose.Workflow[WorkflowInput, WorkflowOutput],
	nodeRefs map[string]*compose.WorkflowNode,
	idx *graphIndex,
	sourceID string,
	sourceNode graphNode,
) error {
	edges := idx.outgoing[sourceID]
	if len(edges) == 0 {
		return nil
	}
	branchID := edgeBranchNodeID(sourceID)
	wf.AddPassthroughNode(branchID).AddInput(sourceID)

	endNodes := map[string]bool{compose.END: true}
	for _, edge := range edges {
		endNodes[edge.Target] = true
	}

	sortedEdges := append([]graphEdge(nil), edges...)
	sortEdgesByCanvas(sortedEdges, idx.nodes)

	branch := compose.NewGraphBranch(func(runCtx context.Context, _ map[string]any) (string, error) {
		rt := workflowRuntimeFrom(runCtx)
		if rt == nil {
			return compose.END, fmt.Errorf("workflow runtime missing in context")
		}
		for edgeIdx, edge := range sortedEdges {
			if edgeAllowed(edge, sourceNode, edgeIdx, rt.state) {
				return edge.Target, nil
			}
		}
		return compose.END, nil
	}, endNodes)
	wf.AddBranch(branchID, branch)

	for _, edge := range edges {
		if target, ok := nodeRefs[edge.Target]; ok {
			target.AddInput(branchID)
		}
	}
	return nil
}
