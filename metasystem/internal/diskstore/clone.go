package diskstore

// A person's release of an unregistered clone (design engine-owns-disk-
// lifetimes Part B, 3.10 and 3.12; R22; U6d-2). A clone beside the checkout
// that shares its root commit is copied whole into the checkout's common
// store before it is removed: every ref, a detached HEAD, every stash
// entry, every linked worktree's HEAD, every worktree's index (wrapped in a
// commit so its staged blobs stay reachable) and every initialized
// submodule's refs and HEAD, recursively. The rows are persisted in a
// mapping before the first fetch, fetched atomically under refs/archive/,
// read back and verified in the common store, and only then is the clone
// removed. A dirty clone or worktree is kept until a person discards; an
// unmerged index is kept with its stages listed. A repeat resumes from the
// mapping and never makes a second archive name.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// The clone mapping's states, in order.
const (
	CloneMapped   = "mapped"
	CloneVerified = "verified"
	CloneRemoved  = "removed"
)

// CloneRow is one thing a clone holds and where its archive lives.
type CloneRow struct {
	// Kind is ref, head, stash, worktree-head, index, submodule-ref,
	// submodule-head, not-initialized or unmerged.
	Kind    string `json:"kind"`
	Repo    string `json:"repo,omitempty"`
	Source  string `json:"source"`
	SHA     string `json:"sha,omitempty"`
	Archive string `json:"archive,omitempty"`
	Note    string `json:"note,omitempty"`
}

// CloneMapping is the persisted inventory a release works from.
type CloneMapping struct {
	Schema    string     `json:"schema"`
	Path      string     `json:"path"`
	Base      string     `json:"base"`
	State     string     `json:"state"`
	Worktrees []string   `json:"worktrees"`
	Rows      []CloneRow `json:"rows"`
	// Blockers are what the archive cannot carry (uncommitted work,
	// ignored content over the allowance, hooks, configuration, excludes,
	// LFS objects, gitlinks it cannot archive, missing worktrees): each
	// keeps the clone until a person discards (Round B3-2 R4).
	Blockers []string `json:"blockers,omitempty"`
	// Discard is the person's discard that released the clone anyway.
	Discard *Discard  `json:"discard,omitempty"`
	Created time.Time `json:"created"`
}

// CloneMappingPath is where a clone's mapping lives in the checkout's
// registry.
func CloneMappingPath(registry Registry, clone string) string {
	sum := sha256.Sum256([]byte(clone))
	return filepath.Join(registry.Dir, "clones", hex.EncodeToString(sum[:8])+".json")
}

// CloneReleaseRequest releases one clone.
type CloneReleaseRequest struct {
	Registry Registry
	// GitRoot is the checkout whose common store receives the archive.
	GitRoot string
	Path    string
	Git     WorkspaceGit
	Census  *UseCensus
	Discard *Discard
	Now     time.Time
	// Protected are the host's armed checkouts and the landing lane's
	// checkout, never released as a clone.
	Protected []string
	// IgnoredReleaseBytes is disk.workspace-ignored-release-mib in bytes.
	IgnoredReleaseBytes int64
}

// CloneRelease is a clone release's outcome.
type CloneRelease struct {
	Done    bool   `json:"done"`
	Already bool   `json:"already,omitempty"`
	Kept    bool   `json:"kept,omitempty"`
	Pending bool   `json:"pending,omitempty"`
	Reason  string `json:"reason"`
	Command string `json:"command,omitempty"`
	Base    string `json:"base,omitempty"`
	Rows    int    `json:"rows,omitempty"`
}

