package branch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	RebaseConflictCode  = "GOAL_REBASE_CONFLICT"
	RebaseJudgementCode = "GOAL_REBASE_JUDGEMENT"
)

type RebaseRequest struct {
	SubjectCheck                      func(string, AttestationSubject) (GateObservation, error)
	Repo, Remote, EndpointTip, GoalID string
	CheckClaim                        func() error
	Gate                              func(string) (string, error)
	Transport                         PushTransport
	Resolve                           func(RebaseResolution) (string, error)
	Answer                            func(string) (string, error)
	RecordDrop                        func(goal.UnitDrop, goal.UnitDrop) error
}

// RebaseResolution asks the unit runner to correct one stopped replay.
type RebaseResolution struct {
	Unit, Commit, Worktree, Base, Conflicts, MainTip string
	Paths                                            []string
}

type RebaseResult struct {
	ReviewReasons map[string]string `json:"reviewReasons,omitempty"`

	State       string   `json:"state"`
	Behind      int      `json:"behind"`
	OldTip      string   `json:"oldTip"`
	NewTip      string   `json:"newTip"`
	MainTip     string   `json:"mainTip"`
	Carried     []string `json:"carried"`
	NeedsReview []string `json:"needsReview"`
	Unknown     []string `json:"unknown,omitempty"`
	Regenerated []string `json:"regenerated,omitempty"`
	Resolved    []string `json:"resolved,omitempty"`
}

type rebaseDependencies struct {
	repository commitRepository
	git        func(string, ...string) ([]byte, error)
	keptTips   func(repo, goal string) ([]string, error)
	change     func(repo, commit string) (string, error)
	tests      func(repo, goal, commit string) ([]TestChange, error)
	gate       func(ReadGateRequest) (GateObservation, error)
	commitRead func(CommitReadRequest) (string, Attestation, error)
	push       func(PushRequest) (PushResult, error)
	newID      func(string) (string, error)
	run        func([]string, string, *os.File, func(int64) error) error
}

func gitRebaseDependencies() rebaseDependencies {
	return rebaseDependencies{
		repository: gitCommitRepository(), git: gitOutput,
		keptTips: func(repo, goal string) ([]string, error) {
			out, err := gitOutput(repo, "for-each-ref", "--sort=-committerdate", "--format=%(objectname)", "refs/metasystem/goals/before/"+goal+"/")
			return strings.Fields(string(out)), err
		},
		change: func(repo, commit string) (string, error) {
			return changeDigestWithReads(gitAttestationReads{}, repo, commit)
		},
		tests: func(repo, goal, commit string) ([]TestChange, error) {
			data, err := attestationFileAt(gitAttestationReads{}, repo, "", attestationPath(goal, commit))
			if err != nil {
				return nil, err
			}
			var prior Attestation
			if err := json.Unmarshal(data, &prior); err != nil {
				return nil, err
			}
			return prior.TestsChanged, nil
		},
		run: conflict.RunRegenerationArgv, gate: ResolveReadGate, commitRead: CommitRead, push: Push, newID: branchReadID,
	}
}

func Rebase(req RebaseRequest) (RebaseResult, error) {
	return rebaseWith(req, gitRebaseDependencies())
}

