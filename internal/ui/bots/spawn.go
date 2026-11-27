package bots

import (
	"regexp"
	"strconv"
	"strings"
)

// Bot is the Session a spawn from a Profile opens. It starts on an empty
// composer: its Specialization and memory reach it as configuration.
type Bot struct {
	Name string
}

var nameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Spawn is the Bot a spawn from p opens. taken are the names of the sessions
// already running, so a second Bot from the same Profile is told apart by the
// lowest number no session holds.
func Spawn(p Profile, taken []string) Bot {
	slug := slug(p)
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
	return Bot{Name: name}
}

// slug is the Profile's name as its Bots and its own directory are named,
// else its id when the name has nothing a file name may carry.
func slug(p Profile) string {
	s := strings.ToLower(strings.Trim(nameChars.ReplaceAllString(p.Name, "-"), "-"))
	if s == "" {
		s = strings.Trim(nameChars.ReplaceAllString(p.ID, "-"), "-")
	}
	return s
}
