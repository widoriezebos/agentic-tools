package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestGateArgumentRefusals(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"--fast"}, 2, "go gate: unknown argument --fast"},
		{[]string{"--goal"}, 2, "go gate: --goal needs an accepted goal id"},
		{[]string{"--cap-min", "0"}, 2, "go gate: --cap-min needs positive minutes"},
		{[]string{"--witness-check-only", "--goal", "g"}, 2, "go gate: --witness-check-only is a probe and combines with no other mode"},
		{[]string{"--controller-pid", "12"}, 2, "apply only to --arm"},
		{[]string{"--arm", "plain", "--goal", "g"}, 2, "go gate: --arm combines with no other gate mode"},
		{[]string{"--arm", "sometimes", "--controller-pid", "1", "--state-out", "x"}, 1, "witness-gate refused: --arm must be plain or none (got 'sometimes')"},
		{[]string{"--arm", "plain"}, 2, "--arm needs --controller-pid PID and --state-out FILE"},
	} {
		w := newGateWorld(t)
		if code := w.gate(test.args...); code != test.code || !strings.Contains(w.stderr.String(), test.stderr) {
			t.Fatalf("%q: exit %d stderr %q, want %d with %q", test.args, code, w.stderr.String(), test.code, test.stderr)
		}
		if len(w.calls) != 0 {
			t.Fatalf("%q: a refusal ran tools %v", test.args, w.calls)
		}
	}
}

func TestGateRelaunchesAStandaloneRunUnderItsRetainedProofOwner(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["worker"] = 1
	w.statuses["launch"] = 23
	if code := w.gate("--goal", "fx", "--cap-min", "30"); code != 23 {
		t.Fatalf("exit %d, want the launcher's 23:\n%s", code, w.output())
	}
	builds := w.called("go build -p=3 -o ")
	if len(builds) != 1 || !strings.HasSuffix(builds[0].String(), "/metasystem ./cmd/metasystem") {
		t.Fatalf("standalone engine build = %v", builds)
	}
	engine := builds[0].args[3]
	if !strings.HasPrefix(engine, filepath.Join(w.tmp, "metasystem-go-gate.")) {
		t.Fatalf("private engine %s is not under the gate's temporary root", engine)
	}
	launches := w.called("internal proof-run launch")
	if len(launches) != 1 || launches[0].name != engine {
		t.Fatalf("launches = %v", launches)
	}
	args := launches[0].args
	separator := slices.Index(args, "--")
	if separator < 0 {
		t.Fatalf("launch has no command: %q", args)
	}
	wantCommand := []string{"env", "METASYSTEM_GO_GATE_RELAUNCHED=1", "METASYSTEM_PROOF_AUTH_BIN=" + engine, "METASYSTEM_TEST_WORKERS=3",
		"go", "-C", w.root, "run", "./cmd/devgate", "gate"}
	if !slices.Equal(args[separator+1:], wantCommand) {
		t.Fatalf("relaunched command = %q, want %q", args[separator+1:], wantCommand)
	}
	for _, flag := range [][]string{{"--suite", "go-gate"}, {"--command-class", "go-gate"}, {"--scope", "full"}, {"--goal", "fx"},
		{"--cap-min", "30"}, {"--tmp", filepath.Dir(engine)}, {"--conf", filepath.Join(w.root, "metasystem.conf")},
		{"--banner", "suite-cost suite=go-gate witness=unarmed duration=full-gate"}} {
		index := slices.Index(args[:separator], flag[0])
		if index < 0 || args[index+1] != flag[1] {
			t.Fatalf("launch lacks %q: %q", flag, args[:separator])
		}
	}
	if envValue(launches[0].env, "GOFLAGS") != "-mod=readonly" {
		t.Fatalf("the full proof did not pin readonly module resolution before its reservation: %q", launches[0].env)
	}
	if len(w.called("gofmt")) != 0 {
		t.Fatalf("the unauthorized parent ran gate stages itself")
	}
}

