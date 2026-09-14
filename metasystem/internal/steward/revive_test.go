package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

type handoffProbeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f handoffProbeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return f(pid)
}

func handoffProbe(binding HandoffBinding, state identity.Liveness, includeTag bool, before func()) identity.Prober {
	return handoffProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if before != nil {
			before()
		}
		exact := identity.Exact{Pid: pid, StartedAt: time.Unix(binding.Predecessor.StartedAtSec, 0)}
		if includeTag {
			exact.Argv = []string{"fixture", binding.PredecessorTag}
			exact.ArgvKnown = true
		}
		return exact, state, nil
	})
}

func useHandoffProber(t *testing.T, prober identity.Prober) {
	t.Helper()
	previous := handoffProber
	handoffProber = prober
	t.Cleanup(func() { handoffProber = previous })
}

func stagedRevivalHandoff(t *testing.T, nonce string) (string, Intent) {
	t.Helper()
	root := stagedRepo(t)
	fixture := writeStagedHandoffFixture(t, root, nonce)
	intent, err := StageHandoffIntent(root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", fixture.binding)
	if err != nil {
		t.Fatal(err)
	}
	return root, intent
}

func prepareRevivalHandoff(t *testing.T, nonce string) (string, Intent) {
	t.Helper()
	root, intent := stagedRevivalHandoff(t, nonce)
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
		t.Fatal(err)
	}
	return root, intent
}

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

