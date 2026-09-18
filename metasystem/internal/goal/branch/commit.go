package branch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	UnavailableCode   = "GOAL_BRANCH_UNAVAILABLE"
	NotHolderCode     = "GOAL_BRANCH_NOT_HOLDER"
	CheckoutArmedCode = "GOAL_BRANCH_CHECKOUT_ARMED"
)

type OpError struct {
	Code    string
	Message string
}

func (e *OpError) Error() string { return e.Code + ": " + e.Message }

func operationRefusal(code, format string, args ...any) error {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// pushProtocolAvailable is set by the push owner when the binary can publish
// every commit it creates. A binary assembled without that owner fails closed.
var pushProtocolAvailable bool

func PushProtocolAvailable() bool { return pushProtocolAvailable }

type CommitRequest struct {
	Repo, Remote, EndpointTip, GoalID, Unit, OpID string
	Units                                         []string
	Kind                                          Kind
	Amend                                         bool
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
		return operationRefusal(NotHolderCode, "the goal claim cannot be verified")
	}
	if err := check(); err != nil {
		return operationRefusal(NotHolderCode, "%v", err)
	}
	return nil
}

func CheckHolder(check func() error) error { return checkClaim(check) }

func CheckCommitAccess(goalID string, check func() error) error {
	if !pushProtocolAvailable {
		return operationRefusal(UnavailableCode, "this binary cannot publish goal branches")
	}
	if !validName(goalID) {
		return fmt.Errorf("goal id must be one nonempty path-free word")
	}
	return checkClaim(check)
}

// CheckCommitCheckout keeps an enrolled checkout on its endpoint branch. A
// linked worktree and a plain clone already on the goal branch are the two
// places where the commit verb may install its new tip.
func CheckCommitCheckout(repo, goalID string, linked bool) error {
	if linked {
		return nil
	}
	currentOut, _ := gitOutput(repo, "symbolic-ref", "-q", "HEAD")
	if strings.TrimSpace(string(currentOut)) == goalBranchRef(goalID) {
		return nil
	}
	checkout, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	return operationRefusal(CheckoutArmedCode,
		"checkout %s is not on goal/%s; run git worktree add <path> goal/%s, then run goal branch commit there",
		filepath.Clean(checkout), goalID, goalID)
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
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	if req.Remote == "" || !validName(req.OpID) {
		return commitBranchState{}, fmt.Errorf("commit needs a remote and operation id")
	}
	state := commitBranchState{}
	var err error
	state.localTip, state.localPresent, err = localBranchTip(req.Repo, goalBranchRef(req.GoalID))
	if err != nil {
		return state, err
	}
	if state.localPresent {
		if _, err := ValidateRange(req.Repo, req.EndpointTip, state.localTip, req.GoalID); err != nil {
			return state, err
		}
	}
	var remoteTip string
	remoteTip, state.remotePresent, err = req.Transport.RemoteTip(req.Repo, req.Remote, goalBranchRef(req.GoalID))
	if err != nil {
		return state, err
	}
	if state.remotePresent {
		if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, remoteTip, req.Transport); err != nil {
			return state, err
		}
	}
	if !state.localPresent && !state.remotePresent {
		headOut, err := gitOutput(req.Repo, "rev-parse", "HEAD^{commit}")
		if err != nil {
			return state, err
		}
		if strings.TrimSpace(string(headOut)) != req.EndpointTip {
			return state, operationRefusal(RangeCode, "the first commit must start at endpoint tip %s", req.EndpointTip)
		}
	}
	state.originTip, _, err = localBranchTip(req.Repo, originTipRef(req.GoalID))
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
		remoteBuiltOnLocal, err := ancestor(req.Repo, state.localTip, remoteTip)
		if err != nil {
			return state, err
		}
		if remoteBuiltOnLocal {
			state.baseTip, state.originTip, state.adopt = remoteTip, remoteTip, true
			return state, nil
		}
		builtOnRemote, err := ancestor(req.Repo, remoteTip, state.localTip)
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
		return state, staleBranch(req.Remote, state.localTip, remoteTip, state.originTip)
	case state.originTip != "":
		return state, staleBranch(req.Remote, state.localTip, "", state.originTip)
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
		return "goal " + req.GoalID + " units " + list, "Goal-Unit: " + req.GoalID + "/" + list, nil
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
	reads, prose := 0, 0
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		class := PathClass(path)
		allowed := kind == Unit && class == ClassUnit ||
			kind == Plan && (class == ClassPlan || path == landingRecordPath(goalID))
		if kind == Read {
			allowed = class == ClassRead || class == ClassReadProse
		}
		if !allowed {
			return operationRefusal(RangeCode, "path %s has class %s, which kind %s does not allow", path, class, kind)
		}
		if class == ClassRead {
			reads++
		}
		if class == ClassReadProse {
			prose++
		}
	}
	if kind == Read && (reads != 1 || prose > 1) {
		return operationRefusal(RangeCode, "read commit needs one attestation and at most one prose record; found %d and %d", reads, prose)
	}
	return nil
}

