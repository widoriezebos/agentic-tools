package landing

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

const advanceUpstream = "refs/remotes/origin/main"

var advancePins = []string{
	"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
	"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
	"-c", "gc.auto=0", "-c", "maintenance.auto=false",
}

type advanceRawStep struct {
	dir   func() string
	args  func() []string
	stdin []byte
	index bool
	reply func(gittree.RawRequest) gittree.RawResult
}

type advanceRawFixture struct {
	t                                   *testing.T
	root, detached                      string
	steps                               []advanceRawStep
	next                                int
	indexFile                           string
	head, landing, peer, rebased, moved string
	landingTree, rebasedTree            string
	branch                              string
	index                               map[string]string
	trees                               map[string]map[string]string
}

func advanceID(s string) string { return fmt.Sprintf("%x", sha1.Sum([]byte(s))) }
func advanceTreeID(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s\x00%s\x00", key, files[key])
	}
	return advanceID(b.String())
}
func advanceCopy(files map[string]string) map[string]string {
	out := make(map[string]string, len(files))
	for k, v := range files {
		out[k] = v
	}
	return out
}
func newAdvanceRawFixture(t *testing.T) *advanceRawFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"product.txt": "seed\n", "scripts/agents/go-gate.sh": "seed\n",
		"memory/receipts.log": "receipt=seed\n", "records/narrator-digest.log": "digest=seed\n",
	}
	for path, content := range files {
		writeAdvanceFile(t, root, path, content)
	}
	f := &advanceRawFixture{t: t, root: root, index: advanceCopy(files), trees: map[string]map[string]string{}, branch: "refs/heads/main"}
	f.landing, f.peer, f.rebased, f.moved = advanceID("landing"), advanceID("peer"), advanceID("rebased"), advanceID("moved")
	f.head = f.landing
	f.setLanding(files)
	f.setRebased(files)
	t.Cleanup(func() {
		if f.next != len(f.steps) {
			t.Errorf("raw Git transcript consumed %d of %d calls", f.next, len(f.steps))
		}
	})
	return f
}
func (f *advanceRawFixture) setLanding(files map[string]string) {
	f.landingTree = advanceTreeID(files)
	f.trees[f.landingTree] = advanceCopy(files)
}
func (f *advanceRawFixture) setRebased(files map[string]string) {
	f.rebasedTree = advanceTreeID(files)
	f.trees[f.rebasedTree] = advanceCopy(files)
}
func (f *advanceRawFixture) edit(path, value string) {
	f.t.Helper()
	writeAdvanceFile(f.t, f.root, path, value)
}
func (f *advanceRawFixture) add(dir func() string, args []string, stdout string) {
	f.steps = append(f.steps, advanceRawStep{dir: dir, args: func() []string { return args }, reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{Stdout: []byte(stdout)} }})
}
func (f *advanceRawFixture) addRoot(args []string, stdout string) {
	f.add(func() string { return f.root }, args, stdout)
}
func (f *advanceRawFixture) addReply(dir func() string, args []string, reply func(gittree.RawRequest) gittree.RawResult) {
	f.steps = append(f.steps, advanceRawStep{dir: dir, args: func() []string { return args }, reply: reply})
}
func (f *advanceRawFixture) detachedDir() string { return f.detached }
func (f *advanceRawFixture) run(r gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	if f.next >= len(f.steps) {
		f.t.Fatalf("unexpected raw Git call: %q", r.Args)
	}
	step := f.steps[f.next]
	f.next++
	if len(step.args()) == 5 && reflect.DeepEqual(step.args()[:3], []string{"worktree", "add", "--detach"}) && f.detached == "" {
		f.detached = r.Args[len(r.Args)-2]
	}
	if r.Dir != step.dir() {
		f.t.Fatalf("raw call %d dir = %q, want %q", f.next, r.Dir, step.dir())
	}
	wantArgs := append([]string{"-C", r.Dir}, advancePins...)
	wantArgs = append(wantArgs, step.args()...)
	if !reflect.DeepEqual(r.Args, wantArgs) {
		f.t.Fatalf("raw call %d argv = %q, want %q", f.next, r.Args, wantArgs)
	}
	if r.Operation != "git "+strings.Join(step.args(), " ") {
		f.t.Fatalf("raw call %d operation = %q", f.next, r.Operation)
	}
	if !bytes.Equal(r.Stdin, step.stdin) || (r.Stdin == nil) != (step.stdin == nil) {
		f.t.Fatalf("raw call %d stdin = %q, want %q", f.next, r.Stdin, step.stdin)
	}
	wantEnv := gittree.ScrubbedEnviron()
	if step.index {
		var current string
		for _, v := range r.Env {
			if strings.HasPrefix(v, "GIT_INDEX_FILE=") {
				current = strings.TrimPrefix(v, "GIT_INDEX_FILE=")
			}
		}
		if current == "" {
			f.t.Fatalf("raw call %d lacks isolated index", f.next)
		}
		if f.indexFile == "" {
			f.indexFile = current
		} else if current != f.indexFile {
			f.t.Fatalf("raw call %d switched isolated index", f.next)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + current)
	}
	if !reflect.DeepEqual(r.Env, wantEnv) {
		f.t.Fatalf("raw call %d environment differs from scrubbed environment", f.next)
	}
	result := step.reply(r)
	if step.index && reflect.DeepEqual(step.args(), []string{"write-tree"}) {
		f.indexFile = ""
	}
	return result
}
func (f *advanceRawFixture) workspace() gittree.Workspace {
	return gittree.Workspace{Dir: f.root, RawSource: f.run}
}
func (f *advanceRawFixture) finish() {
	f.t.Helper()
	if f.next != len(f.steps) {
		f.t.Fatalf("raw transcript consumed %d of %d calls", f.next, len(f.steps))
	}
	if f.detached != "" {
		if _, err := os.Stat(filepath.Dir(f.detached)); !os.IsNotExist(err) {
			f.t.Fatalf("temporary detached parent remains: %v", err)
		}
		if _, err := os.Stat(filepath.Join(f.root, ".git", "metasystem-worktree-admin.lock")); err != nil {
			f.t.Fatalf("worktree administration lock was not used: %v", err)
		}
	}
}
func (f *advanceRawFixture) staged() {
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addRoot([]string{"ls-files", "--stage", "-z"}, f.indexRecords())
	f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"read-tree", "--empty"} }, index: true, reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{} }})
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	records := f.indexRecords()
	f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"update-index", "-z", "--index-info"} }, stdin: []byte(records), index: true, reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{} }})
	f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"write-tree"} }, index: true, reply: func(gittree.RawRequest) gittree.RawResult {
		return gittree.RawResult{Stdout: []byte(advanceTreeID(f.index) + "\n")}
	}})
}
func (f *advanceRawFixture) indexRecords() string {
	keys := make([]string, 0, len(f.index))
	for k := range f.index {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		oid := advanceID("blob " + f.index[k])
		fmt.Fprintf(&b, "100644 %s 0\t%s\x00", oid, k)
	}
	return b.String()
}
func (f *advanceRawFixture) preflight(detached, staged bool) {
	if detached {
		f.addReply(func() string { return f.root }, []string{"symbolic-ref", "--quiet", "HEAD"}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{ExitCode: 1} })
		return
	}
	f.addRoot([]string{"symbolic-ref", "--quiet", "HEAD"}, f.branch+"\n")
	f.addReply(func() string { return f.root }, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{Stdout: []byte(f.head + "\n")} })
	headTree := f.landingTree
	if f.head == f.rebased {
		headTree = f.rebasedTree
	}
	f.addRoot([]string{"rev-parse", "--verify", f.head + "^{tree}"}, headTree+"\n")
	f.staged()
	if staged {
		return
	}
	f.addRoot([]string{"rev-parse", "--verify", advanceUpstream + "^{commit}"}, f.peer+"\n")
}
func (f *advanceRawFixture) ancestor(yes bool) {
	code := 1
	if yes {
		code = 0
	}
	f.addReply(func() string { return f.root }, []string{"merge-base", "--is-ancestor", f.peer, f.head}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{ExitCode: code} })
}
func (f *advanceRawFixture) detach(conflict bool) {
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addRoot([]string{"rev-parse", "--show-prefix"}, "")
	f.addRoot([]string{"rev-parse", "--git-common-dir"}, filepath.Join(f.root, ".git")+"\n")
	f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"worktree", "add", "--detach", f.detached, f.landing} }, reply: func(r gittree.RawRequest) gittree.RawResult {
		path := r.Args[len(r.Args)-2]
		parentName := filepath.Base(filepath.Dir(path))
		if !filepath.IsAbs(path) || !strings.HasPrefix(parentName, "metasystem-landing-advance.") || filepath.Base(path) != "worktree-"+strings.TrimPrefix(parentName, "metasystem-landing-advance.") {
			f.t.Fatalf("unexpected detached worktree path %q", path)
		}
		if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
			f.t.Fatalf("detached parent was not created: %v", err)
		}
		f.detached = path
		return gittree.RawResult{}
	}})
	driver, err := contractgit.RuntimeDriverArgs()
	if err != nil {
		f.t.Fatal(err)
	}
	args := append(append([]string{}, driver...), "rebase", advanceUpstream)
	if conflict {
		f.addReply(f.detachedDir, args, func(gittree.RawRequest) gittree.RawResult {
			return gittree.RawResult{Stderr: []byte("CONFLICT (content): Merge conflict in product.txt\n"), ExitCode: 1}
		})
		f.add(f.detachedDir, []string{"rebase", "--abort"}, "")
	} else {
		f.add(f.detachedDir, args, "")
		f.add(f.detachedDir, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, f.rebased+"\n")
	}
	f.addRoot([]string{"rev-parse", "--git-common-dir"}, filepath.Join(f.root, ".git")+"\n")
	f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"worktree", "remove", "--force", "--force", f.detached} }, reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{} }})
}
func (f *advanceRawFixture) status() string {
	keys := make([]string, 0, len(f.index))
	for k := range f.index {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(k)))
		if err != nil || string(data) != f.index[k] {
			fmt.Fprintf(&b, " M %s\x00", k)
		}
	}
	return b.String()
}
func (f *advanceRawFixture) observeStatus() []gittree.StatusEntry {
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addReply(func() string { return f.root }, []string{"status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=normal"}, func(gittree.RawRequest) gittree.RawResult {
		return gittree.RawResult{Stdout: []byte(f.status())}
	})
	entries, err := f.workspace().Status()
	if err != nil {
		f.t.Fatal(err)
	}
	return entries
}
func (f *advanceRawFixture) changed() []string {
	before, after := f.trees[f.landingTree], f.trees[f.rebasedTree]
	set := map[string]bool{}
	for k := range before {
		set[k] = true
	}
	for k := range after {
		set[k] = true
	}
	out := []string{}
	for k := range set {
		if before[k] != after[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
func (f *advanceRawFixture) entries(tree string, paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		if data, ok := f.trees[tree][p]; ok {
			fmt.Fprintf(&b, "100644 blob %s\t%s\x00", advanceID("blob "+data), p)
		}
	}
	return b.String()
}
func (f *advanceRawFixture) classify() {
	f.addRoot([]string{"rev-parse", "--verify", f.rebased + "^{tree}"}, f.rebasedTree+"\n")
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addRoot([]string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", f.landingTree, f.rebasedTree, "--"}, strings.Join(f.changed(), "\x00")+func() string {
		if len(f.changed()) > 0 {
			return "\x00"
		}
		return ""
	}())
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addReply(func() string { return f.root }, []string{"status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=normal"}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{Stdout: []byte(f.status())} })
	f.addRoot([]string{"rev-parse", "--show-prefix"}, "")
	overlap := []string{}
	changed := map[string]bool{}
	for _, p := range f.changed() {
		changed[p] = true
	}
	for k := range f.index {
		data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(k)))
		if (err != nil || string(data) != f.index[k]) && changed[k] {
			overlap = append(overlap, k)
		}
	}
	sort.Strings(overlap)
	if len(overlap) == 0 {
		return
	}
	for _, tree := range []string{f.landingTree, f.rebasedTree} {
		args := append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--"}, overlap...)
		f.addRoot(args, f.entries(tree, overlap))
	}
	for _, path := range overlap {
		if path != "memory/receipts.log" && path != "records/narrator-digest.log" {
			continue
		}
		_, exists := f.trees[f.landingTree][path]
		_, present := f.trees[f.rebasedTree][path]
		if !exists || !present {
			continue
		}
		f.addRoot([]string{"ls-files", "--stage", "-z"}, f.indexRecords())
		f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"read-tree", "--empty"} }, index: true, reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{} }})
		f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
		f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"update-index", "-z", "--index-info"} }, index: true, stdin: []byte(f.indexRecords()), reply: func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{} }})
		f.steps = append(f.steps, advanceRawStep{dir: func() string { return f.root }, args: func() []string { return []string{"write-tree"} }, index: true, reply: func(gittree.RawRequest) gittree.RawResult {
			return gittree.RawResult{Stdout: []byte(advanceTreeID(f.index) + "\n")}
		}})
		indexTree := advanceTreeID(f.index)
		f.trees[indexTree] = advanceCopy(f.index)
		f.addRoot([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", indexTree, "--", path}, f.entries(indexTree, []string{path}))
		f.addRoot([]string{"cat-file", "blob", advanceID("blob " + f.index[path])}, f.index[path])
	}
}
func (f *advanceRawFixture) swap() {
	f.addReply(func() string { return f.root }, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{Stdout: []byte(f.head + "\n")} })
	f.addRoot([]string{"symbolic-ref", "--quiet", "HEAD"}, f.branch+"\n")
}
func (f *advanceRawFixture) reset() {
	f.addRoot([]string{"rev-parse", "--show-toplevel"}, f.root+"\n")
	f.addReply(func() string { return f.root }, []string{"reset", "--keep", f.rebased}, func(gittree.RawRequest) gittree.RawResult {
		f.head = f.rebased
		f.index = advanceCopy(f.trees[f.rebasedTree])
		for path, data := range f.index {
			if path != "memory/receipts.log" && path != "records/narrator-digest.log" {
				f.edit(path, data)
			}
		}
		return gittree.RawResult{}
	})
	f.addReply(func() string { return f.root }, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, func(gittree.RawRequest) gittree.RawResult { return gittree.RawResult{Stdout: []byte(f.head + "\n")} })
}
func (f *advanceRawFixture) call() (string, error) {
	var out bytes.Buffer
	err := advanceWithWorkspace(f.root, advanceUpstream, &out, &out, f.workspace())
	return out.String(), err
}

