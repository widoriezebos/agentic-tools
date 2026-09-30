package batch

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
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
	return "main moved and the batch kept no base to compare it with; metasystem landing status shows the batch"
}

type LandSeams struct {
	Prepare            func(base string) error
	Apply              func(Unit) error
	AppendReceipt      func(Unit, PrefixReceipt) error
	Commit             func(Unit, PrefixReceipt) (string, error)
	ApplyBuild         func(Unit, BranchBuild) error
	AppendBuildReceipt func(Unit, BranchBuild, PrefixReceipt) error
	CommitBuild        func(Unit, BranchBuild, PrefixReceipt) (string, error)
	// ReplayChange lands a change member's commit on the landing branch
	// with its Landing-Change trailer and returns the new commit.
	ReplayChange   func(Unit) (string, error)
	Held           func(base, tip string) error
	VerifySeries   func(units []Unit, commits map[string]string) error
	PublishBranch  func(expected, tip string) error
	Push           func(base, tip string) error
	Reset          func(base string) error
	Cleanup        func() error
	Origin         func() (string, error)
	OriginTree     func(string) (string, error)
	Abandon        func(tip, detachAt string) error
	SeriesOnOrigin func(origin, tip string) (bool, error)
	LeaseBase      string
	// RecoverPush rebases, re-verifies and pushes a refused series. It calls
	// recheck immediately before its own endpoint push, so a known flake's
	// allowance that expired while it rebased publishes nothing (BL3S-01).
	RecoverPush func(refusedOrigin, base, tip string, recheck func() error) (PushRecovery, error)
	// FlakeRegister and Now recheck, immediately before any publication, the
	// known flakes a composed proof landed on (BL3S-01).
	FlakeRegister func() ([]OpenEntry, error)
	Now           func() (time.Time, error)
}

// FlakeAllowanceRefusal is the recheck's refusal to publish a composed proof
// whose known flake is no longer carried: every member was already returned.
type FlakeAllowanceRefusal struct{ Code, Reason string }

func (refusal *FlakeAllowanceRefusal) Error() string { return refusal.Reason }

