package app

import (
	"errors"
	"os"
	"testing"
)

func TestLockManagerRefusesASecondHolderOfTheSameSessions(t *testing.T) {
	dir := t.TempDir()
	release, err := LockManager(dir, "/tmp/a/agentmgr")
	if err != nil {
		t.Fatal(err)
	}
	_, err = LockManager(dir, "/tmp/a/agentmgr")
	var running *ManagerAlreadyRunning
	if !errors.As(err, &running) || running.PID != os.Getpid() {
		t.Fatalf("second LockManager = %v, want already running with pid %d", err, os.Getpid())
	}
	other, err := LockManager(dir, "/tmp/b/agentmgr")
	if err != nil {
		t.Fatalf("a different socket must not be blocked: %v", err)
	}
	other()
	release()
	again, err := LockManager(dir, "/tmp/a/agentmgr")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
}
