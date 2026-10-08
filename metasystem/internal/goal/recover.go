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
	"encoding/json"
	"errors"
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

// ParkRecoveryPolicy supplies the live branch precondition to a recovered
// park. Journal text cannot reconstruct the remote branch observation.
type ParkRecoveryPolicy interface {
	ParkBranchCheck(Endpoint) func(goalID, next string) (string, error)
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
		report, err := recoverEntry(e, tip, entry, policy)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// RecoverDeadBlocker runs the one rule for a single pushed entry that
// blocks this clone, and only when its owner is provably dead: the opid on a
// fresh capture decides first, and a dead owner's work is completed from its
// stored intent, never pushed blindly. It reports whether it acted; a live
// owner, or one whose liveness cannot be proved, is left alone.
func RecoverDeadBlocker(e Endpoint, blocking Entry, policy SensitiveRecoveryPolicy) (RecoveryReport, bool, error) {
	if blocking.Phase != PhasePushed || OwnerAlive(blocking) {
		return RecoveryReport{}, false, nil
	}
	nonce, err := readNonce()
	if err != nil {
		return RecoveryReport{}, false, err
	}
	tip, err := CaptureTip(e, nonce)
	CleanupRefs(e, nonce)
	if err != nil {
		return RecoveryReport{}, false, err
	}
	// Re-read after the capture: another process may have classified it.
	entry, err := ReadEntry(e.Root, blocking.Opid)
	if err != nil {
		return RecoveryReport{}, false, err
	}
	if entry.Phase != PhasePushed || OwnerAlive(entry) {
		return RecoveryReport{}, false, nil
	}
	report, err := recoverEntry(e, tip, entry, policy)
	return report, true, err
}

// recoverEntry is the one rule for one entry on the captured tip.
func recoverEntry(e Endpoint, tip string, entry Entry, policy SensitiveRecoveryPolicy) (RecoveryReport, error) {
	present, trErr := TrailerPresent(e, tip, entry.Opid)
	if trErr != nil {
		return RecoveryReport{}, trErr
	}
	post := PostconditionAbsent
	if present {
		post = PostconditionPresent
	}
	action := ClassifyRecovery(entry, post, OwnerAlive(entry), callerIsOwner(entry), PastDeadline(entry, timeNowUTC()))
	report := RecoveryReport{Opid: entry.Opid, Action: action}
	switch action {
	case ActionConfirm:
		if err := recoverConfirmedEffect(e, tip, entry); err != nil {
			return report, err
		}
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeConfirmed, "opid found on "+short(tip)+" by recovery"); err != nil {
			return report, err
		}
		// Accepted advances only onto a VALIDATED tip, recovery
		// included, and a refused advance is said in the
		// report — never discarded.
		if valErr := validateCommitFor(e, tip); valErr != nil {
			report.Detail = "confirmed on the canonical tip; accepted NOT advanced (the tip does not validate): " + valErr.Error()
		} else if advErr := advanceAcceptedFor(e, tip); advErr != nil {
			report.Detail = "confirmed on the canonical tip; accepted NOT advanced: " + advErr.Error()
		} else {
			report.Detail = "confirmed on the canonical tip"
		}
		CleanupRefs(e, entry.Opid)
	case ActionConfirmLate:
		if err := recoverConfirmedEffect(e, tip, entry); err != nil {
			return report, err
		}
		if err := CorrectLate(e.Root, entry.Opid, "opid found on "+short(tip)+" by recovery"); err != nil {
			return report, err
		}
		report.Detail = "belief corrected to confirmed-late"
	case ActionComplete:
		if entry.Intent.Verb == "slice-start" && entry.Intent.Args["by"] == "" {
			if err := MarkTerminal(e.Root, entry.Opid, OutcomeAbandoned, "slice-start owner died before its postcondition landed; dispatch never acquired reservation authority"); err != nil {
				return report, err
			}
			CleanupRefs(e, entry.Opid)
			report.Detail = "slice-start abandoned without marking the goal sliced; no reservation was authorized"
			break
		}
		detail, err := completeFromIntent(e, entry, policy)
		if err != nil {
			return report, err
		}
		report.Detail = detail
	case ActionLeaveToOwner, ActionKeepRetrying:
		report.Detail = "a live owner's entry; untouched"
	case ActionAbandonOwn:
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeAbandoned, "the owner abandons its never-pushed work"); err != nil {
			return report, err
		}
		CleanupRefs(e, entry.Opid)
		report.Detail = "abandoned by its owner"
	case ActionExpireOwn:
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeExpired, "the owner's deadline passed"); err != nil {
			return report, err
		}
		CleanupRefs(e, entry.Opid)
		report.Detail = "expired at its own deadline"
	default:
		report.Detail = "nothing to do"
	}
	return report, nil
}

