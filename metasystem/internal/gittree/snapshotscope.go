package gittree

// The snapshot-scope primitives: a repository carries bytes three ways —
// worktree, index, committed history — and, in a nested checkout, a whole
// repository sits around the workspace. These primitives give the wall
// one typed probe per carrier: committed HEAD and the ref map, the staged
// projections at both scopes, the pseudoref and worktree censuses, the
// comparison-target-seeded worktree projection, and the runner-owned
// commit anchor. Every probe distinguishes "the repository answered X"
// (HEAD unborn and ref absent included) from "the command could not run",
// so the wall's violation/error split never string-matches.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// RunFailure marks a git invocation that could not run at all — spawn
// refusal, git absent, timeout. Callers treat it as the runner's own
// environment failure; every other outcome is the repository answering.
type RunFailure struct {
	Op  string
	Err error
}

func (f *RunFailure) Error() string { return fmt.Sprintf("git %s could not run: %v", f.Op, f.Err) }
func (f *RunFailure) Unwrap() error { return f.Err }

// gitProbe runs one bounded git invocation and reports its exit code:
// err is non-nil ONLY when the command could not run (a RunFailure); a
// nonzero exit is an answer for the caller to type.
func (w Workspace) gitProbe(dir string, env []string, stdin []byte, args ...string) (stdout, stderr string, code int, err error) {
	full := append(append([]string{"-C", dir}, configPins...), args...)
	cmd := exec.Command("git", full...)
	cmd.Env = ScrubbedEnviron(env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	limit := boundedexec.Timeout(filepath.Join(w.Dir, "metasystem.conf"), boundedexec.Local)
	runErr := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " "))
	switch typed := runErr.(type) {
	case nil:
		return outBuf.String(), errBuf.String(), 0, nil
	case *exec.ExitError:
		return outBuf.String(), errBuf.String(), typed.ExitCode(), nil
	default:
		return outBuf.String(), errBuf.String(), -1, &RunFailure{Op: args[0], Err: runErr}
	}
}

// answerErr words a probe outcome that ran but did not answer the
// expected shape — a repository-state answer, never a RunFailure.
func answerErr(op, stderr, stdout string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	return fmt.Errorf("git %s: %s", op, detail)
}

// HeadCommit resolves the commit HEAD names. unborn=true is a
// ran-and-answered outcome: the repository exists and HEAD names no
// commit yet. A RunFailure means the probe itself could not run.
func (w Workspace) HeadCommit() (oid string, unborn bool, err error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return "", false, err
	}
	if code == 0 {
		oid = strings.TrimSpace(stdout)
		if !treeID.MatchString(oid) {
			return "", false, fmt.Errorf("gittree head: rev-parse returned %q", oid)
		}
		return oid, false, nil
	}
	// HEAD did not resolve; only a repository that ANSWERS proves the
	// unborn state rather than a broken probe.
	_, dirStderr, dirCode, err := w.gitProbe(w.Dir, nil, nil, "rev-parse", "--git-dir")
	if err != nil {
		return "", false, err
	}
	if dirCode == 0 {
		return "", true, nil
	}
	return "", false, answerErr("rev-parse HEAD", stderr+dirStderr, stdout)
}

// SymbolicHead names the branch HEAD is attached to; detached=true is a
// ran-and-answered outcome.
func (w Workspace) SymbolicHead() (ref string, detached bool, err error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return "", false, err
	}
	if code == 0 {
		return strings.TrimSpace(stdout), false, nil
	}
	if code == 1 {
		return "", true, nil
	}
	return "", false, answerErr("symbolic-ref HEAD", stderr, stdout)
}

// RefMap enumerates EVERY ref tip in the repository, no namespace filter:
// ref-retained bytes are accountable in every namespace — heads, tags,
// remotes, custom hierarchies. Ref names cannot contain newlines, so the
// NUL field / newline record framing is exact.
func (w Workspace) RefMap() (map[string]string, error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "for-each-ref", "--format=%(objectname)%00%(refname)")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("for-each-ref", stderr, stdout)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue
		}
		oid, name, ok := strings.Cut(line, "\x00")
		if !ok || !treeID.MatchString(oid) || name == "" {
			return nil, fmt.Errorf("gittree refmap: unparseable record %q", line)
		}
		refs[name] = oid
	}
	return refs, nil
}

