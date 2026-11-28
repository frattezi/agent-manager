package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogTail(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ name, content, want string }{
		{"last line", "one\ntwo\n", "two"},
		{"skips blanks", "one\ntwo\n\n  \n", "two"},
		{"strips escapes", "one\n\x1b[32mgreen\x1b[0m\x07\n", "green"},
		{"carriage returns", "one\rtwo\r", "two"},
		{"empty", "", ""},
		{"long log", strings.Repeat("filler line\n", 5000) + "the end", "the end"},
	} {
		path := filepath.Join(dir, "log")
		if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := LogTail(path); got != tc.want {
			t.Errorf("%s: LogTail = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := LogTail(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("missing log: %q", got)
	}
}
