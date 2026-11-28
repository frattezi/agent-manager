package runs

import "testing"

func TestDerivations(t *testing.T) {
	tests := []struct {
		name                   string
		rec                    Record
		project, title, number string
	}{
		{"ticket number and prefix", Record{RepoPath: "/w/esquerda", Branch: "fractory/270-wire-marina-s-logo-into-the-favicon-foot", TicketRef: "github:o/esquerda#270"}, "esquerda", "Wire marina s logo into the favicon foot", "#270"},
		{"no number no prefix", Record{Branch: "fix-login", TicketRef: "github:o/r#5"}, "", "Fix login", "#5"},
		{"pull request wins", Record{Branch: "fractory/270-x", TicketRef: "github:o/r#270", PRURL: "https://github.com/o/r/pull/293"}, "", "X", "PR 293"},
		{"pr url not digits", Record{Branch: "a/b", TicketRef: "github:o/r#7", PRURL: "https://x/pulls"}, "", "B", "#7"},
		{"jira ref", Record{Branch: "fractory/abc-1-thing", TicketRef: "jira:ABC-1"}, "", "Abc 1 thing", ""},
		{"empty branch", Record{TicketRef: "jira:ABC-1"}, "", "", ""},
		{"recorded title wins", Record{Title: "Real title", Branch: "fractory/1-other", TicketRef: "github:o/r#1"}, "", "Real title", "#1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rec.Project(); got != tt.project {
				t.Errorf("Project = %q, want %q", got, tt.project)
			}
			if got := tt.rec.RunTitle(); got != tt.title {
				t.Errorf("RunTitle = %q, want %q", got, tt.title)
			}
			if got := tt.rec.Number().String(); got != tt.number {
				t.Errorf("Number = %q, want %q", got, tt.number)
			}
		})
	}
}