// ErrUnmergedWorkspaceIndex is the ran-and-answered refusal for a
// conflicted WORKSPACE index entry: a conflicted entry has no tree, and
// the wall fails toward itself rather than guessing one.
var ErrUnmergedWorkspaceIndex = errors.New("the workspace index carries unmerged entries")

// stagedEntry is one logical index entry: mode, object id, stage number,
// and path — exactly the facts ls-files --stage serializes, never the
// index file's stat cache, which churns without any staged byte changing.
type stagedEntry struct {
	mode, oid, path string
	stage           int
}

func parseStagedEntries(raw string) ([]stagedEntry, error) {
	entries := []stagedEntry{}
	for _, record := range strings.Split(raw, "\x00") {
		if record == "" {
			continue
		}
		head, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(head)
		if !ok || len(fields) != 3 || path == "" {
			return nil, fmt.Errorf("gittree staged: unparseable ls-files record %q", record)
		}
		stage := -1
		if _, err := fmt.Sscanf(fields[2], "%d", &stage); err != nil || stage < 0 || stage > 3 {
			return nil, fmt.Errorf("gittree staged: unparseable stage in %q", record)
		}
		if !treeID.MatchString(fields[1]) {
			return nil, fmt.Errorf("gittree staged: unparseable object id in %q", record)
		}
		entries = append(entries, stagedEntry{mode: fields[0], oid: fields[1], path: path, stage: stage})
	}
	return entries, nil
}

// indexInfoLine renders one entry in update-index --index-info form.
func (e stagedEntry) indexInfoLine() string {
	return fmt.Sprintf("%s %s %d\t%s", e.mode, e.oid, e.stage, e.path)
}

// writeTreeOf loads logical entries into a fresh isolated index and
// write-trees them. The real index is never opened for writing.
func (w Workspace) writeTreeOf(entries []stagedEntry) (string, error) {
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if _, err := w.git(env, "read-tree", "--empty"); err != nil {
		return "", fmt.Errorf("gittree staged: %w", err)
	}
	if len(entries) > 0 {
		lines := make([]string, 0, len(entries))
		for _, entry := range entries {
			lines = append(lines, entry.indexInfoLine())
		}
		stdin := []byte(strings.Join(lines, "\x00") + "\x00")
		top, err := w.topLevel()
		if err != nil {
			return "", err
		}
		stdout, stderr, code, perr := w.gitProbe(top, env, stdin, "update-index", "-z", "--index-info")
		if perr != nil {
			return "", perr
		}
		if code != 0 {
			return "", answerErr("update-index --index-info", stderr, stdout)
		}
	}
	tree, err := w.gitLine(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gittree staged: %w", err)
	}
	if !treeID.MatchString(tree) {
		return "", fmt.Errorf("gittree staged: write-tree returned %q", tree)
	}
	return tree, nil
}

// stagedEntriesAt reads the logical staged entries visible from a
// directory: ls-files scopes to the invocation directory and prints
// paths relative to it, so the workspace call yields workspace-relative
// entries and a toplevel call yields the whole index.
func (w Workspace) stagedEntriesAt(dir string) ([]stagedEntry, error) {
	stdout, stderr, code, err := w.gitProbe(dir, nil, nil, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("ls-files --stage", stderr, stdout)
	}
	return parseStagedEntries(stdout)
}

// StagedTree is the workspace staged projection, computed by
// RECONSTRUCTION: the workspace-restricted ls-files --stage entries
// loaded into a fresh isolated index and write-tree'd there — the
// worktree and the real index untouched. A preexisting sibling conflict
// in a nested checkout never enters (the entries are read from the
// workspace, so sibling entries — conflicted or not — are outside the
// read); a conflicted WORKSPACE entry refuses with
// ErrUnmergedWorkspaceIndex. The returned tree's paths are
// workspace-relative, the package's one path space.
func (w Workspace) StagedTree() (string, error) {
	entries, err := w.stagedEntriesAt(w.Dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.stage != 0 {
			return "", fmt.Errorf("%w: %s", ErrUnmergedWorkspaceIndex, entry.path)
		}
	}
	return w.writeTreeOf(entries)
}

// StagedPosture is the logical staged state of one index at repository
// scope: the stage-0 entries as a content-addressed tree, and every
// unmerged entry serialized beside it — so preexisting sibling conflicts
// are representable and refuse nothing, and motion is judged by
// comparing postures, never index-file bytes.
type StagedPosture struct {
	Tree     string
	Unmerged []string
}