func TestGateRefusesAnUnauthorizedRelaunchedChild(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["worker"] = 1
	w.setenv("METASYSTEM_GO_GATE_RELAUNCHED=1")
	if code := w.gate(); code != 1 || !strings.Contains(w.stderr.String(), "go gate: relaunched child is not an authorized proof worker") {
		t.Fatalf("exit %d stderr %q", code, w.stderr.String())
	}
	if len(w.called("proof-run launch")) != 0 {
		t.Fatalf("an unauthorized relaunched child relaunched again")
	}
}

func TestGateCandidateSourceAuthenticatesUnderAnOlderEngine(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["worker"] = 64
	w.statuses["candidate-worker"] = 0
	w.setenv("METASYSTEM_PROOF_ATTEMPT=attempt-1", "METASYSTEM_PROOF_CONTROL_ROOT="+w.root, "METASYSTEM_GATE_FORCE=1")
	if code := w.gate(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	candidate := w.called("env GOFLAGS=-mod=readonly GOWORK=off CGO_ENABLED=0 go run -p=3 ./cmd/metasystem proof-run worker-authorized --root " + w.root)
	if len(candidate) != 1 {
		t.Fatalf("the candidate verifier did not authenticate the worker exactly once: %v", w.calls)
	}
	if len(w.called("proof-run launch")) != 0 {
		t.Fatalf("an authenticated worker relaunched")
	}
}

func TestGateFullPassRunsEveryStageInOrder(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.gitInside = true
	if code := w.gate(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	var order []string
	for _, call := range w.calls {
		switch {
		case call.name == "gofmt":
			order = append(order, "gofmt")
		case call.name == "go" && call.args[0] == "vet":
			order = append(order, "vet")
		case call.name == "go" && slices.Contains(call.args, staticcheckModule):
			order = append(order, "staticcheck")
		case call.name == "go" && call.args[0] == "build" && envValue(call.env, "GOOS") == "linux":
			if envValue(call.env, "CGO_ENABLED") != "0" {
				t.Fatalf("cross-build without CGO pinned off: %q", call.env)
			}
			order = append(order, "linux/"+envValue(call.env, "GOARCH"))
		case call.name == "go" && slices.Contains(call.args, govulncheckModule):
			order = append(order, "govulncheck")
		case call.name == "go" && call.args[0] == "build" && slices.Contains(call.args, "-o"):
			order = append(order, "build")
		case call.name == "go" && call.args[0] == "list" && !slices.Contains(call.args, "-f"):
			order = append(order, "list")
		case call.name == "go" && call.args[0] == "test":
			t.Fatalf("the full gate ran the fast-only refusal register test")
		}
	}
	want := []string{"gofmt", "vet", "staticcheck", "build", "linux/amd64", "linux/arm64", "govulncheck", "build", "list"}
	if !slices.Equal(order, want) {
		t.Fatalf("stage order = %v, want %v", order, want)
	}
	if len(w.nativeCalls) != 1 || w.nativeCalls[0].Workers != 3 || w.nativeCalls[0].Root != w.root ||
		envValue(w.nativeCalls[0].Environment, "METASYSTEM_TEST_WORKERS") != "3" || envValue(w.nativeCalls[0].Environment, "METASYSTEM_RUN_OWNER") == "" {
		t.Fatalf("native selection calls = %+v", w.nativeCalls)
	}
	if !strings.Contains(w.stdout.String(), "native census and diagnostic\n") {
		t.Fatalf("the native log was not printed:\n%s", w.stdout.String())
	}
	if !strings.HasSuffix(w.stdout.String(), "go gate: PASSED (gofmt, shell parse, vet, race tests, coverage ratchet, build @ "+fixtureCommit[:9]+")\n") {
		t.Fatalf("stdout does not end with the PASSED line:\n%s", w.stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(w.root, "bin", "metasystem"))
	if err != nil || !strings.Contains(string(data), "stamp "+fixtureCommit) {
		t.Fatalf("installed engine = %q, %v", data, err)
	}
	if leftovers, _ := os.ReadDir(w.tmp); len(leftovers) != 0 {
		var names []string
		for _, entry := range leftovers {
			names = append(names, entry.Name())
		}
		t.Fatalf("the passing gate left temporary evidence: %v", names)
	}
	if kept, _ := filepath.Glob(filepath.Join(w.root, "artifacts", "agents", "gate-failures", "*")); len(kept) != 0 {
		t.Fatalf("a passing gate retained failure evidence: %v", kept)
	}
}

func TestGateStageRefusalsNameTheStage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ key, stderr string }{
		{"linux/amd64", "go gate: linux/amd64 cross-build failed"},
		{"linux/arm64", "go gate: linux/arm64 cross-build failed"},
		{"govulncheck", "go gate: govulncheck v1.2.0 refused (or could not run)"},
	} {
		w := newGateWorld(t)
		w.statuses[test.key] = 1
		if code := w.gate(); code != 1 || !strings.Contains(w.stderr.String(), test.stderr) {
			t.Fatalf("%s: exit %d stderr %q", test.key, code, w.stderr.String())
		}
		if len(w.nativeCalls) != 0 {
			t.Fatalf("%s: a red deterministic stage still paid the native selection", test.key)
		}
	}
}

func TestGateNativeFailureKeepsItsCompleteOutput(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["native"] = 1
	if code := w.gate(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	keep := filepath.Join(w.root, "artifacts", "agents", "gate-failures", "20260927T043000Z-"+strconv.Itoa(os.Getpid())+".log")
	if !strings.Contains(w.stderr.String(), "go gate: native Go tests failed (output kept: artifacts/agents/gate-failures/20260927T043000Z-"+strconv.Itoa(os.Getpid())+".log)") {
		t.Fatalf("stderr:\n%s", w.stderr.String())
	}
	// The complete diagnostic reaches the parent log before any move.
	if !strings.Contains(w.stderr.String(), "native red output\n") || !strings.Contains(w.stderr.String(), "proof-run go-gate-tests: native selection failed") {
		t.Fatalf("native diagnostic not printed:\n%s", w.stderr.String())
	}
	data, err := os.ReadFile(keep)
	if err != nil || !strings.Contains(string(data), "native red output") || !strings.Contains(string(data), "native selection failed") {
		t.Fatalf("kept log = %q, %v", data, err)
	}
	if _, err := os.Stat(strings.TrimSuffix(keep, ".log") + ".native/go-gate-native.log"); err != nil {
		t.Fatalf("native evidence not kept: %v", err)
	}
	if len(w.called("go list -p=3 ./internal/...")) != 0 {
		t.Fatalf("a red native selection reached the ratchet")
	}
}

func TestGateCoverageRatchetRefusalRetainsTheBundle(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.nativeOutput = "ok  \t" + fixtureModule + "/internal/fixture\t0.1s\tcoverage: 10.0% of statements\n"
	if code := w.gate(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	keep := filepath.Join(w.root, "artifacts", "agents", "gate-failures", "20260927T043000Z-"+strconv.Itoa(os.Getpid())+"-coverage")
	if !strings.Contains(w.stderr.String(), "coverage ratchet: package internal/fixture") ||
		!strings.Contains(w.stderr.String(), "go gate: coverage ratchet refused (evidence kept: "+keep+")") {
		t.Fatalf("stderr:\n%s", w.stderr.String())
	}
	for _, rel := range []string{"coverage.jsonl", "package-inventory.txt", "native/go-gate-native.log"} {
		if _, err := os.Stat(filepath.Join(keep, rel)); err != nil {
			t.Fatalf("retained bundle lacks %s: %v", rel, err)
		}
	}
}

func TestGateRetentionFollowsTheProofOwner(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	owner := filepath.Join(w.tmp, "execution-owner")
	w.setenv("METASYSTEM_PROOF_CONTROL_ROOT="+w.root, "METASYSTEM_PROOF_EXECUTION_ROOT="+owner)
	w.nativeOutput = ""
	if code := w.gate(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if kept, _ := filepath.Glob(filepath.Join(owner, "gate-failures", "*-coverage", "coverage.jsonl")); len(kept) != 1 {
		t.Fatalf("coverage evidence did not reach the execution owner:\n%s", w.stderr.String())
	}
}

func TestGateSeedAndMissingBaseline(t *testing.T) {
	t.Parallel()
	seed := newGateWorld(t)
	seed.setenv("METASYSTEM_COVERAGE_RATCHET_SEED=1")
	seed.nativeOutput = ""
	if code := seed.gate(); code != 0 || !strings.Contains(seed.stderr.String(), "go gate: coverage seed inputs retained; floors were not enforced (evidence kept: ") {
		t.Fatalf("seed: exit %d\n%s", code, seed.output())
	}

	missing := newGateWorld(t)
	if err := os.Remove(filepath.Join(missing.root, "scripts", "agents", "coverage-ratchet.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(missing.root, "scripts", "agents", "coverage-ratchet-linux.json")); err != nil {
		t.Fatal(err)
	}
	if code := missing.gate(); code != 1 || !strings.Contains(missing.stderr.String(), "run the two-pass seed bootstrap first (evidence kept: ") {
		t.Fatalf("missing baseline: exit %d\n%s", code, missing.output())
	}
}

func TestGateCoverageHandoffClaimsAndPublishesAsTheProducer(t *testing.T) {
	t.Parallel()
	attempt := func(w *gateWorld) {
		w.setenv("METASYSTEM_PROOF_ATTEMPT=attempt-7", "METASYSTEM_PROOF_CONTROL_ROOT="+w.root)
	}
	w := newGateWorld(t)
	attempt(w)
	if code := w.gate(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	pid := strconv.Itoa(os.Getpid())
	suffix := " producer=" + pid + " caller=" + pid + " attempt=attempt-7 baseline=coverage-ratchet.json root=" + w.root
	if want := []string{"coverage-eligible" + suffix, "coverage-begin" + suffix, "coverage-complete" + suffix}; !slices.Equal(w.coverage, want) {
		t.Fatalf("coverage custody = %q, want %q", w.coverage, want)
	}
	if !strings.Contains(w.stdout.String(), `"attemptId":"attempt-7"`) {
		t.Fatalf("published evidence not printed:\n%s", w.stdout.String())
	}

	foreign := newGateWorld(t)
	attempt(foreign)
	foreign.statuses["coverage-eligible"] = 3
	if code := foreign.gate(); code != 0 || len(foreign.coverage) != 1 {
		t.Fatalf("a foreign nested producer claimed coverage: exit %d %q", code, foreign.coverage)
	}

	refused := newGateWorld(t)
	attempt(refused)
	refused.statuses["coverage-eligible"] = 1
	if code := refused.gate(); code != 1 || !strings.Contains(refused.stderr.String(), "go gate: coverage producer eligibility could not be authenticated") {
		t.Fatalf("unauthenticated eligibility: exit %d\n%s", code, refused.output())
	}
	if len(refused.nativeCalls) != 0 {
		t.Fatalf("an unauthenticated producer measured coverage")
	}

	owned := newGateWorld(t)
	attempt(owned)
	owned.statuses["coverage-begin"] = 1
	if code := owned.gate(); code != 1 || !strings.Contains(owned.stderr.String(), "go gate: could not claim the authenticated coverage producer slot") {
		t.Fatalf("owned slot: exit %d\n%s", code, owned.output())
	}

	unpublished := newGateWorld(t)
	attempt(unpublished)
	unpublished.statuses["coverage-complete"] = 1
	if code := unpublished.gate(); code != 1 || !strings.Contains(unpublished.stderr.String(), "go gate: authenticated coverage publication refused (evidence kept: ") {
		t.Fatalf("refused publication: exit %d\n%s", code, unpublished.output())
	}
}

func TestGateProofLocatorWithoutControlRootRefuses(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.setenv("METASYSTEM_PROOF_ATTEMPT=attempt-1")
	if code := w.gate(); code != 1 || !strings.Contains(w.stderr.String(), "go gate: proof worker locator has no control root") {
		t.Fatalf("exit %d stderr %q", code, w.stderr.String())
	}
}

func TestMoveEvidenceCopiesAcrossFilesystemsAndRemovesTheSource(t *testing.T) {
	t.Parallel()
	from := filepath.Join(t.TempDir(), "native")
	if err := os.MkdirAll(filepath.Join(from, "counters"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from, "counters", "c"), []byte("counter\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	to := filepath.Join(t.TempDir(), "kept")
	if err := copyEvidence(from, to); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(to, "counters", "c")); err != nil || string(data) != "counter\n" {
		t.Fatalf("copied evidence = %q %v", data, err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	if err := moveEvidence(to, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Fatalf("moved source survived: %v", err)
	}
}
