package validate

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// These facts describe one three-tree merge. The two changed lines occupy
// separate hunks of the same text file, so verification must run the real
// merge algorithm, including its temporary-file canonical diffs.
const (
	rawClosureBaseTree   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rawClosureReviewTree = "cccccccccccccccccccccccccccccccccccccccc"
	rawClosureTargetTree = "dddddddddddddddddddddddddddddddddddddddd"
	rawClosureMergedTree = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	rawClosureTargetHead = "1111111111111111111111111111111111111111"
	rawClosureBaseBlob   = "top\nmiddle\nbottom\n"
	rawClosureChainBlob  = "chain\nmiddle\nbottom\n"
	rawClosureTargetBlob = "top\nmiddle\ntarget\n"
	rawClosureMergedBlob = "chain\nmiddle\ntarget\n"
)

type rawClosureFacts struct {
	t                  *testing.T
	f                  *recertificationClosureFixture
	record             RecertificationRecord
	path               string
	patch, mergedPatch []byte
	seq                string
	want               string
	indices            map[string]string
	activeIndex        string
	diffDir            string
	worktreeBytes      map[string][]byte
	snapshotIndex      string
	allowWorktreeDrift bool
}

func rawClosureOID(content string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
	return fmt.Sprintf("%x", h[:])
}

func rawClosurePatch(old, new, hunk string) []byte {
	return []byte(fmt.Sprintf("diff --git a/source.txt b/source.txt\nindex %s..%s 100644\n--- a/source.txt\n+++ b/source.txt\n%s",
		rawClosureOID(old)[:7], rawClosureOID(new)[:7], hunk))
}

func rawClosureNoIndex(old, new, hunk string) []byte {
	return []byte(fmt.Sprintf("diff --git a/old b/new\nindex %s..%s 100644\n--- a/old\n+++ b/new\n%s",
		rawClosureOID(old)[:7], rawClosureOID(new)[:7], hunk))
}

