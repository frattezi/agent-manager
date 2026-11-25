package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Modes a Run or repo can be in.
const (
	ModeWork     = "work"
	ModePersonal = "personal"
)

// Unknown is shown for a rule the manager cannot derive; it never guesses.
const Unknown = "unknown"

// RepoConfig is the part of a repo's `.fractory.json` that decides the
// workflow rules. A nil TicketGate is a file that does not say.
type RepoConfig struct {
	Mode       string `json:"mode"`
	Flow       string `json:"flow"`
	TicketGate *bool  `json:"ticketGate"`
}

// ReadRepoConfig reads `.fractory.json` from repoPath; nil when it is missing
// or malformed, so callers fall back to the RunRecord.
func ReadRepoConfig(repoPath string) *RepoConfig {
	if repoPath == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(repoPath, ".fractory.json"))
	if err != nil {
		return nil
	}
	var config RepoConfig
	if json.Unmarshal(data, &config) != nil {
		return nil
	}
	return &config
}

// Rules are the workflow rules a Mode implies, mirroring Fractory's policy.
type Rules struct {
	Merge      string
	TicketGate string
	Agents     string
}

// EffectiveMode is the Run's own mode, else its repo's, else "".
func (r Record) EffectiveMode() string {
	if r.Mode != "" {
		return r.Mode
	}
	if r.Config != nil {
		return r.Config.Mode
	}
	return ""
}

// EffectiveFlow is the Run's own flow, else its repo's.
func (r Record) EffectiveFlow() string {
	if r.Flow != "" {
		return r.Flow
	}
	if r.Config != nil {
		return r.Config.Flow
	}
	return ""
}

// RulesFor derives the rules for a mode. ticketGate is the repo's setting, nil
// when unknown; Work Mode always gates tickets, Personal Mode only when asked.
func RulesFor(mode string, ticketGate *bool) Rules {
	switch mode {
	case ModeWork:
		return Rules{
			Merge:      "PR Gate: human merges",
			TicketGate: "on",
			Agents:     "claude@work",
		}
	case ModePersonal:
		rules := Rules{
			Merge:      "Auto-merge when the Run is done, mergeable and checks pass (or no CI)",
			TicketGate: Unknown,
			Agents:     "claude@personal, opencode@personal",
		}
		if ticketGate != nil {
			rules.TicketGate = "off"
			if *ticketGate {
				rules.TicketGate = "on"
			}
		}
		return rules
	}
	return Rules{Merge: Unknown, TicketGate: Unknown, Agents: Unknown}
}

// Rules is the workflow rules of this Run's repo.
func (r Record) Rules() Rules {
	var gate *bool
	if r.Config != nil {
		gate = r.Config.TicketGate
	}
	return RulesFor(r.EffectiveMode(), gate)
}
