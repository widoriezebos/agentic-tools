package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type ownershipFixture struct {
	t            *testing.T
	root         string
	identity     ProofIdentity
	launcher     ProcessIdentity
	now          time.Time
	admissionDir string
}

func newOwnershipFixture(t *testing.T) ownershipFixture {
	t.Helper()
	root, _ := proofAttemptFixture(t, "testing")
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("dispatch.cap-max=120\nmetasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := BuildProofIdentity(root, conf, "full", "testing", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	return ownershipFixture{t: t, root: root, identity: identity, launcher: launcher, now: time.Now().UTC(),
		admissionDir: filepath.Join(root, "artifacts", "agents", "host-admission-fixture")}
}

func TestRunTestPlanRefusesWrongAdmittedContext(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	component := strings.Repeat("a", 64)
	attempt := f.reserveExecuted("goal-context", "plan-context", map[string]string{"native": component}, "", "", 0)
	for _, check := range []struct {
		name, tree, root string
	}{
		{"wrong-tree", strings.Repeat("c", 40), attempt.ExecutionRoot},
		{"wrong-root", attempt.CandidateTree, t.TempDir()},
	} {
		t.Run(check.name, func(t *testing.T) {
			_, _, err := RunTestPlan(context.Background(), TestRunRequest{ControlRoot: f.root, AttemptID: attempt.AttemptID,
				ProjectRoot: check.root, CandidateTree: check.tree, ComponentIdentities: map[string]string{"native": component}})
			if err == nil || !strings.Contains(err.Error(), "tree or root differs") {
				t.Fatalf("ordinary admitted worker accepted mismatched context: %v", err)
			}
		})
	}
}

func (f ownershipFixture) reserve(goal, plan string, groups map[string]string, episode, binding string, offset time.Duration) (Attempt, LaunchResult) {
	return f.reserveWithPolicy(goal, plan, groups, episode, binding, offset, false, "")
}

func (f ownershipFixture) reserveWithPolicy(goal, plan string, groups map[string]string, episode, binding string, offset time.Duration, forceGroups bool, retryDecisionPath string) (Attempt, LaunchResult) {
	f.t.Helper()
	identity := BindIdentityInputs(f.identity, append(append([]string(nil), f.identity.IdentityInputs...), "plan:"+plan))
	request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: goal, GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
		Identity: identity, Launcher: f.launcher, Now: f.now.Add(offset),
		ComponentIdentities: groups, SharedComponents: true, FreshnessEpisode: episode, FreshnessBinding: binding,
		ForceGroups: forceGroups, RetryDecisionPath: retryDecisionPath})
	request = WithTestHostAdmissionDirectory(request, f.admissionDir)
	guard, err := AcquireMutation(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	defer guard.Release()
	attempt, decision, err := ReserveLocked(request)
	if err != nil {
		f.t.Fatal(err)
	}
	return attempt, decision
}

func (f ownershipFixture) reserveExecuted(goal, plan string, groups map[string]string, episode, binding string, offset time.Duration) Attempt {
	f.t.Helper()
	attempt, decision := f.reserve(goal, plan, groups, episode, binding, offset)
	if decision.Disposition != DispositionExecuted {
		f.t.Fatalf("fixture reservation did not execute: %+v", decision)
	}
	return attempt
}

func (f ownershipFixture) finish(attempt Attempt, statuses map[string]string, at time.Time) {
	f.t.Helper()
	ids := make([]string, 0, len(attempt.TestInventory))
	for id := range attempt.TestInventory {
		ids = append(ids, id)
	}
	// Test result order follows the admitted plan in these fixtures.
	result := componentAttemptResult(attempt.AttemptID, ids[0], attempt.TestInventory[ids[0]], statuses[ids[0]])
	result.FreshnessEpisode, result.FreshnessBinding = attempt.FreshnessEpisode, attempt.FreshnessBinding
	result.SelectedGroups, result.RequiredGroups, result.Groups = nil, nil, nil
	result.LaunchCounts.Test = 0
	terminal := TerminalSuccess
	for _, id := range ids {
		status := statuses[id]
		if status == "" {
			status = "passed"
		}
		var part GroupResult
		if attempt.TestWaits[id] != "" {
			var err error
			part, err = WaitForTestProducer(context.Background(), f.root, attempt, id)
			if err != nil {
				f.t.Fatal(err)
			}
			result.LaunchCounts.ReusedTest++
		} else {
			part = componentAttemptResult(attempt.AttemptID, id, attempt.TestInventory[id], status).Groups[0]
			part.StartedAt = at.Add(-time.Second).Format(time.RFC3339Nano)
			part.EndedAt = at.Format(time.RFC3339Nano)
			result.LaunchCounts.Test++
		}
		result.SelectedGroups = append(result.SelectedGroups, id)
		result.RequiredGroups = append(result.RequiredGroups, id)
		result.Groups = append(result.Groups, part)
		if part.Status != "passed" && part.Status != "reused" {
			terminal = TerminalFailed
		}
	}
	result.RecomputeDelivery()
	code := 0
	if terminal != TerminalSuccess {
		code = 23
	}
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, terminal, code, "fixture", nil, &result, at); err != nil {
		f.t.Fatal(err)
	}
}

func TestOverlappingReservationsTerminateOnce(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	a, b, c := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	first, decision := f.reserve("goal-a", "a-b", map[string]string{"a": a, "b": b}, "", "", 0)
	if decision.Disposition != DispositionExecuted || len(first.TestOwned) != 2 || len(first.TestWaits) != 0 {
		t.Fatalf("first ownership: %+v %+v", first, decision)
	}
	second, decision := f.reserve("goal-b", "b-c", map[string]string{"b": b, "c": c}, "", "", time.Millisecond)
	if decision.Disposition != DispositionExecuted || second.TestWaits["b"] != first.AttemptID || second.TestOwned["c"] != c || second.TestOwned["b"] != "" || second.TestAdmission <= first.TestAdmission {
		t.Fatalf("second ownership: %+v %+v", second, decision)
	}
	guard, err := AcquireMutation(f.root)
	if err != nil {
		t.Fatal(err)
	}
	withdrawErr := WithdrawReservationLocked(f.root, first.AttemptID)
	guard.Release()
	if withdrawErr == nil {
		t.Fatal("withdrawal orphaned a consumer's earlier producer")
	}
	observationTemplate := componentAttemptResult("", "b", b, "passed")
	before, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if observed, found := newestReuseObservation(observationTemplate, before, "b", b, ""); !found || !observed.live || observed.attemptID != first.AttemptID {
		t.Fatalf("consumer plan masked the live producer: %+v found=%v", observed, found)
	}
	// A later consumer can wait on both earlier producers. Neither earlier
	// producer acquires a wait edge, so the graph terminates in admission order.
	third := f.reserveExecuted("goal-c", "a-c", map[string]string{"a": a, "c": c}, "", "", 2*time.Millisecond)
	if third.TestWaits["a"] != first.AttemptID || third.TestWaits["c"] != second.AttemptID || len(third.TestOwned) != 0 {
		t.Fatalf("third ownership: %+v", third)
	}
	f.finish(first, map[string]string{}, f.now.Add(time.Second))
	after, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if observed, found := newestReuseObservation(observationTemplate, after, "b", b, ""); !found || observed.live || !observed.passed || observed.attemptID != first.AttemptID {
		t.Fatalf("consumer plan masked the terminal producer: %+v found=%v", observed, found)
	}
	borrowed, err := WaitForTestProducer(context.Background(), f.root, second, "b")
	if err != nil || borrowed.Status != "reused" || borrowed.ReuseAttempt != first.AttemptID || borrowed.NativeLaunched {
		t.Fatalf("borrowed b: %+v %v", borrowed, err)
	}
	f.finish(second, map[string]string{}, f.now.Add(2*time.Second))
	borrowed, err = WaitForTestProducer(context.Background(), f.root, third, "c")
	if err != nil || borrowed.Status != "reused" || borrowed.ReuseAttempt != second.AttemptID {
		t.Fatalf("borrowed c: %+v %v", borrowed, err)
	}
}

