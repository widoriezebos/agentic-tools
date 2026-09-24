package landing

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

var (
	observeBaseTree = strings.Repeat("a", 40)
	observeTreeB    = strings.Repeat("b", 40)
	observeTreeC    = strings.Repeat("c", 40)
	observeBaseBlob = strings.Repeat("1", 40)
	observeNextBlob = strings.Repeat("2", 40)
)

type observationKey struct{ tree, path string }

type observationFile struct {
	data    []byte
	present bool
}

type observationEntriesKey struct{ tree, requestedPaths string }

func entriesKey(tree string, paths []string) observationEntriesKey {
	return observationEntriesKey{tree: tree, requestedPaths: strings.Join(paths, "\x00")}
}

// The fixture materializes the installation layout used by the real owner
// resolver. Tree contents and comparisons are separately declared facts.
type repositoryObservationFixture struct {
	t          *testing.T
	repository string
	root       string
	baseFiles  map[string][]byte
}

func newRepositoryObservationFixture(t *testing.T) *repositoryObservationFixture {
	return newRepositoryObservationFixtureAt(t, false)
}

func newAdoptedRepositoryObservationFixture(t *testing.T) *repositoryObservationFixture {
	return newRepositoryObservationFixtureAt(t, true)
}

func newRepositoryObservationFixtureAt(t *testing.T, adopted bool) *repositoryObservationFixture {
	t.Helper()
	repository := t.TempDir()
	root := filepath.Join(repository, "metasystem")
	if adopted {
		root = repository
	}
	f := &repositoryObservationFixture{t: t, repository: repository, root: root, baseFiles: map[string][]byte{}}
	if !adopted {
		f.writeOutside("development/metasystem-design.md", "fixture\n")
	}
	f.base(".gitignore", "artifacts/\n")
	f.base("product.txt", "before\n")
	for _, policy := range []string{"path-classes.txt", "landing-classes.json"} {
		path := "scripts/agents/" + policy
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", policy))
		if err != nil {
			t.Fatalf("read fixture policy %s: %v", path, err)
		}
		f.base(path, string(data))
	}
	f.base("memory/rulings.md", "| R-1 | existing ruling |\n| R-35-m0 | landing class authority |\n| R-54-m1 | tier-1 landing authority |\n")
	f.base("memory/receipts.log", "receipt=existing\n")
	f.base("records/narrator-digest.log", "digest=existing\n")
	canonicalRepository, err := filepath.EvalSymlinks(f.repository)
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.repository, f.root = canonicalRepository, canonicalRoot
	return f
}

