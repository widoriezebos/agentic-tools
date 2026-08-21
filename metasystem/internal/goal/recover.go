package goal

// Executable recovery (F7): the one rule, run for real. Recover
// walks every non-terminal journal entry, resolves the opid
// postcondition against a fresh capture, and ACTS — confirming what
// landed, correcting terminalized beliefs the canonical history
// contradicts, completing a provably dead owner's work from its
// stored intent through the same transaction loop the living used,
// and leaving a live owner's entries strictly alone. A pushed entry
// stops blocking this clone the moment recovery classifies it.

import (
	"fmt"
	"strings"
	"time"
)

// RecoveryReport is one entry's disposition.
type RecoveryReport struct {
	Opid   string
	Action RecoveryAction
	Detail string
}

// Recover runs the rule over the whole journal. Verbs whose stored
// intent cannot be rebuilt generically (reconcile re-runs from the
// checkout it captures; migrate re-runs from its reviewed inputs)
// terminalize toward their own re-runnable entry points, named.
func Recover(e Endpoint) ([]RecoveryReport, error) {
	entries, err := Entries(e.Root)
	if err != nil {
		return nil, err
	}
	nonce, err := readNonce()
	if err != nil {
		return nil, err
	}
	tip, err := CaptureTip(e, nonce)
	CleanupRefs(e, nonce)
	if err != nil {
		return nil, err
	}

	var reports []RecoveryReport
	for _, entry := range entries {
		if entry.Phase == PhaseTerminal &&
			(entry.Outcome == OutcomeConfirmed || entry.Outcome == OutcomeConfirmedLate) {
			continue
		}
		present, trErr := TrailerPresent(e, tip, entry.Opid)
		if trErr != nil {
			return reports, trErr
		}
		post := PostconditionAbsent
		if present {
			post = PostconditionPresent
		}
		action := ClassifyRecovery(entry, post, OwnerAlive(entry), callerIsOwner(entry), PastDeadline(entry, timeNowUTC()))
		report := RecoveryReport{Opid: entry.Opid, Action: action}
		switch action {
		case ActionConfirm:
			if err := MarkTerminal(e.Root, entry.Opid, OutcomeConfirmed, "opid found on "+short(tip)+" by recovery"); err != nil {
				return reports, err
			}
			// Accepted advances only onto a VALIDATED tip, recovery
			// included (R2-3), and a refused advance is said in the
			// report — never discarded.
			if valErr := ValidateCommit(e.Root, tip); valErr != nil {
				report.Detail = "confirmed on the canonical tip; accepted NOT advanced (the tip does not validate): " + valErr.Error()
			} else if advErr := advanceAcceptedForward(e.Root, tip); advErr != nil {
				report.Detail = "confirmed on the canonical tip; accepted NOT advanced: " + advErr.Error()
			} else {
				report.Detail = "confirmed on the canonical tip"
			}
			CleanupRefs(e, entry.Opid)
		case ActionConfirmLate:
			if err := CorrectLate(e.Root, entry.Opid, "opid found on "+short(tip)+" by recovery"); err != nil {
				return reports, err
			}
			report.Detail = "belief corrected to confirmed-late"
		case ActionComplete:
			detail, err := completeFromIntent(e, entry)
			if err != nil {
				return reports, err
			}
			report.Detail = detail
		case ActionLeaveToOwner, ActionKeepRetrying:
			report.Detail = "a live owner's entry; untouched"
		case ActionAbandonOwn:
			if err := MarkTerminal(e.Root, entry.Opid, OutcomeAbandoned, "the owner abandons its never-pushed work"); err != nil {
				return reports, err
			}
			CleanupRefs(e, entry.Opid)
			report.Detail = "abandoned by its owner"
		case ActionExpireOwn:
			if err := MarkTerminal(e.Root, entry.Opid, OutcomeExpired, "the owner's deadline passed"); err != nil {
				return reports, err
			}
			CleanupRefs(e, entry.Opid)
			report.Detail = "expired at its own deadline"
		default:
			report.Detail = "nothing to do"
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// completeFromIntent finishes a dead owner's operation: take over
// the entry, rebuild the mutation from the stored intent, and run
// the same transaction loop the living use. The rebuilt push
// resolves identically to the delayed one — same opid, same intent.
func completeFromIntent(e Endpoint, entry Entry) (string, error) {
	taken, err := TakeOver(e.Root, entry.Opid)
	if err != nil {
		return "", err
	}
	mutate, rebuildErr := rebuildMutate(e, taken)
	if rebuildErr != nil {
		// A verb recovery cannot rebuild generically terminalizes
		// toward its own re-runnable entry point, named — never a
		// silent wedge (the pushed block clears).
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, rebuildErr.Error()); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "not rebuildable: " + rebuildErr.Error(), nil
	}
	res, err := runTransaction(e, PublishRequest{
		Opid: taken.Opid, Machine: taken.Machine, Lineage: taken.Lineage,
		Intent:  taken.Intent,
		Message: "goal " + taken.Intent.Verb + " (recovered)",
		Mutate:  mutate,
		Validate: func(commit string) error {
			return ValidateCommit(e.Root, commit)
		},
	})
	if err != nil {
		return "", err
	}
	return "completed from the stored intent: " + string(res.Outcome), nil
}

// rebuildMutate reconstructs a verb's mutation from its stored
// intent. The recovered actor is the ENTRY's — the opid already
// binds machine and lineage, and the rebuilt commit must resolve
// the same postcondition.
func rebuildMutate(e Endpoint, entry Entry) (func(tip string) ([]Change, error), error) {
	in := entry.Intent
	r := VerbRequest{
		Endpoint: e,
		Actor:    actorFromEntry(entry),
		Ulid:     "recovered", // unused: the opid comes from the entry
		Now:      timeNowUTC(),
	}
	// The opid must be the ENTRY's, not derived: recovery's history
	// lines carry the original operation.
	opid := entry.Opid

	target := func() (string, error) {
		if len(in.Targets) != 1 {
			return "", fmt.Errorf("the stored intent names %d targets; this verb rebuilds one", len(in.Targets))
		}
		return in.Targets[0], nil
	}

	switch in.Verb {
	case "open":
		id, err := target()
		if err != nil {
			return nil, err
		}
		return func(tip string) ([]Change, error) {
			t, err := loadTree(e.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, exists := t.Live[id]; exists {
				if hasOpid(f, opid) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			f := &GoalFile{
				Id: id, State: StateQueued, Intent: in.Args["intent"],
				Origin: in.Args["origin"], NextStep: in.Args["next"],
				OpenedAt: r.stamp(), Revision: 0,
			}
			touchWithOpid(f, r, opid, "open", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, nil
	case "claim":
		id, err := target()
		if err != nil {
			return nil, err
		}
		return func(tip string) ([]Change, error) {
			t, err := loadTree(e.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; the recovered claim has nothing to take", id)
			}
			if hasOpid(f, opid) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateQueued {
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s on the rebuilt tip", id, dep)
				}
			}
			f.State = StateClaimed
			f.Claimed = &ClaimRecord{Machine: entry.Machine, Lineage: entry.Lineage, At: r.stamp()}
			touchWithOpid(f, r, opid, "claim", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, nil
	case "release", "done", "park", "unpark", "edit":
		id, err := target()
		if err != nil {
			return nil, err
		}
		return func(tip string) ([]Change, error) {
			t, err := loadTree(e.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				if in.Verb == "done" {
					if df, archived := t.Done[id]; archived && hasOpid(df, opid) {
						return nil, AlreadyApplied{}
					}
				}
				return nil, fmt.Errorf("goal %s is not live on the rebuilt tip; the recovered %s refuses", id, in.Verb)
			}
			if hasOpid(f, opid) {
				return nil, AlreadyApplied{}
			}
			switch in.Verb {
			case "release":
				if f.State != StateClaimed {
					return nil, LostToCompetitor{Winner: lastOpid(f)}
				}
				f.State = StateQueued
				f.Claimed = nil
			case "done":
				f.State = StateDone
				f.Conclude = in.Args["conclusion"]
				f.Claimed = nil
				f.Parked = nil
				touchWithOpid(f, r, opid, "done", []string{id})
				delete(t.Live, id)
				t.Done[id] = f
				return []Change{
					{Path: livePath(id), Delete: true},
					{Path: donePath(id), Content: RenderFile(f)},
				}, nil
			case "park":
				if f.State != StateQueued && f.State != StateClaimed {
					return nil, LostToCompetitor{Winner: lastOpid(f)}
				}
				f.State = StateParked
				f.Parked = &ParkRecord{By: actorFromEntry(entry).historyActor(), At: r.stamp(), Because: in.Args["because"]}
				f.Claimed = nil
			case "unpark":
				if f.State != StateParked {
					return nil, LostToCompetitor{Winner: lastOpid(f)}
				}
				f.State = StateQueued
				f.Parked = nil
			case "edit":
				for _, d := range in.Deltas {
					switch d.Field {
					case "intent":
						f.Intent = d.New
					case "next":
						f.NextStep = d.New
					case "origin":
						f.Origin = d.New
					case "blockedBy":
						f.Blocked = nil
						for _, dep := range strings.Split(d.New, ",") {
							if dep = strings.TrimSpace(dep); dep != "" {
								f.Blocked = append(f.Blocked, dep)
							}
						}
					}
				}
			}
			touchWithOpid(f, r, opid, in.Verb, []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, nil
	}
	return nil, fmt.Errorf("verb %q rebuilds through its own entry point (reconcile re-runs from the checkout, migrate from its reviewed inputs); this entry is closed and the operation re-runs by hand", in.Verb)
}

func actorFromEntry(entry Entry) Actor {
	by := entry.Intent.Args["by"]
	return Actor{Machine: entry.Machine, Lineage: entry.Lineage, Human: by}
}

// touchWithOpid is touch with the ENTRY's opid instead of a derived
// one — the recovered history line carries the original operation.
func touchWithOpid(f *GoalFile, r VerbRequest, opid, verb string, targets []string) {
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: r.stamp(), Opid: opid, Verb: verb,
		Actor: r.Actor.historyActor(), Targets: targets, Keep: -1,
	})
}

func hasOpid(f *GoalFile, opid string) bool {
	for _, h := range f.History {
		if h.Opid == opid {
			return true
		}
	}
	return false
}

func timeNowUTC() time.Time { return time.Now().UTC() }