func TestSharedFailureReachesAllWaitersWithoutRetry(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("d", 64)
	producer := f.reserveExecuted("goal-a", "producer", map[string]string{"failing": identity}, "", "", 0)
	first := f.reserveExecuted("goal-b", "first", map[string]string{"failing": identity}, "", "", time.Millisecond)
	second := f.reserveExecuted("goal-c", "second", map[string]string{"failing": identity}, "", "", 2*time.Millisecond)
	f.finish(producer, map[string]string{"failing": "failed"}, f.now.Add(time.Second))
	for _, consumer := range []Attempt{first, second} {
		group, err := WaitForTestProducer(context.Background(), f.root, consumer, "failing")
		if err != nil || group.Status != "failed" || group.NativeLaunched || group.ReuseAttempt != producer.AttemptID {
			t.Fatalf("shared failure: %+v %v", group, err)
		}
	}
	_, decision := f.reserve("goal-d", "later", map[string]string{"failing": identity}, "", "", 3*time.Millisecond)
	if decision.Disposition != DispositionRetryRequired || decision.PriorAttempt != producer.AttemptID {
		t.Fatalf("failure triggered a duplicate run: %+v", decision)
	}
}

func TestCancelledConsumerDetachesFromSharedProducer(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("e", 64)
	producer := f.reserveExecuted("goal-a", "producer", map[string]string{"test": identity}, "", "", 0)
	consumer := f.reserveExecuted("goal-b", "consumer", map[string]string{"test": identity}, "", "", time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WaitForTestProducer(ctx, f.root, consumer, "test"); err != context.Canceled {
		t.Fatalf("cancelled wait: %v", err)
	}
	live, err := ReadAttempt(f.root, producer.AttemptID)
	if err != nil || live.Terminal != nil || live.CancellationIntent != "" {
		t.Fatalf("consumer changed producer: %+v %v", live, err)
	}
	f.finish(producer, map[string]string{}, f.now.Add(time.Second))
}

func TestBlockedWithoutNativeProducerPreservesBothIdentitySchemas(t *testing.T) {
	t.Parallel()
	const id = "dependent"
	identity := strings.Repeat("b", 64)
	for _, schema := range []int{PreviousTestResultSchemaVersion, TestResultSchemaVersion} {
		attempt := Attempt{TestResult: &TestResult{SchemaVersion: schema, Groups: []GroupResult{{
			ID: id, ExecutionIdentity: identity, Status: "blocked",
		}}}}
		if !blockedWithoutNativeProducer(attempt, id, identity) {
			t.Fatalf("schema %d blocked dependent became a failed native producer", schema)
		}
		attempt.TestResult.Groups[0].NativeLaunched = true
		if blockedWithoutNativeProducer(attempt, id, identity) {
			t.Fatalf("schema %d omitted a true native failure", schema)
		}
	}
	legacy := Attempt{TestResult: &TestResult{SchemaVersion: LegacyTestResultSchemaVersion, Groups: []GroupResult{{
		ID: id, ExecutionIdentity: identity, Status: "blocked",
	}}}}
	if blockedWithoutNativeProducer(legacy, id, identity) {
		t.Fatal("schema-1 not-run evidence lost its conservative retry fence")
	}
}

func TestProducerWaitUsesSemanticDeadline(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		at   time.Duration
		pass bool
	}{{"before", -time.Nanosecond, true}, {"at", 0, false}, {"after", time.Nanosecond, false}} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newOwnershipFixture(t)
			component := strings.Repeat("f", 64)
			producer, _ := f.reserve("goal-producer", "producer", map[string]string{"check": component}, "", "", 0)
			consumer, _ := f.reserve("goal-consumer", "consumer", map[string]string{"check": component}, "", "", time.Millisecond)
			deadline := f.now.Add(time.Minute)
			now := f.now
			ready := make(chan struct{})
			once := false
			var clockMu sync.Mutex
			check := func() error {
				clockMu.Lock()
				defer clockMu.Unlock()
				if !once {
					once = true
					close(ready)
				}
				if !deadline.After(now) {
					return fmt.Errorf("semantic producer deadline reached")
				}
				return nil
			}
			type answer struct {
				group GroupResult
				err   error
			}
			finished := make(chan answer, 1)
			go func() {
				group, err := WaitForTestProducerWithWaitCheck(t.Context(), f.root, consumer, "check", check)
				finished <- answer{group: group, err: err}
			}()
			<-ready
			clockMu.Lock()
			now = deadline.Add(testCase.at)
			clockMu.Unlock()
			if testCase.pass {
				f.finish(producer, map[string]string{}, now)
			}
			result := <-finished
			if testCase.pass {
				if result.err != nil || result.group.Status != "reused" || result.group.ReuseAttempt != producer.AttemptID {
					t.Fatalf("producer result before deadline: %+v err=%v", result.group, result.err)
				}
			} else if result.err == nil || !strings.Contains(result.err.Error(), "semantic producer deadline reached") {
				t.Fatalf("producer wait at offset %s: %+v err=%v", testCase.at, result.group, result.err)
			}
		})
	}
}

func TestDistinctPrefixVariantsOwnDistinctAttempts(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	firstIdentity, secondIdentity := strings.Repeat("a", 64), strings.Repeat("b", 64)
	first := f.reserveExecuted("goal-a", "prefix-one", map[string]string{"group": firstIdentity}, "", "", 0)
	second := f.reserveExecuted("goal-b", "prefix-two", map[string]string{"group": secondIdentity}, "", "", time.Millisecond)
	if first.TestOwned["group"] != firstIdentity || second.TestOwned["group"] != secondIdentity || len(second.TestWaits) != 0 {
		t.Fatalf("different prefix identities shared one producer: first=%+v second=%+v", first, second)
	}
}