func newRawRecertificationClosure(t *testing.T) (*recertificationClosureFixture, string, *rawClosureFacts) {
	t.Helper()
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong-repository"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "wrong-index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.filemode")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	f := newFileRecertificationClosureFixture(t)
	r := &rawClosureFacts{t: t, f: f, indices: map[string]string{}}
	r.worktreeBytes = map[string][]byte{}
	for _, name := range []string{".gitignore", "source.txt", "docs/note.md", "metasystem.conf", "scripts/agents/path-classes.txt"} {
		data, err := os.ReadFile(filepath.Join(f.fixture.worktree, name))
		if err != nil {
			t.Fatal(err)
		}
		r.worktreeBytes[name] = data
	}
	r.worktreeBytes["source.txt"] = []byte(rawClosureMergedBlob)
	r.patch = rawClosurePatch(rawClosureBaseBlob, rawClosureChainBlob, "@@ -1 +1 @@\n-top\n+chain\n")
	r.mergedPatch = rawClosurePatch(rawClosureTargetBlob, rawClosureMergedBlob, "@@ -1 +1 @@\n-top\n+chain\n")
	f.subject.ReviewedProjectTree = rawClosureReviewTree
	f.subject.DiffDigest = sha256Bytes(r.patch)
	f.writeClosureSubject(f.subject)
	f.patch, f.reviewedTree = r.patch, rawClosureReviewTree
	writeRecertificationReview(t, f.fixture, 1, "implementation", rawClosureReviewTree, r.patch, nil)
	writeRecertificationReview(t, f.fixture, 2, "implementation-r2", rawClosureReviewTree, r.patch, nil)
	for _, round := range []int{1, 2} {
		job, critic := "implementation", "critic"
		if round == 2 {
			job, critic = "implementation-r2", "critic-r2"
		}
		f.fixture.writeJSON(fmt.Sprintf("artifacts/agents/implementation/rounds/%d/return.json", round), map[string]any{
			"jobId": job, "round": round, "diffBoundary": []string{"source.txt"},
		})
		f.fixture.writeJSON(fmt.Sprintf("artifacts/agents/critic/rounds/%d/return.json", round), map[string]any{
			"jobId": critic, "round": round, "reviewedTree": rawClosureReviewTree,
			"findings": []any{}, "verdictMaterialCount": 0,
		})
	}
	review := "artifacts/agents/implementation/rounds/2/review.json"
	patch := "artifacts/agents/implementation/rounds/2/diff.patch"
	critic := "artifacts/agents/critic/rounds/2/return.json"
	digest := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(f.fixture.controller, path))
		if err != nil {
			t.Fatal(err)
		}
		return sha256Bytes(data)
	}
	manifest := []gittree.HunkManifestEntry{
		{Path: "source.txt", Side: "chain", OldRange: gittree.HunkRange{Start: 1, Count: 1}, NewRange: gittree.HunkRange{Start: 1, Count: 1}},
		{Path: "source.txt", Side: "main", OldRange: gittree.HunkRange{Start: 3, Count: 1}, NewRange: gittree.HunkRange{Start: 3, Count: 1}},
	}
	r.record = RecertificationRecord{
		SchemaVersion: 1, ProofKind: gittree.DisjointMergeProofKind, GitVersion: "git version fixture",
		HunkManifest: manifest, HunkManifestDigest: gittree.ManifestDigest(manifest),
		RootChain: "implementation", CertifiedImplementerJob: "implementation-r2",
		ReviewArtifact: review, ReviewDigest: digest(review), PatchArtifact: patch, PatchDigest: digest(patch),
		CriticRoot: "critic", CriticTerminalRound: 2, CriticReturnArtifact: critic, CriticReturnDigest: digest(critic),
		SourceHead: f.fixture.baseSha, BaseCommit: f.fixture.baseSha, BaseTree: rawClosureBaseTree,
		ReviewedTree: rawClosureReviewTree, TargetCommit: rawClosureTargetHead, TargetTree: rawClosureTargetTree,
		MergedTree: rawClosureMergedTree, SourceWholeTree: rawClosureReviewTree, MergedWholeTree: rawClosureMergedTree,
		CertifiedPaths: []string{"source.txt"}, MergedPatchDigest: sha256Bytes(r.mergedPatch),
		GateWidth: "area", TestCommand: "true",
	}
	var err error
	r.record.InputDigest, err = RecertificationInputDigest(r.record)
	if err != nil {
		t.Fatal(err)
	}
	r.record.SourceAnchorRef = "refs/metasystem/landing/recertifications/" + r.record.InputDigest + "/source"
	r.record.MergedAnchorRef = "refs/metasystem/landing/recertifications/" + r.record.InputDigest + "/merged"
	r.path = "artifacts/agents/landing/recertifications/implementation/" + r.record.InputDigest + "/record.json"
	r.record.DiffArtifact = filepath.ToSlash(filepath.Join(filepath.Dir(r.path), "diff.patch"))
	r.record.RecordDigest, err = RecertificationRecordDigest(r.record)
	if err != nil {
		t.Fatal(err)
	}
	f.fixture.writeJSON(r.path, r.record)
	path := filepath.Join(f.fixture.controller, r.record.DiffArtifact)
	if err := os.WriteFile(path, r.mergedPatch, 0o644); err != nil {
		t.Fatal(err)
	}
	r.prepareSuccess()
	return f, r.path, r
}

func (r *rawClosureFacts) resetForRefusal() {
	r.t.Helper()
	r.seq = ""
	r.indices = map[string]string{}
	r.activeIndex = ""
	r.diffDir = ""
	r.snapshotIndex = ""
	r.prepareRefusal()
}

const rawClosureSuccessSequence = "TTPTTTTTTCDCBbPdPRTAWXXYEFGEHFIGJNnVNnVThrTUWTTrTAWZstmuPPPTTT"
const rawClosureRefusalSequence = "TTPTTTTTTCDCBbPdPRTAWXXYEFGEHFIGJNnVNnVThrTUWTTrTAWZstmuPPP"
const rawClosureCombinedSequence = rawClosureSuccessSequence + "LPqaWPPqaWP"
const rawClosureChangedSequence = rawClosureSuccessSequence + "LPqaWP"

