package batch

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var evidenceNow = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

const evidenceBatch = "01j5x00000000000000000kd11"

// evidenceStore holds one batch of six joined members, the proof attempts
// and composition evidence prove and begin recorded through their writers.
func evidenceStore(t *testing.T) Store {
	t.Helper()
	store := NewStore(t.TempDir(), nil)
	var units []Unit
	for _, member := range []string{"m-red", "m-unavailable", "m-green", "m-conflict", "m-seam", "m-person", "m-flaky", "m-recovered"} {
		units = append(units, Unit{GoalID: member, Chain: "chain-" + member, SeatRoot: "/seat", State: UnitJoined,
			Claim: Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}})
	}
	must(t, store.Create(Record{Schema: 1, BatchID: evidenceBatch, State: StateProving, Units: units}))
	// The batch's series, which the attempts prove.
	must(t, store.Update(evidenceBatch, func(record *Record) error {
		record.Openings = append(record.Openings, Opening{OpID: "op-1", Actor: "lane:test"})
		return nil
	}))
	for _, row := range []struct {
		attempt ProofAttempt
		status  string
	}{
		// m-red's own red places the batch's red on it.
		{ProofAttempt{ID: "a0", Subject: SubjectMember, Member: "m-red", Covers: []string{"m-red"}}, AttemptRed},
		{ProofAttempt{ID: "a1", Subject: SubjectBatch, Covers: []string{"m-red"}}, AttemptRed},
		{ProofAttempt{ID: "a2", Subject: SubjectMember, Member: "m-unavailable", Covers: []string{"m-unavailable"}}, AttemptUnavailable},
		{ProofAttempt{ID: "a3", Subject: SubjectBatch, Covers: []string{"m-unavailable", "m-green"}}, AttemptUnavailable},
		{ProofAttempt{ID: "a4", Subject: SubjectMember, Member: "m-green", Covers: []string{"m-green"}}, AttemptGreen},
		{ProofAttempt{ID: "a5", Subject: SubjectBase}, AttemptRed},
		{ProofAttempt{ID: "a6", Subject: SubjectMember, Member: "m-flaky", Covers: []string{"m-flaky"}}, AttemptRed},
		{ProofAttempt{ID: "a7", Subject: SubjectMember, Member: "m-flaky", Covers: []string{"m-flaky"}}, AttemptGreen},
		{ProofAttempt{ID: "a8", Subject: SubjectBatch, Covers: []string{"m-recovered"}}, AttemptRed},
		{ProofAttempt{ID: "a9", Subject: SubjectMember, Member: "m-recovered", Covers: []string{"m-recovered"}}, AttemptUnavailable},
		// A red still running has not spoken for its member.
		{ProofAttempt{ID: "a10", Subject: SubjectMember, Member: "m-green", Covers: []string{"m-green"}}, AttemptRunning},
	} {
		row.attempt.OpID = "op-1"
		must(t, StartAttempt(store, evidenceBatch, row.attempt))
		if row.status != AttemptRunning {
			must(t, FinishAttempt(store, evidenceBatch, row.attempt.ID, row.status, "", "", nil, evidenceNow))
		}
	}
	must(t, RecordComposition(store, evidenceBatch, CompositionEvidence{OpID: "op-1", Actor: "lane:test", Kind: CompositionConflict, Members: []string{"m-conflict"}, Detail: "units/a.txt"}, evidenceNow))
	must(t, RecordComposition(store, evidenceBatch, CompositionEvidence{OpID: "op-1", Actor: "lane:test", Kind: CompositionSeamTooLarge, Members: []string{"m-seam"}, Detail: "57 lines over 40"}, evidenceNow))
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
		{"m-red", DispositionRed, "", "attempt a1 (batch, placed by attempt a0 (member:m-red))", ""},
		{"m-unavailable", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-green", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-conflict", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-conflict", DispositionConflict, "", "composition 1 (conflict at begin op-1)", ""},
		{"m-conflict", DispositionSeamTooLarge, "", "", CodeReturnEvidenceMissing},
		{"m-seam", DispositionSeamTooLarge, "", "composition 2 (seam-too-large at begin op-1)", ""},
		{"m-seam", DispositionConflict, "", "", CodeReturnEvidenceMissing},
		{"m-person", DispositionPerson, "", "", CodeReturnEvidenceMissing},
		{"m-person", DispositionPerson, "Wido", "person Wido", ""},
		{"m-red", "flaky", "", "", CodeReturnDispositionUnknown},
		// The newest attempt naming the member speaks for it: a red then a
		// green is not red now, nor a red then a run that could not start.
		{"m-flaky", DispositionRed, "", "", CodeReturnEvidenceMissing},
		{"m-recovered", DispositionRed, "", "", CodeReturnEvidenceMissing},
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
	if err != nil || evidence != "attempt a1 (batch, placed by attempt a0 (member:m-red))" {
		t.Fatalf("red return = %q, %v", evidence, err)
	}
	returned, err := store.Load(evidenceBatch)
	must(t, err)
	unit := returned.Units[0]
	if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || unit.Disposition != DispositionRed || unit.Evidence != evidence ||
		unit.Failure != "red: attempt a1 (batch, placed by attempt a0 (member:m-red)): its own test fails" {
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
}

// diagnosisRecord is a batch of members whose series op-1 the attempts
// prove, each with its outcome, as landing prove records them: a batch
// attempt covers every member, member:M only M, the base none.
func diagnosisRecord(t *testing.T, members []string, attempts ...[2]string) Record {
	t.Helper()
	store := NewStore(t.TempDir(), nil)
	var units []Unit
	for _, member := range members {
		units = append(units, Unit{GoalID: member, Chain: "chain-" + member, SeatRoot: "/seat", State: UnitJoined,
			Claim: Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}})
	}
	must(t, store.Create(Record{Schema: 1, BatchID: evidenceBatch, State: StateProving, Units: units}))
	must(t, store.Update(evidenceBatch, func(record *Record) error {
		record.Openings = append(record.Openings, Opening{OpID: "op-1", Actor: "lane:test", Members: members})
		return nil
	}))
	for index, row := range attempts {
		attempt := ProofAttempt{ID: fmt.Sprintf("d%d", index+1), OpID: "op-1"}
		switch subject := row[0]; {
		case subject == SubjectBatch:
			attempt.Subject, attempt.Covers = SubjectBatch, members
		case subject == SubjectBase:
			attempt.Subject = SubjectBase
		default:
			attempt.Subject, attempt.Member, attempt.Covers = SubjectMember, strings.TrimPrefix(subject, "member:"), []string{strings.TrimPrefix(subject, "member:")}
		}
		must(t, StartAttempt(store, evidenceBatch, attempt))
		must(t, FinishAttempt(store, evidenceBatch, attempt.ID, row[1], "", "", nil, evidenceNow))
	}
	record, err := store.Load(evidenceBatch)
	must(t, err)
	return record
}