// ReleaseClone archives a clone whole into the checkout's common store,
// verifies the archive, and removes the clone and its linked worktrees.
func ReleaseClone(ctx context.Context, request CloneReleaseRequest) (CloneRelease, error) {
	path := filepath.Clean(request.Path)
	mappingPath := CloneMappingPath(request.Registry, path)
	mapping, mapped, err := readCloneMapping(mappingPath)
	if err != nil {
		return CloneRelease{}, err
	}
	if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
		if mapped && mapping.State != CloneRemoved {
			mapping.State = CloneRemoved
			if err := writeCloneMapping(mappingPath, mapping); err != nil {
				return CloneRelease{}, err
			}
		}
		return CloneRelease{Done: true, Already: true, Base: mapping.Base, Rows: len(mapping.Rows), Reason: "nothing is at " + path}, nil
	}
	if err := checkCloneCandidate(ctx, request, path); err != nil {
		return CloneRelease{}, err
	}
	// The inventory is taken afresh every time: the clone may have gained
	// work since an earlier, interrupted or kept release; the earlier
	// mapping only lends its index commits, so an unchanged index keeps
	// its archive name.
	base := cloneArchiveBase(path)
	if mapped {
		base = mapping.Base
	}
	mapping, err = InventoryClone(ctx, request.Git, path, base, request.Now, mapping)
	if err != nil {
		return CloneRelease{}, err
	}
	mapping.Blockers = append(mapping.Blockers, cloneBlockers(ctx, request.Git, path, mapping, request.IgnoredReleaseBytes)...)
	if request.Discard != nil {
		mapping.Discard = request.Discard
	}
	if err := writeCloneMapping(mappingPath, mapping); err != nil {
		return CloneRelease{}, err
	}
	outcome := CloneRelease{Base: mapping.Base, Rows: len(mapping.Rows)}
	if len(mapping.Blockers) > 0 && request.Discard == nil {
		outcome.Kept, outcome.Reason = true, "the archive cannot carry all the clone holds: "+firstPaths(mapping.Blockers)
		outcome.Command = "settle each, or a person's metasystem work workspace --release --path " + path + " --discard --reason TEXT"
		return outcome, nil
	}
	if verdict := cloneUse(request.Census, append([]string{path}, mapping.Worktrees...)); verdict.Decision != Release {
		outcome.Pending, outcome.Reason, outcome.Command = true, verdict.Reason, verdict.Command
		return outcome, nil
	}
	if err := archiveCloneRows(ctx, request, &mapping, mappingPath); err != nil {
		return CloneRelease{Pending: true, Base: mapping.Base, Reason: "the archive is not complete (" + err.Error() + "); nothing was removed",
			Command: "metasystem work workspace --release --path " + path}, nil
	}
	mapping.State = CloneVerified
	if err := writeCloneMapping(mappingPath, mapping); err != nil {
		return CloneRelease{}, err
	}
	for _, worktree := range mapping.Worktrees {
		if worktree == path || strings.HasPrefix(worktree, path+string(filepath.Separator)) {
			continue
		}
		if _, err := os.Lstat(worktree); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if gitdir := gitdirOf(worktree); !strings.HasPrefix(gitdir, path+string(filepath.Separator)) {
			return CloneRelease{Pending: true, Base: mapping.Base, Reason: worktree + " is no longer a worktree of the clone; a person looks at it",
				Command: "metasystem disk show"}, nil
		}
		if err := RemoveTree(ctx, worktree); err != nil {
			return CloneRelease{Pending: true, Base: mapping.Base, Reason: "removal cut short (" + err.Error() + "); the repeat finishes it",
				Command: "metasystem work workspace --release --path " + path}, nil
		}
	}
	if err := RemoveTree(ctx, path); err != nil {
		return CloneRelease{Pending: true, Base: mapping.Base, Reason: "removal cut short (" + err.Error() + "); the repeat finishes it",
			Command: "metasystem work workspace --release --path " + path}, nil
	}
	mapping.State = CloneRemoved
	if err := writeCloneMapping(mappingPath, mapping); err != nil {
		return CloneRelease{}, err
	}
	outcome.Done, outcome.Reason = true, "archived and removed"
	return outcome, nil
}

func cloneArchiveBase(path string) string {
	sum := sha256.Sum256([]byte(path))
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, filepath.Base(path))
	return "refs/archive/clone-" + name + "-" + hex.EncodeToString(sum[:4])
}

