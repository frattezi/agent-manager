package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type surfaceArgs struct {
	Action    string `json:"action" jsonschema:"publish, list or show"`
	Document  string `json:"document,omitempty" jsonschema:"publish: the A2UI v0.9 document as JSON text, an array of createSurface, updateComponents and updateDataModel messages for one surface"`
	SessionID string `json:"session_id,omitempty" jsonschema:"list: the session whose surfaces to list; omit for this session's own"`
	SurfaceID string `json:"surface_id,omitempty" jsonschema:"show: the stored surface id that publish or list returned"`
	Width     int    `json:"width,omitempty" jsonschema:"show: cells to draw the surface in; omit for the default"`
}

type surfaceOutput struct {
	Surfaces []sessioncmd.AgentSurface   `json:"surfaces,omitempty" jsonschema:"the session's stored surfaces, oldest first, for action list"`
	Surface  *sessioncmd.AgentSurface    `json:"surface,omitempty" jsonschema:"the surface published"`
	Rendered *sessioncmd.RenderedSurface `json:"rendered,omitempty" jsonschema:"the surface drawn as text, for action show"`
}

type surfaceCommands interface {
	PublishSurface(sessionID string, document []byte) (sessioncmd.AgentSurface, error)
	Surfaces(sessionID, targetID string) ([]sessioncmd.AgentSurface, error)
	ShowSurface(sessionID, id string, width int) (sessioncmd.RenderedSurface, error)
}

func addSurface(server *mcp.Server, noCaller error, sessionID string, surfaces surfaceCommands) {
	addTool(server, noCaller, &mcp.Tool{
		Name: "surface",
		Description: "Agent Surfaces: ask the user something richer than prose, as a form the manager draws outside your pane; action picks the operation. " +
			"publish stores an A2UI document as this session's surface; a document the manager cannot read is refused with the field or version at fault and nothing is stored, so fix it and publish again, or ask in prose instead. " +
			"Only an agent session publishes, since the answer comes back into its conversation. " +
			"list reads the surfaces a session published, oldest first, and whether each is answered. show draws one stored surface as text.",
		Annotations: toolAnnotations(false, false, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args surfaceArgs) (*mcp.CallToolResult, surfaceOutput, error) {
		if err := localOnly("surface", args.SessionID); err != nil {
			return nil, surfaceOutput{}, err
		}
		switch args.Action {
		case "publish":
			if strings.TrimSpace(args.Document) == "" {
				return nil, surfaceOutput{}, fmt.Errorf("document is empty; pass the A2UI document as JSON text")
			}
			published, err := surfaces.PublishSurface(sessionID, []byte(args.Document))
			if err != nil {
				return nil, surfaceOutput{}, err
			}
			return textContent(sessioncmd.FormatSurfacePublished(published)), surfaceOutput{Surface: &published}, nil
		case "list":
			listed, err := surfaces.Surfaces(sessionID, args.SessionID)
			if err != nil {
				return nil, surfaceOutput{}, err
			}
			return textContent(sessioncmd.FormatSurfaceList(listed)), surfaceOutput{Surfaces: listed}, nil
		case "show":
			shown, err := surfaces.ShowSurface(sessionID, args.SurfaceID, args.Width)
			if err != nil {
				return nil, surfaceOutput{}, err
			}
			return textContent(strings.TrimRight(shown.Text, "\n")), surfaceOutput{Rendered: &shown}, nil
		default:
			return nil, surfaceOutput{}, fmt.Errorf("unknown action %q (publish, list, show)", args.Action)
		}
	})
}
