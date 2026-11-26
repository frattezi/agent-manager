// Package fractoryhome resolves the directory Fractory keeps its state in,
// so every reader and writer of that state agrees on where it is.
package fractoryhome

import (
	"os"
	"path/filepath"
)

// Dir is $FRACTORY_HOME, else ~/.fractory; empty when neither resolves.
func Dir() string {
	if home := os.Getenv("FRACTORY_HOME"); home != "" {
		return home
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(user, ".fractory")
}

// Sub is name under Dir; empty when Dir is.
func Sub(name string) string {
	home := Dir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, name)
}