// checkCloneCandidate refuses anything but the main worktree of a
// repository beside the checkout that shares its root commit and no store
// record holds.
func checkCloneCandidate(ctx context.Context, request CloneReleaseRequest, path string) error {
	checkout := filepath.Clean(request.GitRoot)
	if !filepath.IsAbs(path) || path == checkout || strings.HasPrefix(checkout, path+string(filepath.Separator)) || strings.HasPrefix(path, checkout+string(filepath.Separator)) {
		return fmt.Errorf("%s is this checkout, holds it, or lies inside it; only a clone beside it is released this way", path)
	}
	if filepath.Dir(path) != filepath.Dir(checkout) {
		return fmt.Errorf("%s is not beside the checkout %s; only a clone in the same directory is released this way", path, checkout)
	}
	for _, protected := range request.Protected {
		if filepath.Clean(protected) == path {
			return fmt.Errorf("%s is an armed checkout of this host or the landing lane's checkout; it is no clone to release", path)
		}
	}
	common, err := request.Git(ctx, checkout, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("the checkout's common store cannot be read: %w", err)
	}
	if inside(filepath.Clean(strings.TrimSpace(string(common))), path) {
		return fmt.Errorf("%s holds this checkout's own object store; it is no clone to release", path)
	}
	listing, err := request.Git(ctx, checkout, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("the checkout's worktrees cannot be read: %w", err)
	}
	for _, line := range strings.Split(string(listing), "\n") {
		if worktree, ok := strings.CutPrefix(line, "worktree "); ok && inside(filepath.Clean(worktree), path) {
			return fmt.Errorf("%s holds %s, a worktree of this checkout; it is no clone to release", path, worktree)
		}
	}
	if repo := alternatesInto(path, filepath.Dir(path)); repo != "" {
		return fmt.Errorf("%s reads objects from %s through its alternates; removing it would break that repository", repo, path)
	}
	top, err := request.Git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(strings.TrimSpace(string(top))) != path {
		return fmt.Errorf("%s is not the top of a git clone", path)
	}
	gitdir, err := request.Git(ctx, path, "rev-parse", "--absolute-git-dir")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(gitdir)), path+string(filepath.Separator)) {
		return fmt.Errorf("%s is a linked worktree, not a clone; its own repository releases it", path)
	}
	roots := func(dir string) map[string]bool {
		out, _ := request.Git(ctx, dir, "rev-list", "--max-parents=0", "--all")
		set := map[string]bool{}
		for _, sha := range strings.Fields(string(out)) {
			set[sha] = true
		}
		return set
	}
	ours, shared := roots(checkout), false
	for sha := range roots(path) {
		shared = shared || ours[sha]
	}
	if !shared {
		return fmt.Errorf("%s shares no root commit with %s; it is not a clone of this project", path, checkout)
	}
	records, _ := request.Registry.Inventory()
	for _, record := range records {
		if record.Path == path && record.State != StateReleased {
			return fmt.Errorf("%s is registered store %s; it is released by its own owner", path, record.ID)
		}
	}
	return nil
}

