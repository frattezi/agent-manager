package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoanWai/agent-manager/internal/rebuild"
)

const usageRebuild = "rebuild"

// The seams let tests stub the build and the install without a toolchain
// or a binary to replace.
var (
	rebuildExecutable = os.Executable
	rebuildSteps      = rebuild.Default
)

func rebuildSection(version string) section {
	return section{
		title: "Rebuild",
		commands: []command{
			{
				name:  "rebuild",
				usage: usageRebuild,
				about: "build the checkout named by the " + rebuild.CheckoutSetting + " setting and install it over this binary, archiving the outgoing build; runs no git command that changes the checkout",
				run: func(args []string, _ func() string, configDir string) error {
					return runRebuild(os.Stdout, os.Stderr, version, programName(), args, configDir)
				},
			},
		},
	}
}

// programName is the name this binary was invoked as, so the command a
// refusal prints is one the user's shell runs.
func programName() string {
	return strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
}

func runRebuild(out, errs io.Writer, version, program string, args []string, configDir string) error {
	set := newFlagSet(usageRebuild)
	if _, err := parseCommand(out, set, args, 0, 0); err != nil {
		return err
	}
	checkout, err := rebuildCheckout(configDir)
	if err != nil {
		return err
	}
	setCommand := fmt.Sprintf("%s settings set %s /path/to/checkout", program, rebuild.CheckoutSetting)
	if checkout == "" {
		return fmt.Errorf("rebuild: no checkout is configured; set one with\n  %s", setCommand)
	}
	if !filepath.IsAbs(checkout) {
		return fmt.Errorf("rebuild: the checkout %q is not an absolute path; set one with\n  %s", checkout, setCommand)
	}
	target, err := rebuildExecutable()
	if err != nil {
		return err
	}
	result, err := rebuild.Run(rebuildSteps(), rebuild.Request{
		Checkout: checkout, Target: target, Outgoing: version,
		VersionsDir: rebuild.VersionsDir(configDir), Out: out, Errs: errs,
	})
	if err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}
	_, err = fmt.Fprintf(out, "installed %s at %s\nrestart the manager to run it\n", result.Version, result.Target)
	return err
}

func rebuildCheckout(configDir string) (string, error) {
	st, err := openSettingsStore(configDir)
	if err != nil {
		return "", err
	}
	defer st.Close()
	value, err := st.Setting(rebuild.CheckoutSetting)
	return strings.TrimSpace(value), err
}
