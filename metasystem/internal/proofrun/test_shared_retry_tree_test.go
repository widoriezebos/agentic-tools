package proofrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestSharedRetryUsesNewestObservationOnRequestedTree(t *testing.T) {
	root, base := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	treeB, treeA, unseenTree := strings.Repeat("2", 40), strings.Repeat("1", 40), strings.Repeat("3", 40)
	groupIdentity := strings.Repeat("a", 64)
	groups := map[string]string{"group": groupIdentity}
	request := func(tree, plan string, offset time.Duration) AdmissionRequest {
		item := componentAdmissionRequest(root, base, launcher, now.Add(offset), tree, plan, groups)
		item.SharedComponents = true
		return item
	}
	failedB := retainComponentAttempt(t, request(treeB, "red-b", 0), treeB,
		[]componentStatus{{"group", "failed"}}, now.Add(time.Second))
	failedA := retainComponentAttempt(t, request(treeA, "red-a", 2*time.Second), treeA,
		[]componentStatus{{"group", "failed"}}, now.Add(3*time.Second))
	for _, item := range []struct {
		tree, prior string
	}{
		{treeB, failedB.AttemptID},
		{treeA, failedA.AttemptID},
	} {
		candidate := request(item.tree, "return", 4*time.Second)
		decision, decided, err := NoChildDecisionLocked(candidateAdmission(candidate))
		if err != nil || !decided || decision.Disposition != DispositionRetryRequired || decision.PriorAttempt != item.prior {
			t.Fatalf("candidate %s lost its own failed producer: decision=%+v decided=%t err=%v", item.tree, decision, decided, err)
		}
	}
	candidate := request(unseenTree, "unseen", 4*time.Second)
	if decision, decided, err := NoChildDecisionLocked(candidateAdmission(candidate)); err != nil || decided {
		t.Fatalf("unseen tree inherited another tree's retry: decision=%+v decided=%t err=%v", decision, decided, err)
	}
	fresh, decision, err := ReserveLocked(candidateAdmission(candidate))
	if err != nil || decision.Disposition != DispositionExecuted || fresh.TestOwned["group"] != groupIdentity || fresh.TestSources["group"] != "" {
		t.Fatalf("unseen tree did not own fresh execution: attempt=%+v decision=%+v err=%v", fresh, decision, err)
	}
}

