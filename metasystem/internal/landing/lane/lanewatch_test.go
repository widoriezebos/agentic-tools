package lane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seatPush commits name on main in the seat clone, with message, and
// pushes it to origin past the lane (a seat landing its own work, or code
// run under the lane's account).
func (bed *publishBed) seatPush(t *testing.T, name, message string) string {
	t.Helper()
	bed.git(t, bed.seat, "fetch", "--quiet", "origin")
	bed.git(t, bed.seat, "checkout", "--quiet", "--detach", "origin/main")
	writeFile(t, filepath.Join(bed.seat, name), name+"\n")
	bed.git(t, bed.seat, "add", name)
	bed.git(t, bed.seat, "commit", "--quiet", "-m", message)
	commit := bed.git(t, bed.seat, "rev-parse", "HEAD")
	bed.git(t, bed.seat, "push", "--quiet", "origin", commit+":"+MainRef)
	return commit
}

func (bed *publishBed) watch(clock *time.Time) LaneWatch {
	return LaneWatch{Home: bed.home, Self: bed.checkout, Now: func() time.Time { return *clock }}
}

func watchHits(t *testing.T, home string) []Hit {
	t.Helper()
	store, err := ReadStopLoss(home)
	if err != nil {
		t.Fatal(err)
	}
	var hits []Hit
	for _, hit := range store.Hits {
		if hit.Kind == HitWatch {
			hits = append(hits, hit)
		}
	}
	return hits
}

// TestNoLaneNoDetector (K10, §1): with no lane registered there is no
// detector: a commit carrying the lane's trailers on main is not judged,
// nothing is written and nothing is paused.
func TestNoLaneNoDetector(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	// A computer that never registered a lane.
	bed.home = filepath.Join(t.TempDir(), "home")
	clock := laneNow
	bed.seatPush(t, "trailed.txt", "a lane-looking commit\n\n"+LandingProofTrailer+": b-1 attempt a-1")
	if line := bed.watch(&clock).Step(); line != "" {
		t.Fatalf("no lane: the watch said %q", line)
	}
	if _, err := os.Stat(HostDir(bed.home)); !os.IsNotExist(err) {
		t.Fatalf("no lane: the watch wrote host state (%v)", err)
	}
}

// TestNoLaneModeNeverFlags (K10, §1, rehearsal step 10): after a lane is
// unset the seats land their own work, with or without trailers, and
// nothing is ever flagged; a lane registered again starts from the main it
// finds, and the work landed meanwhile is not judged.
func TestNoLaneModeNeverFlags(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	clock := laneNow
	if line := bed.watch(&clock).Step(); line != "" {
		t.Fatalf("the first look: %q", line)
	}
	if err := removeLane(bed.home); err != nil {
		t.Fatal(err)
	}
	bed.seatPush(t, "own.txt", "a seat lands its own work")
	bed.seatPush(t, "trailed.txt", "a seat's commit that names the lane\n\n"+LandingProofTrailer+": b-1 attempt a-1")
	if line := bed.watch(&clock).Step(); line != "" {
		t.Fatalf("no lane: the watch said %q", line)
	}
	register(t, bed.home, bed.checkout)
	if line := bed.watch(&clock).Step(); line != "" {
		t.Fatalf("a lane registered again judged the no-lane work: %q", line)
	}
	if _, paused := ReadPause(bed.home); paused || len(watchHits(t, bed.home)) != 0 {
		t.Fatalf("no-lane mode was flagged: hits %+v", watchHits(t, bed.home))
	}
}

// TestLaneTrailedPushWithoutPublicationPausesTheLane (K10, K3 row,
// rehearsal step 8): what the lane publishes is never flagged; a commit
// carrying the lane's trailers that no publication pushed, and a rewound
// main, each pause the lane and owe an alert; a seat's own commit without
// the lane's trailers is not judged.
func TestLaneTrailedPushWithoutPublicationPausesTheLane(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	clock := laneNow
	watch := bed.watch(&clock)
	var alerted []Hit
	watch.Alert = func(root string, hit Hit) error { alerted = append(alerted, hit); return nil }
	watch.Step()

	old := bed.main(t)
	head := bed.commit(t, bed.checkout, old, "landed.txt")
	trailed, err := TrailedSeries(bed.checkout, old, head, ProofValue("batch-1", "attempt-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Publish(bed.home, bed.tuple(t, old, trailed), OpPublish, AuthorityAgent); err != nil {
		t.Fatal(err)
	}
	bed.seatPush(t, "seat.txt", "a seat's ledger write")
	if line := watch.Step(); line != "" || len(watchHits(t, bed.home)) != 0 {
		t.Fatalf("a lane publication and a seat's own write were flagged: %q %+v", line, watchHits(t, bed.home))
	}

	forged := bed.seatPush(t, "forged.txt", "a candidate test pushes\n\n"+LandingProofTrailer+": batch-1 attempt-1")
	line := watch.Step()
	pause, paused := ReadPause(bed.home)
	if !paused || pause.By != WatchBy || len(watchHits(t, bed.home)) != 1 || !strings.Contains(line, forged[:12]) || len(alerted) != 1 {
		t.Fatalf("a forged lane commit: line %q, pause %+v %t, hits %+v, alerts %d; want the lane stopped and one alert", line, pause, paused, watchHits(t, bed.home), len(alerted))
	}
	if line := watch.Step(); line != "" || len(alerted) != 1 {
		t.Fatalf("the same main judged twice: %q, alerts %d", line, len(alerted))
	}

	// A rewound main.
	if _, err := ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.seat, "push", "--quiet", "--force", "origin", old+":"+MainRef)
	line = watch.Step()
	if _, paused := ReadPause(bed.home); !paused || len(watchHits(t, bed.home)) != 2 || !strings.Contains(line, "rewound") {
		t.Fatalf("a rewound main: line %q, paused %t, hits %+v", line, paused, watchHits(t, bed.home))
	}
}