// Equal reports whether two postures describe the same logical staged
// state.
func (p StagedPosture) Equal(other StagedPosture) bool {
	if p.Tree != other.Tree || len(p.Unmerged) != len(other.Unmerged) {
		return false
	}
	for i := range p.Unmerged {
		if p.Unmerged[i] != other.Unmerged[i] {
			return false
		}
	}
	return true
}

// stagedPostureAt builds the posture of the index visible from dir.
func (w Workspace) stagedPostureAt(dir string) (StagedPosture, error) {
	entries, err := w.stagedEntriesAt(dir)
	if err != nil {
		return StagedPosture{}, err
	}
	merged := []stagedEntry{}
	unmerged := []string{}
	for _, entry := range entries {
		if entry.stage == 0 {
			merged = append(merged, entry)
			continue
		}
		unmerged = append(unmerged, entry.indexInfoLine())
	}
	sort.Strings(unmerged)
	tree, err := w.writeTreeOf(merged)
	if err != nil {
		return StagedPosture{}, err
	}
	return StagedPosture{Tree: tree, Unmerged: unmerged}, nil
}

// TopStagedPosture is the toplevel real-index posture — the origin and
// judge of staged sibling motion in a nested checkout.
func (w Workspace) TopStagedPosture() (StagedPosture, error) {
	top, err := w.topLevel()
	if err != nil {
		return StagedPosture{}, err
	}
	return w.stagedPostureAt(top)
}

// SnapshotSeeded projects the current worktree against a COMPARISON
// TARGET: the isolated index seeds from the resolved seed commit's
// toplevel tree with the workspace prefix subtree replaced by
// expectedTree, so tracked-and-ignored membership follows the
// comparison's own right-hand side — a patch-added ignored-but-tracked
// path projects whether or not HEAD has reached the full composition.
// Declared host-artifact paths are content-free and may be ignored files
// the expected tree cannot yet name, so their membership is an explicit
// forced input. seedCommit is a RESOLVED commit id (never the symbolic
// name HEAD — resolution at exec time would open a gap between judging
// and projecting); empty means an unborn HEAD and seeds empty.
func (w Workspace) SnapshotSeeded(seedCommit, expectedTree string, declaredPaths []string) (string, error) {
	if !treeID.MatchString(expectedTree) {
		return "", fmt.Errorf("gittree seeded snapshot: %q is not a tree id", expectedTree)
	}
	prefix, err := w.treePrefix()
	if err != nil {
		return "", err
	}
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if prefix == "" {
		// At the toplevel the expected tree IS the whole seed.
		if _, err := w.git(env, "read-tree", expectedTree); err != nil {
			return "", fmt.Errorf("gittree seeded snapshot: %w", err)
		}
	} else {
		if seedCommit == "" {
			if _, err := w.git(env, "read-tree", "--empty"); err != nil {
				return "", fmt.Errorf("gittree seeded snapshot: %w", err)
			}
		} else {
			if !treeID.MatchString(seedCommit) {
				return "", fmt.Errorf("gittree seeded snapshot: %q is not a commit id", seedCommit)
			}
			if _, err := w.git(env, "read-tree", seedCommit+"^{tree}"); err != nil {
				return "", fmt.Errorf("gittree seeded snapshot: %w", err)
			}
		}
		// The graft: drop the seed's own workspace subtree, then read the
		// expected tree in under the prefix.
		existing, err := w.gitTop(env, "ls-files", "-z", "--", prefix)
		if err != nil {
			return "", fmt.Errorf("gittree seeded snapshot: %w", err)
		}
		if len(bytes.TrimRight(existing, "\x00")) > 0 {
			top, err := w.topLevel()
			if err != nil {
				return "", err
			}
			stdout, stderr, code, perr := w.gitProbe(top, env, existing, "update-index", "-z", "--force-remove", "--stdin")
			if perr != nil {
				return "", perr
			}
			if code != 0 {
				return "", answerErr("update-index --force-remove", stderr, stdout)
			}
		}
		if _, err := w.gitTop(env, "read-tree", "--prefix="+prefix, expectedTree); err != nil {
			return "", fmt.Errorf("gittree seeded snapshot: %w", err)
		}
	}
	// add runs FROM THE WORKSPACE on purpose, exactly like Snapshot: its
	// pathspec scopes the capture to the workspace's own files.
	if _, err := w.git(env, "add", "-A", "--", "."); err != nil {
		return "", fmt.Errorf("gittree seeded snapshot: %w", err)
	}
	// Declared paths that exist in the worktree join by force — an
	// ignored declared artifact the expected tree cannot yet name must
	// not vanish from the projection and falsely read as drift. Absent
	// ones are simply no delta.
	present := []string{}
	for _, declared := range declaredPaths {
		if _, err := os.Lstat(filepath.Join(w.Dir, filepath.FromSlash(declared))); err == nil {
			present = append(present, declared)
		}
	}
	if len(present) > 0 {
		args := append([]string{"add", "-f", "--"}, present...)
		if _, err := w.git(env, args...); err != nil {
			return "", fmt.Errorf("gittree seeded snapshot: %w", err)
		}
	}
	tree, err := w.gitLine(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gittree seeded snapshot: %w", err)
	}
	if !treeID.MatchString(tree) {
		return "", fmt.Errorf("gittree seeded snapshot: write-tree returned %q", tree)
	}
	return w.subtreeOf(tree)
}

