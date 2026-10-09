package branch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

const (
	UnavailableCode    = "GOAL_BRANCH_UNAVAILABLE"
	NotHolderCode      = "GOAL_BRANCH_NOT_HOLDER"
	ReplayConflictCode = "GOAL_BRANCH_REPLAY_CONFLICT"
)

type OpError struct {
	Code    string
	Message string
	// Cause is the error the refusal reports when it carries facts a caller
	// reads with errors.As; nil for a refusal of its own.
	Cause error
}

// Error is the refusal's words; its code is data ("Messages a Person
// Reads"): RefusalCode, and goal.RecordText for records.
func (e *OpError) Error() string { return e.Message }

// RefusalCode is the refusal's code.
func (e *OpError) RefusalCode() string { return e.Code }

// Unwrap is the reported cause, if any.
func (e *OpError) Unwrap() error { return e.Cause }

func operationRefusal(code, format string, args ...any) error {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// firstLine is an error's line 1: a refusal wrapped into another keeps its
// words and gives way to the wrapper's line 2 ("Messages a Person Reads").
func firstLine(err error) string {
	if err == nil {
		return ""
	}
	line, _, _ := strings.Cut(err.Error(), "\n")
	return line
}

// pushProtocolAvailable is set by the push owner when the binary can publish
// every commit it creates. A binary assembled without that owner fails closed.
var pushProtocolAvailable bool

type CommitRequest struct {
	BeforeCommit                                  func(dir, parent, tree string) error
	BeforeInstall                                 func() (func() error, error)
	PrepareOnly                                   bool
	ResumeWorktree                                string
	KeepWorktree                                  func() bool
	FrozenPatch                                   []byte
	Repo, Remote, EndpointTip, GoalID, Unit, OpID string
	Units                                         []string
	Kind                                          Kind
	Amend                                         bool
	Whole                                         bool
	CheckClaim                                    func() error
	Transport                                     PushTransport
}

type commitBranchState struct {
	localTip, baseTip, originTip string
	localPresent, remotePresent  bool
	adopt                        bool
}

func goalBranchRef(goalID string) string { return "refs/heads/goal/" + goalID }

func validName(value string) bool {
	return value != "" && !strings.Contains(value, "/") && strings.IndexFunc(value, unicode.IsSpace) < 0
}

func checkClaim(check func() error) error {
	if check == nil {
		return operationRefusal(NotHolderCode, "this session's claim on the goal can't be checked\nrun: metasystem goal list")
	}
	if err := check(); err != nil {
		return &OpError{Code: NotHolderCode, Message: err.Error(), Cause: err}
	}
	return nil
}

func CheckHolder(check func() error) error { return checkClaim(check) }

func CheckCommitAccess(goalID string, check func() error) error {
	if !pushProtocolAvailable {
		return operationRefusal(UnavailableCode, "this engine was built without the goal-branch publisher, so it can't publish work\nrun: metasystem system check")
	}
	if !validName(goalID) {
		return fmt.Errorf("goal id must be one nonempty path-free word")
	}
	return checkClaim(check)
}

func localBranchTip(repo, ref string) (string, bool, error) {
	out, err := gitOutput(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", false, nil
	}
	return strings.TrimSpace(string(out)), true, nil
}

func stagedPaths(repo string) ([]string, error) {
	out, err := gitOutput(repo, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range bytes.Split(out, []byte{0}) {
		if len(item) != 0 {
			paths = append(paths, string(item))
		}
	}
	return paths, nil
}

func gitInput(repo string, input []byte, args ...string) ([]byte, error) {
	return gitInputEnv(repo, nil, input, args...)
}

func gitInputEnv(repo string, env []string, input []byte, args ...string) ([]byte, error) {
	full, err := branchGitCommand(repo, args...)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", full...)
	cmd.Env = gittree.ScrubbedEnviron(append([]string{"LC_ALL=C"}, env...)...)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

func inspectCommitBranch(req CommitRequest) (commitBranchState, error) {
	return gitCommitRepository().inspectCommitBranch(req)
}

func (r commitRepository) inspectCommitBranch(req CommitRequest) (commitBranchState, error) {
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	if req.Remote == "" || !validName(req.OpID) {
		return commitBranchState{}, fmt.Errorf("commit needs a remote and operation id")
	}
	state := commitBranchState{}
	var err error
	state.localTip, state.localPresent, err = r.facts.Tip(req.Repo, goalBranchRef(req.GoalID))
	if err != nil {
		return state, err
	}
	if state.localPresent {
		if _, err := r.facts.Range(req.Repo, req.EndpointTip, state.localTip, req.GoalID); err != nil {
			return state, err
		}
	}
	var remoteTip string
	remoteTip, state.remotePresent, err = req.Transport.RemoteTip(req.Repo, req.Remote, goalBranchRef(req.GoalID))
	if err != nil {
		return state, err
	}
	if state.remotePresent {
		if err := fetchAndValidateWith(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, remoteTip, req.Transport,
			fetchValidationDependencies{validateRange: r.facts.Range, clearRef: r.effects.ClearFetch}); err != nil {
			return state, err
		}
	}
	if !state.localPresent && !state.remotePresent {
		head, err := r.facts.Head(req.Repo)
		if err != nil {
			return state, err
		}
		if head != req.EndpointTip {
			return state, operationRefusal(RangeCode, "goal %s's first commit must start at main's tip %s, and this checkout is elsewhere\nrun: metasystem work status %s", req.GoalID, req.EndpointTip, req.GoalID)
		}
	}
	state.originTip, _, err = r.facts.Tip(req.Repo, originTipRef(req.GoalID))
	if err != nil {
		return state, err
	}
	switch {
	case !state.localPresent && state.remotePresent:
		state.baseTip, state.originTip, state.adopt = remoteTip, remoteTip, true
		return state, nil
	case !state.localPresent:
		state.baseTip = req.EndpointTip
		return state, nil
	case state.remotePresent && remoteTip == state.localTip:
		state.baseTip, state.originTip = state.localTip, remoteTip
		return state, nil
	case state.remotePresent:
		remoteBuiltOnLocal, err := r.facts.Ancestor(req.Repo, state.localTip, remoteTip)
		if err != nil {
			return state, err
		}
		if remoteBuiltOnLocal {
			state.baseTip, state.originTip, state.adopt = remoteTip, remoteTip, true
			return state, nil
		}
		builtOnRemote, err := r.facts.Ancestor(req.Repo, remoteTip, state.localTip)
		if err != nil {
			return state, err
		}
		if builtOnRemote || state.originTip == remoteTip {
			state.baseTip, state.originTip = state.localTip, remoteTip
			return state, nil
		}
		if state.originTip != "" && state.localTip == state.originTip {
			state.baseTip, state.originTip, state.adopt = remoteTip, remoteTip, true
			return state, nil
		}
		return state, staleBranch(req.GoalID, req.Remote, state.localTip, remoteTip, state.originTip)
	case state.originTip != "":
		return state, staleBranch(req.GoalID, req.Remote, state.localTip, "", state.originTip)
	default:
		state.baseTip = state.localTip
		return state, nil
	}
}

func commitMessage(req CommitRequest, subjectCommit string) (string, string, error) {
	units, err := requestUnits(req)
	if err != nil {
		return "", "", err
	}
	list := unitList(units)
	switch req.Kind {
	case Unit:
		if len(units) == 0 {
			return "", "", fmt.Errorf("unit commits need one or more distinct unit names")
		}
		trailers := "Goal-Unit: " + req.GoalID + "/" + list
		if req.Whole {
			trailers += "\nGoal-Whole: " + req.GoalID
		}
		return "goal " + req.GoalID + " units " + list, trailers, nil
	case Drop:
		if len(units) != 1 || req.Amend || req.Whole || !validName(req.OpID) || req.BeforeCommit == nil {
			return "", "", fmt.Errorf("a drop needs one unit, its saved operation and checks, without amend or whole")
		}
		return "goal " + req.GoalID + " drop " + list, "Goal-Drop: " + req.GoalID + "/" + list + " " + req.OpID, nil
	case Plan:
		if len(units) != 0 || req.Amend {
			return "", "", fmt.Errorf("plan commits take neither --unit nor --amend")
		}
		return "goal " + req.GoalID + " plan", "Goal-Plan: " + req.GoalID, nil
	case Read:
		if len(units) == 0 || !hex40(subjectCommit) || req.Amend {
			return "", "", fmt.Errorf("read commits need one or more units and one full subject commit")
		}
		return "goal " + req.GoalID + " read " + list, "Goal-Read: " + req.GoalID + "/" + list + " " + subjectCommit, nil
	default:
		return "", "", fmt.Errorf("kind must be unit, plan, or read")
	}
}

func requestUnits(req CommitRequest) ([]string, error) {
	if req.Unit != "" && len(req.Units) != 0 {
		return nil, fmt.Errorf("unit names must use either Unit or Units, not both")
	}
	units := append([]string(nil), req.Units...)
	if req.Unit != "" {
		units = []string{req.Unit}
	}
	if len(units) == 0 {
		return nil, nil
	}
	parsed, ok := parseUnits(strings.Join(units, "+"))
	if !ok || len(parsed) != len(units) {
		return nil, fmt.Errorf("unit names must be distinct nonempty path-free words")
	}
	return parsed, nil
}

func validateCommitPaths(kind Kind, paths []string, goalID string) error {
	reads, closures, prose := 0, 0, 0
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		class := PathClass(path)
		allowed := (kind == Unit || kind == Drop) && class == ClassUnit ||
			kind == Plan && (class == ClassPlan || path == landingRecordPath(goalID))
		if kind == Read {
			allowed = class == ClassRead || class == ClassReadClosure || class == ClassReadProse
		}
		if !allowed {
			return operationRefusal(RangeCode, "a %s may not change %s (a %s file)\nrun: metasystem work status %s", commitWord(kind), path, class, goalID)
		}
		if class == ClassRead {
			reads++
		}
		if class == ClassReadProse {
			prose++
		}
		if class == ClassReadClosure {
			closures++
		}
	}
	if kind == Read && (reads != 1 || closures > 1 || prose > 1) {
		return operationRefusal(RangeCode, "a review commit holds one review record and at most one of each extra; found %d, %d and %d\nrun: metasystem work review %s", reads, closures, prose, goalID)
	}
	return nil
}

func commitStaged(req CommitRequest, r commitRepository) (string, error) {
	return commitStagedSubject(req, "", r)
}

func commitStagedSubject(req CommitRequest, subjectCommit string, r commitRepository) (string, error) {
	if err := CheckCommitAccess(req.GoalID, req.CheckClaim); err != nil {
		return "", err
	}
	state, err := r.inspectCommitBranch(req)
	if err != nil {
		return "", err
	}
	return r.commitPreparedState(req, subjectCommit, state)
}

func (r commitRepository) commitPreparedState(req CommitRequest, subjectCommit string, state commitBranchState) (string, error) {
	if req.Amend {
		return r.amendUnit(req, state)
	}
	paths, err := r.facts.Staged(req.Repo)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 && req.Kind != Drop {
		return "", fmt.Errorf("the staged tree has no change to commit")
	}
	if err := validateCommitPaths(req.Kind, paths, req.GoalID); err != nil {
		return "", err
	}
	subject, trailer, err := commitMessage(req, subjectCommit)
	if err != nil {
		return "", err
	}
	return r.commitStagedOnto(req, state, subject, trailer)
}

func CommitStaged(req CommitRequest) (string, error) { return commitStaged(req, gitCommitRepository()) }

func CommitStagedWithInputs(req CommitRequest, facts CommitFacts, effects CommitEffects) (string, error) {
	if !facts.complete() || !effects.complete() {
		return "", fmt.Errorf("commit facts and effects must be complete")
	}
	return commitStaged(req, commitRepository{facts: facts, effects: effects})
}

func adoptionCheckoutClean(req CommitRequest, state commitBranchState, allowed []string) error {
	return gitCommitRepository().adoptionCheckoutClean(req, state, allowed)
}

func (r commitRepository) adoptionCheckoutClean(req CommitRequest, state commitBranchState, allowed []string) error {
	unstaged, err := r.facts.Unstaged(req.Repo)
	if err != nil {
		return err
	}
	allowedSet := map[string]bool{}
	for _, path := range allowed {
		allowedSet[path] = true
	}
	for _, item := range unstaged {
		if !allowedSet[item] {
			return operationRefusal(StaleCode, "goal %s's branch moved on origin to %s, and this checkout has unstaged changes in its way\nrun: metasystem work status %s", req.GoalID, state.baseTip, req.GoalID)
		}
	}
	return nil
}

func buildCommitOnto(req CommitRequest, state commitBranchState, subject, trailer string, patch []byte) (string, error) {
	return gitCommitRepository().buildCommitOnto(req, state, subject, trailer, patch)
}

func (r commitRepository) buildCommitOnto(req CommitRequest, state commitBranchState, subject, trailer string, patch []byte) (string, error) {
	worktree, close, err := r.openCommitWorktree(req, state.baseTip, false)
	if err != nil {
		return "", err
	}
	defer req.closeWorktree(close)
	if req.ResumeWorktree == "" && req.Kind != Drop {
		if err := r.effects.Apply(worktree, patch); err != nil {
			return "", operationRefusal(ReplayConflictCode, "the staged change doesn't apply to goal %s's branch as origin holds it (%s): %v\nrun: metasystem work status %s", req.GoalID, state.baseTip, err, req.GoalID)
		}
	}

	parent, err := r.facts.Head(worktree)
	if err != nil {
		return "", err
	}
	resumedCommit := req.ResumeWorktree != "" && req.Kind == Drop && !req.PrepareOnly && parent != state.baseTip
	if resumedCommit {
		// An interrupted install can leave the inverse committed only in scratch.
		suffix, err := r.facts.Suffix(worktree, state.baseTip, parent)
		if err != nil {
			return "", err
		}
		ancestor, err := r.facts.Ancestor(worktree, state.baseTip, parent)
		if err != nil {
			return "", err
		}
		kind, err := r.facts.Kind(worktree, parent, req.GoalID)
		if err != nil || !ancestor || len(suffix) != 1 || suffix[0] != parent || kind.Kind != Drop || kind.Operation != req.OpID || kind.Unit != req.Unit {
			return "", operationRefusal(StaleCode, "the branch moved while its publication check ran\nrun: metasystem work review %s --work %s", req.GoalID, unitList(req.Units))
		}
	}
	if req.BeforeCommit != nil {
		tree, err := r.facts.Index(worktree)
		if err != nil {
			return "", err
		}
		expectedParent := parent
		if resumedCommit {
			expectedParent = state.baseTip
		}
		if err := req.BeforeCommit(worktree, expectedParent, tree); err != nil {
			return "", err
		}
	}
	current, err := r.facts.Head(worktree)
	if err != nil {
		return "", err
	}
	if req.ResumeWorktree != "" && (current != parent || !resumedCommit && current != state.baseTip) {
		return "", operationRefusal(StaleCode, "the branch moved while its publication check ran\nrun: metasystem work review %s --work %s", req.GoalID, unitList(req.Units))
	}
	if resumedCommit {
		index, err := r.facts.Index(worktree)
		if err != nil {
			return "", err
		}
		tree, err := r.facts.Tree(worktree)
		if err != nil || index != tree {
			return "", fmt.Errorf("the retained drop commit changed: %v", err)
		}
		if _, err := r.facts.Range(worktree, req.EndpointTip, parent, req.GoalID); err != nil {
			return "", err
		}
		if err := checkClaim(req.CheckClaim); err != nil {
			return "", err
		}
		return parent, nil
	}
	if req.PrepareOnly {
		return r.facts.Index(worktree)
	}
	if err := r.effects.Commit(worktree, subject, trailer, false); err != nil {
		return "", err
	}
	newTip, err := r.facts.Head(worktree)
	if err != nil {
		return "", err
	}
	if _, err := r.facts.Range(worktree, req.EndpointTip, newTip, req.GoalID); err != nil {
		return "", err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return "", err
	}
	return newTip, nil
}

func (r commitRepository) checkoutInstallPreflight(req CommitRequest, newTip string) (string, string, error) {
	indexTree, err := r.facts.Index(req.Repo)
	if err != nil {
		return "", "", err
	}
	unstaged, err := r.facts.Unstaged(req.Repo)
	if err != nil {
		return "", "", err
	}
	tipChanges, err := r.facts.Changes(req.Repo, indexTree, newTip)
	if err != nil {
		return "", "", err
	}
	unstagedSet := make(map[string]bool, len(unstaged))
	for _, path := range unstaged {
		unstagedSet[path] = true
	}
	for _, path := range tipChanges {
		if unstagedSet[path] {
			return "", "", operationRefusal(StaleCode, "the new commit %s would overwrite your unstaged change to %s\nrun: metasystem work status %s", newTip, path, req.GoalID)
		}
	}
	current := r.facts.HeadRef(req.Repo)
	if current != goalBranchRef(req.GoalID) {
		worktrees, err := r.facts.Worktrees(req.Repo)
		if err != nil {
			return "", "", err
		}
		for _, worktree := range worktrees {
			if worktree.Branch == goalBranchRef(req.GoalID) {
				return "", "", operationRefusal(StaleCode, "goal %s's branch is checked out in another worktree, %s\nrun: metasystem work status %s", req.GoalID, worktree.Path, req.GoalID)
			}
		}
	}
	return current, indexTree, nil
}

func restoreHead(repo, ref, commit string) error {
	if ref != "" {
		_, err := gitOutput(repo, "symbolic-ref", "HEAD", ref)
		return err
	}
	_, err := gitOutput(repo, "update-ref", "--no-deref", "HEAD", commit)
	return err
}

func installCommitOnto(req CommitRequest, state commitBranchState, newTip string) error {
	return gitCommitRepository().installCommitOnto(req, state, newTip)
}

func (r commitRepository) installCommitOnto(req CommitRequest, state commitBranchState, newTip string) error {
	current, indexTree, err := r.checkoutInstallPreflight(req, newTip)
	if err != nil {
		return err
	}
	currentCommit, err := r.facts.Head(req.Repo)
	if err != nil {
		return err
	}
	if err := r.effects.Checkout(req.Repo, indexTree, newTip); err != nil {
		return operationRefusal(StaleCode, "the new commit %s couldn't be checked out here: %v\nrun: metasystem work status %s", newTip, err, req.GoalID)
	}
	rollbackCheckout := func(cause error) error {
		if rollbackErr := r.effects.Checkout(req.Repo, newTip, indexTree); rollbackErr != nil {
			return operationRefusal(StaleCode, "%s; putting the checkout back failed too (%v)\nrun: metasystem work status %s", firstLine(cause), rollbackErr, req.GoalID)
		}
		return cause
	}
	if current != goalBranchRef(req.GoalID) {
		if err := r.effects.Attach(req.Repo, goalBranchRef(req.GoalID)); err != nil {
			return rollbackCheckout(err)
		}
	}
	if err := r.effects.Publish(req.Repo, req.GoalID, state.localTip, newTip, state.originTip); err != nil {
		if headErr := r.effects.Restore(req.Repo, current, currentCommit); headErr != nil {
			return fmt.Errorf("%v; HEAD rollback failed: %w", err, headErr)
		}
		return rollbackCheckout(err)
	}
	return nil
}

func (r commitRepository) commitStagedOnto(req CommitRequest, state commitBranchState, subject, trailer string) (string, error) {
	if state.adopt {
		if err := r.adoptionCheckoutClean(req, state, nil); err != nil {
			return "", err
		}
	}
	patch, err := r.facts.Patch(req.Repo)
	if err != nil {
		return "", err
	}
	if req.FrozenPatch != nil {
		patch = req.FrozenPatch
	}
	newTip, err := r.buildCommitOnto(req, state, subject, trailer, patch)
	if err != nil {
		return "", err
	}
	var undo func() error
	if req.BeforeInstall != nil {
		undo, err = req.BeforeInstall()
		if err != nil {
			return "", err
		}
	}
	if req.PrepareOnly {
		return newTip, nil
	}
	if err := r.installCommitOnto(req, state, newTip); err != nil {
		if undo != nil {
			if rollback := undo(); rollback != nil {
				return "", fmt.Errorf("%v; patch restoration failed: %w", err, rollback)
			}
		}
		return "", err
	}
	return newTip, nil
}

func treeWithoutPaths(repo, tree string, paths []string) (string, error) {
	if len(paths) == 0 {
		return tree, nil
	}
	scratch, done, err := diskstore.ScratchDir("goal-branch-tree-*")
	if err != nil {
		return "", err
	}
	defer done()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	if _, err := gitInputEnv(repo, env, nil, "read-tree", tree); err != nil {
		return "", err
	}
	args := append([]string{"update-index", "--force-remove", "--"}, paths...)
	if _, err := gitInputEnv(repo, env, nil, args...); err != nil {
		return "", err
	}
	out, err := gitInputEnv(repo, env, nil, "write-tree")
	return strings.TrimSpace(string(out)), err
}

func (r commitRepository) amendUnit(req CommitRequest, state commitBranchState) (string, error) {
	units, unitsErr := requestUnits(req)
	if req.Kind != Unit || unitsErr != nil || len(units) == 0 {
		return "", fmt.Errorf("--amend needs --kind unit and one or more unit names")
	}
	list := unitList(units)
	previous := state.baseTip
	paths, err := r.facts.Staged(req.Repo)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 && req.Kind != Drop {
		return "", fmt.Errorf("the staged tree has no change to commit")
	}
	for _, path := range paths {
		if class := PathClass(path); class != ClassUnit {
			return "", operationRefusal(RangeCode, "a build may not change %s (a %s file)\nrun: metasystem work status %s", path, class, req.GoalID)
		}
	}
	commits, err := r.facts.Range(req.Repo, req.EndpointTip, previous, req.GoalID)
	if err != nil {
		return "", err
	}
	target := ""
	for _, commit := range commits {
		if commit.Kind == Unit && sameUnits(commit.Units, units) {
			if target != "" {
				return "", operationRefusal(RangeCode, "goal %s's branch holds build %s twice\nrun: metasystem work status %s", req.GoalID, list, req.GoalID)
			}
			target = commit.ID
		}
	}
	if target == "" {
		return "", operationRefusal(RangeCode, "goal %s's branch has no build %s to correct\nrun: metasystem work status %s", req.GoalID, list, req.GoalID)
	}
	patch, err := r.facts.Patch(req.Repo)
	if err != nil {
		return "", err
	}
	wantedTree, err := r.facts.Index(req.Repo)
	if err != nil {
		return "", err
	}
	suffix, err := r.facts.Suffix(req.Repo, target, previous)
	if err != nil {
		return "", err
	}
	worktree, close, err := r.openCommitWorktree(req, target, true)
	if err != nil {
		return "", err
	}
	defer req.closeWorktree(close)
	if req.ResumeWorktree == "" {
		if err := r.effects.Apply(worktree, patch); err != nil {
			return "", operationRefusal(RangeCode, "the staged fix doesn't apply to build %s: %v\nrun: metasystem work status %s", list, err, req.GoalID)
		}
	}
	if req.BeforeCommit != nil {
		if err := req.BeforeCommit(worktree, previous, wantedTree); err != nil {
			return "", err
		}
	}
	subject, trailer, err := commitMessage(req, "")
	if err != nil {
		return "", err
	}
	if err := r.effects.Commit(worktree, subject, trailer, true); err != nil {
		return "", err
	}
	var skippedPaths []string
	for _, commit := range suffix {
		info, err := r.facts.Kind(worktree, commit, req.GoalID)
		if err != nil {
			return "", err
		}
		if info.Kind == Read && info.CommitID == target {
			entries, err := r.facts.Entries(worktree, commit)
			if err != nil {
				return "", err
			}
			for _, entry := range entries {
				skippedPaths = append(skippedPaths, entry.Path)
			}
			continue
		}
		if err := r.effects.Replay(worktree, commit); err != nil {
			return "", operationRefusal(RangeCode, "after correcting build %s, the later commit %s no longer applies: %v\nrun: metasystem work status %s", list, commit, err, req.GoalID)
		}
	}
	newTip, err := r.facts.Head(worktree)
	if err != nil {
		return "", err
	}
	newTree, err := r.facts.Tree(worktree)
	if err != nil {
		return "", err
	}
	if !state.adopt {
		wantedTree, err = r.effects.WithoutPaths(req.Repo, wantedTree, skippedPaths)
		if err != nil {
			return "", err
		}
		if newTree != wantedTree {
			return "", operationRefusal(RangeCode, "after the correction, the goal branch doesn't hold what was staged\nrun: metasystem work status %s", req.GoalID)
		}
	}
	if _, err := r.facts.Range(worktree, req.EndpointTip, newTip, req.GoalID); err != nil {
		return "", err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return "", err
	}
	// Keep the old branch before moving it so an interrupted correction can still carry its reviews.
	if err := r.effects.KeepTip(req.Repo, req.GoalID, state.baseTip); err != nil {
		return "", err
	}
	if err := r.installCommitOnto(req, state, newTip); err != nil {
		return "", err
	}
	return newTip, nil
}

func (req CommitRequest) closeWorktree(close func()) {
	if req.KeepWorktree == nil || !req.KeepWorktree() {
		close()
	}
}

// CloseCommitWorktree removes the exact scratch tree created for publication.
func CloseCommitWorktree(repo, worktree string) error {
	prefix, err := gitOutput(repo, "rev-parse", "--show-prefix")
	if err != nil {
		return err
	}
	_ = os.Remove(landpath.TokenPath(filepath.Join(worktree, strings.TrimRight(string(prefix), "\n"))))
	if _, err := gitOutput(repo, "worktree", "remove", "--force", worktree); err != nil {
		return err
	}
	return os.Remove(filepath.Dir(worktree))
}

func (r commitRepository) openCommitWorktree(req CommitRequest, base string, amend bool) (string, func(), error) {
	if req.ResumeWorktree == "" {
		return r.effects.Open(req.Repo, base, amend)
	}
	prefix, err := gitOutput(req.Repo, "rev-parse", "--show-prefix")
	if err != nil {
		return "", nil, err
	}
	if err := mintScratchCommitToken(filepath.Join(req.ResumeWorktree, strings.TrimRight(string(prefix), "\n"))); err != nil {
		return "", nil, err
	}
	return req.ResumeWorktree, func() { _ = CloseCommitWorktree(req.Repo, req.ResumeWorktree) }, nil
}
