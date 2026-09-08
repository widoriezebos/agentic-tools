package gittree

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DisjointMergeProofKind is the stable name of the only merge proof this
// package produces. Callers persist it with the manifest and reject every
// other value.
const DisjointMergeProofKind = "disjoint-hunks-v1"

// IsRecertifiableText is the ONE definition of text admitted by the
// recertification merge. It is deliberately independent of filenames, Git
// attributes, MIME guesses, and Git's own binary heuristic.
func IsRecertifiableText(blob []byte) bool {
	if !utf8.Valid(blob) {
		return false
	}
	for len(blob) > 0 {
		r, width := utf8.DecodeRune(blob)
		blob = blob[width:]
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		if r >= 0x7f && r <= 0x9f {
			return false
		}
	}
	return true
}

// HunkRange is one half of a canonical Git hunk header. Starts are Git's
// one-based coordinates; a zero-count old range denotes the boundary Start.
type HunkRange struct {
	Start int `json:"start"`
	Count int `json:"count"`
}

// HunkManifestEntry is the content-independent portion of one canonical
// hunk, ordered by path, side, then its old/new coordinates.
type HunkManifestEntry struct {
	Path     string    `json:"path"`
	Side     string    `json:"side"`
	OldRange HunkRange `json:"oldRange"`
	NewRange HunkRange `json:"newRange"`
}

// DisjointMergeResult is the deterministic three-tree result and the facts
// needed to reproduce it later.
type DisjointMergeResult struct {
	MergedTree string              `json:"mergedTree"`
	GitVersion string              `json:"gitVersion"`
	Manifest   []HunkManifestEntry `json:"hunkManifest"`
}

// DisjointMergeError is a typed proof refusal. Kind is overlap or unproven;
// Detail is the stable structured detail consumed by validation and landing.
type DisjointMergeError struct {
	Kind       string
	Detail     string
	Path       string
	ChainRange HunkRange
	MainRange  HunkRange
	Err        error
}

func (e *DisjointMergeError) Error() string {
	switch e.Kind {
	case "overlap":
		return fmt.Sprintf("disjoint merge: %s: chain old range %d,%d overlaps main old range %d,%d",
			e.Path, e.ChainRange.Start, e.ChainRange.Count, e.MainRange.Start, e.MainRange.Count)
	default:
		if e.Path != "" {
			return fmt.Sprintf("disjoint merge: %s: %s: %v", e.Path, e.Detail, e.Err)
		}
		return fmt.Sprintf("disjoint merge: %s: %v", e.Detail, e.Err)
	}
}

func (e *DisjointMergeError) Unwrap() error { return e.Err }

type parsedEdit struct {
	oldRange    HunkRange
	newRange    HunkRange
	replacement []byte
	oldPayload  []byte
}