// InventoryClone lists every row of a clone and its dirty worktrees. It
// writes into the clone only the index commits that make each worktree's
// staged content reachable.
func InventoryClone(ctx context.Context, git WorkspaceGit, path, base string, now time.Time, earlier CloneMapping) (CloneMapping, error) {
	mapping := CloneMapping{Schema: Schema, Path: path, Base: base, State: CloneMapped, Created: now.UTC(), Rows: []CloneRow{}}
	gitdirOut, err := git(ctx, path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return mapping, err
	}
	gitdir := strings.TrimSpace(string(gitdirOut))
	refs, err := repoRefs(ctx, git, path)
	if err != nil {
		return mapping, err
	}
	for _, ref := range refs {
		mapping.Rows = append(mapping.Rows, CloneRow{Kind: "ref", Repo: gitdir, Source: ref[1], SHA: ref[0], Archive: base + "/" + ref[1]})
	}
	if out, err := git(ctx, path, "log", "-g", "--format=%H", "refs/stash"); err == nil {
		for index, sha := range strings.Fields(string(out)) {
			mapping.Rows = append(mapping.Rows, CloneRow{Kind: "stash", Repo: gitdir, Source: fmt.Sprintf("stash@{%d}", index), SHA: sha, Archive: fmt.Sprintf("%s/stash/%d", base, index)})
		}
	}
	listing, err := git(ctx, path, "worktree", "list", "--porcelain")
	if err != nil {
		return mapping, err
	}
	for index, entry := range strings.Split(strings.TrimSpace(string(listing)), "\n\n") {
		var worktree, head string
		for _, line := range strings.Split(entry, "\n") {
			if value, ok := strings.CutPrefix(line, "worktree "); ok {
				worktree = filepath.Clean(value)
			}
			if value, ok := strings.CutPrefix(line, "HEAD "); ok {
				head = value
			}
		}
		if worktree == "" {
			continue
		}
		if _, err := os.Lstat(worktree); err != nil {
			mapping.Blockers = append(mapping.Blockers, worktree+": a worktree of the clone whose directory is gone")
			continue
		}
		mapping.Worktrees = append(mapping.Worktrees, worktree)
		prefix := fmt.Sprintf("%s/worktree/%d", base, index)
		if head != "" && !strings.HasPrefix(head, "0000000") {
			mapping.Rows = append(mapping.Rows, CloneRow{Kind: "worktree-head", Repo: gitdir, Source: worktree, SHA: head, Archive: prefix + "/HEAD"})
		}
		if err := inventoryWorktree(ctx, git, &mapping, worktree, head, gitdir, prefix, earlier); err != nil {
			return mapping, err
		}
	}
	if err := inventoryReflogs(ctx, git, &mapping, path, gitdir, refs); err != nil {
		return mapping, err
	}
	return mapping, nil
}