func (r *rawClosureFacts) prepareSuccess() {
	r.want = rawClosureSuccessSequence
}

func (r *rawClosureFacts) prepareRefusal() {
	r.want = rawClosureRefusalSequence
}

func (r *rawClosureFacts) consumed() {
	r.t.Helper()
	if r.want != "" && r.seq != r.want {
		r.t.Fatalf("raw calls %q, want %q", r.seq, r.want)
	}
	if r.activeIndex != "" || r.diffDir != "" || r.snapshotIndex != "" {
		r.t.Fatalf("raw operations left index %q, diff directory %q, or snapshot index %q active", r.activeIndex, r.diffDir, r.snapshotIndex)
	}
}

// A seeded write-tree answer represents the entire worktree, not just the
// certified source path. Keep the fixture's finite file set and bytes bound
// to that answer so a sibling change cannot be hidden by a canned tree ID.
func (r *rawClosureFacts) worktreeDrift() error {
	seen := map[string]bool{}
	var changed []string
	err := filepath.Walk(r.f.fixture.worktree, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.f.fixture.worktree, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		want, declared := r.worktreeBytes[rel]
		if !declared || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			return fmt.Errorf("undeclared worktree entry %q with mode %s", rel, info.Mode())
		}
		got, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			changed = append(changed, rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	for name := range r.worktreeBytes {
		if !seen[name] {
			return fmt.Errorf("worktree entry %q is missing", name)
		}
	}
	if len(changed) != 0 {
		return fmt.Errorf("worktree bytes changed at %q", strings.Join(changed, ", "))
	}
	return nil
}

func (r *rawClosureFacts) answer(request gittree.RawRequest) gittree.RawResult {
	r.t.Helper()
	if request.Operation == "" || request.Timeout.Limit <= 0 {
		r.t.Fatalf("unbounded raw operation: %+v", request)
	}
	if request.Dir == "" || len(request.Args) < 3 || request.Args[0] != "-C" || request.Args[1] != request.Dir {
		r.t.Fatalf("raw directory mismatch: %+v", request)
	}
	args := request.Args[2:]
	pinned := len(args) >= len(rawPins) && reflect.DeepEqual(args[:len(rawPins)], rawPins)
	if pinned {
		args = args[len(rawPins):]
	}
	key := strings.Join(args, " ")
	if !pinned && key != "rev-parse --show-prefix" {
		r.t.Fatalf("unpinned raw operation %q", key)
	}
	for _, entry := range request.Env {
		name, _, _ := strings.Cut(entry, "=")
		if name == "GIT_DIR" || name == "GIT_WORK_TREE" || name == "GIT_COMMON_DIR" ||
			name == "GIT_OBJECT_DIRECTORY" || name == "GIT_ALTERNATE_OBJECT_DIRECTORIES" ||
			name == "GIT_NAMESPACE" || name == "GIT_REPLACE_REF_BASE" ||
			name == "GIT_CONFIG_COUNT" || name == "GIT_CONFIG_KEY_0" || name == "GIT_CONFIG_VALUE_0" {
			r.t.Fatalf("unscrubbed raw environment: %s", name)
		}
	}
	index := ""
	indexCount := 0
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			index = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
			indexCount++
		}
	}
	if indexCount > 1 || (index != "" && index == os.Getenv("GIT_INDEX_FILE")) {
		r.t.Fatalf("isolated index retains inherited value or is repeated: %q", index)
	}
	if strings.HasPrefix(filepath.Base(request.Dir), "metasystem-disjoint-diff.") {
		for _, pin := range []string{"LC_ALL=C", "LANG=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull} {
			if !containsLine(request.Env, pin) {
				r.t.Fatalf("canonical diff lacks environment pin %q", pin)
			}
		}
	}
	indexOperation := key == "read-tree "+rawClosureBaseTree || key == "read-tree "+rawClosureTargetTree ||
		key == "read-tree "+rawClosureMergedTree || key == "write-tree" ||
		key == "apply --cached --binary --whitespace=nowarn -" ||
		key == "update-index -z --add --index-info" || key == "add -A -- ."
	if index != "" && !indexOperation {
		r.t.Fatalf("unexpected isolated index on %q", key)
	}
	if index != "" {
		if !filepath.IsAbs(index) {
			r.t.Fatalf("non-absolute index: %q", index)
		}
		if key == "read-tree "+rawClosureBaseTree || key == "read-tree "+rawClosureTargetTree || key == "read-tree "+rawClosureMergedTree {
			if _, exists := r.indices[index]; exists {
				r.t.Fatalf("reused index %q", index)
			}
			r.indices[index] = strings.TrimPrefix(key, "read-tree ")
			r.activeIndex = index
		} else if _, exists := r.indices[index]; !exists {
			r.t.Fatalf("index %q used before read-tree for %q", index, key)
		} else if index != r.activeIndex {
			r.t.Fatalf("index changed within operation: %q != %q", index, r.activeIndex)
		}
	} else if indexOperation {
		r.t.Fatalf("isolated index missing for %q", key)
	}
	symbol := r.symbol(key)
	r.seq += symbol
	at := len(r.seq) - 1
	if r.want != "" {
		if at >= len(r.want) || r.want[at] != symbol[0] {
			r.t.Fatalf("raw call %d = %q (%s), want sequence %q", at+1, symbol, key, r.want)
		}
	}
	r.checkDirectory(symbol, at, request.Dir)
	result := r.fact(request, args, key, index)
	if key == "write-tree" {
		r.activeIndex = ""
	}
	return result
}

