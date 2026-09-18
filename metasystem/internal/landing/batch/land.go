package batch

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// LandingProgress is the durable boundary between local construction, the
// single push, and trailer-based recovery.
type LandingProgress struct {
	Base          string            `json:"base"`
	Commits       map[string]string `json:"commits,omitempty"`
	BuildCommits  map[string]string `json:"buildCommits,omitempty"`
	BranchTip     string            `json:"branchTip,omitempty"` // legacy current-tip projection
	CandidateTip  string            `json:"candidateTip,omitempty"`
	ReceiptTip    string            `json:"receiptTip,omitempty"`
	HeldChecked   bool              `json:"heldChecked,omitempty"`
	PushComplete  bool              `json:"pushComplete,omitempty"`
	PushedTip     string            `json:"pushedTip,omitempty"`
	RearmComplete bool              `json:"rearmComplete,omitempty"`
	CleanupDone   bool              `json:"cleanupDone,omitempty"`
	RefusedOrigin string            `json:"refusedOrigin,omitempty"`
	RefusedBase   string            `json:"refusedBase,omitempty"`
	PushRounds    int               `json:"pushRounds,omitempty"`
	PushRejection *PushRejection    `json:"pushRejection,omitempty"`
}

func (progress LandingProgress) candidateTip() string {
	if progress.CandidateTip != "" {
		return progress.CandidateTip
	}
	return progress.BranchTip
}

func (progress LandingProgress) publishedTip() string {
	if progress.ReceiptTip != "" {
		return progress.ReceiptTip
	}
	return progress.candidateTip()
}

func (progress *LandingProgress) recordReceiptTip(tip string) {
	progress.ReceiptTip, progress.BranchTip = tip, tip
}

// PushRejection is the durable held state for a remote refusal that was not
// caused by a stale lease. The green proof remains valid while the endpoint is
// unchanged, so later ticks have no work to repeat.
type PushRejection struct {
	Text      string `json:"text"`
	At        string `json:"at"`
	OriginTip string `json:"originTip"`
}

// MissingLeaseBaseError reports an impossible endpoint transaction: recovery
// cannot compare a commit lease with a tree or an inferred fallback.
type MissingLeaseBaseError struct{}

func (*MissingLeaseBaseError) Error() string {
	return "BATCH_LAND_PUSH_REFUSED: endpoint lease base is absent"
}

type LandSeams struct {
	Prepare            func(base string) error
	Apply              func(Unit) error
	AppendReceipt      func(Unit, PrefixReceipt) error
	Commit             func(Unit, PrefixReceipt) (string, error)
	ApplyBuild         func(Unit, BranchBuild) error
	AppendBuildReceipt func(Unit, BranchBuild, PrefixReceipt) error
	CommitBuild        func(Unit, BranchBuild, PrefixReceipt) (string, error)
	Held               func(base, tip string) error
	PublishBranch      func(expected, tip string) error
	Push               func(base, tip string) error
	Reset              func(base string) error
	Cleanup            func() error
	Origin             func() (string, error)
	OriginTree         func(string) (string, error)
	Abandon            func(tip, detachAt string) error
	SeriesOnOrigin     func(origin, tip string) (bool, error)
	LeaseBase          string
	RecoverPush        func(refusedOrigin, base, tip string) (PushRecovery, error)
}

// PushRecovery is the one bounded decision after an endpoint lease refusal.
// A changed proof input reopens the batch; otherwise the rebased, identity-
// verified series is reported as one completed endpoint push.
type PushRecovery struct {
	Origin, BaseTree, Tip string
	Reopen, Pushed        bool
}

const maxRecoveryPushRounds = 3

