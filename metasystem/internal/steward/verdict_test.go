package steward

import "testing"

func base() Snapshot {
	return Snapshot{
		Work:               WorkOwned,
		Workers:            Workers{CensusComplete: true},
		TicksSinceProgress: 0,
		StaleTicks:         5,
		DryRevivals:        0,
		MaxRevivals:        3,
	}
}

func TestDeadAfterFreshCommitRevives(t *testing.T) {
	s := base() // zero ticks since progress: the commit just landed
	d := Decide(s)
	if d.Verdict != VerdictStalledDead || d.Action != ActRevive {
		t.Fatalf("proven death ignores staleness: %+v", d)
	}
}

func TestLiveUnannouncedMainIsNotDead(t *testing.T) {
	s := base()
	s.Workers = Workers{Untracked: 1, CensusComplete: true}
	s.TicksSinceProgress = 99
	d := Decide(s)
	if d.Verdict != VerdictUnknown || d.Action != ActNotify {
		t.Fatalf("an untracked live process must prevent a death proof: %+v", d)
	}
}

func TestEmptyEnrollmentWithoutCompleteCensusIsNotDead(t *testing.T) {
	s := base()
	s.Workers = Workers{CensusComplete: false}
	d := Decide(s)
	if d.Verdict != VerdictUnknown || d.Action != ActNotify {
		t.Fatalf("an empty store without a completed scan proves nothing: %+v", d)
	}
}

func TestLivePlusDeadAggregatesLive(t *testing.T) {
	s := base()
	s.Workers = Workers{Live: 1, Unprovable: 2, CensusComplete: true}
	d := Decide(s)
	if d.Verdict != VerdictHealthy {
		t.Fatalf("any owned live worker means LIVE: %+v", d)
	}
}

func TestUnknownDominatesDead(t *testing.T) {
	s := base()
	s.Workers = Workers{Unprovable: 1, CensusComplete: true}
	d := Decide(s)
	if d.Verdict != VerdictUnknown || d.Action == ActRevive {
		t.Fatalf("an unprovable record must block revival: %+v", d)
	}
}

func TestLiveGatePreventsRevival(t *testing.T) {
	s := base()
	s.Workers = Workers{Live: 1, CensusComplete: true} // a live gate counts as live
	s.TicksSinceProgress = 2
	if d := Decide(s); d.Action != ActNone {
		t.Fatalf("a live gate inside the threshold is healthy: %+v", d)
	}
}

func TestLiveQuietInsideThresholdUntouched(t *testing.T) {
	s := base()
	s.Workers.Live = 1
	s.TicksSinceProgress = 4 // threshold 5
	if d := Decide(s); d.Verdict != VerdictHealthy {
		t.Fatalf("lawful long reasoning must not alarm: %+v", d)
	}
}

func TestLiveIdlePastThresholdNotifiesNeverRevives(t *testing.T) {
	s := base()
	s.Workers.Live = 1
	s.TicksSinceProgress = 5
	d := Decide(s)
	if d.Verdict != VerdictStalledIdle || d.Action != ActNotify {
		t.Fatalf("live-idle is notify-only in v1: %+v", d)
	}
}

func TestThresholdBoundaryIsStrict(t *testing.T) {
	s := base()
	s.Workers.Live = 1
	s.TicksSinceProgress = 4
	if Decide(s).Verdict != VerdictHealthy {
		t.Fatal("tick 4 of 5 is inside the threshold")
	}
	s.TicksSinceProgress = 5
	if Decide(s).Verdict != VerdictStalledIdle {
		t.Fatal("tick 5 of 5 crosses it")
	}
}

func TestOneActiveContinuationSuppressesDispatch(t *testing.T) {
	s := base()
	s.ActiveContinuation = true
	d := Decide(s)
	if d.Action != ActNotify {
		t.Fatalf("an open continuation must suppress redispatch: %+v", d)
	}
}

func TestDryCountCapSwitchesToNotifyOnly(t *testing.T) {
	s := base()
	s.DryRevivals = 3
	d := Decide(s)
	if d.Action != ActNotify {
		t.Fatalf("three dry revivals must stop the spawning: %+v", d)
	}
	s.DryRevivals = 2
	if Decide(s).Action != ActRevive {
		t.Fatal("under the cap, proven death still revives")
	}
}

func TestDegradedLedgerNotifiesNeverGuesses(t *testing.T) {
	s := base()
	s.Work = WorkDegraded
	d := Decide(s)
	if d.Verdict != VerdictDegraded || d.Action != ActNotify {
		t.Fatalf("unreadable state must notify, never no-work: %+v", d)
	}
}

func TestNoWorkIsQuiet(t *testing.T) {
	s := base()
	s.Work = WorkNone
	if d := Decide(s); d.Verdict != VerdictNoWork || d.Action != ActNone {
		t.Fatalf("a goal-free repository needs nothing: %+v", d)
	}
}