// PushRecovery is the one bounded decision after an endpoint lease refusal.
// A changed proof input reopens the batch; otherwise the rebased, identity-
// verified series is reported as one completed endpoint push.
type PushRecovery struct {
	Origin, BaseTree, Tip, LandedBy string
	Reopen, Pushed                  bool
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
		return fmt.Errorf("%s: batch %s has no green landing candidate", codeLandStateRefused, id)
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
		return fmt.Errorf("%s: landing actor %s is not %s, who owns the green test run", codeLandDelegationRefused, actor, proofOwner)
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("%s: batch %s has no joined units", codeLandStateRefused, id)
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
			return false, fmt.Errorf("%s: already-landed helper is absent", codeLandUnwired)
		}
		if seams.Abandon == nil {
			return false, fmt.Errorf("%s: abandon helper is absent", codeLandUnwired)
		}
		if err := seams.Abandon(candidateTip, detachAt); err != nil {
			return false, err
		}
		return false, ReopenRefusedPush(store, id, baseTree, actor, at)
	}
	// recheckFlakes refuses publication when a known flake the composed proof
	// used is no longer carried: every member returns and nothing publishes.
	recheckFlakes := func() error {
		if len(record.Proof.Flakes) == 0 {
			return nil
		}
		if seams.FlakeRegister == nil || seams.Now == nil {
			return fmt.Errorf("%s: the flake register recheck is absent", codeLandUnwired)
		}
		open, err := seams.FlakeRegister()
		now, nowErr := seams.Now()
		if err = errors.Join(err, nowErr); err != nil {
			return err
		}
		if carried := flakesStillCarried(record.Proof.Flakes, open, now); carried != nil {
			if seams.Reset != nil {
				if err := seams.Reset(record.BaseTree); err != nil {
					return errors.Join(carried, err)
				}
			}
			reason := "a flaky test the batch relied on is no longer allowed (" + carried.Error() + "); nothing was published; " +
				diagnosticFailure(DiagnosticResult{AttemptID: record.Proof.AttemptID}, record.Proof.RedGroups)
			return errors.Join(&FlakeAllowanceRefusal{Code: codeFlakeAllowanceRefused, Reason: reason}, returnEveryMember(store, record, actor, at, reason))
		}
		return nil
	}
	recoverNow := func(recoveryOrigin, candidateTip string, pushErr error) (bool, error) {
		if err := recheckFlakes(); err != nil {
			return false, err
		}
		if seams.RecoverPush == nil {
			return false, fmt.Errorf("%s: origin %s refused the complete series: %w", codeLandPushRefused, recoveryOrigin, pushErr)
		}
		recovery, recoveryErr := seams.RecoverPush(recoveryOrigin, record.BaseTree, candidateTip, recheckFlakes)
		if recoveryErr != nil {
			var fenced *PrefixFencedRefusal
			var revision *PrefixRevisionRefusal
			var budget *PrefixBudgetRefusal
			var allowance *FlakeAllowanceRefusal
			if errors.As(recoveryErr, &fenced) || errors.As(recoveryErr, &revision) || errors.As(recoveryErr, &budget) || errors.As(recoveryErr, &allowance) {
				// The recovery owner has already requested return and
				// reassembled survivors. Its old landing progress must not
				// overwrite the new open or dissolved batch.
				return false, recoveryErr
			}
		}
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
			return false, fmt.Errorf("%s: origin %s held after remote rejection: %w", codeLandPushRefused, recovery.Origin, recoveryErr)
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
			return false, fmt.Errorf("%s: origin %s recovery failed: %w", codeLandPushRefused, recoveryOrigin, errors.Join(pushErr, recoveryErr))
		}
		if recovery.Reopen {
			if landed, checkErr := markAlreadyLanded(recovery.Origin, candidateTip); checkErr != nil || landed {
				return landed, checkErr
			}
			if seams.SeriesOnOrigin == nil {
				return false, fmt.Errorf("%s: already-landed helper is absent", codeLandUnwired)
			}
			if seams.Abandon == nil {
				return false, fmt.Errorf("%s: abandon helper is absent", codeLandUnwired)
			}
			if abandonErr := seams.Abandon(candidateTip, recovery.Origin); abandonErr != nil {
				return false, abandonErr
			}
			return false, ReopenMovedBase(store, id, recovery.BaseTree, recovery.LandedBy, actor, at)
		}
		if !recovery.Pushed || recovery.Tip == "" {
			return false, fmt.Errorf("%s: origin %s is unchanged after the refused push: %w", codeLandPushRefused, recoveryOrigin, pushErr)
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
		if err := recheckFlakes(); err != nil {
			return false, err
		}
		if seams.VerifySeries != nil {
			if err := seams.VerifySeries(units, cloneStrings(progress.Commits)); err != nil {
				return false, fmt.Errorf("%s: final series: %w", codePrefixProofRefused, err)
			}
		}
		if seams.Push == nil {
			return false, fmt.Errorf("%s: push helper is absent", codeLandUnwired)
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
			return false, fmt.Errorf("%s: origin %s held after remote rejection: %w", codeLandPushRefused, origin, pushErr)
		}
		if !IsStaleEndpointLease(pushErr) {
			return false, fmt.Errorf("%s: endpoint push failed without a stale lease: %w", codeLandPushRefused, pushErr)
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
			return false, fmt.Errorf("%s: origin %s is unchanged after the refused push: %w", codeLandPushRefused, origin, pushErr)
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
			return fmt.Errorf("%s: held landing has no candidate tip", codeLandPushRefused)
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
			return fmt.Errorf("%s: refused landing has no candidate tip", codeLandPushRefused)
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
		if unit.IsChange() {
			if seams.ReplayChange == nil {
				return fmt.Errorf("%s: change replay helper is absent", codeLandUnwired)
			}
			commit, replayErr := seams.ReplayChange(unit)
			if replayErr != nil {
				return ejectRefusedMember(store, id, actor, at, record.BaseTree, unit, replayErr, seams.Reset)
			}
			if commit == "" {
				return fmt.Errorf("%s: change %s returned no commit", codeLandCommitRefused, unit.GoalID)
			}
			progress.Commits[unit.GoalID] = commit
			if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
				return err
			}
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
			return fmt.Errorf("%s: unit %s has no exact prefix receipt", codeLandReceiptRefused, unit.GoalID)
		}
		if len(unit.Builds) != 0 {
			if seams.ApplyBuild == nil || seams.AppendBuildReceipt == nil || seams.CommitBuild == nil {
				return fmt.Errorf("%s: branch member helpers are incomplete", codeLandUnwired)
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
					return fmt.Errorf("%s: build %s returned no commit", codeLandCommitRefused, build.Commit)
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
			return fmt.Errorf("%s: local series helpers are incomplete", codeLandUnwired)
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
			return fmt.Errorf("%s: unit %s returned no commit", codeLandCommitRefused, unit.GoalID)
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
			return fmt.Errorf("%s: held helper is absent", codeLandHeldRefused)
		}
		if heldErr := seams.Held(record.BaseTree, tip); heldErr != nil {
			// A refusal naming one member's commit ejects that member (a
			// change or a goal) and reopens the rest; the refused step is
			// never tried again as it was (U11b).
			var series *HeldSeriesRefusal
			if errors.As(heldErr, &series) {
				// About the series or the lane's configuration, not one
				// member: the batch holds with the refusal as its reason.
				return RecordHold(store, id, actor, "held refused the series: "+heldErr.Error(), at)
			}
			var refused *HeldCommitRefusal
			if errors.As(heldErr, &refused) && refused.Commit != "" {
				for _, unit := range units {
					if landedCommitOf(progress, unit, refused.Commit) {
						return ejectHeldMember(store, id, actor, at, record.BaseTree, unit, heldErr, seams.Reset)
					}
				}
			}
			return fmt.Errorf("%s: complete series did not pass held: %w", codeLandHeldRefused, heldErr)
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
	return ReopenMovedBase(store, id, newBaseTree, "", actor, at)
}

// ReopenMovedBase reopens an open, sealed, proving or landing batch on the
// base another landing moved main to (D2 (b), (c)): the survivors reassemble
// there, a member whose changes conflict with what batch landedBy landed is
// ejected with the files named, and a proof in flight is discarded by its
// token, because the plan it ran is cleared.
func ReopenMovedBase(store Store, id, newBaseTree, landedBy, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, landedBy, actor, at, false, "selected proof input moved")
}

// ReopenRefusedPush gives up a bounded endpoint transaction. Unlike a moved-
// input reopen, the endpoint may still be the batch's recorded base.
func ReopenRefusedPush(store Store, id, newBaseTree, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, "", actor, at, true, "endpoint push refused")
}

