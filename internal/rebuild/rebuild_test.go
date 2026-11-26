package rebuild

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stubSteps(buildErr error) (Steps, *[]string) {
	var calls []string
	return Steps{
		Inspect: func(dir string) (Checkout, error) {
			calls = append(calls, "inspect")
			return Checkout{Repo: dir, Branch: "main", Commit: "abc1234", Dirty: true, Stamp: "0.40.0-2-gabc1234-dirty"}, nil
		},
		Build: func(repo, stamp, output string, stdout, _ io.Writer) error {
			calls = append(calls, "build")
			io.WriteString(stdout, "compiling\n")
			if buildErr != nil {
				return buildErr
			}
			return os.WriteFile(output, []byte("new build "+stamp), 0o755)
		},
		Version: func(string) (string, error) {
			calls = append(calls, "version")
			return "0.40.0-2-gabc1234-dirty", nil
		},
		Install: func(staged, target string) error {
			calls = append(calls, "install")
			return install(staged, target)
		},
	}, &calls
}

func request(t *testing.T, out *bytes.Buffer) Request {
	t.Helper()
	target := filepath.Join(t.TempDir(), "manager")
	writeBinary(t, target, "old build", time.Now().Add(-time.Hour))
	return Request{
		Checkout: t.TempDir(), Target: target, Outgoing: "0.39.0-0-gfff0000",
		VersionsDir: filepath.Join(t.TempDir(), "versions"), Out: out, Errs: out,
	}
}

func TestRunArchivesTheOutgoingBuildAndInstallsTheNewOne(t *testing.T) {
	var out bytes.Buffer
	steps, _ := stubSteps(nil)
	req := request(t, &out)

	result, err := Run(steps, req)
	if err != nil {
		t.Fatal(err)
	}
	installed, _ := os.ReadFile(req.Target)
	if string(installed) != "new build 0.40.0-2-gabc1234-dirty" {
		t.Fatalf("target holds %q", installed)
	}
	archived, _ := os.ReadFile(filepath.Join(req.VersionsDir, "0.39.0-0-gfff0000", "manager"))
	if string(archived) != "old build" {
		t.Fatalf("archive holds %q", archived)
	}
	if result.Version != "0.40.0-2-gabc1234-dirty" {
		t.Fatalf("result %+v", result)
	}
	for _, want := range []string{req.Checkout, "main", "abc1234", "dirty", "compiling"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output does not name %q:\n%s", want, out.String())
		}
	}
	if strings.Index(out.String(), "abc1234") > strings.Index(out.String(), "compiling") {
		t.Fatalf("the checkout is named after the build ran:\n%s", out.String())
	}
}

func TestRunPrunesTheArchiveToTheKeptBuilds(t *testing.T) {
	var out bytes.Buffer
	steps, _ := stubSteps(nil)
	req := request(t, &out)
	old := filepath.Join(t.TempDir(), "manager")
	for i, stamp := range []string{"a", "b", "c"} {
		writeBinary(t, old, stamp, time.Now().Add(-time.Duration(10-i)*time.Hour))
		if _, err := Archive(req.VersionsDir, old, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(steps, req); err != nil {
		t.Fatal(err)
	}
	entries, _ := Entries(req.VersionsDir)
	if len(entries) != Kept || entries[0].Version != req.Outgoing {
		t.Fatalf("archive holds %+v", entries)
	}
}

func TestAFailedBuildLeavesTheBinaryAndTheArchiveInPlace(t *testing.T) {
	for name, steps := range map[string]Steps{
		"build": func() Steps { s, _ := stubSteps(errors.New("exit status 1")); return s }(),
		"version": func() Steps {
			s, _ := stubSteps(nil)
			s.Version = func(string) (string, error) { return "", errors.New("exec format error") }
			return s
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			req := request(t, &out)
			installed := false
			steps.Install = func(string, string) error { installed = true; return nil }
			_, err := Run(steps, req)
			if err == nil {
				t.Fatal("a failed build reported success")
			}
			if installed {
				t.Fatal("a failed build was installed")
			}
			data, _ := os.ReadFile(req.Target)
			if string(data) != "old build" {
				t.Fatalf("target holds %q", data)
			}
			if _, err := os.Stat(req.VersionsDir); !os.IsNotExist(err) {
				t.Fatalf("a failed build touched the archive: %v", err)
			}
		})
	}
}

func TestInspectReadsTheCheckoutWithoutChangingIt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "trunk")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "first")
	run("tag", "v1.0.0")
	head := run("rev-parse", "--short", "HEAD")

	clean, err := inspect(repo)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Branch != "trunk" || clean.Commit != head || clean.Dirty || !strings.Contains(clean.Stamp, "g"+head) {
		t.Fatalf("clean checkout reads %+v", clean)
	}

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := inspect(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty.Dirty || !strings.HasSuffix(dirty.Stamp, "-dirty") || !strings.Contains(dirty.Stamp, head) {
		t.Fatalf("dirty checkout reads %+v", dirty)
	}
	if run("rev-parse", "--abbrev-ref", "HEAD") != "trunk" || run("status", "--porcelain") != "M a.txt" {
		t.Fatal("inspecting changed the checkout")
	}
}

func TestInspectRefusesADirectoryThatIsNoCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := inspect(t.TempDir()); err == nil {
		t.Fatal("a plain directory read as a checkout")
	}
}
