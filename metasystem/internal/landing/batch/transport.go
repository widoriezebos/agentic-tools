package batch

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// EndpointPushError preserves whether git named a stale lease or another
// remote rejection, so the landing state machine does not infer policy from a
// generic non-zero exit.
type EndpointPushError struct {
	Cause                      error
	StaleLease, RemoteRejected bool
}

func (err *EndpointPushError) Error() string { return err.Cause.Error() }
func (err *EndpointPushError) Unwrap() error { return err.Cause }

func IsNonLeaseEndpointRejection(err error) bool {
	var push *EndpointPushError
	return errors.As(err, &push) && push.RemoteRejected && !push.StaleLease
}

func IsStaleEndpointLease(err error) bool {
	var push *EndpointPushError
	return errors.As(err, &push) && push.StaleLease
}

func classifyEndpointPushError(text string, cause error) error {
	return &EndpointPushError{Cause: cause, StaleLease: strings.Contains(text, "stale info"),
		RemoteRejected: strings.Contains(text, "[rejected]") || strings.Contains(text, "[remote rejected]")}
}

func landingBranchRef(id string) string { return "refs/heads/landing/" + id }

// PrepareLandingBranch rebuilds the local assembly branch at the exact base.
// The landing checkout is dedicated to the owner, so resetting this named
// branch is the recoverable local half of the series transaction.
func PrepareLandingBranch(root, id, baseCommit string) error {
	return runLandingGit(root, "BATCH_LANDING_BRANCH_PREP_REFUSED", "checkout", "-B", "landing/"+id, baseCommit)
}

// PublishLandingBranch exposes the exact candidate commit under a leased
// batch ref. An empty expected tip means the ref must still be absent.
func PublishLandingBranch(root, id, expected, tip string) error {
	lease := "--force-with-lease=" + landingBranchRef(id) + ":" + expected
	command := exec.Command("git", "-C", root, "push", "origin", tip+":"+landingBranchRef(id), lease)
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	if output, err := command.CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(output))
		cause := fmt.Errorf("%s: %s: %w", codeLandingBranchMoved, text, err)
		return classifyEndpointPushError(text, cause)
	}
	return nil
}

func DeleteLandingBranch(root, id, expected string) error {
	if expected == "" {
		return nil
	}
	lease := "--force-with-lease=" + landingBranchRef(id) + ":" + expected
	if err := runLandingGit(root, "BATCH_LANDING_BRANCH_MOVED", "push", "origin", ":"+landingBranchRef(id), lease); err != nil {
		present, lookupErr := remoteLandingBranchPresent(root, id)
		if lookupErr == nil && !present {
			return nil
		}
		return err
	}
	return nil
}

// RebuildLandingBranch replaces a proved candidate after a whole member is
// ejected. Each surviving contribution remains a distinct commit in original join
// order, and the old branch tip is the authority for replacement.
func RebuildLandingBranch(root, id, baseTree, expected, actor string, survivors []Unit) (string, error) {
	baseCommit, err := commitForTreeInRef(root, "origin/main", baseTree)
	if err != nil {
		return "", err
	}
	parent, tree := baseCommit, baseTree
	for _, unit := range survivors {
		if unit.IsChange() {
			next, err := assembleChangeUnit(root, tree, unit)
			if err != nil {
				return "", err
			}
			commit, err := commitChange(root, unit, next, parent)
			if err != nil {
				return "", fmt.Errorf("%s: %w", codeLandingBranchPrepRefused, err)
			}
			parent, tree = commit, next
			continue
		}
		for index, build := range unit.Builds {
			prefixes, err := AssembleBranchMembers(root, tree, []BranchMember{{GoalID: unit.GoalID, Tip: unit.BranchTip, Builds: []BranchBuild{build}}})
			if err != nil {
				return "", err
			}
			tree = prefixes[0]
			command := exec.Command("git", "-C", root, "commit-tree", tree, "-p", parent)
			command.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_NAME="+unit.AuthorName, "GIT_AUTHOR_EMAIL="+unit.AuthorEmail,
				"GIT_COMMITTER_NAME="+unit.AuthorName, "GIT_COMMITTER_EMAIL="+unit.AuthorEmail)
			command.Stdin = strings.NewReader(BranchLandingMessage(unit.GoalID, build, unit.GoalLast && index == len(unit.Builds)-1) + "Landed-By: " + actor + "\n")
			output, err := command.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("%s: %s: %w", codeLandingBranchPrepRefused, strings.TrimSpace(string(output)), err)
			}
			parent = strings.TrimSpace(string(output))
		}
	}
	if parent == baseCommit {
		return "", fmt.Errorf("%s: no surviving builds", codeLandingBranchPrepRefused)
	}
	present, err := remoteLandingBranchPresent(root, id)
	if err != nil {
		return "", err
	}
	if !present {
		expected = ""
	}
	if err := PublishLandingBranch(root, id, expected, parent); err != nil {
		if !IsStaleEndpointLease(err) {
			return "", err
		}
		remoteTip, remoteTree, present, inspectErr := remoteLandingBranchTipAndTree(root, id)
		if inspectErr != nil {
			return "", errors.Join(err, inspectErr)
		}
		if !present || remoteTree != tree {
			return "", err
		}
		return remoteTip, nil
	}
	return parent, nil
}

