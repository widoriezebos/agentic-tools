package mission

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const contractFence = "```"

// baseContract is a complete, valid mission contract over the fixture repo's
// frozen instruments.
func baseContract() string {
	return strings.Join([]string{
		"# Intent", "",
		"Reach the declared score with frozen instruments.", "",
		"# Non-goals", "",
		"Do not publish or deploy.", "",
		"# Initial streams", "",
		"Keep one stream active.", "",
		contractFence + "mission",
		"gate.command=scripts/gate.sh",
		"gate.ref=instruments",
		"gate.paths=scripts/gate.sh",
		"truth.paths=truth/*.txt",
		"truth.certification=certified",
		"gate.direction=max",
		"gate.threshold.score=>=1",
		"gate.noise-floor.score=0",
		"guard.audit.command=scripts/gate.sh",
		"guard.audit.floor=1",
		"guard.audit.noise=0",
		"guard.cadence=1",
		"ledger.cycle-budget=3",
		"ledger.no-gain-budget=2",
		"fence.wall-clock-hours=2",
		"fence.cycles=3",
		"fence.jobs=4",
		"fence.concurrency=2",
		"fence.job-cap-min=30",
		"host.runtime=fake",
		"host.model=fake-model",
		"host.turn-cap-min=20",
		"stream.primary=Reach the acceptance score.",
		"envelope.dependencies=jq",
		"exposure=EUR:10",
		contractFence,
		"",
	}, "\n")
}

const projectRules = `### Mission envelope eligibility

| id | Description | pre-authorizable | bound |
| --- | --- | --- | --- |
| ` + "`dependencies`" + ` | Adding or upgrading dependencies | yes | dependency allowlist |
| ` + "`dispatch-allow`" + ` | Dispatching away from the roster | yes | exact runtime:model pairs |
| ` + "`production-data`" + ` | Production data | no | never |
`

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFileMode(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// newContractRepo builds a git repository with the frozen instruments committed
// and tagged, and writes the base contract at plans/mission-alpha.contract.md.
func newContractRepo(t *testing.T) (repo, contractPath string) {
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
		"#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\n'\n", 0o755)
	writeFileMode(t, filepath.Join(repo, "truth", "reference.txt"), "certified truth\n", 0o644)
	writeFileMode(t, filepath.Join(repo, "docs", "project-rules.md"), projectRules, 0o644)
	writeFileMode(t, filepath.Join(repo, "scripts", "agents", "arm-supervision.sh"),
		"#!/usr/bin/env bash\nprintf 'fixture-fingerprint\\n'\n", 0o755)
	runGitCmd(t, repo, "add", ".")
	runGitCmd(t, repo, "commit", "-qm", "instruments")
	runGitCmd(t, repo, "tag", "instruments")

	contractPath = filepath.Join(repo, "plans", "mission-alpha.contract.md")
	writeFileMode(t, contractPath, baseContract(), 0o644)
	return repo, contractPath
}

func TestContractValidateAcceptsBase(t *testing.T) {
	_, contractPath := newContractRepo(t)
	resolved, warnings, err := ContractValidate(contractPath)
	if err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	if !strings.HasSuffix(resolved, "mission-alpha.contract.md") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}
	// no-gain 2 against fence.cycles 3 is not below half the fence.
	if len(warnings) != 0 {
		t.Fatalf("base contract should carry no calibration warning: %v", warnings)
	}
}

