package validate

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const rawBaseTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

var rawTargetCommit = strings.Repeat("1", 40)

type rawConformanceFixture struct {
	f          *conformanceFixture
	committed  bool
	calls      []rawConformanceCall
	next       int
	indexPaths map[int]string
}

type rawConformanceCall struct {
	dir    string
	args   []string
	stdout []byte
	stdin  []byte
	pinned bool
	index  int
}

type rawConformanceFact struct {
	tree    string
	paths   []string
	patch   []byte
	numstat string
}

func newRawConformanceFixture(t *testing.T) *conformanceFixture {
	f := newFileConformanceFixture(t)
	f.raw = &rawConformanceFixture{f: f}
	return f
}

func rawBlobID(data string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(data), data)))
	return fmt.Sprintf("%x", h[:])[:7]
}

func rawPatch(path, before, after string) string {
	oldID := "0000000"
	oldPath := "/dev/null"
	header := "new file mode 100644\n"
	mode := ""
	if before != "" {
		oldID, oldPath, header = rawBlobID(before), "a/"+path, ""
		mode = " 100644"
	}
	oldLines := strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	if before == "" {
		oldLines = nil
	}
	newLines := strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	var hunk strings.Builder
	if before == "" {
		fmt.Fprintf(&hunk, "@@ -0,0 +1,%d @@\n", len(newLines))
		for _, line := range newLines {
			fmt.Fprintf(&hunk, "+%s\n", line)
		}
	} else {
		fmt.Fprintf(&hunk, "@@ -1 +1,%d @@\n", len(newLines))
		fmt.Fprintf(&hunk, " %s\n", oldLines[0])
		for _, line := range newLines[1:] {
			fmt.Fprintf(&hunk, "+%s\n", line)
		}
	}
	return fmt.Sprintf("diff --git a/%s b/%s\n%sindex %s..%s%s\n--- %s\n+++ b/%s\n%s",
		path, path, header, oldID, rawBlobID(after), mode, oldPath, path, hunk.String())
}