func (r *rawClosureFacts) checkDirectory(symbol string, at int, dir string) {
	r.t.Helper()
	if symbol == "N" {
		if r.diffDir != "" || !strings.HasPrefix(filepath.Base(dir), "metasystem-disjoint-diff.") ||
			dir == r.f.fixture.controller || dir == r.f.fixture.worktree {
			r.t.Fatalf("raw call %d: unexpected canonical diff directory %q", at+1, dir)
		}
		r.diffDir = dir
		return
	}
	if symbol == "n" || symbol == "V" {
		if r.diffDir == "" || dir != r.diffDir {
			r.t.Fatalf("raw call %d: canonical diff directory %q, want %q", at+1, dir, r.diffDir)
		}
		if symbol == "V" {
			r.diffDir = ""
		}
		return
	}
	want := r.f.fixture.controller
	if (r.want == rawClosureCombinedSequence || r.want == rawClosureChangedSequence) && at > len(rawClosureSuccessSequence) {
		want = r.f.fixture.worktree
	}
	if dir != want {
		r.t.Fatalf("raw call %d (%s): directory %q, want %q", at+1, symbol, dir, want)
	}
}

func (r *rawClosureFacts) symbol(key string) string {
	switch key {
	case "rev-parse --show-toplevel":
		return "T"
	case "rev-parse --show-prefix":
		return "P"
	case "rev-parse --verify " + r.f.fixture.baseSha + "^{commit}":
		return "C"
	case "rev-parse --verify " + rawClosureTargetHead + "^{commit}":
		return "D"
	case "merge-base --all " + r.f.fixture.baseSha + " " + rawClosureTargetHead:
		return "B"
	case "rev-parse " + r.f.fixture.baseSha + "^{tree}":
		return "b"
	case "rev-parse " + rawClosureTargetHead + "^{tree}":
		return "d"
	case "read-tree " + rawClosureBaseTree:
		return "R"
	case "read-tree " + rawClosureTargetTree:
		return "r"
	case "read-tree " + rawClosureMergedTree:
		return "q"
	case "add -A -- .":
		return "a"
	case "rev-parse --verify HEAD^{commit}":
		return "L"
	case "apply --cached --binary --whitespace=nowarn -":
		return "A"
	case "write-tree":
		return "W"
	case "hash-object -w --stdin":
		return "h"
	case "update-index -z --add --index-info":
		return "U"
	case "rev-parse --git-dir":
		return "N"
	case "--version":
		return "V"
	case "rev-parse --verify " + r.record.SourceAnchorRef:
		return "s"
	case "rev-parse --verify " + r.record.MergedAnchorRef:
		return "m"
	case "cat-file -t " + rawClosureReviewTree:
		return "t"
	case "cat-file -t " + rawClosureMergedTree:
		return "u"
	case "diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureBaseTree + " " + rawClosureReviewTree + " --":
		return "X"
	case "diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureBaseTree + " " + rawClosureTargetTree + " --":
		return "Y"
	case "diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureTargetTree + " " + rawClosureMergedTree + " --":
		return "Z"
	}
	if key == "diff --no-index --text --no-renames --diff-algorithm=myers --minimal --no-indent-heuristic --unified=0 --inter-hunk-context=0 --no-ext-diff --no-textconv --no-color --src-prefix=a/ --dst-prefix=b/ -- old new" {
		return "n"
	}
	if strings.HasPrefix(key, "--literal-pathspecs ls-tree -r -z --full-tree ") {
		switch {
		case strings.Contains(key, rawClosureBaseTree):
			return "E"
		case strings.Contains(key, rawClosureReviewTree):
			return "F"
		case strings.Contains(key, rawClosureTargetTree):
			return "G"
		}
	}
	if key == "cat-file blob "+rawClosureOID(rawClosureBaseBlob) {
		return "H"
	}
	if key == "cat-file blob "+rawClosureOID(rawClosureChainBlob) {
		return "I"
	}
	if key == "cat-file blob "+rawClosureOID(rawClosureTargetBlob) {
		return "J"
	}
	r.t.Fatalf("undeclared raw operation %q", key)
	return ""
}

