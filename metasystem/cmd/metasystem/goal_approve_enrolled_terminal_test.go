package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// TestGoalApproveAtTheEnrolledTerminalProvesThroughTheRuntimeRegistry: the
// human-reserved `goal approve` still works once the adapter scripts are gone
// (verbs-object-action U6a/U6c). The installation carries no
// scripts/agents/adapters at all; the enrolled-terminal proof classifies the
// ancestry against the runtime signatures the engine's registry declares
// (humanauthority.signatureSet over census.InstalledAdapterSignatures), so an
// approval run at the enrolled terminal is proven, never relayed, and the same
// approval from another shell is refused.
func TestGoalApproveAtTheEnrolledTerminalProvesThroughTheRuntimeRegistry(t *testing.T) {
	fixture := newObligationCommandFixture(t)
	root := fixture.root()
	_, reader := enrollGoalSyncTerminal(t, root, "ttys:fixture_approve")
	if _, err := os.Stat(filepath.Join(root, "scripts", "agents", "adapters")); !os.IsNotExist(err) {
		t.Fatalf("the installation must carry no adapter scripts: %v", err)
	}
	proveAt := func(shell goalSyncEnrollmentReader) goalAuthorityProver {
		return func(root string, _ int64, _ humanauthority.Reader, word, reviewBy string, now time.Time) (humanauthority.Proof, error) {
			return humanauthority.ProveOrTemporaryGoalAuthority(root, shell.exact.Pid, shell, word, reviewBy, now)
		}
	}
	approve := func(prove goalAuthorityProver) (int, string, string) {
		code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
			return runGoalApproveWithInputs([]string{"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "m1"},
				prove, fixture.commandNow, withStreams(fixture.dependencies(), stdout, stderr), nil)
		})
		return code, stdout, stderr
	}

	other := reader
	other.terminalID = "ttys:another-shell"
	if code, stdout, stderr := approve(proveAt(other)); code == 0 || !strings.Contains(stdout+stderr, "does not descend from the terminal enrolled on this machine") {
		t.Fatalf("an approval from another shell was not refused by the terminal proof: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if refused, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json")); len(refused) != 0 {
		t.Fatalf("a refused approval recorded proofs: %v", refused)
	}

	code, stdout, stderr := approve(proveAt(reader))
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) || strings.Contains(stdout, "TEMPORARY authority") {
		t.Fatalf("goal approve at the enrolled terminal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	_, accepted := fixture.acceptedGoal()
	if rendered := string(accepted); !strings.Contains(rendered, "authority=proven") {
		t.Fatalf("the approval was not recorded as proven:\n%s", rendered)
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("goal approve recorded %d proofs (%v), want one", len(matches), err)
	}
	record, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"action": "goal approve"`, humanauthority.OutcomeProven, `"` + humanauthority.GradeEnrolled + `"`} {
		if !strings.Contains(string(record), want) {
			t.Fatalf("the approval proof lacks %s: %s", want, record)
		}
	}
}
