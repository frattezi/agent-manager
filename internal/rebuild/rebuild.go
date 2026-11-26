// Package rebuild builds agent-manager from a checkout on this machine and
// installs it over the binary that was invoked, archiving the outgoing build
// first so a rollback has something to restore. It runs no git command that
// changes the checkout: what is checked out, dirty or not, is what gets built.
package rebuild

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckoutSetting is the stored setting naming the checkout to build from.
const CheckoutSetting = "rebuild_checkout"

// Checkout is what a build is about to take in.
type Checkout struct {
	Repo   string
	Branch string
	Commit string
	Dirty  bool
	// Stamp is the version the build is stamped with. It carries the
	// commit, so two builds of one release stay apart.
	Stamp string
}

// Steps are the boundaries a rebuild crosses: git and the Go toolchain on
// the way in, the binary on disk on the way out. Tests replace them.
type Steps struct {
	Inspect func(dir string) (Checkout, error)
	Build   func(repo, stamp, output string, stdout, stderr io.Writer) error
	Version func(binary string) (string, error)
	Install func(staged, target string) error
}

// Default crosses the real boundaries.
func Default() Steps {
	return Steps{Inspect: inspect, Build: build, Version: builtVersion, Install: install}
}

// Request names one rebuild.
type Request struct {
	// Checkout is the configured directory to build from.
	Checkout string
	// Target is the executable to replace.
	Target string
	// Outgoing is the version stamp of the binary at Target.
	Outgoing    string
	VersionsDir string
	Out, Errs   io.Writer
}

// Result is what a rebuild installed and what it archived.
type Result struct {
	Version  string
	Target   string
	Archived Entry
}

// Run builds the checkout, archives the binary at the target and installs
// the new build in its place. A step that fails leaves every later one
// undone, so a failed build touches neither the target nor the archive.
func Run(steps Steps, req Request) (Result, error) {
	target, err := filepath.EvalSymlinks(req.Target)
	if err != nil {
		return Result{}, fmt.Errorf("finding the installed binary: %w", err)
	}
	checkout, err := steps.Inspect(req.Checkout)
	if err != nil {
		return Result{}, fmt.Errorf("reading the checkout %s: %w", req.Checkout, err)
	}
	tree := "clean"
	if checkout.Dirty {
		tree = "dirty"
	}
	fmt.Fprintf(req.Out, "building %s\n  repo    %s\n  branch  %s\n  commit  %s (%s tree)\n", checkout.Stamp, checkout.Repo, checkout.Branch, checkout.Commit, tree)

	staging, err := os.MkdirTemp("", "agent-manager-rebuild-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, filepath.Base(target))
	if err := steps.Build(checkout.Repo, checkout.Stamp, staged, req.Out, req.Errs); err != nil {
		return Result{}, fmt.Errorf("the build failed: %w", err)
	}
	version, err := steps.Version(staged)
	if err != nil {
		return Result{}, fmt.Errorf("the new build does not run: %w", err)
	}

	entry, err := Archive(req.VersionsDir, target, req.Outgoing)
	if err != nil {
		return Result{}, fmt.Errorf("archiving the installed build: %w", err)
	}
	fmt.Fprintf(req.Out, "archived %s in %s\n", entry.Version, entry.Dir)
	if err := Prune(req.VersionsDir, Kept); err != nil {
		return Result{}, fmt.Errorf("pruning old builds: %w", err)
	}
	if err := steps.Install(staged, target); err != nil {
		return Result{}, fmt.Errorf("installing over %s: %w", target, err)
	}
	return Result{Version: version, Target: target, Archived: entry}, nil
}

func inspect(dir string) (Checkout, error) {
	repo, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Checkout{}, err
	}
	branch, err := git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return Checkout{}, err
	}
	if branch == "HEAD" {
		branch = "detached HEAD"
	}
	commit, err := git(repo, "rev-parse", "--short", "HEAD")
	if err != nil {
		return Checkout{}, err
	}
	status, err := git(repo, "status", "--porcelain")
	if err != nil {
		return Checkout{}, err
	}
	// The same stamp make build writes, so a rebuilt binary reads the way
	// one built by hand does.
	stamp, err := git(repo, "describe", "--tags", "--always", "--dirty", "--long")
	if err != nil {
		return Checkout{}, err
	}
	return Checkout{Repo: filepath.FromSlash(repo), Branch: branch, Commit: commit, Dirty: status != "", Stamp: stamp}, nil
}

// git reads the checkout. --no-optional-locks keeps status and describe
// from refreshing the index, so the read writes nothing to the repository.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-optional-locks", "-C", dir}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("git %s: %s", args[0], message)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func build(repo, stamp, output string, stdout, stderr io.Writer) error {
	args := []string{"build", "-o", output, "-ldflags=-X=main.version=" + stamp, "."}
	fmt.Fprintf(stdout, "go %s\n", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Dir = repo
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// builtVersion runs the new binary, which proves it starts on this machine
// before it replaces one that does.
func builtVersion(binary string) (string, error) {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("it printed no version")
	}
	return fields[len(fields)-1], nil
}

// install copies the build beside the target, so the swap is a rename on
// one filesystem, and keeps the target's permissions.
func install(staged, target string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	sibling, err := copyFile(staged, filepath.Dir(target), "."+filepath.Base(target)+".rebuild-*", info.Mode().Perm())
	if err != nil {
		return err
	}
	return swapBinary(sibling, target)
}

// copyFile copies src to a new file in dir named by pattern and returns its
// path; a failed copy leaves nothing behind.
func copyFile(src, dir, pattern string, perm os.FileMode) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(out.Name(), perm)
	}
	if err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}
