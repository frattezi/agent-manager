package runs

import (
	"os"
	"path/filepath"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestRulesForWorkMode(t *testing.T) {
	got := RulesFor(ModeWork, boolPtr(false))
	want := Rules{Merge: "PR Gate: human merges", TicketGate: "on", Agents: "claude@work"}
	if got != want {
		t.Fatalf("RulesFor(work) = %+v; want %+v", got, want)
	}
}

func TestRulesForPersonalModeFollowsTicketGate(t *testing.T) {
	cases := []struct {
		gate *bool
		want string
	}{{boolPtr(true), "on"}, {boolPtr(false), "off"}, {nil, Unknown}}
	for _, tc := range cases {
		got := RulesFor(ModePersonal, tc.gate)
		if got.TicketGate != tc.want {
			t.Errorf("ticket gate = %q; want %q", got.TicketGate, tc.want)
		}
		if got.Merge != "Auto-merge when the Run is done, mergeable and checks pass (or no CI)" || got.Agents != "claude@personal, opencode@personal" {
			t.Errorf("personal rules = %+v", got)
		}
	}
}

func TestRulesForUnknownModeSaysUnknown(t *testing.T) {
	for _, mode := range []string{"", "weekend"} {
		if got := RulesFor(mode, nil); got != (Rules{Merge: Unknown, TicketGate: Unknown, Agents: Unknown}) {
			t.Fatalf("RulesFor(%q) = %+v", mode, got)
		}
	}
}

func TestLoadReadsRepoConfigAndRecordFallsBack(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".fractory.json"), []byte(`{"mode":"personal","flow":"fast","ticketGate":true,"hasCi":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	body := func(id, repoPath, extra string) string {
		return `{"version":1,"id":"` + id + `","ticketRef":"jira:A-1","repoPath":"` + repoPath + `","status":"done","logPath":"/l","updatedAt":"2026-01-01T00:00:00Z"` + extra + `}`
	}
	writeRun(t, dir, "cfg.json", body("cfg", repo, ""))
	writeRun(t, dir, "rec.json", body("rec", "/no/such/repo", `,"mode":"work","flow":"slow","agents":["claude@work"]`))
	got, err := Load(dir)
	if err != nil || len(got) != 2 {
		t.Fatalf("Load = %v, %v", got, err)
	}
	byID := map[string]Record{got[0].ID: got[0], got[1].ID: got[1]}
	cfg, rec := byID["cfg"], byID["rec"]
	if cfg.EffectiveMode() != ModePersonal || cfg.EffectiveFlow() != "fast" || cfg.Rules().TicketGate != "on" {
		t.Fatalf("config-backed run = %q %q %+v", cfg.EffectiveMode(), cfg.EffectiveFlow(), cfg.Rules())
	}
	if rec.Config != nil || rec.EffectiveMode() != ModeWork || rec.EffectiveFlow() != "slow" || rec.Rules().Merge != "PR Gate: human merges" || len(rec.Agents) != 1 {
		t.Fatalf("record-backed run = %+v", rec)
	}
}