// A red of the whole batch covers every member but names none by itself
// (K8): it is a member's red evidence only when diagnosis places it there,
// by the member's own red on the same base, or by a green base with every
// other member green alone. The base's own red names nobody.
func TestBatchRedNamesAMemberOnlyByDiagnosis(t *testing.T) {
	t.Parallel()
	pair := []string{"goal-a", "goal-b"}
	for _, row := range []struct {
		name     string
		members  []string
		attempts [][2]string
		member   string
		evidence string
	}{
		{"batch red alone", pair, [][2]string{{"batch", AttemptRed}}, "goal-a", ""},
		{"batch red alone, the other member", pair, [][2]string{{"batch", AttemptRed}}, "goal-b", ""},
		{"the member's own red first", pair, [][2]string{{"member:goal-a", AttemptRed}, {"batch", AttemptRed}}, "goal-a",
			"attempt d2 (batch, placed by attempt d1 (member:goal-a))"},
		{"another member's red places nothing here", pair, [][2]string{{"member:goal-a", AttemptRed}, {"batch", AttemptRed}}, "goal-b", ""},
		{"the member's own red, then green", pair, [][2]string{{"member:goal-a", AttemptRed}, {"member:goal-a", AttemptGreen}, {"batch", AttemptRed}}, "goal-a", ""},
		{"base green and the other member green alone", pair, [][2]string{{"batch", AttemptRed}, {"base", AttemptGreen}, {"member:goal-b", AttemptGreen}}, "goal-a",
			"attempt d1 (batch, placed by the green base in attempt d2)"},
		{"base green, the other member not run alone", pair, [][2]string{{"batch", AttemptRed}, {"base", AttemptGreen}}, "goal-a", ""},
		{"base green, but this member green alone", pair, [][2]string{{"batch", AttemptRed}, {"base", AttemptGreen}, {"member:goal-b", AttemptGreen}}, "goal-b", ""},
		{"one member, base green", []string{"goal-a"}, [][2]string{{"batch", AttemptRed}, {"base", AttemptGreen}}, "goal-a",
			"attempt d1 (batch, placed by the green base in attempt d2)"},
		{"one member, base red", []string{"goal-a"}, [][2]string{{"batch", AttemptRed}, {"base", AttemptRed}}, "goal-a", ""},
		{"one member, base could not run", []string{"goal-a"}, [][2]string{{"batch", AttemptRed}, {"base", AttemptUnavailable}}, "goal-a", ""},
		{"the member's red after the batch's speaks itself", pair, [][2]string{{"batch", AttemptRed}, {"member:goal-a", AttemptRed}}, "goal-a",
			"attempt d2 (member:goal-a)"},
	} {
		record := diagnosisRecord(t, row.members, row.attempts...)
		evidence, err := ReturnEvidence(record, row.member, DispositionRed, "")
		var refusal *ReturnRefusal
		switch {
		case row.evidence != "" && (err != nil || evidence != row.evidence):
			t.Errorf("%s: %s as red = %q, %v; want %q", row.name, row.member, evidence, err, row.evidence)
		case row.evidence == "" && (!errors.As(err, &refusal) || refusal.Code != CodeReturnEvidenceMissing):
			t.Errorf("%s: %s as red = %q, %v; want refused for want of evidence", row.name, row.member, evidence, err)
		}
	}
	record := diagnosisRecord(t, pair, [2]string{"batch", AttemptRed})
	if _, err := ReturnEvidence(record, "goal-a", DispositionRed, ""); err == nil || !strings.Contains(err.Error(), "--subject member:goal-a") {
		t.Errorf("an unplaced batch red's refusal = %v; want it to name the member's own proof", err)
	}
}
