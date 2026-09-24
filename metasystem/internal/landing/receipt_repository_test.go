package landing

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// receiptReaderFixture declares the index and worktree versions independently.
// The raw source checks the files before naming a worktree snapshot.
type receiptReaderFixture struct {
	t          *testing.T
	repository string
	root       string
	before     map[string][]byte
	after      map[string][]byte
	index      map[string][]byte
	candidate  string
	indexAfter string
	rawBefore  string
	rawAfter   string
	identity   string
	indices    map[string]receiptPrivateIndex
	isolated   struct {
		root  string
		after map[string][]byte
	}
}

type receiptPrivateIndex struct{ kind, phase, seed string }

func newReceiptReaderFixture(t *testing.T, changedPath string, staged bool) *receiptReaderFixture {
	t.Helper()
	base := newRepositoryObservationFixture(t)
	base.write("product.txt", "candidate\n")
	before := make(map[string][]byte, len(base.baseFiles))
	for path, data := range base.baseFiles {
		before[path] = bytes.Clone(data)
	}
	before["product.txt"] = []byte("candidate\n")
	after := make(map[string][]byte, len(before))
	for path, data := range before {
		after[path] = bytes.Clone(data)
	}
	if changedPath != "" {
		after[changedPath] = append(after[changedPath], "after-receipt\n"...)
	}
	index := before
	if staged {
		index = after
	}
	f := &receiptReaderFixture{
		t: t, repository: base.repository, root: base.root,
		before: before, after: after, index: index,
		candidate: receiptFactID(before), indexAfter: receiptFactID(index),
		rawBefore: receiptFactID(before), rawAfter: receiptFactID(after),
		identity: receiptFactID(receiptWithoutRegisters(before)),
		indices:  map[string]receiptPrivateIndex{},
	}
	f.checkFactFiles(f.before)
	return f
}

func receiptWithoutRegisters(files map[string][]byte) map[string][]byte {
	filtered := make(map[string][]byte, len(files))
	for path, data := range files {
		if !slices.Contains(appendOnlyRegisters, path) {
			filtered[path] = data
		}
	}
	return filtered
}

