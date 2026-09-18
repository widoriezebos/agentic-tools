package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGateCadenceTickRefusesEveryNonOwnerAndPrintsOneOwnerResult(t *testing.T) {
	originalAcquire, originalTick, originalClock := batchOwnerAcquire, cadenceTick, cadenceProductionClock
	t.Cleanup(func() {
		batchOwnerAcquire, cadenceTick, cadenceProductionClock = originalAcquire, originalTick, originalClock
	})
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	cadenceProductionClock = func() time.Time { return now }
	owner, seat := t.TempDir(), t.TempDir()
	for _, root := range []string{owner, seat} {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+owner+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runGateCadenceTick([]string{"--root", seat})
	})
	if code != 3 || !strings.Contains(stderr, cadenceNotLandingOwner) || !strings.Contains(stderr, owner) {
		t.Fatalf("non-owner code=%d stderr=%q", code, stderr)
	}
	acquires := 0
	batchOwnerAcquire = func(root string) (batchOwnerLease, error) {
		acquires++
		return batchOwnerLease{root: root, epoch: 7}, nil
	}
	cadenceTick = func(root string, held batchOwnerLease, clock func() time.Time) (cadenceTickOutput, error) {
		return cadenceTickOutput{Trunk: gaterun.CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)},
			Tick: gaterun.CadenceTickResult{Executed: true, Status: &goal.CadenceStatus{Trigger: goal.CadenceTriggerIdentityChanged, AttemptID: "attempt-1"}}}, nil
	}
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGateCadenceTick([]string{"--root", owner})
	})
	if code != 0 || stderr != "" || acquires != 1 || strings.Count(strings.TrimSpace(stdout), "\n") != 0 ||
		!strings.Contains(stdout, "trigger=identity happened=executed") || !strings.Contains(stdout, "attempt=attempt-1") {
		t.Fatalf("owner code=%d stdout=%q stderr=%q acquires=%d", code, stdout, stderr, acquires)
	}
}