func TestContractValidateWarnsOnUndersizedNoGainBudget(t *testing.T) {
	_, contractPath := newContractRepo(t)
	undersized := strings.Replace(baseContract(), "fence.cycles=3", "fence.cycles=6", 1)
	writeFileMode(t, contractPath, undersized, 0o644)
	_, warnings, err := ContractValidate(contractPath)
	if err != nil {
		t.Fatalf("an undersized no-gain budget warns, never refuses: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "plans/stop-loss-core.md") {
		t.Fatalf("expected one warning naming the design, got %v", warnings)
	}
	// Exactly half the fence is not below half: no warning.
	atHalf := strings.Replace(baseContract(), "fence.cycles=3", "fence.cycles=4", 1)
	writeFileMode(t, contractPath, atHalf, 0o644)
	if _, warnings, err = ContractValidate(contractPath); err != nil || len(warnings) != 0 {
		t.Fatalf("a budget at half the fence must not warn: %v %v", warnings, err)
	}
}

func TestContractValidateRejects(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(string) string
		contains string
	}{
		{"missing gate.command", func(s string) string {
			return removeLine(s, "gate.command=scripts/gate.sh")
		}, "missing required key(s): gate.command"},
		{"malformed exposure", func(s string) string {
			return strings.Replace(s, "exposure=EUR:10", "exposure=10-ish", 1)
		}, "exposure must be a human-priced amount"},
		{"unbounded envelope", func(s string) string {
			return strings.Replace(s, "envelope.dependencies=jq", "envelope.dependencies=all", 1)
		}, "unbounded or non-literal"},
		{"forbidden envelope", func(s string) string {
			return strings.Replace(s, "envelope.dependencies=jq", "envelope.production-data=fixture", 1)
		}, "not marked pre-authorizable"},
		{"retired tier-move", func(s string) string {
			return strings.Replace(s, "envelope.dependencies=jq", "envelope.tier-move=3", 1)
		}, "use envelope.dispatch-allow"},
		{"unknown key", func(s string) string {
			return strings.Replace(s, "exposure=EUR:10", "exposure=EUR:10\nmystery.key=1", 1)
		}, "unknown key"},
		{"noise floor without threshold", func(s string) string {
			return strings.Replace(s, "gate.noise-floor.score=0", "gate.noise-floor.other=0", 1)
		}, "matching noise floor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			_ = repo
			_, contractPath := newContractRepo(t)
			writeFileMode(t, contractPath, tc.mutate(baseContract()), 0o644)
			_, _, err := ContractValidate(contractPath)
			if err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.contains)
			}
		})
	}
}

func TestContractSealWritesBlockAndDigest(t *testing.T) {
	_, contractPath := newContractRepo(t)
	digest, err := ContractSeal(contractPath)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	if !hashRe.MatchString(digest) {
		t.Fatalf("seal digest is not a sha256: %q", digest)
	}
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"```mission-seal",
		"sealed.version=1",
		"candidate.branch=main",
		"sealed.baseline.failure-count=0",
		"sealed.baseline.score=1",
		"sealed.exposure.fence.jobs=4",
		"sealed.exposure.statement=EUR:10|fence.wall-clock-hours=2,fence.cycles=3,fence.jobs=4,fence.concurrency=2,fence.job-cap-min=30",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sealed contract missing %q\n%s", want, text)
		}
	}

	// A sealed contract may not be resealed.
	if _, err := ContractSeal(contractPath); err == nil || !strings.Contains(err.Error(), "already sealed") {
		t.Fatalf("expected already-sealed refusal, got %v", err)
	}
}

func TestContractSealRefusesAfterApproval(t *testing.T) {
	_, contractPath := newContractRepo(t)
	writeFileMode(t, contractPath,
		baseContract()+"\nApproval: name=Human; date=2026-08-04; contract-sha256="+strings.Repeat("0", 64)+"\n", 0o644)
	if _, err := ContractSeal(contractPath); err == nil || !strings.Contains(err.Error(), "before approval") {
		t.Fatalf("expected refusal to seal after approval, got %v", err)
	}
}

func TestContractPreflightUnsealed(t *testing.T) {
	_, contractPath := newContractRepo(t)
	_, _, err := ContractPreflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "unsealed") {
		t.Fatalf("expected unsealed refusal, got %v", err)
	}
}

func TestContractPreflightUnsigned(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := ContractSeal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	_, _, err := ContractPreflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "unsigned") {
		t.Fatalf("expected unsigned refusal, got %v", err)
	}
}

func TestContractPreflightApprovalHashMismatch(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := ContractSeal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	data, _ := os.ReadFile(contractPath)
	writeFileMode(t, contractPath,
		string(data)+"\nApproval: name=Human; date=2026-08-04; contract-sha256="+strings.Repeat("0", 64)+"\n", 0o644)
	_, _, err := ContractPreflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "approval hash") {
		t.Fatalf("expected approval-hash refusal, got %v", err)
	}
}

func TestContractPreflightStaleExposure(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := ContractSeal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	data, _ := os.ReadFile(contractPath)
	// Change an authored fence value after sealing; the sealed exposure now
	// disagrees with the live contract.
	writeFileMode(t, contractPath, strings.Replace(string(data), "fence.jobs=4", "fence.jobs=5", 1), 0o644)
	_, _, err := ContractPreflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "exposure is stale") {
		t.Fatalf("expected stale-exposure refusal, got %v", err)
	}
}