func commitForTreeInRef(root, ref, tree string) (string, error) {
	command := exec.Command("git", "-C", root, "log", "--first-parent", "--format=%H %T", ref)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == tree {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s: no %s commit has base tree %s", codeLandTrunkMoved, ref, tree)
}

// LandLandingBranch performs the only endpoint update and deletes the
// candidate branch in the same atomic remote transaction. Both refs are
// leased, so a moved endpoint or candidate writes neither ref.
func LandLandingBranch(root, id, baseCommit, tip string) error {
	args := []string{"push", "--atomic", "origin",
		tip + ":refs/heads/main", ":" + landingBranchRef(id),
		"--force-with-lease=refs/heads/main:" + baseCommit,
		"--force-with-lease=" + landingBranchRef(id) + ":" + tip}
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	if output, err := command.CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(output))
		cause := fmt.Errorf("%s: %s: %w", codeLandPushRefused, text, err)
		return classifyEndpointPushError(text, cause)
	}
	return nil
}

// AbandonLandingBranch removes an unlanded candidate against its exact tip
// before a proof-input-changing trunk move returns the batch to open.
func AbandonLandingBranch(root, id, expectedTip, detachAt string) error {
	present, err := remoteLandingBranchPresent(root, id)
	if err != nil {
		return err
	}
	if !present {
		return CleanupLandingBranch(root, id, detachAt)
	}
	if err := runLandingGit(root, "BATCH_LANDING_BRANCH_MOVED", "push", "origin", ":"+landingBranchRef(id),
		"--force-with-lease="+landingBranchRef(id)+":"+expectedTip); err != nil {
		present, lookupErr := remoteLandingBranchPresent(root, id)
		if lookupErr == nil && !present {
			return CleanupLandingBranch(root, id, detachAt)
		}
		return err
	}
	return CleanupLandingBranch(root, id, detachAt)
}

func remoteLandingBranchPresent(root, id string) (bool, error) {
	_, present, err := remoteLandingBranchTip(root, id)
	return present, err
}

func remoteLandingBranchTip(root, id string) (string, bool, error) {
	command := exec.Command("git", "-C", root, "ls-remote", "--heads", "origin", landingBranchRef(id))
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return "", false, fmt.Errorf("%s: inspect %s: %s: %w", codeLandingBranchMoved, landingBranchRef(id), strings.TrimSpace(string(output)), err)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", false, nil
	}
	if len(fields) != 2 {
		return "", false, fmt.Errorf("%s: inspect %s returned an unreadable ref", codeLandingBranchMoved, landingBranchRef(id))
	}
	return fields[0], true, nil
}

func remoteLandingBranchTipAndTree(root, id string) (string, string, bool, error) {
	tip, present, err := remoteLandingBranchTip(root, id)
	if err != nil || !present {
		return tip, "", present, err
	}
	tree, err := landingGitOutput(root, "rev-parse", tip+"^{tree}")
	if err != nil {
		return "", "", false, fmt.Errorf("%s: inspect %s tree: %w", codeLandingBranchMoved, landingBranchRef(id), err)
	}
	return tip, tree, true, nil
}

