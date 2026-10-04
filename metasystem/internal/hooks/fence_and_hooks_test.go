package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// On separated roots, an installation metasystem/ whose state root is the
// checkout above it, the stop-refusal record is run state: it lands under the
// installation's artifacts/, where the state-root owner's answer points, and
// never under the checkout.
func TestStopRefusalRecordSeparatedRoots(t *testing.T) {
	t.Parallel()
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := hookInstallationAt(t, filepath.Join(checkout, "metasystem"))
	resolver := stateroot.NewResolver(func(string) (string, error) { return checkout, nil }, nil)
	if state, err := resolver.RootForInstallation(stateroottest.Installation(t, installation.root)); err != nil || state.Path() != checkout {
		t.Fatalf("the fixture's state root = %q %v, want the checkout", state, err)
	}
	ops := newFakeOps(t, installation)
	ops.stateRoot = func(candidate string) (string, int) {
		root, err := stateroot.RootForCandidate(candidate)
		if err != nil {
			return "", 1
		}
		return root.Path() + "\n", 0
	}
	ops.turnVerdict = func(request TurnVerdictRequest) (string, string, int) {
		_ = os.WriteFile(request.CompletionFile, []byte("{}\n"), 0o600)
		return ops.verdict, "", 0
	}
	runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"separated"}`})
	if _, err := os.Stat(filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals", "separated.json")); err != nil {
		t.Fatalf("no stop-refusal record under the installation: %v\n%s", err, ops.trace())
	}
	if _, err := os.Stat(filepath.Join(checkout, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("the checkout gained run state: %v", err)
	}
}

// Admitting the hook's installation keeps its answers: an installation whose
// engine is missing still answers engine-missing, and only a directory
// without metasystem.conf, which is no installation, answers as a hook that
// finds none.
func TestFenceAndHooksAdmissionKeepsTheEngineMissingAnswer(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	if err := os.Remove(filepath.Join(installation.root, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	for index, want := range []string{"engine-missing", "checkout-identification"} {
		if index == 1 {
			if err := os.Remove(filepath.Join(installation.root, "metasystem.conf")); err != nil {
				t.Fatal(err)
			}
		}
		run := runHook(t, installation, newFakeOps(t, installation), hookCall{runtime: "claude", event: "start", payload: "{}"})
		if run.status != 0 || run.stdout != noticeOf(want) {
			t.Fatalf("start = status %d stdout %q, want %s", run.status, run.stdout, want)
		}
	}
}