func TestCachedSourceIsPinnedWithTheMissingSet(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	x, y := strings.Repeat("a", 64), strings.Repeat("b", 64)
	producer := f.reserveExecuted("goal-a", "prior", map[string]string{"x": x}, "", "", 0)
	f.finish(producer, map[string]string{}, f.now.Add(time.Second))
	consumer, decision := f.reserve("goal-b", "expanded", map[string]string{"x": x, "y": y}, "", "", 2*time.Second)
	if decision.Disposition != DispositionExecuted || consumer.TestSources["x"] != producer.AttemptID || consumer.TestOwned["y"] != y || consumer.TestOwned["x"] != "" {
		t.Fatalf("cached source and missing work were not frozen together: %+v %+v", consumer, decision)
	}
}

func TestAdmissionSequenceSurvivesWithdrawnReservation(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	groups := map[string]string{"check": strings.Repeat("a", 64)}
	first, firstDecision := f.reserve("goal-a", "withdrawn", groups, "", "", 0)
	if firstDecision.Disposition != DispositionExecuted {
		t.Fatalf("first fixture reservation did not execute: %+v", firstDecision)
	}
	guard, err := AcquireMutation(f.root)
	if err != nil {
		t.Fatal(err)
	}
	err = WithdrawReservationLocked(f.root, first.AttemptID)
	guard.Release()
	if err != nil {
		t.Fatal(err)
	}
	second, secondDecision := f.reserve("goal-b", "after-withdrawal", groups, "", "", time.Millisecond)
	if secondDecision.Disposition != DispositionExecuted {
		t.Fatalf("second fixture reservation did not execute: %+v", secondDecision)
	}
	if second.TestAdmission != first.TestAdmission+1 {
		t.Fatalf("durable admission sequence reused %d after withdrawal; second=%d", first.TestAdmission, second.TestAdmission)
	}
}

func TestSameClockAdmissionOrdersGreenRedAndRepair(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("9", 64)
	groups := map[string]string{"check": identity}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, Groups: []testpolicy.Group{{
		ID: "check", Kind: "unit", Obligations: []string{"same-clock-order"},
	}}}
	finish := func(attempt Attempt, status, marker string) TestResult {
		t.Helper()
		result := componentAttemptResult(attempt.AttemptID, "check", identity, status)
		result.Groups[0].StartedAt = f.now.Format(time.RFC3339Nano)
		result.Groups[0].EndedAt = f.now.Format(time.RFC3339Nano)
		result.RecomputeDelivery()
		terminal, code := TerminalSuccess, 0
		if status != "passed" {
			terminal, code = TerminalFailed, 23
		}
		payload := json.RawMessage(fmt.Sprintf(`{"schemaVersion":2,"marker":%q}`, marker))
		if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, terminal, code, marker, payload, &result, f.now); err != nil {
			t.Fatal(err)
		}
		return result
	}
	read := func() []Attempt {
		t.Helper()
		attempts, err := ReadAttempts(f.root)
		if err != nil {
			t.Fatal(err)
		}
		return attempts
	}
	projection := func(template TestResult, attempts []Attempt) TestResult {
		template.AttemptID = ""
		template.Groups = nil
		template.semanticNow = f.now
		return ReusedTestResult(template, attempts, groups, contract)
	}

	green, decision := f.reserveWithPolicy("goal-order", "same", groups, "", "", 0, false, "")
	if decision.Disposition != DispositionExecuted || green.TestAdmission == 0 {
		t.Fatalf("green admission=%+v decision=%+v", green, decision)
	}
	greenResult := finish(green, "passed", "green")
	attempts := read()
	greenProjection := projection(greenResult, attempts)
	greenForecast := ForecastRetainedGroupObservation(greenResult, attempts, "check", identity)
	if !greenProjection.Delivery.Sufficient || greenForecast.Status != "reusable" || greenForecast.AttemptID != green.AttemptID {
		t.Fatalf("green reuse=%+v forecast=%+v", greenProjection, greenForecast)
	}
	if exact, ok := ExactReusableTestResult(greenResult, attempts, groups, "goal-order", 1); !ok || exact.AttemptID != green.AttemptID {
		t.Fatalf("green exact recovery=%+v ok=%t", exact, ok)
	}

	red, decision := f.reserveWithPolicy("goal-order", "same", groups, "", "", 0, true, "")
	if decision.Disposition != DispositionExecuted || red.TestAdmission != green.TestAdmission+1 {
		t.Fatalf("forced-red admission=%+v decision=%+v", red, decision)
	}
	redResult := finish(red, "failed", "red")
	attempts = read()
	redProjection := projection(redResult, attempts)
	redForecast := ForecastRetainedGroupObservation(redResult, attempts, "check", identity)
	if redProjection.Delivery.Sufficient || redProjection.Groups[0].NotRunReason != "newest-observation-failed" || redForecast.Status != "failed" || redForecast.AttemptID != red.AttemptID {
		t.Fatalf("red reuse=%+v forecast=%+v", redProjection, redForecast)
	}
	if exact, ok := ExactReusableTestResult(greenResult, attempts, groups, "goal-order", 1); ok {
		t.Fatalf("newer same-clock red allowed exact green recovery: %+v", exact)
	}
	if _, retryRequired := f.reserveWithPolicy("goal-order", "same", groups, "", "", 0, false, ""); retryRequired.Disposition != DispositionRetryRequired || retryRequired.PriorAttempt != red.AttemptID {
		t.Fatalf("same-clock red did not require its retry: %+v", retryRequired)
	}

	evidencePath := filepath.Join(f.root, "same-clock-red.log")
	if err := os.WriteFile(evidencePath, []byte("reviewed same-clock red\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(f.root, "same-clock-retry.json")
	encoded, _ := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: red.AttemptID,
		Cause: "deterministic group failure", EvidencePath: filepath.Base(evidencePath), Rationale: "the failing group was reviewed"})
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, decision := f.reserveWithPolicy("goal-order", "same", groups, "", "", 0, false, decisionPath)
	if decision.Disposition != DispositionExecuted || repaired.TestAdmission != red.TestAdmission+1 || repaired.PreviousAttempt != red.AttemptID {
		t.Fatalf("repair admission=%+v decision=%+v", repaired, decision)
	}
	repairedResult := finish(repaired, "passed", "repaired")
	attempts = read()
	repairedProjection := projection(repairedResult, attempts)
	repairedForecast := ForecastRetainedGroupObservation(repairedResult, attempts, "check", identity)
	if !repairedProjection.Delivery.Sufficient || repairedProjection.Groups[0].ReuseAttempt != repaired.AttemptID || repairedForecast.Status != "reusable" || repairedForecast.AttemptID != repaired.AttemptID {
		t.Fatalf("repaired reuse=%+v forecast=%+v", repairedProjection, repairedForecast)
	}
	if exact, ok := ExactReusableTestResult(repairedResult, attempts, groups, "goal-order", 1); !ok || exact.AttemptID != repaired.AttemptID {
		t.Fatalf("repaired exact recovery=%+v ok=%t", exact, ok)
	}
	if _, reusable := f.reserveWithPolicy("goal-order", "same", groups, "", "", 0, false, ""); reusable.Disposition != DispositionReusableSuccess {
		t.Fatalf("repaired pass was not admission-reusable: %+v", reusable)
	}
	for index := range attempts {
		attempts[index].ReservationOwner = &ReservationOwner{
			ControlRoot: f.root, RunID: "same-clock-governed", RunGeneration: 1, LaunchNonce: "same-clock",
			GoalRevision: attempts[index].GoalRevision, ObligationRevision: 1, AttemptOrdinal: uint64(index + 1),
			BudgetEpoch: attempts[index].BudgetEpoch, Deadline: attempts[index].Deadline,
		}
		if err := writeAttempt(attempts[index]); err != nil {
			t.Fatal(err)
		}
	}
	governedAttempt, governedResult, err := LatestGovernedTestResult(f.root, "same-clock-governed")
	if err != nil || governedAttempt.AttemptID != repaired.AttemptID || governedResult.AttemptID != repaired.AttemptID {
		t.Fatalf("governed same-clock recovery attempt=%+v result=%+v err=%v", governedAttempt, governedResult, err)
	}

	var greenAttempt, redAttempt, repairedAttempt Attempt
	for _, attempt := range attempts {
		switch attempt.AttemptID {
		case green.AttemptID:
			greenAttempt = attempt
		case red.AttemptID:
			redAttempt = attempt
		case repaired.AttemptID:
			repairedAttempt = attempt
		}
	}
	greenAttempt.TestAdmission, redAttempt.TestAdmission = 0, 0
	legacyProjection := projection(redResult, []Attempt{greenAttempt, redAttempt})
	if legacyProjection.Delivery.Sufficient || ForecastRetainedGroupObservation(redResult, []Attempt{greenAttempt, redAttempt}, "check", identity).Status != "failed" {
		t.Fatalf("ambiguous legacy same-time history became reusable: %+v", legacyProjection)
	}
	greenAttempt.TestAdmission, repairedAttempt.TestAdmission = green.TestAdmission, green.TestAdmission
	duplicate := []Attempt{greenAttempt, repairedAttempt}
	if duplicateProjection := projection(repairedResult, duplicate); duplicateProjection.Delivery.Sufficient {
		t.Fatalf("duplicate nonzero admission became reusable: %+v", duplicateProjection)
	}
	if exact, ok := ExactReusableTestResult(repairedResult, duplicate, groups, "goal-order", 1); ok {
		t.Fatalf("duplicate nonzero admission selected an exact result: %+v", exact)
	}
	greenAttempt.TestAdmission = repaired.TestAdmission
	if err := writeAttempt(greenAttempt); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LatestGovernedTestResult(f.root, "same-clock-governed"); err == nil || !strings.Contains(err.Error(), "ambiguous duplicate testing admission") {
		t.Fatalf("governed duplicate admission was not refused: %v", err)
	}
}

