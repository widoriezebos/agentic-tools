package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// reviveRepo: a real repository with an owned goal, a working notify
// channel, and a provably-dead worker set.
func reviveRepo(t *testing.T) string {
	root := gitRepoWithCurrentGoal(t)
	if out, err := gitConfig(root, "metasystem.steward.notify-command", "true"); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	return root
}

func TestFailedRevivalIsTheFirstPointThatEscalates(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-failed")); err != nil {
		t.Fatal(err)
	}
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-failed", func(Intent) error {
		return errors.New("dispatcher unavailable")
	})
	if err != nil || outcome.Launched || !outcome.Escalate || !strings.Contains(outcome.Reason, "dispatch failed") {
		t.Fatalf("only the ended recovery attempt should request escalation: %+v %v", outcome, err)
	}
}

func deadCensus() WorkerCensus {
	return fakeCensus{workers: Workers{CensusComplete: true}}
}

func TestRevivalLaunchesOnceBeforeAnyNotification(t *testing.T) {
	root := reviveRepo(t)
	it := testIntent("rev-1")
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), it); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-1", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("a dead worker must be healed once without a notification gate: %+v %v launched=%d", out, err, launched)
	}
	ev, _ := LoadEvidence(EvidencePath(root))
	if ev.DryRevivals != 1 {
		t.Fatalf("the launch counts against the dry cap: %+v", ev)
	}
	out, err = CompleteRevival(root, TickConfig{}, deadCensus(), "rev-1", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 1 {
		t.Fatalf("a consumed intent must never launch again: %+v launched=%d", out, launched)
	}
}

func TestConcurrentReviversShareOneConsumedIntent(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-race")); err != nil {
		t.Fatal(err)
	}
	type result struct {
		out ReviveOutcome
		err error
	}
	launchEntered := make(chan struct{})
	releaseLaunch := make(chan struct{})
	first := make(chan result, 1)
	second := make(chan result, 1)
	go func() {
		out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-race", func(Intent) error {
			close(launchEntered)
			<-releaseLaunch
			return nil
		})
		first <- result{out: out, err: err}
	}()
	<-launchEntered
	go func() {
		out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-race", func(Intent) error {
			t.Error("the losing reviver launched")
			return nil
		})
		second <- result{out: out, err: err}
	}()
	close(releaseLaunch)
	firstResult := <-first
	secondResult := <-second
	if firstResult.err != nil || !firstResult.out.Launched {
		t.Fatalf("the winner must launch once: %+v %v", firstResult.out, firstResult.err)
	}
	if secondResult.err != nil || secondResult.out.Launched || !strings.Contains(secondResult.out.Reason, "already consumed") {
		t.Fatalf("the loser must observe the consumed intent: %+v %v", secondResult.out, secondResult.err)
	}
}

func TestRevivalReapsAProvablyDeadContinuationBeforeComputingTheGuard(t *testing.T) {
	root := reviveRepo(t)
	dead := testIntent("rev-dead")
	dead.LaunchStamped = true
	consumedIntentOnDisk(t, root, dead)
	jobProcessRecordOnDisk(t, root, dead.JobId, "running", 99999999, 1, "")
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-after-ended")); err != nil {
		t.Fatal(err)
	}

	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-after-ended", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("a dead continuation must be reaped before the guard is computed: %+v %v launched=%d", out, err, launched)
	}
	if active, _ := ConsumedActive(root); len(active) != 1 || active[0].Nonce != "rev-after-ended" {
		t.Fatalf("only the newly launched continuation should remain active: %+v", active)
	}
}

func TestRevivalLeavesARunningContinuationOnTheGuard(t *testing.T) {
	root := reviveRepo(t)
	running := testIntent("rev-running")
	running.LaunchStamped = true
	consumedIntentOnDisk(t, root, running)
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("the test process identity is unavailable: %v %s", err, state)
	}
	jobProcessRecordOnDisk(t, root, running.JobId, "running", int64(os.Getpid()), exact.StartedAt.Unix(), "")
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-after-running")); err != nil {
		t.Fatal(err)
	}

	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-after-running", func(Intent) error {
		t.Fatal("a running continuation must suppress another launch")
		return nil
	})
	if err != nil || out.Launched || !strings.Contains(out.Reason, "continuation is already open and unreaped") {
		t.Fatalf("a running continuation must remain on the guard: %+v %v", out, err)
	}
}

