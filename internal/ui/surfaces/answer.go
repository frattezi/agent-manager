package surfaces

import (
	"fmt"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
)

// AnswerText is the round as the owner answered it, read against the round
// as it was published: each question in order, what was chosen, and whether
// that is the answer the agent recommended or the owner's override. It is
// what the session reads and what the store keeps.
func AnswerText(published, answered a2ui.Document) string {
	asked, given := a2ui.Fields(&published), a2ui.Fields(&answered)
	lines := []string{}
	for i, field := range given {
		label := strings.TrimSpace(field.Label)
		if label == "" {
			label = fmt.Sprintf("Question %d", i+1)
		}
		lines = append(lines, label)
		var before *a2ui.Node
		if i < len(asked) {
			before = asked[i]
		}
		lines = append(lines, "  "+fieldAnswer(before, field))
	}
	if len(lines) == 0 {
		return "(the round asked nothing to answer; the user sent it as it was)"
	}
	return strings.Join(lines, "\n")
}

func fieldAnswer(before, after *a2ui.Node) string {
	switch after.Type {
	case "ChoicePicker":
		now := chosenLabels(after)
		if before == nil {
			return now
		}
		if was := chosenLabels(before); was != now {
			return now + " (overriding the recommended: " + was + ")"
		}
		return now + " (the recommended answer)"
	case "CheckBox":
		now := checked(after.Checked)
		if before == nil {
			return now
		}
		if before.Checked != after.Checked {
			return now + " (overriding the recommended: " + checked(before.Checked) + ")"
		}
		return now + " (the recommended answer)"
	case "TextField":
		if strings.TrimSpace(after.Value) == "" {
			return "(left empty)"
		}
		return after.Value
	}
	return ""
}

func chosenLabels(n *a2ui.Node) string {
	var chosen []string
	for _, o := range n.Options {
		if o.Selected {
			label := o.Label
			if o.Value != "" && o.Value != o.Label {
				label += " [" + o.Value + "]"
			}
			chosen = append(chosen, label)
		}
	}
	if len(chosen) == 0 {
		return "(none chosen)"
	}
	return strings.Join(chosen, "; ")
}

func checked(on bool) string {
	if on {
		return "checked"
	}
	return "unchecked"
}

// AnswerRequest sends the owner's answer to one surface: it is recorded on
// the surface and queued on the session's inbox, which types it in once
// the pane is at rest. Nothing is written to the pane here.
type AnswerRequest struct {
	SessionID string
	// SurfaceID is the stored surface; SurfaceName the surfaceId its
	// document gave, which the session wrote and recognises.
	SurfaceID   string
	SurfaceName string
	Answer      string
}

func (AnswerRequest) EffectName() string { return "Agent Surface answer" }

func (r AnswerRequest) Run(services Services) (any, error) {
	if err := services.Store.AnswerSurface(r); err != nil {
		return Inbox{}, err
	}
	latest, err := services.Store.LatestSurfaces()
	if err != nil {
		return Inbox{}, err
	}
	return NewInbox(latest), nil
}