func TestMixedLegacyAndNumberedRecordsUseSymmetricObservationOrder(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("8", 64)
	groups := map[string]string{"check": identity}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, Groups: []testpolicy.Group{{
		ID: "check", Kind: "unit", Obligations: []string{"mixed-record-order"},
	}}}
	finish := func(attempt Attempt, status string) Attempt {
		t.Helper()
		result := componentAttemptResult(attempt.AttemptID, "check", identity, status)
		result.Groups[0].StartedAt = f.now.Format(time.RFC3339Nano)
		result.Groups[0].EndedAt = f.now.Format(time.RFC3339Nano)
		result.RecomputeDelivery()
		terminal, code := TerminalSuccess, 0
		if status != "passed" {
			terminal, code = TerminalFailed, 23
		}
		payload := json.RawMessage(fmt.Sprintf(`{"schemaVersion":2,"status":%q}`, status))
		if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, terminal, code, status, payload, &result, f.now); err != nil {
			t.Fatal(err)
		}
		stored, err := ReadAttempt(f.root, attempt.AttemptID)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	green, greenDecision := f.reserveWithPolicy("goal-mixed", "green", groups, "", "", 0, true, "")
	if greenDecision.Disposition != DispositionExecuted || green.TestAdmission == 0 {
		t.Fatalf("mixed green admission: attempt=%+v decision=%+v", green, greenDecision)
	}
	green = finish(green, "passed")
	red, redDecision := f.reserveWithPolicy("goal-mixed", "red", groups, "", "", 0, true, "")
	if redDecision.Disposition != DispositionExecuted || red.TestAdmission <= green.TestAdmission {
		t.Fatalf("mixed red admission: green=%+v red=%+v/%+v", green, red, redDecision)
	}
	red = finish(red, "failed")

	clone := func(source Attempt) Attempt {
		t.Helper()
		encoded, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		var copied Attempt
		if err := json.Unmarshal(encoded, &copied); err != nil {
			t.Fatal(err)
		}
		return copied
	}
	legacy := func(source Attempt, id string) Attempt {
		copied := clone(source)
		copied.AttemptID = id
		copied.SchemaVersion = CandidateAttemptSchemaVersion
		copied.TestAdmission = 0
		copied.PendingTestGroups = map[string]string{"check": identity}
		copied.TestInventory, copied.TestOwned, copied.TestWaits, copied.TestSources = nil, nil, nil, nil
		copied.TestFreshGroups = nil
		copied.FreshnessEpisode, copied.FreshnessBinding, copied.FreshnessExpiresAt = "", "", ""
		copied.TestResult.AttemptID = id
		copied.TestResult.SchemaVersion = LegacyTestResultSchemaVersion
		return copied
	}
	move := func(source Attempt, id string, at time.Time) Attempt {
		copied := clone(source)
		copied.AttemptID = id
		copied.StartedAt = at.Format(time.RFC3339Nano)
		copied.EndedAt = at.Format(time.RFC3339Nano)
		copied.Terminal.At = copied.EndedAt
		copied.TestResult.AttemptID = id
		copied.TestResult.StartedAt, copied.TestResult.EndedAt = copied.StartedAt, copied.EndedAt
		copied.TestResult.Groups[0].StartedAt, copied.TestResult.Groups[0].EndedAt = copied.StartedAt, copied.EndedAt
		return copied
	}
	serialized := func(records ...Attempt) map[string]Attempt {
		t.Helper()
		root := t.TempDir()
		for _, record := range records {
			record.ControlRoot, record.ExecutionRoot = root, root
			if err := writeAttempt(record); err != nil {
				t.Fatalf("write valid mixed record %s: %v", record.AttemptID, err)
			}
		}
		read, err := ReadAttempts(root)
		if err != nil || len(read) != len(records) {
			t.Fatalf("read valid mixed records: count=%d err=%v", len(read), err)
		}
		byID := make(map[string]Attempt, len(read))
		for _, record := range read {
			byID[record.AttemptID] = record
		}
		return byID
	}
	type expectedObservation struct {
		status, attemptID string
		exact             bool
		ambiguous         bool
	}
	assertConsumers := func(name string, retained []Attempt, successful Attempt, want expectedObservation) {
		t.Helper()
		owned, found := newestOwnedObservation(retained, "check", identity, "", "", "", f.now.Add(2*time.Second))
		if !found || owned.ambiguous != want.ambiguous ||
			want.status == "reusable" && (!owned.passed || owned.attempt.AttemptID != want.attemptID) ||
			want.status == "failed" && (!owned.failed || owned.live || want.attemptID != "" && owned.attempt.AttemptID != want.attemptID) ||
			want.status == "live" && (!owned.live || owned.attempt.AttemptID != want.attemptID) {
			t.Fatalf("%s ownership=%+v found=%t", name, owned, found)
		}
		template := *successful.TestResult
		template.AttemptID, template.Groups, template.semanticNow = "", nil, f.now.Add(2*time.Second)
		projection := ReusedTestResult(template, retained, groups, contract)
		forecast := ForecastRetainedGroupObservation(*successful.TestResult, retained, "check", identity)
		exact, exactOK := ExactReusableTestResult(*successful.TestResult, retained, groups, successful.AccountedGoal(), successful.AccountedRevision())
		if (want.status == "reusable") != projection.Delivery.Sufficient || forecast.Status != want.status ||
			forecast.AttemptID != want.attemptID || exactOK != want.exact || want.exact && exact.AttemptID != want.attemptID {
			t.Fatalf("%s projection=%+v forecast=%+v exact=%+v/%t", name, projection, forecast, exact, exactOK)
		}
	}
	assertPermutations := func(name string, first, second, successful Attempt, want expectedObservation) {
		t.Helper()
		stored := serialized(first, second)
		assertConsumers(name+" legacy-first", []Attempt{stored[first.AttemptID], stored[second.AttemptID]}, stored[successful.AttemptID], want)
		assertConsumers(name+" numbered-first", []Attempt{stored[second.AttemptID], stored[first.AttemptID]}, stored[successful.AttemptID], want)
	}

	legacyRed := legacy(red, "legacy-red")
	currentGreen := clone(green)
	currentGreen.AttemptID, currentGreen.TestResult.AttemptID = "current-green", "current-green"
	assertPermutations("equal legacy red and current green", legacyRed, currentGreen, currentGreen,
		expectedObservation{status: "failed", ambiguous: true})

	legacyGreen := legacy(green, "legacy-green")
	currentRed := clone(red)
	currentRed.AttemptID, currentRed.TestResult.AttemptID = "current-red", "current-red"
	assertPermutations("equal current red and legacy green", legacyGreen, currentRed, legacyGreen,
		expectedObservation{status: "failed", ambiguous: true})

	laterLegacyRed := legacy(move(red, "later-red-source", f.now.Add(time.Second)), "later-legacy-red")
	assertPermutations("later legacy red", laterLegacyRed, currentGreen, currentGreen,
		expectedObservation{status: "failed", attemptID: laterLegacyRed.AttemptID})

	liveLegacy := legacyRed
	liveLegacy.AttemptID = "live-legacy"
	liveLegacy.StartedAt = f.now.Add(time.Second).Format(time.RFC3339Nano)
	liveLegacy.Terminal, liveLegacy.TestResult = nil, nil
	liveLegacy.EndedAt, liveLegacy.ObservedMinutes = "", 0
	liveLegacy.DeliveryReceipt, liveLegacy.DeliveryReceiptBytes = nil, nil
	assertPermutations("live legacy", liveLegacy, currentGreen, currentGreen,
		expectedObservation{status: "live", attemptID: liveLegacy.AttemptID})

	repair := move(green, "numbered-repair", f.now.Add(time.Second))
	repair.TestAdmission = red.TestAdmission + 1
	assertPermutations("later numbered repair", legacyRed, repair, repair,
		expectedObservation{status: "reusable", attemptID: repair.AttemptID, exact: true})
}