func (f *repositoryObservationFixture) writeOutside(path, content string) {
	f.t.Helper()
	file := filepath.Join(f.repository, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *repositoryObservationFixture) write(path, content string) {
	f.t.Helper()
	file := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *repositoryObservationFixture) base(path, content string) {
	f.write(path, content)
	f.baseFiles[path] = []byte(content)
}

// observationCase fixes one base/candidate comparison. Unlisted reads fail,
// and all returned slices and maps are copies of its immutable facts.
type observationCase struct {
	fixture    *repositoryObservationFixture
	candidate  string
	diff       []byte
	changed    []string
	files      map[observationKey]observationFile
	entries    map[observationEntriesKey]map[string]gittree.Entry
	sealed     bool
	exact      *exactObservationFacts
	chain      *chainObservationFacts
	receiptRaw func(gittree.RawRequest) gittree.RawResult
	diffRaw    func(string, ...string) ([]byte, error)
}

// An exact case permits only its declared reads and checks that every read
// needed to decide the outcome came from this case.
type exactObservationFacts struct {
	targetPaths                 []string
	preimageTree, postimageTree string
	parentCommit, revertCommit  string
	expected, consumed          map[string]int
}

func (c *observationCase) exactRevert(targetPaths ...string) {
	c.fixture.t.Helper()
	if c.sealed || c.exact != nil {
		c.fixture.t.Fatal("exact facts changed after use or declared twice")
	}
	c.exact = &exactObservationFacts{
		targetPaths:  append([]string(nil), targetPaths...),
		preimageTree: strings.Repeat("d", 40), postimageTree: strings.Repeat("e", 40),
		parentCommit: strings.Repeat("f", 40), revertCommit: strings.Repeat("0", 40),
		expected: map[string]int{}, consumed: map[string]int{},
	}
	c.expectExact("head", 3)
	c.expectExact("diff:"+observeBaseTree+":"+c.candidate, 1)
	c.expectExact("paths:"+observeBaseTree+":"+c.candidate, 1)
	c.expectExact("parent:"+c.exact.revertCommit, 1)
	c.expectExact("tree:"+c.exact.parentCommit, 1)
	c.expectExact("tree:"+c.exact.revertCommit, 1)
	c.expectExact("paths:"+c.exact.preimageTree+":"+c.exact.postimageTree, 1)
	c.expectExact("file:"+observeBaseTree+":scripts/agents/landing-classes.json", 1)
	c.expectExact("file:"+observeBaseTree+":memory/rulings.md", 1)
	c.expectExact("file:"+observeBaseTree+":"+"scripts/agents/path-classes.txt", 1)
	c.expectExact("prefix", 1)
	for _, path := range exactRevertPaths(c.changed, targetPaths) {
		c.expectExact("owner:"+path, 1)
	}
}

func (c *observationCase) revertCommit() string { return c.exact.revertCommit }

func (c *observationCase) expectExact(key string, count int) {
	c.exact.expected[key] += count
}

func (c *observationCase) consumeExact(key string) {
	if c.exact == nil && c.chain == nil {
		return
	}
	c.fixture.t.Helper()
	if c.chain != nil {
		if c.chain.expected[key] == c.chain.consumed[key] {
			c.fixture.t.Fatalf("undeclared or repeated chain observation call %q", key)
		}
		c.chain.consumed[key]++
		return
	}
	if c.exact.expected[key] == c.exact.consumed[key] {
		c.fixture.t.Fatalf("undeclared or repeated exact observation call %q", key)
	}
	c.exact.consumed[key]++
}

func (c *observationCase) verifyExactConsumption() {
	c.fixture.t.Helper()
	if c.chain != nil {
		for key, expected := range c.chain.expected {
			if got := c.chain.consumed[key]; got != expected {
				c.fixture.t.Errorf("chain observation call %q: got %d, want %d", key, got, expected)
			}
		}
		return
	}
	for key, expected := range c.exact.expected {
		if got := c.exact.consumed[key]; got != expected {
			c.fixture.t.Errorf("exact observation call %q: got %d, want %d", key, got, expected)
		}
	}
}

// The current base carries the original commit's postimage, and the candidate
// carries its preimage. Policy may refuse before comparing entries.
func (c *observationCase) declareInverseEntries(path string, before, after *string) {
	c.fixture.t.Helper()
	for _, state := range []struct {
		tree  string
		value *string
		oid   string
	}{
		{c.exact.preimageTree, before, observeBaseBlob},
		{c.exact.postimageTree, after, observeNextBlob},
		{observeBaseTree, after, observeNextBlob},
		{c.candidate, before, observeBaseBlob},
	} {
		entries := map[string]gittree.Entry{}
		if state.value != nil {
			entries[path] = gittree.Entry{Mode: "100644", OID: state.oid}
		}
		c.declareEntries(state.tree, []string{path}, entries)
	}
}

func (f *repositoryObservationFixture) comparison(candidate, rawDiff string, paths ...string) *observationCase {
	f.t.Helper()
	if candidate != observeTreeB && candidate != observeTreeC {
		f.t.Fatalf("undeclared symbolic candidate tree %q", candidate)
	}
	c := &observationCase{
		fixture: f, candidate: candidate, diff: []byte(rawDiff), changed: append([]string(nil), paths...),
		files: map[observationKey]observationFile{}, entries: map[observationEntriesKey]map[string]gittree.Entry{},
	}
	for path, data := range f.baseFiles {
		c.files[observationKey{observeBaseTree, path}] = observationFile{data: append([]byte(nil), data...), present: true}
	}
	return c
}

func (c *observationCase) declare(path string, before, after *string) {
	c.fixture.t.Helper()
	if c.sealed {
		c.fixture.t.Fatal("observation facts changed after use")
	}
	for _, state := range []struct {
		tree    string
		value   *string
		blobOID string
	}{{observeBaseTree, before, observeBaseBlob}, {c.candidate, after, observeNextBlob}} {
		key := observationKey{state.tree, path}
		if state.value == nil {
			c.files[key] = observationFile{}
			c.declareEntries(state.tree, []string{path}, nil)
		} else {
			c.files[key] = observationFile{data: []byte(*state.value), present: true}
			if c.chain != nil {
				state.blobOID = chainBlobOID([]byte(*state.value))
			}
			c.declareEntries(state.tree, []string{path}, map[string]gittree.Entry{
				path: {Mode: "100644", OID: state.blobOID},
			})
		}
	}
	if after != nil {
		c.fixture.write(path, *after)
	}
}

func (c *observationCase) declareEntries(tree string, paths []string, entries map[string]gittree.Entry) {
	c.fixture.t.Helper()
	if c.sealed {
		c.fixture.t.Fatal("observation facts changed after use")
	}
	if tree != observeBaseTree && tree != c.candidate &&
		(c.exact == nil || tree != c.exact.preimageTree && tree != c.exact.postimageTree) &&
		(c.chain == nil || tree != c.chain.expectedTree && tree != c.chain.reviewedTree) {
		c.fixture.t.Fatalf("entry response has unexpected tree %q", tree)
	}
	copyOfEntries := make(map[string]gittree.Entry, len(entries))
	for path, entry := range entries {
		copyOfEntries[path] = entry
	}
	c.entries[entriesKey(tree, paths)] = copyOfEntries
}

func observationText(content string) *string { return &content }

func observationHeldGoal(id, machine, lineage string) []byte {
	return goal.RenderFile(&goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Exercise record carriage.", OpenedAt: "2026-09-03T08:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: "2026-09-03T08:01:00Z", Revision: 1},
		History: []goal.HistoryLine{{
			At: "2026-09-03T08:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001",
			Verb: "claim", Actor: machine + "+" + lineage, Targets: []string{id}, Keep: -1,
		}},
	})
}

