package multiagent

import (
	"context"
	"strings"

	"cyberstrike-ai/internal/authctx"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func isLocalPrivilegeTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "execute", "ls", "read_file", "write_file", "edit_file", "glob", "grep":
		return true
	default:
		return false
	}
}

func localToolPermissionDenied(ctx context.Context, name string) bool {
	if !isLocalPrivilegeTool(name) {
		return false
	}
	principal, ok := authctx.PrincipalFromContext(ctx)
	return !ok || !principal.HasPermission("agent:local-execute")
}

func localToolRBACMiddleware() compose.ToolMiddleware {
	denied := "Permission denied: agent:local-execute is required for local filesystem and shell tools."
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				if input != nil && localToolPermissionDenied(ctx, input.Name) {
					return &compose.ToolOutput{Result: denied}, nil
				}
				return next(ctx, input)
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
				if input != nil && localToolPermissionDenied(ctx, input.Name) {
					return &compose.StreamToolOutput{Result: schema.StreamReaderFromArray([]string{denied})}, nil
				}
				return next(ctx, input)
			}
		},
	}
}
