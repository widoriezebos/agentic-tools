package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A hook stages its files in the process's scratch root (Part B U1b-2): a
// hook killed at the Stop budget never runs its deferred removal, and a
// registered root is what the sweeper can prove about afterwards.
func TestHookStagesInTheProcessScratch(t *testing.T) {
	t.Parallel()
	scratch, err := diskstore.ProcessScratch()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := mkdirStaging((Invocation{}).withDefaults().TempDir, "metasystem-stop-deadline.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dir) })
	if filepath.Dir(dir) != scratch {
		t.Fatalf("a hook with no named staging root staged %s, want a directory in the process scratch root %s", dir, scratch)
	}
}

// A Stop whose staging root cannot take a directory answers in the staging
// form; it never falls back to a shared /tmp where nothing owns the files.
func TestStopDeadlineHasNoTmpFallback(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	absent := filepath.Join(t.TempDir(), "absent")
	before, _ := filepath.Glob(filepath.Join("/tmp", "metasystem-stop-deadline.*"))
	run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{tempDir: absent})
	if run.stdout != mustForm(t, "allowed", "staging-failed")+"\n" {
		t.Fatalf("an unusable staging root = stdout %q stderr %q, want the staging form", run.stdout, run.stderr)
	}
	after, _ := filepath.Glob(filepath.Join("/tmp", "metasystem-stop-deadline.*"))
	if len(after) > len(before) {
		t.Fatalf("the Stop staged in /tmp: %s", strings.Join(after, ", "))
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("the absent staging root was created: %v", err)
	}
}
