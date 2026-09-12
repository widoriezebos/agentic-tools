package proofrun

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// reuseFixture retains attempts under a shared proof identity so the
// composer can be driven with hand-built group observations.
type reuseFixture struct {
	t        *testing.T
	root     string
	identity ProofIdentity
	launcher ProcessIdentity
	now      time.Time
	contract testpolicy.Contract
}

func newReuseFixture(t *testing.T) *reuseFixture {
	t.Helper()
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &reuseFixture{t: t, root: root, identity: identity, launcher: launcher, now: time.Now().UTC(),
		contract: testpolicy.Contract{Groups: []testpolicy.Group{{ID: "first", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"first"}}}}}
}

// reserve opens an attempt for a goal whose plan names the group's identity
// in its proof identity inputs, the way a top-level test run publishes it.
// Component identities are left off the admission so a second attempt of
// the same goal is reserved instead of answered reusable-success.
func (f *reuseFixture) reserve(goalID string, revision uint64, startedAt time.Time, plan string) Attempt {
	f.t.Helper()
	identity := BindIdentityInputs(f.identity, []string{"group:first:" + strings.Repeat("1", 64), "plan:" + plan})
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root, GoalID: goalID, GoalRevision: revision, AccountingRevision: revision,
		ReservedMinutes: 2, Identity: identity, Launcher: f.launcher, Now: startedAt})
	if err != nil {
		f.t.Fatal(err)
	}
	return attempt
}

// finalize retains the attempt with one observation of first at the shared
// identity, ended at endedAt, under the terminal named.
func (f *reuseFixture) finalize(attempt Attempt, status, terminal string, endedAt time.Time) {
	f.t.Helper()
	result := componentAttemptResult(attempt.AttemptID, "first", strings.Repeat("1", 64), status)
	result.Groups[0].StartedAt = endedAt.Add(-time.Second).Format(time.RFC3339Nano)
	result.Groups[0].EndedAt = endedAt.Format(time.RFC3339Nano)
	code := 0
	if terminal != TerminalSuccess {
		code = 23
	}
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, terminal, code, "fixture", nil, &result, endedAt); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reuseFixture) compose(purpose testpolicy.Purpose) GroupResult {
	f.t.Helper()
	attempts, err := ReadAttempts(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	template := componentAttemptResult("", "first", strings.Repeat("1", 64), "passed")
	template.Groups = nil
	template.Purpose = purpose
	projection := ReusedTestResult(template, attempts, map[string]string{"first": strings.Repeat("1", 64)}, f.contract)
	if len(projection.Groups) != 1 {
		f.t.Fatalf("projection has %d groups: %+v", len(projection.Groups), projection.Groups)
	}
	return projection.Groups[0]
}

func TestReuseCrossesGoalsAndAttempts(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "x")
	f.finalize(a, "passed", TerminalSuccess, f.now.Add(time.Second))
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "reused" || group.ReuseAttempt != a.AttemptID {
		t.Fatalf("a pass retained under goal x was not reused for a template of another goal: %+v", group)
	}
	// The composer no longer takes a goal: the template above named none and
	// the attempt named goal-x; a failed terminal on the owning attempt does
	// not matter either, only the group's own record does.
	b := f.reserve("goal-y", 1, f.now.Add(2*time.Second), "y")
	f.finalize(b, "passed", TerminalFailed, f.now.Add(3*time.Second))
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "reused" || group.ReuseAttempt != b.AttemptID {
		t.Fatalf("a complete pass inside a failed attempt was not the newest reusable observation: %+v", group)
	}
	if err := validateRetainedGroupReuse(f.root, "first", group(t, f, b.AttemptID)); err != nil {
		t.Fatalf("the runner refused a pass owned by a failed attempt: %v", err)
	}
}

func group(t *testing.T, f *reuseFixture, attemptID string) GroupResult {
	t.Helper()
	reused := f.compose(testpolicy.PurposeDelivery)
	if reused.ReuseAttempt != attemptID {
		t.Fatalf("expected reuse from %s, got %+v", attemptID, reused)
	}
	return reused
}

func TestGreenThenRedYieldsNoReuseAndTheClockIsTheGroupsEnd(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "a")
	f.finalize(a, "passed", TerminalSuccess, f.now.Add(time.Second))
	b := f.reserve("goal-x", 3, f.now.Add(2*time.Second), "b")
	f.finalize(b, "failed", TerminalFailed, f.now.Add(3*time.Second))
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "not-run" || group.NotRunReason != "newest-observation-failed" {
		t.Fatalf("a newer failure did not block reuse of the older pass: %+v", group)
	}
	c := f.reserve("goal-x", 3, f.now.Add(4*time.Second), "c")
	f.finalize(c, "passed", TerminalSuccess, f.now.Add(5*time.Second))
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "reused" || group.ReuseAttempt != c.AttemptID {
		t.Fatalf("a newer pass after the failure was not reused: %+v", group)
	}
	// Overlap: d starts after c but judged the group earlier than c did; the
	// group's own end decides, so c's later pass still wins.
	d := f.reserve("goal-x", 3, f.now.Add(4*time.Second+500*time.Millisecond), "d")
	f.finalize(d, "failed", TerminalFailed, f.now.Add(4*time.Second+800*time.Millisecond))
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "reused" || group.ReuseAttempt != c.AttemptID {
		t.Fatalf("an overlapping earlier failure from a later-started attempt outranked the later pass: %+v", group)
	}
}

