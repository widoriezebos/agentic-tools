package batch

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A batch member's release set is recorded at join and run by the batch's
// own P6 step (disk-lifetimes Part B 3.6, U6d): nothing is released before
// the pushed series is recognized; a release cut short is finished by the
// next recovery, which touches only the recorded ids; a finished set is
// never run again.
func TestBatchP6ReleasesTheRecordedSetOnlyAfterThePush(t *testing.T) {
	_, store := landingBed(t)
	must(t, store.Update(testBatchID, func(next *Record) error {
		next.Units[0].ReleaseSet = &diskstore.ReleaseSet{Tip: "tip-a", Stores: []diskstore.ReleaseEntry{{ID: "01K6WORKSPACE0000000000000", State: diskstore.ReleasePending}}}
		return nil
	}))
	calls := 0
	var seen []string
	release := func(unit Unit, set *diskstore.ReleaseSet) {
		calls++
		for index := range set.Stores {
			seen = append(seen, set.Stores[index].ID)
			if calls > 1 {
				set.Stores[index].State = diskstore.ReleaseReleased
			}
		}
	}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		Finalize:     func(Unit, string) error { return nil },
		Rearm:        func(string) error { return nil },
		Release:      release,
	}
	if err := RecoverPushedSeries(store, testBatchID, "owner", time.Unix(4, 0), seams); err == nil || calls != 0 {
		t.Fatalf("before the push: err=%v releases=%d", err, calls)
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&[]string{})))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), seams))
	if record := load(t, store); calls != 1 || !record.Units[0].P6Done || record.Units[0].ReleaseSet.Finished() {
		t.Fatalf("a release cut short stays unfinished: calls=%d unit=%+v", calls, record.Units[0])
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(6, 0), seams))
	record := load(t, store)
	if calls != 2 || !record.Units[0].ReleaseSet.Finished() || record.Units[0].ReleaseSet.Stores[0].State != diskstore.ReleaseReleased {
		t.Fatalf("the next recovery finishes the set: calls=%d unit=%+v", calls, record.Units[0])
	}
	for _, id := range seen {
		if id != "01K6WORKSPACE0000000000000" {
			t.Fatalf("a retry touched an id the set never recorded: %v", seen)
		}
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(7, 0), seams))
	if calls != 2 {
		t.Fatalf("a finished set runs again: calls=%d", calls)
	}
	if record.Units[1].ReleaseSet != nil {
		t.Fatalf("a member with no recorded set gained one: %+v", record.Units[1])
	}
}

// Round D3 N6: the sweeper's retry finishes a landed member's unfinished
// set through the same step, touching only recorded ids, and leaves a
// member P6 has not reached alone.
func TestRetryReleaseSetsFinishesOnlyLandedMembersSets(t *testing.T) {
	_, store := landingBed(t)
	pending := func() *diskstore.ReleaseSet {
		return &diskstore.ReleaseSet{Tip: "t", Stores: []diskstore.ReleaseEntry{{ID: "01K6WORKSPACE0000000000000", State: diskstore.ReleasePending}}}
	}
	must(t, store.Update(testBatchID, func(next *Record) error {
		next.Units[0].ReleaseSet, next.Units[0].P6Done = pending(), true
		next.Units[1].ReleaseSet = pending()
		return nil
	}))
	var released []string
	must(t, RetryReleaseSets(store, testBatchID, func(unit Unit, set *diskstore.ReleaseSet) {
		released = append(released, unit.GoalID)
		set.Stores[0].State = diskstore.ReleaseReleased
	}))
	record := load(t, store)
	if len(released) != 1 || released[0] != record.Units[0].GoalID || !record.Units[0].ReleaseSet.Finished() || record.Units[1].ReleaseSet.Finished() {
		t.Fatalf("released %v, units %+v", released, record.Units)
	}
	must(t, RetryReleaseSets(store, testBatchID, func(unit Unit, _ *diskstore.ReleaseSet) { released = append(released, "again:"+unit.GoalID) }))
	if len(released) != 1 {
		t.Fatalf("a finished set ran again: %v", released)
	}
}
