package automations

import (
	"os"
	"strings"
	"time"
)

// LocalZone is the IANA name of this machine's timezone, which a new
// Automation is offered, or empty when the machine does not say: Go names
// the local zone only "Local". TZ wins, then the zoneinfo file
// /etc/localtime links to, on the systems that have one.
func LocalZone() string {
	if name := strings.TrimPrefix(os.Getenv("TZ"), ":"); name != "" {
		if _, err := time.LoadLocation(name); err == nil && name != "Local" {
			return name
		}
	}
	link, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	_, name, found := strings.Cut(link, "zoneinfo/")
	if !found {
		return ""
	}
	if _, err := time.LoadLocation(name); err != nil {
		return ""
	}
	return name
}