// inventoryReflogs adds a row for every reflog entry of every worktree's
// HEAD and every branch that no other row reaches (Round B3-2 R5).
func inventoryReflogs(ctx context.Context, git WorkspaceGit, mapping *CloneMapping, path, gitdir string, refs [][2]string) error {
	var candidates []string
	seen := map[string]bool{}
	for _, row := range mapping.Rows {
		seen[row.SHA] = true
	}
	add := func(dir, ref string) {
		out, err := git(ctx, dir, "log", "-g", "--format=%H", ref)
		if err != nil {
			return
		}
		for _, sha := range strings.Fields(string(out)) {
			if !seen[sha] {
				seen[sha] = true
				candidates = append(candidates, sha)
			}
		}
	}
	for _, worktree := range mapping.Worktrees {
		add(worktree, "HEAD")
	}
	for _, ref := range refs {
		if strings.HasPrefix(ref[1], "refs/heads/") {
			add(path, ref[1])
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	args := append([]string{"rev-list"}, candidates...)
	args = append(args, "--not")
	for _, row := range mapping.Rows {
		if row.SHA != "" && row.Repo == gitdir {
			args = append(args, row.SHA)
		}
	}
	out, err := git(ctx, path, args...)
	if err != nil {
		return err
	}
	unreached := map[string]bool{}
	for _, sha := range strings.Fields(string(out)) {
		unreached[sha] = true
	}
	for _, sha := range candidates {
		if unreached[sha] {
			mapping.Rows = append(mapping.Rows, CloneRow{Kind: "reflog", Repo: gitdir, Source: "reflog", SHA: sha, Archive: mapping.Base + "/reflog/" + shortSHA(sha)})
		}
	}
	return nil
}

// cloneBlockers are the repository-wide things the archive cannot carry:
// ignored content over the allowance, hooks, configuration beyond a
// clone's defaults, info/exclude lines, LFS objects and gitlinks without a
// git directory.
func cloneBlockers(ctx context.Context, git WorkspaceGit, path string, mapping CloneMapping, ignoredLimit int64) []string {
	var blockers []string
	gitdir := filepath.Join(path, ".git")
	var ignoredBytes int64
	var ignored []string
	for _, worktree := range mapping.Worktrees {
		status, err := git(ctx, worktree, "status", "--porcelain=v1", "--untracked-files=all", "--ignored")
		if err != nil {
			blockers = append(blockers, worktree+": its status cannot be read")
			continue
		}
		for _, line := range strings.Split(string(status), "\n") {
			if rest, ok := strings.CutPrefix(line, "!! "); ok {
				bytes, _, _ := Measure(ctx, filepath.Join(worktree, strings.TrimSuffix(rest, "/")))
				ignoredBytes += bytes
				ignored = append(ignored, filepath.Join(worktree, rest))
			}
		}
	}
	if ignoredBytes > ignoredLimit {
		blockers = append(blockers, fmt.Sprintf("%s of ignored files, over disk.workspace-ignored-release-mib: %s", formatBytes(ignoredBytes), firstPaths(ignored)))
	}
	if entries, err := os.ReadDir(filepath.Join(gitdir, "hooks")); err == nil {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sample") {
				blockers = append(blockers, "hook "+entry.Name())
			}
		}
	}
	if out, err := git(ctx, path, "config", "--file", filepath.Join(gitdir, "config"), "--list", "--name-only"); err != nil {
		blockers = append(blockers, "its configuration cannot be read")
	} else {
		for _, name := range strings.Fields(string(out)) {
			if !defaultCloneConfig(name) {
				blockers = append(blockers, "configuration "+name)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(gitdir, "info", "exclude")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				blockers = append(blockers, "info/exclude line "+line)
			}
		}
	}
	if bytes, _, _ := Measure(ctx, filepath.Join(gitdir, "lfs", "objects")); bytes > 0 {
		blockers = append(blockers, "LFS objects in .git/lfs")
	}
	if out, err := git(ctx, path, "ls-files", "-s"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "160000 ") {
				continue
			}
			_, sub, _ := strings.Cut(line, "\t")
			if _, err := os.Lstat(filepath.Join(path, sub, ".git")); err != nil {
				blockers = append(blockers, sub+": a gitlink without a git directory; its commit cannot be archived")
			}
		}
	}
	return blockers
}

// defaultCloneConfig reports a configuration key a plain clone writes, or
// a remote or branch (whose refs are archived).
func defaultCloneConfig(name string) bool {
	switch name {
	case "core.repositoryformatversion", "core.filemode", "core.bare", "core.logallrefupdates", "core.ignorecase", "core.precomposeunicode", "core.symlinks":
		return true
	}
	return strings.HasPrefix(name, "remote.") || strings.HasPrefix(name, "branch.") || strings.HasPrefix(name, "submodule.")
}

// inside reports whether path is dir or lies under it.
func inside(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

// alternatesInto names a repository beside the clone whose alternates read
// objects from inside it.
func alternatesInto(path, parent string) string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		repo := filepath.Join(parent, entry.Name())
		if repo == path {
			continue
		}
		for _, file := range []string{filepath.Join(repo, ".git", "objects", "info", "alternates"), filepath.Join(repo, "objects", "info", "alternates")} {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				if line = strings.TrimSpace(line); line != "" && inside(filepath.Clean(line), path) {
					return repo
				}
			}
		}
	}
	return ""
}

// repoRefs lists a repository's refs but the temporary archive refs, as
// (sha, refname) pairs.
func repoRefs(ctx context.Context, git WorkspaceGit, dir string) ([][2]string, error) {
	out, err := git(ctx, dir, "for-each-ref", "--format=%(objectname) %(refname)")
	if err != nil {
		return nil, err
	}
	var refs [][2]string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		sha, ref, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(ref, "refs/metasystem-archive/") || ref == "refs/stash" {
			continue
		}
		refs = append(refs, [2]string{sha, ref})
	}
	return refs, nil
}

