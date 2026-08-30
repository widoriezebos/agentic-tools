package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestClaimLaunchSerializesOperationIdentityAcrossJobsAndChains(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	type observed struct {
		result ClaimResult
		err    error
	}
	results := make(chan observed, 2)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	for index, job := range []string{"chain-a", "chain-b"} {
		params := claimParamsForTest(root, job)
		params.OperationID = "shared-operation"
		params.GoalID, params.GoalRevision, params.MachineID = "", 0, ""
		params.Request.SessionKey = fmt.Sprintf("fake:chain-%d", index)
		go func(p ClaimLaunchParams) {
			ready.Done()
			<-start
			result, err := ClaimLaunch(p, claimDependenciesForTest(&now, identity.Verification{}))
			results <- observed{result: result, err: err}
		}(params)
	}
	ready.Wait()
	close(start)
	counts := map[ClaimOutcome]int{}
	for range 2 {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		counts[got.result.Outcome]++
	}
	if counts[ClaimWON] != 1 || counts[ClaimRefusedOpIDMismatch] != 1 {
		t.Fatalf("shared operation outcomes = %v, want one winner and one mismatch", counts)
	}
}

func TestClaimLaunchRefusesOperationIdentityReuseByAncestor(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "chain-parent.json", map[string]any{
		"jobId": "chain-parent", "operationId": "shared-operation", "status": "completed",
	})
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	child := claimParamsForTest(root, "chain-child")
	child.OperationID = "shared-operation"
	child.AdapterVerb = "follow-up"
	child.Request.DispatchMode = DispatchModeFollowUp
	child.Request.SessionKey = "codex:chain-child"
	resumedChild := "session-chain-child"
	child.Request.ResumedSessionID = &resumedChild
	result, err := ClaimLaunch(child, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil || result.Outcome != ClaimRefusedOpIDMismatch {
		t.Fatalf("ancestor operation reuse = %s, %v; want REFUSED-OPID-MISMATCH", result.Outcome, err)
	}
}

func TestClaimLaunchRefusesHazardWithoutExecutableRuntimeEffort(t *testing.T) {
	root := t.TempDir()
	params := claimParamsForTest(root, "claude-destructive")
	params.Request.Runtime = "claude"
	params.Request.DestructiveReach = HazardDestructiveReach
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	_, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err == nil || !strings.Contains(err.Error(), "no executable maximal-effort mapping") {
		t.Fatalf("unsupported runtime hazard admission = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "artifacts", "agents", "jobs", "claude-destructive.json")); !os.IsNotExist(statErr) {
		t.Fatalf("refused runtime hazard published a reservation: %v", statErr)
	}
}

type fixedClaimVerifier struct {
	verification identity.Verification
}

func (v fixedClaimVerifier) Verify(int64, string) identity.Verification {
	return v.verification
}

type fixedOccupancyReader struct {
	result SessionOccupancy
}

func (r fixedOccupancyReader) Prepare(string, string) (SessionOccupancyPreparation, error) {
	return SessionOccupancyPreparation{}, nil
}

func (r fixedOccupancyReader) Resolve(_ string, _ string, _ string, _ SessionOccupancyPreparation, decide func(SessionOccupancy, *SessionIndexTransaction) error) error {
	return decide(r.result, &SessionIndexTransaction{disabled: true})
}

type panicOccupancyReader struct{}

func (panicOccupancyReader) Prepare(string, string) (SessionOccupancyPreparation, error) {
	panic("the busy gate ran before same-opid resolution")
}

func (panicOccupancyReader) Resolve(string, string, string, SessionOccupancyPreparation, func(SessionOccupancy, *SessionIndexTransaction) error) error {
	panic("the busy gate ran before same-opid resolution")
}

func claimDependenciesForTest(now *time.Time, process identity.Verification) ClaimLaunchDependencies {
	creator := nativeTestExact(9001, 1)
	return ClaimLaunchDependencies{
		Now:              func() time.Time { return *now },
		Sleep:            func(time.Duration) {},
		CreatorPID:       creator.Pid,
		IdentityReader:   fixedStartReader{exact: creator, state: identity.Alive},
		ProcessVerifier:  fixedClaimVerifier{verification: process},
		Occupancy:        fixedOccupancyReader{},
		Nonce:            func() (string, error) { return "0123456789abcdef", nil },
		LaunchCapability: func() (string, error) { return "launch-capability-for-test", nil },
	}
}

func claimParamsForTest(root, opid string) ClaimLaunchParams {
	return ClaimLaunchParams{
		Root: root, OpID: opid,
		Request:           launchFingerprintRequestForTest(),
		MainID:            "main-1",
		ClaimEpoch:        "5",
		GoalID:            "goal-a",
		GoalRevision:      3,
		MachineID:         "m-test",
		AdapterVerb:       "dispatch",
		DefaultCapMinutes: 120,
	}
}

func writeClaimRecord(t *testing.T, root, opid string, request LaunchFingerprintRequest, fields map[string]any) map[string]any {
	t.Helper()
	request.GoalID = "goal-a"
	request.GoalRevision = 3
	fingerprint, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatal(err)
	}
	record := map[string]any{
		"jobId": opid, "status": "pending-setup", "phase": "reservation",
		"proofLevel":         "proven",
		"sessionKey":         request.SessionKey,
		"fingerprintVersion": fingerprint.Version,
		"fingerprint":        fingerprint.Digest,
		"instanceTag":        "metasystem-job-" + opid + "-nonce",
		"pid":                nil,
	}
	for key, value := range fields {
		record[key] = value
	}
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), opid+".json", record)
	return record
}

