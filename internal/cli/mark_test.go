package cli

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"slices"
	"testing"

	"github.com/YoanWai/agent-manager/internal/store"
)

type fakeMarks struct {
	calls  []string
	setErr error
	marked bool
}

func (f *fakeMarks) SetMark(sessionID, name string, pid int) error {
	f.calls = append(f.calls, "set "+sessionID+" "+name)
	if f.setErr == nil {
		f.marked = true
	}
	return f.setErr
}

func (f *fakeMarks) ClearMark(sessionID, name string, pid int) error {
	f.calls = append(f.calls, "clear "+sessionID+" "+name)
	f.marked = false
	return nil
}

func TestMarkHoldsTheMarkExactlyWhileTheCommandRuns(t *testing.T) {
	marks := &fakeMarks{}
	var markedDuring bool
	err := runMarked(marks, "cafe0001", store.MarkBabysit, []string{"fractory", "babysit"}, 42, func(_ context.Context, argv []string) error {
		markedDuring = marks.marked
		return nil
	})
	if err != nil || !markedDuring || marks.marked {
		t.Fatalf("want the mark set during the run and cleared after: err=%v during=%v after=%v", err, markedDuring, marks.marked)
	}
}

func TestMarkClearsAndPassesOnAFailedCommandsStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	marks := &fakeMarks{}
	err := runMarked(marks, "cafe0001", store.MarkBabysit, []string{"sh", "-c", "exit 7"}, 42, func(ctx context.Context, argv []string) error {
		return exec.CommandContext(ctx, argv[0], argv[1:]...).Run()
	})
	if code := ExitCode(err); code != 7 || marks.marked {
		t.Fatalf("want exit 7 and the mark cleared, got %d (%v), marked=%v", code, err, marks.marked)
	}
}

func TestMarkNeverRunsTheCommandWhenTheMarkIsRefused(t *testing.T) {
	marks := &fakeMarks{setErr: errors.New("not a mark")}
	ran := false
	_ = runMarked(marks, "cafe0001", "deploying", []string{"true"}, 42, func(context.Context, []string) error { ran = true; return nil })
	if ran || !slices.Equal(marks.calls, []string{"set cafe0001 deploying"}) {
		t.Fatalf("a refused mark must not run the command: ran=%v calls=%v", ran, marks.calls)
	}
}

func TestMarkWantsANameThenTheCommandAfterASeparator(t *testing.T) {
	for _, args := range [][]string{{}, {"babysit"}, {"babysit", "fractory"}, {"babysit", "--"}, {"--", "fractory"}} {
		if err := runMark(&bytes.Buffer{}, &fakeMarks{}, args, "cafe0001"); err == nil {
			t.Fatalf("%q: want a usage error", args)
		}
	}
}
