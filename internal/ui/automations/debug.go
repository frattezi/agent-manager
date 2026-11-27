package automations

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrUnfinished refuses a Debug Session on a firing that is still queued or
// running: there is nothing finished to look into yet.
var ErrUnfinished = errors.New("this firing has not finished yet, so there is nothing to debug; open its log to follow it")

// Debug is the Debug Session a finished firing opens: a chat in the
// Automation's repository whose first prompt says which firing it is about.
type Debug struct {
	Name   string
	Dir    string
	Prompt string
}

var debugNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// DebugSession is the session a Debug Session on record opens. The prompt
// names the Automation, the instant, the outcome and where the log and the
// output directory are; the worktree is gone by then. It carries paths only, so the agent reads what it needs and
// nothing the firing printed is pasted in.
func DebugSession(config Config, record Record) (Debug, error) {
	if !record.Finished() {
		return Debug{}, ErrUnfinished
	}
	firedFor := record.FiredFor
	if zone, err := time.LoadLocation(config.Timezone); err == nil {
		firedFor = firedFor.In(zone)
	}
	slug := strings.Trim(debugNameChars.ReplaceAllString(config.Name, "-"), "-")
	if slug == "" {
		slug = strings.Trim(debugNameChars.ReplaceAllString(config.ID, "-"), "-")
	}
	name := "debug-" + slug + "-" + firedFor.Format("20060102-1504")

	outcome := record.Outcome()
	if outcome != record.Status {
		outcome += " (status " + record.Status + ")"
	}
	logLine := "Its log is at " + strconv.Quote(record.LogPath) + "."
	if record.LogPath == "" {
		logLine = "Its record names no log."
	}
	outputLine := "Its output directory is at " + strconv.Quote(record.OutputPath) + "."
	if record.OutputPath == "" {
		outputLine = "Its record names no output directory."
	}
	lines := []string{
		fmt.Sprintf("Look into a firing of the %s Automation %q (id %s), which runs %q in this repository.", config.Kind, config.Name, config.ID, config.Entry),
		"It fired for " + firedFor.Format(time.RFC3339) + " and its outcome was " + outcome + ".",
		logLine,
		outputLine,
		"Read them to find out what it did and why it ended that way.",
	}
	return Debug{Name: name, Dir: config.RepoPath, Prompt: strings.Join(lines, "\n")}, nil
}

// Group is a group of the workspace as ProjectGroup weighs it.
type Group struct {
	Path     string
	Dir      string
	Archived bool
}

// ProjectGroup is the group a Debug Session on repo goes under: the live
// group whose default directory is repo, the shallowest when several are.
// When none is, it names a new group after the repository, one no existing
// group already holds, and reports that it must be created.
func ProjectGroup(repo string, groups []Group) (string, bool) {
	repo = filepath.Clean(repo)
	var matches []string
	taken := map[string]bool{}
	for _, group := range groups {
		taken[group.Path] = true
		if !group.Archived && group.Dir != "" && filepath.Clean(group.Dir) == repo {
			matches = append(matches, group.Path)
		}
	}
	if len(matches) > 0 {
		slices.SortFunc(matches, func(a, b string) int {
			if depth := strings.Count(a, "/") - strings.Count(b, "/"); depth != 0 {
				return depth
			}
			return strings.Compare(a, b)
		})
		return matches[0], false
	}
	base := filepath.Base(repo)
	name := base
	for n := 2; taken[name]; n++ {
		name = base + "-" + strconv.Itoa(n)
	}
	return name, true
}
