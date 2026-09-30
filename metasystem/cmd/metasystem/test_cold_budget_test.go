package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestColdCandidateBuildRefusalRunsBeforeNativeBuilder(t *testing.T) {
	const role = "candidate-cold-refusal-assertions"
	if got := os.Getenv("GO_WANT_CANDIDATE_COLD_CHILD"); got != "" {
		if got != role {
			t.Fatalf("unexpected candidate cold child role %q", got)
		}
		t.Log("candidate cold child role: " + role)
		testColdCandidateBuildRefusalRunsBeforeNativeBuilder(t)
		return
	}
	t.Parallel()
	command := exec.Command(os.Args[0], "-test.run=^TestColdCandidateBuildRefusalRunsBeforeNativeBuilder$", "-test.v", "-test.count=1")
	command.Env = append(os.Environ(), "GO_WANT_CANDIDATE_COLD_CHILD="+role)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "candidate cold child role: "+role) ||
		!strings.Contains(string(output), "--- PASS: TestColdCandidateBuildRefusalRunsBeforeNativeBuilder") ||
		strings.Contains(string(output), "--- SKIP:") {
		t.Fatalf("isolated cold refusal assertions failed: %v\n%s", err, output)
	}
	t.Logf("isolated cold refusal assertions passed for %s:\n%s", role, output)
}

func testColdCandidateBuildRefusalRunsBeforeNativeBuilder(t *testing.T) {
	fixture := newOrdinaryCandidateFixture(t)
	controlRoot := t.TempDir()
	want := &coldBuildBudgetRefusal{detail: "fixture exhausted"}
	called := 0
	environment := testrun.Environment(os.Environ())
	// Outside a proof run's scratch nothing is prepared and the legacy
	// namespace gains nothing (engine-owns-disk-lifetimes 3.5, DL2-11).
	fixture.queueIdentity(ordinaryProjectTree, ordinaryEngineTree, ordinaryBuildFive, environment, false)
	if artifact, err := candidateengine.Prepare(context.Background(), controlRoot, fixture.workspace(), "metasystem", ordinaryProjectTree,
		environment, func() error { t.Fatal("no cold build outside a scratch run"); return nil }, fixture.dependency()); artifact != nil || err == nil ||
		!strings.Contains(err.Error(), "scratch") {
		t.Fatalf("preparation outside a scratch run is refused: artifact=%+v err=%v", artifact, err)
	}
	if _, err := os.Stat(filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines")); !os.IsNotExist(err) {
		t.Fatalf("a refused preparation writes nothing: %v", err)
	}
	fixture.queueIdentity(ordinaryProjectTree, ordinaryEngineTree, ordinaryBuildFive, environment, false)
	artifact, err := candidateengine.Prepare(candidateScratchContext(t, context.Background(), controlRoot), controlRoot,
		fixture.workspace(), "metasystem", ordinaryProjectTree,
		environment, func() error { called++; return want }, fixture.dependency())
	fixture.assertDrained()
	var refusal *coldBuildBudgetRefusal
	if artifact != nil || !errors.As(err, &refusal) || refusal != want || called != 1 {
		t.Fatalf("cold preflight did not refuse before build: artifact=%+v err=%v called=%d", artifact, err, called)
	}
	entries, err := os.ReadDir(filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines", "v2"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".lock") {
			t.Fatalf("refused cold preparation published candidate engine artifact %s", entry.Name())
		}
	}
}

func TestColdBudgetScreenPreservesPossibleReuseAndExtension(t *testing.T) {
	t.Parallel()
	groups := []string{"a", "b"}
	attempts := []proofrun.Attempt{{TestResult: &proofrun.TestResult{Groups: []proofrun.GroupResult{
		{ID: "a", Status: "passed", CollectionComplete: true},
		{ID: "b", Status: "reused", CollectionComplete: true},
	}}}}
	if !retainedSuccessCouldCoverSelection(attempts, groups, testrun.SelectionRequest{}) {
		t.Fatal("complete retained observations did not defer budget screen for receipt-only reuse")
	}
	attempts[0].TestResult.Groups[1].CollectionComplete = false
	if retainedSuccessCouldCoverSelection(attempts, groups, testrun.SelectionRequest{}) {
		t.Fatal("incomplete retained group was treated as possible complete reuse")
	}
	breach := dispatchcore.GoalRevisionAdmission{Refusal: &dispatchcore.GoalAdmissionRefusal{
		Breaches: []dispatchcore.BudgetBreach{{Field: "reservedJobMinutesLimit"}},
	}}
	if !permanentBudgetRefusal(breach) {
		t.Fatal("known minute exhaustion did not screen a cold build")
	}
	breach.Extension = &dispatchcore.BudgetExtensionOffer{}
	if permanentBudgetRefusal(breach) {
		t.Fatal("possible earned extension was refused before final admission")
	}
	breach.Extension = nil
	breach.Refusal.Breaches = []dispatchcore.BudgetBreach{{Field: "activeJobLimit"}}
	if permanentBudgetRefusal(breach) {
		t.Fatal("transient active capacity was treated as permanent exhaustion")
	}
}

// The cold-build budget refusal is two plain lines; its code reaches a
// parent through the test run's --json envelope, never through its words.
func TestColdBudgetRefusalCarriesItsCodeOutsideItsWords(t *testing.T) {
	t.Parallel()
	refusal := &coldBuildBudgetRefusal{goal: "g1", detail: "goal g1 before candidate-engine build: attemptLimit"}
	lines := strings.Split(refusal.Error(), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "BUDGET_REFUSED") || lines[1] != "run: metasystem goal budget g1" {
		t.Fatalf("refusal words = %q", refusal.Error())
	}
	envelope := verbresult.FromError("internal test run", 78, refusal, nil)
	if envelope.Code != "BUDGET_REFUSED" || envelope.Outcome != verbresult.Refused {
		t.Fatalf("envelope = %+v", envelope)
	}
}