func TestForceGroupsRecoversTiedTerminalLegacyProducers(t *testing.T) {
	t.Parallel()
	identity := strings.Repeat("8", 64)
	groups := map[string]string{"check": identity}
	for _, testCase := range []struct {
		name, lowStatus, highStatus string
	}{
		{name: "green-before-red", lowStatus: "passed", highStatus: "failed"},
		{name: "red-before-green", lowStatus: "failed", highStatus: "passed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newOwnershipFixture(t)
			if err := os.MkdirAll(attemptsDir(f.root), 0o700); err != nil {
				t.Fatal(err)
			}
			legacyRecord := func(id, status string) Attempt {
				result := componentAttemptResult(id, "check", identity, status)
				result.SchemaVersion = LegacyTestResultSchemaVersion
				result.StartedAt, result.EndedAt = f.now.Format(time.RFC3339Nano), f.now.Format(time.RFC3339Nano)
				result.Groups[0].StartedAt, result.Groups[0].EndedAt = result.StartedAt, result.EndedAt
				terminal, code := TerminalSuccess, 0
				if status == "failed" {
					terminal, code = TerminalFailed, 23
				}
				return Attempt{SchemaVersion: CandidateAttemptSchemaVersion, AttemptID: id,
					GoalID: "legacy-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
					CandidateGoalID: "legacy-goal", CandidateRevision: 1, CandidateTree: strings.Repeat("b", 40),
					StartedAt: f.now.Format(time.RFC3339Nano), Deadline: f.now.Add(2 * time.Minute).Format(time.RFC3339Nano),
					EndedAt: f.now.Format(time.RFC3339Nano), ObservedMinutes: 1,
					Terminal:      &AttemptTerminal{Result: terminal, ExitStatus: code, At: f.now.Format(time.RFC3339Nano)},
					ProofIdentity: f.identity, ControlRoot: f.root, ExecutionRoot: f.root, Launcher: f.launcher,
					PendingTestGroups: map[string]string{"check": identity}, TestResult: &result}
			}
			for _, record := range []Attempt{
				legacyRecord("legacy-a", testCase.lowStatus), legacyRecord("legacy-z", testCase.highStatus),
			} {
				if err := writeAttempt(record); err != nil {
					t.Fatalf("write schema-valid legacy producer %s: %v", record.AttemptID, err)
				}
			}
			retained, err := ReadAttempts(f.root)
			if err != nil || len(retained) != 2 || retained[0].AttemptID != "legacy-a" || retained[1].AttemptID != "legacy-z" {
				t.Fatalf("read legacy producer permutation: attempts=%+v err=%v", retained, err)
			}
			request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
				GoalID: "fresh-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
				Identity: BindIdentityInputs(f.identity, append(append([]string(nil), f.identity.IdentityInputs...), "plan:fresh")),
				Launcher: f.launcher, Now: f.now.Add(time.Second), ComponentIdentities: groups, SharedComponents: true})
			if _, decided, err := NoChildDecisionLocked(request); err == nil || decided || !strings.Contains(err.Error(), "ambiguous admission order") {
				t.Fatalf("ordinary admission accepted tied legacy history: decided=%t err=%v", decided, err)
			}
			request.ForceGroups = true
			fixtureHostAdmissionMu.Lock()
			fresh, decision, err := ReserveLocked(request)
			fixtureHostAdmissionMu.Unlock()
			if err != nil || decision.Disposition != DispositionExecuted || fresh.TestOwned["check"] != identity ||
				fresh.TestAdmission == 0 || fresh.TestSources["check"] != "" || fresh.TestWaits["check"] != "" {
				t.Fatalf("forced fresh ownership: attempt=%+v decision=%+v err=%v", fresh, decision, err)
			}
			retained, err = ReadAttempts(f.root)
			if err != nil || len(retained) != 3 || retained[2].AttemptID != fresh.AttemptID || retained[2].TestAdmission != fresh.TestAdmission {
				t.Fatalf("forced ownership was not durable exactly once: attempts=%+v err=%v", retained, err)
			}
			sequence, err := os.ReadFile(filepath.Join(attemptsDir(f.root), "test-admission-sequence"))
			if err != nil || strings.TrimSpace(string(sequence)) != strconv.FormatUint(fresh.TestAdmission, 10) {
				t.Fatalf("durable admission sequence=%q admission=%d err=%v", sequence, fresh.TestAdmission, err)
			}
			f.finish(fresh, map[string]string{"check": "passed"}, f.now.Add(2*time.Second))
			request.Now, request.ForceGroups = f.now.Add(3*time.Second), false
			if decision, decided, err := NoChildDecisionLocked(request); err != nil || !decided || decision.Disposition != DispositionReusableSuccess {
				t.Fatalf("later numbered success was not reusable: decision=%+v decided=%t err=%v", decision, decided, err)
			}
		})
	}

	t.Run("ambiguous-live-history-stays-closed", func(t *testing.T) {
		f := newOwnershipFixture(t)
		if err := os.MkdirAll(attemptsDir(f.root), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"legacy-live-a", "legacy-live-z"} {
			record := Attempt{SchemaVersion: CandidateAttemptSchemaVersion, AttemptID: id,
				GoalID: "legacy-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
				CandidateGoalID: "legacy-goal", CandidateRevision: 1, CandidateTree: strings.Repeat("b", 40),
				StartedAt: f.now.Format(time.RFC3339Nano), Deadline: f.now.Add(2 * time.Minute).Format(time.RFC3339Nano),
				ProofIdentity: f.identity, ControlRoot: f.root, ExecutionRoot: f.root, Launcher: f.launcher,
				PendingTestGroups: map[string]string{"check": identity}}
			if err := writeAttempt(record); err != nil {
				t.Fatal(err)
			}
		}
		request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
			GoalID: "fresh-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
			Identity: f.identity, Launcher: f.launcher, Now: f.now.Add(time.Second),
			ComponentIdentities: groups, SharedComponents: true, ForceGroups: true})
		if _, decided, err := NoChildDecisionLocked(request); err == nil || decided || !strings.Contains(err.Error(), "ambiguous admission order") {
			t.Fatalf("forced execution accepted ambiguous live history: decided=%t err=%v", decided, err)
		}
	})

	t.Run("one-red-still-requires-its-retry-decision", func(t *testing.T) {
		f := newOwnershipFixture(t)
		result := componentAttemptResult("legacy-red", "check", identity, "failed")
		result.SchemaVersion = LegacyTestResultSchemaVersion
		result.StartedAt, result.EndedAt = f.now.Format(time.RFC3339Nano), f.now.Format(time.RFC3339Nano)
		result.Groups[0].StartedAt, result.Groups[0].EndedAt = result.StartedAt, result.EndedAt
		record := Attempt{SchemaVersion: CandidateAttemptSchemaVersion, AttemptID: "legacy-red",
			GoalID: "legacy-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
			CandidateGoalID: "legacy-goal", CandidateRevision: 1, CandidateTree: strings.Repeat("b", 40),
			StartedAt: f.now.Format(time.RFC3339Nano), Deadline: f.now.Add(2 * time.Minute).Format(time.RFC3339Nano),
			EndedAt: f.now.Format(time.RFC3339Nano), ObservedMinutes: 1,
			Terminal:      &AttemptTerminal{Result: TerminalFailed, ExitStatus: 23, At: f.now.Format(time.RFC3339Nano)},
			ProofIdentity: f.identity, ControlRoot: f.root, ExecutionRoot: f.root, Launcher: f.launcher,
			PendingTestGroups: map[string]string{"check": identity}, TestResult: &result}
		if err := writeAttempt(record); err != nil {
			t.Fatal(err)
		}
		request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
			GoalID: "fresh-goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
			Identity: f.identity, Launcher: f.launcher, Now: f.now.Add(time.Second),
			ComponentIdentities: groups, SharedComponents: true, ForceGroups: true})
		decision, decided, err := NoChildDecisionLocked(request)
		if err != nil || !decided || decision.Disposition != DispositionRetryRequired || decision.PriorAttempt != record.AttemptID {
			t.Fatalf("forced groups bypassed one red producer: decision=%+v decided=%t err=%v", decision, decided, err)
		}
	})
}

