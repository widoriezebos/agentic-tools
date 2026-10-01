package batch

// Returns (simple lane): a member leaves a batch with one disposition.
//
//   - red: the landing agent's word, with its reason, that the member broke
//     the batch or does not merge; the agent decides which member caused a
//     red, so no recorded proof has to place it;
//   - person: a person proven at an enrolled terminal.
//
// The return itself is the existing settlement (ReturnUnits): the member
// is return-pending until its custody is handed back.

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// The return dispositions.
const (
	DispositionRed    = "red"
	DispositionPerson = "person"
)

// Dispositions are the dispositions a return may name.
var Dispositions = []string{DispositionRed, DispositionPerson}

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

// ReturnEvidence names what admits returning member with disposition, or
// refuses: a person proven at an enrolled terminal (person names them,
// empty when none was), or the agent's reason.
func ReturnEvidence(record Record, member, disposition, person, reason string) (string, error) {
	refuse := func(code, message string) (string, error) {
		return "", &ReturnRefusal{Code: code, Member: member, Disposition: disposition, Message: message}
	}
	if !slices.Contains(Dispositions, disposition) {
		return refuse(CodeReturnDispositionUnknown, fmt.Sprintf("%q is not a way a member leaves a batch (%s), so nothing was returned", disposition, strings.Join(Dispositions, ", ")))
	}
	if !slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.GoalID == member }) {
		return refuse(CodeReturnMemberAbsent, fmt.Sprintf("%s is not a member of batch %s, so nothing was returned", member, record.BatchID))
	}
	if disposition == DispositionPerson {
		if strings.TrimSpace(person) == "" {
			return refuse(CodeReturnEvidenceMissing, fmt.Sprintf("only a person at an enrolled terminal returns %s at their word, so nothing was returned", member))
		}
		return "person " + strings.TrimSpace(person), nil
	}
	if strings.TrimSpace(reason) == "" {
		return refuse(CodeReturnEvidenceMissing, fmt.Sprintf("the return of %s names no reason, so nothing was returned", member))
	}
	return "the landing agent's word", nil
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
		found, err := ReturnEvidence(*record, member, disposition, person, reason)
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
