package behaviorsurface

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeConsumerFixture(t *testing.T, path, body string, executable bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0o644)
	if executable {
		mode = 0o755
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func consumerGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func runConsumerWrapper(root, marker string, args ...string) (string, error) {
	commandArgs := append([]string{"__lease-held", "human"}, args...)
	command := exec.Command(filepath.Join(root, "scripts", "agents", "commit.sh"), commandArgs...)
	command.Env = append(os.Environ(), "CONSUMER_PROOF_MARKER="+marker)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestLandingConsumerUsesProspectivePolicyForStaleBinaryAndRename(t *testing.T) {
	root := t.TempDir()
	commitBody, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "commit.sh"))
	if err != nil {
		t.Fatal(err)
	}
	coverageDeltaBody, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "coverage-delta.sh"))
	if err != nil {
		t.Fatal(err)
	}
	writeConsumerFixture(t, filepath.Join(root, "scripts", "agents", "commit.sh"), string(commitBody), true)
	writeConsumerFixture(t, filepath.Join(root, "scripts", "agents", "coverage-delta.sh"), string(coverageDeltaBody), true)
	writeConsumerFixture(t, filepath.Join(root, "scripts", "audit-metasystem.sh"), "#!/usr/bin/env bash\nexit 0\n", true)
	writeConsumerFixture(t, filepath.Join(root, "scripts", "agents", "go-gate.sh"), `#!/usr/bin/env bash
set -euo pipefail
proof=
while (($#)); do
  case "$1" in --proof-out) proof=$2; shift 2 ;; *) shift ;; esac
done
[[ -n "$proof" ]]
cp "$(cd "$(dirname "$0")" && pwd -P)/prospective-engine.sh" "$proof"
chmod +x "$proof"
`, true)
	writeConsumerFixture(t, filepath.Join(root, "bin", "metasystem"), `#!/usr/bin/env bash
case "$1 $2" in
  "behavior-surface select")
    echo "stale live policy binary was consulted" >&2
    exit 91 ;;
  "lease require-holder") echo '{}' ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token"|"gate weight-add") : ;;
  *) : ;;
esac
`, true)
	prospectivePolicy := `#!/usr/bin/env bash
set -euo pipefail
[[ -z "${CONSUMER_PROOF_MARKER:-}" ]] || : >"$CONSUMER_PROOF_MARKER"
case "$1 $2" in
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals.md|memory/receipts.log) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
  "gate weight-add") : ;;
  *) : ;;
esac
`
	proofPath := filepath.Join(root, "scripts", "agents", "prospective-engine.sh")
	oldPolicy := strings.Replace(prospectivePolicy, "*) printf", "prospective.txt) ;; *) printf", 1)
	writeConsumerFixture(t, proofPath, oldPolicy, true)
	writeConsumerFixture(t, filepath.Join(root, ".gitignore"), "bin/\nartifacts/\n", false)
	writeConsumerFixture(t, filepath.Join(root, "README.md"), "seed\n", false)

	consumerGit(t, root, "init", "-q", "-b", "main")
	consumerGit(t, root, "config", "user.name", "fixture")
	consumerGit(t, root, "config", "user.email", "fixture@example.invalid")
	consumerGit(t, root, "config", "metasystem.goal.machine", "fixture")
	consumerGit(t, root, "add", ".")
	consumerGit(t, root, "commit", "-qm", "seed")

	// The prospective policy starts including this path while the stale live
	// binary cannot classify anything. The wrapper must use the proof artifact
	// and refuse the untracked projected byte by name.
	writeConsumerFixture(t, proofPath, prospectivePolicy, true)
	consumerGit(t, root, "add", "scripts/agents/prospective-engine.sh")
	writeConsumerFixture(t, filepath.Join(root, "prospective.txt"), "not staged\n", false)
	marker := filepath.Join(t.TempDir(), "proof-used")
	output, err := runConsumerWrapper(root, marker, "-m", "must refuse prospective input")
	if err == nil || !strings.Contains(output, "prospective.txt") {
		t.Fatalf("prospective policy did not refuse its own newly included path: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("proof-built engine was not invoked: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "prospective.txt")); err != nil {
		t.Fatal(err)
	}

	// A stale live policy binary that exits 91 cannot block a lawful landing;
	// the proof-built engine owns both selection and weight classification.
	writeConsumerFixture(t, filepath.Join(root, "README.md"), "prospective policy landing\n", false)
	consumerGit(t, root, "add", "README.md")
	output, err = runConsumerWrapper(root, marker, "-q", "-m", "prospective policy wins")
	if err != nil {
		t.Fatalf("stale live binary blocked prospective policy: %v\n%s", err, output)
	}
	if !strings.Contains(output, "coverage delta: no ratchet registry at this root; skipped") {
		t.Fatalf("registry-free root did not state the coverage skip:\n%s", output)
	}

	// Intent-to-add makes both sides of this unstaged cross-class rename
	// visible to diff. Rename detection would emit only the excluded
	// destination; --no-renames must retain the projected removal.
	writeConsumerFixture(t, filepath.Join(root, "landing.txt"), "landing bytes\n", false)
	consumerGit(t, root, "add", "landing.txt")
	consumerGit(t, root, "commit", "-qm", "rename bed")
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "landing.txt"), filepath.Join(root, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	consumerGit(t, root, "add", "-N", "plans/goals.md")
	output, err = runConsumerWrapper(root, marker, "-m", "must refuse hidden removal")
	if err == nil || !strings.Contains(output, "landing.txt") {
		t.Fatalf("rename detection hid the projected removal: %v\n%s", err, output)
	}

	// Restore the rename bed, then prove the suffix-based critical-input guard
	// covers nested scripts and the repository workspace checksum.
	if err := os.Remove(filepath.Join(root, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	writeConsumerFixture(t, filepath.Join(root, "landing.txt"), "landing bytes\n", false)
	consumerGit(t, root, "add", "-A")
	writeConsumerFixture(t, filepath.Join(root, "target.txt"), "target\n", false)
	consumerGit(t, root, "add", "target.txt")
	consumerGit(t, root, "commit", "-qm", "symlink bed")
	if err := os.MkdirAll(filepath.Join(root, "nested", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "target.txt"), filepath.Join(root, "nested", "scripts", "check.sh")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "go.work.sum")); err != nil {
		t.Fatal(err)
	}
	consumerGit(t, root, "add", "nested/scripts/check.sh", "go.work.sum")
	output, err = runConsumerWrapper(root, marker, "-m", "must refuse critical symlinks")
	if err == nil || !strings.Contains(output, "nested/scripts/check.sh") || !strings.Contains(output, "go.work.sum") {
		t.Fatalf("nested critical symlinks were accepted: %v\n%s", err, output)
	}
}

func TestEveryEffectiveDeliverySkipIsPolicyOwned(t *testing.T) {
	policy := mustPolicy(t)
	const witnessEngineGate = "witness-engine-gate"
	if !policy.SkipAllowed(WitnessScope, witnessEngineGate) {
		t.Fatalf("the witnessed engine-gate omission is absent from policy: %q", witnessEngineGate)
	}
	goGate, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	consult := `--scope WITNESS --family witness-engine-gate`
	if !strings.Contains(string(goGate), `go run ./cmd/metasystem behavior-surface skip-allowed`) || !strings.Contains(string(goGate), consult) {
		t.Fatalf("the witness fast path does not consult its declared skip family: %q", witnessEngineGate)
	}
	for _, family := range policy.DeliveryContractSkips {
		if !policy.SkipAllowed(DeliveryScope, family) {
			t.Errorf("declared family was not accepted: %q", family)
		}
	}
	if policy.SkipAllowed(DeliveryScope, "not-declared") {
		t.Fatal("undeclared validation family was accepted")
	}
}