func TestFreshEpisodeResumesButRenewsOnBinding(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("f", 64)
	groups := map[string]string{"test": identity}
	ordinary := f.reserveExecuted("goal-a", "ordinary", groups, "", "", 0)
	f.finish(ordinary, map[string]string{}, f.now.Add(time.Second))
	episode, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
	fresh := f.reserveExecuted("goal-b", "fresh", groups, episode, binding, 2*time.Second)
	if fresh.TestOwned["test"] != identity {
		t.Fatalf("cached ordinary pass satisfied fresh episode: %+v", fresh)
	}
	f.finish(fresh, map[string]string{}, f.now.Add(3*time.Second))
	resume, decision := f.reserve("goal-c", "resume", groups, episode, binding, 4*time.Second)
	if decision.Disposition != DispositionReusableSuccess || resume.AttemptID != "" {
		t.Fatalf("same episode did not reuse terminal native proof: %+v %+v", resume, decision)
	}
	changed := f.reserveExecuted("goal-d", "changed-base", groups, episode, strings.Repeat("3", 64), 5*time.Second)
	if changed.TestOwned["test"] != identity {
		t.Fatalf("changed binding reused old observation: %+v", changed)
	}
	newEpisode := f.reserveExecuted("goal-e", "new-decision", groups, strings.Repeat("4", 64), binding, 6*time.Second)
	if newEpisode.TestOwned["test"] != identity {
		t.Fatalf("new episode reused old observation: %+v", newEpisode)
	}
}

