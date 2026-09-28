package main

import (
	"os"
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
		{"copied engine mismatch", "change non-tailored engine bytes", "ENGINE equality refusal", "adopt", "copied target changed non-tailored %s bytes", 1},
		{"copied payload mismatch", "change non-tailored payload bytes", "PAYLOAD equality refusal", "adopt", `for _, projection := range []behaviorsurface.Projection{behaviorsurface.Engine, behaviorsurface.Payload}`, 1},
		{"copied Claude drift", "mutate copied Claude skill", "registration path named", "adopt", `range []string{".claude/skills/verify", ".agents/skills/verify"}`, 1},
		{"copied Codex drift", "mutate copied Codex skill", "registration path named", "adopt", `registration+": existing copied skill differs from its source at SKILL.md"`, 1},
		{"generation replacement", "arm an older live engine generation", "component=supervision-owner outcome=replaced", "supervision", "func TestLiveGenerationReplacementStopsAndReplacesTheRecordedOwner(", 1},
		{"unlanded rearm", "install an engine not on the landing ref", "not landed on refs/remotes/origin/trunk", "supervision", "func TestNotLandedRebuildNamesFetchAndTerminalRepairs(", 1},
	}
	sources := map[string]string{
		"cap-contract": fixtureSource(t, "cmd", "metasystem", "cap_contract_test.go") +
			fixtureSource(t, "cmd", "metasystem", "dispatch_verbs_test.go") +
			fixtureSource(t, "internal", "adapter", "adjudicate_test.go") +
			fixtureSource(t, "internal", "dispatch", "decisions_test.go") +
			fixtureSource(t, "internal", "dispatch", "critique_chain_test.go") +
			fixtureSource(t, "internal", "dispatch", "severity_tiered_rigor_test.go"),
		"adopt":       fixtureSource(t, "cmd", "metasystem", "adoption_comparison_test.go"),
		"supervision": fixtureSource(t, "internal", "supervise", "arming_test.go") + fixtureSource(t, "internal", "up", "up_test.go"),
	}
	for _, witness := range witnesses {
		t.Run(witness.assertion, func(t *testing.T) {
			if got := strings.Count(sources[witness.source], witness.needle); got != witness.count {
				t.Fatalf("injected fault %q must retain exact result %q in %d witness; got %d", witness.injectedFault, witness.exactResult, witness.count, got)
			}
		})
	}
}

// The dispatcher bed's optimizations retired with the bed (verbs-object-action
// U6b: its scenarios are Go tests); the adoption pruning keeps its witnesses.
func TestAdoptionOptimizationsKeepPrivateOwnersAndUniqueAssertions(t *testing.T) {
	adopt := fixtureSource(t, "cmd", "metasystem", "adoption_comparison_test.go")
	if strings.Contains(adopt, "validate-metasystem.sh") || strings.Contains(adopt, "validate-section-selector.sh") {
		t.Fatal("the adoption comparison still drives the retired validator")
	}
	for _, required := range []string{
		`run(filled, nil, engine, "system", "check", "--repo", filled, "--json")`,
		`fillAdoptionHarnessTestingContract(t, filepath.Join(source, "testing.json"), filepath.Join(target, "testing.json"))`,
		`TEST_CONTRACT_READY`,
		`for _, projection := range []behaviorsurface.Projection{behaviorsurface.Engine, behaviorsurface.Payload}`,
		`"internal", "runtime", "setup", "--repo", target, "--runtimes", runtimes}`,
		`range []string{".claude/skills/verify", ".agents/skills/verify"}`,
		`registration+": existing copied skill differs from its source at SKILL.md"`,
	} {
		if !strings.Contains(adopt, required) {
			t.Fatalf("adoption pruning lost prerequisite or retained witness: %q", required)
		}
	}
}
