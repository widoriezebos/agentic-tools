package steward

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if err := SaveEvidence(path, e); err != nil {
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