// anchorName is the grammar for named commit anchors — no slashes, no
// hex-only names, so a commit anchor can never collide with a tree
// anchor's id-named ref in the same mission namespace.
var anchorName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// AnchorCommit keeps a commit reachable under a runner-owned NAMED ref
// (refs/metasystem/missions/<mission>/<name>), compare-and-swap updated
// so two racing writers cannot silently drop one another. Sibling of
// Anchor; DropAnchors removes it at mission close with the tree anchors.
func (w Workspace) AnchorCommit(mission, name, commit string) error {
	if !missionID.MatchString(mission) {
		return fmt.Errorf("gittree anchor commit: %q is not a mission id", mission)
	}
	if !anchorName.MatchString(name) || treeID.MatchString(name) {
		return fmt.Errorf("gittree anchor commit: %q is not an anchor name", name)
	}
	if !treeID.MatchString(commit) {
		return fmt.Errorf("gittree anchor commit: %q is not a full commit id", commit)
	}
	objectType, err := w.gitLine(nil, "cat-file", "-t", commit)
	if err != nil {
		return fmt.Errorf("gittree anchor commit: %w", err)
	}
	if objectType != "commit" {
		return fmt.Errorf("gittree anchor commit: %s is a %s, not a commit", commit, objectType)
	}
	ref := anchorRef(mission, name)
	stdout, stderr, code, perr := w.gitProbe(w.Dir, nil, nil, "rev-parse", "--verify", "--quiet", ref)
	if perr != nil {
		return perr
	}
	old := ""
	if code == 0 {
		old = strings.TrimSpace(stdout)
	} else if strings.TrimSpace(stderr) != "" {
		return answerErr("rev-parse "+ref, stderr, stdout)
	}
	if _, err := w.git(nil, "update-ref", ref, commit, old); err != nil {
		return fmt.Errorf("gittree anchor commit: %w", err)
	}
	return nil
}

// HistorySteeringFiles names the repository-local files that rewrite
// what every ancestry walk reports — a grafts file forges first-parent
// chains even with replacement disabled, and a shallow boundary
// truncates them. The wall admits neither: their presence is judged at
// admission and at every capture.
func (w Workspace) HistorySteeringFiles() ([]string, error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("rev-parse --git-common-dir", stderr, stdout)
	}
	commonDir := strings.TrimSuffix(stdout, "\n")
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(w.Dir, commonDir)
	}
	present := []string{}
	for _, name := range []string{filepath.Join("info", "grafts"), "shallow"} {
		if _, statErr := os.Stat(filepath.Join(commonDir, name)); statErr == nil {
			present = append(present, name)
		}
	}
	return present, nil
}

// Pseudoref is one root-level pseudoref file and every object id it
// carries. Parseable=false records a file whose non-empty content
// yielded no object id — an answer for the wall to judge, not an error.
type Pseudoref struct {
	Name      string
	OIDs      []string
	Parseable bool
}

// pseudorefName admits the whole all-caps *_HEAD family plus AUTO_MERGE —
// membership BY CLASS, not by list, so a new git pseudoref joins the
// census without a code change. HEAD itself is not a member: it is the
// resolved head, judged separately.
var pseudorefName = regexp.MustCompile(`^[A-Z][A-Z_]*_HEAD$`)

