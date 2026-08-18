package workflow

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// compileAgentSubgraph wraps an Agent canvas node as an Eino subgraph (AddGraphNode best practice).
func compileAgentSubgraph(_ context.Context, node graphNode) (compose.AnyGraph, error) {
	n := node
	prepareID := n.ID + "__agent_prepare"
	executeID := n.ID + "__agent_execute"
	finalizeID := n.ID + "__agent_finalize"
	g := compose.NewGraph[WorkflowNodeOutput, WorkflowNodeOutput]()
	_ = g.AddLambdaNode(prepareID, compose.InvokableLambda(func(_ context.Context, input WorkflowNodeOutput) (WorkflowNodeOutput, error) {
		if input == nil {
			input = WorkflowNodeOutput{}
		}
		input["agent_subgraph_stage"] = "prepare"
		input["agent_node_id"] = n.ID
		return input, nil
	}))
	_ = g.AddLambdaNode(executeID, compose.InvokableLambda(func(runCtx context.Context, _ WorkflowNodeOutput) (WorkflowNodeOutput, error) {
		return runWorkflowNodeLambda(runCtx, n)
	}))
	_ = g.AddLambdaNode(finalizeID, compose.InvokableLambda(func(_ context.Context, output WorkflowNodeOutput) (WorkflowNodeOutput, error) {
		if output == nil {
			output = WorkflowNodeOutput{}
		}
		output["agent_subgraph_stage"] = "finalize"
		output["agent_node_id"] = n.ID
		return output, nil
	}))
	if err := g.AddEdge(compose.START, prepareID); err != nil {
		return nil, err
	}
	if err := g.AddEdge(prepareID, executeID); err != nil {
		return nil, err
	}
	if err := g.AddEdge(executeID, finalizeID); err != nil {
		return nil, err
	}
	if err := g.AddEdge(finalizeID, compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
