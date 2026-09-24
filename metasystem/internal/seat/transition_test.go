package seat

import (
	"strings"
	"testing"
	"time"
)

func standingsAt(machine string, at time.Time, now time.Time, claims []string) []MachineStanding {
	return Fleet(FleetInput{
		This:   "m1e",
		Copy:   Copy{Records: map[string]Record{machine: presence(machine, at, 600)}},
		Claims: map[string][]string{machine: claims},
		Now:    now, Window: 30 * time.Minute,
	})
}

func TestTheFirstReadAfterArmingNotifiesNothing(t *testing.T) {
	t.Parallel()
	standings := standingsAt("m1c", fixtureClock.Add(-6*time.Hour), fixtureClock, nil)
	if queued := Transitions(nil, standings, true); len(queued) != 0 {
		t.Fatalf("the baseline read queued %v", queued)
	}
}

func TestTheTransitionCheckFiresOncePerChange(t *testing.T) {
	t.Parallel()
	silent := standingsAt("m1c", fixtureClock.Add(-6*time.Hour), fixtureClock,
		[]string{"tests-parallel-and-deterministic", "goal-b"})
	previous := map[string]Observation{"m1c": {Standing: Reachable, Since: FormatTime(fixtureClock.Add(-8 * time.Hour))}}
	queued := Transitions(previous, silent, false)
	if len(queued) != 1 {
		t.Fatalf("queued = %+v; want one notification", queued)
	}
	if !strings.Contains(queued[0].Message, "m1c has been unreachable since 09:40 and holds 2 goals: tests-parallel-and-deterministic, goal-b") {
		t.Fatalf("message = %q", queued[0].Message)
	}
	if queued[0].Nonce != NotificationNonce("m1c", Unreachable, silent[len(silent)-1].Since) {
		t.Fatalf("nonce = %q", queued[0].Nonce)
	}
	// The same standing at the next tick is not news again.
	settled := Observations(silent)
	if again := Transitions(settled, silent, false); len(again) != 0 {
		t.Fatalf("an unchanged standing queued %+v", again)
	}
	// Recovery is one notification of its own.
	back := standingsAt("m1c", fixtureClock.Add(time.Hour-time.Minute), fixtureClock.Add(time.Hour), nil)
	recovered := Transitions(settled, back, false)
	if len(recovered) != 1 || recovered[0].Message != "m1c is reachable again" {
		t.Fatalf("recovery queued %+v", recovered)
	}
}

func TestANonceIsAlwaysAPlainFileName(t *testing.T) {
	t.Parallel()
	nonce := NotificationNonce("m1c", Unreachable, FormatTime(fixtureClock))
	if strings.ContainsAny(nonce, "/\\ ") || strings.Contains(nonce, "..") {
		t.Fatalf("nonce = %q; it is a file name", nonce)
	}
	if !strings.HasPrefix(nonce, "seat-presence-m1c-unreachable-") {
		t.Fatalf("nonce = %q", nonce)
	}
}

func TestThisMachineIsNotItsOwnPeerNotification(t *testing.T) {
	t.Parallel()
	mine := Fleet(FleetInput{
		This: "m1e",
		Copy: Copy{Records: map[string]Record{"m1e": presence("m1e", fixtureClock.Add(-6*time.Hour), 600)}},
		Now:  fixtureClock, Window: 30 * time.Minute,
	})
	previous := map[string]Observation{"m1e": {Standing: Reachable, Since: FormatTime(fixtureClock.Add(-8 * time.Hour))}}
	if queued := Transitions(previous, mine, false); len(queued) != 0 {
		t.Fatalf("this machine notified about itself: %+v", queued)
	}
}

func TestAMachineFirstSeenReachableIsNoNews(t *testing.T) {
	t.Parallel()
	fresh := standingsAt("m1c", fixtureClock.Add(-time.Minute), fixtureClock, nil)
	if queued := Transitions(map[string]Observation{}, fresh, false); len(queued) != 0 {
		t.Fatalf("a newly seen reachable machine queued %+v", queued)
	}
	silent := standingsAt("m1d", fixtureClock.Add(-6*time.Hour), fixtureClock, nil)
	if queued := Transitions(map[string]Observation{}, silent, false); len(queued) != 1 {
		t.Fatalf("a newly seen silent machine queued %+v; want one", queued)
	}
}
