package landing

import (
	"fmt"
	"os"
	"path/filepath"
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
	t.Helper()
	repository := t.TempDir()
	root := filepath.Join(repository, "metasystem")
	f := &repositoryObservationFixture{t: t, repository: repository, root: root, baseFiles: map[string][]byte{}}
	f.writeOutside("development/metasystem-design.md", "fixture\n")
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
	fixture   *repositoryObservationFixture
	candidate string
	diff      []byte
	changed   []string
	files     map[observationKey]observationFile
	entries   map[observationEntriesKey]map[string]gittree.Entry
	sealed    bool
	exact     *exactObservationFacts
	chain     *chainObservationFacts
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
			if !strings.HasPrefix(path, prefix) {
				c.fixture.t.Fatalf("unexpected ownership path %q", path)
			}
			c.consumeExact("owner:" + strings.TrimPrefix(path, prefix))
		}
		return resolver.OwnerForInstallation(installation, path)
	}
	facts := observationFacts{reader: c, installation: c.fixture.root, ownerForInstallation: owner}
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
	return "metasystem/", nil
}