func TestAdvanceLeavesRegistersUntouched(t *testing.T) {
	f := newAdvanceRawFixture(t)
	landing := advanceCopy(f.index)
	landing["scripts/agents/go-gate.sh"] += "landing\n"
	f.index = advanceCopy(landing)
	f.setLanding(landing)
	f.edit("scripts/agents/go-gate.sh", landing["scripts/agents/go-gate.sh"])
	rebased := advanceCopy(landing)
	rebased["product.txt"] += "peer\n"
	f.setRebased(rebased)
	f.edit("memory/receipts.log", "receipt=seed\nreceipt=local\n")
	f.edit("records/narrator-digest.log", "digest=seed\ndigest=local\n")
	receipts, _ := os.ReadFile(filepath.Join(f.root, "memory/receipts.log"))
	digest, _ := os.ReadFile(filepath.Join(f.root, "records/narrator-digest.log"))
	f.preflight(false, false)
	f.ancestor(false)
	f.detach(false)
	f.classify()
	f.swap()
	f.reset()
	out, err := f.call()
	if err != nil {
		t.Fatal(err)
	}
	if f.head == f.landing || !strings.Contains(out, "advance: "+f.landing+" -> "+f.rebased+"; registers untouched: memory/receipts.log, records/narrator-digest.log") {
		t.Fatalf("advance result = %q, HEAD %s", out, f.head)
	}
	for path, want := range map[string][]byte{"memory/receipts.log": receipts, "records/narrator-digest.log": digest} {
		got, err := os.ReadFile(filepath.Join(f.root, path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("register %s = %q, %v", path, got, err)
		}
	}
	wantStatus := []gittree.StatusEntry{
		{Index: ' ', Worktree: 'M', Path: "memory/receipts.log"},
		{Index: ' ', Worktree: 'M', Path: "records/narrator-digest.log"},
	}
	if got := f.observeStatus(); !reflect.DeepEqual(got, wantStatus) {
		t.Fatalf("dirty status = %+v", got)
	}
	f.preflight(false, false)
	f.ancestor(true)
	out, err = f.call()
	if err != nil || !strings.Contains(out, "advance: up to date with "+advanceUpstream) {
		t.Fatalf("second advance = %q, %v", out, err)
	}
	f.finish()
}
func TestAdvanceRefusesContendedRegister(t *testing.T) {
	f := newAdvanceRawFixture(t)
	landing := advanceCopy(f.index)
	landing["scripts/agents/go-gate.sh"] += "landing\n"
	f.index = advanceCopy(landing)
	f.setLanding(landing)
	f.edit("scripts/agents/go-gate.sh", landing["scripts/agents/go-gate.sh"])
	rebased := advanceCopy(landing)
	rebased["records/narrator-digest.log"] += "digest=peer\n"
	f.setRebased(rebased)
	f.edit("records/narrator-digest.log", "digest=seed\ndigest=local\n")
	want, _ := os.ReadFile(filepath.Join(f.root, "records/narrator-digest.log"))
	f.preflight(false, false)
	f.ancestor(false)
	f.detach(false)
	f.classify()
	_, err := f.call()
	if err == nil || !strings.Contains(err.Error(), "advance refused: advance-register-contended: records/narrator-digest.log") || !regexp.MustCompile(`[0-9a-f]{40,64}`).MatchString(err.Error()) {
		t.Fatalf("contended advance = %v", err)
	}
	if f.head != f.landing {
		t.Fatalf("refusal moved HEAD to %s", f.head)
	}
	got, readErr := os.ReadFile(filepath.Join(f.root, "records/narrator-digest.log"))
	if readErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("digest = %q, %v", got, readErr)
	}
	wantStatus := []gittree.StatusEntry{{Index: ' ', Worktree: 'M', Path: "records/narrator-digest.log"}}
	if got := f.observeStatus(); !reflect.DeepEqual(got, wantStatus) {
		t.Fatalf("contended status = %+v", got)
	}
	f.finish()
}
func TestAdvanceRefusals(t *testing.T) {
	t.Run("unstaged drift", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		landing := advanceCopy(f.index)
		landing["scripts/agents/go-gate.sh"] += "landing\n"
		f.index = advanceCopy(landing)
		f.setLanding(landing)
		f.edit("scripts/agents/go-gate.sh", landing["scripts/agents/go-gate.sh"])
		rebased := advanceCopy(landing)
		rebased["product.txt"] = "peer\n"
		f.setRebased(rebased)
		f.edit("product.txt", "dirty\n")
		f.preflight(false, false)
		f.ancestor(false)
		f.detach(false)
		f.classify()
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-unstaged-drift")
		data, _ := os.ReadFile(filepath.Join(f.root, "product.txt"))
		if f.head != f.landing || string(data) != "dirty\n" {
			t.Fatalf("refusal changed HEAD/product: %s, %q", f.head, data)
		}
		f.finish()
	})
	t.Run("rebase conflict", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		f.preflight(false, false)
		f.ancestor(false)
		f.detach(true)
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-rebase-conflict")
		if f.head != f.landing {
			t.Fatalf("conflict moved HEAD to %s", f.head)
		}
		f.finish()
	})
	t.Run("register removed", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		rebased := advanceCopy(f.index)
		delete(rebased, "records/narrator-digest.log")
		f.setRebased(rebased)
		f.edit("records/narrator-digest.log", "digest=seed\ndigest=local\n")
		f.preflight(false, false)
		f.ancestor(false)
		f.detach(false)
		f.classify()
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-register-removed")
		f.finish()
	})
	t.Run("staged path", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		f.index["product.txt"] = "staged\n"
		f.edit("product.txt", "staged\n")
		f.preflight(false, true)
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-index-not-empty")
		f.finish()
	})
	t.Run("detached head", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		f.preflight(true, false)
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-not-on-branch")
		f.finish()
	})
	t.Run("head moved", func(t *testing.T) {
		f := newAdvanceRawFixture(t)
		rebased := advanceCopy(f.index)
		rebased["product.txt"] += "peer\n"
		f.setRebased(rebased)
		f.preflight(false, false)
		f.ancestor(false)
		f.detach(false)
		f.classify()
		f.swap()
		advanceBeforeSwap = func() { f.head = f.moved }
		t.Cleanup(func() { advanceBeforeSwap = nil })
		_, err := f.call()
		assertAdvanceRefusal(t, err, "advance-head-moved")
		if !strings.Contains(err.Error(), f.landing) || f.head != f.moved {
			t.Fatalf("head-moved refusal = %v, HEAD %s", err, f.head)
		}
		f.finish()
	})
}
