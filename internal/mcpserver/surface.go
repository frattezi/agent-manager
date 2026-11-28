package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type surfaceArgs struct {
	Action    string `json:"action" jsonschema:"publish, list, show or answer"`
	Document  string `json:"document,omitempty" jsonschema:"publish: the A2UI v0.9 document as JSON text, an array of createSurface, updateComponents and updateDataModel messages for one surface"`
	SessionID string `json:"session_id,omitempty" jsonschema:"list: the session whose surfaces to list; omit for this session's own"`
	SurfaceID string `json:"surface_id,omitempty" jsonschema:"show and answer: the stored surface id that publish or list returned"`
	Width     int    `json:"width,omitempty" jsonschema:"show: cells to draw the surface in; omit for the default"`
	// IncludeDocument cannot share document's name: that one carries the
	// document in, for publish.
	IncludeDocument bool   `json:"include_document,omitempty" jsonschema:"show: also return the A2UI document as it was published, byte for byte, for a host that draws it itself"`
	Delivered       bool   `json:"delivered,omitempty" jsonschema:"answer: true when the asking session already received the answer as a user turn; only that session may set it, and nothing is queued"`
	Answer          string `json:"answer,omitempty" jsonschema:"answer: what the user answered, as the text the session that asked will read"`
}

type surfaceOutput struct {
	Surfaces []sessioncmd.AgentSurface   `json:"surfaces,omitempty" jsonschema:"the session's stored surfaces, oldest first, for action list"`
	Surface  *sessioncmd.AgentSurface    `json:"surface,omitempty" jsonschema:"the surface published or answered"`
	Rendered *sessioncmd.RenderedSurface `json:"rendered,omitempty" jsonschema:"the surface drawn as text, and its document when asked for, for action show"`
}

type surfaceCommands interface {
	PublishSurface(sessionID string, document []byte) (sessioncmd.AgentSurface, error)
	Surfaces(sessionID, targetID string) ([]sessioncmd.AgentSurface, error)
	ShowSurface(sessionID, id string, width int, withDocument bool) (sessioncmd.RenderedSurface, error)
	AnswerSurface(sessionID, id, answer string, delivered bool) (sessioncmd.AgentSurface, error)
}

func addSurface(server *mcp.Server, noCaller error, sessionID string, surfaces surfaceCommands) {
	addTool(server, noCaller, &mcp.Tool{
		Name: "surface",
		Description: "Agent Surfaces: ask the user something richer than prose, as a form the manager draws outside your pane; action picks the operation. " +
			"publish stores an A2UI document as this session's surface; a document the manager cannot read is refused with the field or version at fault and nothing is stored, so fix it and publish again, or ask in prose instead. " +
			"Only an agent session publishes, since the answer comes back into its conversation. " +
			"list reads the surfaces a session published, oldest first, and whether each is answered. show draws one stored surface as text, and with include_document also returns the A2UI document as it was published, for a host that draws the form itself. " +
			"answer records the user's answer to another session's surface, as sending from the manager's card does: the round is marked answered and the answer is typed into the session that asked once it is at rest; with delivered, the session that asked records an answer it already received as a user turn and nothing is queued; a surface that is unknown or already answered is refused.",
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
			shown, err := surfaces.ShowSurface(sessionID, args.SurfaceID, args.Width, args.IncludeDocument)
			if err != nil {
				return nil, surfaceOutput{}, err
			}
			return textContent(sessioncmd.FormatSurfaceShown(shown)), surfaceOutput{Rendered: &shown}, nil
		case "answer":
			answered, err := surfaces.AnswerSurface(sessionID, args.SurfaceID, args.Answer, args.Delivered)
			if err != nil {
				return nil, surfaceOutput{}, err
			}
			return textContent(sessioncmd.FormatSurfaceAnswered(answered)), surfaceOutput{Surface: &answered}, nil
		default:
			return nil, surfaceOutput{}, fmt.Errorf("unknown action %q (publish, list, show, answer)", args.Action)
		}
	})
}