func (r *rawConformanceFixture) fact() rawConformanceFact {
	r.f.t.Helper()
	base := map[string]string{
		".gitignore": "artifacts/\nlocal.conf\n", "source.txt": "base\n",
		"docs/note.md": "base\n", "metasystem.conf": "metasystem.version=1\nrole.code-critic.runtime=fake\n",
	}
	classes, err := os.ReadFile(filepath.Join(r.f.controller, "scripts", "agents", "path-classes.txt"))
	if err != nil {
		r.f.t.Fatal(err)
	}
	base["scripts/agents/path-classes.txt"] = string(classes)
	changes := map[string]string{}
	seen := map[string]bool{}
	err = filepath.WalkDir(r.f.worktree, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(r.f.worktree, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == "artifacts" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("raw repository entry %s is not a regular file", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen[rel] = true
		if original, known := base[rel]; known && original == string(data) {
			return nil
		}
		changes[rel] = string(data)
		return nil
	})
	if err != nil {
		r.f.t.Fatal(err)
	}
	for path := range base {
		if !seen[path] {
			r.f.t.Fatalf("raw repository baseline file %s is missing", path)
		}
	}
	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	key := strings.Join(paths, ",")
	var tree string
	switch key {
	case "source.txt":
		if changes[key] != "base\nchanged\n" {
			r.f.t.Fatalf("unexpected source bytes %q", changes[key])
		}
		tree = strings.Repeat("c", 40)
	case "extra.txt,source.txt":
		if changes["extra.txt"] != "undeclared\n" || changes["source.txt"] != "base\nchanged\n" {
			r.f.t.Fatalf("unexpected extra/source bytes: %q", changes)
		}
		tree = strings.Repeat("d", 40)
	case "plans/note.md,source.txt":
		if changes["plans/note.md"] != "delegate plan\n" || changes["source.txt"] != "base\nchanged\n" {
			r.f.t.Fatalf("unexpected plan/source bytes: %q", changes)
		}
		tree = strings.Repeat("e", 40)
	case "memory/receipts.log":
		if changes[key] != "delegate receipt\n" {
			r.f.t.Fatalf("unexpected receipt bytes %q", changes[key])
		}
		tree = strings.Repeat("f", 40)
	case "scripts/tool.sh":
		if changes[key] != "changed\n" {
			r.f.t.Fatalf("unexpected script bytes %q", changes[key])
		}
		tree = strings.Repeat("3", 40)
	case "records/note.md":
		if changes[key] != rawProse(10) && changes[key] != rawProse(40) {
			r.f.t.Fatalf("unexpected prose bytes %q", changes[key])
		}
		if changes[key] == rawProse(10) {
			tree = strings.Repeat("4", 40)
		} else {
			tree = strings.Repeat("5", 40)
		}
	case "plans/note.md":
		if changes[key] != "delegate plan\n" {
			r.f.t.Fatalf("unexpected plan bytes %q", changes[key])
		}
		tree = strings.Repeat("6", 40)
	case "AGENTS.md":
		if changes[key] != "changed\n" && changes[key] != "small behavior change\n" {
			r.f.t.Fatalf("unexpected instruction bytes %q", changes[key])
		}
		if changes[key] == "changed\n" {
			tree = strings.Repeat("7", 40)
		} else {
			tree = strings.Repeat("8", 40)
		}
	case "docs/note.md":
		if changes[key] != "base\nsmall\n" {
			r.f.t.Fatalf("unexpected docs bytes %q", changes[key])
		}
		tree = strings.Repeat("9", 40)
	case "plans/NEWRT.md":
		if changes[key] != "changed\n" {
			r.f.t.Fatalf("unexpected runtime instruction bytes %q", changes[key])
		}
		tree = strings.Repeat("0", 40)
	default:
		r.f.t.Fatalf("undeclared raw repository state: %q", key)
	}
	fact := rawConformanceFact{tree: tree, paths: paths}
	for _, path := range paths {
		before, after := base[path], changes[path]
		fact.patch = append(fact.patch, rawPatch(path, before, after)...)
		added := strings.Count(after, "\n") - strings.Count(before, "\n")
		fact.numstat += fmt.Sprintf("%d\t0\t%s\n", added, path)
	}
	return fact
}

func rawProse(count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func (r *rawConformanceFixture) direct(dir string, output string, args ...string) {
	r.calls = append(r.calls, rawConformanceCall{dir: dir, args: args, stdout: []byte(output)})
}

func (r *rawConformanceFixture) work(dir string, output string, index int, stdin []byte, args ...string) {
	r.calls = append(r.calls, rawConformanceCall{dir: dir, args: args, stdout: []byte(output), pinned: true, index: index, stdin: stdin})
}

func (r *rawConformanceFixture) treeOf(dir, rev, tree string) {
	r.work(dir, tree+"\n", 0, nil, "rev-parse", rev+"^{tree}")
	r.work(dir, "\n", 0, nil, "rev-parse", "--show-prefix")
}

func (r *rawConformanceFixture) snapshot(dir string, index int, tree string) {
	r.work(dir, "", index, nil, "read-tree", "HEAD")
	r.work(dir, "", index, nil, "add", "-A", "--", ".")
	r.work(dir, tree+"\n", index, nil, "write-tree")
	r.work(dir, "\n", 0, nil, "rev-parse", "--show-prefix")
}

func (r *rawConformanceFixture) diff(dir string, fact rawConformanceFact) {
	r.work(dir, string(fact.patch), 0, nil, "diff", "--binary", "--no-renames", "--unified=3", "--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none", "--src-prefix=a/", "--dst-prefix=b/", rawBaseTree, fact.tree, "--")
	r.work(dir, strings.Join(fact.paths, "\x00")+"\x00", 0, nil, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", rawBaseTree, fact.tree, "--")
}

func (r *rawConformanceFixture) prepare(stage string) {
	r.f.t.Helper()
	r.calls, r.next, r.indexPaths = nil, 0, map[int]string{}
	f := r.f
	fact := r.fact()
	r.direct(f.worktree, "", "cat-file", "-e", f.baseSha+"^{commit}")
	r.direct(f.controller, f.controller+"\n", "rev-parse", "--show-toplevel")
	r.direct(f.worktree, f.worktree+"\n", "rev-parse", "--show-toplevel")
	r.direct(f.controller, rawTargetCommit+"\n", "rev-parse", "HEAD^{commit}")
	r.direct(f.worktree, f.baseSha+"\n", "merge-base", rawTargetCommit, "HEAD")
	r.direct(f.controller, "\n", "rev-parse", "--show-prefix")
	if stage == "review" {
		r.snapshot(f.worktree, 1, fact.tree)
		r.snapshot(f.worktree, 2, fact.tree)
		r.treeOf(f.worktree, f.baseSha, rawBaseTree)
		r.diff(f.worktree, fact)
		return
	}
	if stage != "merge" || !r.committed {
		f.t.Fatalf("raw fixture cannot run stage %q before commit", stage)
	}
	r.treeOf(f.worktree, "HEAD", fact.tree)
	r.treeOf(f.worktree, "HEAD", fact.tree)
	job, ok := readJobRecord(filepath.Join(f.controller, "artifacts", "agents", "jobs"), "impl")
	if !ok {
		f.t.Fatal("implementer record missing")
	}
	if len(fact.paths) == 1 && fact.paths[0] == "plans/NEWRT.md" {
		return
	}
	if job["critiqueWaived"] != nil {
		r.direct(f.worktree, strings.Join(fact.paths, "\x00")+"\x00", "diff", "--name-only", "-z", "--no-renames", f.baseSha, "HEAD", "--")
		r.direct(f.worktree, fact.numstat, "diff", "--numstat", "--no-renames", f.baseSha, "HEAD", "--")
		canonical, err := filepath.EvalSymlinks(f.controller)
		if err != nil {
			f.t.Fatal(err)
		}
		for range fact.paths {
			r.work(canonical, canonical+"\n", 0, nil, "rev-parse", "--show-toplevel")
		}
	}
	if job["mission"] != nil {
		r.treeOf(f.worktree, f.baseSha, rawBaseTree)
		r.diff(f.worktree, fact)
		wardenRecord, wardenOK := readJobRecord(filepath.Join(f.controller, "artifacts", "agents", "jobs"), "net-warden")
		wardenBytes, wardenErr := os.ReadFile(filepath.Join(f.controller, "artifacts", "agents", "net-warden", "rounds", "1", "return.json"))
		var wardenReturn map[string]any
		if wardenErr == nil {
			wardenErr = json.Unmarshal(wardenBytes, &wardenReturn)
		}
		if !wardenOK || wardenRecord["chainClosed"] != true || wardenErr != nil || wardenReturn["reviewedTree"] != fact.tree {
			return
		}
		for _, tree := range []string{rawBaseTree, fact.tree} {
			r.work(f.controller, "tree\n", 0, nil, "cat-file", "-t", tree)
			r.work(f.controller, tree+"\n", 0, nil, "rev-parse", "--verify", tree)
			r.work(f.controller, "", 0, nil, "update-ref", "refs/metasystem/missions/m-net/"+tree, tree)
		}
		r.work(f.worktree, "", 3, nil, "read-tree", rawBaseTree)
		r.work(f.worktree, f.worktree+"\n", 0, nil, "rev-parse", "--show-toplevel")
		r.work(f.worktree, "", 3, fact.patch, "apply", "--cached", "--binary", "--whitespace=nowarn", "-")
		r.work(f.worktree, fact.tree+"\n", 3, nil, "write-tree")
	}
}

var rawPins = []string{
	"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
	"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
	"-c", "gc.auto=0", "-c", "maintenance.auto=false",
}

func (r *rawConformanceFixture) answer(request gittree.RawRequest) gittree.RawResult {
	r.f.t.Helper()
	if r.next >= len(r.calls) {
		r.f.t.Fatalf("unexpected raw Git call %v from %s", request.Args, request.Dir)
	}
	want := r.calls[r.next]
	r.next++
	args := append([]string{"-C", want.dir}, want.args...)
	if want.pinned {
		args = append(append([]string{"-C", want.dir}, rawPins...), want.args...)
	}
	if request.Dir != want.dir || !reflect.DeepEqual(request.Args, args) || !bytes.Equal(request.Stdin, want.stdin) {
		r.f.t.Fatalf("raw Git call %d: got dir=%q args=%q stdin=%q; want dir=%q args=%q stdin=%q", r.next, request.Dir, request.Args, request.Stdin, want.dir, args, want.stdin)
	}
	index := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			index = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	if want.index == 0 && index != "" {
		r.f.t.Fatalf("unexpected isolated index %q", index)
	}
	if want.index != 0 {
		if index == "" {
			r.f.t.Fatal("isolated index missing")
		}
		if previous := r.indexPaths[want.index]; previous == "" {
			for group, other := range r.indexPaths {
				if group != want.index && other == index {
					r.f.t.Fatalf("index reused across chains: %q", index)
				}
			}
			r.indexPaths[want.index] = index
		} else if previous != index {
			r.f.t.Fatalf("index changed inside chain: %q != %q", previous, index)
		}
		if info, err := os.Stat(filepath.Dir(index)); err != nil || !info.IsDir() {
			r.f.t.Fatalf("isolated index directory missing: %v", err)
		}
	}
	if request.Operation == "" || request.Timeout.Limit <= 0 {
		r.f.t.Fatalf("raw command lost bound or operation: %+v", request)
	}
	return gittree.RawResult{Stdout: want.stdout}
}

func (r *rawConformanceFixture) consumed() {
	r.f.t.Helper()
	if r.next != len(r.calls) {
		r.f.t.Fatalf("raw Git calls consumed %d of %d; next %+v", r.next, len(r.calls), r.calls[r.next])
	}
}