// inventoryWorktree adds one worktree's dirt, index and submodules.
func inventoryWorktree(ctx context.Context, git WorkspaceGit, mapping *CloneMapping, worktree, head, gitdir, prefix string, earlier CloneMapping) error {
	status, err := git(ctx, worktree, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	unmerged := false
	var changes []string
	for _, line := range strings.Split(strings.TrimRight(string(status), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		if strings.Contains(code, "U") || code == "AA" || code == "DD" {
			unmerged = true
		}
		changes = append(changes, worktree+": "+strings.TrimSpace(line))
	}
	mapping.Blockers = append(mapping.Blockers, changes...)
	if unmerged {
		stages, _ := git(ctx, worktree, "ls-files", "-u")
		mapping.Blockers = append(mapping.Blockers, worktree+": an unmerged index; INDEX-STAGES.txt:\n"+string(stages))
	} else if treeOut, err := git(ctx, worktree, "write-tree"); err == nil {
		tree := strings.TrimSpace(string(treeOut))
		note := "tree " + tree + " parent " + head
		commit := ""
		for _, row := range earlier.Rows {
			if row.Kind == "index" && row.Source == worktree && row.Note == note {
				commit = row.SHA
			}
		}
		if commit == "" {
			args := []string{"-c", "user.name=metasystem", "-c", "user.email=metasystem@localhost", "commit-tree", tree}
			if head != "" && !strings.HasPrefix(head, "0000000") {
				args = append(args, "-p", head)
			}
			out, err := git(ctx, worktree, append(args, "-m", "index of "+worktree)...)
			if err != nil {
				return err
			}
			commit = strings.TrimSpace(string(out))
		}
		mapping.Rows = append(mapping.Rows, CloneRow{Kind: "index", Repo: gitdir, Source: worktree, SHA: commit, Archive: prefix + "/index", Note: note})
	}
	out, err := git(ctx, worktree, "submodule", "status", "--recursive")
	if err != nil {
		mapping.Blockers = append(mapping.Blockers, worktree+": its submodules cannot be read ("+strings.TrimSpace(err.Error())+")")
		return nil
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) < 2 {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) < 2 {
			continue
		}
		sub := fields[1]
		if line[0] == '-' {
			mapping.Blockers = append(mapping.Blockers, worktree+":"+sub+": a gitlink without a git directory; its commit "+fields[0]+" cannot be archived")
			continue
		}
		subPath := filepath.Join(worktree, sub)
		subGitdir, err := git(ctx, subPath, "rev-parse", "--absolute-git-dir")
		if err != nil {
			return err
		}
		subRepo := strings.TrimSpace(string(subGitdir))
		subPrefix := prefix + "/submodule/" + strings.ReplaceAll(sub, "/", "-")
		subRefs, err := repoRefs(ctx, git, subPath)
		if err != nil {
			return err
		}
		for _, ref := range subRefs {
			mapping.Rows = append(mapping.Rows, CloneRow{Kind: "submodule-ref", Repo: subRepo, Source: sub + ":" + ref[1], SHA: ref[0], Archive: subPrefix + "/" + ref[1]})
		}
		if subHead, err := git(ctx, subPath, "rev-parse", "HEAD"); err == nil {
			mapping.Rows = append(mapping.Rows, CloneRow{Kind: "submodule-head", Repo: subRepo, Source: sub + ":HEAD", SHA: strings.TrimSpace(string(subHead)), Archive: subPrefix + "/HEAD"})
		}
		subStatus, _ := git(ctx, subPath, "status", "--porcelain=v1", "--untracked-files=all")
		for _, change := range strings.Split(strings.TrimSpace(string(subStatus)), "\n") {
			if change != "" {
				mapping.Blockers = append(mapping.Blockers, subPath+": "+change)
			}
		}
	}
	return nil
}

// archiveCloneRows fetches every row into the common store under its
// archive name, never overwriting an archive ref that holds another commit
// (the row is renamed and the mapping rewritten first), then reads each
// back and checks its object is in the common store, which no alternates
// line may name the clone for.
func archiveCloneRows(ctx context.Context, request CloneReleaseRequest, mapping *CloneMapping, mappingPath string) error {
	common, err := request.Git(ctx, request.GitRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	if alternates, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(common)), "objects", "info", "alternates")); err == nil {
		for _, line := range strings.Split(string(alternates), "\n") {
			if line = strings.TrimSpace(line); line != "" && strings.HasPrefix(filepath.Clean(line), mapping.Path+string(filepath.Separator)) {
				return fmt.Errorf("the common store's alternates name %s inside the clone; its objects would read through the clone", line)
			}
		}
	}
	renamed := false
	byRepo := map[string][]int{}
	for index := range mapping.Rows {
		row := &mapping.Rows[index]
		if row.SHA == "" || row.Archive == "" {
			continue
		}
		if existing, found := revParseIn(ctx, request.Git, request.GitRoot, row.Archive); found {
			if existing == row.SHA {
				continue
			}
			for attempt := 2; ; attempt++ {
				name := fmt.Sprintf("%s@%s-%d", row.Archive, request.Now.UTC().Format("20060102T150405Z"), attempt)
				if _, taken := revParseIn(ctx, request.Git, request.GitRoot, name); !taken {
					row.Archive, renamed = name, true
					break
				}
			}
		}
		byRepo[row.Repo] = append(byRepo[row.Repo], index)
	}
	if renamed {
		if err := writeCloneMapping(mappingPath, *mapping); err != nil {
			return err
		}
	}
	for repo, indexes := range byRepo {
		var refspecs []string
		for _, index := range indexes {
			temporary := "refs/metasystem-archive/" + strconv.Itoa(index)
			if _, err := request.Git(ctx, repo, "update-ref", temporary, mapping.Rows[index].SHA); err != nil {
				return err
			}
			refspecs = append(refspecs, temporary+":"+mapping.Rows[index].Archive)
		}
		args := append([]string{"fetch", "--quiet", "--no-tags", "--atomic", repo}, refspecs...)
		if _, err := request.Git(ctx, request.GitRoot, args...); err != nil {
			return err
		}
	}
	for _, row := range mapping.Rows {
		if row.SHA == "" || row.Archive == "" {
			continue
		}
		if got, found := revParseIn(ctx, request.Git, request.GitRoot, row.Archive); !found || got != row.SHA {
			return fmt.Errorf("%s does not read back as %s", row.Archive, row.SHA)
		}
		if _, err := request.Git(ctx, request.GitRoot, "cat-file", "-e", row.SHA); err != nil {
			return fmt.Errorf("%s is not in the common store: %w", row.SHA, err)
		}
	}
	return nil
}