func TestExpiredFreshEpisodeHasNoReusableObservation(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity, episode, binding := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	attempt := f.reserveExecuted("goal-a", "mutable", map[string]string{"mutable": identity}, episode, binding, 0)
	f.finish(attempt, map[string]string{}, f.now.Add(time.Second))
	retained, err := ReadAttempt(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	expires := f.now.Add(time.Hour)
	retained.FreshnessExpiresAt = expires.Format(time.RFC3339Nano)
	retained.TestResult.FreshnessExpiresAt = retained.FreshnessExpiresAt
	if err := writeAttempt(retained); err != nil {
		t.Fatal(err)
	}
	attempts, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	template := componentAttemptResult("", "mutable", identity, "passed")
	template.FreshnessEpisode, template.FreshnessBinding = episode, binding
	template.FreshnessExpiresAt = retained.FreshnessExpiresAt
	for _, testCase := range []struct {
		name  string
		now   time.Time
		found bool
	}{
		{"before expiry", expires.Add(-time.Nanosecond), true},
		{"at expiry", expires, false},
		{"after expiry", expires.Add(time.Nanosecond), false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			template.semanticNow = testCase.now
			observed, found := newestReuseObservation(template, attempts, "mutable", identity, "")
			if found != testCase.found {
				t.Fatalf("reuse at %s: found=%v observation=%+v", testCase.now, found, observed)
			}
			observedOwner, ownerFound := newestOwnedObservation(attempts, "mutable", identity, episode, binding, retained.FreshnessExpiresAt, testCase.now)
			if ownerFound != testCase.found {
				t.Fatalf("ownership at %s: found=%v observation=%+v", testCase.now, ownerFound, observedOwner)
			}
		})
	}
	consumer := retained
	consumer.AttemptID = strings.Repeat("d", 64)
	group := retained.TestResult.Groups[0]
	group.NativeLaunched = false
	group.Status = "reused"
	group.ReuseAttempt = retained.AttemptID
	if _, err := nativeSourceGroup(consumer, group, expires.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("native source expired early: %v", err)
	}
	if _, err := nativeSourceGroup(consumer, group, expires); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("native source survived its exact expiry: %v", err)
	}
}

