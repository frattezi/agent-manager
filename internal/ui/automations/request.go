package automations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
	"github.com/YoanWai/agent-manager/internal/fractoryhome"
)

// RequestVersion is the request contract this build writes.
const RequestVersion = 1

// instantLayout names a request's instant: UTC, to the second, and free of
// the colons Windows refuses in a file name.
const instantLayout = "20060102T150405Z"

// Request is version 1 of the request contract: the configuration as the
// store keeps it, and the instant it fired for.
type Request struct {
	Version     int             `json:"version"`
	RequestedAt time.Time       `json:"requestedAt"`
	FiredFor    time.Time       `json:"firedFor"`
	Automation  json.RawMessage `json:"automation"`
}

// RequestsDir is where Fractory picks requests up: $FRACTORY_HOME/automations.
func RequestsDir() string { return fractoryhome.Sub("automations") }

// RequestName is the file a request for the Automation id at firedFor lives
// in. The same id and instant always give the same name.
func RequestName(id string, firedFor time.Time) (string, error) {
	if err := validID(id); err != nil {
		return "", err
	}
	if err := validInstant(firedFor); err != nil {
		return "", err
	}
	return id + "@" + firedFor.UTC().Format(instantLayout) + ".json", nil
}

func validID(id string) error {
	if id == "" || id == "." || id == ".." || strings.HasPrefix(id, ".") {
		return fmt.Errorf("automation id %q cannot name a file", id)
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("automation id %q cannot name a file", id)
		}
	}
	return nil
}

func validInstant(t time.Time) error {
	utc := t.UTC()
	switch {
	case t.IsZero():
		return errors.New("fired-for instant is zero")
	case utc.Year() < 1 || utc.Year() > 9999:
		return fmt.Errorf("fired-for instant %v is outside years 1 to 9999", t)
	case utc.Nanosecond() != 0:
		// A name drops what is below a second, so two such instants would share one file.
		return fmt.Errorf("fired-for instant %v is not a whole second", t)
	}
	return nil
}

// WriteRequest asks Fractory to run the Automation whose stored configuration
// is config, for the instant firedFor. The file appears whole or not at all,
// and a request already on disk for that instant is left as it is. It
// reports whether a new request was written.
func WriteRequest(dir string, config []byte, firedFor, now time.Time) (bool, error) {
	var parsed Config
	if err := json.Unmarshal(config, &parsed); err != nil {
		return false, fmt.Errorf("configuration: %w", err)
	}
	name, err := RequestName(parsed.ID, firedFor)
	if err != nil {
		return false, err
	}
	out, err := json.Marshal(Request{
		Version:     RequestVersion,
		RequestedAt: now.UTC().Truncate(time.Second),
		FiredFor:    firedFor.UTC(),
		Automation:  config,
	})
	if err != nil {
		return false, err
	}
	err = atomicfile.CreateFile(filepath.Join(dir, name), append(out, '\n'), 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	return err == nil, err
}

// LoadRequests lists the requests in dir Fractory has yet to take, oldest
// instant first. A missing dir is none; a file it cannot use is skipped.
func LoadRequests(dir string) ([]Request, []Skip, error) {
	return loadDir(dir, parseRequest, func(a, b Request) int { return a.FiredFor.Compare(b.FiredFor) })
}

func parseRequest(data []byte) (Request, error) {
	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		return Request{}, err
	}
	switch {
	case request.Version != RequestVersion:
		return Request{}, fmt.Errorf("request version %d, this manager reads version %d", request.Version, RequestVersion)
	case request.FiredFor.IsZero() || len(request.Automation) == 0:
		return Request{}, errors.New("request is missing a required field")
	}
	return request, nil
}