func cloneUse(census *UseCensus, paths []string) Verdict {
	command := "metasystem work workspace --release --path " + paths[0]
	switch {
	case census == nil || !census.Taken:
		return Verdict{Decision: Pending, Reason: "use census not taken", Command: command}
	case !census.Complete():
		return Verdict{Decision: Pending, Reason: "use census incomplete: " + joinLines(census.GapLines()), Command: command + ", once those processes are readable or ended"}
	}
	for _, path := range paths {
		if holders := census.Holders(path); len(holders) != 0 {
			holder := holders[0]
			return Verdict{Decision: Pending, Reason: fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command),
				Command: fmt.Sprintf("%s, once pid %d has ended", command, holder.Pid)}
		}
	}
	return Verdict{Decision: Release}
}

func readCloneMapping(path string) (CloneMapping, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CloneMapping{}, false, nil
	}
	if err != nil {
		return CloneMapping{}, false, err
	}
	var mapping CloneMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return CloneMapping{}, false, fmt.Errorf("clone mapping %s is unreadable: %w", path, err)
	}
	return mapping, true, nil
}

func writeCloneMapping(path string, mapping CloneMapping) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(path, append(data, '\n'), 0o600, filepath.Dir(filepath.Dir(path)))
	if err == nil && !durable {
		err = fmt.Errorf("clone mapping %s is published but its durability is unconfirmed", path)
	}
	return err
}
