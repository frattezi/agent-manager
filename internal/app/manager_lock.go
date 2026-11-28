package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ManagerAlreadyRunning is returned by LockManager while another manager TUI
// holds the same sessions. PID is 0 when the holder's pid cannot be read.
type ManagerAlreadyRunning struct{ PID int }

func (e *ManagerAlreadyRunning) Error() string {
	if e.PID > 0 {
		return fmt.Sprintf("Fractory Manager is already running (pid %d); quit it first, or stop it with `kill %d`", e.PID, e.PID)
	}
	return "Fractory Manager is already running; quit it first"
}

// LockManager takes the lock that lets one manager TUI own a set of sessions.
// Two managers resize every session's window to their own screen and undo
// each other. The lock is keyed on the multiplexer socket, so a manager
// started with its own TMUX_TMPDIR, or its own config directory, is a
// different one. The operating system releases it when the process dies, so
// a crash leaves nothing behind; the pid file beside it only names the
// holder for the message.
func LockManager(profileDir, socketPath string) (release func(), err error) {
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(socketPath))
	path := filepath.Join(profileDir, "manager-"+hex.EncodeToString(sum[:4])+".lock")
	unlock, acquired, err := tryServeLock(path)
	if err != nil {
		return nil, err
	}
	if !acquired {
		data, _ := os.ReadFile(path + ".pid")
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return nil, &ManagerAlreadyRunning{PID: pid}
	}
	_ = os.WriteFile(path+".pid", []byte(strconv.Itoa(os.Getpid())), 0o600)
	return unlock, nil
}
