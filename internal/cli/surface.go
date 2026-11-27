package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
)

const (
	usageSurfacePublish = "surface publish [file|-] [--json]"
	usageSurfaceList    = "surface list [<session-id>] [--json]"
	usageSurfaceShow    = "surface show <surface-id> [--width <cells>] [--json]"
)

type surfaceCommands interface {
	PublishSurface(sessionID string, document []byte) (sessioncmd.AgentSurface, error)
	Surfaces(sessionID, targetID string) ([]sessioncmd.AgentSurface, error)
	ShowSurface(sessionID, id string, width int) (sessioncmd.RenderedSurface, error)
}

func newSurfaces(configDir string) surfaceCommands {
	return sessioncmd.NewSessions(configDir, sessioncmd.CLIVocabulary())
}

func surfaceVerbs() []command {
	return surfaceVerbsWith(newSurfaces, os.Stdin)
}

func surfaceVerbsWith(open func(string) surfaceCommands, input io.Reader) []command {
	publish := func(out io.Writer, surfaces surfaceCommands, args []string, sessionID string) error {
		return runSurfacePublish(out, input, surfaces, args, sessionID)
	}
	return []command{
		{name: "publish", usage: usageSurfacePublish, about: "ask the user something richer than prose: store an A2UI document, read from the file or stdin, as this session's Agent Surface; a refusal names the field to fix", run: bind(open, publish)},
		{name: "list", usage: usageSurfaceList, about: "the Agent Surfaces a session published, oldest first, and whether each is answered; omit the session to list your own", run: bind(open, runSurfaceList)},
		{name: "show", usage: usageSurfaceShow, about: "draw one stored Agent Surface as text", run: bind(open, runSurfaceShow)},
	}
}

const surfaceAbout = "what a session asks the user in a form instead of prose, and the rounds it asked before"

func surfaceSection() section {
	return groupSection("Agent Surfaces", "surface", surfaceAbout, surfaceVerbs())
}

func runSurfacePublish(out io.Writer, input io.Reader, surfaces surfaceCommands, args []string, sessionID string) error {
	set := newFlagSet(usageSurfacePublish)
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 0, 1)
	if err != nil {
		return err
	}
	if len(operands) == 1 && operands[0] != "-" {
		file, err := os.Open(operands[0])
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}
	document, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	if len(document) == 0 {
		return errors.New("the document is empty; pass an A2UI document as a file or on stdin; usage: agent-manager " + usageSurfacePublish)
	}
	published, err := surfaces.PublishSurface(sessionID, document)
	if err != nil {
		return err
	}
	return emit(out, *asJSON, published, sessioncmd.FormatSurfacePublished(published))
}

func runSurfaceList(out io.Writer, surfaces surfaceCommands, args []string, sessionID string) error {
	set := newFlagSet(usageSurfaceList)
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 0, 1)
	if err != nil {
		return err
	}
	target := ""
	if len(operands) == 1 {
		target = operands[0]
	}
	listed, err := surfaces.Surfaces(sessionID, target)
	if err != nil {
		return err
	}
	return emit(out, *asJSON, listed, sessioncmd.FormatSurfaceList(listed))
}

func runSurfaceShow(out io.Writer, surfaces surfaceCommands, args []string, sessionID string) error {
	set := newFlagSet(usageSurfaceShow)
	width := set.Int("width", 0, fmt.Sprintf("cells to draw the surface in, default %d", sessioncmd.DefaultSurfaceWidth))
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	shown, err := surfaces.ShowSurface(sessionID, operands[0], *width)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(out, shown)
	}
	return emit(out, false, nil, strings.TrimRight(shown.Text, "\n"))
}