func rebaseWith(req RebaseRequest, d rebaseDependencies) (RebaseResult, error) {
	result := RebaseResult{MainTip: req.EndpointTip, Carried: []string{}, NeedsReview: []string{}}
	if !validName(req.GoalID) || req.Remote == "" || !hex40(req.EndpointTip) {
		return result, fmt.Errorf("rebase needs a goal, remote and main's full commit name\nrun: metasystem work status")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return result, err
	}
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	r := d.repository
	local, present, err := r.facts.Tip(req.Repo, goalBranchRef(req.GoalID))
	if err != nil {
		return result, err
	}
	if !present {
		return result, operationRefusal(StaleCode, "goal %s has no branch here\nrun: metasystem work build %s", req.GoalID, req.GoalID)
	}
	result.OldTip, result.NewTip = local, local
	remote, remotePresent, err := req.Transport.RemoteTip(req.Repo, req.Remote, goalBranchRef(req.GoalID))
	if err != nil {
		return result, err
	}
	if !remotePresent {
		remote = ""
	}
	origin, originPresent, err := r.facts.Tip(req.Repo, originTipRef(req.GoalID))
	if err != nil {
		return result, err
	}
	if !originPresent {
		origin = ""
	}
	if !(remotePresent && remote == local || remotePresent && originPresent && origin == remote || !remotePresent && !originPresent) {
		return result, staleBranch(req.GoalID, req.Remote, local, remote, origin)
	}
	for _, paths := range []func(string) ([]string, error){r.facts.Staged, r.facts.Unstaged} {
		changed, err := paths(req.Repo)
		if err != nil {
			return result, err
		}
		if len(changed) != 0 {
			return result, operationRefusal(StaleCode, "goal %s's worktree has changes; commit or discard them first\nrun: metasystem work status %s", req.GoalID, req.GoalID)
		}
	}
	log, logErr := os.OpenFile(rebaseRegenerationLog(req), os.O_WRONLY|os.O_TRUNC, 0o600)
	if logErr == nil {
		if err := log.Close(); err != nil {
			return result, err
		}
	} else if !errors.Is(logErr, os.ErrNotExist) {
		return result, logErr
	}
	onMain, err := r.facts.Ancestor(req.Repo, req.EndpointTip, local)
	if err != nil {
		return result, err
	}
	var resolved []string
	if !onMain {
		resolve := req.Resolve
		if resolve != nil {
			req.Resolve = func(stop RebaseResolution) (string, error) {
				record, err := resolve(stop)
				if err == nil {
					resolved = append(resolved, stop.Paths...)
				}
				return record, err
			}
		}
		onMain, err = rebaseLedgerOnly(req, local, d.git)
		if err != nil {
			return result, err
		}
	}
	if !onMain {
		next, regenerated, err := replayRebase(req, local, d)
		if err != nil {
			return result, err
		}
		current, _, err := r.checkoutInstallPreflight(CommitRequest{Repo: req.Repo, GoalID: req.GoalID}, next)
		if err != nil {
			return result, err
		}
		if err := checkClaim(req.CheckClaim); err != nil {
			return result, err
		}
		// Keep the old branch before moving it so an interrupted move can still carry its reviews.
		if _, err := d.git(req.Repo, "update-ref", "refs/metasystem/goals/before/"+req.GoalID+"/"+local, local); err != nil {
			return result, err
		}
		checkedOut := current == goalBranchRef(req.GoalID)
		if checkedOut {
			if err := r.effects.Checkout(req.Repo, local, next); err != nil {
				return result, err
			}
		}
		moveErr := checkClaim(req.CheckClaim)
		if moveErr == nil {
			// Record the observed origin tip: the moved branch replaces this tip on its next push.
			moveErr = r.effects.Publish(req.Repo, req.GoalID, local, next, remote)
		}
		if moveErr != nil {
			if checkedOut {
				moveErr = errors.Join(moveErr, r.effects.Checkout(req.Repo, next, local))
			}
			return result, moveErr
		}
		result.NewTip, result.State, result.Regenerated = next, "rebased", regenerated
		result.Resolved = resolved
	}
	drops, covered, err := rebaseDrops(req, result.NewTip, d)
	if err != nil {
		return result, err
	}
	carried, err := carryReviewsWith(CarryRequest(req), d, covered)
	if carried.NewTip != "" {
		result.NewTip = carried.NewTip
	}
	result.Carried, result.NeedsReview, result.Unknown = carried.Carried, carried.NeedsReview, carried.Unknown
	result.ReviewReasons = carried.ReviewReasons
	if err != nil {
		return result, err
	}
	if result.NewTip != remote {
		if err := checkClaim(req.CheckClaim); err != nil {
			return result, err
		}
		op, err := d.newID("goal-rebase-push")
		if err != nil {
			return result, err
		}
		pushed, err := d.push(PushRequest{Repo: req.Repo, Remote: req.Remote, EndpointTip: req.EndpointTip, GoalID: req.GoalID, OpID: op, CheckClaim: req.CheckClaim, Transport: req.Transport})
		if err != nil {
			return result, err
		}
		result.NewTip = pushed.Tip
		if result.State == "" {
			result.State = "pushed"
		}
	}
	for _, drop := range drops {
		if err := checkClaim(req.CheckClaim); err != nil {
			return result, err
		}
		if err := req.RecordDrop(drop.before, drop.after); err != nil {
			return result, fmt.Errorf("the rebased drop was not recorded: %w", err)
		}
	}
	if result.State == "" {
		result.State = "held"
		if len(result.Carried) != 0 {
			result.State = "carried"
		}
	}
	count, err := d.git(req.Repo, "rev-list", "--count", result.NewTip+".."+req.EndpointTip)
	if err != nil {
		return result, err
	}
	result.Behind, err = strconv.Atoi(strings.TrimSpace(string(count)))
	if err != nil {
		return result, err
	}
	if len(result.Resolved) > 0 {
		if err := os.Remove(filepath.Join(req.Repo, "artifacts", "agents", "goals", req.GoalID, "conflict.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	return result, nil
}

type CarryRequest RebaseRequest

type CarryResult struct {
	ReviewReasons map[string]string `json:"reviewReasons,omitempty"`

	NewTip      string   `json:"newTip"`
	Carried     []string `json:"carried"`
	NeedsReview []string `json:"needsReview"`
	Unknown     []string `json:"unknown,omitempty"`
}

// CarryReviews records reviews for unchanged units from a kept branch tip without publishing them.
func CarryReviews(req CarryRequest) (CarryResult, error) {
	result := CarryResult{Carried: []string{}, NeedsReview: []string{}}
	if !validName(req.GoalID) || req.Remote == "" || !hex40(req.EndpointTip) {
		return result, fmt.Errorf("carrying reviews needs a goal, remote and main's full commit name\nrun: metasystem work status")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return result, err
	}
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	return carryReviewsWith(req, gitRebaseDependencies())
}

func carryReviewsWith(req CarryRequest, d rebaseDependencies, dropped ...map[string]bool) (CarryResult, error) {
	result := CarryResult{Carried: []string{}, NeedsReview: []string{}}
	r := d.repository
	tip, present, err := r.facts.Tip(req.Repo, goalBranchRef(req.GoalID))
	if err != nil {
		return result, err
	}
	if !present {
		return result, operationRefusal(StaleCode, "goal %s has no branch here\nrun: metasystem work build %s", req.GoalID, req.GoalID)
	}
	result.NewTip = tip
	commits, err := r.facts.Range(req.Repo, req.EndpointTip, result.NewTip, req.GoalID)
	if err != nil {
		return result, err
	}
	reviewed, err := rebaseReviewed(req.Repo, req.GoalID, commits, r)
	if err != nil {
		return result, err
	}
	var kept []string
	loaded := false
	unknown := func(unit Commit) {
		result.NeedsReview = append(result.NeedsReview, unit.Units...)
		result.Unknown = append(result.Unknown, unit.Units...)
	}
	for _, unit := range commits {
		if unit.Kind != Unit || reviewed[unit.ID] || len(dropped) > 0 && dropped[0][unit.ID] {
			continue
		}
		if !loaded {
			kept, err = d.keptTips(req.Repo, req.GoalID)
			if err != nil {
				return result, err
			}
			loaded = true
		}
		prior, err := rebasePredecessor(req, unit, kept, r)
		if err != nil {
			unknown(unit)
			continue
		}
		equal := false
		if prior != "" {
			before, err := d.change(req.Repo, prior)
			if err != nil {
				unknown(unit)
				continue
			}
			after, err := d.change(req.Repo, unit.ID)
			if err != nil {
				unknown(unit)
				continue
			}
			equal = before == after
		}
		if !equal {
			result.NeedsReview = append(result.NeedsReview, unit.Units...)
			continue
		}
		tests, err := d.tests(req.Repo, req.GoalID, prior)
		if err != nil {
			unknown(unit)
			continue
		}
		gate, err := d.gate(ReadGateRequest{Repo: req.Repo, GoalID: req.GoalID, UnitCommit: unit.ID, Gate: req.Gate, SubjectCheck: req.SubjectCheck})
		if err != nil {
			var missing *DeclarationUnavailableError
			if errors.As(err, &missing) {
				result.NeedsReview = append(result.NeedsReview, unit.Units...)
				if result.ReviewReasons == nil {
					result.ReviewReasons = map[string]string{}
				}
				for _, name := range unit.Units {
					result.ReviewReasons[name] = missing.Error()
				}
				continue
			}
			return result, err
		}
		op, err := d.newID("goal-rebase-read")
		if err != nil {
			return result, err
		}
		next, _, err := d.commitRead(CommitReadRequest{Repo: req.Repo, Remote: req.Remote, EndpointTip: req.EndpointTip,
			GoalID: req.GoalID, Units: unit.Units, OpID: op, CheckClaim: req.CheckClaim, Transport: req.Transport,
			Carry: prior, TestsChanged: tests, GateRunID: gate.RunID, GateTree: gate.Tree, GateObservation: &gate})
		var refusal *OpError
		if errors.As(err, &refusal) && refusal.Code == ReadStaleCode {
			result.NeedsReview = append(result.NeedsReview, unit.Units...)
			continue
		}
		if errors.As(err, &refusal) && refusal.Code == ReadInvalidCode {
			unknown(unit)
			continue
		}
		if err != nil {
			return result, err
		}
		result.NewTip = next
		result.Carried = append(result.Carried, unit.Units...)
	}
	return result, nil
}

// Goal history lines advance main without changing a unit's code. The lane
// merges these files without conflict, so only ledger changes need no replay.
// This also lets the rebase's own history line leave a repeat with nothing to do.
func rebaseLedgerOnly(req RebaseRequest, local string, git func(string, ...string) ([]byte, error)) (bool, error) {
	base, err := git(req.Repo, "merge-base", req.EndpointTip, local)
	if err != nil {
		return false, err
	}
	// Compare trees so changes brought onto main by a merge are included.
	changed, err := git(req.Repo, "diff", "--name-only", "-z", strings.TrimSpace(string(base)), req.EndpointTip)
	if err != nil {
		return false, err
	}
	prefix, err := git(req.Repo, "rev-parse", "--show-prefix")
	if err != nil {
		return false, err
	}
	installation := strings.TrimSpace(string(prefix))
	for _, name := range strings.Split(string(changed), "\x00") {
		if name != "" && (!strings.HasPrefix(name, installation) || !goal.IsLedgerFile(strings.TrimPrefix(name, installation))) {
			return false, nil
		}
	}
	return true, nil
}

func replayRebase(req RebaseRequest, local string, d rebaseDependencies) (string, []string, error) {
	r := d.repository
	dir, close, err := r.effects.Open(req.Repo, local, false)
	if err != nil {
		return "", nil, err
	}
	defer close()
	var log *os.File
	defer func() {
		if log != nil {
			_ = log.Close()
		}
	}()
	var regenerated []string
	// Read commits retain their files so unchanged builds can carry their reviews.
	_, stop := d.git(dir, "rebase", "--reapply-cherry-picks", "--empty=keep", "--no-autosquash", req.EndpointTip)
	for stop != nil {
		paths, opened, err := resolveRebaseStop(req, local, dir, log, d, stop)
		log = opened
		if err != nil {
			return "", nil, err
		}
		for _, path := range paths {
			if !slices.Contains(regenerated, path) {
				regenerated = append(regenerated, path)
			}
		}
		_, stop = d.git(dir, "-c", "core.editor=true", "rebase", "--continue")
	}

	tip, err := r.facts.Head(dir)
	if err != nil {
		return "", nil, err
	}
	_, err = r.facts.Range(req.Repo, req.EndpointTip, tip, req.GoalID)
	return tip, regenerated, err
}

func rebaseReviewed(repo, goal string, commits []Commit, r commitRepository) (map[string]bool, error) {
	reviewed := map[string]bool{}
	for _, c := range commits {
		if c.Kind != Read {
			continue
		}
		kind, err := r.facts.Kind(repo, c.ID, goal)
		if err != nil {
			return nil, err
		}
		reviewed[kind.CommitID] = true
	}
	return reviewed, nil
}

func rebasePredecessor(req CarryRequest, unit Commit, kept []string, r commitRepository) (string, error) {
	for _, tip := range kept {
		commits, err := r.facts.Range(req.Repo, req.EndpointTip, tip, req.GoalID)
		if err != nil {
			return "", err
		}
		reviewed, err := rebaseReviewed(req.Repo, req.GoalID, commits, r)
		if err != nil {
			return "", err
		}
		for _, old := range commits {
			if old.Kind == Unit && sameUnits(old.Units, unit.Units) && reviewed[old.ID] {
				return old.ID, nil
			}
		}
	}
	return "", nil
}
