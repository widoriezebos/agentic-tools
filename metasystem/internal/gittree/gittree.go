// Package gittree is the ONE owner of the isolated-index tree projection
// (host-implementer wall, HIW-O4). Every component that needs to turn a
// worktree into a tree object, apply an authorized patch, or compare tree
// entries goes through this package, so "the tree" means the same bytes
// everywhere: the conformance validator's review object and the mission
// runner's wall equation are computed by the same code.
//
// The projection's stated boundaries (fixture-tested): git modes, symlink
// target blobs, binary content, deletions, and superproject gitlink ids are
// INSIDE the projection; arbitrary POSIX metadata, ignored untracked files,
// and dirty or untracked content inside a submodule are OUTSIDE it.
// core.fileMode is pinned true on every invocation so "mode" always means
// git mode, never a filesystem accident.
package gittree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// Workspace binds tree operations to one repository worktree. All index
// operations run against throwaway isolated indexes (GIT_INDEX_FILE in a
// temp directory), so the caller's real index is never read or written.
type Workspace struct {
	Dir string
}

var treeID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// configPins force every invocation onto one deterministic git behavior,
// whatever the repository or user config says (slice-1 critique F1/F4):
// core.fileMode so "mode" means git mode; prefix and driver settings so
// patch bytes are identical everywhere Diff runs; apply.ignoreWhitespace
// so "exact" admits no whitespace fuzz; logAllRefUpdates so anchor refs
// never grow reflogs that keep a dropped tree GC-reachable.
var configPins = []string{
	"-c", "core.fileMode=true",
	"-c", "diff.noprefix=false",
	"-c", "diff.mnemonicPrefix=false",
	"-c", "apply.ignoreWhitespace=no",
	"-c", "core.logAllRefUpdates=false",
}

// git runs one bounded git invocation in the workspace with the package's
// config pins applied. Bounded like every other external call (B4).
func (w Workspace) git(env []string, args ...string) ([]byte, error) {
	full := append(append([]string{"-C", w.Dir}, configPins...), args...)
	cmd := exec.Command("git", full...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	limit := boundedexec.Timeout(filepath.Join(w.Dir, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " ")); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", args[0], detail)
	}
	return stdout.Bytes(), nil
}

func (w Workspace) gitLine(env []string, args ...string) (string, error) {
	out, err := w.git(env, args...)
	return strings.TrimSpace(string(out)), err
}

// isolatedIndex returns the env for a throwaway index and its cleanup.
func isolatedIndex() ([]string, func(), error) {
	dir, err := os.MkdirTemp("", "metasystem-gittree.")
	if err != nil {
		return nil, nil, fmt.Errorf("gittree: %w", err)
	}
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	return env, func() { os.RemoveAll(dir) }, nil
}

// Snapshot projects the current worktree into a tree object: the isolated
// index is seeded from baseline (normally HEAD, so tracked-and-ignored
// files stay projected), then add -A captures every addition, modification,
// deletion, mode change, symlink, and gitlink, and write-tree names the
// result. The worktree and the real index are untouched.
func (w Workspace) Snapshot(baseline string) (string, error) {
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if _, err := w.git(env, "read-tree", baseline); err != nil {
		return "", fmt.Errorf("gittree snapshot: %w", err)
	}
	if _, err := w.git(env, "add", "-A", "--", "."); err != nil {
		return "", fmt.Errorf("gittree snapshot: %w", err)
	}
	tree, err := w.gitLine(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gittree snapshot: %w", err)
	}
	if !treeID.MatchString(tree) {
		return "", fmt.Errorf("gittree snapshot: write-tree returned %q", tree)
	}
	return tree, nil
}

// Diff produces the exact binary patch between two trees, renames
// represented as delete+add so changed-path sets stay literal. Every
// driver a config could inject is disabled and the a/ b/ prefixes are
// forced explicitly: the same two trees yield the same patch bytes in
// every repository.
func (w Workspace) Diff(fromTree, toTree string) ([]byte, error) {
	patch, err := w.git(nil, "diff", "--binary", "--no-renames", "--unified=3",
		"--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none",
		"--src-prefix=a/", "--dst-prefix=b/", fromTree, toTree, "--")
	if err != nil {
		return nil, fmt.Errorf("gittree diff: %w", err)
	}
	return patch, nil
}

// ChangedPaths returns the paths that differ between two trees, sorted by
// git, renames as delete+add endpoints.
func (w Workspace) ChangedPaths(fromTree, toTree string) ([]string, error) {
	raw, err := w.git(nil, "diff", "--name-only", "-z", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=none", fromTree, toTree, "--")
	if err != nil {
		return nil, fmt.Errorf("gittree changed paths: %w", err)
	}
	var paths []string
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) > 0 {
			paths = append(paths, string(p))
		}
	}
	return paths, nil
}

