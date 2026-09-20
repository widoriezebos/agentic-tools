package proofrun

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func reserveOwnershipReviewFixture(f ownershipFixture, goal, plan string, groups map[string]string, episode, binding string, offset time.Duration) (Attempt, LaunchResult) {
	f.t.Helper()
	fixtureHostAdmissionMu.Lock()
	defer fixtureHostAdmissionMu.Unlock()
	identity := BindIdentityInputs(f.identity, append(append([]string(nil), f.identity.IdentityInputs...), "plan:"+plan))
	request := candidateAdmission(AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: goal, GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 2,
		Identity: identity, Launcher: f.launcher, Now: f.now.Add(offset),
		ComponentIdentities: groups, SharedComponents: true, FreshnessEpisode: episode, FreshnessBinding: binding})
	request.loadOptions = []loadSampleOption{withTestHostLoad("0")}
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

func TestGLEOwnershipFreshExpiryIsBoundAtWriteReadAndReuse(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("a", 64)
	episode, binding := strings.Repeat("b", 64), strings.Repeat("c", 64)
	attempt, decision := reserveOwnershipReviewFixture(f, "goal-fresh", "fresh-expiry", map[string]string{"check": identity}, episode, binding, 0)
	if decision.Disposition != DispositionExecuted {
		t.Fatalf("fresh fixture admission: %+v", decision)
	}
	admittedExpiry := f.now.Add(time.Minute).Format(time.RFC3339Nano)
	changedExpiry := f.now.Add(90 * time.Second).Format(time.RFC3339Nano)
	attempt.FreshnessExpiresAt = admittedExpiry
	if err := writeAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	result := componentAttemptResult(attempt.AttemptID, "check", identity, "passed")
	result.FreshnessEpisode, result.FreshnessBinding = episode, binding
	result.FreshnessExpiresAt = changedExpiry
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, TerminalSuccess, 0, "native pass", nil, &result, f.now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "freshness") {
		t.Fatalf("writer accepted a changed expiry: %v", err)
	}
	result.FreshnessExpiresAt = admittedExpiry
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, TerminalSuccess, 0, "native pass", nil, &result, f.now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retained, err := ReadAttempt(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	retained.TestResult.FreshnessExpiresAt = changedExpiry
	path, err := AttemptPath(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(retained, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAttempt(f.root, attempt.AttemptID); err == nil || !strings.Contains(err.Error(), "freshness") {
		t.Fatalf("serialized reader accepted a changed expiry: %v", err)
	}
	template := result
	template.AttemptID = ""
	if observed, found := newestReuseObservation(template, []Attempt{retained}, "check", identity, ""); found {
		t.Fatalf("retained-result scan reused a changed-expiry producer: %+v", observed)
	}
	if observed, found := newestOwnedObservation([]Attempt{retained}, "check", identity, episode, binding, admittedExpiry, f.now); found {
		t.Fatalf("ownership scan reused a changed-expiry producer: %+v", observed)
	}
	projection := ReusedTestResult(template, []Attempt{retained}, map[string]string{"check": identity},
		testpolicy.Contract{Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Inputs: []string{"source"}}}})
	if projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].Status != "not-run" {
		t.Fatalf("receipt projection accepted changed-expiry native proof: %+v", projection.Groups)
	}
}

func TestGLEOwnershipSerializedLegacyReusedConsumerDoesNotMaskProducer(t *testing.T) {
	t.Parallel()
	f := newOwnershipFixture(t)
	identity := strings.Repeat("d", 64)
	producer, decision := reserveOwnershipReviewFixture(f, "goal-producer", "legacy-native", map[string]string{"check": identity}, "", "", 0)
	if decision.Disposition != DispositionExecuted {
		t.Fatalf("producer fixture admission: %+v", decision)
	}
	consumer, decision := reserveOwnershipReviewFixture(f, "goal-consumer", "legacy-reused", map[string]string{"check": identity}, "", "", time.Millisecond)
	if decision.Disposition != DispositionExecuted {
		t.Fatalf("consumer fixture admission: %+v", decision)
	}
	f.finish(producer, nil, f.now.Add(time.Second))
	f.finish(consumer, nil, f.now.Add(2*time.Second))
	for _, id := range []string{producer.AttemptID, consumer.AttemptID} {
		retained, err := ReadAttempt(f.root, id)
		if err != nil {
			t.Fatal(err)
		}
		retained.SchemaVersion = CandidateAttemptSchemaVersion
		retained.TestResult.SchemaVersion = LegacyTestResultSchemaVersion
		retained.PendingTestGroups = map[string]string{"check": identity}
		retained.TestInventory, retained.TestOwned, retained.TestWaits, retained.TestSources = nil, nil, nil, nil
		retained.TestAdmission, retained.TestFreshGroups = 0, nil
		retained.FreshnessEpisode, retained.FreshnessBinding, retained.FreshnessExpiresAt = "", "", ""
		encoded, err := json.MarshalIndent(retained, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(encoded, []byte(`"schemaVersion": 3`)) || bytes.Contains(encoded, []byte(`"testInventory"`)) {
			t.Fatalf("fixture is not an old serialized attempt: %s", encoded)
		}
		path, err := AttemptPath(f.root, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	attempts, err := ReadAttempts(f.root)
	if err != nil || len(attempts) != 2 {
		t.Fatalf("decode serialized legacy attempts: %d, %v", len(attempts), err)
	}
	for _, attempt := range attempts {
		if attempt.AttemptID == consumer.AttemptID && ownsTestComponent(attempt, "check", identity) {
			t.Fatal("terminal reused consumer became a native owner")
		}
	}
	owned, found := newestOwnedObservation(attempts, "check", identity, "", "", "", f.now.Add(3*time.Second))
	if !found || owned.attempt.AttemptID != producer.AttemptID || !owned.passed || owned.failed {
		t.Fatalf("ownership scan lost original native producer: %+v found=%v", owned, found)
	}
	template := componentAttemptResult("", "check", identity, "passed")
	reuse, found := newestReuseObservation(template, attempts, "check", identity, "")
	if !found || reuse.attemptID != producer.AttemptID || !reuse.passed || reuse.live {
		t.Fatalf("retained-result scan lost original native producer: %+v found=%v", reuse, found)
	}
	projection := ReusedTestResult(template, attempts, map[string]string{"check": identity},
		testpolicy.Contract{Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Inputs: []string{"source"}}}})
	if !projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].ReuseAttempt != producer.AttemptID || projection.Groups[0].Status != "reused" {
		t.Fatalf("legacy consumer masked native source in receipt projection: %+v", projection.Groups)
	}
}
