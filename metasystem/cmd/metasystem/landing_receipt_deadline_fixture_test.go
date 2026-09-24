package main

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type deadlineReceiptIndex struct{ kind, phase string }

type deadlineReceiptTreeFixture struct {
	t                            *testing.T
	root, frozen, tree, filtered string
	files                        map[string][]byte
	ignored                      map[string][]byte
	indices                      map[string]deadlineReceiptIndex
}

func newDeadlineReceiptTreeFixture(t *testing.T, repository *proofAdmissionRepository) *deadlineReceiptTreeFixture {
	t.Helper()
	f := &deadlineReceiptTreeFixture{t: t, root: repository.root, files: map[string][]byte{}, ignored: map[string][]byte{}, indices: map[string]deadlineReceiptIndex{}}
	repository.mu.Lock()
	accepted := proofAdmissionClone(repository.commits[repository.accepted].files)
	repository.mu.Unlock()
	for full, data := range accepted {
		if relative, ok := strings.CutPrefix(full, "metasystem/"); ok {
			f.write(relative, data)
			if strings.HasPrefix(relative, "plans/goals/") || relative == "memory/receipts.log" {
				f.ignored[relative] = data
				delete(f.files, relative)
			}
		}
	}
	f.write(".gitignore", []byte("artifacts/\nplans/goals/\nmemory/receipts.log\n"))
	f.write("payload.txt", []byte("receipt deadline candidate\n"))
	f.tree = deadlineReceiptTreeID(f.files)
	filtered := make(map[string][]byte, len(f.files))
	for path, data := range f.files {
		if !slices.Contains(landing.AppendOnlyRegisters(), path) {
			filtered[path] = data
		}
	}
	f.filtered = deadlineReceiptTreeID(filtered)
	f.checkFiles(f.root)
	return f
}