// Apply applies a patch EXACTLY to a base tree in an isolated index and
// returns the resulting tree id: no three-way merge, no rejects, no fuzz,
// no worktree involvement. A patch that does not apply byte-exactly is an
// error, and refusal makes no worktree, real-index, ref, or shippable-tree
// transition — the isolated index is discarded either way. (A refused or
// partial apply may leave unreachable loose objects in the object
// database; those are invisible to every tree and reclaimed by gc.)
func (w Workspace) Apply(baseTree string, patch []byte) (string, error) {
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if _, err := w.git(env, "read-tree", baseTree); err != nil {
		return "", fmt.Errorf("gittree apply: %w", err)
	}
	full := append(append([]string{"-C", w.Dir}, configPins...),
		"apply", "--cached", "--binary", "--whitespace=nowarn", "-")
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = bytes.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	limit := boundedexec.Timeout(filepath.Join(w.Dir, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git apply --cached"); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("gittree apply: patch does not apply exactly: %s", detail)
	}
	tree, err := w.gitLine(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gittree apply: %w", err)
	}
	return tree, nil
}

// Entry is one tree entry: git mode and object id — the two facts the
// wall's per-entry equality check compares (r5: after application every
// changed entry must carry the SAME object id and git mode as the reviewed
// tree).
type Entry struct {
	Mode string
	OID  string
}

// Entries returns mode and object id for each requested path in a tree.
// Paths absent from the tree (deletions) are simply missing from the map —
// absence on both sides is equality.
func (w Workspace) Entries(tree string, paths []string) (map[string]Entry, error) {
	if len(paths) == 0 {
		return map[string]Entry{}, nil
	}
	// --literal-pathspecs: the requested paths are filenames, never
	// patterns — a file named "a[1]" must resolve to itself, not to a
	// glob that silently misses (slice-1 critique F2).
	args := append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--"}, paths...)
	raw, err := w.git(nil, args...)
	if err != nil {
		return nil, fmt.Errorf("gittree entries: %w", err)
	}
	entries := map[string]Entry{}
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		// <mode> SP <type> SP <oid> TAB <path>
		head, path, ok := strings.Cut(string(record), "\t")
		if !ok {
			return nil, fmt.Errorf("gittree entries: unparseable ls-tree record %q", record)
		}
		fields := strings.Fields(head)
		if len(fields) != 3 {
			return nil, fmt.Errorf("gittree entries: unparseable ls-tree record %q", record)
		}
		entries[path] = Entry{Mode: fields[0], OID: fields[2]}
	}
	return entries, nil
}

// missionID is the production mission grammar (the job-id grammar): no
// slashes, so one mission's anchors can never nest inside another's
// namespace and DropAnchors cannot cross a mission boundary.
var missionID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// anchorRef is the pinned namespace from the wall's named contracts.
func anchorRef(mission, tree string) string {
	return "refs/metasystem/missions/" + mission + "/" + tree
}

// Anchor keeps a snapshot tree reachable across git garbage collection by
// pointing a runner-owned ref at it. Idempotent. The object must BE a tree
// under its full unabbreviated id — a commit, blob, or abbreviation that
// happens to resolve is refused, because the anchor's name doubles as the
// tree's identity in the wall's records.
func (w Workspace) Anchor(mission, tree string) error {
	if !missionID.MatchString(mission) {
		return fmt.Errorf("gittree anchor: %q is not a mission id", mission)
	}
	if !treeID.MatchString(tree) {
		return fmt.Errorf("gittree anchor: %q is not a tree id", tree)
	}
	objectType, err := w.gitLine(nil, "cat-file", "-t", tree)
	if err != nil {
		return fmt.Errorf("gittree anchor: %w", err)
	}
	if objectType != "tree" {
		return fmt.Errorf("gittree anchor: %s is a %s, not a tree", tree, objectType)
	}
	resolved, err := w.gitLine(nil, "rev-parse", "--verify", tree)
	if err != nil || resolved != tree {
		return fmt.Errorf("gittree anchor: %q is not a full unabbreviated tree id", tree)
	}
	if _, err := w.git(nil, "update-ref", anchorRef(mission, tree), tree); err != nil {
		return fmt.Errorf("gittree anchor: %w", err)
	}
	return nil
}

// DropAnchors deletes every anchor ref the mission owns — called once at
// mission close, never per turn.
func (w Workspace) DropAnchors(mission string) error {
	if !missionID.MatchString(mission) {
		return fmt.Errorf("gittree drop anchors: %q is not a mission id", mission)
	}
	prefix := "refs/metasystem/missions/" + mission + "/"
	raw, err := w.git(nil, "for-each-ref", "--format=%(refname)", prefix)
	if err != nil {
		return fmt.Errorf("gittree drop anchors: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		if _, err := w.git(nil, "update-ref", "-d", ref); err != nil {
			return fmt.Errorf("gittree drop anchors: %w", err)
		}
	}
	return nil
}