func reopenLandingCandidate(store Store, id, newBaseTree, landedBy, actor string, at time.Time, allowSameBase bool, detail string) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	reopenable := record.State == StateLanding || !allowSameBase && slices.Contains([]string{StateOpen, StateSealed, StateProving}, record.State)
	if !reopenable || newBaseTree == "" || (!allowSameBase && newBaseTree == record.BaseTree) {
		return fmt.Errorf("%s: moved trunk did not supply a new landing base", codeLandPushRefused)
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("%s: batch %s has no joined units", codeLandStateRefused, id)
	}
	return reassembleSurvivorsOnBase(store, id, actor, at, nil, newBaseTree, detail, landedBy)
}

// BaseMove is what moved main under a batch: the changed paths, the
// installation prefix they are relative to, and the batch whose landing
// moved it (empty when no batch of this store did).
type BaseMove struct {
	Changed          []string
	Prefix, LandedBy string
}

// MovedBase is the moved-base decision: reopen, or rebase and keep the proof.
type MovedBase struct {
	Reopen bool
}

// DecideMovedBase is the one moved-base decision (D2, R6), made at the
// owner's tick and at the push: the batch reopens on the new base when a
// changed path is an engine path of this installation or lies in a selected
// group's recorded input manifest (a proof without one reopens); otherwise
// the series rebases and keeps its proof. An open or sealed batch and a
// planned proof learn their manifests with the proof's result, so only an
// engine move reopens them at the tick; their landing decides again.
func DecideMovedBase(record Record, changed []string, installationPrefix string) MovedBase {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return MovedBase{Reopen: true}
	}
	installationPrefix = strings.Trim(filepath.ToSlash(installationPrefix), "/")
	for _, changedPath := range changed {
		policyPath := filepath.ToSlash(changedPath)
		if installationPrefix != "" && policyPath != installationPrefix && !strings.HasPrefix(policyPath, installationPrefix+"/") {
			continue
		}
		// ENGINE changes invalidate proof inputs only when the changed path is
		// part of this installation; sibling repositories have separate engines.
		if included, err := policy.Includes(behaviorsurface.Engine, policyPath, installationPrefix); err != nil || included {
			return MovedBase{Reopen: true}
		}
	}
	if record.State == StateOpen || record.State == StateSealed || record.State == StateProving && record.Proof != nil && record.Proof.Status == "planned" {
		return MovedBase{}
	}
	if record.Proof == nil {
		return MovedBase{Reopen: true}
	}
	for _, groupID := range record.Proof.SelectedGroups {
		manifest := record.Proof.InputManifests[groupID]
		if len(manifest) == 0 {
			return MovedBase{Reopen: true}
		}
		for _, changedPath := range changed {
			if slices.ContainsFunc(manifest, func(declaration string) bool {
				matched, err := pathpattern.MatchManifestEntry(declaration, changedPath)
				return err != nil || matched
			}) {
				return MovedBase{Reopen: true}
			}
		}
	}
	return MovedBase{}
}

