//go:build windows

package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// swapBinary moves target aside and staged into its place: Windows lets a
// running executable be renamed but not replaced or deleted. Each rebuild
// moves target to a name of its own, since one moved aside earlier stays
// locked while a manager started from it runs.
func swapBinary(staged, target string) error {
	removeAsideBinaries(target)
	old := fmt.Sprintf("%s.old-%d", target, time.Now().UnixNano())
	if err := os.Rename(target, old); err != nil {
		os.Remove(staged)
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		os.Rename(old, target)
		os.Remove(staged)
		return err
	}
	return nil
}

// removeAsideBinaries deletes what earlier swaps moved aside. One that a
// running manager still holds refuses the delete and waits for a later one.
func removeAsideBinaries(target string) {
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		return
	}
	prefix := filepath.Base(target) + ".old-"
	for _, entry := range entries {
		stamp, ok := strings.CutPrefix(entry.Name(), prefix)
		if !ok {
			continue
		}
		if _, err := strconv.ParseInt(stamp, 10, 64); err == nil {
			os.Remove(filepath.Join(filepath.Dir(target), entry.Name()))
		}
	}
}
