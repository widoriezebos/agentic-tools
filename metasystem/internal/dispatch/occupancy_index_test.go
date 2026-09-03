package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func occupancyIndexFixture(t *testing.T, root, sessionKey string, generation int64, occupants ...SessionOccupant) {
	t.Helper()
	_, indexPath, _ := sessionOccupancyPaths(root, sessionKey)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeSessionOccupancyIndex(indexPath, sessionOccupancyIndex{
		SessionKey: sessionKey,
		Generation: generation,
		Occupants:  occupants,
	}); err != nil {
		t.Fatal(err)
	}
}

func resolvePreparedOccupancy(t *testing.T, reader IndexedSessionOccupancyReader, root, sessionKey string) SessionOccupancy {
	t.Helper()
	prepared, err := reader.Prepare(root, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	var resolved SessionOccupancy
	err = reader.Resolve(root, sessionKey, "new-opid", prepared, func(occupancy SessionOccupancy, _ *SessionIndexTransaction) error {
		resolved = occupancy
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestSessionOccupancyBusyWithoutRecordHealsToFree(t *testing.T) {
	root := t.TempDir()
	sessionKey := "codex:crashed-creation"
	occupancyIndexFixture(t, root, sessionKey, 7, SessionOccupant{
		OpID: "vanished-reservation", Status: "pending-setup", ProofLevel: "proven", Reason: "reservation",
	})

	resolved := resolvePreparedOccupancy(t, IndexedSessionOccupancyReader{}, root, sessionKey)
	if resolved.Busy != nil || resolved.Unprovable != nil || resolved.Healing == nil || !resolved.Healing.Applied ||
		resolved.Healing.Resolution != "session-record-recovery-applied" || resolved.Healing.RecordsRead != 2 {
		t.Fatalf("busy-without-record recovery = %+v, want labeled free healing", resolved)
	}
	index, err := readSessionOccupancyIndex(root, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if index.Generation != 8 || len(index.Occupants) != 0 {
		t.Fatalf("healed index = %+v, want generation 8 and no occupants", index)
	}
}

func TestSessionOccupancyBusyWithoutRecordRechecksBeforeHealing(t *testing.T) {
	root := t.TempDir()
	sessionKey := "codex:creation-finishes"
	occupancyIndexFixture(t, root, sessionKey, 3, SessionOccupant{
		OpID: "finishing", Status: "pending-setup", ProofLevel: "proven", Reason: "reservation",
	})
	reader := IndexedSessionOccupancyReader{BeforeGenerationCheck: func() {
		writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "finishing.json", map[string]any{
			"jobId": "finishing", "sessionKey": sessionKey, "status": "pending-setup", "proofLevel": "proven",
		})
	}}

	resolved := resolvePreparedOccupancy(t, reader, root, sessionKey)
	if resolved.Busy == nil || resolved.Busy.OpID != "finishing" || resolved.Healing == nil ||
		resolved.Healing.Resolution != "session-record-recovery-revalidated-current" || resolved.Healing.Applied {
		t.Fatalf("finishing creation = %+v, want the under-lock record reread to preserve busy", resolved)
	}
	index, err := readSessionOccupancyIndex(root, sessionKey)
	if err != nil || index.Generation != 3 || len(index.Occupants) != 1 {
		t.Fatalf("revalidated index = %+v err=%v", index, err)
	}
}

func TestSessionOccupancyStaleBusyHealsAfterTerminalRecord(t *testing.T) {
	root := t.TempDir()
	sessionKey := "codex:crashed-terminal"
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "finished.json", map[string]any{
		"jobId": "finished", "sessionKey": sessionKey, "status": "completed", "proofLevel": "proven",
	})
	occupancyIndexFixture(t, root, sessionKey, 11, SessionOccupant{
		OpID: "finished", Status: "running", ProofLevel: "proven", Reason: "custodial-liveness-indeterminate",
	})

	resolved := resolvePreparedOccupancy(t, IndexedSessionOccupancyReader{}, root, sessionKey)
	if resolved.Busy != nil || resolved.Unprovable != nil || resolved.Healing == nil || !resolved.Healing.Applied ||
		resolved.Healing.Resolution != "session-record-recovery-applied" || resolved.Healing.RecordsRead != 2 {
		t.Fatalf("stale-busy recovery = %+v, want labeled free healing", resolved)
	}
	index, err := readSessionOccupancyIndex(root, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if index.Generation != 12 || len(index.Occupants) != 0 {
		t.Fatalf("healed index = %+v, want generation 12 and no occupants", index)
	}
}

func TestSessionOccupancyTerminalCrashAfterPreparationStillHeals(t *testing.T) {
	root := t.TempDir()
	sessionKey := "codex:terminal-after-prepare"
	recordPath := writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "ending.json", map[string]any{
		"jobId": "ending", "sessionKey": sessionKey, "status": "running", "proofLevel": "proven",
	})
	occupancyIndexFixture(t, root, sessionKey, 5, SessionOccupant{
		OpID: "ending", Status: "running", ProofLevel: "proven", Reason: "custodial-liveness-indeterminate",
	})
	reader := IndexedSessionOccupancyReader{BeforeGenerationCheck: func() {
		writeJSONFile(t, filepath.Dir(recordPath), filepath.Base(recordPath), map[string]any{
			"jobId": "ending", "sessionKey": sessionKey, "status": "completed", "proofLevel": "proven",
		})
	}}

	resolved := resolvePreparedOccupancy(t, reader, root, sessionKey)
	if resolved.Busy != nil || resolved.Unprovable != nil || resolved.Healing == nil ||
		resolved.Healing.Resolution != "session-record-recovery-applied" || !resolved.Healing.Applied {
		t.Fatalf("post-preparation terminal crash = %+v, want bounded healing to free", resolved)
	}
	index, err := readSessionOccupancyIndex(root, sessionKey)
	if err != nil || index.Generation != 6 || len(index.Occupants) != 0 {
		t.Fatalf("post-preparation healed index = %+v err=%v", index, err)
	}
}

func TestSessionOccupancyRecordVerbsKeepOnePublicationOrder(t *testing.T) {
	root := t.TempDir()
	sessionKey := "fake:record-verbs"
	setup := filepath.Join(t.TempDir(), "setup.json")
	capResolution := filepath.Join(t.TempDir(), "cap.json")
	if err := WriteCapResolution(capResolution, 120, "built-in", "default"); err != nil {
		t.Fatal(err)
	}
	if err := BuildSetup(root, setup, "record-verbs", "implementer", "", "main-1", "5", "", 0, 0, capResolution, "", ""); err != nil {
		t.Fatal(err)
	}
	// Main's BuildSetup predates session indexing; the reservation
	// carries its session identity in the setup record directly, as
	// claim.go writes it in production.
	setupRecord := readJSONFile(t, setup)
	setupRecord["sessionKey"] = sessionKey
	setupRecord["sessionNonce"] = "nonce1"
	setupRecord["proofLevel"] = "proven"
	writeJSONDoc(t, setup, setupRecord)
	if err := RecordCreate(root, "record-verbs", setup); err != nil {
		t.Fatal(err)
	}
	index, err := readSessionOccupancyIndex(root, sessionKey)
	if err != nil || len(index.Occupants) != 1 || index.Occupants[0].Status != "pending-setup" {
		t.Fatalf("creation index = %+v err=%v", index, err)
	}
	record := readRecord(t, root, "record-verbs")
	if record["sessionOccupancyHealing"] == nil || record["sessionOccupancyClaimGeneration"] == nil {
		t.Fatalf("reservation did not label its index rebuild: %+v", record)
	}

	reservation := readRecord(t, root, "record-verbs")
	pendingDoc := map[string]any{
		"jobId": "record-verbs", "role": "implementer", "runtime": "fake", "round": 1,
		"parentJob": nil, "status": "pending", "phase": "handshake", "error": nil,
		"startedAt": "2026-08-28T10:00:00Z", "endedAt": nil,
	}
	for _, carry := range []string{"mainId", "claimEpoch", "goalId", "goalRevision", "goalTier", "operationId", "capResolution", "machineId", "approvedRef", "capMin", "sessionKey", "sessionNonce", "proofLevel"} {
		if v, ok := reservation[carry]; ok {
			pendingDoc[carry] = v
		}
	}
	pending := writeJSON(t, filepath.Join(t.TempDir(), "pending.json"), pendingDoc)
	if err := RecordSetup(root, "record-verbs", pending); err != nil {
		t.Fatal(err)
	}
	index, err = readSessionOccupancyIndex(root, sessionKey)
	if err != nil || len(index.Occupants) != 1 || index.Occupants[0].Status != "pending" {
		t.Fatalf("setup index = %+v err=%v", index, err)
	}

	patch := writeJSON(t, filepath.Join(t.TempDir(), "terminal.json"), map[string]any{"error": nil})
	if _, err := RecordCAS(root, "record-verbs", "pending", "cancelled", patch); err != nil {
		t.Fatal(err)
	}
	index, err = readSessionOccupancyIndex(root, sessionKey)
	if err != nil || len(index.Occupants) != 0 {
		t.Fatalf("terminal index = %+v err=%v", index, err)
	}
	if got := readRecord(t, root, "record-verbs")["status"]; got != "cancelled" {
		t.Fatalf("terminal record status = %v", got)
	}
}

func TestSessionOccupancyConcurrentDistinctOpIDClaimantsHaveOneWinner(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	paramsA := claimParamsForTest(root, "occupancy-racer-a")
	paramsB := claimParamsForTest(root, "occupancy-racer-b")
	paramsA.Request.SessionKey = "codex:shared-session"
	paramsB.Request.SessionKey = paramsA.Request.SessionKey
	results := make(chan ClaimResult, 2)
	errors := make(chan error, 2)
	for _, params := range []ClaimLaunchParams{paramsA, paramsB} {
		params := params
		go func() {
			dependencies := claimDependenciesForTest(&now, identity.Verification{})
			dependencies.Occupancy = nil
			result, err := ClaimLaunch(params, dependencies)
			results <- result
			errors <- err
		}()
	}
	counts := map[ClaimOutcome]int{}
	for range 2 {
		result := <-results
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
		counts[result.Outcome]++
	}
	if counts[ClaimWON] != 1 || counts[ClaimRefusedSessionBusy] != 1 {
		t.Fatalf("distinct-opid outcomes = %v, want one WON and one REFUSED-SESSION-BUSY", counts)
	}
}

func TestSessionOccupancySlowRegistryRecoveryDoesNotHoldCapLock(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	reader := IndexedSessionOccupancyReader{BeforeRegistryScan: func() {
		close(started)
		<-release
	}}
	prepared := make(chan SessionOccupancyPreparation, 1)
	errors := make(chan error, 1)
	go func() {
		value, err := reader.Prepare(root, "codex:slow-registry")
		prepared <- value
		errors <- err
	}()
	<-started

	capPath := filepath.Join(root, "artifacts", "agents", "supervision", "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(capPath), 0o755); err != nil {
		t.Fatal(err)
	}
	acquiredAt := time.Now()
	if err := OwnerLockClaim(capPath, int64(os.Getpid()), "occupancy-slow-registry-fixture"); err != nil {
		t.Fatalf("cap lock was held while the registry scan was blocked: %v", err)
	}
	defer OwnerLockRelease(capPath, int64(os.Getpid()), "occupancy-slow-registry-fixture")
	if elapsed := time.Since(acquiredAt); elapsed >= 10*time.Second {
		t.Fatalf("cap lock acquisition took %s, want less than the 10 second ceiling", elapsed)
	}
	close(release)
	<-prepared
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}

func TestSessionOccupancyGenerationRaceRereadsIndex(t *testing.T) {
	root := t.TempDir()
	sessionKey := "codex:generation-race"
	var once sync.Once
	reader := IndexedSessionOccupancyReader{BeforeGenerationCheck: func() {
		once.Do(func() {
			writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "newer.json", map[string]any{
				"jobId": "newer", "sessionKey": sessionKey, "status": "pending-setup", "proofLevel": "proven",
			})
			occupancyIndexFixture(t, root, sessionKey, 1, SessionOccupant{
				OpID: "newer", Status: "pending-setup", ProofLevel: "proven", Reason: "reservation",
			})
		})
	}}

	resolved := resolvePreparedOccupancy(t, reader, root, sessionKey)
	if resolved.Busy == nil || resolved.Busy.OpID != "newer" || resolved.Healing == nil || resolved.Healing.Resolution != "generation-changed-index-reread" {
		t.Fatalf("generation race = %+v, want the newer busy index rather than the stale free scan", resolved)
	}
}

func writeJSONDoc(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
