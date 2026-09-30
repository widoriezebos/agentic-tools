package batch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var evidenceNow = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

const evidenceBatch = "01j5x00000000000000000kd11"

// evidenceStore holds one batch of six joined members, the proof attempts
// and composition evidence begin and prove recorded through their seams.
func evidenceStore(t *testing.T) Store {
	t.Helper()
	store := NewStore(t.TempDir(), nil)
	var units []Unit
	for _, member := range []string{"m-red", "m-unavailable", "m-green", "m-conflict", "m-seam", "m-person"} {
		units = append(units, Unit{GoalID: member, Chain: "chain-" + member, SeatRoot: "/seat", State: UnitJoined,
			Claim: Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}})
	}
	must(t, store.Create(Record{Schema: 1, BatchID: evidenceBatch, State: StateProving, Units: units}))
	for _, attempt := range []ProofAttempt{
		{ID: "a1", Subject: SubjectBatch, Outcome: AttemptRed, Names: []string{"m-red"}},
		{ID: "a2", Subject: MemberSubject("m-unavailable"), Outcome: AttemptUnavailable},
		{ID: "a3", Subject: SubjectBatch, Outcome: AttemptUnavailable, Names: []string{"m-unavailable", "m-green"}},
		{ID: "a4", Subject: MemberSubject("m-green"), Outcome: AttemptGreen},
		{ID: "a5", Subject: SubjectBase, Outcome: AttemptRed},
	} {
		must(t, RecordProofAttempt(store, evidenceBatch, attempt, "lane:test", evidenceNow))
	}
	must(t, RecordComposition(store, evidenceBatch, CompositionEvidence{ID: "c1", Kind: CompositionConflict, Members: []string{"m-conflict"}, Detail: "units/a.txt"}, "lane:test", evidenceNow))
	must(t, RecordComposition(store, evidenceBatch, CompositionEvidence{ID: "c2", Kind: CompositionSeamTooLarge, Members: []string{"m-seam"}, Detail: "57 lines over 40"}, "lane:test", evidenceNow))
	return store
}

// Each disposition needs its own evidence (lane design r10 K8): red a red
// attempt of this batch on the member or a red batch attempt naming it;
// conflict and seam-too-large composition evidence of that kind naming the
// member; person a proven person. Unavailable proof evidence is never
// member-failure evidence, and a red of the base names no member.
func TestReturnEvidencePerDisposition(t *testing.T) {
	t.Parallel()
	store := evidenceStore(t)
	record, err := store.Load(evidenceBatch)
	must(t, err)
	for _, row := range []struct {
		member, disposition, person string
		evidence, code              string
	}{
		{"m-red", DispositionRed, "", "attempt a1 (batch)", ""},
		{"m-unavailable", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-green", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-conflict", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-conflict", DispositionConflict, "", "composition c1", ""},
		{"m-conflict", DispositionSeamTooLarge, "", "", CodeReturnEvidenceMissing},
		{"m-seam", DispositionSeamTooLarge, "", "composition c2", ""},
		{"m-seam", DispositionConflict, "", "", CodeReturnEvidenceMissing},
		{"m-person", DispositionPerson, "", "", CodeReturnEvidenceMissing},
		{"m-person", DispositionPerson, "Wido", "person Wido", ""},
		{"m-red", "flaky", "", "", CodeReturnDispositionUnknown},
		{"m-absent", DispositionPerson, "Wido", "", CodeReturnMemberAbsent},
	} {
		evidence, err := ReturnEvidence(record, row.member, row.disposition, row.person)
		var refusal *ReturnRefusal
		switch {
		case row.code == "" && (err != nil || evidence != row.evidence):
			t.Errorf("%s as %s = %q, %v; want evidence %q", row.member, row.disposition, evidence, err, row.evidence)
		case row.code != "" && (!errors.As(err, &refusal) || refusal.Code != row.code || evidence != ""):
			t.Errorf("%s as %s = %q, %v; want refused %s", row.member, row.disposition, evidence, err, row.code)
		}
	}
	if _, err := ReturnEvidence(record, "m-unavailable", DispositionRed, ""); err == nil || !strings.Contains(err.Error(), "could not run") {
		t.Errorf("an unavailable proof's refusal = %v; want it named as a proof that could not run", err)
	}
}

// A typed return reads its evidence under the batch lock and keeps it on
// the member; a refused return changes nothing; a repeat changes nothing;
// another disposition for a member already leaving is refused.
func TestTypedReturnKeepsItsEvidenceAndRefusesWithout(t *testing.T) {
	t.Parallel()
	store := evidenceStore(t)
	before, err := store.Load(evidenceBatch)
	must(t, err)
	if _, err := RequestTypedReturn(store, evidenceBatch, "m-unavailable", DispositionRed, "", "", "lane:test", evidenceNow); err == nil {
		t.Fatal("a member was returned red on unavailable evidence")
	}
	after, err := store.Load(evidenceBatch)
	must(t, err)
	if len(after.History) != len(before.History) || after.Units[1].State != UnitJoined {
		t.Fatalf("a refused return changed the batch: %+v", after.Units[1])
	}
	evidence, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionRed, "", "its own test fails", "lane:test", evidenceNow)
	if err != nil || evidence != "attempt a1 (batch)" {
		t.Fatalf("red return = %q, %v", evidence, err)
	}
	returned, err := store.Load(evidenceBatch)
	must(t, err)
	unit := returned.Units[0]
	if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || unit.Disposition != DispositionRed || unit.Evidence != evidence ||
		unit.Failure != "red: attempt a1 (batch): its own test fails" {
		t.Fatalf("returned member = %+v", unit)
	}
	if again, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionRed, "", "its own test fails", "lane:test", evidenceNow); err != nil || again != evidence {
		t.Fatalf("repeat = %q, %v", again, err)
	}
	if repeated, _ := store.Load(evidenceBatch); len(repeated.History) != len(returned.History) {
		t.Fatalf("the repeat wrote history")
	}
	var refusal *ReturnRefusal
	if _, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionPerson, "Wido", "", "Wido", evidenceNow); !errors.As(err, &refusal) {
		t.Fatalf("another disposition for a leaving member = %v", err)
	}
	if _, err := RequestTypedReturn(store, evidenceBatch, "m-person", DispositionPerson, "Wido", "", "Wido", evidenceNow); err != nil {
		t.Fatal(err)
	}
	withdrawn, _ := store.Load(evidenceBatch)
	if unit := withdrawn.Units[5]; unit.Outcome != UnitWithdrawn || unit.Disposition != DispositionPerson || unit.Evidence != "person Wido" {
		t.Fatalf("person return = %+v", unit)
	}
	if err := RecordProofAttempt(store, evidenceBatch, ProofAttempt{ID: "bad", Subject: "member:", Outcome: AttemptRed}, "lane:test", evidenceNow); err == nil {
		t.Fatal("an attempt without a member subject was recorded")
	}
}