func (f *deadlineReceiptTreeFixture) write(path string, data []byte) {
	f.t.Helper()
	full := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(full, 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.files[path] = append([]byte(nil), data...)
}

func deadlineReceiptOID(kind string, content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s %d\x00", kind, len(content))
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func deadlineReceiptTreeID(files map[string][]byte) string {
	type item struct{ name, mode, oid string }
	var entries []item
	directories := map[string]map[string][]byte{}
	for path, data := range files {
		if head, tail, ok := strings.Cut(path, "/"); ok {
			if directories[head] == nil {
				directories[head] = map[string][]byte{}
			}
			directories[head][tail] = data
		} else {
			entries = append(entries, item{path, "100644", deadlineReceiptOID("blob", data)})
		}
	}
	for name, children := range directories {
		entries = append(entries, item{name, "40000", deadlineReceiptTreeID(children)})
	}
	slices.SortFunc(entries, func(a, b item) int { return strings.Compare(a.name, b.name) })
	var content bytes.Buffer
	for _, entry := range entries {
		fmt.Fprintf(&content, "%s %s\x00", entry.mode, entry.name)
		for i := 0; i < len(entry.oid); i += 2 {
			var octet byte
			fmt.Sscanf(entry.oid[i:i+2], "%02x", &octet)
			content.WriteByte(octet)
		}
	}
	return deadlineReceiptOID("tree", content.Bytes())
}

func (f *deadlineReceiptTreeFixture) entries() []byte {
	paths := make([]string, 0, len(f.files))
	for path := range f.files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var entries bytes.Buffer
	for _, path := range paths {
		fmt.Fprintf(&entries, "100644 %s 0\t%s\x00", deadlineReceiptOID("blob", f.files[path]), path)
	}
	return entries.Bytes()
}

func (f *deadlineReceiptTreeFixture) checkFiles(root string) {
	f.t.Helper()
	expected := f.files
	if root == f.root {
		for path, want := range f.ignored {
			full := filepath.Join(root, filepath.FromSlash(path))
			got, err := os.ReadFile(full)
			if err != nil || !bytes.Equal(got, want) {
				f.t.Fatalf("accepted file %s = %q, %v; want %q", path, got, err, want)
			}
			info, err := os.Lstat(full)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
				f.t.Fatalf("accepted mode %s = %v, %v", path, info, err)
			}
		}
	} else {
		// Freeze carries the accepted live goal pages even though this fixture's
		// Git projection ignores them. Check their exact bytes in the export.
		expected = make(map[string][]byte, len(f.files)+len(f.ignored))
		for path, data := range f.files {
			expected[path] = data
		}
		for path, data := range f.ignored {
			name, ok := strings.CutPrefix(path, "plans/goals/")
			if ok && name != "backlog.md" && !strings.Contains(name, "/") && strings.HasSuffix(name, ".md") {
				expected[path] = data
			}
		}
	}
	for path, want := range expected {
		full := filepath.Join(root, filepath.FromSlash(path))
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(got, want) {
			f.t.Fatalf("receipt file %s = %q, %v; want %q", path, got, err, want)
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() ||
			(root == f.root && info.Mode().Perm() != 0o644) ||
			(root != f.root && (info.Mode().Perm()&0o600 != 0o600 || info.Mode().Perm()&^os.FileMode(0o644) != 0)) {
			f.t.Fatalf("receipt mode %s = %v, %v", path, info, err)
		}
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "artifacts" && entry.IsDir() {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			if _, ok := expected[rel]; !ok {
				if root == f.root {
					if _, accepted := f.ignored[rel]; accepted {
						return nil
					}
				}
				return fmt.Errorf("undeclared receipt file %s", rel)
			}
		}
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *deadlineReceiptTreeFixture) workspace() gittree.Workspace {
	return gittree.Workspace{Dir: f.root, RawSource: f.strictRaw}
}

func (f *deadlineReceiptTreeFixture) strictRaw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	if request.Dir != f.root && request.Dir != f.frozen {
		f.t.Fatalf("raw receipt cwd %q", request.Dir)
	}
	pins := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	prefix := append([]string{"-C", request.Dir}, pins...)
	if len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
		f.t.Fatalf("raw receipt pins %q", request.Args)
	}
	args := request.Args[len(prefix):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw receipt operation %q args %q", request.Operation, args)
	}
	private := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			if private != "" {
				f.t.Fatal("multiple private indices")
			}
			private = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	wantEnv := gittree.ScrubbedEnviron()
	if private != "" {
		if !filepath.IsAbs(private) || filepath.Base(private) != "index" {
			f.t.Fatalf("private index %q", private)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + private)
	}
	if !reflect.DeepEqual(request.Env, wantEnv) {
		f.t.Fatal("raw receipt environment differs from scrubbed environment")
	}
	if request.Stdin != nil && !(private != "" && slices.Equal(args, []string{"update-index", "-z", "--index-info"})) {
		f.t.Fatalf("unexpected raw stdin for %q", args)
	}
	answer := func(id string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(id + "\n")} }
	state, known := f.indices[private]
	switch {
	case private == "" && slices.Equal(args, []string{"diff", "--binary", "--no-renames", "--unified=3", "--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none", "--src-prefix=a/", "--dst-prefix=b/", f.tree, f.tree, "--"}):
		return gittree.RawResult{}
	case private == "" && slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		return answer("")
	case private == "" && slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
		return answer(request.Dir)
	case private == "" && slices.Equal(args, []string{"ls-files", "--stage", "-z"}):
		return gittree.RawResult{Stdout: f.entries()}
	case private != "" && (!known || state.phase == "done") && slices.Equal(args, []string{"read-tree", "--empty"}):
		f.indices[private] = deadlineReceiptIndex{"staged", "seeded"}
		return gittree.RawResult{}
	case private != "" && (!known || state.phase == "done") && slices.Equal(args, []string{"read-tree", "HEAD"}):
		f.indices[private] = deadlineReceiptIndex{"snapshot", "seeded"}
		return gittree.RawResult{}
	case private != "" && (!known || state.phase == "done") && slices.Equal(args, []string{"read-tree", f.tree}):
		f.indices[private] = deadlineReceiptIndex{"filter", "seeded"}
		return gittree.RawResult{}
	case private != "" && state.kind == "staged" && state.phase == "seeded" && slices.Equal(args, []string{"update-index", "-z", "--index-info"}) && bytes.Equal(request.Stdin, f.entries()):
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && state.kind == "snapshot" && state.phase == "seeded" && slices.Equal(args, []string{"add", "-A", "--", "."}):
		f.checkFiles(request.Dir)
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && state.kind == "filter" && state.phase == "seeded" && slices.Equal(args, append([]string{"update-index", "--force-remove", "--"}, landing.AppendOnlyRegisters()...)):
		state.phase = "ready"
		f.indices[private] = state
		return gittree.RawResult{}
	case private != "" && state.phase == "ready" && slices.Equal(args, []string{"write-tree"}):
		state.phase = "done"
		f.indices[private] = state
		switch state.kind {
		case "staged", "snapshot":
			return answer(f.tree)
		case "filter":
			return answer(f.filtered)
		}
	}
	f.indices[private] = state
	f.t.Fatalf("undeclared raw receipt request: cwd=%q args=%q stdin=%q state=%+v", request.Dir, args, request.Stdin, state)
	return gittree.RawResult{}
}
