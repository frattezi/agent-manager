package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/YoanWai/agent-manager/internal/rebuild"
)

const usageRollback = "rollback [--list] [<version>]"

func rollbackCommand(version string) command {
	return command{
		name:  "rollback",
		usage: usageRollback,
		about: "install an archived build over this binary: the one before the last rebuild, or the version named; --list prints the archive; builds nothing and runs no git command",
		run: func(args []string, _ func() string, configDir string) error {
			return runRollback(os.Stdout, version, programName(), args, configDir)
		},
	}
}

func runRollback(out io.Writer, version, program string, args []string, configDir string) error {
	set := newFlagSet(usageRollback)
	list := set.Bool("list", false, "print the archived builds, newest first, marking the one installed now")
	operands, err := parseCommand(out, set, args, 0, 1)
	if err != nil {
		return err
	}
	if *list && len(operands) > 0 {
		return usageError(usageRollback)
	}
	versionsDir := rebuild.VersionsDir(configDir)
	entries, err := rebuild.Entries(versionsDir)
	if err != nil {
		return fmt.Errorf("rollback: reading %s: %w", versionsDir, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("rollback: no archived builds in %s; %s rebuild archives the installed build before it replaces it", versionsDir, program)
	}
	if *list {
		return writeArchive(out, entries, version)
	}
	name := ""
	if len(operands) == 1 {
		name = operands[0]
	}
	entry, err := rebuild.Pick(entries, version, name)
	if err != nil {
		var available strings.Builder
		writeArchive(&available, entries, version)
		return fmt.Errorf("rollback: %w (installed: %s); archived builds:\n%s", err, version, strings.TrimRight(available.String(), "\n"))
	}
	target, err := rebuildExecutable()
	if err != nil {
		return err
	}
	installed, err := rebuild.Restore(entry, target)
	if err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	_, err = fmt.Fprintf(out, "installed %s at %s\nrestart the manager to run it\n", entry.Version, installed)
	return err
}

func writeArchive(out io.Writer, entries []rebuild.Entry, current string) error {
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, entry := range entries {
		mark := " "
		if entry.Version == current {
			mark = "*"
		}
		fmt.Fprintf(table, "%s %s\t%s\n", mark, entry.Version, entry.InstalledAt.Local().Format(time.DateTime))
	}
	return table.Flush()
}