// LandedBatch names the batch of this store whose pushed tip is one of the
// commits main moved by; empty when none is.
func LandedBatch(store Store, commits []string) string {
	paths, _ := filepath.Glob(filepath.Join(store.root, "artifacts", "agents", "landing-batches", "*.json"))
	for _, path := range paths {
		record, err := store.Load(strings.TrimSuffix(filepath.Base(path), ".json"))
		if err == nil && record.Landing != nil && record.Landing.PushedTip != "" && slices.Contains(commits, record.Landing.PushedTip) {
			return record.BatchID
		}
	}
	return ""
}

// movedBaseConflictLine is the return of a member whose changes do not apply
// on the moved base: BATCH_JOIN_CONFLICT's files and the next command.
func movedBaseConflictLine(conflict *assemblyConflict, landedBy string) string {
	files := strings.Join(conflict.Paths, ", ")
	if files == "" {
		files = conflict.Error()
	}
	with := "what landed on main"
	if landedBy != "" {
		with = "what landed in batch " + landedBy
	}
	return fmt.Sprintf("CONFLICT with %s (files %s). Rebase goal/%s on main, then metasystem work land %s.", with, files, conflict.GoalID, conflict.GoalID)
}

func ejectRefusedMember(store Store, id, actor string, at time.Time, base string, unit Unit, err error, reset func(string) error) error {
	if !isBoundaryRefusal(err) {
		return err
	}
	refusal := fmt.Errorf("commit %s refused: %w", unit.GoalID, err)
	if reset != nil {
		if resetErr := reset(base); resetErr != nil {
			return errors.Join(refusal, fmt.Errorf("reset: %w", resetErr))
		}
	}
	if reassembleErr := ReassembleSurvivorsWithReturns(store, id, actor, at,
		[]ReturnDecision{{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: refusal.Error()}}); reassembleErr != nil {
		return errors.Join(refusal, reassembleErr)
	}
	return fmt.Errorf("%w; goal %s was ejected; fix and rejoin it, then re-prove before the next landing", refusal, unit.GoalID)
}

// landedCommitOf reports whether commit is one the landing made for unit.
func landedCommitOf(progress LandingProgress, unit Unit, commit string) bool {
	if progress.Commits[unit.GoalID] == commit {
		return true
	}
	for _, build := range unit.Builds {
		if progress.BuildCommits[build.Commit] == commit {
			return true
		}
	}
	return false
}

// ejectHeldMember returns a member held refused at the push, with the
// refusal as its reason, and reassembles the survivors on the same base.
func ejectHeldMember(store Store, id, actor string, at time.Time, base string, unit Unit, cause error, reset func(string) error) error {
	if reset != nil {
		if err := reset(base); err != nil {
			return fmt.Errorf("%s: change %s: reset: %w", codeLandHeldRefused, unit.GoalID, err)
		}
	}
	kind := "goal"
	if unit.IsChange() {
		kind = "change"
	}
	reason := "EJECTED from landing batch " + id + ": held refused " + kind + " " + unit.GoalID + " at the push: " + cause.Error()
	return ReassembleSurvivorsWithReturns(store, id, actor, at, []ReturnDecision{{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: reason}})
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
