//go:build !windows

package rebuild

import "os"

// swapBinary renames staged over target. A manager already running keeps
// its unlinked image; its next start runs the new build.
func swapBinary(staged, target string) error {
	if err := os.Rename(staged, target); err != nil {
		os.Remove(staged)
		return err
	}
	return nil
}