func TestDecideForRevivalHoldsUntilThePredecessorIsDead(t *testing.T) {
	t.Run("alive predecessor", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000001")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, true, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold ||
			!strings.Contains(decision.Reason, intent.Nonce) || !strings.Contains(decision.Reason, "pid 4242 is alive") {
			t.Fatalf("a live predecessor must hold with its identity visible: %+v %v", decision, err)
		}
	})

	t.Run("unknown predecessor", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000002")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Unknown, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || decision.Verdict != VerdictUnknown ||
			!strings.Contains(decision.Reason, intent.Nonce) || !strings.Contains(decision.Reason, "pid 4242 is unknown") {
			t.Fatalf("an unknown predecessor must hold instead of authorizing a launch: %+v %v", decision, err)
		}
	})

	t.Run("dead predecessor", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000003")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActRevive || !strings.Contains(decision.Reason, "observed-dead predecessor pid 4242") {
			t.Fatalf("only observed predecessor death may reach revival: %+v %v", decision, err)
		}
	})

	t.Run("live seat main after predecessor death", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000010")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		census := fakeCensus{workers: Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true}}
		decision, _, err := decideForRevival(root, TickConfig{}, census, Evidence{}, intent)
		if err != nil || decision.Action != ActHold || decision.Verdict != VerdictHealthy ||
			!strings.Contains(decision.Reason, "a live worker with recent progress") {
			t.Fatalf("a live seat main must hold a dead-predecessor handoff: %+v %v", decision, err)
		}
	})

	t.Run("uncertain worker census after predecessor death", func(t *testing.T) {
		cases := []struct {
			name    string
			nonce   string
			workers Workers
		}{
			{name: "incomplete", nonce: "3000000000000011", workers: Workers{}},
			{name: "untracked", nonce: "3000000000000012", workers: Workers{CensusComplete: true, Untracked: 1}},
			{name: "unprovable", nonce: "3000000000000013", workers: Workers{CensusComplete: true, Unprovable: 1}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root, intent := stagedRevivalHandoff(t, tc.nonce)
				useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
				decision, _, err := decideForRevival(root, TickConfig{}, fakeCensus{workers: tc.workers}, Evidence{}, intent)
				if err != nil || decision.Action != ActHold || decision.Verdict != VerdictUnknown ||
					!strings.Contains(decision.Reason, "death not provable") {
					t.Fatalf("worker-census doubt must hold a dead-predecessor handoff: %+v %v", decision, err)
				}
			})
		}
	})

	t.Run("matching identity with missing tag", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000004")
		useHandoffProber(t, handoffProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
			return identity.Exact{
				Pid: pid, StartedAt: time.Unix(intent.Handoff.Predecessor.StartedAtSec, 0),
				Argv: []string{"fixture", "some-other-tag"}, ArgvKnown: true,
			}, identity.Alive, nil
		}))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || decision.Verdict != VerdictUnknown ||
			!strings.Contains(decision.Reason, "pid 4242 is unknown") {
			t.Fatalf("a matching live identity without its recorded tag must hold: %+v %v", decision, err)
		}
	})

	t.Run("matching identity with unreadable argv", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000014")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || decision.Verdict != VerdictUnknown ||
			!strings.Contains(decision.Reason, "pid 4242 is unknown") {
			t.Fatalf("an unreadable tag cannot authorize a handoff launch: %+v %v", decision, err)
		}
	})

	t.Run("reused predecessor identity", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000015")
		useHandoffProber(t, handoffProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
			return identity.Exact{
				Pid: pid, StartedAt: time.Unix(intent.Handoff.Predecessor.StartedAtSec+1, 0),
				Argv: []string{"fixture", intent.Handoff.PredecessorTag}, ArgvKnown: true,
			}, identity.Alive, nil
		}))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActRevive {
			t.Fatalf("a reused pid must not be mistaken for the recorded predecessor: %+v %v", decision, err)
		}
	})

	t.Run("untagged identity does not require argv", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000005")
		intent.Handoff.PredecessorTag = ""
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || decision.Verdict != VerdictHealthy ||
			!strings.Contains(decision.Reason, "pid 4242 is alive") {
			t.Fatalf("an untagged matching identity stays alive without an argv read: %+v %v", decision, err)
		}
	})

	t.Run("another continuation", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000006")
		other := testIntent("other-continuation")
		other.JobId = "other-job"
		if err := MintIntent(root, other); err != nil {
			t.Fatal(err)
		}
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || !strings.Contains(decision.Reason, "another continuation is open and unreaped") {
			t.Fatalf("an existing continuation must keep the handoff staged: %+v %v", decision, err)
		}
	})

	t.Run("unreaped consumed continuation", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000c")
		other := testIntent("consumed-continuation")
		other.JobId = "consumed-job"
		consumedIntentOnDisk(t, root, other)
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || !strings.Contains(decision.Reason, "another continuation is open and unreaped") {
			t.Fatalf("an unreaped consumed continuation must keep the handoff staged: %+v %v", decision, err)
		}
	})

	t.Run("another continuation precedes dry cap", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000f")
		other := testIntent("other-before-dry-cap")
		other.JobId = "other-before-dry-cap-job"
		if err := MintIntent(root, other); err != nil {
			t.Fatal(err)
		}
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{MaxRevivals: 1}, deadCensus(), Evidence{DryRevivals: 1}, intent)
		if err != nil || decision.Action != ActHold || !strings.Contains(decision.Reason, "another continuation is open and unreaped") {
			t.Fatalf("an existing continuation must hold before the dry cap is considered: %+v %v", decision, err)
		}
	})

	t.Run("dry revival cap", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000007")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{MaxRevivals: 2}, deadCensus(), Evidence{DryRevivals: 2}, intent)
		if err != nil || decision.Action != ActNotify || !strings.Contains(decision.Reason, "2 revivals produced no progress") {
			t.Fatalf("the existing dry cap must terminate a dead-predecessor handoff: %+v %v", decision, err)
		}
	})

	t.Run("provider outage", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000008")
		if _, err := outage.Record(root, "overloaded", "API Error: 529", "test", time.Now()); err != nil {
			t.Fatal(err)
		}
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActHold || !strings.Contains(decision.Reason, "provider is overloaded") {
			t.Fatalf("an outage must hold rather than spend a launch: %+v %v", decision, err)
		}
	})

	t.Run("alive predecessor dominates later guards", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000d")
		if _, err := outage.Record(root, "overloaded", "API Error: 529", "test", time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := MintIntent(root, testIntent("other-while-predecessor-lives")); err != nil {
			t.Fatal(err)
		}
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, true, nil))
		decision, _, err := decideForRevival(root, TickConfig{MaxRevivals: 1}, deadCensus(), Evidence{DryRevivals: 1}, intent)
		if err != nil || decision.Action != ActHold || !strings.Contains(decision.Reason, "pid 4242 is alive") {
			t.Fatalf("no later guard may terminate a handoff before predecessor death: %+v %v", decision, err)
		}
	})

	t.Run("dry cap precedes outage after death", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000e")
		if _, err := outage.Record(root, "overloaded", "API Error: 529", "test", time.Now()); err != nil {
			t.Fatal(err)
		}
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{MaxRevivals: 1}, deadCensus(), Evidence{DryRevivals: 1}, intent)
		if err != nil || decision.Action != ActNotify || !strings.Contains(decision.Reason, "revivals produced no progress") {
			t.Fatalf("the existing dry cap must terminate even during an outage: %+v %v", decision, err)
		}
	})

	t.Run("goal no longer owned", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "3000000000000009")
		writeLedger(t, root, "# Goals\n\n## Current goal: another-goal — Repair something else\n- Origin: main\n- Next step: Repair it.\n")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err != nil || decision.Action != ActNotify || !strings.Contains(decision.Reason, "no longer names a goal claimed or landing") {
			t.Fatalf("a missing target must terminate the authorization: %+v %v", decision, err)
		}
	})

	t.Run("missing binding", func(t *testing.T) {
		root := reviveRepo(t)
		intent := testIntent("missing-handoff-binding")
		intent.Reason = seatHandoffReason
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err == nil || decision.Action == ActRevive || !strings.Contains(err.Error(), "carries no handoff binding") {
			t.Fatalf("a malformed seatHandoff must remain a refusal: %+v %v", decision, err)
		}
	})

	t.Run("invalid binding", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000a")
		intent.Handoff.StateDigest = "invalid"
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		decision, _, err := decideForRevival(root, TickConfig{}, deadCensus(), Evidence{}, intent)
		if err == nil || decision.Action == ActRevive || !strings.Contains(err.Error(), "invalid handoff binding") {
			t.Fatalf("an invalid binding must remain live for inspection and launch nothing: %+v %v", decision, err)
		}
	})

	t.Run("running goal job belongs to successor snapshot", func(t *testing.T) {
		root, intent := stagedRevivalHandoff(t, "300000000000000b")
		exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != identity.Alive {
			t.Fatalf("fixture process identity: %s %v", state, err)
		}
		jobProcessRecordOnDisk(t, root, "running-goal-job", "running", int64(os.Getpid()), exact.StartedAt.Unix(), "")
		useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
		liveJobCensus := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
		decision, _, err := decideForRevival(root, TickConfig{}, liveJobCensus, Evidence{}, intent)
		if err != nil || decision.Action != ActRevive {
			t.Fatalf("a goal job in the snapshot must not suppress its successor: %+v %v", decision, err)
		}
	})
}