// CleanupLandingBranch leaves no checked-out landing branch after the remote
// transaction, which also makes this cleanup safe to repeat after recovery.
func CleanupLandingBranch(root, id, tip string) error {
	if err := runLandingGit(root, "BATCH_LAND_CLEANUP_REFUSED", "checkout", "--detach", tip); err != nil {
		return err
	}
	command := exec.Command("git", "-C", root, "branch", "-D", "landing/"+id)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil && !strings.Contains(string(output), "not found") {
		return fmt.Errorf("%s: %s: %w", codeLandCleanupRefused, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func runLandingGit(root, code string, args ...string) error {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", code, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CommitDeclaration is the landing declaration a unit commits under.
type CommitDeclaration struct {
	chain, attested, snapshot, base string
}

func ChainDeclaration(chain string) CommitDeclaration {
	return CommitDeclaration{chain: chain}
}

func AttestedDeclaration(commit, snapshot, base string) CommitDeclaration {
	return CommitDeclaration{attested: commit, snapshot: snapshot, base: base}
}

// CommitBoundary runs the commit boundary (landpath.Commit) for one request
// and returns what it printed and its exit status.
type CommitBoundary func(landpath.CommitRequest) (string, int)

func CommitWithWrapperWithRead(root string, declaration CommitDeclaration, goalID, receipt, message, authorName, authorEmail, landedBy, lineage string, commit CommitBoundary, readGit func(root string, args ...string) (string, error)) error {
	if authorName == "" || authorEmail == "" {
		return fmt.Errorf("%s: goal %s has no configured approver identity", codeLandAuthorUnbound, goalID)
	}
	before, err := readGit(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	// The message lives outside the work tree: an untracked file there is a
	// working-tree byte the commit would not record, which the boundary
	// refuses.
	messageFile, done, err := diskstore.ScratchFile("metasystem-batch-commit-message-*")
	if err != nil {
		return err
	}
	name := messageFile.Name()
	defer done()
	if _, err := messageFile.WriteString(message); err != nil {
		messageFile.Close()
		return err
	}
	if err := messageFile.Close(); err != nil {
		return err
	}
	request := landpath.CommitRequest{Root: root, Chain: declaration.chain, Attested: declaration.attested,
		AttestedSnapshot: declaration.snapshot, AttestedBase: declaration.base, Goal: goalID, GoalSet: true,
		TestReceipt: receipt, MessageFile: name, OwnerLineage: lineage, LandedBy: landedBy,
		Env: []string{"GIT_AUTHOR_NAME=" + authorName, "GIT_AUTHOR_EMAIL=" + authorEmail,
			"GIT_COMMITTER_NAME=" + authorName, "GIT_COMMITTER_EMAIL=" + authorEmail}}
	if output, status := commit(request); status != 0 {
		return fmt.Errorf("commit boundary for goal %s exited %d: %s", goalID, status, strings.TrimSpace(output))
	}
	after, err := readGit(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if after == before {
		return refuseBatch("BATCH_LAND_UNPROVENANCED", fmt.Sprintf("goal %s commit boundary did not advance HEAD by exactly one commit", goalID))
	}
	parent, err := readGit(root, "rev-parse", after+"^")
	if err != nil || parent != before {
		return refuseBatch("BATCH_LAND_UNPROVENANCED", fmt.Sprintf("goal %s commit boundary did not advance HEAD by exactly one commit", goalID))
	}
	return nil
}

func landingGitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}

func RequirePassingCommitVerdictWithRead(root, goalID, commit string, readGit func(root string, args ...string) (string, error)) error {
	output, err := readGit(root, "show", "-s", "--format=%(trailers:key=Landing-Provenance-Verdict,valueonly)", commit)
	verdict := strings.TrimSpace(output)
	if err != nil {
		verdict = "unreadable"
	}
	if verdict != "pass" && !strings.HasPrefix(verdict, "pass ") {
		return refuseBatch("BATCH_LAND_UNPROVENANCED", fmt.Sprintf("goal %s commit %s verdict %s", goalID, commit, verdict))
	}
	return nil
}