func jobProcessRecordOnDisk(t *testing.T, root, jobId, status string, pid, started int64, tag string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"jobId": jobId, "status": status, "endedAt": "", "pid": pid,
		"pidStartedAt": started, "pgid": pid, "instanceTag": tag,
		"capDeadline": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, jobId+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRevivalLeavesAnUncertainContinuationOnTheGuard(t *testing.T) {
	root := reviveRepo(t)
	consumedIntentOnDisk(t, root, testIntent("rev-uncertain"))
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-after-uncertain")); err != nil {
		t.Fatal(err)
	}

	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-after-uncertain", func(Intent) error {
		t.Fatal("an uncertain continuation must suppress another launch")
		return nil
	})
	if err != nil || out.Launched || !strings.Contains(out.Reason, "continuation is already open and unreaped") {
		t.Fatalf("an uncertain continuation must remain on the guard: %+v %v", out, err)
	}
}

func TestNotifierOutageCannotBlockARevival(t *testing.T) {
	root := reviveRepo(t)
	if out, err := gitConfig(root, "metasystem.steward.notify-command", "exit 1"); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-2")); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-2", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("healing must proceed while the notifier is unavailable: %+v launched=%d", out, launched)
	}
	// The channel coming back cannot replay an already consumed repair.
	if cfgOut, err := gitConfig(root, "metasystem.steward.notify-command", "true"); err != nil {
		t.Fatalf("config: %v\n%s", err, cfgOut)
	}
	out, err = CompleteRevival(root, TickConfig{}, deadCensus(), "rev-2", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 1 {
		t.Fatalf("the completed repair must not replay when notification recovers: %+v launched=%d", out, launched)
	}
}

func TestEnrollmentAfterReservationCancelsTheRevival(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-3")); err != nil {
		t.Fatal(err)
	}
	// A worker enrolls between reservation and launch: the fence bumps.
	arb, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := BumpEnrollmentFence(root); err != nil {
		t.Fatal(err)
	}
	arb.Release()
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-3", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 0 || !strings.Contains(out.Reason, "enrolled after the reservation") {
		t.Fatalf("the fence must cancel the reservation: %+v launched=%d", out, launched)
	}
	if live, _ := LiveIntents(root); len(live) != 0 {
		t.Fatalf("a cancelled intent leaves the live set: %v", live)
	}
}

func TestAWorldThatTurnedLiveCancelsBeforeLaunch(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-4")); err != nil {
		t.Fatal(err)
	}
	liveNow := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, liveNow, "rev-4", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 0 || !strings.Contains(out.Reason, "world changed") {
		t.Fatalf("a live worker at re-arbitration must cancel: %+v launched=%d", out, launched)
	}
}

func TestSeatIdleIntentLaunchesAlthoughTheSeatMainIsAlive(t *testing.T) {
	root := reviveRepo(t)
	intent := testIntent("seat-idle-live")
	intent.Reason = "seatIdle"
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
		t.Fatal(err)
	}
	liveMain := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, liveMain, intent.Nonce, func(got Intent) error {
		launched++
		if got.Reason != "seatIdle" || got.Goal != "fix-it" {
			t.Fatalf("the launch did not consume the seat-idle intent: %+v", got)
		}
		return nil
	}, func(Intent) error {
		t.Fatal("the steward tried to claim this machine's already-held goal")
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("the explicit seat-idle handoff must bypass only live-main suppression: %+v %v launched=%d", out, err, launched)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("a successful steward handoff raised the human idle alarm: %+v %v", pending, pendingErr)
	}
}