func TestCompleteRevivalHoldsThenLaunchesAHandoff(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000001")
	previous := handoffProber
	defer func() { handoffProber = previous }()
	handoffProber = handoffProbe(*intent.Handoff, identity.Alive, true, nil)

	for attempt := 0; attempt < 2; attempt++ {
		outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
			t.Fatal("a live predecessor launched its successor")
			return nil
		})
		if err != nil || !outcome.Held || outcome.Launched || outcome.Escalate {
			t.Fatalf("held tick %d changed the launch authorization: %+v %v", attempt+1, outcome, err)
		}
	}
	if live, err := LiveIntents(root); err != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
		t.Fatalf("held ticks must preserve one live intent: %+v %v", live, err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 1 ||
		pending[0].Nonce != handoffNoticeNonce(intent.Nonce) || pending[0].Message != "steward: handoff "+intent.Nonce+" held: predecessor pid 4242 is alive" {
		t.Fatalf("repeated holds must replace one exact pending notice: %+v %v", pending, err)
	}
	if evidence, err := LoadEvidence(EvidencePath(root)); err != nil || evidence.DryRevivals != 0 {
		t.Fatalf("held ticks must not spend the dry revival cap: %+v %v", evidence, err)
	}

	handoffProber = handoffProbe(*intent.Handoff, identity.Dead, false, nil)
	type result struct {
		outcome ReviveOutcome
		err     error
	}
	launchEntered := make(chan struct{})
	releaseLaunch := make(chan struct{})
	results := make(chan result, 2)
	var launches atomic.Int32
	go func() {
		outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
			launches.Add(1)
			close(launchEntered)
			<-releaseLaunch
			return nil
		})
		results <- result{outcome: outcome, err: err}
	}()
	<-launchEntered
	go func() {
		outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
			launches.Add(1)
			return nil
		})
		results <- result{outcome: outcome, err: err}
	}()
	close(releaseLaunch)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || launches.Load() != 1 {
		t.Fatalf("competing revivers must share one launch: first=%+v second=%+v launches=%d", first, second, launches.Load())
	}
	launched := 0
	alreadyConsumed := 0
	for _, got := range []ReviveOutcome{first.outcome, second.outcome} {
		if got.Launched {
			launched++
		}
		if strings.Contains(got.Reason, "already consumed") {
			alreadyConsumed++
		}
	}
	if launched != 1 || alreadyConsumed != 1 {
		t.Fatalf("one reviver must launch and one must observe consumption: first=%+v second=%+v", first.outcome, second.outcome)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("a launched handoff must leave no stale hold notice: %+v %v", pending, err)
	}
	if evidence, err := LoadEvidence(EvidencePath(root)); err != nil || evidence.DryRevivals != 1 {
		t.Fatalf("only the irreversible launch spends one dry revival: %+v %v", evidence, err)
	}
}

