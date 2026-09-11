package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureSource(t *testing.T, relative ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, relative...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture owner %s: %v", path, err)
	}
	return string(data)
}

// This table is the pruning review map: every removed duplicate names the
// injected fault, exact retained result and number of live witnesses. A
// deletion is allowed only while its owner needle remains exactly once.
func TestFixturePruningRetainsEveryDistinctFaultWitness(t *testing.T) {
	type retainedWitness struct {
		assertion, injectedFault, exactResult, source, needle string
		count                                                 int
	}
	witnesses := []retainedWitness{
		{"critic runtime fault", "adapter exits seven", "finish failed protocol_error runtime", "cap-contract", "TestCapContractPublicAdapterAndRetiredVerb", 2},
		{"critic empty reply", "adapter returns no terminal payload", "finish failed protocol_error delivery", "cap-contract", "TestAdjudicateTurnPureVerdicts", 1},
		{"retired cap command", "invoke exhaustion-patches", "verb absent from the public router", "cap-contract", "TestCapContractPublicAdapterAndRetiredVerb", 2},
		{"bounded cap", "attempt round four after three bounded rounds", "cap-exhausted-human-raise at round 3", "cap-contract", "TestBoundaryAtThreeRounds", 1},
		{"before-limit cap", "attempt continuation after two completed rounds", "continuation remains admitted before the limit", "cap-contract", "TestCritiqueBeforeRoundLimitAllowsContinuation", 1},
		{"protocol cap", "one completed round and one real protocol-error round", "both rounds count while cancellation does not", "cap-contract", "TestCompletedAndFailedRoundsCountButCancelledDoesNot", 1},
		{"human adjustment", "raise the approved review-round member", "critique budget rebind raises the frozen limit", "cap-contract", "TestRaiseByRebind", 1},
		{"severe core cap", "severe finding at exhausted round", "cap-exhausted-human-raise at terminal round 3", "cap-contract", "TestCritiqueSevereTerminalBoundary", 1},
		{"severe cap", "severe finding at exhausted round", "exit 10 and terminal round 3", "cap-contract", "TestDispatchCritiqueAdvanceVerbsPath", 1},
		{"copied engine mismatch", "change non-tailored engine bytes", "ENGINE equality refusal", "adopt", "copied target changed non-tailored $projection bytes", 1},
		{"copied payload mismatch", "change non-tailored payload bytes", "PAYLOAD equality refusal", "adopt", "for projection in ENGINE PAYLOAD", 1},
		{"copied Claude drift", "mutate copied Claude skill", "registration path named", "adopt", "drifted claude skill copy refusal did not name its registration", 1},
		{"copied Codex drift", "mutate copied Codex skill", "registration path named", "adopt", "drifted codex skill copy refusal did not name its registration", 1},
		{"copied orphan", "remove copied skill source", "orphaned registration refusal", "adopt", "pruned-skill failure did not name the orphaned copy", 1},
		{"generation replacement", "arm an older live engine generation", "component=supervision-owner outcome=replaced", "supervision", "ordinary up did not replace the live older engine generation", 1},
		{"unlanded rearm", "install an engine not on the landing ref", "not landed on refs/remotes/origin/trunk", "supervision", "unlanded refusal did not name the remote-tracking ref", 1},
	}
	sources := map[string]string{
		"cap-contract": fixtureSource(t, "cmd", "metasystem", "cap_contract_test.go") +
			fixtureSource(t, "cmd", "metasystem", "dispatch_verbs_test.go") +
			fixtureSource(t, "internal", "adapter", "adjudicate_test.go") +
			fixtureSource(t, "internal", "dispatch", "decisions_test.go") +
			fixtureSource(t, "internal", "dispatch", "critique_chain_test.go") +
			fixtureSource(t, "internal", "dispatch", "severity_tiered_rigor_test.go"),
		"adopt":       fixtureSource(t, "scripts", "adopt-fixtures.sh") + fixtureSource(t, "scripts", "adopt-fixture-helpers.sh"),
		"supervision": fixtureSource(t, "scripts", "agents", "supervision-fixtures.sh"),
	}
	for _, witness := range witnesses {
		t.Run(witness.assertion, func(t *testing.T) {
			if got := strings.Count(sources[witness.source], witness.needle); got != witness.count {
				t.Fatalf("injected fault %q must retain exact result %q in %d witness; got %d", witness.injectedFault, witness.exactResult, witness.count, got)
			}
		})
	}
}