var oidToken = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// PseudorefCensusAt enumerates the pseudoref files directly under the
// git directory that serves dir — a linked worktree resolves its own
// private git directory, so its ORIG_HEAD is distinct from the main
// checkout's.
func (w Workspace) PseudorefCensusAt(dir string) ([]Pseudoref, error) {
	stdout, stderr, code, err := w.gitProbe(dir, nil, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("rev-parse --absolute-git-dir", stderr, stdout)
	}
	gitDir := strings.TrimSuffix(stdout, "\n")
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		return nil, fmt.Errorf("gittree pseudoref census: %w", err)
	}
	census := []Pseudoref{}
	for _, entry := range entries {
		name := entry.Name()
		if !pseudorefName.MatchString(name) && name != "AUTO_MERGE" {
			continue
		}
		if !entry.Type().IsRegular() {
			census = append(census, Pseudoref{Name: name, OIDs: []string{}, Parseable: false})
			continue
		}
		data, err := os.ReadFile(filepath.Join(gitDir, name))
		if err != nil {
			return nil, fmt.Errorf("gittree pseudoref census: %w", err)
		}
		ref := Pseudoref{Name: name, OIDs: []string{}, Parseable: true}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			// Multi-OID formats (FETCH_HEAD, MERGE_HEAD) put the id
			// first on every line; a line that opens with anything else
			// is content the census cannot vouch for.
			token := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })
			if len(token) > 0 && oidToken.MatchString(token[0]) {
				ref.OIDs = append(ref.OIDs, token[0])
			} else {
				ref.Parseable = false
			}
		}
		census = append(census, ref)
	}
	sort.Slice(census, func(i, j int) bool { return census[i].Name < census[j].Name })
	return census, nil
}

// PseudorefCensus is the census of the workspace's own checkout.
func (w Workspace) PseudorefCensus() ([]Pseudoref, error) {
	return w.PseudorefCensusAt(w.Dir)
}

// WorktreeRecord is one worktree's carrier posture: its identity, HEAD,
// its private pseudoref census, and its logical staged posture — a
// detached worktree is otherwise a complete private carrier that no
// main-checkout observable ever sees.
type WorktreeRecord struct {
	Path            string
	HeadOID         string
	Branch          string
	Detached        bool
	Bare            bool
	Prunable        bool
	PostureReadable bool
	Pseudorefs      []Pseudoref
	Staged          StagedPosture
}

// WorktreeCensus enumerates every worktree of the repository with its
// posture. A worktree whose directory is gone (prunable) keeps its list
// entry with PostureReadable=false — absence of a posture is itself an
// answer.
func (w Workspace) WorktreeCensus() ([]WorktreeRecord, error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("worktree list", stderr, stdout)
	}
	records := []WorktreeRecord{}
	var current *WorktreeRecord
	flush := func() {
		if current != nil {
			records = append(records, *current)
			current = nil
		}
	}
	for _, token := range strings.Split(stdout, "\x00") {
		switch {
		case token == "":
			flush()
		case strings.HasPrefix(token, "worktree "):
			flush()
			current = &WorktreeRecord{Path: strings.TrimPrefix(token, "worktree ")}
		case current == nil:
			return nil, fmt.Errorf("gittree worktree census: attribute %q before any worktree", token)
		case strings.HasPrefix(token, "HEAD "):
			current.HeadOID = strings.TrimPrefix(token, "HEAD ")
		case strings.HasPrefix(token, "branch "):
			current.Branch = strings.TrimPrefix(token, "branch ")
		case token == "detached":
			current.Detached = true
		case token == "bare":
			current.Bare = true
		case strings.HasPrefix(token, "prunable"):
			current.Prunable = true
		}
	}
	flush()
	for i := range records {
		record := &records[i]
		if record.Bare || record.Prunable {
			continue
		}
		if info, err := os.Stat(record.Path); err != nil || !info.IsDir() {
			continue
		}
		pseudorefs, err := w.PseudorefCensusAt(record.Path)
		if err != nil {
			return nil, err
		}
		staged, err := w.stagedPostureAt(record.Path)
		if err != nil {
			return nil, err
		}
		record.PostureReadable = true
		record.Pseudorefs = pseudorefs
		record.Staged = staged
	}
	return records, nil
}
