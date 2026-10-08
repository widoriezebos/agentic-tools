package plain

import (
	"os"
	"testing"
	"time"
)

func TestProofAdmissionClosesEveryMatchingStoppedSubject(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"g1", "g2"} {
		if err := appendLine(stopsPath(install), Stop{Loop: "lane-proof", Subject: subject, BatchID: "batch", Decision: "stop", Required: []string{"metasystem", "landing", "prove"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := appendLine(stopsPath(install), Stop{Loop: "lane-proof", Subject: "unrelated", Decision: "stop", Required: []string{"metasystem", "landing", "prove"}}); err != nil {
		t.Fatal(err)
	}
	running := Running{Attempt: "check", BatchID: "batch", Person: &ActProvenance{Person: "Wido"}}
	for range 2 {
		if err := withLock(install, func() error { return closeProofAdmissionStopsLocked(install, running, time.Unix(10, 0)) }); err != nil {
			t.Fatal(err)
		}
	}
	if stops, err := OpenStops(install); err != nil || len(stops) != 1 || stops[0].Subject != "unrelated" {
		t.Fatalf("proof admission must close every matching subject and retain the unrelated stop: %+v %v", stops, err)
	}
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil || len(lines) != 5 {
		t.Fatalf("each subject needs one close; replay must not append another: %+v %v", lines, err)
	}
}
