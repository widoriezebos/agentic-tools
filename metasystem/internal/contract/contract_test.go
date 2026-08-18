package contract

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
// sealableContract is baseContract with the no-gain budget raised to the
// cycle fence: the binary-gate short-fuse refusal (issue #4) makes the
// original combination unsignable by design.
func sealableContract() string {
	return strings.Replace(baseContract(), "ledger.no-gain-budget=2", "ledger.no-gain-budget=3", 1)
}

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
	writeFileMode(t, contractPath, sealableContract(), 0o644)
	return repo, contractPath
}

func TestContractValidateAcceptsBase(t *testing.T) {
	_, contractPath := newContractRepo(t)
	resolved, warnings, err := Validate(contractPath)
	if err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	if !strings.HasSuffix(resolved, "mission-alpha.contract.md") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}
	// no-gain 2 against fence.cycles 3 is not below half the fence, but
	// it sits under the critique cadence — the base fixture is
	// fixture-scale on purpose, and the cadence warning is expected.
	if len(warnings) != 2 || !strings.Contains(warnings[0], "critique cadence") ||
		!strings.Contains(warnings[1], "host.max-turns is not sealed") {
		t.Fatalf("base contract should carry the cadence and unsealed-cap warnings: %v", warnings)
	}
}

func TestContractValidateWarnsOnUndersizedNoGainBudget(t *testing.T) {
	_, contractPath := newContractRepo(t)
	undersized := strings.Replace(baseContract(), "fence.cycles=3", "fence.cycles=6", 1)
	writeFileMode(t, contractPath, undersized, 0o644)
	_, warnings, err := Validate(contractPath)
	if err != nil {
		t.Fatalf("an undersized no-gain budget warns, never refuses: %v", err)
	}
	if len(warnings) != 3 || !strings.Contains(warnings[0], "docs/design/stop-loss-core.md") {
		t.Fatalf("expected the half-fence, cadence, and unsealed-cap warnings, got %v", warnings)
	}
	// Exactly half the fence is not below half: only the cadence warning
	// remains (baseContract's budget of 2 is under the critique cadence).
	atHalf := strings.Replace(baseContract(), "fence.cycles=3", "fence.cycles=4", 1)
	writeFileMode(t, contractPath, atHalf, 0o644)
	if _, warnings, err = Validate(contractPath); err != nil ||
		len(warnings) != 2 || !strings.Contains(warnings[0], "critique cadence") {
		t.Fatalf("a budget at half the fence warns for cadence and unsealed caps: %v %v", warnings, err)
	}
	// A budget of 3 sits exactly at the critique cadence: still fused
	// before a serialized host implements, so it warns; 4 clears it.
	atCadence := strings.Replace(strings.Replace(baseContract(),
		"ledger.no-gain-budget=2", "ledger.no-gain-budget=3", 1),
		"fence.cycles=3", "fence.cycles=6", 1)
	writeFileMode(t, contractPath, atCadence, 0o644)
	if _, warnings, err = Validate(contractPath); err != nil ||
		len(warnings) != 2 || !strings.Contains(warnings[0], "critique cadence") {
		t.Fatalf("budget 3 warns for cadence and unsealed caps: %v %v", warnings, err)
	}
	aboveCadence := strings.Replace(strings.Replace(baseContract(),
		"ledger.no-gain-budget=2", "ledger.no-gain-budget=4", 1),
		"fence.cycles=3", "fence.cycles=8", 1)
	writeFileMode(t, contractPath, aboveCadence, 0o644)
	if _, warnings, err = Validate(contractPath); err != nil ||
		len(warnings) != 1 || !strings.Contains(warnings[0], "host.max-turns is not sealed") {
		t.Fatalf("budget 4 with cycles 8 warns only for unsealed caps: %v %v", warnings, err)
	}
	// Sealing the caps clears the last warning entirely.
	sealed := strings.Replace(aboveCadence, "host.turn-cap-min=",
		"host.max-turns=150\nhost.max-budget-usd=5.00\nhost.turn-cap-min=", 1)
	writeFileMode(t, contractPath, sealed, 0o644)
	if _, warnings, err = Validate(contractPath); err != nil || len(warnings) != 0 {
		t.Fatalf("sealed caps must clear every warning: %v %v", warnings, err)
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
			_, _, err := Validate(contractPath)
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
	digest, err := Seal(contractPath)
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
	if _, err := Seal(contractPath); err == nil || !strings.Contains(err.Error(), "already sealed") {
		t.Fatalf("expected already-sealed refusal, got %v", err)
	}
}

func TestContractSealRefusesAfterApproval(t *testing.T) {
	_, contractPath := newContractRepo(t)
	writeFileMode(t, contractPath,
		sealableContract()+"\nApproval: name=Human; date=2026-08-04; contract-sha256="+strings.Repeat("0", 64)+"\n", 0o644)
	if _, err := Seal(contractPath); err == nil || !strings.Contains(err.Error(), "before approval") {
		t.Fatalf("expected refusal to seal after approval, got %v", err)
	}
}

func TestContractPreflightUnsealed(t *testing.T) {
	_, contractPath := newContractRepo(t)
	_, _, err := Preflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "unsealed") {
		t.Fatalf("expected unsealed refusal, got %v", err)
	}
}

func TestContractPreflightUnsigned(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	_, _, err := Preflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "unsigned") {
		t.Fatalf("expected unsigned refusal, got %v", err)
	}
}

