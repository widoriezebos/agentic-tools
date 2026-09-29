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