func TestDispatcherSharedEngineCapabilityRejectsTamperAndForeignBed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(root, "bin", "metasystem")
	if err := os.WriteFile(shim, []byte("#!/usr/bin/env bash\nset -euo pipefail\n[[ $1 == util && $2 == sha256 && $3 == --file ]]\nshasum -a 256 \"$4\" | awk '{print $1}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := t.TempDir()
	engine := filepath.Join(owner, "fixture-engine")
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=fixture-shared-build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build shared capability fixture engine: %v\n%s", err, output)
	}
	helper, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(script string) (string, error) {
		command := exec.Command("bash", "-c", "set -euo pipefail\nfixture_bed_root=$1\nsource $2\n"+script, "fixture-capability", root, helper, owner, engine)
		output, commandErr := command.CombinedOutput()
		return string(output), commandErr
	}
	out, err := run("cap=$(harness_dispatch_fixture_bed_mint_capability \"$3\" 1 dispatch \"$4\")\nharness_dispatch_fixture_bed_child_scenario dispatch --fixture-bed-child dispatch \"$cap\"\nprintf '%s|%s|%s\\n' \"$harness_fixture_child_scenario\" \"$harness_fixture_shared_engine\" \"$harness_fixture_shared_engine_digest\"")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(out), "dispatch|"+engine+"|") {
		t.Fatalf("valid private capability refused: err=%v output=%q", err, out)
	}
	out, err = run("cap=$(harness_dispatch_fixture_bed_mint_capability \"$3\" 2 dispatch \"$4\")\nprintf tamper >>\"$4\"\nharness_dispatch_fixture_bed_child_scenario dispatch --fixture-bed-child dispatch \"$cap\"")
	if err == nil || !strings.Contains(out, "shared immutable engine digest mismatch") {
		t.Fatalf("tampered engine did not refuse exactly: err=%v output=%q", err, out)
	}
	out, err = run("cap=$(harness_dispatch_fixture_bed_mint_capability \"$3\" 3 dispatch \"$4\")\nawk 'NR == 4 {$0=\"foreign-build\"} {print}' \"$cap\" >\"$cap.changed\"\nmv \"$cap.changed\" \"$cap\"\nharness_dispatch_fixture_bed_child_scenario dispatch --fixture-bed-child dispatch \"$cap\"")
	if err == nil || !strings.Contains(out, "shared immutable engine build descriptor mismatch") {
		t.Fatalf("tampered engine descriptor did not refuse exactly: err=%v output=%q", err, out)
	}
	foreign := t.TempDir()
	foreignEngine := filepath.Join(foreign, "fixture-engine")
	if err := os.WriteFile(foreignEngine, []byte("foreign engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digestCommand := exec.Command(shim, "util", "sha256", "--file", foreignEngine)
	digestBytes, err := digestCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	capability := filepath.Join(owner, "foreign.capability")
	if err := os.WriteFile(capability, []byte(fmt.Sprintf("dispatch\n%s\n%s\nforeign-build\n", foreignEngine, strings.TrimSpace(string(digestBytes)))), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err = run("harness_dispatch_fixture_bed_child_scenario dispatch --fixture-bed-child dispatch \"$3/foreign.capability\"")
	if err == nil || !strings.Contains(out, "shared immutable engine locator is foreign to the private parent") {
		t.Fatalf("foreign-bed engine did not refuse exactly: err=%v output=%q", err, out)
	}
	out, err = run("cap=$(harness_fixture_bed_mint_capability \"$3\" 4 unaffected)\nharness_fixture_bed_child_scenario unaffected --fixture-bed-child unaffected \"$cap\"")
	if err != nil || strings.TrimSpace(out) != "unaffected" {
		t.Fatalf("legacy private three-argument fixture capability changed shape: err=%v output=%q", err, out)
	}
}

func TestDispatcherAndAdoptionOptimizationsKeepPrivateOwnersAndUniqueAssertions(t *testing.T) {
	dispatch := fixtureSource(t, "scripts", "agents", "dispatch-fixtures.sh")
	for _, required := range []string{
		`dispatch_fixture_shared_engine=$1/fixture-engine`,
		`harness_dispatch_fixture_bed_mint_capability "$1" "$2" "$3" "$dispatch_fixture_shared_engine"`,
		`fixture_bed_mint_capability=dispatch_fixture_bed_mint_capability`,
		`if harness_dispatch_fixture_bed_child_scenario dispatch "$@"; then`,
		`fixture_scenario=$harness_fixture_child_scenario`,
		`export METASYSTEM_SUPERVISION_REGISTRY_HOME="$tmp/supervision-home"`,
		`armed_supervision_repos=()`,
		`if [[ "$fixture_scenario" == mission-runner ]]; then`,
		`if [[ "$fixture_scenario" == adapter-selftest ]]; then`,
		`bash scripts/agents/go-build.sh --out "$engine"`, // standalone fallback remains
		`(cd "$skew_repo" && bash scripts/agents/go-build.sh >/dev/null)`,
	} {
		if !strings.Contains(dispatch, required) {
			t.Fatalf("dispatcher optimization lost required private owner or unique assertion: %q", required)
		}
	}
	if strings.Contains(dispatch, "cap_fixture_round") || strings.Contains(dispatch, "cap_fixture=") {
		t.Fatal("repeated shell cap probes remain after the Go-owned cap-contract cutover")
	}
	for _, scenario := range []string{"dispatch", "mission-runner", "adapter-selftest", "steward-continuation", "brain-delegate-refuses", "brain-cancel-close-reap-refuse", "brain-breach-stop-exempt", "brain-absent-node-proceeds", "brain-fence-helper-fails", "seat-refused"} {
		if !strings.Contains(dispatch, scenario) {
			t.Fatalf("dispatcher parent dropped scenario %s", scenario)
		}
	}

	adopt := fixtureSource(t, "scripts", "adopt-fixtures.sh") + fixtureSource(t, "scripts", "adopt-fixture-helpers.sh")
	if strings.Contains(adopt, `"$tmp/adopt-copy/scripts/validate-metasystem.sh" --delivery-contract`) {
		t.Fatal("copied-registration duplicate full delivery validation remains")
	}
	for _, required := range []string{
		`"$tgt/scripts/validate-metasystem.sh" --delivery-contract`,
		`fill_harness_testing_contract "$srcrepo/testing.json" "$tgt/testing.json"`,
		`fill_harness_testing_contract "$srcrepo/testing.json" "$tmp/adopt-copy/testing.json"`,
		`TEST_CONTRACT_READY`,
		`for projection in ENGINE PAYLOAD`,
		`validate-section-selector.sh" run runtime-contract-audits`,
		`registered skill copy has drifted from its source: .claude/skills/verify vs skills/verify`,
		`registered skill copy has drifted from its source: .agents/skills/verify vs skills/verify`,
		`validation missed an orphaned copy of a pruned skill`,
	} {
		if !strings.Contains(adopt, required) {
			t.Fatalf("adoption pruning lost prerequisite or retained witness: %q", required)
		}
	}
}