func TestClaimLaunchOutcomeWONCreatesExactReservation(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	result, err := ClaimLaunch(claimParamsForTest(root, "job-won"), claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimWON {
		t.Fatalf("outcome = %s evidence=%v", result.Outcome, result.Evidence)
	}
	record := readRecord(t, root, "job-won")
	if !looseEqual(record["fingerprintVersion"], 2) || record["fingerprint"] == "" {
		t.Fatalf("fingerprint fields = %+v", record)
	}
	if record["reservationDeadline"] != "2026-08-27T10:10:00Z" {
		t.Fatalf("reservation deadline = %v", record["reservationDeadline"])
	}
	creator, ok := record["creatorLiveness"].(map[string]any)
	if !ok || !looseEqual(creator["pid"], 9001) {
		t.Fatalf("creator breadcrumb = %+v", record["creatorLiveness"])
	}
	assertNativeIdentityFields(t, creator, 1)
	if record["instanceTag"] != "metasystem-job-job-won-0123456789abcdef" {
		t.Fatalf("instance tag = %v", record["instanceTag"])
	}
	if record["mainId"] != "main-1" || !looseEqual(record["claimEpoch"], 5) || record["goalId"] != "goal-a" {
		t.Fatalf("reservation provenance = mainId:%v claimEpoch:%v goalId:%v", record["mainId"], record["claimEpoch"], record["goalId"])
	}
}

func TestClaimLaunchWONReservationCompletesRecordSetup(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-setup")
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimWON {
		t.Fatalf("outcome = %s evidence=%v", result.Outcome, result.Evidence)
	}
	// Main's setup handshake compares every provenance field the
	// reservation carries; the setup document mirrors the live
	// reservation rather than a hand-typed subset.
	reservation := readRecord(t, root, params.OpID)
	setupDoc := map[string]any{
		"jobId": "job-setup", "status": "pending", "workspaceRoot": root,
		"outputStream": filepath.Join(root, "events.jsonl"),
	}
	for _, carry := range []string{"mainId", "claimEpoch", "goalId", "goalRevision", "operationId", "capResolution", "machineId", "approvedRef", "capMin", "instanceTag", "fingerprint", "fingerprintVersion"} {
		if v, ok := reservation[carry]; ok {
			setupDoc[carry] = v
		}
	}
	setup := writeJSON(t, filepath.Join(t.TempDir(), "setup.json"), setupDoc)
	if err := RecordSetup(root, params.OpID, setup); err != nil {
		t.Fatalf("complete claim-launch reservation: %v", err)
	}
	record := readRecord(t, root, params.OpID)
	if record["status"] != "pending" || record["mainId"] != "main-1" ||
		!looseEqual(record["claimEpoch"], 5) || record["goalId"] != "goal-a" || record["fingerprint"] == "" {
		t.Fatalf("completed claim-launch reservation = %+v", record)
	}
}

func TestClaimLaunchOutcomeINPROGRESS(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-progress")
	writeClaimRecord(t, root, params.OpID, params.Request, nil)
	deps := claimDependenciesForTest(&now, identity.Verification{})
	deps.Occupancy = panicOccupancyReader{}
	result, err := ClaimLaunch(params, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimInProgress || result.Evidence["waitAttempt"] != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchSameOpIDResolutionPrecedesBusyGate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-same-first")
	owned := nativeTestExact(41, 1)
	fields := exactIdentityFields(owned.Ref())
	fields["status"] = "running"
	writeClaimRecord(t, root, params.OpID, params.Request, fields)
	deps := claimDependenciesForTest(&now, identity.Verification{
		Outcome: identity.VerificationVerified, Presence: identity.Alive, Identity: owned,
	})
	deps.Occupancy = panicOccupancyReader{}
	result, err := ClaimLaunch(params, deps)
	if err != nil || result.Outcome != ClaimBound {
		t.Fatalf("same-opid resolution = %+v err=%v", result, err)
	}
}

func TestConcurrentSameOpIDClaimsCreateOneReservation(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-concurrent-same")
	results := make(chan ClaimResult, 2)
	errors := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func() {
			result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
			results <- result
			errors <- err
		}()
	}
	counts := map[ClaimOutcome]int{}
	for index := 0; index < 2; index++ {
		result := <-results
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
		counts[result.Outcome]++
	}
	if counts[ClaimWON] != 1 || counts[ClaimInProgress] != 1 {
		t.Fatalf("same-opid outcomes = %v, want one WON and one IN-PROGRESS", counts)
	}
}

func TestClaimLaunchOutcomeBOUND(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-bound")
	owned := nativeTestExact(41, 1)
	fields := exactIdentityFields(owned.Ref())
	fields["status"] = "running"
	writeClaimRecord(t, root, params.OpID, params.Request, fields)
	verification := identity.Verification{
		Outcome: identity.VerificationVerified, Presence: identity.Alive, Identity: owned,
	}
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, verification))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimBound {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchOutcomeREPLAYEDForEveryTerminalStatus(t *testing.T) {
	for _, status := range []string{"completed", "failed", "cancelled", "timeout"} {
		t.Run(status, func(t *testing.T) {
			root := t.TempDir()
			now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
			params := claimParamsForTest(root, "job-replay-"+status)
			writeClaimRecord(t, root, params.OpID, params.Request, map[string]any{
				"status": status, "error": "recorded-result",
			})
			result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != ClaimOutcome("REPLAYED-"+status) {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestClaimLaunchOutcomeRECONCILINGRecordsAdoptionHandoff(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-reconcile")
	owned := nativeTestExact(41, 1)
	fields := exactIdentityFields(owned.Ref())
	fields["status"] = "running"
	writeClaimRecord(t, root, params.OpID, params.Request, fields)
	verification := identity.Verification{Outcome: identity.VerificationDead, Presence: identity.Dead}
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, verification))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimReconciling {
		t.Fatalf("result = %+v", result)
	}
	record := readRecord(t, root, params.OpID)
	handoff, ok := record["reconciliationHandoff"].(map[string]any)
	if !ok || handoff["capability"] != "nonce-global-adoption" || handoff["reason"] != "recorded-process-dead" {
		t.Fatalf("handoff does not name the adoption engine: %+v", record["reconciliationHandoff"])
	}
	if record["status"] != "running" {
		t.Fatalf("an unbound reconciliation guessed a terminal state: %v", record["status"])
	}
}

func TestClaimLaunchOutcomeREFUSEDOPIDMISMATCH(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-mismatch")
	writeClaimRecord(t, root, params.OpID, params.Request, nil)
	params.Request.InputHash = "3333333333333333333333333333333333333333333333333333333333333333"
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedOpIDMismatch {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchNeverComparesAcrossFingerprintVersions(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-version")
	writeClaimRecord(t, root, params.OpID, params.Request, map[string]any{"fingerprintVersion": 1})
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedUnprovableLegacy {
		t.Fatalf("version mismatch = %+v", result)
	}
}

func TestClaimLaunchOutcomeREFUSEDUNPROVABLELEGACY(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-legacy")
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), params.OpID+".json", map[string]any{
		"jobId": params.OpID, "status": "running", "sessionKey": params.Request.SessionKey,
	})
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedUnprovableLegacy {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchOutcomeREFUSEDSESSIONBUSY(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	deps := claimDependenciesForTest(&now, identity.Verification{})
	deps.Occupancy = fixedOccupancyReader{result: SessionOccupancy{
		Busy: &SessionOccupant{OpID: "other-job", Status: "running", Reason: "custodial-liveness-indeterminate"},
	}}
	result, err := ClaimLaunch(claimParamsForTest(root, "job-busy"), deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedSessionBusy || result.Evidence["occupancyReason"] != "custodial-liveness-indeterminate" {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchOutcomeREFUSEDUNPROVABLE(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-unprovable")
	owned := nativeTestExact(41, 1)
	fields := exactIdentityFields(owned.Ref())
	fields["status"] = "running"
	writeClaimRecord(t, root, params.OpID, params.Request, fields)
	verification := identity.Verification{Outcome: identity.VerificationIndeterminate, Presence: identity.Unknown}
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, verification))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedUnprovable {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaimLaunchReconcilesAStableRecycledPIDEvenWhenArgvIsUnreadable(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-recycled")
	recorded := nativeTestExact(41, 1)
	fields := exactIdentityFields(recorded.Ref())
	fields["status"] = "running"
	writeClaimRecord(t, root, params.OpID, params.Request, fields)
	observed := nativeTestExact(41, 2)
	verification := identity.Verification{
		Outcome: identity.VerificationIndeterminate, Presence: identity.Alive, Identity: observed,
	}
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, verification))
	if err != nil || result.Outcome != ClaimReconciling {
		t.Fatalf("recycled pid result = %+v err=%v", result, err)
	}
}

func TestClaimWaitBoundIgnoresForwardAndBackwardClockSteps(t *testing.T) {
	for _, step := range []time.Duration{24 * time.Hour, -24 * time.Hour} {
		name := "forward"
		if step < 0 {
			name = "backward"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
			params := claimParamsForTest(root, "job-clock-"+name)
			params.Wait = true
			writeClaimRecord(t, root, params.OpID, params.Request, nil)
			deps := claimDependenciesForTest(&now, identity.Verification{})
			sleeps := 0
			deps.Sleep = func(duration time.Duration) {
				if duration != ClaimWaitInterval {
					t.Fatalf("sleep = %s, want %s", duration, ClaimWaitInterval)
				}
				sleeps++
				now = now.Add(step)
			}
			result, err := ClaimLaunch(params, deps)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != ClaimReconciling || sleeps != ClaimWaitAttempts {
				t.Fatalf("result=%+v sleeps=%d, want RECONCILING after %d retry intervals", result, sleeps, ClaimWaitAttempts)
			}
			if got := result.Evidence["waitAttempts"]; !looseEqual(got, int64(ClaimWaitAttempts+1)) {
				t.Fatalf("recorded claim reads = %v, want %d (initial read plus retries)", got, ClaimWaitAttempts+1)
			}
			if got := result.Evidence["waitAttempt"]; !looseEqual(got, int64(ClaimWaitAttempts+1)) {
				t.Fatalf("final claim read number = %v, want %d", got, ClaimWaitAttempts+1)
			}
			record := readRecord(t, root, params.OpID)
			if record["status"] != "pending-setup" {
				t.Fatalf("deadline or stub failed the reservation: status=%v", record["status"])
			}
		})
	}
}

func TestIndexedSessionOccupancyTotality(t *testing.T) {
	tests := []struct {
		name         string
		record       map[string]any
		busy         bool
		freeEvidence bool
	}{
		{"pending setup reservation", map[string]any{"status": "pending-setup", "proofLevel": "proven"}, true, false},
		{"pending reservation", map[string]any{"status": "pending", "proofLevel": "proven"}, true, false},
		{"custodial liveness indeterminate", map[string]any{"status": "running", "proofLevel": "proven"}, true, false},
		{"observing seam", map[string]any{"status": "observing", "proofLevel": "seam"}, true, false},
		{"stalled seam", map[string]any{"status": "seam-stalled", "proofLevel": "seam"}, true, false},
		{"proven-ended seam", map[string]any{"status": "observing", "proofLevel": "seam", "selfReport": map[string]any{"status": "completed"}}, false, true},
		{"terminal", map[string]any{"status": "completed", "proofLevel": "proven"}, false, false},
		{"reconciled proven absent", map[string]any{"status": "reconciled-proven-absent", "proofLevel": "proven"}, false, false},
		{"archived seam", map[string]any{"status": "seam-archived", "proofLevel": "seam"}, false, false},
		{"opaque seam", map[string]any{"status": "seam-opaque", "proofLevel": "seam"}, false, true},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.record["jobId"] = fmt.Sprintf("occupant-%d", index)
			test.record["sessionKey"] = "runtime:busy"
			writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), fmt.Sprintf("occupant-%d.json", index), test.record)
			got := resolvePreparedOccupancy(t, IndexedSessionOccupancyReader{}, root, "runtime:busy")
			if (got.Busy != nil) != test.busy || (len(got.FreeEvidence) > 0) != test.freeEvidence || got.Unprovable != nil {
				t.Fatalf("occupancy = %+v, want busy=%v freeEvidence=%v", got, test.busy, test.freeEvidence)
			}
		})
	}
}

func TestClaimWONRecordsFreeOpaqueSessionEvidence(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	deps := claimDependenciesForTest(&now, identity.Verification{})
	deps.Occupancy = fixedOccupancyReader{result: SessionOccupancy{
		FreeEvidence: []SessionOccupant{{OpID: "opaque-observation", Status: "seam-opaque", Reason: "opaque-does-not-prove-occupancy"}},
	}}
	params := claimParamsForTest(root, "job-after-opaque")
	result, err := ClaimLaunch(params, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimWON {
		t.Fatalf("result = %+v", result)
	}
	record := readRecord(t, root, params.OpID)
	evidence, ok := record["sessionOccupancyEvidence"].([]any)
	if !ok || len(evidence) != 1 || evidence[0].(map[string]any)["status"] != "seam-opaque" {
		t.Fatalf("opaque free evidence was not named in the reservation: %+v", record["sessionOccupancyEvidence"])
	}
}

func TestIndexedSessionOccupancyRetiresTheTemporaryRegistryReadLimit(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	for index := 0; index <= 4096; index++ {
		writeJSONFile(t, jobs, fmt.Sprintf("terminal-%d.json", index), map[string]any{
			"jobId": fmt.Sprintf("terminal-%d", index), "status": "completed", "sessionKey": "other",
		})
	}
	got := resolvePreparedOccupancy(t, IndexedSessionOccupancyReader{}, root, "runtime:free")
	if got.Unprovable != nil || got.Busy != nil || got.Healing == nil || got.Healing.RecordsRead != 4097 {
		t.Fatalf("indexed fallback = %+v, want a complete one-time rebuild", got)
	}
}

func TestClaimRejectsInvalidFingerprintRequestWithoutCreatingARecord(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	params := claimParamsForTest(root, "job-invalid")
	params.Request.DispatchMode = DispatchModeFollowUp
	params.Request.ResumedSessionID = nil
	if _, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{})); err == nil {
		t.Fatal("a follow-up with an absent resumed session was accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "jobs", params.OpID+".json")); !os.IsNotExist(err) {
		t.Fatalf("invalid request left a record: %v", err)
	}
}
