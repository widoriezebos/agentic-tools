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
	"strconv"
	"strings"
	"time"
)

// RecoveryReport is one entry's disposition.
type RecoveryReport struct {
	Opid   string
	Action RecoveryAction
	Detail string
}

// SensitiveRecoveryPolicy re-establishes authority that cannot lawfully be
// reconstructed from journal text. The returned release function owns any
// ranked lock held across the recovered transaction.
type SensitiveRecoveryPolicy interface {
	BreachStop(Endpoint, Entry) (PublishRequest, func(), error)
}

// Recover runs the rule over the whole journal. Verbs whose stored
// intent cannot be rebuilt generically (reconcile re-runs from the
// checkout it captures; migrate re-runs from its reviewed inputs)
// terminalize toward their own re-runnable entry points, named.
func Recover(e Endpoint) ([]RecoveryReport, error) {
	return RecoverWithPolicy(e, nil)
}

// RecoverWithPolicy applies live policy to authority-sensitive operations.
// A journal remains evidence of intent, never an authority credential.
func RecoverWithPolicy(e Endpoint, policy SensitiveRecoveryPolicy) ([]RecoveryReport, error) {
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
			if err := recoverSplitConfirmedEffect(e, tip, entry); err != nil {
				return reports, err
			}
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
			if err := recoverSplitConfirmedEffect(e, tip, entry); err != nil {
				return reports, err
			}
			if err := CorrectLate(e.Root, entry.Opid, "opid found on "+short(tip)+" by recovery"); err != nil {
				return reports, err
			}
			report.Detail = "belief corrected to confirmed-late"
		case ActionComplete:
			if entry.Intent.Verb == "slice-start" {
				if err := MarkTerminal(e.Root, entry.Opid, OutcomeAbandoned, "slice-start owner died before its postcondition landed; dispatch never acquired reservation authority"); err != nil {
					return reports, err
				}
				CleanupRefs(e, entry.Opid)
				report.Detail = "slice-start abandoned without marking the goal sliced; no reservation was authorized"
				break
			}
			detail, err := completeFromIntent(e, entry, policy)
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
func completeFromIntent(e Endpoint, entry Entry, policy SensitiveRecoveryPolicy) (string, error) {
	taken, err := TakeOver(e.Root, entry.Opid)
	if err != nil {
		return "", err
	}
	if taken.Intent.Verb == "resume" {
		detail := "human authority cannot be recovered from journal text; rerun goal resume from the enrolled terminal"
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	if taken.Intent.Verb == "steal" {
		detail := "human authority cannot be recovered from journal text; rerun goal steal from the enrolled terminal and pass its --approved-ref again when the goal is over norm"
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	if taken.Intent.Verb == "split" && taken.Intent.Args["ratifierTier"] == RatifierHuman {
		detail := "human split ratification cannot be recovered from journal text; rerun goal split from the enrolled terminal against the recorded draft"
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	var release func()
	var req PublishRequest
	var rebuildErr error
	if taken.Intent.Verb == "breach-stop" {
		if policy == nil {
			rebuildErr = fmt.Errorf("breach-stop recovery requires a live budget projection under the goal-revision lock")
		} else {
			req, release, rebuildErr = policy.BreachStop(e, taken)
		}
	} else {
		req, rebuildErr = requestForEntry(e, taken)
	}
	if release != nil {
		defer release()
	}
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
		Endpoint:    e,
		Actor:       actorFromEntry(entry),
		Ulid:        entry.Opid[:26],
		Now:         timeNowUTC(),
		ApprovedRef: entry.Intent.Args["approvedRef"],
	}
	if raw := entry.Intent.Args["claimEpoch"]; raw != "" {
		epoch, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || epoch < 1 {
			return PublishRequest{}, fmt.Errorf("the stored intent carries invalid claimEpoch %q; close it by hand", raw)
		}
		r.ClaimEpoch = epoch
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
	case "classify-sweep":
		by := strings.TrimSpace(in.Args["by"])
		if by == "" {
			return PublishRequest{}, fmt.Errorf("the stored classify-sweep confirmation carries no human; close it by hand")
		}
		r.Actor.Human = by
		return installTierLawRequest(r), nil
	case "answer":
		step, err := strconv.ParseInt(in.Args["step"], 10, 64)
		if err != nil || step < 1 {
			return PublishRequest{}, fmt.Errorf("the stored answer step is invalid; close it by hand")
		}
		proof := AnswerProof{Provider: in.Args["provider"], User: in.Args["user"], Ref: in.Args["ref"], Step: step}
		if proof.Provider == "" || proof.User == "" || proof.Ref == "" || strings.TrimSpace(in.Args["question"]) == "" || strings.TrimSpace(in.Args["text"]) == "" {
			return PublishRequest{}, fmt.Errorf("the stored answer intent is incomplete; close it by hand")
		}
		return answerRequest(r, target, in.Args["question"], in.Args["text"], in.Args["wants"], proof), nil
	case "open":
		tier := uint64(3)
		if in.Args["tier"] != "" {
			parsedTier, err := strconv.ParseUint(in.Args["tier"], 10, 8)
			if err != nil || parsedTier < 1 || parsedTier > 3 {
				return PublishRequest{}, fmt.Errorf("the stored open tier is invalid; close it by hand")
			}
			tier = parsedTier
		}
		var budget *Budget
		if in.Args["elapsedLimit"] != "" || in.Args["attemptLimit"] != "" ||
			in.Args["reservedJobMinutesLimit"] != "" || in.Args["activeJobLimit"] != "" || in.Args["reviewRoundLimit"] != "" {
			parsedBudget, err := budgetFromIntentArgs(in.Args)
			if err != nil {
				return PublishRequest{}, err
			}
			budget = &parsedBudget
		}
		var risk *RiskRecord
		if in.Args["risk"] != "" {
			parsedRisk, err := ParseRiskRecord(in.Args["risk"], in.Args["basis"])
			if err != nil {
				return PublishRequest{}, fmt.Errorf("the stored open risk is invalid: %w", err)
			}
			risk = &parsedRisk
		}
		return openRequest(r, target, in.Args["intent"], in.Args["origin"], in.Args["next"], uint8(tier), budget, risk, in.Args["why"], commaValues(in.Args["labels"]))
	case "open-claim":
		budget, err := budgetFromIntentArgs(in.Args)
		if err != nil {
			return PublishRequest{}, err
		}
		return openClaimRequest(r, target, in.Args["intent"], in.Args["origin"], in.Args["next"], budget, commaValues(in.Args["labels"]))
	case "claim":
		var budget *Budget
		if in.Args["elapsedLimit"] != "" || in.Args["attemptLimit"] != "" ||
			in.Args["reservedJobMinutesLimit"] != "" || in.Args["activeJobLimit"] != "" {
			parsed, err := budgetFromIntentArgs(in.Args)
			if err != nil {
				return PublishRequest{}, err
			}
			budget = &parsed
		}
		if cascade {
			return claimArcRequest(r, target, budget), nil
		}
		return claimRequest(r, target, budget), nil
	case "set-budget":
		return PublishRequest{}, fmt.Errorf("APPROVAL_REQUIRED: set-budget is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary and close this entry by hand")
	case "split":
		members, err := ParseMemberDraft([]byte(in.Args["members"]), target)
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored split members do not parse: %w", err)
		}
		epoch, err := strconv.ParseInt(in.Args["ratifierClaimEpoch"], 10, 64)
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored split ratifier claim epoch is invalid: %w", err)
		}
		ratification := SplitRatification{
			Tier: in.Args["ratifierTier"], MainID: in.Args["ratifierMainId"],
			ClaimEpoch: epoch, DraftSHA256: in.Args["draftSha256"],
		}
		return splitRequest(r, target, members, ratification, nil)
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
		fields := EditFields{Why: in.Args["why"], Evidence: in.Args["evidence"]}
		var riskScores, riskBasis string
		for _, d := range in.Deltas {
			value := d.New
			switch d.Field {
			case "intent":
				fields.Intent = &value
			case "tier":
				tier, err := strconv.ParseUint(value, 10, 8)
				if err != nil || tier < 1 || tier > 3 {
					return PublishRequest{}, fmt.Errorf("the stored edit tier is invalid; close this entry by hand")
				}
				tierValue := uint8(tier)
				fields.Tier = &tierValue
			case "risk":
				riskScores = value
			case "basis":
				riskBasis = value
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
		if riskScores != "" {
			risk, err := ParseRiskRecord(riskScores, riskBasis)
			if err != nil {
				return PublishRequest{}, fmt.Errorf("the stored edit risk is invalid: %w", err)
			}
			fields.Risk = &risk
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

func recoverSplitConfirmedEffect(e Endpoint, tip string, entry Entry) error {
	if entry.Intent.Verb != "split" {
		return nil
	}
	if len(entry.Intent.Targets) != 1 {
		return fmt.Errorf("confirmed split %s has no unique parent target", entry.Opid)
	}
	return raiseSplitOldArcDebt(e, tip, entry.Intent.Targets[0], entry.Opid, timeNowUTC())
}

func actorFromEntry(entry Entry) Actor {
	return Actor{Machine: entry.Machine, Lineage: entry.Lineage}
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