func commitPrepared(req CommitRequest, subjectCommit string) (string, error) {
	if err := CheckCommitAccess(req.GoalID, req.CheckClaim); err != nil {
		return "", err
	}
	state, err := inspectCommitBranch(req)
	if err != nil {
		return "", err
	}
	return commitPreparedState(req, subjectCommit, state)
}

func commitPreparedState(req CommitRequest, subjectCommit string, state commitBranchState) (string, error) {
	if req.Amend {
		return amendUnit(req, state)
	}
	paths, err := stagedPaths(req.Repo)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("the staged tree has no change to commit")
	}
	if err := validateCommitPaths(req.Kind, paths, req.GoalID); err != nil {
		return "", err
	}
	subject, trailer, err := commitMessage(req, subjectCommit)
	if err != nil {
		return "", err
	}
	return commitStagedOnto(req, state, subject, trailer)
}

func CommitStaged(req CommitRequest) (string, error) { return commitPrepared(req, "") }

func adoptionCheckoutClean(req CommitRequest, state commitBranchState, allowed []string) error {
	unstaged, err := gitOutput(req.Repo, "diff", "--name-only", "-z")
	if err != nil {
		return err
	}
	allowedSet := map[string]bool{}
	for _, path := range allowed {
		allowedSet[path] = true
	}
	for _, item := range bytes.Split(unstaged, []byte{0}) {
		if len(item) != 0 && !allowedSet[string(item)] {
			return operationRefusal(StaleCode, "local tip %s cannot adopt remote tip %s while the checkout has unstaged tracked changes", state.localTip, state.baseTip)
		}
	}
	return nil
}