// These names identify declared facts; the production parser still computes
// which observations must agree. This fixture does not serialize Git trees.
func receiptFactID(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	h := sha1.New()
	for _, path := range paths {
		fmt.Fprintf(h, "100644 %s\x00%s\x00", path, files[path])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (f *receiptReaderFixture) receipt() string {
	f.t.Helper()
	receipt := TestReceipt{
		SchemaVersion: 3, Tree: f.candidate, Command: "true", ExitStatus: 0,
		Time: "2026-09-24T08:00:00Z", Binding: filteredReceiptBinding(f.candidate, f.identity),
		WorktreeProjection: &TestReceiptProjection{Excludes: AppendOnlyRegisters(), Tree: f.identity},
	}
	writeTestReceiptFixture(f.t, f.root, f.candidate, receipt)
	return TestReceiptPath(f.root, f.candidate)
}

func (f *receiptReaderFixture) params() ObserveParams {
	return ObserveParams{RepoRoot: f.root, CandidateTree: f.candidate, TestReceipt: TestReceiptPath(f.root, f.candidate)}
}

func (f *receiptReaderFixture) read() (TestReceipt, error) {
	return readTestReceiptWithWorkspace(f.params(), gittree.Workspace{Dir: f.root, RawSource: f.raw})
}

func (f *receiptReaderFixture) checkFiles() {
	f.checkFactFiles(f.after)
}

func (f *receiptReaderFixture) checkFactFiles(facts map[string][]byte) {
	f.checkFactFilesAt(f.root, facts)
}

func (f *receiptReaderFixture) checkFactFilesAt(root string, facts map[string][]byte) {
	f.t.Helper()
	for path, want := range facts {
		full := filepath.Join(root, filepath.FromSlash(path))
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(got, want) {
			f.t.Fatalf("snapshot file %s = %q, %v; want %q", path, got, err, want)
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			f.t.Fatalf("snapshot mode %s = %v, %v; want regular 0644", path, info, err)
		}
	}
	outside, err := os.ReadFile(filepath.Join(f.repository, "development", "metasystem-design.md"))
	if err != nil || string(outside) != "fixture\n" {
		f.t.Fatalf("nested installation sibling = %q, %v", outside, err)
	}
}

func (f *receiptReaderFixture) checkIsolatedFiles() string {
	f.t.Helper()
	register := "records/narrator-digest.log"
	actual, err := os.ReadFile(filepath.Join(f.isolated.root, register))
	if err != nil {
		f.t.Fatal(err)
	}
	if bytes.Equal(actual, f.before[register]) {
		f.checkFactFilesAt(f.isolated.root, f.before)
		return f.candidate
	}
	f.checkFactFilesAt(f.isolated.root, f.isolated.after)
	return receiptFactID(f.isolated.after)
}

func (f *receiptReaderFixture) entries() []byte {
	paths := make([]string, 0, len(f.index))
	for path := range f.index {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var lines bytes.Buffer
	for _, path := range paths {
		fmt.Fprintf(&lines, "100644 %s 0\t%s\x00", chainBlobOID(f.index[path]), path)
	}
	return lines.Bytes()
}

func (f *receiptReaderFixture) raw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	pins := []string{
		"-c", "core.fileMode=true", "-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no",
		"-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
	}
	isolated := f.isolated.root != "" && request.Dir == f.isolated.root
	workspace := request.Dir == f.root || isolated
	toplevel := request.Dir == f.repository || isolated
	if !workspace && !toplevel {
		f.t.Fatalf("raw Git cwd = %q", request.Dir)
	}
	prefix := append([]string{"-C", request.Dir}, pins...)
	if len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
		f.t.Fatalf("raw Git pins/cwd = %q, want prefix %q", request.Args, prefix)
	}
	args := request.Args[len(prefix):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw Git operation = %q, args = %q", request.Operation, args)
	}
	var private string
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			if private != "" {
				f.t.Fatal("multiple private index paths")
			}
			private = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	wantEnv := gittree.ScrubbedEnviron()
	if private != "" {
		if filepath.Base(private) != "index" || !filepath.IsAbs(private) {
			f.t.Fatalf("private index path = %q", private)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + private)
	}
	if !reflect.DeepEqual(request.Env, wantEnv) {
		f.t.Fatal("raw Git environment differs from scrubbed environment and private index")
	}
	if request.Stdin != nil && !(toplevel && slices.Equal(args, []string{"update-index", "-z", "--index-info"})) {
		f.t.Fatalf("unexpected raw stdin for %q", args)
	}
	answer := func(id string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(id + "\n")} }
	state, known := f.indices[private]
	switch {
	case private == "" && request.Dir == f.root && slices.Equal(args, []string{"diff", "--binary", "--no-renames", "--unified=3", "--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none", "--src-prefix=a/", "--dst-prefix=b/", f.candidate, f.candidate, "--"}):
		return gittree.RawResult{}
	case private == "" && isolated && slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		return answer("")
	case private == "" && isolated && slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
		return answer(f.isolated.root)
	case private == "" && request.Dir == f.root && slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		return answer("metasystem/")
	case private == "" && request.Dir == f.root && slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
		return answer(f.repository)
	case private == "" && request.Dir == f.root && len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], ":metasystem"):
		if args[1] != f.snapshotTop()+":metasystem" {
			f.t.Fatalf("undeclared subtree request %q", args)
		}
		return answer(f.rawAfter)
	case private == "" && workspace && slices.Equal(args, []string{"ls-files", "--stage", "-z"}):
		return gittree.RawResult{Stdout: f.entries()}
	case private != "" && !known && workspace && slices.Equal(args, []string{"read-tree", "--empty"}):
		f.indices[private] = receiptPrivateIndex{kind: "staged", phase: "seeded"}
		return gittree.RawResult{}
	case private != "" && !known && workspace && slices.Equal(args, []string{"read-tree", "HEAD"}):
		f.indices[private] = receiptPrivateIndex{kind: "snapshot", phase: "seeded"}
		return gittree.RawResult{}
	case private != "" && !known && workspace && len(args) == 2 && args[0] == "read-tree" && (args[1] == f.candidate || args[1] == f.rawAfter || isolated && args[1] == receiptFactID(f.isolated.after)):
		f.indices[private] = receiptPrivateIndex{kind: "filter", phase: "seeded", seed: args[1]}
		return gittree.RawResult{}
	case private != "" && known && state.kind == "staged" && state.phase == "seeded" && toplevel && slices.Equal(args, []string{"update-index", "-z", "--index-info"}) && bytes.Equal(request.Stdin, f.entries()):
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && known && state.kind == "snapshot" && state.phase == "seeded" && workspace && slices.Equal(args, []string{"add", "-A", "--", "."}):
		if isolated {
			f.checkIsolatedFiles()
		} else {
			f.checkFiles()
		}
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && known && state.kind == "filter" && state.phase == "seeded" && toplevel && slices.Equal(args, append([]string{"update-index", "--force-remove", "--"}, appendOnlyRegisters...)):
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && known && state.phase == "ready" && workspace && slices.Equal(args, []string{"write-tree"}):
		state.phase = "done"
		f.indices[private] = state
		switch state.kind {
		case "staged":
			return answer(f.indexAfter)
		case "snapshot":
			if isolated {
				return answer(f.checkIsolatedFiles())
			}
			f.checkFiles()
			return answer(f.snapshotTop())
		case "filter":
			if state.seed == f.candidate || isolated && state.seed == receiptFactID(f.isolated.after) {
				return answer(f.identity)
			}
			return answer(receiptFactID(receiptWithoutRegisters(f.after)))
		}
	}
	f.t.Fatalf("undeclared raw receipt request: cwd=%q, args=%q, stdin=%q, index=%+v", request.Dir, args, request.Stdin, state)
	return gittree.RawResult{}
}

func (f *receiptReaderFixture) snapshotTop() string {
	sum := sha1.Sum([]byte("nested snapshot:" + f.rawAfter + ":fixture\n"))
	return fmt.Sprintf("%x", sum)
}
