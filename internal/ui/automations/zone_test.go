package automations

import "testing"

func TestLocalZoneTakesTZ(t *testing.T) {
	t.Setenv("TZ", "Europe/Lisbon")
	if got := LocalZone(); got != "Europe/Lisbon" {
		t.Fatalf("LocalZone = %q", got)
	}
	t.Setenv("TZ", ":America/Sao_Paulo")
	if got := LocalZone(); got != "America/Sao_Paulo" {
		t.Fatalf("LocalZone = %q", got)
	}
}