func (r *rawClosureFacts) fact(request gittree.RawRequest, args []string, key, index string) gittree.RawResult {
	r.t.Helper()
	root := r.f.fixture.controller
	if strings.HasPrefix(key, "diff --no-index ") {
		if request.Dir == root || request.Dir == r.f.fixture.worktree {
			r.t.Fatal("canonical diff ran inside a checkout")
		}
		old, err := os.ReadFile(filepath.Join(request.Dir, "old"))
		if err != nil {
			r.t.Fatal(err)
		}
		newer, err := os.ReadFile(filepath.Join(request.Dir, "new"))
		if err != nil {
			r.t.Fatal(err)
		}
		switch {
		case bytes.Equal(old, []byte(rawClosureBaseBlob)) && bytes.Equal(newer, []byte(rawClosureChainBlob)):
			return gittree.RawResult{Stdout: rawClosureNoIndex(rawClosureBaseBlob, rawClosureChainBlob, "@@ -1 +1 @@\n-top\n+chain\n"), ExitCode: 1}
		case bytes.Equal(old, []byte(rawClosureBaseBlob)) && bytes.Equal(newer, []byte(rawClosureTargetBlob)):
			return gittree.RawResult{Stdout: rawClosureNoIndex(rawClosureBaseBlob, rawClosureTargetBlob, "@@ -3 +3 @@\n-bottom\n+target\n"), ExitCode: 1}
		default:
			r.t.Fatalf("undeclared temporary blob bytes: old=%q new=%q", old, newer)
		}
	}
	line := func(s string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(s + "\n")} }
	switch key {
	case "rev-parse --show-toplevel":
		return line(request.Dir)
	case "rev-parse --show-prefix":
		return line("")
	case "rev-parse --verify HEAD^{commit}":
		return line(rawClosureTargetHead)
	case "rev-parse --git-dir":
		return gittree.RawResult{ExitCode: 128}
	case "--version":
		return line("git version fixture")
	case "rev-parse --verify " + r.f.fixture.baseSha + "^{commit}":
		return line(r.f.fixture.baseSha)
	case "rev-parse --verify " + rawClosureTargetHead + "^{commit}":
		return line(rawClosureTargetHead)
	case "merge-base --all " + r.f.fixture.baseSha + " " + rawClosureTargetHead:
		return line(r.f.fixture.baseSha)
	case "rev-parse " + r.f.fixture.baseSha + "^{tree}":
		return line(rawClosureBaseTree)
	case "rev-parse " + rawClosureTargetHead + "^{tree}":
		return line(rawClosureTargetTree)
	case "rev-parse --verify " + r.record.SourceAnchorRef:
		return line(rawClosureReviewTree)
	case "rev-parse --verify " + r.record.MergedAnchorRef:
		return line(rawClosureMergedTree)
	case "cat-file -t " + rawClosureReviewTree, "cat-file -t " + rawClosureMergedTree:
		return line("tree")
	case "diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureBaseTree + " " + rawClosureReviewTree + " --",
		"diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureBaseTree + " " + rawClosureTargetTree + " --",
		"diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none " + rawClosureTargetTree + " " + rawClosureMergedTree + " --":
		return gittree.RawResult{Stdout: []byte("source.txt\x00")}
	case "read-tree " + rawClosureBaseTree, "read-tree " + rawClosureTargetTree, "read-tree " + rawClosureMergedTree:
		if index == "" {
			r.t.Fatal("read-tree without isolated index")
		}
		return gittree.RawResult{}
	case "add -A -- .":
		if request.Dir != r.f.fixture.worktree || index == "" || r.indices[index] != rawClosureMergedTree {
			r.t.Fatalf("undeclared seeded snapshot: dir=%q index=%q", request.Dir, index)
		}
		if err := r.worktreeDrift(); err != nil {
			if !r.allowWorktreeDrift || err.Error() != `worktree bytes changed at "docs/note.md"` {
				r.t.Fatalf("seeded snapshot: %v", err)
			}
			r.indices[index] = strings.Repeat("f", 40)
		}
		r.snapshotIndex = index
		return gittree.RawResult{}
	case "apply --cached --binary --whitespace=nowarn -":
		if index == "" {
			r.t.Fatal("apply without isolated index")
		}
		seed := r.indices[index]
		if seed == rawClosureBaseTree && bytes.Equal(request.Stdin, r.patch) {
			r.indices[index] = rawClosureReviewTree
			return gittree.RawResult{}
		}
		if seed == rawClosureTargetTree && bytes.Equal(request.Stdin, r.mergedPatch) {
			r.indices[index] = rawClosureMergedTree
			return gittree.RawResult{}
		}
		r.t.Fatalf("undeclared apply: seed=%s patch=%q", seed, request.Stdin)
	case "write-tree":
		if index == "" {
			r.t.Fatal("write-tree without isolated index")
		}
		if index == r.snapshotIndex {
			err := r.worktreeDrift()
			if err != nil && (!r.allowWorktreeDrift || err.Error() != `worktree bytes changed at "docs/note.md"` || r.indices[index] != strings.Repeat("f", 40)) {
				r.t.Fatalf("seeded write-tree: %v", err)
			}
			if err == nil && r.indices[index] != rawClosureMergedTree {
				r.t.Fatal("seeded write-tree returned a changed tree for unchanged bytes")
			}
			r.snapshotIndex = ""
		}
		return line(r.indices[index])
	case "hash-object -w --stdin":
		if !bytes.Equal(request.Stdin, []byte(rawClosureMergedBlob)) {
			r.t.Fatalf("undeclared hashed blob %q", request.Stdin)
		}
		return line(rawClosureOID(rawClosureMergedBlob))
	case "update-index -z --add --index-info":
		want := []byte("100644 " + rawClosureOID(rawClosureMergedBlob) + "\tsource.txt\x00")
		if index == "" || r.indices[index] != rawClosureTargetTree || !bytes.Equal(request.Stdin, want) {
			r.t.Fatalf("undeclared index update: %q", request.Stdin)
		}
		r.indices[index] = rawClosureMergedTree
		return gittree.RawResult{}
	}
	if strings.HasPrefix(key, "--literal-pathspecs ls-tree -r -z --full-tree ") {
		if len(args) != 8 || args[6] != "--" || args[7] != "source.txt" {
			r.t.Fatalf("unexpected ls-tree path: %q", key)
		}
		tree := args[5]
		blob := map[string]string{rawClosureBaseTree: rawClosureBaseBlob, rawClosureReviewTree: rawClosureChainBlob, rawClosureTargetTree: rawClosureTargetBlob}[tree]
		if blob == "" {
			r.t.Fatalf("undeclared ls-tree tree %q", tree)
		}
		return gittree.RawResult{Stdout: []byte("100644 blob " + rawClosureOID(blob) + "\tsource.txt\x00")}
	}
	if strings.HasPrefix(key, "cat-file blob ") {
		id := strings.TrimPrefix(key, "cat-file blob ")
		for _, blob := range []string{rawClosureBaseBlob, rawClosureChainBlob, rawClosureTargetBlob} {
			if rawClosureOID(blob) == id {
				return gittree.RawResult{Stdout: []byte(blob)}
			}
		}
	}
	r.t.Fatalf("undeclared raw call dir=%q key=%q stdin=%q", request.Dir, key, request.Stdin)
	return gittree.RawResult{}
}

