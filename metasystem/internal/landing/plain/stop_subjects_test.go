package plain

import (
	"os"
	"testing"
)

func TestLandingRunClosesEveryStoppedSubject(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"g1", "g2"} {
		if err := appendLine(stopsPath(install), Stop{Loop: "lane-proof", Subject: subject, Decision: "stop"}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := CloseProofLoop(install); err != nil {
			t.Fatal(err)
		}
	}
	if stop, err := NewestStop(install); err != nil || stop != nil {
		t.Fatalf("landing run left an older subject stopped: %+v %v", stop, err)
	}
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil || len(lines) != 4 {
		t.Fatalf("each subject needs one close; replay must not append another: %+v %v", lines, err)
	}
}