func (c *observationCase) observe(params ObserveParams) Observation {
	c.fixture.t.Helper()
	c.sealed = true
	if params.RepoRoot != c.fixture.root || params.CandidateTree != c.candidate {
		c.fixture.t.Fatalf("observation root/tree mismatch: root=%q tree=%q", params.RepoRoot, params.CandidateTree)
	}
	for _, path := range c.changed {
		for _, tree := range []string{observeBaseTree, c.candidate} {
			key := observationKey{tree, path}
			if _, ok := c.files[key]; !ok {
				c.fixture.t.Fatalf("changed path %q has no declared file fact in %s", path, tree)
			}
			if _, ok := c.entries[entriesKey(tree, []string{path})]; !ok {
				c.fixture.t.Fatalf("changed path %q has no declared entry fact in %s", path, tree)
			}
		}
	}
	resolver := stateroot.NewResolver(func(path string) (string, error) {
		if path != c.fixture.root {
			c.fixture.t.Fatalf("unexpected repository-top lookup %q", path)
		}
		return c.fixture.repository, nil
	}, func() (string, error) {
		c.fixture.t.Fatal("owner resolver requested executable path")
		return "", fmt.Errorf("unexpected executable lookup")
	})
	owner := func(installation, path string) (stateroot.Ownership, string, error) {
		if installation != c.fixture.root {
			c.fixture.t.Fatalf("unexpected installation root %q", installation)
		}
		if c.exact != nil || c.chain != nil {
			prefix := "metasystem/"
			if c.fixture.root == c.fixture.repository {
				prefix = ""
			}
			if !strings.HasPrefix(path, prefix) || path == "" {
				c.fixture.t.Fatalf("unexpected ownership path %q", path)
			}
			c.consumeExact("owner:" + strings.TrimPrefix(path, prefix))
		}
		return resolver.OwnerForInstallation(installation, path)
	}
	facts := observationFacts{reader: c, installation: c.fixture.root, ownerForInstallation: owner}
	facts.receiptRawSource = c.receiptRaw
	facts.diffCommand = c.diffRaw
	if c.chain != nil {
		facts.apply = c.Apply
	}
	if c.exact != nil {
		facts.singleParent = c.SingleParent
		facts.treeOf = c.TreeOf
	}
	result := observeWithFacts(params, facts)
	if c.exact != nil || c.chain != nil {
		c.verifyExactConsumption()
	}
	return result
}