func TestSharedCandidatePassSupersedesItsEarlierFailure(t *testing.T) {
	root, base := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	treeB, treeA := strings.Repeat("2", 40), strings.Repeat("1", 40)
	groupIdentity := strings.Repeat("a", 64)
	groups := map[string]string{"group": groupIdentity}
	request := func(tree, plan string, offset time.Duration) AdmissionRequest {
		item := componentAdmissionRequest(root, base, launcher, now.Add(offset), tree, plan, groups)
		item.SharedComponents = true
		return item
	}
	prior := retainComponentAttempt(t, request(treeB, "red-b", 0), treeB,
		[]componentStatus{{"group", "failed"}}, now.Add(time.Second))
	evidencePath := filepath.Join(root, "failure.log")
	if err := os.WriteFile(evidencePath, []byte("reviewed failing group\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "retry.json")
	encoded, err := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: prior.AttemptID,
		Cause: "deterministic group failure", EvidencePath: filepath.Base(evidencePath), Rationale: "the failing group was reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	retry := request(treeB, "red-b", 2*time.Second)
	retry.RetryDecisionPath = decisionPath
	passed, decision, err := ReserveLocked(candidateAdmission(retry))
	if err != nil || decision.Disposition != DispositionExecuted || passed.PreviousAttempt != prior.AttemptID || passed.TestOwned["group"] != groupIdentity {
		t.Fatalf("accountable retry did not own native group: attempt=%+v decision=%+v err=%v", passed, decision, err)
	}
	result := componentAttemptResult(passed.AttemptID, "group", groupIdentity, "passed")
	result.CandidateTree = treeB
	result.Groups[0].EndedAt = now.Add(3 * time.Second).Format(time.RFC3339Nano)
	if _, err := FinalizeAttemptWithTestResultLocked(root, passed.AttemptID, TerminalSuccess, 0, "green", nil, &result, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	other := request(treeA, "red-a", 4*time.Second)
	other.ForceAttempt, other.ForceGroups = true, true
	retainComponentAttempt(t, other, treeA, []componentStatus{{"group", "failed"}}, now.Add(5*time.Second))
	candidate := request(treeB, "return-b", 6*time.Second)
	if decision, decided, err := NoChildDecisionLocked(candidateAdmission(candidate)); err != nil || decided {
		t.Fatalf("earlier red survived a newer pass on its tree: decision=%+v decided=%t err=%v", decision, decided, err)
	}
	fresh, decision, err := ReserveLocked(candidateAdmission(candidate))
	if err != nil || decision.Disposition != DispositionExecuted || fresh.TestOwned["group"] != groupIdentity || fresh.TestSources["group"] != "" {
		t.Fatalf("newer cross-tree red was reused through or refenced by the superseded failure: attempt=%+v decision=%+v err=%v", fresh, decision, err)
	}
}

func TestSharedNewerRedVetoesReuseButDifferentTreeOwnsFreshExecution(t *testing.T) {
	t.Parallel()
	root, base := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	firstTree, secondTree, thirdTree := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	groupIdentity := strings.Repeat("a", 64)
	groups := map[string]string{"group": groupIdentity}
	reserve := func(request AdmissionRequest) (Attempt, LaunchResult) {
		t.Helper()
		fixtureHostAdmissionMu.Lock()
		defer fixtureHostAdmissionMu.Unlock()
		attempt, decision, err := ReserveLocked(candidateAdmission(request))
		if err != nil {
			t.Fatal(err)
		}
		return attempt, decision
	}
	greenRequest := componentAdmissionRequest(root, base, launcher, now, firstTree, "green", groups)
	greenRequest.SharedComponents = true
	green, decision := reserve(greenRequest)
	if decision.Disposition != DispositionExecuted || green.TestOwned["group"] != groupIdentity {
		t.Fatalf("first native producer was not owned: attempt=%+v decision=%+v", green, decision)
	}
	greenResult := componentAttemptResult(green.AttemptID, "group", groupIdentity, "passed")
	greenResult.CandidateTree = firstTree
	greenResult.Groups[0].EndedAt = now.Add(time.Second).Format(time.RFC3339Nano)
	if _, err := FinalizeAttemptWithTestResultLocked(root, green.AttemptID, TerminalSuccess, 0, "green", nil, &greenResult, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	redRequest := componentAdmissionRequest(root, base, launcher, now.Add(2*time.Second), firstTree, "red", groups)
	redRequest.SharedComponents, redRequest.ForceAttempt, redRequest.ForceGroups = true, true, true
	red, decision := reserve(redRequest)
	if decision.Disposition != DispositionExecuted || red.TestOwned["group"] != groupIdentity {
		t.Fatalf("forced newer red did not own native group: attempt=%+v decision=%+v", red, decision)
	}
	redResult := componentAttemptResult(red.AttemptID, "group", groupIdentity, "failed")
	redResult.CandidateTree = firstTree
	redResult.Groups[0].EndedAt = now.Add(3 * time.Second).Format(time.RFC3339Nano)
	if _, err := FinalizeAttemptWithTestResultLocked(root, red.AttemptID, TerminalFailed, 23, "red", nil, &redResult, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	sameTree := componentAdmissionRequest(root, base, launcher, now.Add(4*time.Second), firstTree, "same-tree", groups)
	sameTree.SharedComponents = true
	if decision, decided, err := NoChildDecisionLocked(candidateAdmission(sameTree)); err != nil || !decided ||
		decision.Disposition != DispositionRetryRequired || decision.PriorAttempt != red.AttemptID {
		t.Fatalf("same-tree native red lost accountable retry: decision=%+v decided=%t err=%v", decision, decided, err)
	}
	otherTree := componentAdmissionRequest(root, base, launcher, now.Add(4*time.Second), secondTree, "reassembled", groups)
	otherTree.SharedComponents = true
	if decision, decided, err := NoChildDecisionLocked(candidateAdmission(otherTree)); err != nil || decided {
		t.Fatalf("other tree required retry instead of fresh execution: decision=%+v decided=%t err=%v", decision, decided, err)
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	var completedRed Attempt
	for _, attempt := range attempts {
		if attempt.AttemptID == red.AttemptID {
			completedRed = attempt
			break
		}
	}
	if completedRed.Terminal == nil {
		t.Fatal("newer red producer did not retain its terminal record")
	}
	template := componentAttemptResult("", "group", groupIdentity, "passed")
	template.CandidateTree, template.Groups = secondTree, nil
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "group", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"group"}}}}
	projection := ReusedTestResult(template, attempts, groups, contract)
	if projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].Status != "not-run" {
		t.Fatalf("older green was reused through newer red: %+v", projection)
	}
	// An older-format failed reservation without a trustworthy tree remains
	// an accountable retry. Restore the exact current-format producer before
	// proving the different-tree fresh-owner path.
	legacy := completedRed
	legacy.SchemaVersion = AttemptSchemaVersion
	legacy.CandidateGoalID, legacy.CandidateRevision, legacy.CandidateBudgetEpoch, legacy.CandidateTree = "", 0, nil, ""
	legacy.TestResult, legacy.TestInventory, legacy.TestOwned, legacy.TestSources, legacy.TestWaits = nil, nil, nil, nil, nil
	legacy.TestAdmission = 0
	legacy.PendingTestGroups = map[string]string{"group": groupIdentity}
	if err := writeAttempt(legacy); err != nil {
		t.Fatal(err)
	}
	if decision, decided, err := NoChildDecisionLocked(candidateAdmission(otherTree)); err != nil || !decided ||
		decision.Disposition != DispositionRetryRequired || decision.PriorAttempt != red.AttemptID {
		t.Fatalf("unknown prior tree lost conservative retry: decision=%+v decided=%t err=%v", decision, decided, err)
	}
	if err := writeAttempt(completedRed); err != nil {
		t.Fatal(err)
	}
	fresh, decision := reserve(otherTree)
	if decision.Disposition != DispositionExecuted || fresh.TestOwned["group"] != groupIdentity ||
		fresh.TestSources["group"] != "" || fresh.TestWaits["group"] != "" {
		t.Fatalf("reassembled tree did not own fresh native execution: attempt=%+v decision=%+v", fresh, decision)
	}
	later := componentAdmissionRequest(root, base, launcher, now.Add(5*time.Second), thirdTree, "later", groups)
	later.SharedComponents = true
	waiter, waitDecision := reserve(later)
	if waitDecision.Disposition != DispositionExecuted || waiter.TestWaits["group"] != fresh.AttemptID || waiter.TestOwned["group"] != "" {
		t.Fatalf("live producer was not shared across trees: attempt=%+v decision=%+v", waiter, waitDecision)
	}
}
