package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPermissions turns a permissions envelope into the absolute-rooted
// form a launch is measured against: "." becomes the repository, "<worktree>"
// becomes the job workspace, relative roots resolve against the repository.
// Writable roots demand a worktree and must stay inside it — a delegate never
// writes outside the workspace it was given. A repository-wide network floor
// of deny overrides whatever the envelope asked for; it only ever narrows.
func ExpandPermissions(sourcePath, repo, workspace string, isWorktree bool, preset, networkFloor, outputPath string) error {
	data, err := readJSON(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid permissions envelope: %v", err)
	}
	envelope, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
	}
	expected := []string{"readRoots", "writeRoots", "network", "approvals", "tools"}
	if len(envelope) != len(expected) {
		return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
	}
	for _, key := range expected {
		if _, present := envelope[key]; !present {
			return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
		}
	}
	readRoots, readOK := envelope["readRoots"].([]any)
	writeRoots, writeOK := envelope["writeRoots"].([]any)
	if !readOK || !writeOK {
		return fmt.Errorf("permission roots must be arrays")
	}

	repoResolved := resolvePath(repo)
	workspaceResolved := resolvePath(workspace)
	expand := func(value string) string {
		switch {
		case value == ".":
			return repoResolved
		case value == "<worktree>":
			return workspaceResolved
		case filepath.IsAbs(value):
			return resolvePath(value)
		default:
			return resolvePath(filepath.Join(repoResolved, value))
		}
	}
	expandAll := func(items []any) []any {
		expanded := []any{}
		for _, item := range items {
			if value, ok := item.(string); ok {
				expanded = append(expanded, expand(value))
			}
		}
		return expanded
	}
	envelope["readRoots"] = expandAll(readRoots)
	expandedWrite := expandAll(writeRoots)
	envelope["writeRoots"] = expandedWrite

	if len(expandedWrite) > 0 && !isWorktree {
		return fmt.Errorf("writable permissions require --worktree")
	}
	for _, item := range expandedWrite {
		root := item.(string)
		if !pathWithin(resolvePath(root), workspaceResolved) {
			return fmt.Errorf("permission write root escapes the job worktree: %s", root)
		}
	}
	// A worktree delegate must be able to COMMIT its own rounds (issue
	// #5): git keeps the worktree's index and HEAD under the MAIN repo's
	// .git/worktrees/<name>, writes loose objects into the common
	// objects/, and updates the branch ref (plus its reflog) under the
	// common refs/heads — all outside the worktree, all read-only under
	// the old envelope, so every worktree implementer's commit died with
	// EROFS and cost its mission a cycle. The roots are DERIVED from the
	// worktree by the engine, never accepted from the caller, and they
	// grant exactly: this worktree's git dir, the shared object store
	// (content-addressed; unreachable garbage at worst), and the agent
	// branch namespace — never main's ref, never HEAD of the checkout.
	if isWorktree && len(expandedWrite) > 0 {
		gitRoots, err := worktreeGitWriteRoots(workspaceResolved)
		if err != nil {
			return err
		}
		for _, root := range gitRoots {
			expandedWrite = append(expandedWrite, root)
		}
		envelope["writeRoots"] = expandedWrite
	}
	if networkFloor == "deny" {
		envelope["network"] = "deny"
	}
	envelope["preset"] = preset
	return writeRecord(outputPath, envelope)
}

// worktreeGitWriteRoots derives the git-metadata write roots a worktree
// delegate needs to commit on its own branch, verified against
// a live probe: the worktree git dir (index, HEAD, COMMIT_EDITMSG, its
// logs), the common object store, and the branch's ref namespace with its
// reflog (ref updates create sibling .lock files, so the namespace
// DIRECTORY must be writable, not just the ref file).
func worktreeGitWriteRoots(worktree string) ([]string, error) {
	gitDir, err := gitOutput(worktree, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, fmt.Errorf("worktree git dir unreadable: %v", err)
	}
	commonDir, err := gitOutput(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("worktree common git dir unreadable: %v", err)
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktree, commonDir)
	}
	// The files ref backend is the only one these roots model:
	// a reftable repository writes shared reftable storage instead,
	// and granting refs/heads there would be meaningless while commits
	// silently failed elsewhere.
	if _, err := os.Stat(filepath.Join(commonDir, "reftable")); err == nil {
		return nil, fmt.Errorf("worktree delegate commits require the files ref backend; this repository uses reftable")
	}
	branch, err := gitOutput(worktree, "branch", "--show-current")
	if err != nil || branch == "" {
		return nil, fmt.Errorf("worktree delegate must be on its own branch (detached or unreadable HEAD)")
	}
	// Only the agent/ namespace is grantable (a bare branch named
	// "topic" would grant ALL of refs/heads,
	// main included). Dispatch creates agent/<job>; anything else refuses.
	if !strings.HasPrefix(branch, "agent/") {
		return nil, fmt.Errorf("worktree delegate must sit on an agent/ branch, not %q", branch)
	}
	refDir := filepath.Join(commonDir, "refs", "heads", "agent")
	logDir := filepath.Join(commonDir, "logs", "refs", "heads", "agent")
	// Pre-create the reflog namespace so the sandbox root resolves — and
	// refuse when it cannot be created: a discarded error
	// here loses the delegate's commits for the whole cycle.
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot prepare the reflog namespace: %v", err)
	}
	// NO shared objects/ grant: loose objects go to the
	// worktree's quarantine store (GIT_OBJECT_DIRECTORY, set by the
	// adapters; alternates-linked by the engine at worktree creation),
	// which the workspace write root already covers. The shared object
	// database stays read-only to the delegate.
	return []string{
		resolvePath(gitDir),
		resolvePath(refDir),
		resolvePath(logDir),
	}, nil
}