// completeFromIntent takes over a dead owner's operation. A stored human name
// closes rejected because journal text is never an authority credential;
// otherwise recovery rebuilds the mutation and runs the same transaction loop
// the living use with the original opid and intent.
func completeFromIntent(e Endpoint, entry Entry, policy SensitiveRecoveryPolicy) (string, error) {
	taken, err := TakeOver(e.Root, entry.Opid)
	if err != nil {
		return "", err
	}
	// A stored human name records who intended the act; it cannot become the
	// credential that authorizes replay as that person.
	journaledHuman := taken.Intent.Args["by"] != ""
	if journaledHuman && !hasConditionalRecoveryHumanBoundary(taken.Intent.Verb) {
		return refuseJournaledHumanIntent(e, taken, recoveryHumanBoundaryDetail(taken.Intent.Verb, nil))
	}
	if taken.Intent.Verb == "resume" {
		detail := "an interrupted resume was a person's act and can't be finished for them; the person runs it again: metasystem goal resume " + recoveryTarget(taken.Intent)
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	if taken.Intent.Verb == "set-priority" {
		detail := "an interrupted prioritize was a person's act and can't be finished for them; the person runs metasystem goal prioritize again"
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	if taken.Intent.Verb == "steal" {
		detail := "an interrupted take-over was a person's act and can't be finished for them; the person runs it again: metasystem goal claim " + recoveryTarget(taken.Intent) + " --take-over --reason TEXT"
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "escalation required: " + detail, nil
	}
	if taken.Intent.Verb == "split" && taken.Intent.Args["ratifierTier"] == RatifierHuman {
		detail := "an interrupted split was a person's act and can't be finished for them; the person runs metasystem goal split " + recoveryTarget(taken.Intent) + " again"
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
		var parkCheck func(string, string) (string, error)
		if taken.Intent.Verb == "park" {
			if parkPolicy, ok := policy.(ParkRecoveryPolicy); ok {
				parkCheck = parkPolicy.ParkBranchCheck(e)
			}
			if journaledHuman && parkCheck == nil {
				parkCheck = func(string, string) (string, error) { return "", nil }
			}
		}
		req, rebuildErr = requestForEntry(e, taken, parkCheck)
	}
	if release != nil {
		defer release()
	}
	if rebuildErr != nil {
		if journaledHuman {
			return refuseJournaledHumanIntent(e, taken, recoveryHumanBoundaryDetail(taken.Intent.Verb, nil))
		}
		// A verb recovery cannot rebuild generically terminalizes
		// toward its own re-runnable entry point, named — never a
		// silent wedge (the pushed block clears).
		if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, RecordText(rebuildErr)); err != nil {
			return "", err
		}
		CleanupRefs(e, entry.Opid)
		return "not rebuildable: " + RecordText(rebuildErr), nil
	}
	req = recoveryHumanBoundaryRequest(req, journaledHuman)
	res, err := runTransaction(e, req)
	if err != nil {
		return "", err
	}
	return "completed from the stored intent: " + string(res.Outcome), nil
}

// recoveryHumanBoundaryRequest leaves ordinary replay unchanged. For the
// three stopping verbs, it translates the real verb's conditional human-proof
// requirement and rejects every stored human name with the journal-specific
// fresh-boundary remedy.
func recoveryHumanBoundaryRequest(req PublishRequest, journaledHuman bool) PublishRequest {
	if !hasConditionalRecoveryHumanBoundary(req.Intent.Verb) {
		return req
	}
	verb := req.Intent.Verb
	mutate := req.Mutate
	req.Mutate = func(tip string) ([]Change, error) {
		changes, err := mutate(tip)
		if err != nil {
			var required humanAuthorityRequired
			if errors.As(err, &required) {
				return nil, errors.New(recoveryHumanBoundaryDetail(verb, &required))
			}
			var unavailable parkBranchSafetyUnavailable
			if journaledHuman && errors.As(err, &unavailable) {
				return nil, errors.New(recoveryHumanBoundaryDetail(verb, nil))
			}
			return changes, err
		}
		if journaledHuman {
			return nil, errors.New(recoveryHumanBoundaryDetail(verb, nil))
		}
		return changes, err
	}
	return req
}

func refuseJournaledHumanIntent(e Endpoint, entry Entry, detail string) (string, error) {
	if err := MarkTerminal(e.Root, entry.Opid, OutcomeRejected, detail); err != nil {
		return "", err
	}
	CleanupRefs(e, entry.Opid)
	return "escalation required: " + detail, nil
}

func hasConditionalRecoveryHumanBoundary(verb string) bool {
	switch verb {
	case "park", "unpark", "release", "unblock":
		return true
	default:
		return false
	}
}

func recoveryHumanBoundaryDetail(verb string, required *humanAuthorityRequired) string {
	detail := fmt.Sprintf("an interrupted %s was a person's act and can't be finished for them; the person runs it again", verb)
	if required != nil {
		detail += fmt.Sprintf(" (%s needs a %s check of who acts)", required.row.Name, required.grade)
	}
	return detail
}

// requestForEntry rebuilds the COMPLETE verb request from the
// entry's stored intent through the SAME constructors the live
// verbs run: cascades, Goal-free clears, authority checks,
// displacement markers, and acknowledgment piggybacks all replay
// identically, and the derived opid — the entry's ulid under the
// entry's pair — IS the entry's opid, so every rebuilt history
// line carries the original operation with no injection seam.
func requestForEntry(e Endpoint, entry Entry, parkChecks ...func(string, string) (string, error)) (PublishRequest, error) {
	if len(entry.Opid) < 27 {
		return PublishRequest{}, fmt.Errorf("pending ledger write %q has a malformed id; close it by hand", entry.Opid)
	}
	r := VerbRequest{
		Endpoint:    e,
		Actor:       actorFromEntry(entry),
		Ulid:        entry.Opid[:26],
		Now:         timeNowUTC(),
		ApprovedRef: entry.Intent.Args["approvedRef"],
	}
	if len(parkChecks) != 0 {
		r.ParkBranchCheck = parkChecks[0]
	}
	if raw := entry.Intent.Args["claimEpoch"]; raw != "" {
		epoch, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || epoch < 0 {
			return PublishRequest{}, fmt.Errorf("the stored intent carries invalid claimEpoch %q; close it by hand", raw)
		}
		r.ClaimEpoch = epoch
	}
	r.CallerClass = entry.Intent.Args["callerClass"]
	if r.opid() != entry.Opid {
		return PublishRequest{}, fmt.Errorf("pending ledger write %s doesn't match the session it names; close it by hand", entry.Opid)
	}
	in := entry.Intent
	target := ""
	if len(in.Targets) > 0 {
		target = in.Targets[0]
	}
	cascade := in.Args["cascade"] == "arc"
	switch in.Verb {
	case "record-flake":
		var args FlakeRecordArgs
		if err := json.Unmarshal([]byte(in.Args["flake"]), &args); err != nil {
			return PublishRequest{}, fmt.Errorf("the stored flaky test sighting is malformed; close it by hand: %v", err)
		}
		at, err := time.Parse(time.RFC3339, in.Args["at"])
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored flaky test sighting has an invalid time; close it by hand")
		}
		r.Now = at
		return flakeRecordRequest(r, args)
	case "trunk-red-record":
		var args TrunkRedRecordArgs
		if err := json.Unmarshal([]byte(in.Args["red"]), &args); err != nil {
			return PublishRequest{}, fmt.Errorf("the stored trunk-red record is malformed; close it by hand: %v", err)
		}
		return trunkRedRecordRequest(r, args), nil
	case "trunk-red-own":
		return trunkRedOwnRequest(r, TrunkRedOwnArgs{Entry: target, Goal: in.Args["goal"], Branch: in.Args["branch"],
			BranchCommit: in.Args["branchCommit"], To: in.Args["to"], By: in.Args["by"]}), nil
	case "trunk-red-clear":
		branchMerged, err := strconv.ParseBool(in.Args["branchMerged"])
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored trunk-red clear has an invalid branchMerged value; close it by hand")
		}
		args := TrunkRedClearArgs{Entry: in.Args["entry"], Attempt: in.Args["attempt"],
			BaseCommit: in.Args["baseCommit"], BaseTree: in.Args["baseTree"], Group: in.Args["group"], BranchMerged: branchMerged,
			Executed: in.Args["executed"] == "true", FixCommit: in.Args["fixCommit"]}
		expectedEntry, hasExpectedEntry := in.Args["expectedEntry"]
		if hasExpectedEntry {
			if err := json.Unmarshal([]byte(expectedEntry), &args.ExpectedEntry); err != nil {
				return PublishRequest{}, fmt.Errorf("the stored trunk-red clear has no readable entry binding; close it by hand")
			}
		} else {
			args.legacyUnbound = true
		}
		return trunkRedClearRequest(r, args), nil
	case "carrying":
		return carryingRequest(r, CarryingArgs{Goal: target, ApprovedRef: in.Args["approvedRef"], Workspace: in.Args["workspace"], Project: in.Args["tree"], By: in.Args["by"]}), nil
	case "carried":
		return carriedRequestFromIntentMode(e, entry, true)
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
		// Both directions are rebuilt, in the order they were named, so a
		// replayed open carries the goals it blocks and the goals it waits
		// for rather than half of its dependencies.
		// A replay carries no proof, so it is a seat's hand whatever name the
		// journal recorded — and a journaled name is refused above, so the
		// opens that reach here are the seat's own and are judged as such.
		return openRequest(r, target, in.Args["intent"], in.Args["origin"], in.Args["next"],
			commaValues(in.Args["blocks"]), commaValues(in.Args["blockedBy"]), uint8(tier), budget, risk, in.Args["why"], false, false, commaValues(in.Args["labels"]))
	case "block":
		return blockRequest(r, target, in.Args["blocker"], false), nil
	case "unblock":
		// An early unblock never reaches the mutation with a proof here: the
		// conditional boundary below refuses it by name, and a satisfied edge
		// needs none.
		return unblockRequest(r, target, in.Args["blocker"], nil), nil
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
	case "restamp":
		if r.CallerClass != "MAIN" {
			return PublishRequest{}, errors.New("the interrupted restamp wasn't made by the session holding the checkout; close it by hand")
		}
		r.EpochAuthority = EpochAuthorityHolder
		return restampRequest(r, target), nil
	case "set-budget":
		return PublishRequest{}, coded("APPROVAL_REQUIRED", fmt.Errorf("an interrupted budget change was a person's act; close it by hand\nrun: metasystem goal budget %s  (as that person)", target))
	case "extend-budget":
		parse := func(key string) (uint64, error) {
			value, err := strconv.ParseUint(in.Args[key], 10, 64)
			if err != nil || value == 0 {
				return 0, fmt.Errorf("the stored extend-budget %s is invalid; close it by hand", key)
			}
			return value, nil
		}
		attemptFrom, err := parse("attemptLimitFrom")
		if err != nil {
			return PublishRequest{}, err
		}
		attemptTo, err := parse("attemptLimitTo")
		if err != nil {
			return PublishRequest{}, err
		}
		reservedFrom, err := parse("reservedJobMinutesFrom")
		if err != nil {
			return PublishRequest{}, err
		}
		reservedTo, err := parse("reservedJobMinutesTo")
		if err != nil {
			return PublishRequest{}, err
		}
		offer := BudgetExtensionOffer{
			EvidenceKind: in.Args["evidenceKind"], EvidenceID: in.Args["evidenceId"], EvidenceAt: in.Args["evidenceAt"],
			AttemptLimitFrom: attemptFrom, AttemptLimitTo: attemptTo,
			ReservedJobMinutesFrom: reservedFrom, ReservedJobMinutesTo: reservedTo,
		}
		return extendBudgetRequest(r, target, offer), nil
	case "grant", "revoke":
		return PublishRequest{}, fmt.Errorf("an interrupted grant %s was a person's act; close it by hand, then the person runs it again", in.Verb)
	case "set-priority":
		return PublishRequest{}, errors.New("an interrupted prioritize was a person's act; the person runs metasystem goal prioritize again")
	case "engine-floor":
		return PublishRequest{}, errors.New("engine-floor no longer exists, so its interrupted write can't be finished; close it by hand")
	case "abandon", "carry", reviewVerb, LandWithoutSittingVerb:
		return PublishRequest{}, fmt.Errorf("an interrupted %s was a person's act and can't be finished for them; the person runs it again", in.Verb)
	case "split":
		members, err := ParseMemberDraft([]byte(in.Args["members"]), target)
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored split members do not parse: %w", err)
		}
		epoch, err := strconv.ParseInt(in.Args["ratifierClaimEpoch"], 10, 64)
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the interrupted split names an unreadable claim: %w", err)
		}
		ratification := SplitRatification{
			Tier: in.Args["ratifierTier"], MainID: in.Args["ratifierMainId"],
			ClaimEpoch: epoch, DraftSHA256: in.Args["draftSha256"],
		}
		return splitRequest(r, target, members, ratification, nil)
	case "release":
		if cascade {
			return releaseArcRequestWithReason(r, target, in.Args["reason"]), nil
		}
		return releaseRequestWithReason(r, target, in.Args["reason"]), nil
	case "land-ready":
		return landReadyRequest(r, target), nil
	case LandedVerb:
		return landedRequest(r, target, in.Args["reason"]), nil
	case "rebase":
		return rebasedRequest(r, target, in.Args["reason"]), nil
	case sendBackVerb:
		answer, err := parseSendBackReason(in.Args["reason"])
		if err != nil {
			return PublishRequest{}, fmt.Errorf("the stored send-back answer does not parse; close it by hand: %w", err)
		}
		return answerSendBackRequest(r, target, answer), nil
	case "done":
		r.ForceBy = in.Args["force"]
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
		if in.Args["under"] != "" {
			return PublishRequest{}, fmt.Errorf("an interrupted unpark under a grant is judged at the moment; close it by hand\nrun: metasystem goal resume %s --under GRANT --verified TEXT", target)
		}
		return unparkRequest(r, target, ""), nil
	case "reopen":
		if in.Args["from"] == "abandoned" {
			return PublishRequest{}, fmt.Errorf("an interrupted reopen was a person's act; the person runs metasystem goal reopen %s again", target)
		}
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
			case "nextAppend":
				fields.NextStepAppend = &value
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
			case "permission":
				change, err := parsePermissionDelta(value)
				if err != nil {
					return PublishRequest{}, fmt.Errorf("%v; close this entry by hand", err)
				}
				if change.Allowed {
					return PublishRequest{}, fmt.Errorf("an interrupted permission change was a person's act; the person runs it again: %s", AllowCommand(target, change.Name))
				}
				fields.Permission = &change
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
		return stealRequestWithReason(r, target, in.Args["reason"]), nil
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
	return PublishRequest{}, fmt.Errorf("an interrupted %s is finished by running it again (from its checkout or reviewed inputs)", in.Verb)
}

func recoverConfirmedEffect(e Endpoint, tip string, entry Entry) error {
	switch entry.Intent.Verb {
	case "split":
		if len(entry.Intent.Targets) != 1 {
			return fmt.Errorf("confirmed split %s has no unique parent target", entry.Opid)
		}
		return splitAfterConfirmed(e, tip, entry.Intent.Targets[0], entry.Opid, timeNowUTC())
	case "carried":
		return carriedAfterConfirmed(e, entry.Intent.Args["approvedRef"], timeNowUTC())(tip)
	default:
		return nil
	}
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

// recoveryTarget is the goal an interrupted act names, or G when it names
// none: the goal a person's rerun names.
func recoveryTarget(in Intent) string {
	if len(in.Targets) > 0 && in.Targets[0] != "" {
		return in.Targets[0]
	}
	return "G"
}