// The receipt validator receives only the raw tree and entry responses its
// command-receipt path needs. Every other request is a fixture error.
func (c *observationCase) bindCommandReceipt(command string) string {
	c.fixture.t.Helper()
	identity := strings.Repeat("9", 40)
	entryLines := c.receiptEntries()
	indexModes := map[string]string{}
	c.receiptRaw = func(request gittree.RawRequest) gittree.RawResult {
		c.fixture.t.Helper()
		args := request.Args
		if len(args) < 3 || args[0] != "-C" || args[1] != request.Dir {
			c.fixture.t.Fatalf("malformed raw receipt request: %+v", request)
		}
		for len(args) > 2 && args[2] == "-c" {
			if len(args) < 5 {
				c.fixture.t.Fatalf("malformed Git pins: %q", args)
			}
			args = append(args[:2:2], args[4:]...)
		}
		operation := args[2:]
		index := ""
		for _, entry := range request.Env {
			if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
				index = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
			}
		}
		if request.Dir != c.fixture.root && request.Dir != c.fixture.repository {
			c.fixture.t.Fatalf("unexpected raw receipt directory %q", request.Dir)
		}
		answer := func(value string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(value)} }
		switch {
		case slices.Equal(operation, []string{"rev-parse", "--show-prefix"}) && request.Dir == c.fixture.root && index == "":
			return answer("metasystem/\n")
		case slices.Equal(operation, []string{"rev-parse", "--show-toplevel"}) && request.Dir == c.fixture.root && index == "":
			return answer(c.fixture.repository + "\n")
		case slices.Equal(operation, []string{"rev-parse", c.candidate + ":metasystem"}) && request.Dir == c.fixture.root && index == "":
			return answer(c.candidate + "\n")
		case slices.Equal(operation, []string{"ls-files", "--stage", "-z"}) && index == "" && request.Dir == c.fixture.root:
			return gittree.RawResult{Stdout: append([]byte(nil), entryLines...)}
		case slices.Equal(operation, []string{"read-tree", "--empty"}) && index != "" && indexModes[index] == "" && request.Dir == c.fixture.root:
			indexModes[index] = "stage-seeded"
			return gittree.RawResult{}
		case slices.Equal(operation, []string{"update-index", "-z", "--index-info"}) && indexModes[index] == "stage-seeded" && bytes.Equal(request.Stdin, entryLines) && request.Dir == c.fixture.repository:
			indexModes[index] = "stage-ready"
			return gittree.RawResult{}
		case slices.Equal(operation, []string{"read-tree", "HEAD"}) && index != "" && indexModes[index] == "" && request.Dir == c.fixture.root:
			indexModes[index] = "snapshot-seeded"
			return gittree.RawResult{}
		case slices.Equal(operation, []string{"add", "-A", "--", "."}) && indexModes[index] == "snapshot-seeded" && request.Dir == c.fixture.root:
			indexModes[index] = "snapshot-ready"
			return gittree.RawResult{}
		case slices.Equal(operation, []string{"read-tree", c.candidate}) && index != "" && indexModes[index] == "" && request.Dir == c.fixture.root:
			indexModes[index] = "filter-seeded"
			return gittree.RawResult{}
		case len(operation) >= 3 && slices.Equal(operation[:3], []string{"update-index", "--force-remove", "--"}) && slices.Equal(operation[3:], appendOnlyRegisters) && indexModes[index] == "filter-seeded" && request.Dir == c.fixture.repository:
			indexModes[index] = "filter-ready"
			return gittree.RawResult{}
		case slices.Equal(operation, []string{"write-tree"}) && indexModes[index] == "filter-ready" && request.Dir == c.fixture.root:
			indexModes[index] = "done"
			return answer(identity + "\n")
		case slices.Equal(operation, []string{"write-tree"}) && (indexModes[index] == "stage-ready" || indexModes[index] == "snapshot-ready") && request.Dir == c.fixture.root:
			indexModes[index] = "done"
			return answer(c.candidate + "\n")
		}
		c.fixture.t.Fatalf("undeclared raw receipt request: %q, stdin=%q", request.Operation, request.Stdin)
		return gittree.RawResult{}
	}
	receipt := TestReceipt{SchemaVersion: 3, Tree: c.candidate, Command: command, ExitStatus: 0,
		Time: "2026-09-24T08:00:00Z", Binding: filteredReceiptBinding(c.candidate, identity),
		WorktreeProjection: &TestReceiptProjection{Excludes: AppendOnlyRegisters(), Tree: identity}}
	writeTestReceiptFixture(c.fixture.t, c.fixture.root, c.candidate, receipt)
	return TestReceiptPath(c.fixture.root, c.candidate)
}