func TestVerifyRecertificationNativeProducerMaterializesPublishesAndHandsOff(t *testing.T) {
	f := newRecertificationClosureFixture(t)
	proof := f.prepareVerifiedRecertification()
	data, err := os.ReadFile(filepath.Join(f.fixture.controller, proof))
	if err != nil {
		t.Fatal(err)
	}
	var record RecertificationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"source.txt": "reviewed change\n", "docs/note.md": "target change\n"} {
		got, err := os.ReadFile(filepath.Join(f.fixture.worktree, name))
		if err != nil || string(got) != want {
			t.Fatalf("materialized %s = %q: %v", name, got, err)
		}
	}
	for ref, want := range map[string]string{record.SourceAnchorRef: record.SourceWholeTree, record.MergedAnchorRef: record.MergedWholeTree} {
		if got := f.fixture.git(f.fixture.worktree, "rev-parse", "--verify", ref); got != want {
			t.Fatalf("published anchor %s = %s, want %s", ref, got, want)
		}
	}
	if got, err := VerifyRecertification(f.fixture.controller, "implementation", proof); err != nil || got.Record.RecordDigest != record.RecordDigest {
		t.Fatalf("published proof handoff = %+v, %v", got, err)
	}
}

func TestMergeRecertifiedRawVerifierHandoff(t *testing.T) {
	f, path, raw := newRawRecertificationClosure(t)
	if err := os.WriteFile(filepath.Join(f.fixture.worktree, "source.txt"), []byte(rawClosureMergedBlob), 0o644); err != nil {
		t.Fatal(err)
	}
	raw.want = rawClosureCombinedSequence
	f.run.workspace = f.fixture.worktree
	f.run.rawSource = raw.answer
	jobPath := filepath.Join(f.fixture.controller, "artifacts", "agents", "jobs", "implementation-r2.json")
	out, errs, code := f.run.mergeRecertified(jobPath, path)
	if code != 0 || len(errs) != 0 || !containsLine(out, "recertification="+path) ||
		!containsLine(out, "certifiedTree="+rawClosureMergedTree) {
		t.Fatalf("recertified merge: code=%d out=%v errs=%v", code, out, errs)
	}
	raw.consumed()
}

func TestMergeRecertifiedRawVerifierRejectsSiblingDrift(t *testing.T) {
	f, path, raw := newRawRecertificationClosure(t)
	if err := os.WriteFile(filepath.Join(f.fixture.worktree, "source.txt"), []byte(rawClosureMergedBlob), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.fixture.worktree, "docs/note.md"), []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw.want = rawClosureChangedSequence
	raw.allowWorktreeDrift = true
	f.run.workspace = f.fixture.worktree
	f.run.rawSource = raw.answer
	jobPath := filepath.Join(f.fixture.controller, "artifacts", "agents", "jobs", "implementation-r2.json")
	_, errs, code := f.run.mergeRecertified(jobPath, path)
	if code != 1 || !containsLine(errs, "conformance failure: chain-recertification-source-changed: final source worktree is not the exact materialized merged tree") {
		t.Fatalf("sibling drift: code=%d errs=%v", code, errs)
	}
	raw.consumed()
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
