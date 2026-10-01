package batch

// Typed returns with matching evidence (lane design r10 K8). A member leaves
// a batch with one disposition, and the batch's own records must hold the
// evidence that disposition names:
//
//   - red: a red proof attempt of this batch whose subject is the member
//     (member:M), or the batch whose red names the member;
//   - conflict and seam-too-large: composition evidence of that kind
//     naming the member, which begin records when it refuses the series or
//     a member's replay fails (begin --record-conflict);
//   - person: a person proven at an enrolled terminal.
//
// Unavailable proof evidence (a proof that could not run) is never evidence
// that a member failed.
//
// The evidence is the batch's one record set: the attempts landing prove
// writes (StartAttempt, FinishAttempt) and the composition evidence landing
// begin writes (RecordComposition); a return reads exactly those.

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// The return dispositions (K8).
const (
	DispositionRed          = "red"
	DispositionConflict     = CompositionConflict
	DispositionSeamTooLarge = CompositionSeamTooLarge
	DispositionPerson       = "person"
)

// Dispositions are the dispositions a return may name.
var Dispositions = []string{DispositionRed, DispositionConflict, DispositionSeamTooLarge, DispositionPerson}

// The refusals of a typed return.
const (
	CodeReturnDispositionUnknown = "LANE_RETURN_DISPOSITION_UNKNOWN"
	CodeReturnMemberAbsent       = "LANE_RETURN_MEMBER_ABSENT"
	CodeReturnEvidenceMissing    = "LANE_RETURN_EVIDENCE_MISSING"
)

// ReturnRefusal is a typed return the batch's records do not admit; nothing
// was changed.
type ReturnRefusal struct {
	Code, Member, Disposition, Message string
}

func (refusal *ReturnRefusal) Error() string { return refusal.Code + ": " + refusal.Message }

// RefusalCode is the registered code.
func (refusal *ReturnRefusal) RefusalCode() string { return refusal.Code }

// RefusalWords is the refusal as a person reads it.
func (refusal *ReturnRefusal) RefusalWords() string { return refusal.Message }

// ReturnEvidence names the evidence in record that admits returning member
// with disposition, or refuses. person is the name of a person proven at an
// enrolled terminal, empty when none was.
func ReturnEvidence(record Record, member, disposition, person string) (string, error) {
	refuse := func(code, message string) (string, error) {
		return "", &ReturnRefusal{Code: code, Member: member, Disposition: disposition, Message: message}
	}
	if !slices.Contains(Dispositions, disposition) {
		return refuse(CodeReturnDispositionUnknown, fmt.Sprintf("%q is not a way a member leaves a batch (%s), so nothing was returned", disposition, strings.Join(Dispositions, ", ")))
	}
	if !slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.GoalID == member }) {
		return refuse(CodeReturnMemberAbsent, fmt.Sprintf("%s is not a member of batch %s, so nothing was returned", member, record.BatchID))
	}
	switch disposition {
	case DispositionPerson:
		if strings.TrimSpace(person) == "" {
			return refuse(CodeReturnEvidenceMissing, fmt.Sprintf("only a person at an enrolled terminal returns %s without evidence, so nothing was returned", member))
		}
		return "person " + strings.TrimSpace(person), nil
	case DispositionRed:
		// Only the newest attempt that names the member speaks for it: a
		// red followed by a green, or by a run that could not start, is
		// not evidence that the member fails now.
		message := fmt.Sprintf("no failing test run of this batch names %s, so it was not returned as red", member)
		for index := len(record.Attempts) - 1; index >= 0; index-- {
			attempt := record.Attempts[index]
			// A running attempt has not spoken yet.
			if !names(attempt, member) || !attempt.Terminal() {
				continue
			}
			switch attempt.Status {
			case AttemptRed:
				return "attempt " + attempt.ID + " (" + attemptSubject(attempt) + ")", nil
			case AttemptUnavailable:
				message = fmt.Sprintf("the tests of %s could not run, which is not its failure, so it was not returned as red", member)
			default:
				message = fmt.Sprintf("the newest test run of %s passed, so it was not returned as red", member)
			}
			break
		}
		return refuse(CodeReturnEvidenceMissing, message)
	}
	for index := len(record.Compositions) - 1; index >= 0; index-- {
		evidence := record.Compositions[index]
		if evidence.Kind == disposition && slices.Contains(evidence.Members, member) {
			return fmt.Sprintf("composition %d (%s at begin %s)", index+1, evidence.Kind, evidence.OpID), nil
		}
	}
	return refuse(CodeReturnEvidenceMissing, fmt.Sprintf("batch %s records no %s for %s, so it was not returned as %s", record.BatchID, disposition, member, disposition))
}

// names reports whether attempt is evidence about member: its subject is
// the member, or it is a batch attempt that names the member.
func names(attempt ProofAttempt, member string) bool {
	return attempt.Subject == SubjectMember && attempt.Member == member || attempt.Subject == SubjectBatch && slices.Contains(attempt.Names, member)
}

// attemptSubject is the subject as landing prove is asked for it: batch,
// base or member:M.
func attemptSubject(attempt ProofAttempt) string {
	if attempt.Subject == SubjectMember {
		return SubjectMember + ":" + attempt.Member
	}
	return attempt.Subject
}

// returnOutcome is the unit state a disposition leaves the member in.
func returnOutcome(disposition string) string {
	if disposition == DispositionPerson {
		return UnitWithdrawn
	}
	return UnitEjected
}

// RequestTypedReturn asks for member's return with disposition, under the
// batch lock, after reading its evidence in the record it changes. The
// disposition and the evidence stay on the unit. A repeat of a return
// already asked for with the same disposition changes nothing.
func RequestTypedReturn(store Store, batchID, member, disposition, person, reason, actor string, at time.Time) (string, error) {
	var evidence string
	err := store.Update(batchID, func(record *Record) error {
		for index := range record.Units {
			unit := &record.Units[index]
			if unit.GoalID != member || unit.Disposition == "" || unit.State == UnitJoining || unit.State == UnitJoined {
				continue
			}
			if unit.Disposition != disposition {
				return &ReturnRefusal{Code: CodeReturnMemberAbsent, Member: member, Disposition: disposition,
					Message: fmt.Sprintf("%s is already being returned as %s, so it was not returned as %s", member, unit.Disposition, disposition)}
			}
			evidence = unit.Evidence
			return nil
		}
		found, err := ReturnEvidence(*record, member, disposition, person)
		if err != nil {
			return err
		}
		evidence = found
		failure := disposition + ": " + evidence
		if strings.TrimSpace(reason) != "" {
			failure += ": " + strings.TrimSpace(reason)
		}
		if err := requestUnitReturn(record, member, returnOutcome(disposition), failure, actor, at); err != nil {
			return &ReturnRefusal{Code: CodeReturnMemberAbsent, Member: member, Disposition: disposition,
				Message: fmt.Sprintf("%s is no longer waiting in batch %s (%v), so nothing was returned", member, record.BatchID, err)}
		}
		for index := range record.Units {
			if unit := &record.Units[index]; unit.GoalID == member && unit.State == UnitReturnPending {
				unit.Disposition, unit.Evidence = disposition, evidence
			}
		}
		return nil
	})
	return evidence, err
}