func TestSeatIdleIntentClaimsAsRecordedActorThenLaunches(t *testing.T) {
	root := reviveRepo(t)
	writeLedger(t, root, "# Goals\n\n## Queued goal: next-work — Repair the next thing\n- Origin: main\n- Next step: Claim it.\n")
	intent := testIntent("seat-idle-claim")
	intent.Reason = "seatIdle"
	intent.Goal = "next-work"
	intent.ClaimNeeded = true
	intent.SeatActor = &SeatActor{Machine: "machine-a", Lineage: "main-lineage"}
	intent.SeatClaimEpoch = 19
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
		t.Fatal(err)
	}
	claimed := 0
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(got Intent) error {
		launched++
		if got.Goal != "next-work" || !got.ClaimNeeded {
			t.Fatalf("the launch lost the deferred claim record: %+v", got)
		}
		return nil
	}, func(got Intent) error {
		claimed++
		if got.SeatActor == nil || got.SeatActor.Machine != "machine-a" ||
			got.SeatActor.Lineage != "main-lineage" || got.SeatClaimEpoch != 19 {
			t.Fatalf("the claim did not use the actor and lease epoch recorded at Stop: %+v", got)
		}
		return nil
	})
	if err != nil || !out.Launched || claimed != 1 || launched != 1 {
		t.Fatalf("the steward did not claim then launch exactly once: out=%+v err=%v claimed=%d launched=%d", out, err, claimed, launched)
	}
}

func TestSeatIdleRefusedClaimRaisesHumanAlarmAndDoesNotLaunch(t *testing.T) {
	root := reviveRepo(t)
	writeLedger(t, root, "# Goals\n\n## Queued goal: next-work — Repair the next thing\n- Origin: main\n- Next step: Claim it.\n")
	intent := testIntent("seat-idle-refused")
	intent.Reason = "seatIdle"
	intent.Goal = "next-work"
	intent.ClaimNeeded = true
	intent.SeatActor = &SeatActor{Machine: "machine-a", Lineage: "main-lineage"}
	intent.SeatClaimEpoch = 19
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launched++
		return nil
	}, func(Intent) error {
		return errors.New("machine already holds another claim")
	})
	if err != nil || out.Launched || out.Escalate || launched != 0 ||
		!strings.Contains(out.Reason, "machine already holds another claim") {
		t.Fatalf("a refused claim did not stop before launch: out=%+v err=%v launched=%d", out, err, launched)
	}
	pending, pendingErr := PendingNotifications(root)
	if pendingErr != nil || len(pending) != 1 ||
		!strings.Contains(pending[0].Message, "machine already holds another claim") {
		t.Fatalf("the claim refusal was not carried by the human idle alarm: %+v %v", pending, pendingErr)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("the refused claim left a resumable launch authorization: %+v %v", live, liveErr)
	}
}

func TestSeatIdleIntentStillHonorsOneActiveContinuationGuard(t *testing.T) {
	root := reviveRepo(t)
	intent := testIntent("seat-idle-guarded")
	intent.Reason = "seatIdle"
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
		t.Fatal(err)
	}
	other := testIntent("other-live-intent")
	other.JobId = "other-job"
	if err := MintIntent(root, other); err != nil {
		t.Fatal(err)
	}
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		t.Fatal("the seat-idle intent launched through another live continuation")
		return nil
	})
	if err != nil || out.Launched || !strings.Contains(out.Reason, "continuation is already open and unreaped") {
		t.Fatalf("the one-active-continuation guard did not hold: %+v %v", out, err)
	}
}

func TestProviderOutageArrivingBeforeLaunchCancelsTheRevival(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("rev-outage")); err != nil {
		t.Fatal(err)
	}
	if _, err := outage.Record(root, "overloaded", "API Error: 529", "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-outage", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 0 || !strings.Contains(out.Reason, "provider is overloaded") {
		t.Fatalf("provider outage did not cancel before dispatch: outcome=%+v launched=%d err=%v", out, launched, err)
	}
	if live, err := LiveIntents(root); err != nil || len(live) != 0 {
		t.Fatalf("outage-cancelled intent remained live: intents=%+v err=%v", live, err)
	}
}

func TestResumableIntentReportsUnreadableStore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(intentsDir(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intentsDir(root), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResumableIntent(root); err == nil {
		t.Fatal("an unreadable live-intent store looked empty")
	}
}

func TestResumableIntentNamesAPreparedUnlaunchedRevival(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := ResumableIntent(root); err != nil || ok {
		t.Fatalf("an empty store resumes nothing: %v %v", ok, err)
	}
	if err := MintIntent(root, testIntent("rs-1")); err != nil {
		t.Fatal(err)
	}
	nonce, ok, err := ResumableIntent(root)
	if err != nil || !ok || nonce != "rs-1" {
		t.Fatalf("the prepared-but-unlaunched intent is the one to resume: %q %v %v", nonce, ok, err)
	}
}
