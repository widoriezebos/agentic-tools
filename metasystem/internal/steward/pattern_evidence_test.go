package steward

import (
	"fmt"
	"testing"
	"time"
)

func evidenceAt(minute int) AlertEvidence {
	return AlertEvidence{Record: "r", At: time.Date(2026, 9, 30, 15, minute, 0, 0, time.UTC).Format(time.RFC3339Nano), Fact: fmt.Sprint(minute)}
}

// An episode keeps its first item and the newest 19 by time. A cycle that
// sees the same items again, the trimmed ones included, changes nothing
// (no rewrite); a newer item pushes out the oldest kept after the first.
func TestEvidenceKeepsFirstAndNewestWithoutRewrites(t *testing.T) {
	t.Parallel()
	var all []AlertEvidence
	for minute := 0; minute < 30; minute++ {
		all = append(all, evidenceAt(minute))
	}
	held, changed := mergeEvidence(nil, all)
	if !changed || len(held) != 20 || held[0] != all[0] || held[1] != all[11] || held[19] != all[29] {
		t.Fatalf("first merge: %v %+v", changed, held)
	}
	if again, changed := mergeEvidence(held, all); changed || len(again) != 20 {
		t.Fatalf("the same items again rewrote the evidence: %v %+v", changed, again)
	}
	newer := evidenceAt(45)
	next, changed := mergeEvidence(held, append(all, newer))
	if !changed || next[0] != all[0] || next[1] != all[12] || next[19] != newer {
		t.Fatalf("a newer item: %v %+v", changed, next)
	}
}