func TestContractPreflightApprovalHashMismatch(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	data, _ := os.ReadFile(contractPath)
	writeFileMode(t, contractPath,
		string(data)+"\nApproval: name=Human; date=2026-08-04; contract-sha256="+strings.Repeat("0", 64)+"\n", 0o644)
	_, _, err := Preflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "approval hash") {
		t.Fatalf("expected approval-hash refusal, got %v", err)
	}
}

func TestContractPreflightStaleExposure(t *testing.T) {
	_, contractPath := newContractRepo(t)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	data, _ := os.ReadFile(contractPath)
	// Change an authored fence value after sealing; the sealed exposure now
	// disagrees with the live contract.
	writeFileMode(t, contractPath, strings.Replace(string(data), "fence.jobs=4", "fence.jobs=5", 1), 0o644)
	_, _, err := Preflight(contractPath, "")
	if err == nil || !strings.Contains(err.Error(), "exposure is stale") {
		t.Fatalf("expected stale-exposure refusal, got %v", err)
	}
}

// TestContractApprovalHashStable proves the approval digest is invariant to the
// approval line it protects: sealing yields a digest, and appending exactly
// that approval leaves preflight's recomputed hash matching.
func TestContractApprovalHashStable(t *testing.T) {
	_, contractPath := newContractRepo(t)
	digest, err := Seal(contractPath)
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
	withPatience := strings.Replace(sealableContract(), "exposure=EUR:10",
		"exposure=EUR:10\npatience.rounds.implementer.codex.gpt-5-6-sol=4", 1)
	writeFileMode(t, contractPath, withPatience, 0o644)
	if _, _, err := Validate(contractPath); err != nil {
		t.Fatalf("patience-bearing contract rejected: %v", err)
	}
	digest, err := Seal(contractPath)
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
	if _, _, err := Preflight(contractPath, ""); err != nil &&
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
			_, _, err := Validate(contractPath)
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("expected %q refusal, got %v", tc.contains, err)
			}
		})
	}
}

// The hash-only surface (script-validate-1/D34), pinned against an
// independently constructed canonical image: approval lines dropped,
// per-line trailing whitespace stripped, trailing blanks trimmed.
func TestCanonicalContractHash(t *testing.T) {
	text := "# Title  \nbody line\t\nApproval: name=X; contract-sha256=deadbeef\n\n  \n"
	canonical := "# Title\nbody line"
	expected := sha256Hex(canonical)
	if got := CanonicalContractHash(text); got != expected {
		t.Fatalf("canonical hash drifted: got %s want %s", got, expected)
	}
}

// The per-key grammar matrix, ported from mission-fixtures.sh's mutation
// table (script-fixtures-003/D38): every key of the base contract must
// reject when MISSING and when MALFORMED with the shell table's exact bad
// values. The shell drove ~52 assert-mission subprocesses per suite run to
// prove what this table proves in-process under the gate.
func TestContractValidateRejectsPerKeyMatrix(t *testing.T) {
	badValues := map[string]string{
		"gate.command":           " value ",
		"gate.ref":               "bad ref",
		"gate.paths":             "../outside",
		"truth.paths":            "/absolute",
		"truth.certification":    "goldish",
		"gate.direction":         "up",
		"gate.threshold.score":   "=1",
		"gate.noise-floor.score": "-1",
		"guard.audit.command":    " value ",
		"guard.audit.floor":      "one",
		"guard.audit.noise":      "-1",
		"guard.cadence":          "0",
		"ledger.cycle-budget":    "0",
		"ledger.no-gain-budget":  "none",
		"fence.wall-clock-hours": "0",
		"fence.cycles":           "0",
		"fence.jobs":             "all",
		"fence.concurrency":      "0",
		"fence.job-cap-min":      "1.5",
		"host.runtime":           "bad runtime",
		"host.model":             "bad model",
		"host.turn-cap-min":      "0",
		"stream.primary":         " value ",
		"envelope.dependencies":  "whatever seems safe",
		"exposure":               "10-ish",
	}
	base := baseContract()
	for _, line := range strings.Split(base, "\n") {
		key, _, found := strings.Cut(line, "=")
		if !found || strings.HasPrefix(line, "```") || strings.Contains(key, " ") {
			continue
		}
		bad, mapped := badValues[key]
		t.Run("missing "+key, func(t *testing.T) {
			_, contractPath := newContractRepo(t)
			writeFileMode(t, contractPath, removeLine(base, line), 0o644)
			if _, _, err := Validate(contractPath); err == nil {
				t.Fatalf("a contract missing %s validated", key)
			}
		})
		if !mapped {
			t.Fatalf("no malformed value mapped for key %s — extend the table", key)
		}
		t.Run("malformed "+key, func(t *testing.T) {
			_, contractPath := newContractRepo(t)
			writeFileMode(t, contractPath, strings.Replace(base, line, key+"="+bad, 1), 0o644)
			if _, _, err := Validate(contractPath); err == nil {
				t.Fatalf("a contract with %s=%q validated", key, bad)
			}
		})
	}
}

// Issue #4 option 3: a single binary gate metric with a no-gain budget
// below the cycle fence is UNSIGNABLE — refused at seal by name.
func TestContractSealRefusesBinaryGateShortFuse(t *testing.T) {
	_, contractPath := newContractRepo(t)
	writeFileMode(t, contractPath, baseContract(), 0o644)
	if _, err := Seal(contractPath); err == nil ||
		!strings.Contains(err.Error(), "parks perfect play") {
		t.Fatalf("binary-gate short fuse must refuse at seal: %v", err)
	}
	// Raising the budget to the fence signs cleanly.
	writeFileMode(t, contractPath, sealableContract(), 0o644)
	if _, err := Seal(contractPath); err != nil {
		t.Fatalf("fuse-compliant contract refused: %v", err)
	}
}
