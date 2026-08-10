package mission

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

const oneCycleLedger = `# Mission Ledger

- Cycle budget: 5
- No-gain budget: 3

### Cycle 1
- Classification: contract-improved; candidate-sha=abcdef; observed=score=0.5
`

// anchoredMission builds a git repo on the mission branch with a one-cycle
// ledger and a state whose ledger cycle count matches, then anchors it.
func anchoredMission(t *testing.T) (repo, state, ledger string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo = t.TempDir()
	gitCmd(t, repo, "init", "-q")
	writeText(t, filepath.Join(repo, "README"), "seed\n")
	gitCmd(t, repo, "add", "README")
	gitCmd(t, repo, "commit", "-q", "-m", "init")
	gitCmd(t, repo, "checkout", "-q", "-b", "feature-x")

	ledger = filepath.Join(repo, "ledger.md")
	writeText(t, ledger, oneCycleLedger)
	contract := filepath.Join(repo, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n")
	state = filepath.Join(repo, "state.json")
	if err := InitState(state, contract, ledger, "", "feature-x"); err != nil {
		t.Fatal(err)
	}

	// Advance the state's recorded ledger cycles to match the ledger (1).
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["ledger"].(map[string]any)["cycles"] = 1
	src := state + ".src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, src, hash); err != nil {
		t.Fatal(err)
	}

	if err := Anchor(state, repo, ledger); err != nil {
		t.Fatalf("anchor: %v", err)
	}
	return repo, state, ledger
}

func TestAnchorAndVerifyRoundTrip(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	seq, hash, err := VerifyStateWithAnchor(state, repo, ledger)
	if err != nil {
		t.Fatalf("a freshly anchored state should verify: %v", err)
	}
	if !hashRe.MatchString(hash) || seq < 1 {
		t.Fatalf("unexpected verify result: seq=%d hash=%q", seq, hash)
	}
	// The anchor commit carries the mission trailers.
	log := gitCmd(t, repo, "log", "-1", "--format=%B")
	if !strings.Contains(log, "Mission-Id: demo") || !strings.Contains(log, "Mission-Cycle: 1") {
		t.Fatalf("anchor commit missing trailers:\n%s", log)
	}
}

func TestVerifyAnchorDetectsLedgerTamper(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	// Tamper the working-tree ledger after anchoring.
	writeText(t, ledger, oneCycleLedger+"\ntampered\n")
	if _, _, err := VerifyStateWithAnchor(state, repo, ledger); err == nil ||
		!strings.Contains(err.Error(), "Mission-Ledger-SHA256") {
		t.Fatalf("a tampered ledger must fail anchor verification, got %v", err)
	}
}

func TestReconcileEqualCyclesReturnsZero(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	code, err := Reconcile(state, repo, ledger)
	if err != nil {
		t.Fatalf("reconcile errored: %v", err)
	}
	if code != 0 {
		t.Fatalf("a state that matches its anchor should reconcile to 0, got %d", code)
	}
}

func TestReconcileParksOnLedgerBehindState(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	// Truncate the ledger so its cycle count (0) is behind the state (1).
	writeText(t, ledger, "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n")
	code, err := Reconcile(state, repo, ledger)
	if err != nil {
		t.Fatalf("reconcile errored: %v", err)
	}
	if code != 3 {
		t.Fatalf("a ledger behind the state must park (3), got %d", code)
	}
	doc, _ := readStateDoc(state)
	if r, _ := doc["parkReason"].(string); r != "state-integrity" {
		t.Fatalf("the mission should be parked for state-integrity, got %q", r)
	}
}
