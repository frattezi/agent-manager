package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/config"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uiautomationspersist "github.com/YoanWai/agent-manager/internal/ui/automations/persist"
)

const (
	usageAutomationsList    = "automations list [--json]"
	usageAutomationsShow    = "automations show <id|name>"
	usageAutomationsAdd     = "automations add [--disabled] [file|-] [--json]"
	usageAutomationsChange  = "automations change <id|name> [file|-] [--json]"
	usageAutomationsRemove  = "automations remove <id|name> [--json]"
	usageAutomationsEnable  = "automations enable <id|name> [--json]"
	usageAutomationsDisable = "automations disable <id|name> [--json]"
)

func automationsSection() section {
	verbs := []command{
		{name: "list", usage: usageAutomationsList, about: "every Automation with its id, name, whether it is enabled and its schedule", run: automationsCommand(runAutomationsList)},
		{name: "show", usage: usageAutomationsShow, about: "print one Automation whole: its configuration and the state the manager keeps beside it", run: automationsCommand(runAutomationsShow)},
		{name: "add", usage: usageAutomationsAdd, about: "store a new Automation from a JSON configuration read from the file or stdin; the manager assigns its id unless the configuration names one", run: automationsCommand(runAutomationsAdd)},
		{name: "change", usage: usageAutomationsChange, about: "replace an Automation's configuration; its id, enabled flag and last fired instant stay", run: automationsCommand(runAutomationsChange)},
		{name: "remove", usage: usageAutomationsRemove, about: "delete an Automation so it never fires again", run: automationsCommand(runAutomationsRemove)},
		{name: "enable", usage: usageAutomationsEnable, about: "let an Automation fire on its schedule", run: automationsCommand(runAutomationsEnable)},
		{name: "disable", usage: usageAutomationsDisable, about: "pause an Automation without losing how it is set up", run: automationsCommand(runAutomationsDisable)},
	}
	return groupSection("Automations", "automations", "the scheduled work the manager remembers and hands to Fractory when it comes due", verbs)
}

func automationsCommand(run func(io.Writer, io.Reader, []string, string, time.Time) error) Command {
	return func(args []string, _ func() string, configDir string) error {
		return run(os.Stdout, os.Stdin, args, configDir, time.Now())
	}
}

