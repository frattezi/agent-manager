//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The manager needs a terminal, so each one runs in a pane of a throwaway
// tmux server of its own, apart from the sockets the managers drive.
func TestSecondManagerRefusesWhileTheFirstRuns(t *testing.T) {
	if prepareMainProcess() {
		main()
		return
	}
	root, err := os.MkdirTemp("/tmp", "amlock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	for _, dir := range []string{"home", "config", "tmux", "host"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	host := func(args ...string) *exec.Cmd {
		return exec.Command("tmux", append([]string{"-L", "amlockhost", "-f", os.DevNull}, args...)...)
	}
	cmd := func(name string) string {
		c := mainTestCommand(t)
		env := []string{
			"HOME=" + filepath.Join(root, "home"),
			"XDG_CONFIG_HOME=" + filepath.Join(root, "config"),
			"AGENT_MANAGER_CONFIG_DIR=" + filepath.Join(root, "config"),
			"TMUX_TMPDIR=" + filepath.Join(root, "tmux"),
			"SHELL=/bin/sh", "TERM=xterm-256color",
		}
		return "env -u TMUX " + strings.Join(env, " ") + " AGENT_MANAGER_MAIN_TEST=1 " +
			strings.Join(c.Args, " ") + " ; echo exited-" + name + " ; sleep 30"
	}
	t.Cleanup(func() { host("kill-server").Run() })
	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "host"))
	if out, err := host("new-session", "-d", "-s", "first", "-x", "100", "-y", "30", cmd("first")).CombinedOutput(); err != nil {
		t.Fatalf("start first manager: %v\n%s", err, out)
	}
	pane := func(session string) string {
		out, _ := host("capture-pane", "-p", "-t", session).Output()
		return string(out)
	}
	waitFor := func(session, want string) string {
		deadline := time.Now().Add(8 * time.Second)
		for {
			if got := pane(session); strings.Contains(got, want) || time.Now().After(deadline) {
				return got
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	// Let the first manager draw and hold the lock before the second starts.
	time.Sleep(2 * time.Second)
	if got := pane("first"); strings.Contains(got, "exited-first") {
		t.Fatalf("first manager exited:\n%s", got)
	}
	if out, err := host("new-session", "-d", "-s", "second", "-x", "100", "-y", "30", cmd("second")).CombinedOutput(); err != nil {
		t.Fatalf("start second manager: %v\n%s", err, out)
	}
	got := waitFor("second", "exited-second")
	if !strings.Contains(got, "already running") || !strings.Contains(got, "pid ") {
		t.Fatalf("second manager did not refuse with the holder's pid:\n%s", got)
	}
}
