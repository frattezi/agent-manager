package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/YoanWai/agent-manager/internal/store"
)

var usageMark = "mark <" + strings.Join(store.SessionMarkNames, "|") + "> -- <command> [args...]"

type markCommands interface {
	SetMark(sessionID, name string, pid int) error
	ClearMark(sessionID, name string, pid int) error
}

func newMarks(configDir string) markCommands {
	return sessioncmd.NewSessions(configDir, sessioncmd.CLIVocabulary())
}

func markSection() section {
	return markSectionWith(newMarks)
}

func markSectionWith(open func(string) markCommands) section {
	return section{
		title: "Session Marks",
		commands: []command{
			{name: "mark", usage: usageMark, about: "run a command with a mark on your row for as long as it runs, such as babysit while `fractory babysit` drives Tickets to merged; exits with the command's status", run: bind(open, runMark)},
		},
	}
}

func runMark(out io.Writer, marks markCommands, args []string, sessionID string) error {
	separator := slices.Index(args, "--")
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(out, "usage: agent-manager "+usageMark)
		return ErrUsageShown
	}
	if separator != 1 || len(args) < 3 {
		return usageError(usageMark)
	}
	return runMarked(marks, sessionID, args[0], args[2:], os.Getpid(), func(ctx context.Context, argv []string) error {
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		// A Ctrl-C reaches the whole foreground group, the command included;
		// the wrapper only has to outlive it long enough to clear the mark.
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() {
			for sig := range signals {
				_ = cmd.Process.Signal(sig)
			}
		}()
		return cmd.Wait()
	})
}

// runMarked holds the mark for exactly the life of run; a wrapper killed
// before it clears leaves a mark the manager drops because pid is gone.
func runMarked(marks markCommands, sessionID, name string, argv []string, pid int, run func(context.Context, []string) error) error {
	if err := marks.SetMark(sessionID, name, pid); err != nil {
		return err
	}
	runErr := run(context.Background(), argv)
	clearErr := marks.ClearMark(sessionID, name, pid)
	if runErr != nil {
		return fmt.Errorf("%s: %w", argv[0], runErr)
	}
	return clearErr
}

// commandExitCode is the status a wrapped command exited with, so the
// wrapper's caller reads the command's result rather than the wrapper's.
func commandExitCode(err error) (int, bool) {
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() > 0 {
		return exit.ExitCode(), true
	}
	return 0, false
}
