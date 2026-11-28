package runs

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// logTailBytes is how much of a log's end is read to find its last line.
const logTailBytes = 8 << 10

// LogTail is the last non-empty line of the log at path, escapes stripped and
// control characters dropped. A log whose last line is longer than the window
// read yields that line's end. An unreadable log has no line.
func LogTail(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return ""
	}
	start := max(size-logTailBytes, 0)
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return ""
	}
	lines := strings.FieldsFunc(string(buf), func(r rune) bool { return r == '\n' || r == '\r' })
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.Map(func(r rune) rune {
			if r < ' ' || r == 0x7f {
				return -1
			}
			return r
		}, ansi.Strip(lines[i]))
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
