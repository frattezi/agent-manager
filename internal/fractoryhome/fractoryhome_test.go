package fractoryhome

import (
	"path/filepath"
	"testing"
)

func TestDirPrefersFractoryHome(t *testing.T) {
	t.Setenv("FRACTORY_HOME", "/srv/fractory")
	if got := Sub("runs"); got != filepath.Join("/srv/fractory", "runs") {
		t.Fatalf("Sub = %q", got)
	}
}

func TestDirFallsBackToTheUserHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FRACTORY_HOME", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got := Dir(); got != filepath.Join(home, ".fractory") {
		t.Fatalf("Dir = %q", got)
	}
}

func TestSubIsEmptyWithoutAHome(t *testing.T) {
	t.Setenv("FRACTORY_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("home", "")
	if Dir() != "" {
		t.Skip("this platform resolves a home without HOME")
	}
	if got := Sub("runs"); got != "" {
		t.Fatalf("Sub = %q, want empty", got)
	}
}
