package batch

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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

func IsStaleEndpointLease(err error) bool {
	var push *EndpointPushError
	return errors.As(err, &push) && push.StaleLease
}

// classifyEndpointPushError reads a failed push from git's --porcelain ref
// status lines ("FLAG<TAB>FROM:TO<TAB>SUMMARY"): a ref flagged "!" was
// rejected, and one whose summary is "[rejected] (stale info)" failed its
// lease. Git's text on stderr is never read.
func classifyEndpointPushError(porcelain []byte, cause error) error {
	classified := &EndpointPushError{Cause: cause}
	for _, line := range strings.Split(string(porcelain), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] != "!" {
			continue
		}
		classified.RemoteRejected = true
		if fields[2] == "[rejected] (stale info)" {
			classified.StaleLease = true
		}
	}
	return classified
}

// pushPorcelain runs one git push with --porcelain and, when it fails, the
// error classified from its ref status lines, quoting stderr for a person.
func pushPorcelain(root, code string, args ...string) error {
	command := exec.Command("git", append([]string{"-C", root, "push", "--porcelain"}, args...)...)
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	porcelain, err := command.Output()
	if err == nil {
		return nil
	}
	said := ""
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		said = strings.TrimSpace(string(exit.Stderr))
	}
	return classifyEndpointPushError(porcelain, fmt.Errorf("%s: %s: %w", code, said, err))
}

func landingBranchRef(id string) string { return "refs/heads/landing/" + id }

// PublishLandingBranch exposes the exact candidate commit under a leased
// batch ref. An empty expected tip means the ref must still be absent.
func PublishLandingBranch(root, id, expected, tip string) error {
	lease := "--force-with-lease=" + landingBranchRef(id) + ":" + expected
	return pushPorcelain(root, codeLandingBranchMoved, "origin", tip+":"+landingBranchRef(id), lease)
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

func landingGitOutput(root string, args ...string) (string, error) {
	command := realGit(root, args...)
	command.Env = realObjectsEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
