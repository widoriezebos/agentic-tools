package batch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// CommandSpec is one explicit landing transport boundary.
type CommandSpec struct {
	Dir, Name string
	Args      []string
	Env       []string
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
	if err := runLandingGit(root, "BATCH_LANDING_BRANCH_MOVED", "push", "origin", tip+":"+landingBranchRef(id), lease); err != nil {
		return err
	}
	return nil
}

// LandLandingBranch performs the only endpoint update and deletes the
// candidate branch in the same atomic remote transaction. Both refs are
// leased, so a moved endpoint or candidate writes neither ref.
func LandLandingBranch(root, id, baseCommit, tip string) error {
	return runLandingGit(root, "BATCH_LAND_PUSH_REFUSED", "push", "--atomic", "origin",
		tip+":refs/heads/main", ":"+landingBranchRef(id),
		"--force-with-lease=refs/heads/main:"+baseCommit,
		"--force-with-lease="+landingBranchRef(id)+":"+tip)
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
	command := exec.Command("git", "-C", root, "ls-remote", "--heads", "origin", landingBranchRef(id))
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("BATCH_LANDING_BRANCH_MOVED: inspect %s: %s: %w", landingBranchRef(id), strings.TrimSpace(string(output)), err)
	}
	return len(strings.TrimSpace(string(output))) != 0, nil
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
		return fmt.Errorf("BATCH_LAND_CLEANUP_REFUSED: %s: %w", strings.TrimSpace(string(output)), err)
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

// RunCommand executes a landing boundary without a shell.
func RunCommand(spec CommandSpec) error {
	command := exec.Command(spec.Name, spec.Args...)
	command.Dir = spec.Dir
	command.Env = append(os.Environ(), spec.Env...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %s: %w", spec.Name, spec.Args, output, err)
	}
	return nil
}

// CommitWithWrapper invokes the repository commit boundary for one unit. The
// caller supplies the goal approver's configured identity; ambient git author
// and committer configuration is intentionally ignored.
func CommitWithWrapper(root, chain, goalID, receipt, message, authorName, authorEmail, landedBy string) error {
	if authorName == "" || authorEmail == "" {
		return fmt.Errorf("BATCH_LAND_AUTHOR_UNBOUND: goal %s has no configured approver identity", goalID)
	}
	messageFile, err := os.CreateTemp(root, ".batch-commit-message-*")
	if err != nil {
		return err
	}
	name := messageFile.Name()
	defer os.Remove(name)
	if _, err := messageFile.WriteString(message); err != nil {
		messageFile.Close()
		return err
	}
	if err := messageFile.Close(); err != nil {
		return err
	}
	return RunCommand(CommandSpec{Dir: root, Name: filepath.Join(root, "scripts", "agents", "commit.sh"),
		Args: []string{"--chain", chain, "--goal", goalID, "--test-receipt", receipt, "-F", name},
		Env: []string{"GIT_AUTHOR_NAME=" + authorName, "GIT_AUTHOR_EMAIL=" + authorEmail,
			"GIT_COMMITTER_NAME=" + authorName, "GIT_COMMITTER_EMAIL=" + authorEmail, "METASYSTEM_LANDED_BY=" + landedBy}})
}
