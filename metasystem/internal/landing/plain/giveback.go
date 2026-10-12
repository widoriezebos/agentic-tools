package plain

import (
	"slices"
	"time"
)

// GiveBack records a person's return, preserving a person's running proof.
func GiveBack(install, goal, reason string, person ActProvenance, s ProveSeams) (entry Entry, message string, err error) {
	err = withLock(install, func() error {
		entry, message, err = giveBackLocked(install, goal, reason, person, s)
		return err
	})
	return
}

func giveBackLocked(install, goal, reason string, person ActProvenance, s ProveSeams) (Entry, string, error) {
	entry, known, err := Latest(install, goal)
	if err != nil || !known || entry.State != StateWaiting && entry.State != StateReturned {
		return entry, "nothing to give back: " + goal + " is not in the lane", err
	}
	batch, err := ReadBatch(install)
	if err != nil {
		return entry, "", err
	}
	member := GoalSHA{Goal: goal, SHA: entry.SHA}
	selected := batch != nil && slices.Contains(batch.Members, member)
	if !selected && entry.State == StateReturned {
		return entry, goal + " is already returned: " + entry.Reason, nil
	}
	proof, err := lastGoalProof(install, goal)
	if err != nil {
		return entry, "", err
	}
	if proof.Cause != nil && (proof.Cause.Kind == "main" || proof.Cause.Kind == "other" || proof.Cause.Kind == "own" && proof.Cause.Goal != goal) {
		attributed := proof.Cause.Kind
		if attributed != "main" {
			attributed = "another goal"
		}
		reason += "; the lane's last check attributed " + attributed + " at " + resultTime(proof).Local().Format("2006-01-02 15:04 MST") + "; the person returned the goal as unclassified"
	}
	if selected {
		running, recorded, alive, err := ReadRunning(install, s)
		if err != nil {
			return entry, "", err
		}
		index := slices.IndexFunc(batch.Actions, func(a BatchAction) bool { return a.Kind == "give-back" && slices.Contains(a.With, member) && a.Pending })
		repeated := index >= 0
		if repeated {
			person, reason = *batch.Actions[index].Person, batch.Actions[index].Reason
		} else {
			batch.Actions = append(batch.Actions, BatchAction{Kind: "give-back", By: person.Person, At: person.CheckedAt.UTC().Format(time.RFC3339Nano), With: []GoalSHA{member}, Reason: reason, Person: &person, Pending: true, Attempt: running.Attempt, Impact: "return to the seat after the proof ends"})
			index = len(batch.Actions) - 1
			if err := writeBatch(install, batch); err != nil {
				return entry, "", err
			}
		}
		if recorded && alive && running.BatchID == batch.ID {
			if running.Person != nil {
				message := "give-back recorded for "
				if repeated {
					message = "give-back already pending for "
				}
				return entry, message + goal + "; the person's proof continues", nil
			}
			if cancel, err := cancelProofLocked(install, running, "cancelled: give-back "+goal, s); err != nil {
				return entry, "", err
			} else if err := finishCancel(install, running, *cancel); err != nil {
				return entry, "", err
			}
		}
		s.Person = &person
		entry, _, err = returnLocked(install, goal, reason, &Cause{Kind: "unclassified", Name: person.Person, Goal: goal, SHA: entry.SHA}, nil, s.now(), s)
		if err != nil {
			return entry, "", err
		}
		batch.Members = slices.DeleteFunc(batch.Members, func(m GoalSHA) bool { return m == member })
		batch.State, batch.Open, batch.SealedAt, batch.ClosureReason = BatchPrepared, true, "", ""
		if batch.Person != nil {
			batch.Person.Subject = slices.Clone(batch.Members)
		}
		batch.Actions[index].Pending = false
		batch.Actions[index].Impact = "returned to the seat; rebuild the merge without this goal"
		if err := writeBatch(install, batch); err != nil {
			return entry, "", err
		}
	}
	s.Person = &person
	entry, _, err = returnLocked(install, goal, reason, &Cause{Kind: "unclassified", Name: person.Person, Goal: goal, SHA: entry.SHA}, nil, s.now(), s)
	return entry, "returned " + goal + " to its seat: " + entry.Reason, err
}

// finishGiveBacksLocked consumes durable requests before selecting or pushing.
func finishGiveBacksLocked(install string, s ProveSeams) error {
	batch, err := ReadBatch(install)
	if err != nil || batch == nil {
		return err
	}
	for index, act := range batch.Actions {
		if act.Kind == "give-back" && act.Pending && act.Person != nil {
			for _, member := range act.With {
				batch, err = ReadBatch(install)
				if err != nil {
					return err
				}
				entry, known, err := Latest(install, member.Goal)
				if err != nil {
					return err
				}
				if !known || entry.SHA != member.SHA || !slices.Contains(batch.Members, member) {
					batch.Actions[index].Pending = false
					batch.Actions[index].Impact = "superseded by a newer hand-in; nothing returned"
					if err := writeBatch(install, batch); err != nil {
						return err
					}
					break
				}
				if _, _, err := giveBackLocked(install, member.Goal, act.Reason, *act.Person, s); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
