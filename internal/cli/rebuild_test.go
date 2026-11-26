package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/rebuild"
)

func setRebuildCheckout(t *testing.T, configDir, value string) {
	t.Helper()
	st, err := openSettingsStore(configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetSetting(rebuild.CheckoutSetting, value); err != nil {
		t.Fatal(err)
	}
}

func stubRebuild(t *testing.T, target string) *bool {
	t.Helper()
	built := false
	oldSteps, oldExecutable := rebuildSteps, rebuildExecutable
	t.Cleanup(func() { rebuildSteps, rebuildExecutable = oldSteps, oldExecutable })
	rebuildExecutable = func() (string, error) { return target, nil }
	rebuildSteps = func() rebuild.Steps {
		return rebuild.Steps{
			Inspect: func(dir string) (rebuild.Checkout, error) {
				return rebuild.Checkout{Repo: dir, Branch: "main", Commit: "abc1234", Stamp: "1.0.0-1-gabc1234"}, nil
			},
			Build: func(_, _, output string, _, _ io.Writer) error {
				built = true
				return os.WriteFile(output, []byte("new"), 0o755)
			},
			Version: func(string) (string, error) { return "1.0.0-1-gabc1234", nil },
			Install: func(staged, target string) error { return os.Rename(staged, target) },
		}
	}
	return &built
}

func TestRebuildRefusesWithNoCheckoutAndNamesTheSetCommand(t *testing.T) {
	configDir := t.TempDir()
	built := stubRebuild(t, filepath.Join(t.TempDir(), "manager"))
	var out bytes.Buffer
	err := runRebuild(&out, &out, "dev", "my-launcher", nil, configDir)
	if err == nil {
		t.Fatal("rebuild ran with no checkout configured")
	}
	if want := "my-launcher settings set " + rebuild.CheckoutSetting; !strings.Contains(err.Error(), want) {
		t.Fatalf("refusal %q does not name %q", err, want)
	}
	if *built {
		t.Fatal("rebuild built with no checkout configured")
	}
}

func TestRebuildRefusesARelativeCheckout(t *testing.T) {
	configDir := t.TempDir()
	setRebuildCheckout(t, configDir, "src/agent-manager")
	built := stubRebuild(t, filepath.Join(t.TempDir(), "manager"))
	if err := runRebuild(io.Discard, io.Discard, "dev", "am", nil, configDir); err == nil || *built {
		t.Fatalf("a relative checkout was built: %v", err)
	}
}

func TestRebuildInstallsOverTheInvokedBinaryAndSaysToRestart(t *testing.T) {
	configDir := t.TempDir()
	setRebuildCheckout(t, configDir, t.TempDir())
	target := filepath.Join(t.TempDir(), "any-name")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubRebuild(t, target)
	var out bytes.Buffer
	if err := runRebuild(&out, &out, "0.9.0-0-gfff0000", "any-name", nil, configDir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "new" {
		t.Fatalf("target holds %q", data)
	}
	if archived, _ := os.ReadFile(filepath.Join(rebuild.VersionsDir(configDir), "0.9.0-0-gfff0000", "any-name")); string(archived) != "old" {
		t.Fatalf("archive holds %q", archived)
	}
	for _, want := range []string{"1.0.0-1-gabc1234", "restart"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output does not say %q:\n%s", want, out.String())
		}
	}
}

func TestRebuildTakesNoOperands(t *testing.T) {
	if err := runRebuild(io.Discard, io.Discard, "dev", "am", []string{"extra"}, t.TempDir()); err == nil {
		t.Fatal("rebuild took an operand")
	}
}