var canonicalHunkHeader = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?\n$`)

func parseCount(raw []byte, omittedDefault int) (int, error) {
	if len(raw) == 0 {
		return omittedDefault, nil
	}
	value, err := strconv.ParseUint(string(raw), 10, 31)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func splitDisplayLines(raw []byte) ([][]byte, error) {
	var lines [][]byte
	for len(raw) > 0 {
		at := bytes.IndexByte(raw, '\n')
		if at < 0 {
			return nil, fmt.Errorf("diff output has an unterminated line")
		}
		lines = append(lines, append([]byte(nil), raw[:at+1]...))
		raw = raw[at+1:]
	}
	return lines, nil
}

func blobLines(blob []byte) [][]byte {
	if len(blob) == 0 {
		return nil
	}
	var lines [][]byte
	for len(blob) > 0 {
		at := bytes.IndexByte(blob, '\n')
		if at < 0 {
			lines = append(lines, append([]byte(nil), blob...))
			break
		}
		lines = append(lines, append([]byte(nil), blob[:at+1]...))
		blob = blob[at+1:]
	}
	return lines
}

func flattenLines(lines [][]byte) []byte {
	return bytes.Join(lines, nil)
}

// canonicalBlobDiff invokes Git outside every worktree and with every
// hunk-shaping input fixed. Exit one is the normal "different" result of
// --no-index; every other nonzero result is a proof failure.
func (w Workspace) canonicalBlobDiff(oldBlob, newBlob []byte) ([]byte, string, error) {
	dir, err := os.MkdirTemp("", "metasystem-disjoint-diff.")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)
	env := []string{
		"LC_ALL=C", "LANG=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull,
	}
	// TMPDIR is operator-controlled. Refuse rather than let a temporary
	// directory beneath any repository discover repository-local config or
	// attributes and thereby change the canonical hunk proof.
	_, _, repositoryCode, runErr := w.gitProbe(dir, env, nil, "rev-parse", "--git-dir")
	if runErr != nil {
		return nil, "", runErr
	}
	if repositoryCode == 0 {
		return nil, "", fmt.Errorf("canonical diff temporary directory is inside a Git repository")
	}
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	if err := os.WriteFile(oldPath, oldBlob, 0o600); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(newPath, newBlob, 0o600); err != nil {
		return nil, "", err
	}
	stdout, stderr, code, runErr := w.gitProbe(dir, env, nil,
		"diff", "--no-index", "--text", "--no-renames", "--diff-algorithm=myers", "--minimal",
		"--no-indent-heuristic", "--unified=0", "--inter-hunk-context=0",
		"--no-ext-diff", "--no-textconv", "--no-color",
		"--src-prefix=a/", "--dst-prefix=b/", "--", "old", "new")
	if runErr != nil {
		return nil, "", runErr
	}
	if code != 0 && code != 1 {
		return nil, "", answerErr("diff --no-index", stderr, stdout)
	}
	versionOut, versionErr, versionCode, runErr := w.gitProbe(dir, env, nil, "--version")
	if runErr != nil {
		return nil, "", runErr
	}
	if versionCode != 0 {
		return nil, "", answerErr("--version", versionErr, versionOut)
	}
	return []byte(stdout), strings.TrimSpace(versionOut), nil
}

func parseCanonicalEdits(diff, oldBlob, newBlob []byte) ([]parsedEdit, error) {
	lines, err := splitDisplayLines(diff)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		if bytes.Equal(oldBlob, newBlob) {
			return nil, nil
		}
		return nil, fmt.Errorf("different blobs produced no diff output")
	}
	if len(lines) < 5 || !bytes.Equal(lines[0], []byte("diff --git a/old b/new\n")) ||
		!bytes.HasPrefix(lines[1], []byte("index ")) || !bytes.HasSuffix(lines[1], []byte(" 100644\n")) ||
		!bytes.Equal(lines[2], []byte("--- a/old\n")) || !bytes.Equal(lines[3], []byte("+++ b/new\n")) {
		return nil, fmt.Errorf("malformed canonical diff preamble")
	}
	oldUnits := blobLines(oldBlob)
	newUnits := blobLines(newBlob)
	var edits []parsedEdit
	for index := 4; index < len(lines); {
		if !bytes.HasPrefix(lines[index], []byte("@@ ")) {
			return nil, fmt.Errorf("malformed canonical diff tail %q", lines[index])
		}
		matches := canonicalHunkHeader.FindSubmatch(lines[index])
		if matches == nil {
			return nil, fmt.Errorf("malformed hunk header %q", lines[index])
		}
		oldStart, err := parseCount(matches[1], 0)
		if err != nil {
			return nil, fmt.Errorf("malformed old start: %w", err)
		}
		oldCount, err := parseCount(matches[2], 1)
		if err != nil {
			return nil, fmt.Errorf("malformed old count: %w", err)
		}
		newStart, err := parseCount(matches[3], 0)
		if err != nil {
			return nil, fmt.Errorf("malformed new start: %w", err)
		}
		newCount, err := parseCount(matches[4], 1)
		if err != nil {
			return nil, fmt.Errorf("malformed new count: %w", err)
		}
		index++
		var oldPayload, newPayload [][]byte
		oldSeen, newSeen := 0, 0
		markerEligible := false
		markerOld, markerNew := false, false
		markerUsed := false
		markedOld, markedNew := false, false
		for index < len(lines) && !bytes.HasPrefix(lines[index], []byte("@@ ")) {
			line := lines[index]
			if bytes.Equal(line, []byte("\\ No newline at end of file\n")) {
				if !markerEligible || markerUsed {
					return nil, fmt.Errorf("misplaced or repeated no-final-newline marker")
				}
				if markerOld {
					last := len(oldPayload) - 1
					if last < 0 || !bytes.HasSuffix(oldPayload[last], []byte("\n")) {
						return nil, fmt.Errorf("no-final-newline marker has no old payload")
					}
					oldPayload[last] = oldPayload[last][:len(oldPayload[last])-1]
					markedOld = true
				}
				if markerNew {
					last := len(newPayload) - 1
					if last < 0 || !bytes.HasSuffix(newPayload[last], []byte("\n")) {
						return nil, fmt.Errorf("no-final-newline marker has no new payload")
					}
					newPayload[last] = newPayload[last][:len(newPayload[last])-1]
					markedNew = true
				}
				markerUsed = true
				markerEligible = false
				index++
				continue
			}
			if len(line) == 0 || line[0] == '\\' {
				return nil, fmt.Errorf("unknown diff marker %q", line)
			}
			payload := append([]byte(nil), line[1:]...)
			markerOld, markerNew = false, false
			switch line[0] {
			case '-':
				oldPayload = append(oldPayload, payload)
				oldSeen++
				markerOld = true
			case '+':
				newPayload = append(newPayload, payload)
				newSeen++
				markerNew = true
			case ' ':
				oldPayload = append(oldPayload, append([]byte(nil), payload...))
				newPayload = append(newPayload, payload)
				oldSeen++
				newSeen++
				markerOld, markerNew = true, true
			default:
				return nil, fmt.Errorf("malformed hunk payload %q", line)
			}
			markerEligible = true
			markerUsed = false
			index++
			if oldSeen == oldCount && newSeen == newCount {
				// A following marker still belongs to the last payload; every
				// other line begins the next hunk or is malformed tail data.
				if index >= len(lines) || !bytes.Equal(lines[index], []byte("\\ No newline at end of file\n")) {
					break
				}
			}
		}
		if oldSeen != oldCount || newSeen != newCount {
			return nil, fmt.Errorf("hunk counts are %d/%d, header requires %d/%d", oldSeen, newSeen, oldCount, newCount)
		}
		oldAt := oldStart
		if oldCount > 0 {
			oldAt--
		}
		newAt := newStart
		if newCount > 0 {
			newAt--
		}
		if oldAt < 0 || oldAt+oldCount > len(oldUnits) || newAt < 0 || newAt+newCount > len(newUnits) {
			return nil, fmt.Errorf("hunk range falls outside a blob")
		}
		if !bytes.Equal(flattenLines(oldPayload), flattenLines(oldUnits[oldAt:oldAt+oldCount])) {
			return nil, fmt.Errorf("old hunk payload does not reconstruct its blob")
		}
		if !bytes.Equal(flattenLines(newPayload), flattenLines(newUnits[newAt:newAt+newCount])) {
			return nil, fmt.Errorf("new hunk payload does not reconstruct its blob")
		}
		if markedOld || markedNew {
			if markedOld && oldAt+oldCount != len(oldUnits) {
				return nil, fmt.Errorf("old no-final-newline marker is not on the final blob line")
			}
			if markedNew && newAt+newCount != len(newUnits) {
				return nil, fmt.Errorf("new no-final-newline marker is not on the final blob line")
			}
		}
		edits = append(edits, parsedEdit{
			oldRange:   HunkRange{Start: oldStart, Count: oldCount},
			newRange:   HunkRange{Start: newStart, Count: newCount},
			oldPayload: flattenLines(oldPayload), replacement: flattenLines(newPayload),
		})
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("different blobs produced no hunks")
	}
	if reconstructed, err := applyParsedEdits(oldBlob, edits); err != nil || !bytes.Equal(reconstructed, newBlob) {
		if err == nil {
			err = fmt.Errorf("reconstructed bytes differ")
		}
		return nil, fmt.Errorf("hunks do not reconstruct the new blob: %w", err)
	}
	return edits, nil
}

func editBoundary(edit parsedEdit) int {
	if edit.oldRange.Count == 0 {
		return edit.oldRange.Start
	}
	return edit.oldRange.Start - 1
}

func applyParsedEdits(base []byte, edits []parsedEdit) ([]byte, error) {
	units := blobLines(base)
	ordered := append([]parsedEdit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := editBoundary(ordered[i]), editBoundary(ordered[j])
		if left != right {
			return left < right
		}
		return ordered[i].oldRange.Count < ordered[j].oldRange.Count
	})
	var out bytes.Buffer
	cursor := 0
	for _, edit := range ordered {
		at := editBoundary(edit)
		if at < cursor || at > len(units) || at+edit.oldRange.Count > len(units) {
			return nil, fmt.Errorf("edits are out of order or overlap")
		}
		out.Write(flattenLines(units[cursor:at]))
		if !bytes.Equal(flattenLines(units[at:at+edit.oldRange.Count]), edit.oldPayload) {
			return nil, fmt.Errorf("edit old payload differs from the base")
		}
		out.Write(edit.replacement)
		cursor = at + edit.oldRange.Count
	}
	out.Write(flattenLines(units[cursor:]))
	return out.Bytes(), nil
}

func footprint(r HunkRange) (int, int) {
	if r.Count == 0 {
		return r.Start, r.Start
	}
	return r.Start - 1, r.Start - 1 + r.Count
}

func rangesOverlap(left, right HunkRange) bool {
	leftStart, leftEnd := footprint(left)
	rightStart, rightEnd := footprint(right)
	return leftStart <= rightEnd && rightStart <= leftEnd
}

func (w Workspace) blobEdits(path, side string, base, changed []byte) ([]parsedEdit, []HunkManifestEntry, string, error) {
	diff, version, err := w.canonicalBlobDiff(base, changed)
	if err != nil {
		return nil, nil, "", err
	}
	edits, err := parseCanonicalEdits(diff, base, changed)
	if err != nil {
		return nil, nil, "", err
	}
	manifest := make([]HunkManifestEntry, 0, len(edits))
	for _, edit := range edits {
		manifest = append(manifest, HunkManifestEntry{Path: path, Side: side, OldRange: edit.oldRange, NewRange: edit.newRange})
	}
	return edits, manifest, version, nil
}

func (w Workspace) hashBlob(blob []byte) (string, error) {
	top, err := w.topLevel()
	if err != nil {
		return "", err
	}
	stdout, stderr, code, err := w.gitProbe(top, nil, blob, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", answerErr("hash-object", stderr, stdout)
	}
	oid := strings.TrimSpace(stdout)
	if !treeID.MatchString(oid) {
		return "", fmt.Errorf("hash-object returned %q", oid)
	}
	return oid, nil
}

func (w Workspace) treeWithChanges(seed string, changes map[string]*Entry) (string, error) {
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if _, err := w.git(env, "read-tree", seed); err != nil {
		return "", err
	}
	top, err := w.topLevel()
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry := changes[path]
		if entry == nil {
			if _, err := w.gitTop(env, "update-index", "--force-remove", "--", path); err != nil {
				return "", err
			}
			continue
		}
		line := []byte(fmt.Sprintf("%s %s\t%s\x00", entry.Mode, entry.OID, path))
		stdout, stderr, code, err := w.gitProbe(top, env, line, "update-index", "-z", "--add", "--index-info")
		if err != nil {
			return "", err
		}
		if code != 0 {
			return "", answerErr("update-index --index-info", stderr, stdout)
		}
	}
	return w.gitLine(env, "write-tree")
}

// DisjointMerge constructs the exact merge of the chain tree and target
// tree relative to base. It never consults the worktree or real index.
func (w Workspace) DisjointMerge(baseTree, chainTree, targetTree string) (DisjointMergeResult, error) {
	chainPaths, err := w.ChangedPaths(baseTree, chainTree)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
	}
	mainPaths, err := w.ChangedPaths(baseTree, targetTree)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
	}
	chainSet, mainSet := map[string]bool{}, map[string]bool{}
	union := map[string]bool{}
	for _, path := range chainPaths {
		chainSet[path], union[path] = true, true
	}
	for _, path := range mainPaths {
		mainSet[path], union[path] = true, true
	}
	paths := make([]string, 0, len(union))
	for path := range union {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	baseEntries, err := w.Entries(baseTree, paths)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
	}
	chainEntries, err := w.Entries(chainTree, paths)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
	}
	targetEntries, err := w.Entries(targetTree, paths)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
	}
	changes := map[string]*Entry{}
	var manifest []HunkManifestEntry
	gitVersion := ""
	for _, path := range paths {
		if !chainSet[path] {
			continue
		}
		chainEntry, chainPresent := chainEntries[path]
		if !mainSet[path] {
			if chainPresent {
				copyEntry := chainEntry
				changes[path] = &copyEntry
			} else {
				changes[path] = nil
			}
			continue
		}
		baseEntry, basePresent := baseEntries[path]
		targetEntry, targetPresent := targetEntries[path]
		regular := func(mode string) bool { return mode == "100644" || mode == "100755" }
		if !basePresent || !chainPresent || !targetPresent || !regular(baseEntry.Mode) ||
			baseEntry.Mode != chainEntry.Mode || baseEntry.Mode != targetEntry.Mode {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "unsupported-mode", Path: path, Err: fmt.Errorf("both-side entry shape or mode differs")}
		}
		baseBlob, _, err := w.FileAt(baseTree, path)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Path: path, Err: err}
		}
		chainBlob, _, err := w.FileAt(chainTree, path)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Path: path, Err: err}
		}
		mainBlob, _, err := w.FileAt(targetTree, path)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Path: path, Err: err}
		}
		if !IsRecertifiableText(baseBlob) || !IsRecertifiableText(chainBlob) || !IsRecertifiableText(mainBlob) {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "non-text-blob", Path: path, Err: fmt.Errorf("a blob fails IsRecertifiableText")}
		}
		chainEdits, chainManifest, version, err := w.blobEdits(path, "chain", baseBlob, chainBlob)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "malformed-hunks", Path: path, Err: err}
		}
		if gitVersion == "" {
			gitVersion = version
		}
		mainEdits, mainManifest, version, err := w.blobEdits(path, "main", baseBlob, mainBlob)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "malformed-hunks", Path: path, Err: err}
		}
		if version != gitVersion {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-version-moved", Path: path, Err: fmt.Errorf("git version changed during proof")}
		}
		for _, chainEdit := range chainEdits {
			for _, mainEdit := range mainEdits {
				if rangesOverlap(chainEdit.oldRange, mainEdit.oldRange) {
					return DisjointMergeResult{}, &DisjointMergeError{Kind: "overlap", Detail: "overlap", Path: path,
						ChainRange: chainEdit.oldRange, MainRange: mainEdit.oldRange}
				}
			}
		}
		mergedBlob, err := applyParsedEdits(baseBlob, append(append([]parsedEdit(nil), chainEdits...), mainEdits...))
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "merge-construction", Path: path, Err: err}
		}
		oid, err := w.hashBlob(mergedBlob)
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Path: path, Err: err}
		}
		changes[path] = &Entry{Mode: baseEntry.Mode, OID: oid}
		manifest = append(manifest, chainManifest...)
		manifest = append(manifest, mainManifest...)
	}
	if gitVersion == "" {
		version, err := w.gitLine([]string{"LC_ALL=C", "LANG=C"}, "--version")
		if err != nil {
			return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "git-operation", Err: err}
		}
		gitVersion = version
	}
	mergedTree, err := w.treeWithChanges(targetTree, changes)
	if err != nil {
		return DisjointMergeResult{}, &DisjointMergeError{Kind: "unproven", Detail: "tree-construction", Err: err}
	}
	sort.Slice(manifest, func(i, j int) bool {
		left, right := manifest[i], manifest[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Side != right.Side {
			return left.Side < right.Side
		}
		if left.OldRange != right.OldRange {
			if left.OldRange.Start != right.OldRange.Start {
				return left.OldRange.Start < right.OldRange.Start
			}
			return left.OldRange.Count < right.OldRange.Count
		}
		if left.NewRange.Start != right.NewRange.Start {
			return left.NewRange.Start < right.NewRange.Start
		}
		return left.NewRange.Count < right.NewRange.Count
	})
	return DisjointMergeResult{MergedTree: mergedTree, GitVersion: gitVersion, Manifest: manifest}, nil
}

// ManifestDigest is the stable SHA-256 of the manifest's compact canonical
// JSON representation. Record-level canonicalization is owned by validate;
// this digest binds only the Git operation's ordered manifest.
func ManifestDigest(manifest []HunkManifestEntry) string {
	var buffer bytes.Buffer
	for _, item := range manifest {
		fmt.Fprintf(&buffer, "%d:%s\n%s\n%d,%d\n%d,%d\n", len(item.Path), item.Path, item.Side,
			item.OldRange.Start, item.OldRange.Count, item.NewRange.Start, item.NewRange.Count)
	}
	sum := sha256.Sum256(buffer.Bytes())
	return hex.EncodeToString(sum[:])
}

// MergeBases returns every full merge-base commit in Git's deterministic
// output order. Recertification requires the caller to accept exactly one.
func (w Workspace) MergeBases(left, right string) ([]string, error) {
	stdout, stderr, code, err := w.gitProbe(w.Dir, nil, nil, "merge-base", "--all", left, right)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, answerErr("merge-base --all", stderr, stdout)
	}
	var bases []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		if !treeID.MatchString(line) {
			return nil, fmt.Errorf("merge-base returned %q", line)
		}
		bases = append(bases, line)
	}
	return bases, nil
}

// ResolveCommit verifies and returns one full commit object id.
func (w Workspace) ResolveCommit(rev string) (string, error) {
	oid, err := w.gitLine(nil, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil || !treeID.MatchString(oid) {
		return "", fmt.Errorf("gittree commit %q is unreadable", rev)
	}
	return oid, nil
}

// ResolveRef returns the full object id currently named by ref.
func (w Workspace) ResolveRef(ref string) (string, error) {
	oid, err := w.gitLine(nil, "rev-parse", "--verify", ref)
	if err != nil || !treeID.MatchString(oid) {
		return "", fmt.Errorf("gittree ref %q is unreadable", ref)
	}
	return oid, nil
}

// ResolveTree verifies that rev directly names one full tree object id. It is
// used for retention anchors whose ref name alone must never make a commit or
// blob look like a recoverable materialization tree.
func (w Workspace) ResolveTree(rev string) (string, error) {
	oid, err := w.ResolveRef(rev)
	if err != nil || !treeID.MatchString(oid) {
		return "", fmt.Errorf("gittree tree %q is unreadable", rev)
	}
	objectType, err := w.gitLine(nil, "cat-file", "-t", oid)
	if err != nil || objectType != "tree" {
		return "", fmt.Errorf("gittree tree %q does not name a tree object", rev)
	}
	return oid, nil
}

// GitPath resolves one name inside this worktree's actual Git directory.
// Linked worktrees do not necessarily store these sentinels under .git/.
func (w Workspace) GitPath(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "..") || strings.ContainsAny(name, "\x00\n\r") {
		return "", fmt.Errorf("gittree git-path name is malformed")
	}
	path, err := w.gitPathLine(nil, "rev-parse", "--git-path", name)
	if err != nil || path == "" {
		return "", fmt.Errorf("gittree git-path %q is unreadable", name)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.Dir, path)
	}
	return filepath.Clean(path), nil
}

// AnchorRef points one caller-owned exact ref at an existing full object id.
// It never accepts symbolic names or abbreviations.
func (w Workspace) AnchorRef(ref, oid, objectType string) error {
	if !strings.HasPrefix(ref, "refs/metasystem/landing/recertifications/") || strings.Contains(ref, "..") ||
		strings.ContainsAny(ref, "\x00\n\r") || !treeID.MatchString(oid) {
		return fmt.Errorf("gittree recertification anchor is malformed")
	}
	actualType, err := w.gitLine(nil, "cat-file", "-t", oid)
	if err != nil || actualType != objectType {
		return fmt.Errorf("gittree recertification anchor object %s is not a %s", oid, objectType)
	}
	if _, err := w.git(nil, "update-ref", ref, oid); err != nil {
		return fmt.Errorf("gittree recertification anchor: %w", err)
	}
	return nil
}

// GraftProjectTree replaces this workspace's prefix in seedCommit's whole
// repository tree with projectTree. At a repository toplevel projectTree is
// already the whole result.
func (w Workspace) GraftProjectTree(seedCommit, projectTree string) (string, error) {
	if !treeID.MatchString(projectTree) {
		return "", fmt.Errorf("gittree graft: malformed project tree")
	}
	prefix, err := w.treePrefix()
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return projectTree, nil
	}
	if _, err := w.ResolveCommit(seedCommit); err != nil {
		return "", err
	}
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if _, err := w.git(env, "read-tree", seedCommit+"^{tree}"); err != nil {
		return "", fmt.Errorf("gittree graft: %w", err)
	}
	existing, err := w.gitTop(env, "ls-files", "-z", "--", prefix)
	if err != nil {
		return "", fmt.Errorf("gittree graft: %w", err)
	}
	if len(bytes.TrimRight(existing, "\x00")) > 0 {
		top, err := w.topLevel()
		if err != nil {
			return "", err
		}
		stdout, stderr, code, err := w.gitProbe(top, env, existing, "update-index", "-z", "--force-remove", "--stdin")
		if err != nil {
			return "", err
		}
		if code != 0 {
			return "", answerErr("update-index --force-remove", stderr, stdout)
		}
	}
	if _, err := w.gitTop(env, "read-tree", "--prefix="+prefix, projectTree); err != nil {
		return "", fmt.Errorf("gittree graft: %w", err)
	}
	whole, err := w.gitLine(env, "write-tree")
	if err != nil || !treeID.MatchString(whole) {
		return "", fmt.Errorf("gittree graft: cannot write result tree")
	}
	return whole, nil
}