func TestHandoffLaunchTreatsAMissingHoldNoticeAsClean(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000002")
	useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err != nil || !outcome.Launched || outcome.Held || launches != 1 {
		t.Fatalf("absence of an already-delivered notice must not block launch: %+v %v launches=%d", outcome, err, launches)
	}
}

func TestCancellingAHeldHandoffClearsItsNotice(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000003")
	useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, true, nil))
	if outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !outcome.Held {
		t.Fatalf("fixture handoff did not hold: %+v %v", outcome, err)
	}
	if err := CancelIntent(root, intent.Nonce, "superseded by 4000000000000004"); err != nil {
		t.Fatal(err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("cancellation or superseding must clear the old hold notice: %+v %v", pending, err)
	}
	if _, err := os.Stat(filepath.Join(cancelledDir(root), intent.Nonce+".json")); err != nil {
		t.Fatalf("the recoverable cancellation tombstone is missing: %v", err)
	}
}

func TestEnrollmentAfterReservationCancelsHeldHandoff(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000007")
	useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Alive, true, nil))
	if outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !outcome.Held {
		t.Fatalf("fixture handoff did not hold: %+v %v", outcome, err)
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := BumpEnrollmentFence(root); err != nil {
		arbitration.Release()
		t.Fatal(err)
	}
	arbitration.Release()
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err != nil || outcome.Held || outcome.Launched || launches != 0 || !strings.Contains(outcome.Reason, "enrolled after the reservation") {
		t.Fatalf("the enrollment fence must retain precedence over a hold: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("the changed enrollment fence left a live authorization: %+v %v", live, liveErr)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("fence cancellation left a stale handoff notice: %+v %v", pending, pendingErr)
	}
}

func TestMissingHandoffTargetCancelsThroughTheTerminalPath(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000008")
	intent.Handoff.StateDigest = "invalid"
	if err := UpdateIntent(root, intent); err != nil {
		t.Fatal(err)
	}
	if err := QueueNotification(root, PendingNotification{
		Nonce: handoffNoticeNonce(intent.Nonce), Message: "steward: old hold",
	}); err != nil {
		t.Fatal(err)
	}
	writeLedger(t, root, "# Goals\n\n## Current goal: another-goal — Repair something else\n- Origin: main\n- Next step: Repair it.\n")
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err != nil || outcome.Held || outcome.Launched || launches != 0 || !strings.Contains(outcome.Reason, "world changed before launch") {
		t.Fatalf("a missing target must cancel without launch: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("a missing target left a resumable authorization: %+v %v", live, liveErr)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("terminal cancellation left a stale handoff notice: %+v %v", pending, pendingErr)
	}
	data, readErr := os.ReadFile(filepath.Join(cancelledDir(root), intent.Nonce+".json"))
	if readErr != nil || !strings.Contains(string(data), "no longer names a goal claimed or landing") {
		t.Fatalf("the recoverable tombstone lost the exact cancellation cause: %q %v", data, readErr)
	}
}

func TestMissingHandoffStateFileCancelsThroughTheTerminalPath(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "400000000000000b")
	useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, nil))
	if err := QueueNotification(root, PendingNotification{
		Nonce: handoffNoticeNonce(intent.Nonce), Message: "steward: old hold",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(intent.Handoff.StatePath); err != nil {
		t.Fatal(err)
	}
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err != nil || outcome.Held || outcome.Launched || launches != 0 ||
		!strings.Contains(outcome.Reason, "world changed before launch") ||
		!strings.Contains(outcome.Reason, "state file is missing") {
		t.Fatalf("a missing state file must cancel without launch: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("a missing state file left a resumable authorization: %+v %v", live, liveErr)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("terminal cancellation left a stale handoff notice: %+v %v", pending, pendingErr)
	}
	data, readErr := os.ReadFile(filepath.Join(cancelledDir(root), intent.Nonce+".json"))
	if readErr != nil || !strings.Contains(string(data), "state file is missing") {
		t.Fatalf("the recoverable tombstone lost the missing-state cause: %q %v", data, readErr)
	}
}

func TestUnreadableHandoffGoalRefusesWithoutCancelling(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000009")
	writeLedger(t, root, "# Goals\n\n## Current goal: malformed — Repair it\n- Unknown field: refuse\n")
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err == nil || outcome.Held || outcome.Launched || launches != 0 || !strings.Contains(err.Error(), "goal could not be re-read") {
		t.Fatalf("an unreadable goal authority must refuse without guessing: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
		t.Fatalf("an unreadable authority must preserve the staged handoff: %+v %v", live, liveErr)
	}
	if _, statErr := os.Stat(filepath.Join(cancelledDir(root), intent.Nonce+".json")); !os.IsNotExist(statErr) {
		t.Fatalf("an unreadable authority manufactured a cancellation tombstone: %v", statErr)
	}
}

func TestHandoffCancellationReportsNoticeCleanupFailure(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000005")
	path := filepath.Join(pendingDir(root), handoffNoticeNonce(intent.Nonce)+".json")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "obstruction"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeLedger(t, root, "# Goals\n\n## Current goal: another-goal — Repair something else\n- Origin: main\n- Next step: Repair it.\n")
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		t.Fatal("a missing target launched")
		return nil
	})
	if err == nil || outcome.Launched || !strings.Contains(err.Error(), "was cancelled, but its handoff notice could not be cleared") {
		t.Fatalf("cleanup damage must be reported after the safe cancellation: %+v %v", outcome, err)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("cleanup failure must not restore launch authority: %+v %v", live, liveErr)
	}
	if _, statErr := os.Stat(filepath.Join(cancelledDir(root), intent.Nonce+".json")); statErr != nil {
		t.Fatalf("cleanup failure lost the cancellation tombstone: %v", statErr)
	}
}

func TestFailedHandoffTombstoneReportsPartialCancellation(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "400000000000000a")
	if err := QueueNotification(root, PendingNotification{
		Nonce: handoffNoticeNonce(intent.Nonce), Message: "steward: held",
	}); err != nil {
		t.Fatal(err)
	}
	temporaryTarget := filepath.Join(cancelledDir(root), intent.Nonce+".json.tmp")
	if err := os.MkdirAll(temporaryTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temporaryTarget, "obstruction"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CancelIntent(root, intent.Nonce, "authorized test cancellation")
	if err == nil || !strings.Contains(err.Error(), "authorization was cancelled, but its tombstone failed") {
		t.Fatalf("the partial cancellation must report its exact safe outcome: %v", err)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
		t.Fatalf("a failed tombstone must not restore launch authority: %+v %v", live, liveErr)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("a failed tombstone left a stale hold notice: %+v %v", pending, pendingErr)
	}
	if _, statErr := os.Stat(intent.Handoff.StatePath); statErr != nil {
		t.Fatalf("the immutable recovery state was lost with the tombstone: %v", statErr)
	}
}

func TestHandoffHoldsOnTheFinalOutageCheck(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "4000000000000006")
	var recordErr error
	var observations atomic.Int32
	useHandoffProber(t, handoffProbe(*intent.Handoff, identity.Dead, false, func() {
		if observations.Add(1) == 1 {
			_, recordErr = outage.Record(root, "overloaded", "API Error: 529", "test", time.Now())
		}
	}))
	launches := 0
	outcome, err := CompleteRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if recordErr != nil {
		t.Fatal(recordErr)
	}
	if err != nil || !outcome.Held || outcome.Launched || launches != 0 || !strings.Contains(outcome.Reason, "became overloaded before launch") {
		t.Fatalf("an outage at the final check must keep the handoff live: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
		t.Fatalf("the final outage check cancelled the handoff: %+v %v", live, liveErr)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 1 || pending[0].Nonce != handoffNoticeNonce(intent.Nonce) {
		t.Fatalf("the final outage hold did not leave one notice: %+v %v", pending, pendingErr)
	}
	if evidence, evidenceErr := LoadEvidence(EvidencePath(root)); evidenceErr != nil || evidence.DryRevivals != 0 {
		t.Fatalf("the final outage hold spent a launch: %+v %v", evidence, evidenceErr)
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
