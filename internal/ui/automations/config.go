// Package automations holds the Automation contracts the manager shares
// with Fractory: the configuration an owner writes, and the rules every
// front checks it against before the store keeps it.
package automations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	// Windows ships no zoneinfo, so a timezone is checked against the
	// database built into the binary on every platform.
	_ "time/tzdata"
)

// Version is the configuration contract this build reads and writes.
const Version = 1

const (
	KindAgent    = "agent"
	KindScript   = "script"
	ModeWork     = "work"
	ModePersonal = "personal"
)

// Config is version 1 of the configuration contract. Its bytes travel into
// a request file unchanged, so the field names are Fractory's.
type Config struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	RepoPath  string   `json:"repoPath"`
	Cron      string   `json:"cron"`
	Timezone  string   `json:"timezone"`
	Mode      string   `json:"mode"`
	Tool      string   `json:"tool"`
	Flags     []string `json:"flags"`
	Entry     string   `json:"entry"`
	Retention int      `json:"retention"`
}

// FieldError names the configuration field a refusal is about.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

func refuse(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// Parse reads a configuration document and checks it against tools, the
// CLI names the manager knows. A document without a version is version 1;
// an id is left as written, empty when the store has yet to assign one.
func Parse(data []byte, tools []string) (Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, decodeError(err)
	}
	if decoder.More() {
		return Config{}, errors.New("configuration: more than one JSON document")
	}
	if config.Version == 0 {
		config.Version = Version
	}
	if config.Flags == nil {
		config.Flags = []string{}
	}
	return config, config.Validate(tools)
}

func decodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return refuse(typeErr.Field, "is a JSON %s, want %s", typeErr.Value, jsonType(typeErr.Type.Kind().String()))
	}
	if field, found := strings.CutPrefix(err.Error(), "json: unknown field "); found {
		return refuse(strings.Trim(field, `"`), "is not a configuration field")
	}
	return fmt.Errorf("configuration: %w", err)
}

func jsonType(kind string) string {
	switch kind {
	case "int":
		return "a number"
	case "slice":
		return "a list of strings"
	default:
		return "a " + kind
	}
}

// Validate refuses a configuration the manager could never fire, naming the
// first field that is wrong.
func (c Config) Validate(tools []string) error {
	if c.Version != Version {
		return refuse("version", "is %d, this manager reads version %d", c.Version, Version)
	}
	if strings.TrimSpace(c.Name) == "" {
		return refuse("name", "is empty")
	}
	if c.Kind != KindAgent && c.Kind != KindScript {
		return refuse("kind", "is %q, want %q or %q", c.Kind, KindAgent, KindScript)
	}
	if c.RepoPath == "" {
		return refuse("repoPath", "is empty")
	}
	if !filepath.IsAbs(c.RepoPath) {
		return refuse("repoPath", "%q is not an absolute path", c.RepoPath)
	}
	if err := ValidateCron(c.Cron); err != nil {
		return refuse("cron", "%q %v", c.Cron, err)
	}
	if c.Timezone == "" || c.Timezone == "Local" {
		return refuse("timezone", "is %q, want an IANA name such as Europe/Lisbon", c.Timezone)
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return refuse("timezone", "%q is not an IANA timezone", c.Timezone)
	}
	if c.Mode != ModeWork && c.Mode != ModePersonal {
		return refuse("mode", "is %q, want %q or %q", c.Mode, ModeWork, ModePersonal)
	}
	if c.Kind == KindAgent && c.Tool == "" {
		return refuse("tool", "is empty; an agent Automation names the CLI that runs it: %s", strings.Join(tools, ", "))
	}
	if c.Tool != "" && !slices.Contains(tools, c.Tool) {
		return refuse("tool", "is %q, want one of %s", c.Tool, strings.Join(tools, ", "))
	}
	if strings.TrimSpace(c.Entry) == "" {
		return refuse("entry", "is empty")
	}
	if c.Retention < 1 {
		return refuse("retention", "is %d, want at least 1 run kept", c.Retention)
	}
	return nil
}

// Encode is the document the store keeps and a request carries.
func (c Config) Encode() ([]byte, error) {
	if c.Flags == nil {
		c.Flags = []string{}
	}
	return json.Marshal(c)
}
