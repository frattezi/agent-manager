package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTool registers a tool that refuses outright when the server has no
// calling session, so no tool, present or future, opens the workspace to a
// CLI running outside Agent Manager.
func addTool[In, Out any](server *mcp.Server, noCaller error, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error) {
		if noCaller != nil {
			var none Out
			return nil, none, noCaller
		}
		return handler(ctx, req, args)
	})
}

func toolAnnotations(readOnly, destructive, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  readOnly,
		OpenWorldHint:   &openWorld,
	}
}
