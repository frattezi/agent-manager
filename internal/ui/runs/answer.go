package runs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
)

const (
	answerSuffix  = ".answer.json"
	answerVersion = 1
)

// ErrEmptyAnswer refuses an answer with no text: Fractory would ignore it,
// and the Run would wait on a file that never becomes a Run.
var ErrEmptyAnswer = errors.New("an answer needs some text")

type answerRequest struct {
	Version    int    `json:"version"`
	AnsweredAt string `json:"answeredAt"`
	Answer     string `json:"answer"`
}

// AnswerPath is where the owner's answer to a needs-human Run lives;
// Fractory's next tick queues a Run on the Ticket's branch with the question
// and this answer in its prompt.
func AnswerPath(dir, runID string) string {
	return filepath.Join(dir, runID+answerSuffix)
}

// CanAnswer reports whether the Run stopped with a question for the owner.
func (r Record) CanAnswer() bool { return r.Status == StatusNeedsHuman }

// WriteAnswer saves the owner's answer to the Run, replacing one already on
// disk, so the owner can correct it until the next tick. The write is atomic,
// so Fractory never reads a half-written answer. The text is kept as typed,
// newlines included.
func WriteAnswer(dir, runID, answer string, now time.Time) error {
	if err := validRunID(runID); err != nil {
		return err
	}
	if strings.TrimSpace(answer) == "" {
		return ErrEmptyAnswer
	}
	out, err := json.Marshal(answerRequest{Version: answerVersion, AnsweredAt: now.UTC().Format(time.RFC3339), Answer: answer})
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(AnswerPath(dir, runID), append(out, '\n'), 0o644)
}

// Answers maps the ids of the Runs that have an answer waiting for the next
// tick to its text. A file Fractory would ignore is left out, and an
// unreadable dir lists none; the next poll tries again.
func Answers(dir string) map[string]string {
	ids := sidecarIDs(dir, answerSuffix)
	if len(ids) == 0 {
		return nil
	}
	answers := map[string]string{}
	for id := range ids {
		data, err := os.ReadFile(AnswerPath(dir, id))
		if err != nil {
			continue
		}
		var request answerRequest
		if json.Unmarshal(data, &request) != nil || request.Version != answerVersion || request.Answer == "" {
			continue
		}
		answers[id] = request.Answer
	}
	return answers
}