type automationRecord struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Enabled      bool            `json:"enabled"`
	LastFiredFor *time.Time      `json:"lastFiredFor"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	Config       json.RawMessage `json:"config"`
}

func newAutomationRecord(a store.Automation) automationRecord {
	record := automationRecord{ID: a.ID, Name: a.Name, Enabled: a.Enabled, CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC(), Config: a.Config}
	if !a.LastFiredFor.IsZero() {
		fired := a.LastFiredFor.UTC()
		record.LastFiredFor = &fired
	}
	return record
}

func automationTools() ([]string, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	return cfg.AgentToolNames(), nil
}

// findAutomation takes an id, or a name exactly one Automation carries.
func findAutomation(st *store.Store, key string) (store.Automation, error) {
	row, err := automations.Find(uiautomationspersist.Book{Store: st}, key)
	if err != nil {
		return store.Automation{}, err
	}
	a, _, err := st.Automation(row.ID)
	return a, err
}

func readAutomationConfig(input io.Reader, operands []string) (automations.Config, []string, error) {
	if len(operands) == 1 && operands[0] != "-" {
		file, err := os.Open(operands[0])
		if err != nil {
			return automations.Config{}, nil, err
		}
		defer file.Close()
		input = file
	}
	data, err := io.ReadAll(input)
	if err != nil {
		return automations.Config{}, nil, err
	}
	tools, err := automationTools()
	if err != nil {
		return automations.Config{}, nil, err
	}
	config, err := automations.Parse(data, tools)
	return config, tools, err
}

func runAutomationsList(out io.Writer, _ io.Reader, args []string, configDir string, _ time.Time) error {
	set := newFlagSet(usageAutomationsList)
	asJSON := jsonFlag(set)
	if _, err := parseCommand(out, set, args, 0, 0); err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	all, err := st.Automations()
	if err != nil {
		return err
	}
	records := make([]automationRecord, 0, len(all))
	lines := make([]string, 0, len(all))
	for _, a := range all {
		records = append(records, newAutomationRecord(a))
		state := "enabled"
		if !a.Enabled {
			state = "disabled"
		}
		var config automations.Config
		_ = json.Unmarshal(a.Config, &config)
		lines = append(lines, strings.Join([]string{a.ID, a.Name, state, config.Cron}, "  "))
	}
	if *asJSON {
		return writeJSON(out, records)
	}
	if len(lines) == 0 {
		return nil
	}
	_, err = fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func runAutomationsShow(out io.Writer, _ io.Reader, args []string, configDir string, _ time.Time) error {
	set := newFlagSet(usageAutomationsShow)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	a, err := findAutomation(st, operands[0])
	if err != nil {
		return err
	}
	return writeJSON(out, newAutomationRecord(a))
}

func runAutomationsAdd(out io.Writer, input io.Reader, args []string, configDir string, now time.Time) error {
	set := newFlagSet(usageAutomationsAdd)
	disabled := set.Bool("disabled", false, "store the Automation paused, so it fires only once enabled")
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 0, 1)
	if err != nil {
		return err
	}
	cfg, tools, err := readAutomationConfig(input, operands)
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	if cfg, err = automations.Add(uiautomationspersist.Book{Store: st}, cfg, !*disabled, tools, now); err != nil {
		return err
	}
	return emitAutomation(out, st, *asJSON, cfg.ID, "added "+cfg.Name+" ("+cfg.ID+")")
}

func runAutomationsChange(out io.Writer, input io.Reader, args []string, configDir string, now time.Time) error {
	set := newFlagSet(usageAutomationsChange)
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 1, 2)
	if err != nil {
		return err
	}
	cfg, tools, err := readAutomationConfig(input, operands[1:])
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	current, err := findAutomation(st, operands[0])
	if err != nil {
		return err
	}
	if cfg, err = automations.Change(uiautomationspersist.Book{Store: st}, current.ID, cfg, tools, now); err != nil {
		return err
	}
	return emitAutomation(out, st, *asJSON, current.ID, "changed "+cfg.Name+" ("+current.ID+")")
}

func runAutomationsRemove(out io.Writer, _ io.Reader, args []string, configDir string, _ time.Time) error {
	set := newFlagSet(usageAutomationsRemove)
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	a, err := findAutomation(st, operands[0])
	if err != nil {
		return err
	}
	if err := automations.Remove(uiautomationspersist.Book{Store: st}, a.ID); err != nil {
		return err
	}
	return emit(out, *asJSON, newAutomationRecord(a), "removed "+a.Name+" ("+a.ID+")")
}

func runAutomationsEnable(out io.Writer, _ io.Reader, args []string, configDir string, now time.Time) error {
	return setAutomationEnabled(out, usageAutomationsEnable, true, args, configDir, now)
}

func runAutomationsDisable(out io.Writer, _ io.Reader, args []string, configDir string, now time.Time) error {
	return setAutomationEnabled(out, usageAutomationsDisable, false, args, configDir, now)
}

func setAutomationEnabled(out io.Writer, usage string, enabled bool, args []string, configDir string, now time.Time) error {
	set := newFlagSet(usage)
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	a, err := findAutomation(st, operands[0])
	if err != nil {
		return err
	}
	if err := automations.SetEnabled(uiautomationspersist.Book{Store: st}, a.ID, enabled, now); err != nil {
		return err
	}
	verb := "enabled "
	if !enabled {
		verb = "disabled "
	}
	return emitAutomation(out, st, *asJSON, a.ID, verb+a.Name+" ("+a.ID+")")
}

func emitAutomation(out io.Writer, st *store.Store, asJSON bool, id, human string) error {
	if !asJSON {
		return emit(out, false, nil, human)
	}
	a, _, err := st.Automation(id)
	if err != nil {
		return err
	}
	return writeJSON(out, newAutomationRecord(a))
}