// LandSeries creates every commit locally in original join order, checks the
// whole range once, and pushes the complete series once.
func LandSeries(store Store, id, actor string, at time.Time, seams LandSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || record.Proof == nil || record.Proof.Status != "green" {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no green landing candidate", id)
	}
	proofOwner := ""
	for index := len(record.History) - 1; index >= 0; index-- {
		entry := record.History[index]
		if entry.To != StateLanding {
			continue
		}
		proofOwner = entry.Actor
		break
	}
	if proofOwner == "" || proofOwner != actor {
		return fmt.Errorf("BATCH_LAND_DELEGATION_REFUSED: landing actor %s is not green-proof owner %s", actor, proofOwner)
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no joined units", id)
	}
	progress := LandingProgress{Base: record.BaseTree, Commits: map[string]string{}, BuildCommits: map[string]string{}}
	if record.Landing != nil {
		progress = *record.Landing
		progress.Commits = cloneStrings(record.Landing.Commits)
		progress.BuildCommits = cloneStrings(record.Landing.BuildCommits)
	}
	origin := record.BaseTree
	if seams.Origin != nil {
		origin, err = seams.Origin()
		if err != nil {
			return err
		}
	}
	originTree := func(commit string) (string, error) {
		if seams.OriginTree != nil {
			return seams.OriginTree(commit)
		}
		return record.BaseTree, nil
	}
	persistProgress := func(clearFailure bool) error {
		return store.Update(id, func(current *Record) error {
			current.Landing = &progress
			if clearFailure && current.Proof != nil {
				current.Proof.Failure = ""
			}
			return nil
		})
	}
	markAlreadyLanded := func(endpoint, candidateTip string) (bool, error) {
		if candidateTip == "" || seams.SeriesOnOrigin == nil {
			return false, nil
		}
		landed, checkErr := seams.SeriesOnOrigin(endpoint, candidateTip)
		if checkErr != nil || !landed {
			return false, checkErr
		}
		progress.PushComplete, progress.PushedTip = true, candidateTip
		progress.recordReceiptTip(candidateTip)
		progress.RefusedOrigin, progress.RefusedBase, progress.PushRounds, progress.PushRejection = "", "", 0, nil
		return true, persistProgress(true)
	}
	holdPushRejection := func(rejectionOrigin string, pushErr error) error {
		progress.RefusedOrigin, progress.RefusedBase = "", ""
		progress.PushRejection = &PushRejection{Text: pushErr.Error(), At: at.UTC().Format(time.RFC3339Nano), OriginTip: rejectionOrigin}
		return store.Update(id, func(current *Record) error {
			current.Landing = &progress
			if current.Proof != nil {
				current.Proof.Failure = "endpoint push held: " + pushErr.Error()
			}
			current.Transition(StateLanding, at, "push-held", actor, pushErr.Error())
			return nil
		})
	}
	abandonAndReopen := func(candidateTip, detachAt, baseTree string) (bool, error) {
		if landed, checkErr := markAlreadyLanded(detachAt, candidateTip); checkErr != nil || landed {
			return landed, checkErr
		}
		if seams.SeriesOnOrigin == nil {
			return false, fmt.Errorf("BATCH_LAND_UNWIRED: already-landed helper is absent")
		}
		if seams.Abandon == nil {
			return false, fmt.Errorf("BATCH_LAND_UNWIRED: abandon helper is absent")
		}
		if err := seams.Abandon(candidateTip, detachAt); err != nil {
			return false, err
		}
		return false, ReopenRefusedPush(store, id, baseTree, actor, at)
	}
	recoverNow := func(recoveryOrigin, candidateTip string, pushErr error) (bool, error) {
		if seams.RecoverPush == nil {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s refused the complete series: %w", recoveryOrigin, pushErr)
		}
		recovery, recoveryErr := seams.RecoverPush(recoveryOrigin, record.BaseTree, candidateTip)
		if recovery.Origin == "" {
			recovery.Origin = recoveryOrigin
		}
		progress.PushRejection = nil
		if recovery.Tip != "" {
			// PublishLandingBranch completed. Preserve its lease tip even
			// when the following endpoint transaction was refused.
			progress.recordReceiptTip(recovery.Tip)
		}
		if recoveryErr != nil && IsNonLeaseEndpointRejection(recoveryErr) {
			if storeErr := holdPushRejection(recovery.Origin, recoveryErr); storeErr != nil {
				return false, errors.Join(pushErr, recoveryErr, storeErr)
			}
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s held after remote rejection: %w", recovery.Origin, recoveryErr)
		}
		progress.RefusedOrigin, progress.RefusedBase = recovery.Origin, recoveryOrigin
		if recoveryErr != nil {
			progress.PushRounds++
		}
		if storeErr := persistProgress(true); storeErr != nil {
			return false, errors.Join(pushErr, recoveryErr, storeErr)
		}
		if recoveryErr != nil {
			if progress.PushRounds >= maxRecoveryPushRounds {
				baseTree := recovery.BaseTree
				if baseTree == "" {
					baseTree, err = originTree(recovery.Origin)
					if err != nil {
						return false, err
					}
				}
				return abandonAndReopen(progress.publishedTip(), recovery.Origin, baseTree)
			}
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s recovery failed: %w", recoveryOrigin, errors.Join(pushErr, recoveryErr))
		}
		if recovery.Reopen {
			if landed, checkErr := markAlreadyLanded(recovery.Origin, candidateTip); checkErr != nil || landed {
				return landed, checkErr
			}
			if seams.SeriesOnOrigin == nil {
				return false, fmt.Errorf("BATCH_LAND_UNWIRED: already-landed helper is absent")
			}
			if seams.Abandon == nil {
				return false, fmt.Errorf("BATCH_LAND_UNWIRED: abandon helper is absent")
			}
			if abandonErr := seams.Abandon(candidateTip, recovery.Origin); abandonErr != nil {
				return false, abandonErr
			}
			return false, ReopenMovedTrunk(store, id, recovery.BaseTree, actor, at)
		}
		if !recovery.Pushed || recovery.Tip == "" {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s is unchanged after the refused push: %w", recoveryOrigin, pushErr)
		}
		progress.PushComplete, progress.PushedTip = true, recovery.Tip
		progress.recordReceiptTip(recovery.Tip)
		progress.RefusedOrigin, progress.RefusedBase, progress.PushRounds = "", "", 0
		if storeErr := persistProgress(true); storeErr != nil {
			return false, storeErr
		}
		return true, nil
	}
	attemptPush := func(candidateTip string) (bool, error) {
		if seams.Push == nil {
			return false, fmt.Errorf("BATCH_LAND_UNWIRED: push helper is absent")
		}
		pushErr := seams.Push(record.BaseTree, candidateTip)
		if pushErr == nil {
			progress.PushComplete, progress.PushedTip = true, candidateTip
			progress.RefusedOrigin, progress.RefusedBase, progress.PushRounds, progress.PushRejection = "", "", 0, nil
			return true, persistProgress(true)
		}
		// A retry after a durable non-lease hold must itself identify a stale
		// lease before recovery is allowed to reopen the candidate.
		if seams.Origin != nil {
			origin, err = seams.Origin()
			if err != nil {
				return false, errors.Join(pushErr, err)
			}
		}
		if landed, checkErr := markAlreadyLanded(origin, candidateTip); checkErr != nil {
			return false, errors.Join(pushErr, checkErr)
		} else if landed {
			// The endpoint accepted the atomic transaction but its acknowledgement
			// was lost. P6 recovery will finalize the already-published units.
			return true, nil
		}
		if IsNonLeaseEndpointRejection(pushErr) {
			if storeErr := holdPushRejection(origin, pushErr); storeErr != nil {
				return false, errors.Join(pushErr, storeErr)
			}
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s held after remote rejection: %w", origin, pushErr)
		}
		if !IsStaleEndpointLease(pushErr) {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: endpoint push failed without a stale lease: %w", pushErr)
		}
		if seams.LeaseBase == "" {
			return false, &MissingLeaseBaseError{}
		}
		progress.PushRejection = nil
		progress.RefusedBase, progress.RefusedOrigin = seams.LeaseBase, origin
		if storeErr := persistProgress(false); storeErr != nil {
			return false, errors.Join(pushErr, storeErr)
		}
		if origin == progress.RefusedBase {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s is unchanged after the refused push: %w", origin, pushErr)
		}
		return recoverNow(origin, candidateTip, pushErr)
	}
	if !progress.PushComplete && progress.publishedTip() != "" {
		if landed, checkErr := markAlreadyLanded(origin, progress.publishedTip()); checkErr != nil {
			return checkErr
		} else if landed {
			return nil
		}
	}
	if !progress.PushComplete && progress.PushRejection != nil {
		if origin == progress.PushRejection.OriginTip {
			return nil
		}
		candidateTip := progress.publishedTip()
		if candidateTip == "" {
			candidateTip = progress.Commits[units[len(units)-1].GoalID]
		}
		if candidateTip == "" {
			return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: held landing has no candidate tip")
		}
		pushed, pushErr := attemptPush(candidateTip)
		if pushErr != nil {
			return pushErr
		}
		if !pushed {
			return nil
		}
	}
	if !progress.PushComplete && progress.RefusedBase != "" {
		candidateTip := progress.publishedTip()
		if candidateTip == "" {
			candidateTip = progress.Commits[units[len(units)-1].GoalID]
		}
		if candidateTip == "" {
			return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: refused landing has no candidate tip")
		}
		if progress.RefusedBase == origin {
			baseTree, treeErr := originTree(origin)
			if treeErr != nil {
				return treeErr
			}
			_, reopenErr := abandonAndReopen(candidateTip, origin, baseTree)
			return reopenErr
		}
		pushed, recoveryErr := recoverNow(origin, candidateTip, errors.New("prior endpoint refusal"))
		if recoveryErr != nil {
			return recoveryErr
		}
		if !pushed {
			return nil
		}
	}
	if !progress.PushComplete {
		// A process may die after any local commit. Rebuild the complete local
		// stack from the recorded base so recovery never trusts an unproved
		// worktree shape or pushes a recorded prefix.
		progress.Commits = map[string]string{}
		progress.BuildCommits = map[string]string{}
		progress.HeldChecked = false
		if seams.Prepare != nil {
			if err := seams.Prepare(record.BaseTree); err != nil {
				return err
			}
		}
	}
	for index, unit := range units {
		if progress.Commits[unit.GoalID] != "" {
			continue
		}
		receipt, ok := record.Receipts[unit.GoalID]
		if index == len(units)-1 && !ok {
			receipt = PrefixReceipt{GoalID: unit.GoalID, Tree: record.TipTree, AttemptID: record.Proof.AttemptID,
				CommitIDs: slices.Clone(unit.CommitIDs), LastUnit: unit.LastUnit,
				Reused: cloneStrings(record.Proof.Reuse), Executed: slices.Clone(record.Proof.Executions)}
			for _, build := range unit.Builds {
				receipt.Units = append(receipt.Units, build.Units...)
			}
			ok = true
		}
		if !ok || receipt.Tree != record.PrefixTrees[index] {
			return fmt.Errorf("BATCH_LAND_RECEIPT_REFUSED: unit %s has no exact prefix receipt", unit.GoalID)
		}
		if len(unit.Builds) != 0 {
			if seams.ApplyBuild == nil || seams.AppendBuildReceipt == nil || seams.CommitBuild == nil {
				return fmt.Errorf("BATCH_LAND_UNWIRED: branch member helpers are incomplete")
			}
			for _, build := range unit.Builds {
				if progress.BuildCommits[build.Commit] != "" {
					continue
				}
				if err = seams.ApplyBuild(unit, build); err == nil {
					err = seams.AppendBuildReceipt(unit, build, receipt)
				}
				var commit string
				if err == nil {
					commit, err = seams.CommitBuild(unit, build, receipt)
				}
				if err != nil {
					return ejectRefusedMember(store, id, actor, at, record.BaseTree, unit, err, seams.Reset)
				}
				if commit == "" {
					return fmt.Errorf("BATCH_LAND_COMMIT_REFUSED: build %s returned no commit", build.Commit)
				}
				progress.BuildCommits[build.Commit] = commit
				progress.Commits[unit.GoalID] = commit
				if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
					return err
				}
			}
			continue
		}
		if seams.Apply == nil || seams.AppendReceipt == nil || seams.Commit == nil {
			return fmt.Errorf("BATCH_LAND_UNWIRED: local series helpers are incomplete")
		}
		if err = seams.Apply(unit); err == nil {
			err = seams.AppendReceipt(unit, receipt)
		}
		var commit string
		if err == nil {
			commit, err = seams.Commit(unit, receipt)
		}
		if err != nil {
			return ejectRefusedMember(store, id, actor, at, record.BaseTree, unit, err, seams.Reset)
		}
		if commit == "" {
			return fmt.Errorf("BATCH_LAND_COMMIT_REFUSED: unit %s returned no commit", unit.GoalID)
		}
		progress.Commits[unit.GoalID] = commit
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	tip := progress.Commits[units[len(units)-1].GoalID]
	if !progress.PushComplete && seams.PublishBranch != nil {
		if err := seams.PublishBranch(progress.publishedTip(), tip); err != nil {
			return err
		}
		progress.recordReceiptTip(tip)
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	if !progress.HeldChecked {
		if seams.Held == nil {
			return fmt.Errorf("BATCH_LAND_HELD_REFUSED: held helper is absent")
		}
		if heldErr := seams.Held(record.BaseTree, tip); heldErr != nil {
			return fmt.Errorf("BATCH_LAND_HELD_REFUSED: complete series did not pass held: %w", heldErr)
		}
		progress.HeldChecked = true
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	if !progress.PushComplete {
		pushed, pushErr := attemptPush(tip)
		if pushErr != nil {
			return pushErr
		}
		if !pushed {
			return nil
		}
	}
	if seams.Cleanup != nil {
		if err := seams.Cleanup(); err != nil {
			return err
		}
		progress.CleanupDone = true
		return store.Update(id, func(current *Record) error { current.Landing = &progress; return nil })
	}
	return nil
}

// ReopenMovedTrunk makes a proof-input-changing origin move cross seal and
// proof again on the new base. It never retries the old landing candidate.
func ReopenMovedTrunk(store Store, id, newBaseTree, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, actor, at, false, "selected proof input moved")
}

// ReopenRefusedPush gives up a bounded endpoint transaction. Unlike a moved-
// input reopen, the endpoint may still be the batch's recorded base.
func ReopenRefusedPush(store Store, id, newBaseTree, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, actor, at, true, "endpoint push refused")
}