func (c *observationCase) receiptEntries() []byte {
	c.fixture.t.Helper()
	paths := map[string]observationFile{}
	for path, data := range c.fixture.baseFiles {
		paths[path] = observationFile{data: data, present: true}
	}
	for key, fact := range c.files {
		if key.tree == c.candidate {
			paths[key.path] = fact
		}
	}
	names := make([]string, 0, len(paths))
	for name, fact := range paths {
		if fact.present {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var output bytes.Buffer
	for _, name := range names {
		entry := gittree.Entry{Mode: "100644", OID: chainBlobOID(paths[name].data)}
		if declared, ok := c.entries[entriesKey(c.candidate, []string{name})][name]; ok {
			entry = declared
		}
		fmt.Fprintf(&output, "%s %s 0\t%s\x00", entry.Mode, entry.OID, name)
	}
	return output.Bytes()
}

func (c *observationCase) HeadTree() (string, error) {
	c.consumeExact("head")
	return observeBaseTree, nil
}

func (c *observationCase) SingleParent(commit string) (string, error) {
	c.consumeExact("parent:" + commit)
	return c.exact.parentCommit, nil
}

func (c *observationCase) TreeOf(commit string) (string, error) {
	c.consumeExact("tree:" + commit)
	if commit == c.exact.parentCommit {
		return c.exact.preimageTree, nil
	}
	return c.exact.postimageTree, nil
}

func (c *observationCase) Diff(from, to string) ([]byte, error) {
	c.consumeExact("diff:" + from + ":" + to)
	if from != observeBaseTree || to != c.candidate {
		c.fixture.t.Fatalf("unexpected diff %q -> %q", from, to)
	}
	return append([]byte(nil), c.diff...), nil
}

func (c *observationCase) ChangedPaths(from, to string) ([]string, error) {
	c.consumeExact("paths:" + from + ":" + to)
	if c.chain != nil {
		if paths, ok := c.chain.paths[observationTreePair{from, to}]; ok {
			return append([]string(nil), paths...), nil
		}
	}
	if c.exact != nil && from == c.exact.preimageTree && to == c.exact.postimageTree {
		return append([]string(nil), c.exact.targetPaths...), nil
	}
	if from != observeBaseTree || to != c.candidate {
		c.fixture.t.Fatalf("unexpected changed-path comparison %q -> %q", from, to)
	}
	return append([]string(nil), c.changed...), nil
}

func (c *observationCase) FileAt(tree, path string) ([]byte, bool, error) {
	c.consumeExact("file:" + tree + ":" + path)
	fact, ok := c.files[observationKey{tree, path}]
	if !ok {
		c.fixture.t.Fatalf("undeclared file read %q:%q", tree, path)
	}
	return append([]byte(nil), fact.data...), fact.present, nil
}

func (c *observationCase) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if c.exact != nil || c.chain != nil {
		c.consumeExact("entries:" + tree + ":" + strings.Join(paths, "\x00"))
	}
	facts, ok := c.entries[entriesKey(tree, paths)]
	if !ok {
		c.fixture.t.Fatalf("undeclared entries request %q:%v", tree, paths)
	}
	result := make(map[string]gittree.Entry, len(facts))
	for path, entry := range facts {
		result[path] = entry
	}
	return result, nil
}

func (c *observationCase) Prefix() (string, error) {
	c.consumeExact("prefix")
	if c.fixture.root == c.fixture.repository {
		return "", nil
	}
	return "metasystem/", nil
}
