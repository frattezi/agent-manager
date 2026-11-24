package runs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
)

const (
	commentsSuffix  = ".comments.json"
	commentsVersion = 1
)

// Comment is one review comment left on a Run's diff.
type Comment struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

type commentsFile struct {
	Version  int       `json:"version"`
	Comments []Comment `json:"comments"`
}

// CommentsPath is where a Run's review comments live; Fractory's next tick
// turns them into a review-comment Trigger.
func CommentsPath(dir, runID string) string {
	return filepath.Join(dir, runID+commentsSuffix)
}

// AddComment appends a comment to the Run's file, creating it when absent.
// The write is atomic, so Fractory never reads a half-written file. A file
// that exists but cannot be read back is an error, never overwritten.
func AddComment(dir, runID string, comment Comment, now time.Time) error {
	if runID == "" || strings.ContainsRune(runID, filepath.Separator) {
		return errors.New("invalid run id")
	}
	path := CommentsPath(dir, runID)
	file := commentsFile{Version: commentsVersion}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &file); err != nil {
			return err
		}
		if file.Version != commentsVersion {
			return errors.New("unsupported comments file version")
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	comment.CreatedAt = now.UTC().Format(time.RFC3339)
	file.Comments = append(file.Comments, comment)
	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, append(out, '\n'), 0o644)
}
