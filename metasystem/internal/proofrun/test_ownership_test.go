package proofrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type ownershipFixture struct {
	t        *testing.T
	root     string
	identity ProofIdentity
	launcher ProcessIdentity
	now      time.Time
}

func newOwnershipFixture(t *testing.T) ownershipFixture {
	t.Helper()
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	return ownershipFixture{t: t, root: root, identity: identity, launcher: launcher, now: time.Now().UTC()}
}

func TestRunTestPlanRefusesWrongAdmittedContext(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	component := strings.Repeat("a", 64)
	attempt, _ := f.reserve("goal-context", "plan-context", map[string]string{"native": component}, "", "", 0)
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
	f.t.Helper()
	fixtureHostAdmissionMu.Lock()
	defer fixtureHostAdmissionMu.Unlock()
	identity := BindIdentityInputs(f.identity, append(append([]string(nil), f.identity.IdentityInputs...), "plan:"+plan))
	request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: goal, GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
		Identity: identity, Launcher: f.launcher, Now: f.now.Add(offset),
		ComponentIdentities: groups, SharedComponents: true, FreshnessEpisode: episode, FreshnessBinding: binding})
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
	third, _ := f.reserve("goal-c", "a-c", map[string]string{"a": a, "c": c}, "", "", 2*time.Millisecond)
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
	producer, _ := f.reserve("goal-a", "producer", map[string]string{"failing": identity}, "", "", 0)
	first, _ := f.reserve("goal-b", "first", map[string]string{"failing": identity}, "", "", time.Millisecond)
	second, _ := f.reserve("goal-c", "second", map[string]string{"failing": identity}, "", "", 2*time.Millisecond)
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
	producer, _ := f.reserve("goal-a", "producer", map[string]string{"test": identity}, "", "", 0)
	consumer, _ := f.reserve("goal-b", "consumer", map[string]string{"test": identity}, "", "", time.Millisecond)
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

func TestDistinctPrefixVariantsOwnDistinctAttempts(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	firstIdentity, secondIdentity := strings.Repeat("a", 64), strings.Repeat("b", 64)
	first, _ := f.reserve("goal-a", "prefix-one", map[string]string{"group": firstIdentity}, "", "", 0)
	second, _ := f.reserve("goal-b", "prefix-two", map[string]string{"group": secondIdentity}, "", "", time.Millisecond)
	if first.TestOwned["group"] != firstIdentity || second.TestOwned["group"] != secondIdentity || len(second.TestWaits) != 0 {
		t.Fatalf("different prefix identities shared one producer: first=%+v second=%+v", first, second)
	}
}

func TestCachedSourceIsPinnedWithTheMissingSet(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	x, y := strings.Repeat("a", 64), strings.Repeat("b", 64)
	producer, _ := f.reserve("goal-a", "prior", map[string]string{"x": x}, "", "", 0)
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
	first, _ := f.reserve("goal-a", "withdrawn", groups, "", "", 0)
	guard, err := AcquireMutation(f.root)
	if err != nil {
		t.Fatal(err)
	}
	err = WithdrawReservationLocked(f.root, first.AttemptID)
	guard.Release()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := f.reserve("goal-b", "after-withdrawal", groups, "", "", time.Millisecond)
	if second.TestAdmission != first.TestAdmission+1 {
		t.Fatalf("durable admission sequence reused %d after withdrawal; second=%d", first.TestAdmission, second.TestAdmission)
	}
}

func TestFreshEpisodeResumesButRenewsOnBinding(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("f", 64)
	groups := map[string]string{"test": identity}
	ordinary, _ := f.reserve("goal-a", "ordinary", groups, "", "", 0)
	f.finish(ordinary, map[string]string{}, f.now.Add(time.Second))
	episode, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
	fresh, _ := f.reserve("goal-b", "fresh", groups, episode, binding, 2*time.Second)
	if fresh.TestOwned["test"] != identity {
		t.Fatalf("cached ordinary pass satisfied fresh episode: %+v", fresh)
	}
	f.finish(fresh, map[string]string{}, f.now.Add(3*time.Second))
	resume, decision := f.reserve("goal-c", "resume", groups, episode, binding, 4*time.Second)
	if decision.Disposition != DispositionReusableSuccess || resume.AttemptID != "" {
		t.Fatalf("same episode did not reuse terminal native proof: %+v %+v", resume, decision)
	}
	changed, _ := f.reserve("goal-d", "changed-base", groups, episode, strings.Repeat("3", 64), 5*time.Second)
	if changed.TestOwned["test"] != identity {
		t.Fatalf("changed binding reused old observation: %+v", changed)
	}
	newEpisode, _ := f.reserve("goal-e", "new-decision", groups, strings.Repeat("4", 64), binding, 6*time.Second)
	if newEpisode.TestOwned["test"] != identity {
		t.Fatalf("new episode reused old observation: %+v", newEpisode)
	}
}

func TestExpiredFreshEpisodeHasNoReusableObservation(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity, episode, binding := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	attempt, _ := f.reserve("goal-a", "mutable", map[string]string{"mutable": identity}, episode, binding, 0)
	f.finish(attempt, map[string]string{}, f.now.Add(time.Second))
	retained, err := ReadAttempt(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	retained.FreshnessExpiresAt = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
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
	if observed, found := newestReuseObservation(template, attempts, "mutable", identity, ""); found {
		t.Fatalf("expired fresh proof remained reusable: %+v", observed)
	}
	if observed, found := newestOwnedObservation(attempts, "mutable", identity, episode, binding, retained.FreshnessExpiresAt, time.Now().UTC()); found {
		t.Fatalf("expired native producer remained current: %+v", observed)
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
