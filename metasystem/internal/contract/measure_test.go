package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newMeasureRepo builds a sealed contract over a gate that emits both the
// declared score metric and the audit guard's own metric, so the whole
// measurement path — gate, guards, and classification — can run.
func newMeasureRepo(t *testing.T) (repo, contractPath string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo, "-c", "init.defaultBranch=main", "init", "-q")
	runGitCmd(t, repo, "config", "user.name", "metasystem")
	runGitCmd(t, repo, "config", "user.email", "metasystem@example.invalid")
	runGitCmd(t, repo, "config", "commit.gpgsign", "false")

	writeFileMode(t, filepath.Join(repo, "scripts", "gate.sh"),
		"#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\nmetric=audit=1\\n'\n", 0o755)
	writeFileMode(t, filepath.Join(repo, "truth", "reference.txt"), "certified truth\n", 0o644)
	writeFileMode(t, filepath.Join(repo, "docs", "project-rules.md"), projectRules, 0o644)
	runGitCmd(t, repo, "add", ".")
	runGitCmd(t, repo, "commit", "-qm", "instruments")
	runGitCmd(t, repo, "tag", "instruments")

	contractPath = filepath.Join(repo, "plans", "mission-alpha.contract.md")
	writeFileMode(t, contractPath, sealableContract(), 0o644)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	return repo, contractPath
}

func TestContractMeasureBaseline(t *testing.T) {
	_, contractPath := newMeasureRepo(t)
	result, err := Measure(contractPath, nil)
	if err != nil {
		t.Fatalf("measure failed: %v", err)
	}
	if result.Classification != "unresolved" {
		t.Fatalf("measuring at the baseline should be unresolved, got %s", result.Classification)
	}
	if result.Metrics["score"] != "1" {
		t.Fatalf("unexpected score metric: %q", result.Metrics["score"])
	}
	if result.Guards["audit"] != "1" {
		t.Fatalf("unexpected audit guard: %q", result.Guards["audit"])
	}
	if result.Observed != "score=1" {
		t.Fatalf("unexpected observed line: %q", result.Observed)
	}
	if len(result.CandidateSHA) < 7 {
		t.Fatalf("expected a candidate sha, got %q", result.CandidateSHA)
	}
	if !result.GatePassed {
		t.Fatal("gate should pass: score clears its threshold and audit clears its floor")
	}
}

func TestContractMeasureClassifiesAgainstPrevious(t *testing.T) {
	_, contractPath := newMeasureRepo(t)
	improved, err := Measure(contractPath, map[string]string{"score": "0"})
	if err != nil {
		t.Fatalf("measure failed: %v", err)
	}
	if improved.Classification != "contract-improved" {
		t.Fatalf("rising past the noise floor should improve, got %s", improved.Classification)
	}
	regressed, err := Measure(contractPath, map[string]string{"score": "2"})
	if err != nil {
		t.Fatalf("measure failed: %v", err)
	}
	if regressed.Classification != "no-progress" {
		t.Fatalf("falling past the noise floor should be no-progress, got %s", regressed.Classification)
	}
}

func TestContractMeasureGuardFloor(t *testing.T) {
	_, contractPath := newMeasureRepo(t)
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	// Raise the audit floor above the value the guard emits; the gate must now
	// fail even though the score metric still clears its threshold.
	writeFileMode(t, contractPath, strings.Replace(string(data), "guard.audit.floor=1", "guard.audit.floor=5", 1), 0o644)
	result, err := Measure(contractPath, nil)
	if err != nil {
		t.Fatalf("measure failed: %v", err)
	}
	if result.GatePassed {
		t.Fatal("gate must fail when the audit guard is below its floor")
	}
	if result.Guards["audit"] != "1" {
		t.Fatalf("unexpected audit guard: %q", result.Guards["audit"])
	}
}

// The gate and the guards are arbitrary bash: a gate that MUTATES a guarded
// path must not change what the guards read (mission-contract-2 — each runs
// in its own clean worktree at the same pinned SHAs; a shared live worktree
// would let the gate defeat the frozen-instruments restore).
func TestContractMeasureGuardUnaffectedByGateMutation(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo, "-c", "init.defaultBranch=main", "init", "-q")
	runGitCmd(t, repo, "config", "user.name", "metasystem")
	runGitCmd(t, repo, "config", "user.email", "metasystem@example.invalid")
	runGitCmd(t, repo, "config", "commit.gpgsign", "false")

	// The gate emits its metric, then tampers with the guarded truth file
	// and with the guard's own script in ITS worktree.
	writeFileMode(t, filepath.Join(repo, "scripts", "gate.sh"),
		"#!/usr/bin/env bash\nset -euo pipefail\n"+
			"printf 'metric=score=1\\n'\n"+
			"echo tampered > truth/reference.txt\n"+
			"echo 'echo sabotaged' >> scripts/guard.sh\n", 0o755)
	// The guard reports 1 only while the certified truth bytes are intact.
	writeFileMode(t, filepath.Join(repo, "scripts", "guard.sh"),
		"#!/usr/bin/env bash\nset -euo pipefail\n"+
			"if grep -q 'certified truth' truth/reference.txt; then printf 'metric=audit=1\\n'; else printf 'metric=audit=0\\n'; fi\n", 0o755)
	writeFileMode(t, filepath.Join(repo, "truth", "reference.txt"), "certified truth\n", 0o644)
	writeFileMode(t, filepath.Join(repo, "docs", "project-rules.md"), projectRules, 0o644)
	runGitCmd(t, repo, "add", ".")
	runGitCmd(t, repo, "commit", "-qm", "instruments")
	runGitCmd(t, repo, "tag", "instruments")

	contractPath := filepath.Join(repo, "plans", "mission-alpha.contract.md")
	contract := strings.Replace(sealableContract(),
		"gate.paths=scripts/gate.sh", "gate.paths=scripts/*.sh", 1)
	contract = strings.Replace(contract,
		"guard.audit.command=scripts/gate.sh", "guard.audit.command=scripts/guard.sh", 1)
	writeFileMode(t, contractPath, contract, 0o644)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	result, err := Measure(contractPath, nil)
	if err != nil {
		t.Fatalf("measure failed: %v", err)
	}
	if result.Guards["audit"] != "1" {
		t.Fatalf("the gate's mutation leaked into the guard's reading: audit=%q", result.Guards["audit"])
	}
	if !result.GatePassed {
		t.Fatal("gate must pass: the guard read the pinned bytes, not the gate's tampering")
	}
}

func TestContractMeasureRefusals(t *testing.T) {
	_, contractPath := newMeasureRepo(t)
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}

	// A failing gate command is a measurement failure with its exit code.
	failing := strings.Replace(string(data), "gate.command=scripts/gate.sh", "gate.command=exit 3", 1)
	writeFileMode(t, contractPath, failing, 0o644)
	if _, err := Measure(contractPath, nil); err == nil ||
		!strings.Contains(err.Error(), "gate measurement failed with exit 3") {
		t.Fatalf("a failing gate must refuse with its exit code: %v", err)
	}

	// An unresolvable gate ref refuses before anything is materialized.
	badRef := strings.Replace(string(data), "gate.ref=instruments", "gate.ref=no-such-tag", 1)
	writeFileMode(t, contractPath, badRef, 0o644)
	if _, err := Measure(contractPath, nil); err == nil {
		t.Fatal("an unresolvable gate.ref must refuse the measurement")
	}
}
