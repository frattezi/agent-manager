package bots

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Bot is the Session a spawn from a Profile opens: its name, and the first
// prompt that specializes it.
type Bot struct {
	Name   string
	Prompt string
}

var nameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Spawn is the Bot a spawn from p opens. taken are the names of the sessions
// already running, so a second Bot from the same Profile is told apart by the
// lowest number no session holds.
func Spawn(p Profile, taken []string) Bot {
	slug := strings.ToLower(strings.Trim(nameChars.ReplaceAllString(p.Name, "-"), "-"))
	if slug == "" {
		slug = strings.Trim(nameChars.ReplaceAllString(p.ID, "-"), "-")
	}
	held := make(map[string]bool, len(taken))
	for _, name := range taken {
		held[name] = true
	}
	name := ""
	for n := 1; ; n++ {
		if name = slug + "-" + strconv.Itoa(n); !held[name] {
			break
		}
	}
	return Bot{Name: name, Prompt: FirstPrompt(p)}
}

// FirstPrompt is assembled at spawn time and never stored, so a better one
// reaches every Profile with a release. It names the Specialization to load
// and the ai-memory project to recall from; a Profile with neither starts
// its Bot on the CLI's own empty prompt.
func FirstPrompt(p Profile) string {
	var lines []string
	if p.Specialization != "" {
		lines = append(lines, fmt.Sprintf("Load the %s skill now and work as it says for the rest of this conversation.", p.Specialization))
	}
	if p.MemoryProject != "" {
		lines = append(lines, fmt.Sprintf("Your memory is the ai-memory project %q: recall from it before you answer anything, and store what you conclude there, so the next Bot from this Profile starts where you leave off.", p.MemoryProject))
	}
	if len(lines) == 0 {
		return ""
	}
	head := fmt.Sprintf("You are a Bot started from the Profile %q.", p.Name)
	tail := "Then say in one line that you are ready, and wait for the owner to ask."
	return strings.Join(append(append([]string{head}, lines...), tail), "\n")
}
