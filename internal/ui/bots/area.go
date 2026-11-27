package bots

// CreateHint is how the empty Bots area tells the owner to make a Profile.
const CreateHint = "agent-manager bots create --name <name> --tool <cli>"

// Facts are the Profile's configuration as the detail column lists it,
// label first, leaving out the fields it does not set.
func (p Profile) Facts() [][2]string {
	model := p.Model
	if model == "" {
		model = "the CLI's own default"
	}
	facts := [][2]string{{"Profile", p.Name}, {"Emoji", p.Emoji}, {"Tool", p.Tool}, {"Model", model},
		{"Directory", p.Dir}, {"Flags", p.Flags}, {"Specialization", p.Specialization}, {"Memory", p.MemoryProject}}
	kept := facts[:0]
	for _, fact := range facts {
		if fact[1] != "" {
			kept = append(kept, fact)
		}
	}
	return kept
}
