package dispatch

import (
	"strings"
	"testing"
	"time"
)

func TestFreshAndPostTransitionWaits(t *testing.T) {
	ordinary := CensusWaitCursor{}
	ordinary, done, err := ObserveCensusWait(ordinary, CensusWaitObservation{Fresh: true, Generation: 7, ScanSeq: 41, Interval: 5 * time.Second}, 2, 5)
	if err != nil || !done || ordinary.CompletedPasses != 0 {
		t.Fatalf("already-fresh ordinary launch waited: cursor=%+v done=%t err=%v", ordinary, done, err)
	}
	post := CensusWaitCursor{PostGeneration: 7, PostScanSeq: 41, Marker: 41, Initialized: true}
	post, done, err = ObserveCensusWait(post, CensusWaitObservation{Fresh: true, Generation: 7, ScanSeq: 41, Interval: 5 * time.Second}, 2, 5)
	if err != nil || done {
		t.Fatalf("post-event wait accepted the captured pass: cursor=%+v done=%t err=%v", post, done, err)
	}
	post, done, err = ObserveCensusWait(post, CensusWaitObservation{Fresh: true, Generation: 7, ScanSeq: 42, Interval: 5 * time.Second}, 2, 5)
	if err != nil || !done || post.CompletedPasses != 1 {
		t.Fatalf("post-event wait missed the next fresh pass: cursor=%+v done=%t err=%v", post, done, err)
	}
}

func TestCensusWaitBoundsCompletedFailuresAndRegressions(t *testing.T) {
	cursor := CensusWaitCursor{PostGeneration: 3, PostScanSeq: 10, Marker: 10, Initialized: true}
	var err error
	cursor, _, err = ObserveCensusWait(cursor, CensusWaitObservation{Generation: 3, ScanSeq: 11}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ObserveCensusWait(cursor, CensusWaitObservation{Generation: 3, ScanSeq: 12}, 2, 1)
	if err == nil || !strings.Contains(err.Error(), "2 completed passes") {
		t.Fatalf("completed-pass bound was not enforced: %v", err)
	}
	cursor = CensusWaitCursor{Marker: 9, Initialized: true}
	cursor, _, err = ObserveCensusWait(cursor, CensusWaitObservation{Generation: 3, ScanSeq: 8}, 2, 0)
	if err == nil || !strings.Contains(err.Error(), "regressed") {
		t.Fatalf("sequence regression was not bounded: cursor=%+v err=%v", cursor, err)
	}
}
