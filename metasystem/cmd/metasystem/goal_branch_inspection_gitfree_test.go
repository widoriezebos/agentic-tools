package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

func inspectionID(hex byte) string { return strings.Repeat(string(hex), 40) }

func inspectionAnswer(root string, output []byte, args ...string) testgit.Expectation {
	return testgit.Expectation{Call: testgit.Call{Dir: root, Args: args}, Result: testgit.Result{Stdout: output}}
}

func inspectionFailure(root string, err error, args ...string) testgit.Expectation {
	return testgit.Expectation{Call: testgit.Call{Dir: root, Args: args}, Result: testgit.Result{Err: err}}
}

func inspectionConfig(root, branch string) []testgit.Expectation {
	return []testgit.Expectation{
		inspectionAnswer(root, []byte("upstream\n"), "config", "--get", "goal.sync-remote"),
		inspectionAnswer(root, []byte(branch+"\n"), "config", "--get", "goal.sync-branch"),
	}
}

func inspectionCheckPrefix(root, branch, base string) []testgit.Expectation {
	return append(inspectionConfig(root, branch), inspectionAnswer(root, []byte(base+"\n"),
		"rev-parse", "--verify", "refs/remotes/upstream/main^{commit}"))
}

type inspectionCommit struct {
	id, parent, trailer, path string
}

func inspectionTree(path string) []byte {
	return []byte(":000000 100644 " + inspectionID('0') + " " + inspectionID('1') + " A\x00" + path + "\x00")
}

func inspectionCheckCalls(root, base, tip string, series []inspectionCommit) []testgit.Expectation {
	calls := append(inspectionCheckPrefix(root, "refs/heads/main", base),
		inspectionAnswer(root, []byte(tip+"\n"), "rev-parse", "--verify", "-q", tip+"^{commit}"),
		inspectionAnswer(root, []byte(base+"\n"), "merge-base", base, tip))
	var rows strings.Builder
	for _, commit := range series {
		fmt.Fprintf(&rows, "%s %s\n", commit.id, commit.parent)
	}
	calls = append(calls, inspectionAnswer(root, []byte(rows.String()), "rev-list", "--first-parent", "--reverse", "--parents", base+".."+tip))
	for _, commit := range series {
		calls = append(calls,
			inspectionAnswer(root, []byte(commit.trailer), "show", "-s", "--format=%(trailers:only,unfold=true)", commit.id),
			inspectionAnswer(root, []byte(commit.id+" "+commit.parent+"\n"), "rev-list", "--parents", "-n", "1", commit.id),
			inspectionAnswer(root, inspectionTree(commit.path), "diff-tree", "-r", "-z", "--no-renames", "--full-index", commit.id+"^", commit.id))
		if strings.HasPrefix(commit.trailer, "Goal-Unit:") && commit.path != "metasystem/plans/bad.md" {
			calls = append(calls,
				inspectionAnswer(root, []byte(commit.id+" "+commit.parent+"\n"), "rev-list", "--parents", "-n", "1", commit.id),
				inspectionAnswer(root, inspectionTree(commit.path), "diff-tree", "-r", "-z", "--no-renames", "--full-index", commit.id+"^", commit.id))
		}
	}
	return calls
}

func inspectionReaders(t *testing.T, root string, expected ...testgit.Expectation) (
	func(string, string) (string, error), func(string, ...string) (string, error), func(string, ...string) ([]byte, error),
) {
	t.Helper()
	stub := testgit.New(t, expected...)
	config := func(repo, key string) (string, error) {
		result := stub.Run(testgit.Call{Dir: repo, Args: []string{"config", "--get", key}})
		return string(result.Stdout), result.Err
	}
	cli := func(repo string, args ...string) (string, error) {
		result := stub.Run(testgit.Call{Dir: repo, Args: args})
		return strings.TrimSpace(string(result.Stdout)), result.Err
	}
	rangeRead := func(repo string, args ...string) ([]byte, error) {
		result := stub.Run(testgit.Call{Dir: repo, Args: args})
		return result.Stdout, result.Err
	}
	return config, cli, rangeRead
}

func inspectionMissingRefError(t *testing.T) error {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestGoalBranchMissingRefExitChild$")
	child.Env = append(os.Environ(), "GOAL_BRANCH_MISSING_REF_CHILD=1")
	err = child.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("missing-ref child exit: %v", err)
	}
	return fmt.Errorf("missing goal ref: %w", err)
}

func TestGoalBranchMissingRefExitChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOAL_BRANCH_MISSING_REF_CHILD") == "1" {
		os.Exit(1)
	}
}