func reopenLandingCandidate(store Store, id, newBaseTree, actor string, at time.Time, allowSameBase bool, detail string) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || newBaseTree == "" || (!allowSameBase && newBaseTree == record.BaseTree) {
		return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: moved trunk did not supply a new landing base")
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no joined units", id)
	}
	prefixes, err := assembleUnits(store.root, newBaseTree, units)
	if err != nil {
		var conflict *assemblyConflict
		if !errors.As(err, &conflict) {
			return err
		}
		survivors := slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.GoalID == conflict.GoalID })
		var survivorPrefixes []string
		if len(survivors) != 0 {
			survivorPrefixes, err = assembleUnits(store.root, newBaseTree, survivors)
			if err != nil {
				return err
			}
		}
		branchTip := ""
		if record.Landing != nil {
			if len(survivors) == 0 {
				if err := DeleteLandingBranch(store.root, id, record.Landing.publishedTip()); err != nil {
					return err
				}
			} else if slices.ContainsFunc(survivors, func(unit Unit) bool { return len(unit.Builds) != 0 }) {
				branchTip, err = RebuildLandingBranch(store.root, id, newBaseTree, record.Landing.publishedTip(), actor, survivors)
				if err != nil {
					return err
				}
			}
		}
		return store.Update(id, func(current *Record) error {
			current.BaseTree = newBaseTree
			if returnErr := requestUnitReturn(current, conflict.GoalID, UnitEjected, conflict.Error(), actor, at); returnErr != nil {
				return returnErr
			}
			current.SelectedGroups, current.Seal, current.Proof, current.Receipts, current.Landing = nil, nil, nil, nil, nil
			if len(survivors) == 0 {
				current.PrefixTrees, current.TipTree = nil, newBaseTree
				current.Transition(StateDissolved, at, "reassemble", actor, "no survivors")
				return nil
			}
			if branchTip != "" {
				current.Landing = &LandingProgress{Base: newBaseTree, BranchTip: branchTip, CandidateTip: branchTip}
			}
			current.PrefixTrees, current.TipTree, current.ClosedReason = survivorPrefixes, survivorPrefixes[len(survivorPrefixes)-1], ""
			current.Transition(StateOpen, at, "reassemble", actor, "survivors")
			return nil
		})
	}
	return store.Update(id, func(current *Record) error {
		current.BaseTree, current.PrefixTrees, current.TipTree = newBaseTree, prefixes, prefixes[len(prefixes)-1]
		current.SelectedGroups, current.Seal, current.Proof, current.Receipts, current.Landing = nil, nil, nil, nil, nil
		current.ClosedReason = ""
		current.Transition(StateOpen, at, "trunk-moved", actor, detail)
		return nil
	})
}

func ejectRefusedMember(store Store, id, actor string, at time.Time, base string, unit Unit, err error, reset func(string) error) error {
	err = fmt.Errorf("commit %s refused: %w", unit.GoalID, err)
	if reset != nil {
		if resetErr := reset(base); resetErr != nil {
			return fmt.Errorf("%v; reset: %w", err, resetErr)
		}
	}
	if returnErr := RequestReturn(store, id, unit.GoalID, UnitEjected, err.Error(), actor, at); returnErr != nil {
		return returnErr
	}
	return ReassembleSurvivors(store, id, actor, at)
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