func buildCommitOnto(req CommitRequest, state commitBranchState, subject, trailer string, patch []byte) (string, error) {
	scratch, err := os.MkdirTemp("", "goal-branch-adopt-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	worktree := filepath.Join(scratch, "worktree")
	if _, err := gitOutput(req.Repo, "worktree", "add", "--quiet", "--detach", worktree, state.baseTip); err != nil {
		return "", err
	}
	defer func() { _, _ = gitOutput(req.Repo, "worktree", "remove", "--force", worktree) }()
	if _, err := gitInput(worktree, patch, "apply", "--index", "--3way", "-"); err != nil {
		return "", operationRefusal(StaleCode, "the staged change does not apply to remote tip %s: %v", state.baseTip, err)
	}
	if _, err := gitOutput(worktree, "commit", "--quiet", "-m", subject, "-m", trailer); err != nil {
		return "", err
	}
	newTipOut, err := gitOutput(worktree, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	newTip := strings.TrimSpace(string(newTipOut))
	if _, err := ValidateRange(worktree, req.EndpointTip, newTip, req.GoalID); err != nil {
		return "", err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return "", err
	}
	return newTip, nil
}

func nulNames(data []byte) map[string]bool {
	names := map[string]bool{}
	for _, item := range bytes.Split(data, []byte{0}) {
		if len(item) != 0 {
			names[string(item)] = true
		}
	}
	return names
}

func checkoutInstallPreflight(req CommitRequest, newTip string) (string, string, error) {
	indexTreeOut, err := gitOutput(req.Repo, "write-tree")
	if err != nil {
		return "", "", err
	}
	indexTree := strings.TrimSpace(string(indexTreeOut))
	unstagedOut, err := gitOutput(req.Repo, "diff", "--name-only", "-z")
	if err != nil {
		return "", "", err
	}
	tipChangesOut, err := gitOutput(req.Repo, "diff", "--name-only", "-z", "--no-renames", indexTree, newTip)
	if err != nil {
		return "", "", err
	}
	unstaged := nulNames(unstagedOut)
	for path := range nulNames(tipChangesOut) {
		if unstaged[path] {
			return "", "", operationRefusal(StaleCode, "installing goal branch tip %s would overwrite unstaged tracked path %s", newTip, path)
		}
	}
	currentOut, _ := gitOutput(req.Repo, "symbolic-ref", "-q", "HEAD")
	current := strings.TrimSpace(string(currentOut))
	if current != goalBranchRef(req.GoalID) {
		worktreesOut, err := gitOutput(req.Repo, "worktree", "list", "--porcelain")
		if err != nil {
			return "", "", err
		}
		for _, record := range strings.Split(strings.TrimSpace(string(worktreesOut)), "\n\n") {
			lines := strings.Split(record, "\n")
			path, branch := "", ""
			for _, line := range lines {
				if strings.HasPrefix(line, "worktree ") {
					path = strings.TrimPrefix(line, "worktree ")
				}
				if strings.HasPrefix(line, "branch ") {
					branch = strings.TrimPrefix(line, "branch ")
				}
			}
			if branch == goalBranchRef(req.GoalID) {
				return "", "", operationRefusal(StaleCode, "goal branch %s is already checked out at %s", req.GoalID, path)
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
	current, indexTree, err := checkoutInstallPreflight(req, newTip)
	if err != nil {
		return err
	}
	currentCommitOut, err := gitOutput(req.Repo, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return err
	}
	currentCommit := strings.TrimSpace(string(currentCommitOut))
	if _, err := gitOutput(req.Repo, "read-tree", "-m", "-u", indexTree, newTip); err != nil {
		return err
	}
	rollbackCheckout := func(cause error) error {
		if _, rollbackErr := gitOutput(req.Repo, "read-tree", "-m", "-u", newTip, indexTree); rollbackErr != nil {
			return fmt.Errorf("%v; checkout rollback failed: %w", cause, rollbackErr)
		}
		return cause
	}
	if current != goalBranchRef(req.GoalID) {
		if _, err := gitOutput(req.Repo, "symbolic-ref", "HEAD", goalBranchRef(req.GoalID)); err != nil {
			return rollbackCheckout(err)
		}
	}
	if err := updateBranchAndOrigin(req.Repo, req.GoalID, state.localTip, newTip, state.originTip); err != nil {
		if headErr := restoreHead(req.Repo, current, currentCommit); headErr != nil {
			return fmt.Errorf("%v; HEAD rollback failed: %w", err, headErr)
		}
		return rollbackCheckout(err)
	}
	return nil
}

func commitStagedOnto(req CommitRequest, state commitBranchState, subject, trailer string) (string, error) {
	if err := adoptionCheckoutClean(req, state, nil); err != nil {
		return "", err
	}
	patch, err := gitOutput(req.Repo, "diff", "--cached", "--binary", "--full-index")
	if err != nil {
		return "", err
	}
	newTip, err := buildCommitOnto(req, state, subject, trailer, patch)
	if err != nil {
		return "", err
	}
	if err := installCommitOnto(req, state, newTip); err != nil {
		return "", err
	}
	return newTip, nil
}

func amendUnit(req CommitRequest, state commitBranchState) (string, error) {
	units, unitsErr := requestUnits(req)
	if req.Kind != Unit || unitsErr != nil || len(units) == 0 {
		return "", fmt.Errorf("--amend needs --kind unit and one or more unit names")
	}
	list := unitList(units)
	previous := state.baseTip
	paths, err := stagedPaths(req.Repo)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("the staged tree has no change to commit")
	}
	for _, path := range paths {
		if class := PathClass(path); class != ClassUnit {
			return "", operationRefusal(RangeCode, "path %s has class %s, which kind unit does not allow", path, class)
		}
	}
	commits, err := ValidateRange(req.Repo, req.EndpointTip, previous, req.GoalID)
	if err != nil {
		return "", err
	}
	target := ""
	for _, commit := range commits {
		if commit.Kind == Unit && sameUnits(commit.Units, units) {
			if target != "" {
				return "", operationRefusal(RangeCode, "goal branch repeats build %s", list)
			}
			target = commit.ID
		}
	}
	if target == "" {
		return "", operationRefusal(RangeCode, "goal branch has no build %s to amend", list)
	}
	patch, err := gitOutput(req.Repo, "diff", "--cached", "--binary", "--full-index")
	if err != nil {
		return "", err
	}
	indexTreeOut, err := gitOutput(req.Repo, "write-tree")
	if err != nil {
		return "", err
	}
	wantedTree := strings.TrimSpace(string(indexTreeOut))
	suffixOut, err := gitOutput(req.Repo, "rev-list", "--first-parent", "--reverse", target+".."+previous)
	if err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp("", "goal-branch-amend-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	worktree := filepath.Join(scratch, "worktree")
	if _, err := gitOutput(req.Repo, "worktree", "add", "--quiet", "--detach", worktree, target); err != nil {
		return "", err
	}
	defer func() { _, _ = gitOutput(req.Repo, "worktree", "remove", "--force", worktree) }()
	if _, err := gitInput(worktree, patch, "apply", "--index", "--3way", "-"); err != nil {
		return "", operationRefusal(RangeCode, "the staged fix does not apply to build %s: %v", list, err)
	}
	subject, trailer, err := commitMessage(req, "")
	if err != nil {
		return "", err
	}
	if _, err := gitOutput(worktree, "commit", "--quiet", "--amend", "-m", subject, "-m", trailer); err != nil {
		return "", err
	}
	skippedRead := false
	for _, commit := range strings.Fields(string(suffixOut)) {
		info, err := KindOf(worktree, commit, req.GoalID)
		if err != nil {
			return "", err
		}
		if info.Kind == Read && info.CommitID == target {
			skippedRead = true
			continue
		}
		if _, err := gitOutput(worktree, "cherry-pick", "--quiet", commit); err != nil {
			return "", operationRefusal(RangeCode, "commit %s does not replay after amending build %s: %v", commit, list, err)
		}
	}
	newTipOut, err := gitOutput(worktree, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	newTip := strings.TrimSpace(string(newTipOut))
	newTreeOut, err := gitOutput(worktree, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", err
	}
	if !state.adopt && !skippedRead && strings.TrimSpace(string(newTreeOut)) != wantedTree {
		return "", operationRefusal(RangeCode, "the replayed branch does not equal the staged tree")
	}
	if _, err := ValidateRange(worktree, req.EndpointTip, newTip, req.GoalID); err != nil {
		return "", err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return "", err
	}
	if err := installCommitOnto(req, state, newTip); err != nil {
		return "", err
	}
	return newTip, nil
}
