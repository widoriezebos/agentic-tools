package goal

// Executable recovery: the one rule, run for real. Recover
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
			// included, and a refused advance is said in the
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
	req, rebuildErr := requestForEntry(e, taken)
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
	res, err := runTransaction(e, req)
	if err != nil {
		return "", err
	}
	return "completed from the stored intent: " + string(res.Outcome), nil
}

// requestForEntry rebuilds the COMPLETE verb request from the
// entry's stored intent through the SAME constructors the live
// verbs run: cascades, Goal-free clears, authority checks,
// displacement markers, and acknowledgment piggybacks all replay
// identically, and the derived opid — the entry's ulid under the
// entry's pair — IS the entry's opid, so every rebuilt history
// line carries the original operation with no injection seam.
func requestForEntry(e Endpoint, entry Entry) (PublishRequest, error) {
	if len(entry.Opid) < 27 {
		return PublishRequest{}, fmt.Errorf("the entry's opid %q is not <ulid>-<machine>-<hash>; close it by hand", entry.Opid)
	}
	r := VerbRequest{
		Endpoint: e,
		Actor:    actorFromEntry(entry),
		Ulid:     entry.Opid[:26],
		Now:      timeNowUTC(),
	}
	if r.opid() != entry.Opid {
		return PublishRequest{}, fmt.Errorf("the entry's opid %s does not derive from its recorded identity; close it by hand", entry.Opid)
	}
	in := entry.Intent
	target := ""
	if len(in.Targets) > 0 {
		target = in.Targets[0]
	}
	cascade := in.Args["cascade"] == "arc"
	switch in.Verb {
	case "open":
		return openRequest(r, target, in.Args["intent"], in.Args["origin"], in.Args["next"], commaValues(in.Args["labels"]))
	case "open-claim":
		return openClaimRequest(r, target, in.Args["intent"], in.Args["origin"], in.Args["next"], commaValues(in.Args["labels"]))
	case "claim":
		if cascade {
			return claimArcRequest(r, target), nil
		}
		return claimRequest(r, target), nil
	case "estimate":
		remaining := in.Args["remaining"]
		if _, ok := ParseWorkingDuration(remaining); !ok {
			return PublishRequest{}, fmt.Errorf("the stored estimate carries an invalid remaining duration %q; close it by hand", remaining)
		}
		return estimateRequest(r, target, remaining), nil
	case "release":
		if cascade {
			return releaseArcRequest(r, target), nil
		}
		return releaseRequest(r, target), nil
	case "done":
		return doneRequest(r, target, in.Args["conclusion"]), nil
	case "park":
		if cascade {
			return parkArcRequest(r, target, in.Args["because"]), nil
		}
		return parkRequest(r, target, in.Args["because"]), nil
	case "unpark":
		if cascade {
			return unparkArcRequest(r, target), nil
		}
		return unparkRequest(r, target), nil
	case "reopen":
		return reopenRequest(r, target), nil
	case "edit":
		fields := EditFields{}
		for _, d := range in.Deltas {
			value := d.New
			switch d.Field {
			case "intent":
				fields.Intent = &value
			case "next":
				fields.NextStep = &value
			case "origin":
				// Origin is immutable provenance: a stored
				// origin delta is a pre-fold journal's residue.
				return PublishRequest{}, fmt.Errorf("the stored intent rewrites Origin, which is immutable; close this entry by hand")
			case "blockedBy":
				blocked := commaValues(value)
				fields.Blocked = &blocked
			case "labels":
				labels := commaValues(value)
				fields.Labels = &labels
			}
		}
		return editRequest(r, target, fields)
	case "steal":
		if r.Actor.Human == "" {
			return PublishRequest{}, fmt.Errorf("the stored steal carries no human (--by); it cannot be replayed")
		}
		return stealRequest(r, target), nil
	case "detach":
		return detachRequest(r, target), nil
	case "set-arc":
		return setArcRequest(r, target, in.Args["arc"]), nil
	case "set-pin":
		if r.Actor.Human == "" {
			return PublishRequest{}, fmt.Errorf("the stored set-pin carries no human (--by); it cannot be replayed")
		}
		if pin := in.Args["pin"]; pin != "-" && !validPinnedNickname(pin) {
			return PublishRequest{}, fmt.Errorf("the stored set-pin carries the invalid machine %q; close it by hand", pin)
		}
		return setPinRequest(r, target, in.Args["pin"]), nil
	case "prune":
		keep := 0
		if _, scanErr := fmt.Sscanf(in.Args["keep"], "%d", &keep); scanErr != nil {
			return PublishRequest{}, fmt.Errorf("the stored prune carries no keep count; close it by hand")
		}
		return pruneRequest(r, keep), nil
	case "declare-free":
		return declareFreeRequest(r, in.Args["origin"], in.Args["digest"]), nil
	}
	return PublishRequest{}, fmt.Errorf("verb %q re-runs from its own entry point (reconcile from the checkout it captures, migrate from its reviewed inputs); this entry closes toward that path", in.Verb)
}

func actorFromEntry(entry Entry) Actor {
	by := entry.Intent.Args["by"]
	return Actor{Machine: entry.Machine, Lineage: entry.Lineage, Human: by}
}

func commaValues(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func timeNowUTC() time.Time { return time.Now().UTC() }
