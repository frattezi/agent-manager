package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/bots"
	"github.com/google/uuid"
)

const (
	usageBotsList   = "bots list [--json]"
	usageBotsShow   = "bots show <id|name>"
	usageBotsCreate = "bots create --name <name> --tool <cli> [--emoji e] [--color n] [--model m] [--dir d] [--flags f] [--specialization s] [--memory p] [--json]"
	usageBotsEdit   = "bots edit <id|name> [--name n] [--emoji e] [--color n] [--tool t] [--model m] [--dir d] [--flags f] [--specialization s] [--memory p] [--json]"
)

func botsSection(specs settingSpecs) section {
	verbs := []command{
		{name: "list", usage: usageBotsList, about: "every Bot Profile with its id, emoji, name, CLI and model", run: botsCommand(specs, runBotsList)},
		{name: "show", usage: usageBotsShow, about: "print one Bot Profile whole", run: botsCommand(specs, runBotsShow)},
		{name: "create", usage: usageBotsCreate, about: "store a new Bot Profile; each value is checked against the ones its field takes", run: botsCommand(specs, runBotsCreate)},
		{name: "edit", usage: usageBotsEdit, about: "change the fields given and keep the rest; its id never changes", run: botsCommand(specs, runBotsEdit)},
	}
	return groupSection("Bots", "bots", "the Bot Profiles the manager starts specialized sessions from", verbs)
}

func botsCommand(specs settingSpecs, run func(io.Writer, settingSpecs, []string, string, time.Time) error) Command {
	return func(args []string, _ func() string, configDir string) error {
		return run(os.Stdout, specs, args, configDir, time.Now())
	}
}

type botRecord struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Emoji          string    `json:"emoji"`
	Color          int       `json:"color"`
	Tool           string    `json:"tool"`
	Model          string    `json:"model"`
	Dir            string    `json:"dir"`
	Flags          string    `json:"flags"`
	Specialization string    `json:"specialization"`
	MemoryProject  string    `json:"memoryProject"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func newBotRecord(p store.BotProfile) botRecord {
	return botRecord{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC(),
	}
}

func botProfile(p store.BotProfile) bots.Profile {
	return bots.Profile{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
	}
}

func storedBotProfile(p bots.Profile) store.BotProfile {
	return store.BotProfile{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
	}
}

func findBotProfile(st *store.Store, key string) (store.BotProfile, error) {
	stored, err := st.BotProfiles()
	if err != nil {
		return store.BotProfile{}, err
	}
	profiles := make([]bots.Profile, len(stored))
	for i, p := range stored {
		profiles[i] = botProfile(p)
	}
	found, err := bots.Find(profiles, key)
	if err != nil {
		return store.BotProfile{}, err
	}
	p, _, err := st.BotProfile(found.ID)
	return p, err
}

// botFlags declares one flag per Profile field.
func botFlags(set *flag.FlagSet) map[string]*string {
	values := make(map[string]*string, len(bots.Fields))
	for _, field := range bots.Fields {
		values[field.Key] = set.String(field.Flag, "", field.About)
	}
	return values
}

// applyBotFlags writes the fields the command line named onto p, each
// checked against its SettingSpecs entry, so the shell refuses what the
// dialog would never write.
func applyBotFlags(set *flag.FlagSet, values map[string]*string, specs settingSpecs, p *bots.Profile) (int, error) {
	all, err := specs()
	if err != nil {
		return 0, err
	}
	byKey := make(map[string]store.SettingSpec, len(all))
	for _, spec := range all {
		if spec.Bot {
			byKey[spec.Key] = spec
		}
	}
	given := map[string]bool{}
	set.Visit(func(f *flag.Flag) { given[f.Name] = true })
	changed := 0
	for _, field := range bots.Fields {
		if !given[field.Flag] {
			continue
		}
		value := *values[field.Key]
		if field.Key == bots.DirKey && value != "" {
			if value, err = filepath.Abs(value); err != nil {
				return 0, err
			}
		}
		spec, ok := byKey[field.Key]
		if !ok {
			return 0, fmt.Errorf("SettingSpecs does not name %s", field.Key)
		}
		if err := spec.Check(value); err != nil {
			return 0, err
		}
		if err := p.Set(field.Key, value); err != nil {
			return 0, err
		}
		changed++
	}
	return changed, p.Admit()
}

func runBotsList(out io.Writer, _ settingSpecs, args []string, configDir string, _ time.Time) error {
	set := newFlagSet(usageBotsList)
	asJSON := jsonFlag(set)
	if _, err := parseCommand(out, set, args, 0, 0); err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	all, err := st.BotProfiles()
	if err != nil {
		return err
	}
	records := make([]botRecord, 0, len(all))
	for _, p := range all {
		records = append(records, newBotRecord(p))
	}
	if *asJSON {
		return writeJSON(out, records)
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range records {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", r.ID, strings.TrimSpace(r.Emoji+" "+r.Name), r.Tool, r.Model)
	}
	return table.Flush()
}

func runBotsShow(out io.Writer, _ settingSpecs, args []string, configDir string, _ time.Time) error {
	set := newFlagSet(usageBotsShow)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	p, err := findBotProfile(st, operands[0])
	if err != nil {
		return err
	}
	return writeJSON(out, newBotRecord(p))
}

func runBotsCreate(out io.Writer, specs settingSpecs, args []string, configDir string, now time.Time) error {
	set := newFlagSet(usageBotsCreate)
	values := botFlags(set)
	asJSON := jsonFlag(set)
	if _, err := parseCommand(out, set, args, 0, 0); err != nil {
		return err
	}
	profile := bots.Profile{ID: uuid.NewString()}
	if _, err := applyBotFlags(set, values, specs, &profile); err != nil {
		return err
	}
	st, err := openSettingsStore(configDir)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.AddBotProfile(storedBotProfile(profile), now); err != nil {
		return err
	}
	return emitBotProfile(out, st, *asJSON, profile.ID, "created "+profile.Name+" ("+profile.ID+")")
}

func runBotsEdit(out io.Writer, specs settingSpecs, args []string, configDir string, now time.Time) error {
	set := newFlagSet(usageBotsEdit)
	values := botFlags(set)
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
	current, err := findBotProfile(st, operands[0])
	if err != nil {
		return err
	}
	profile := botProfile(current)
	changed, err := applyBotFlags(set, values, specs, &profile)
	if err != nil {
		return err
	}
	if changed == 0 {
		return errors.New("nothing to change; usage: agent-manager " + usageBotsEdit)
	}
	if err := st.UpdateBotProfile(storedBotProfile(profile), now); err != nil {
		return err
	}
	return emitBotProfile(out, st, *asJSON, profile.ID, "changed "+profile.Name+" ("+profile.ID+")")
}

func emitBotProfile(out io.Writer, st *store.Store, asJSON bool, id, human string) error {
	if !asJSON {
		return emit(out, false, nil, human)
	}
	p, _, err := st.BotProfile(id)
	if err != nil {
		return err
	}
	return writeJSON(out, newBotRecord(p))
}