func TestGateCadenceTickClassifiesOnlyTickRefusalsNonZero(t *testing.T) {
	root := t.TempDir()
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runGateCadenceTick([]string{"--root", root})
	})
	if code != 3 || !strings.Contains(stderr, cadenceOwnerUnconfigured) {
		t.Fatalf("unconfigured code=%d stderr=%q", code, stderr)
	}
	if got := (cadenceRefusal{cadenceFetchRefused, errors.New("fetch").Error()}).Error(); got != "CADENCE_FETCH_REFUSED: fetch" {
		t.Fatalf("fetch refusal=%q", got)
	}
	if got := (cadenceRefusal{cadenceLedgerUnreadable, errors.New("ledger").Error()}).Error(); got != "CADENCE_LEDGER_UNREADABLE: ledger" {
		t.Fatalf("ledger refusal=%q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+root+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalAcquire, originalTick := batchOwnerAcquire, cadenceTick
	t.Cleanup(func() { batchOwnerAcquire, cadenceTick = originalAcquire, originalTick })
	batchOwnerAcquire = func(string) (batchOwnerLease, error) { return batchOwnerLease{}, nil }
	cadenceTick = func(string, batchOwnerLease, func() time.Time) (cadenceTickOutput, error) {
		return cadenceTickOutput{}, errors.New("raw ledger failure")
	}
	code, _, stderr = captureCommandOutput(t, false, true, func() int { return runGateCadenceTick([]string{"--root", root}) })
	if code != 3 || !strings.Contains(stderr, cadenceLedgerUnreadable) || strings.Contains(stderr, "\nraw ledger failure") {
		t.Fatalf("raw tick failure code=%d stderr=%q", code, stderr)
	}
}

func TestCadencePreparationDoesNotRequireClaimedGoal(t *testing.T) {
	request := cadencePreparationRequest("landing", "tree")
	if request.Root != "landing" || request.Tree != "tree" || request.GoalID != "" || request.Purpose != testpolicy.PurposeCadence ||
		request.Mode != testpolicy.ModeDeep || !request.CadencePreflight {
		t.Fatalf("cadence preparation request=%+v", request)
	}
	accounts, err := testingPreparationAccountsToGoal(request)
	if err != nil || accounts {
		t.Fatalf("cadence preflight accounts-to-goal=%t err=%v", accounts, err)
	}
}

func TestCadenceResultNamesTriggerForJoinedAndOccupiedClaims(t *testing.T) {
	for _, outcome := range []goal.CadenceClaimOutcome{goal.CadenceClaimJoined, goal.CadenceClaimOccupied} {
		trigger, happened, attempt := cadenceResultWords(gaterun.CadenceTickResult{ClaimOutcome: outcome, Trigger: goal.CadenceTriggerWeightDue})
		if trigger != "weight" || happened != "joined" || attempt != "" {
			t.Fatalf("outcome=%s trigger=%q happened=%q attempt=%q", outcome, trigger, happened, attempt)
		}
	}
	trigger, happened, attempt := cadenceResultWords(gaterun.CadenceTickResult{ClaimOutcome: goal.CadenceClaimComplete,
		Status: &goal.CadenceStatus{Trigger: goal.CadenceTriggerForcedWindow, AttemptID: "attempt-terminal"}})
	if trigger != "six-hour" || happened != "terminal-read" || attempt != "attempt-terminal" {
		t.Fatalf("complete trigger=%q happened=%q attempt=%q", trigger, happened, attempt)
	}
}

func TestCadenceRunStoreRereadsLandingOwnerEpoch(t *testing.T) {
	originalRequire := batchOwnerRequire
	t.Cleanup(func() { batchOwnerRequire = originalRequire })
	live := true
	batchOwnerRequire = func(batchOwnerLease) error {
		if live {
			return nil
		}
		return errors.New("owner lease moved")
	}
	store := cadenceRunStore("landing-root", batchOwnerLease{epoch: 9}, func() time.Time { return time.Unix(1, 0) })
	if epoch, ok := store.CurrentEpoch(); !ok || epoch == nil || *epoch != 9 {
		t.Fatalf("live epoch=%v ok=%t", epoch, ok)
	}
	live = false
	if epoch, ok := store.CurrentEpoch(); ok || epoch != nil {
		t.Fatalf("stale epoch=%v ok=%t", epoch, ok)
	}
}

func TestCadenceRevalidationDefersMissingEngineBuildUntilClaimedRun(t *testing.T) {
	originalIdentity, originalRetained := cadenceBuildIdentity, cadenceRetainedEngineDigest
	t.Cleanup(func() {
		cadenceBuildIdentity, cadenceRetainedEngineDigest = originalIdentity, originalRetained
	})
	cadenceBuildIdentity = func(context.Context, gittree.Workspace, string, string, []string) (string, error) {
		return "build-identity", nil
	}
	cadenceRetainedEngineDigest = func(testingPreparation, []proofrun.Attempt, string, bool) (string, error) {
		return "", errors.New("no retained engine")
	}
	identity, digest, exact, err := cadenceCandidateEngineIdentity(testingPreparation{ProjectRoot: "root"}, gaterun.CadenceTrunk{Tree: strings.Repeat("a", 40)}, nil)
	if err != nil || identity != "build-identity" || exact || digest != bytesSHA256([]byte("cadence-missing-engine-evidence\x00build-identity")) {
		t.Fatalf("identity=%q digest=%q exact=%t err=%v", identity, digest, exact, err)
	}
}

func TestCadenceRevalidationDoesNotBuildBeforeClaim(t *testing.T) {
	data, err := os.ReadFile("gate_cadence.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(data), "func revalidateCadence(")
	end := strings.Index(string(data), "func cadencePreparationRequest(")
	if start < 0 || end <= start || strings.Contains(string(data[start:end]), "buildCandidateEngine(") {
		t.Fatal("cadence pre-claim revalidation invokes the candidate-engine build")
	}
}
