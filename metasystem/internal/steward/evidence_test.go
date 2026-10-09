package steward

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFleetProgressResetsPausedAgePreservingRestartHistory(t *testing.T) {
	t.Parallel()
	for _, marks := range []Marks{{HeadOid: "new-head", OpidDigest: "old-claims"}, {HeadOid: "old-head", OpidDigest: "new-claims"}} {
		t.Run(marks.HeadOid+"-"+marks.OpidDigest, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			before := Evidence{
				Marks:             Marks{HeadOid: "old-head", OpidDigest: "old-claims"},
				TicksSinceAdvance: 5, DryRevivals: 2, Degraded: 1,
				SampledAt: now.Format(time.RFC3339Nano), Age: 4 * time.Minute,
				AbnormalCount: 1, CurrentSeat: "seat-1", CurrentContinuation: "revival-1",
			}
			before.Abnormal[0] = AbnormalRestart{At: now.Add(-time.Minute), Class: "failed", Marks: before.Marks, Nonce: "revival-1", Pending: true}
			if err := SaveEvidence(root, EvidencePath(root), before); err != nil {
				t.Fatal(err)
			}
			retained, err := LoadEvidence(EvidencePath(root))
			if err != nil {
				t.Fatal(err)
			}
			advanced := Observe(retained, marks)
			if advanced.Marks != marks || advanced.Age != 0 || advanced.SampledAt != "" || advanced.TicksSinceAdvance != 0 || advanced.DryRevivals != 0 || advanced.Degraded != 0 {
				t.Fatalf("retained progress must reset its patience clocks: %+v", advanced)
			}
			if advanced.Abnormal != before.Abnormal || advanced.AbnormalCount != before.AbnormalCount || advanced.CurrentSeat != before.CurrentSeat || advanced.CurrentContinuation != before.CurrentContinuation {
				t.Fatalf("progress must preserve restart reservations and current launches: %+v", advanced)
			}
		})
	}
}

func TestIdenticalEvidenceStaysOld(t *testing.T) {
	m := Marks{HeadOid: "aaa", OpidDigest: "d1"}
	e := Evidence{Marks: m}
	for i := 1; i <= 3; i++ {
		e = Observe(e, m)
		if e.TicksSinceAdvance != i {
			t.Fatalf("identical marks must age monotonically: tick %d got %d", i, e.TicksSinceAdvance)
		}
	}
}

func TestAdvanceResetsAgeAndDryCount(t *testing.T) {
	e := Evidence{Marks: Marks{HeadOid: "aaa"}, TicksSinceAdvance: 7, DryRevivals: 2}
	e = Observe(e, Marks{HeadOid: "bbb"})
	if e.TicksSinceAdvance != 0 || e.DryRevivals != 0 {
		t.Fatalf("a real advance resets age and dry count: %+v", e)
	}
}

func TestOpidAdvanceCountsAsProgress(t *testing.T) {
	e := Evidence{Marks: Marks{HeadOid: "aaa", OpidDigest: "d1"}, TicksSinceAdvance: 4}
	e = Observe(e, Marks{HeadOid: "aaa", OpidDigest: "d2"})
	if e.TicksSinceAdvance != 0 {
		t.Fatalf("claim-History growth is progress: %+v", e)
	}
}

func TestRevivalIncrementsDryCountWithoutTouchingMarks(t *testing.T) {
	e := Evidence{Marks: Marks{HeadOid: "aaa"}, TicksSinceAdvance: 5}
	e = RecordRevival(e)
	if e.DryRevivals != 1 || e.TicksSinceAdvance != 5 || e.Marks.HeadOid != "aaa" {
		t.Fatalf("a revival is not progress: %+v", e)
	}
}

func TestStoreRoundTripsAndSurvivesFirstTick(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifacts", "agents", "steward", "highwater.json")
	e, err := LoadEvidence(path)
	if err != nil || e != (Evidence{}) {
		t.Fatalf("first tick loads the zero state: %+v %v", e, err)
	}
	e = Evidence{Marks: Marks{HeadOid: "abc", OpidDigest: "d"}, TicksSinceAdvance: 2, DryRevivals: 1}
	if err := SaveEvidence(dir, path, e); err != nil {
		t.Fatal(err)
	}
	got, err := LoadEvidence(path)
	if err != nil || got != e {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

func TestMalformedStoreIsAnErrorNotAGuess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "highwater.json")
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEvidence(path); err == nil {
		t.Fatal("a torn store must surface as an error for the degraded path")
	}
}
