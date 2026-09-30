package landpath

import (
	"errors"
	"testing"
)

// The staged route's release set (disk-lifetimes Part B 3.6, U6d): the
// goal's set is recorded for the commit about to be pushed, before the
// push, and released only once the push succeeded; a refused push releases
// nothing; a landing in no goal's name records nothing.
func TestTheStagedFormRecordsItsReleaseSetBeforeThePushAndReleasesAfter(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	var recorded, released []string
	b.owners.RecordRelease = func(commit, branch string) error {
		recorded = append(recorded, commit+"@"+branch)
		b.log.add("record release %s", commit)
		return nil
	}
	b.owners.ReleaseLanded = func(commit string) { released = append(released, commit); b.log.add("release %s", commit) }
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 0)
	if len(recorded) != 1 || len(released) != 1 || recorded[0] != released[0]+"@main" || len(b.git.called("push")) != 1 {
		t.Fatalf("recorded=%v released=%v pushes=%d", recorded, released, len(b.git.called("push")))
	}

	refused := newBed(t)
	driverPathMode(refused)
	gateAnswers(refused, errors.New(gateHeld))
	var refusedReleases []string
	refused.owners.RecordRelease = func(string, string) error { return nil }
	refused.owners.ReleaseLanded = func(commit string) { refusedReleases = append(refusedReleases, commit) }
	refused.expect(refused.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 1, gateHeld)
	if len(refusedReleases) != 0 {
		t.Fatalf("a landing that never pushed released %v", refusedReleases)
	}

	plain := newBed(t)
	driverPathMode(plain)
	plain.owners.RecordRelease = func(string, string) error { t.Fatal("a landing in no goal's name recorded a set"); return nil }
	plain.owners.ReleaseLanded = func(string) { t.Fatal("a landing in no goal's name released") }
	plain.expect(plain.land(LandRequest{Pathspecs: []string{"payload.txt"}, SkipTransport: true}), 0)
}

// The exception route's release set (disk-lifetimes Part B 3.6, U6d): the
// carried form's single push is preceded by the record of the commit about
// to be pushed and followed, once the push and the goal record succeeded,
// by the release; a carried landing the gate stops records nothing it
// could release and releases nothing; a crash after the push releases
// nothing in the process (the route's recovery and the sweeper finish it).
func TestTheCarriedFormRecordsItsReleaseBeforeThePushAndReleasesAfter(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	var recorded, released []string
	c.owners.RecordRelease = func(commit, branch string) error {
		if c.pushes() != 0 {
			t.Fatalf("the release was recorded after the push")
		}
		recorded = append(recorded, commit+"@"+branch)
		c.log.add("record release %s", commit)
		return nil
	}
	c.owners.ReleaseLanded = func(commit string) { released = append(released, commit); c.log.add("release %s", commit) }
	status, killed := c.landCarried(carriedRequest())
	if killed != "" {
		t.Fatalf("killed at %s", killed)
	}
	c.expect(status, 0)
	pushed := c.git.head
	if len(recorded) != 1 || recorded[0] != pushed+"@main" || len(released) != 1 || released[0] != pushed || c.pushes() != 1 {
		t.Fatalf("recorded=%v released=%v head=%s pushes=%d", recorded, released, pushed, c.pushes())
	}
	c.inOrder("intent carrying=row-1", "record release "+pushed, "carried entry=entry-1", "release "+pushed)

	held := newCarriedBed(t)
	gateAnswers(held.bed, errors.New(gateHeld))
	var heldReleases []string
	held.owners.RecordRelease = func(string, string) error { t.Fatal("a stopped carried landing recorded its release"); return nil }
	held.owners.ReleaseLanded = func(commit string) { heldReleases = append(heldReleases, commit) }
	status, _ = held.landCarried(carriedRequest())
	held.expect(status, 1, gateHeld)
	if len(heldReleases) != 0 || held.pushes() != 0 {
		t.Fatalf("a carried landing that never pushed released %v", heldReleases)
	}

	crashed := newCarriedBed(t)
	crashed.crashAt = "after-push"
	var crashRecorded, crashReleased []string
	crashed.owners.RecordRelease = func(commit, _ string) error { crashRecorded = append(crashRecorded, commit); return nil }
	crashed.owners.ReleaseLanded = func(commit string) { crashReleased = append(crashReleased, commit) }
	status, _ = crashed.landCarried(carriedRequest())
	crashed.expect(status, 143)
	if len(crashRecorded) != 1 || len(crashReleased) != 0 || crashed.pushes() != 1 {
		t.Fatalf("crash after the push: recorded=%v released=%v pushes=%d", crashRecorded, crashReleased, crashed.pushes())
	}
}