func TestFailedTerminalKeepsNativeEvidenceWhenAdmittedReuseDisappears(t *testing.T) {
	t.Parallel()
	fixture := func(t *testing.T) (ownershipFixture, Attempt, TestResult) {
		t.Helper()
		f := newOwnershipFixture(t)
		earlierIdentity, nativeIdentity := strings.Repeat("a", 64), strings.Repeat("b", 64)
		producer := f.reserveExecuted("goal-source", "source", map[string]string{"earlier": earlierIdentity}, "", "", 0)
		f.finish(producer, map[string]string{}, f.now.Add(time.Second))
		consumer := f.reserveExecuted("goal-consumer", "consumer", map[string]string{
			"earlier": earlierIdentity, "native": nativeIdentity,
		}, "", "", 2*time.Second)
		if consumer.TestSources["earlier"] != producer.AttemptID || consumer.TestOwned["native"] != nativeIdentity {
			t.Fatalf("fixture did not admit reuse then native work: %+v", consumer)
		}
		earlier, err := nativeSourceGroup(consumer, GroupResult{ID: "earlier", ExecutionIdentity: earlierIdentity,
			ReuseAttempt: producer.AttemptID}, f.now.Add(3*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		earlier.Status, earlier.NativeLaunched, earlier.ReuseAttempt = "reused", false, producer.AttemptID
		native := componentAttemptResult(consumer.AttemptID, "native", nativeIdentity, "passed").Groups[0]
		result := componentAttemptResult(consumer.AttemptID, "earlier", earlierIdentity, "passed")
		result.SelectedGroups, result.RequiredGroups = []string{"earlier", "native"}, []string{"earlier", "native"}
		result.Groups = []GroupResult{earlier, native}
		result.LaunchCounts = LaunchCounts{Test: 1, ReusedTest: 1, CountsComplete: true}
		result.Cost.ReusedLaunches = 1
		result.Uncertainty = []string{"later retained source failed after native work"}
		result.RecomputeDelivery()
		path, err := AttemptPath(f.root, producer.AttemptID)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return f, consumer, result
	}

	t.Run("insufficient failure normalizes only inherited evidence", func(t *testing.T) {
		f, consumer, result := fixture(t)
		retained, err := FinalizeAttemptWithTestResultLocked(f.root, consumer.AttemptID, TerminalFailed, 1, "fixture", nil,
			&result, f.now.Add(4*time.Second))
		if err != nil || retained.TestResult == nil || retained.TestResult.Delivery.Sufficient || len(retained.TestResult.Groups) != 2 ||
			retained.TestResult.Groups[0].Status != "not-run" || retained.TestResult.Groups[0].ReuseAttempt != "" ||
			retained.TestResult.Groups[1].Status != "passed" || !retained.TestResult.Groups[1].NativeLaunched ||
			retained.TestResult.Groups[1].ExecutionIdentity != consumer.TestOwned["native"] || retained.TestResult.LaunchCounts.Test != 1 ||
			retained.TestResult.LaunchCounts.ReusedTest != 0 || len(retained.TestResult.Uncertainty) != 2 {
			t.Fatalf("failed diagnostic retention changed native evidence or kept stale reuse: attempt=%+v err=%v", retained, err)
		}
	})

	for _, testCase := range []struct {
		name, want string
		terminal   string
		mutate     func(*TestResult)
	}{
		{name: "stale claimed success", terminal: TerminalSuccess, want: "has no complete native source", mutate: func(result *TestResult) {
			result.Uncertainty = nil
			result.RecomputeDelivery()
		}},
		{name: "stale sufficient failure", terminal: TerminalFailed, want: "has no complete native source", mutate: func(result *TestResult) {
			result.Uncertainty = nil
			result.RecomputeDelivery()
		}},
		{name: "forged admitted source", terminal: TerminalFailed, want: "changes its admitted source", mutate: func(result *TestResult) {
			result.Groups[0].ReuseAttempt = "forged-source"
		}},
		{name: "forged execution identity", terminal: TerminalFailed, want: "differs from admitted identity", mutate: func(result *TestResult) {
			result.Groups[0].ExecutionIdentity = strings.Repeat("f", 64)
		}},
		{name: "native non-owned evidence", terminal: TerminalFailed, want: "unowned group earlier cannot carry native evidence", mutate: func(result *TestResult) {
			result.Groups[0].Status, result.Groups[0].NativeLaunched, result.Groups[0].ReuseAttempt = "passed", true, ""
			result.LaunchCounts.Test, result.LaunchCounts.ReusedTest = 2, 0
			result.RecomputeDelivery()
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f, consumer, result := fixture(t)
			testCase.mutate(&result)
			if _, err := FinalizeAttemptWithTestResultLocked(f.root, consumer.AttemptID, testCase.terminal, 1, "fixture", nil,
				&result, f.now.Add(4*time.Second)); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("unsafe admitted result was not refused by %q: %v", testCase.want, err)
			}
			stored, err := ReadAttempt(f.root, consumer.AttemptID)
			if err != nil || stored.Terminal != nil || stored.TestResult != nil {
				t.Fatalf("refused result changed its attempt: attempt=%+v err=%v", stored, err)
			}
		})
	}
}

func TestSharedNativeProducerLaunchesOnce(t *testing.T) {
	f := newOwnershipFixture(t)
	counter, release := filepath.Join(t.TempDir(), "native-count"), filepath.Join(t.TempDir(), "release")
	t.Cleanup(func() { _ = os.WriteFile(release, []byte("go"), 0o600) })
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nprintf x >> %s\nwhile [ ! -f %s ]; do sleep .02; done\nmkdir -p reports\nprintf '<testsuite><testcase classname=\"fixture\" name=\"native\"/></testsuite>\\n' > reports/tests.xml\n", strconv.Quote(counter), strconv.Quote(release))
	path := filepath.Join(f.root, "scripts", "native.sh")
	if err := testexec.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runTestResultGit(t, f.root, "init", "-q", "-b", "main")
	runTestResultGit(t, f.root, "config", "user.name", "fixture")
	runTestResultGit(t, f.root, "config", "user.email", "fixture@example.invalid")
	runTestResultGit(t, f.root, "add", ".")
	runTestResultGit(t, f.root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, f.root, "rev-parse", "HEAD^{tree}")
	f.identity = BindIdentityInputs(f.identity, []string{"candidate-tree:" + tree})
	group := testpolicy.Group{ID: "native", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"scripts/native.sh"},
		Outputs: []string{"reports"}, Obligations: []string{"native"}, Platforms: []string{"any"}, TargetMS: 10000,
		Argv: []string{"bash", "scripts/native.sh"}, Reports: []string{"reports"}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/tests.xml", Classname: "fixture", Name: "native"}}}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic, RequestedMode: testpolicy.ModeCanary,
		RequiredMode: testpolicy.ModeCanary, ExecutedMode: testpolicy.ModeCanary,
		RequiredGroups: []string{"native"}, SelectedGroups: []string{"native"},
		Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"native"}}}}
	request := TestRunRequest{ProjectRoot: f.root, ControlRoot: f.root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, CandidateEngineDigest: strings.Repeat("a", 64),
		CandidateEngineBuildIdentity: strings.Repeat("c", 40), ComponentIdentities: map[string]string{}}
	identities, prepared, _, err := PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ComponentIdentities, request.PreparedGroups = identities, prepared
	producer, decision := f.reserve("goal-a", "producer", identities, "", "", 0)
	if decision.Disposition != DispositionExecuted {
		t.Fatalf("producer admission: %+v", decision)
	}
	consumer, decision := f.reserve("goal-b", "consumer", identities, "", "", time.Millisecond)
	if decision.Disposition != DispositionExecuted || consumer.TestWaits["native"] != producer.AttemptID {
		t.Fatalf("consumer admission: %+v %+v", consumer, decision)
	}
	type outcome struct {
		result TestResult
		status int
		err    error
	}
	producerDone, consumerDone := make(chan outcome, 1), make(chan outcome, 1)
	producerRequest := request
	producerRequest.AttemptID, producerRequest.LogRoot = producer.AttemptID, filepath.Join(f.root, "artifacts", producer.AttemptID)
	consumerRequest := request
	consumerRequest.AttemptID, consumerRequest.LogRoot = consumer.AttemptID, filepath.Join(f.root, "artifacts", consumer.AttemptID)
	go func() {
		result, status, err := RunTestPlan(context.Background(), producerRequest)
		producerDone <- outcome{result, status, err}
	}()
	go func() {
		result, status, err := RunTestPlan(context.Background(), consumerRequest)
		consumerDone <- outcome{result, status, err}
	}()
	deadline := time.After(10 * time.Second)
	for {
		if data, err := os.ReadFile(counter); err == nil && len(data) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("native producer never launched")
		case <-time.After(20 * time.Millisecond):
		}
	}
	select {
	case got := <-consumerDone:
		t.Fatalf("consumer finished before producer terminal: %+v", got)
	default:
	}
	if err := os.WriteFile(release, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	var first outcome
	select {
	case first = <-producerDone:
	case <-time.After(20 * time.Second):
		t.Fatal("native producer did not finish")
	}
	if first.err != nil || first.status != 0 || first.result.LaunchCounts.Test != 1 {
		t.Fatalf("producer result: %+v", first)
	}
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, producer.AttemptID, TerminalSuccess, 0, "native complete", nil, &first.result, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var second outcome
	select {
	case second = <-consumerDone:
	case <-time.After(20 * time.Second):
		t.Fatal("consumer did not observe terminal producer")
	}
	if second.err != nil || second.status != 0 || second.result.LaunchCounts.Test != 0 || second.result.LaunchCounts.ReusedTest != 1 || second.result.Groups[0].ReuseAttempt != producer.AttemptID {
		t.Fatalf("consumer result: %+v", second)
	}
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, consumer.AttemptID, TerminalSuccess, 0, "borrowed complete", nil, &second.result, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(counter)
	if err != nil || string(data) != "x" {
		t.Fatalf("native launches %q err=%v", data, err)
	}
	// A cached green from the ordinary selection cannot cover a fresh
	// diagnosis. The retained native observation can cover a resumed decision
	// with the same episode and binding; another decision launches again.
	episode, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
	freshTemplate := request
	freshTemplate.FreshnessEpisode, freshTemplate.FreshnessBinding = episode, binding
	attempts, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if cached := ReusedTestResult(NewTestResult(freshTemplate), attempts, identities, contract); cached.Delivery.Sufficient {
		t.Fatalf("ordinary cached pass satisfied a fresh diagnosis: %+v", cached.Groups)
	}
	fresh, decision := f.reserve("goal-c", "fresh-base", identities, episode, binding, 2*time.Millisecond)
	if decision.Disposition != DispositionExecuted || fresh.TestOwned["native"] != identities["native"] {
		t.Fatalf("fresh base did not reserve native execution: %+v %+v", fresh, decision)
	}
	freshTemplate.AttemptID, freshTemplate.LogRoot = fresh.AttemptID, filepath.Join(f.root, "artifacts", fresh.AttemptID)
	freshResult, status, err := RunTestPlan(context.Background(), freshTemplate)
	if err != nil || status != 0 || freshResult.LaunchCounts.Test != 1 {
		t.Fatalf("fresh native result: status=%d err=%v %+v", status, err, freshResult)
	}
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, fresh.AttemptID, TerminalSuccess, 0, "fresh native complete", nil, &freshResult, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	resume, decision := f.reserve("goal-d", "same-decision", identities, episode, binding, 3*time.Millisecond)
	if decision.Disposition != DispositionReusableSuccess || resume.AttemptID != "" {
		t.Fatalf("same fresh episode launched again: %+v %+v", resume, decision)
	}
	attempts, err = ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if projection := ReusedTestResult(NewTestResult(freshTemplate), attempts, identities, contract); !projection.Delivery.Sufficient || projection.Groups[0].ReuseAttempt != fresh.AttemptID {
		t.Fatalf("unchanged episode could not consume native observation: %+v", projection.Groups)
	}
	renewed, decision := f.reserve("goal-e", "new-decision", identities, strings.Repeat("3", 64), binding, 4*time.Millisecond)
	if decision.Disposition != DispositionExecuted || renewed.TestOwned["native"] == "" {
		t.Fatalf("new episode did not renew native work: %+v %+v", renewed, decision)
	}
	data, err = os.ReadFile(counter)
	if err != nil || string(data) != "xx" {
		t.Fatalf("native launches after fresh diagnosis %q err=%v", data, err)
	}
}
