package steward

import (
	"strings"
	"testing"
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

func deadCensus() WorkerCensus {
	return fakeCensus{workers: Workers{CensusComplete: true}}
}

func TestRevivalLaunchesOnceThroughTheFullGate(t *testing.T) {
	root := reviveRepo(t)
	it := testIntent("rev-1")
	if err := PrepareIntent(root, it); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-1", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("a dead worker with a delivered notification launches once: %+v %v launched=%d", out, err, launched)
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

func TestNotifierOutageGatesTheLaunchUntilDeliverySucceeds(t *testing.T) {
	root := reviveRepo(t)
	if out, err := gitConfig(root, "metasystem.steward.notify-command", "exit 1"); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if err := PrepareIntent(root, testIntent("rev-2")); err != nil {
		t.Fatal(err)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "rev-2", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || out.Launched || launched != 0 || !strings.Contains(out.Reason, "not delivered") {
		t.Fatalf("no launch without delivery: %+v launched=%d", out, launched)
	}
	// The channel comes back: the SAME intent completes, exactly once.
	if cfgOut, err := gitConfig(root, "metasystem.steward.notify-command", "true"); err != nil {
		t.Fatalf("config: %v\n%s", err, cfgOut)
	}
	out, err = CompleteRevival(root, TickConfig{}, deadCensus(), "rev-2", func(Intent) error {
		launched++
		return nil
	})
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("delayed delivery completes the one launch: %+v launched=%d", out, launched)
	}
}

func TestEnrollmentAfterReservationCancelsTheRevival(t *testing.T) {
	root := reviveRepo(t)
	if err := PrepareIntent(root, testIntent("rev-3")); err != nil {
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
	if err := PrepareIntent(root, testIntent("rev-4")); err != nil {
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