// TestContractApprovalHashStable proves the approval digest is invariant to the
// approval line it protects: sealing yields a digest, and appending exactly
// that approval leaves preflight's recomputed hash matching.
func TestContractApprovalHashStable(t *testing.T) {
	_, contractPath := newContractRepo(t)
	digest, err := ContractSeal(contractPath)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	data, _ := os.ReadFile(contractPath)
	signed := string(data) + "\nApproval: name=Human; date=2026-08-04; contract-sha256=" + digest + "\n"
	writeFileMode(t, contractPath, signed, 0o644)
	doc, err := contractRead(resolvePath(contractPath))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if doc.approval == nil {
		t.Fatal("approval not parsed")
	}
	if err := doc.verifyApproval(); err != nil {
		t.Fatalf("approval over sealed bytes rejected: %v", err)
	}
}

func TestContractGlobRegexSpansSeparators(t *testing.T) {
	re, err := contractGlobRegex("docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("docs/a.md") || re.MatchString("docs/a.txt") {
		t.Fatal("simple glob mismatch")
	}
	star, err := contractGlobRegex("docs/*")
	if err != nil {
		t.Fatal(err)
	}
	if !star.MatchString("docs/nested/deep.md") {
		t.Fatal("`*` should span the path separator")
	}
}

func removeLine(text, line string) string {
	var kept []string
	for _, l := range strings.Split(text, "\n") {
		if l == line {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

// Patience-floor entries (plans/patience-satellite-4.md): validated in the
// canonical five-part shape, sealed beside the cap entries through the same
// expectedSeal enumeration preflight recomputes, so a freshly sealed
// patience-bearing contract passes its own preflight (r1/P4-015) and a
// pre-feature contract's seal is unchanged by the feature's existence.
func TestContractPatienceEntriesValidateAndSeal(t *testing.T) {
	_, contractPath := newContractRepo(t)
	withPatience := strings.Replace(baseContract(), "exposure=EUR:10",
		"exposure=EUR:10\npatience.rounds.implementer.codex.gpt-5-6-sol=4", 1)
	writeFileMode(t, contractPath, withPatience, 0o644)
	if _, _, err := ContractValidate(contractPath); err != nil {
		t.Fatalf("patience-bearing contract rejected: %v", err)
	}
	digest, err := ContractSeal(contractPath)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	if !hashRe.MatchString(digest) {
		t.Fatalf("seal digest is not a sha256: %q", digest)
	}
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sealed.exposure.patience.rounds.implementer.codex.gpt-5-6-sol=4") {
		t.Fatalf("patience entry not sealed:\n%s", data)
	}
	// The sealed entry rides the exposure statement echo too.
	if !strings.Contains(string(data), "patience.rounds.implementer.codex.gpt-5-6-sol=4") {
		t.Fatalf("patience entry missing from the echo:\n%s", data)
	}
	// The seal→preflight round-trip (r1/P4-015): approving exactly the seal's
	// digest must let the freshly sealed patience-bearing contract recompute
	// its own seal — field count included — without a stale refusal. A
	// missing ordered-emitter entry fails exactly here.
	signed := string(data) + "\nApproval: name=Human; date=2026-08-12; contract-sha256=" + digest + "\n"
	writeFileMode(t, contractPath, signed, 0o644)
	if _, _, err := ContractPreflight(contractPath, ""); err != nil &&
		(strings.Contains(err.Error(), "seal") || strings.Contains(err.Error(), "stale")) {
		t.Fatalf("patience-bearing seal failed its own preflight recomputation: %v", err)
	}
}

func TestContractPatienceEntriesRejected(t *testing.T) {
	cases := []struct {
		key      string
		contains string
	}{
		{"patience.rounds.implementer.codex=4", "invalid patience key"},
		{"patience.minutes.implementer.codex.gpt-5-6-sol=4", "invalid patience key"},
		{"patience.rounds.implementer.codex.gpt-5.6-sol=4", "not canonical"},
		{"patience.rounds.Implementer.codex.gpt-5-6-sol=4", "invalid patience key"},
		{"patience.rounds.implementer.codex.gpt-5-6-sol=0", "positive integer"},
		{"patience.rounds.implementer.codex.gpt-5-6-sol=four", "positive integer"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			_, contractPath := newContractRepo(t)
			mutated := strings.Replace(baseContract(), "exposure=EUR:10",
				"exposure=EUR:10\n"+tc.key, 1)
			writeFileMode(t, contractPath, mutated, 0o644)
			_, _, err := ContractValidate(contractPath)
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("expected %q refusal, got %v", tc.contains, err)
			}
		})
	}
}