func TestALiveObservationBlocksReuse(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "a")
	f.finalize(a, "passed", TerminalSuccess, f.now.Add(time.Second))
	// A top-level attempt with no terminal publishes its plan in the proof
	// identity inputs; that plan is the newest observation and blocks reuse.
	f.reserve("goal-z", 1, f.now.Add(2*time.Second), "live")
	if group := f.compose(testpolicy.PurposeDelivery); group.Status != "not-run" || group.NotRunReason != "live-observation-blocks-reuse" {
		t.Fatalf("a live plan for the identity did not block reuse: %+v", group)
	}
}

func TestCadenceComposesNoReuse(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "a")
	f.finalize(a, "passed", TerminalSuccess, f.now.Add(time.Second))
	if group := f.compose(testpolicy.PurposeCadence); group.Status != "not-run" || group.NotRunReason != "cadence-executes-afresh" {
		t.Fatalf("a cadence template reused retained proof: %+v", group)
	}
}

func TestVerifyRefusesWithoutAMatchingRetainedResult(t *testing.T) {
	f := newReuseFixture(t)
	attempts, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	template := componentAttemptResult("", "first", strings.Repeat("1", 64), "passed")
	template.Groups = nil
	projection := ReusedTestResult(template, attempts, map[string]string{"first": strings.Repeat("9", 64)}, f.contract)
	if projection.Delivery.Sufficient || len(projection.Delivery.MissingGroups) != 1 || projection.Groups[0].NotRunReason != "missing-proof" {
		t.Fatalf("a required group without a matching retained result was not reported missing: %+v", projection.Delivery)
	}
}

func TestExecuteAfreshNeverAnswersReusableSuccess(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "same")
	f.finalize(a, "passed", TerminalSuccess, f.now.Add(time.Second))
	identity := BindIdentityInputs(f.identity, []string{"group:first:" + strings.Repeat("1", 64), "plan:same"})
	request := AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root, GoalID: "goal-x", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 2, Identity: identity, Launcher: f.launcher, Now: f.now.Add(2 * time.Second), ComponentIdentities: map[string]string{"first": strings.Repeat("1", 64)}}
	if decision, decided, err := NoChildDecisionLocked(request); err != nil || !decided || decision.Disposition != DispositionReusableSuccess {
		t.Fatalf("a same-goal success was not answered reusable-success by default: %+v decided=%v err=%v", decision, decided, err)
	}
	request.ExecuteAfresh = true
	if decision, decided, err := NoChildDecisionLocked(request); err != nil || decided {
		t.Fatalf("an execute-afresh admission still answered without a child: %+v err=%v", decision, err)
	}
	fresh, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || fresh.AttemptID == "" || fresh.PreviousAttempt != "" {
		t.Fatalf("an execute-afresh admission did not reserve a fresh attempt: %+v %+v err=%v", fresh, decision, err)
	}
	duplicate := request
	duplicate.Now = f.now.Add(3 * time.Second)
	if decision, decided, err := NoChildDecisionLocked(duplicate); err != nil || !decided || decision.Disposition != DispositionLiveDuplicate {
		t.Fatalf("execute-afresh admission stopped answering a live duplicate: %+v decided=%v err=%v", decision, decided, err)
	}
}

func TestExactReuseYieldsToANewerObservationOnTheSeat(t *testing.T) {
	f := newReuseFixture(t)
	a := f.reserve("goal-x", 3, f.now, "a")
	result := componentAttemptResult(a.AttemptID, "first", strings.Repeat("1", 64), "passed")
	result.Groups[0].EndedAt = f.now.Add(time.Second).Format(time.RFC3339Nano)
	payload := json.RawMessage(`{"schemaVersion":2,"marker":"byte-exact"}`)
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, a.AttemptID, TerminalSuccess, 0, "green", payload, &result, f.now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	identities := map[string]string{"first": strings.Repeat("1", 64)}
	template := result
	template.AttemptID = ""
	attempts, err := ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ExactReusableTestResult(template, attempts, identities, "goal-x", 3); !ok {
		t.Fatal("exact reuse refused its own committed green")
	}
	b := f.reserve("goal-y", 1, f.now.Add(2*time.Second), "b")
	f.finalize(b, "failed", TerminalFailed, f.now.Add(3*time.Second))
	if attempts, err = ReadAttempts(f.root); err != nil {
		t.Fatal(err)
	}
	if exact, ok := ExactReusableTestResult(template, attempts, identities, "goal-x", 3); ok {
		t.Fatalf("exact reuse republished a green that a newer failure on the seat contradicts: %+v", exact)
	}
}
