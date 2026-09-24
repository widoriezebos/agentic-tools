package branch

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

// These facts describe immutable commits and snapshots. Every repository read
// uses the same graph, so an attestation cannot cite a different version of a path.
type landingFact struct {
	id, parent, tree string
	kind             Kind
	unit             string
	files            map[string][]byte
	raw              []byte
}

type landingFacts struct {
	t                *testing.T
	repo             string
	nodes            map[string]landingFact
	trees            map[string]map[string][]byte
	base             string
	events           []string
	next             int
	remote           string
	scratch          string
	scratchEndpoint  string
	scratchHead      string
	staged           map[string][]byte
	attributeFiles   map[string][]byte
	messages         map[string][]byte
	dates            map[string]string
	stamp            string
	composition      bool
	lastIndex        string
	units            []string
	folds            [][]string
	ancestorError    error
	firstParentError error
	kindError        error
	kindErrorAt      string
}

func newLandingFacts(t *testing.T, baseFiles map[string]string) *landingFacts {
	t.Helper()
	f := &landingFacts{t: t, repo: filepath.Join(t.TempDir(), "source"), nodes: map[string]landingFact{}, trees: map[string]map[string][]byte{}, messages: map[string][]byte{}, dates: map[string]string{}}
	files := map[string][]byte{}
	for path, body := range baseFiles {
		files[path] = []byte(body)
	}
	f.base = f.add("", "base", "", files)
	f.writeFiles(f.repo, f.nodes[f.base].files)
	return f
}

func factHash(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func blobHash(data []byte) string {
	return factHash(append([]byte(fmt.Sprintf("blob %d\x00", len(data))), data...))
}

func cloneFiles(files map[string][]byte) map[string][]byte {
	copy := make(map[string][]byte, len(files))
	for path, data := range files {
		copy[path] = append([]byte(nil), data...)
	}
	return copy
}

func landingSortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func landingTree(files map[string][]byte) string {
	var text strings.Builder
	for _, path := range landingSortedPaths(files) {
		fmt.Fprintf(&text, "%s %s\n", path, blobHash(files[path]))
	}
	return factHash([]byte(text.String()))
}

func landingRaw(before, files map[string][]byte) []byte {
	var raw []byte
	all := cloneFiles(before)
	for path, data := range files {
		all[path] = data
	}
	for _, path := range landingSortedPaths(all) {
		old, oldOK := before[path]
		newData, newOK := files[path]
		if oldOK && newOK && bytes.Equal(old, newData) {
			continue
		}
		oldMode, newMode := "000000", "000000"
		oldBlob, newBlob := strings.Repeat("0", 40), strings.Repeat("0", 40)
		status := "M"
		if oldOK {
			oldMode, oldBlob = "100644", blobHash(old)
		} else {
			status = "A"
		}
		if newOK {
			newMode, newBlob = "100644", blobHash(newData)
		} else {
			status = "D"
		}
		raw = append(raw, []byte(fmt.Sprintf(":%s %s %s %s %s\x00%s\x00", oldMode, newMode, oldBlob, newBlob, status, path))...)
	}
	return raw
}

func (f *landingFacts) add(parent, label, unit string, changes map[string][]byte) string {
	f.t.Helper()
	files := map[string][]byte{}
	if parent != "" {
		files = cloneFiles(f.nodes[parent].files)
	}
	for path, data := range changes {
		if data == nil {
			delete(files, path)
		} else {
			files[path] = append([]byte(nil), data...)
		}
	}
	tree := landingTree(files)
	before := map[string][]byte{}
	if parent != "" {
		before = f.nodes[parent].files
	}
	raw := landingRaw(before, files)
	id := factHash([]byte(parent + "\n" + label + "\n" + tree))
	if prior, exists := f.nodes[id]; exists {
		if prior.parent != parent || !reflect.DeepEqual(prior.files, files) {
			f.t.Fatalf("fact ID reused: %s", id)
		}
		return id
	}
	f.nodes[id] = landingFact{id: id, parent: parent, tree: tree, kind: Kind(label), unit: unit, files: files, raw: raw}
	f.trees[tree] = cloneFiles(files)
	return id
}

func (f *landingFacts) plan(parent, path, body string) string {
	return f.add(parent, string(Plan), "", map[string][]byte{path: []byte(body)})
}

func (f *landingFacts) unit(parent, name, path, body string) string {
	return f.add(parent, string(Unit), name, map[string][]byte{path: []byte(body)})
}

func (f *landingFacts) chain(endpoint, tip string) []landingFact {
	f.t.Helper()
	ancestors := map[string]bool{}
	for id := endpoint; id != ""; id = f.nodes[id].parent {
		ancestors[id] = true
	}
	var reverse []landingFact
	for id := tip; id != "" && !ancestors[id]; id = f.nodes[id].parent {
		node, ok := f.nodes[id]
		if !ok {
			f.t.Fatalf("unknown graph node %s", id)
		}
		reverse = append(reverse, node)
	}
	if len(reverse) == 0 || !ancestors[reverse[len(reverse)-1].parent] {
		f.t.Fatalf("%s and %s have no declared common ancestor", endpoint, tip)
	}
	for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
		reverse[i], reverse[j] = reverse[j], reverse[i]
	}
	return reverse
}

func (f *landingFacts) subject(id string) readsubject.ReadSubject {
	node := f.nodes[id]
	sum := sha256.Sum256(node.raw)
	return readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: id, Parent: node.parent, Tree: node.tree, DiffDigest: hex.EncodeToString(sum[:])}
}

func (f *landingFacts) read(parent, unitID, unitName string) string {
	unit := f.nodes[unitID]
	subject := f.subject(unitID)
	unitDigest := digestRawEntries(unit.raw)
	var folds []Fold
	for _, node := range f.chain(f.base, unitID) {
		if node.kind != Unit && node.id != unitID {
			folds = append(folds, Fold{Kind: node.kind, Commit: node.id, Digest: digestRawEntries(node.raw)})
		}
	}
	recordPath := "metasystem/records/misc/goal-a-" + unitName + "-read.md"
	record := []byte("Read commit " + unitID + " with unit digest " + unitDigest + " and found it clean.\n")
	recordSum := sha256.Sum256(record)
	att := Attestation{SchemaVersion: 1, Goal: "goal-a", Unit: unitName,
		Subject: AttestationSubject{Commit: unitID, Parent: unit.parent, Tree: unit.tree, PatchDigest: subject.DiffDigest, UnitDigest: unitDigest},
		Source:  AttestationSource{Kind: "reader-record", ReaderRecord: recordPath, RecordSHA256: hex.EncodeToString(recordSum[:])},
		Verdict: "LAND", Gate: GateObservation{Kind: "go-gate-fast", Tree: unit.tree, RunID: "fast-" + unitName},
		Folds: folds, TestsChanged: []TestChange{}}
	var err error
	att.SHA256, err = digestAttestation(att)
	if err != nil {
		f.t.Fatal(err)
	}
	attBytes, err := json.MarshalIndent(att, "", "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	attBytes = append(attBytes, '\n')
	return f.add(parent, string(Read), unitName, map[string][]byte{attestationPath("goal-a", unitID): attBytes, recordPath: record})
}

func (f *landingFacts) expect(events ...string) { f.events = append(f.events, events...) }

func (f *landingFacts) call(event string) {
	f.t.Helper()
	if f.next >= len(f.events) || f.events[f.next] != event {
		f.t.Fatalf("repository call %d = %s; expected remaining %v", f.next, event, f.events[f.next:])
	}
	f.next++
}

func (f *landingFacts) consumed() {
	f.t.Helper()
	if f.next != len(f.events) {
		f.t.Fatalf("unconsumed repository calls: %v", f.events[f.next:])
	}
}

func (f *landingFacts) expectStatus(endpoint, tip string) {
	f.expect("claim", "human", "range:"+endpoint+":"+tip)
	for _, node := range f.chain(endpoint, tip) {
		if node.kind != Read {
			continue
		}
		var unit landingFact
		for id := node.parent; id != ""; id = f.nodes[id].parent {
			if f.nodes[id].kind == Unit && f.nodes[id].unit == node.unit {
				unit = f.nodes[id]
				break
			}
		}
		f.expect("kind:"+node.id, "file:"+tip+":"+attestationPath("goal-a", unit.id),
			"subject:"+unit.id, "raw:"+unit.id, "range:"+endpoint+":"+unit.id)
		for _, fold := range f.chain(endpoint, unit.id) {
			if fold.kind != Unit {
				f.expect("raw:" + fold.id)
			}
		}
		f.expect("raw:"+unit.id, "file:"+tip+":metasystem/records/misc/goal-a-"+node.unit+"-read.md")
	}
}

func (f *landingFacts) Range(repo, endpoint, tip, goal string) ([]Commit, error) {
	f.requireRepo(repo, goal)
	f.call("range:" + endpoint + ":" + tip)
	var commits []Commit
	for _, node := range f.chain(endpoint, tip) {
		item := Commit{ID: node.id, Kind: node.kind, Unit: node.unit}
		if node.kind == Unit {
			item.Units, item.Digest = strings.Split(node.unit, "+"), digestRawEntries(node.raw)
		} else if node.kind == Read {
			item.Units = strings.Split(node.unit, "+")
		}
		commits = append(commits, item)
	}
	return commits, nil
}

func (f *landingFacts) Kind(repo, commit, goal string) (KindInfo, error) {
	f.requireRepo(repo, goal)
	f.call("kind:" + commit)
	if f.kindError != nil && (f.kindErrorAt == "" || f.kindErrorAt == commit) {
		return KindInfo{}, f.kindError
	}
	node, ok := f.nodes[commit]
	if !ok {
		return KindInfo{}, fmt.Errorf("unknown commit %s", commit)
	}
	if node.kind == Unit || node.kind == Plan {
		return KindInfo{Kind: node.kind, Unit: node.unit, CommitID: commit}, nil
	}
	for id := node.parent; id != ""; id = f.nodes[id].parent {
		if f.nodes[id].kind == Unit && f.nodes[id].unit == node.unit {
			return KindInfo{Kind: Read, Unit: node.unit, Units: strings.Split(node.unit, "+"), CommitID: id}, nil
		}
	}
	f.t.Fatalf("read %s has no unit", commit)
	return KindInfo{}, nil
}

func (f *landingFacts) ReadSubject(repo, commit string) (readsubject.ReadSubject, error) {
	f.requireRepo(repo, "goal-a")
	f.call("subject:" + commit)
	if f.nodes[commit].kind != Unit {
		f.t.Fatalf("subject %s is not a unit", commit)
	}
	return f.subject(commit), nil
}

func (f *landingFacts) RawEntries(repo, commit string) ([]byte, error) {
	f.requireRepo(repo, "goal-a")
	f.call("raw:" + commit)
	return append([]byte(nil), f.nodes[commit].raw...), nil
}

func (f *landingFacts) SnapshotFile(repo, snapshot, path string) ([]byte, error) {
	f.requireRepo(repo, "goal-a")
	f.call("file:" + snapshot + ":" + path)
	data, present := f.nodes[snapshot].files[path]
	if !present {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (f *landingFacts) unsupported(name string)           { f.t.Fatalf("unexpected attestation method %s", name) }
func (f *landingFacts) TopLevel(string) (string, error)   { f.unsupported("TopLevel"); return "", nil }
func (f *landingFacts) CommitExists(string, string) error { f.unsupported("CommitExists"); return nil }
func (f *landingFacts) Prefix(string) (string, error)     { f.unsupported("Prefix"); return "", nil }
func (f *landingFacts) Transition(string, string, string) ([]byte, error) {
	f.unsupported("Transition")
	return nil, nil
}
func (f *landingFacts) TreeEntry(string, string, string) (string, error) {
	f.unsupported("TreeEntry")
	return "", nil
}

func (f *landingFacts) requireRepo(repo, goal string) {
	f.t.Helper()
	if (repo != f.repo && repo != filepath.Join(f.repo, "metasystem")) || goal != "goal-a" {
		f.t.Fatalf("repository arguments = %s %s", repo, goal)
	}
}

func (f *landingFacts) repository() landingRepository {
	return landingRepository{reads: f,
		human: func(repo, name string) ([]byte, error) {
			f.requireRepo(repo, "goal-a")
			f.call("human")
			if name == "Nobody" {
				return nil, os.ErrNotExist
			}
			if name != "Wido" {
				f.t.Fatalf("human = %s", name)
			}
			return []byte("Wido Approver <wido@example.invalid>"), nil
		},
		open: func(repo, base string) (string, func(), error) {
			f.requireRepo(repo, "goal-a")
			f.call("open:" + base)
			if _, known := f.nodes[base]; !known {
				f.t.Fatalf("scratch base %s is outside the graph", base)
			}
			f.scratch = filepath.Join(f.t.TempDir(), "scratch")
			f.scratchEndpoint = base
			f.scratchHead = base
			dir := filepath.Join(f.scratch, "worktree")
			f.writeFiles(dir, f.nodes[base].files)
			f.scratchHead = base
			return dir, func() {
				f.call("close")
				if err := os.RemoveAll(f.scratch); err != nil {
					f.t.Fatal(err)
				}
			}, nil
		},
		reset: func(dir, base string) error {
			f.call("reset:" + base)
			if dir != filepath.Join(f.scratch, "worktree") || base != f.scratchEndpoint {
				f.t.Fatalf("scratch reset = %s %s", dir, base)
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			f.writeFiles(dir, f.nodes[base].files)
			f.scratchHead = base
			f.staged = nil
			return nil
		},
		head: func(dir string) (string, error) {
			f.call("head")
			node := f.nodes[f.scratchHead]
			if got := f.readFiles(dir); !reflect.DeepEqual(got, node.files) {
				f.t.Fatal("scratch HEAD disagrees with graph snapshot")
			}
			return node.id, nil
		},
		index: func(dir string) (string, error) {
			f.call("index")
			tree, err := f.indexTree(dir)
			f.lastIndex = tree
			return tree, err
		},
		entry: func(repo, tree, path string) (string, string, bool, error) {
			f.requireRepo(repo, "goal-a")
			if f.composition {
				if tree != f.lastIndex {
					f.t.Fatalf("entry tree %s differs from latest index %s", tree, f.lastIndex)
				}
				f.call("entry:" + path)
			} else {
				f.call("entry:" + tree + ":" + path)
			}
			files, known := f.trees[tree]
			if !known {
				f.t.Fatalf("tree %s is outside the graph", tree)
			}
			data, present := files[path]
			if !present {
				return "", "", false, nil
			}
			return "100644", blobHash(data), true, nil
		},
		patch: f.patch,
		apply: f.apply,
		transition: func(repo, before, after string, scope landingScope) ([]byte, error) {
			f.call("transition")
			if repo != f.repo {
				f.scratchDir(repo)
			}
			oldFiles, newFiles := cloneFiles(f.snapshot(before)), cloneFiles(f.snapshot(after))
			if scope == landingContract {
				for path := range oldFiles {
					if path != contractgit.TestingContractPath {
						delete(oldFiles, path)
					}
				}
				for path := range newFiles {
					if path != contractgit.TestingContractPath {
						delete(newFiles, path)
					}
				}
			}
			if scope == landingWithoutContract {
				delete(oldFiles, contractgit.TestingContractPath)
				delete(newFiles, contractgit.TestingContractPath)
			}
			return landingRaw(oldFiles, newFiles), nil
		},
		message: func(repo, commit string) ([]byte, error) {
			f.requireRepo(repo, "goal-a")
			f.call("message")
			return append([]byte(nil), f.messages[commit]...), nil
		},
		stage:       f.stage,
		commit:      f.commit,
		tree:        func(dir string) (string, error) { f.call("tree"); return f.indexTree(dir) },
		exists:      f.exists,
		filterExact: func(repo, tree string, paths []string) (string, error) { return f.filter(repo, tree, paths, false) },
		ancestor: func(repo, ancestor, descendant string) error {
			f.requireRepo(repo, "goal-a")
			f.call("ancestor:" + ancestor + ":" + descendant)
			if f.ancestorError != nil {
				return f.ancestorError
			}
			if _, ok := f.nodes[ancestor]; !ok {
				return fmt.Errorf("unknown ancestor %s", ancestor)
			}
			for id := descendant; id != ""; id = f.nodes[id].parent {
				if _, ok := f.nodes[id]; !ok {
					return fmt.Errorf("unknown descendant %s", id)
				}
				if id == ancestor {
					return nil
				}
			}
			return fmt.Errorf("%s is not an ancestor of %s", ancestor, descendant)
		},
		firstParent: func(repo, from, to string) ([]byte, error) {
			f.requireRepo(repo, "goal-a")
			f.call("first-parent:" + from + ":" + to)
			if f.firstParentError != nil {
				return nil, f.firstParentError
			}
			if _, ok := f.nodes[from]; !ok {
				return nil, fmt.Errorf("unknown range start %s", from)
			}
			if _, ok := f.nodes[to]; !ok {
				return nil, fmt.Errorf("unknown range end %s", to)
			}
			excluded := map[string]bool{}
			for id := from; id != ""; id = f.nodes[id].parent {
				excluded[id] = true
			}
			var reverse []string
			for id := to; id != "" && !excluded[id]; id = f.nodes[id].parent {
				reverse = append(reverse, id)
			}
			var out strings.Builder
			for i := len(reverse) - 1; i >= 0; i-- {
				fmt.Fprintln(&out, reverse[i])
			}
			return []byte(out.String()), nil
		},
		trailers: func(repo, commit string) ([]byte, error) {
			f.call("trailers")
			return append([]byte(nil), f.messages[commit]...), nil
		},
		contracts:  f,
		projection: &landingProjectionFacts{f},
	}
}

func (f *landingFacts) endpointForScratch() string {
	return f.scratchEndpoint
}

func (f *landingFacts) writeFiles(root string, files map[string][]byte) {
	f.t.Helper()
	for path, data := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *landingFacts) readFiles(root string) map[string][]byte {
	f.t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)], err = os.ReadFile(path)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return files
}

func (f *landingFacts) RemoteTip(repo, remote, ref string) (string, bool, error) {
	f.requireRepo(repo, "goal-a")
	if remote != "origin" || ref != landingBranchRef("goal-a") {
		f.t.Fatalf("remote args = %s %s", remote, ref)
	}
	f.call("remote")
	return f.remote, f.remote != "", nil
}
func (f *landingFacts) Fetch(string, string, string, string) error {
	f.t.Fatal("unexpected fetch")
	return nil
}
func (f *landingFacts) Push(repo, remote, ref, expected, next string) (CASOutcome, error) {
	f.requireRepo(repo, "goal-a")
	if remote != "origin" || ref != landingBranchRef("goal-a") {
		f.t.Fatalf("push target = %s %s", remote, ref)
	}
	f.call("push")
	if f.remote != expected {
		return CASRefused, fmt.Errorf("remote moved from %s to %s", expected, f.remote)
	}
	if _, ok := f.nodes[next]; !ok {
		f.t.Fatalf("push commit %s is outside graph", next)
	}
	f.remote = next
	return CASLanded, nil
}

func (f *landingFacts) request(endpoint, tip, out, receipt string) LandRequest {
	return LandRequest{Repo: f.repo, Remote: "origin", EndpointTip: endpoint, BranchTip: tip,
		GoalID: "goal-a", Out: out, TestReceipt: receipt, Last: true, LandingReady: true,
		GoalPage: "ready", ApprovedBy: "human:Wido", Seat: "seat-a", PushTransport: f,
		CheckClaim: func() error { f.call("claim"); return nil }}
}

func requireLandingRefusal(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("refusal = %v, want %s", err, code)
	}
}

func TestGoalLandingLastRefusesUnitBeyondPrefix(t *testing.T) {
	t.Parallel()
	f := newLandingFacts(t, map[string]string{"base.txt": "base", "metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"})
	tip := f.plan(f.base, "metasystem/plans/goal-a.md", "approved goal plan\n")
	for i, body := range []string{"one\n", "two\n", "three\n"} {
		unitName := fmt.Sprintf("u%d", i+1)
		unit := f.unit(tip, unitName, "metasystem/"+[]string{"one.go", "two.go", "three.go"}[i], body)
		tip = f.read(unit, unit, unitName)
		if i == 1 {
			tip = f.plan(tip, "metasystem/plans/between.md", "folded into u3\n")
		}
	}
	tip = f.plan(tip, "metasystem/plans/tail.md", "tail fold\n")
	tip = f.unit(tip, "u4", "metasystem/four.go", "four\n")
	f.writeFiles(f.repo, f.nodes[tip].files)
	before := f.readFiles(f.repo)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"schemaVersion":3,"tree":"0000000000000000000000000000000000000000","exitStatus":0,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":"attempt-deep"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "last")
	if _, err := prepareLanding(f.request(f.base, tip, out, receipt), landingRepository{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete repository was accepted: %v", err)
	}
	f.expectStatus(f.base, tip)
	_, err := prepareLanding(f.request(f.base, tip, out, receipt), f.repository())
	requireLandingRefusal(t, err, LandPartialCode)
	f.consumed()
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output after refusal: %v", err)
	}
	if f.remote != "" || f.scratch != "" {
		t.Fatalf("prefix refusal reached remote or scratch: %q %q", f.remote, f.scratch)
	}
	if got := f.readFiles(f.repo); !reflect.DeepEqual(got, before) {
		t.Fatal("source changed on prefix refusal")
	}
}

func TestGoalLandingRefusesFoldWithStalePreimage(t *testing.T) {
	t.Parallel()
	f := newLandingFacts(t, map[string]string{"base.txt": "base", "metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n", "metasystem/plans/goal-a.md": "first\nsecond\n"})
	fold := f.plan(f.base, "metasystem/plans/goal-a.md", "branch first\nsecond\n")
	unit := f.unit(fold, "u1", "metasystem/code.go", "unit\n")
	tip := f.read(unit, unit, "u1")
	endpoint := f.add(f.base, "endpoint", "", map[string][]byte{"metasystem/plans/goal-a.md": []byte("first\nendpoint second\n")})
	f.writeFiles(f.repo, f.nodes[tip].files)
	before := f.readFiles(f.repo)
	remoteBefore := f.remote
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"schemaVersion":3,"tree":"0000000000000000000000000000000000000000","exitStatus":0,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":"stale-fold"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "stale-fold-out")
	f.expectStatus(endpoint, tip)
	f.expect("remote", "open:"+endpoint, "reset:"+endpoint, "head", "index", "raw:"+fold, "entry:"+f.nodes[endpoint].tree+":metasystem/plans/goal-a.md", "close")
	_, err := prepareLanding(f.request(endpoint, tip, out, receipt), f.repository())
	requireLandingRefusal(t, err, UnitRereadCode)
	if !strings.Contains(err.Error(), "has stale preimage") || !strings.Contains(err.Error(), "metasystem/plans/goal-a.md") {
		t.Fatalf("stale fold refusal = %v", err)
	}
	f.consumed()
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output after refusal: %v", err)
	}
	if _, err := os.Stat(f.scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch after refusal: %v", err)
	}
	if f.remote != remoteBefore {
		t.Fatal("remote ref changed on refusal")
	}
	if got := f.readFiles(f.repo); !reflect.DeepEqual(got, before) {
		t.Fatal("source changed on stale fold refusal")
	}
}

func (f *landingFacts) snapshot(id string) map[string][]byte {
	f.t.Helper()
	if id == "HEAD" {
		id = f.scratchHead
	}
	if parent, ok := strings.CutSuffix(id, "^"); ok {
		id = f.nodes[parent].parent
	}
	if node, ok := f.nodes[id]; ok {
		return node.files
	}
	if files, ok := f.trees[id]; ok {
		return files
	}
	f.t.Fatalf("unknown snapshot %s", id)
	return nil
}

func (f *landingFacts) scratchDir(dir string) {
	f.t.Helper()
	if dir != filepath.Join(f.scratch, "worktree") {
		f.t.Fatalf("scratch directory = %s", dir)
	}
}

func (f *landingFacts) patch(repo, before, after string) ([]byte, error) {
	f.call("patch")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	return landingRaw(f.snapshot(before), f.snapshot(after)), nil
}

func (f *landingFacts) apply(dir string, patch []byte, threeWay bool) error {
	f.call("apply")
	f.scratchDir(dir)
	var source landingFact
	for _, node := range f.nodes {
		if node.kind != Unit && node.kind != Plan && node.kind != Read {
			continue
		}
		if bytes.Equal(node.raw, patch) {
			source = node
			break
		}
	}
	if source.id == "" {
		f.t.Fatal("patch has no declared source transition")
	}
	if threeWay != (source.kind == Unit) {
		f.t.Fatalf("three-way choice for %s = %t", source.id, threeWay)
	}
	files := f.readFiles(dir)
	parent := f.nodes[source.parent].files
	all := cloneFiles(parent)
	for path, data := range source.files {
		all[path] = data
	}
	for _, path := range landingSortedPaths(all) {
		old, oldOK := parent[path]
		newData, newOK := source.files[path]
		if oldOK && newOK && bytes.Equal(old, newData) {
			continue
		}
		current, currentOK := files[path]
		if path != contractgit.TestingContractPath && (oldOK != currentOK || oldOK && !bytes.Equal(current, old)) {
			return fmt.Errorf("patch preimage changed at %s", path)
		}
		if path == contractgit.TestingContractPath && newOK {
			merged, err := contractmerge.MergeBytes(old, current, newData)
			if err != nil {
				return err
			}
			files[path] = merged
			continue
		}
		if newOK {
			files[path] = append([]byte(nil), newData...)
		} else {
			delete(files, path)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	f.writeFiles(dir, files)
	return nil
}

func (f *landingFacts) indexTree(dir string) (string, error) {
	f.scratchDir(dir)
	files := f.readFiles(dir)
	tree := landingTree(files)
	f.trees[tree] = cloneFiles(files)
	return tree, nil
}

func (f *landingFacts) stage(dir string) error {
	f.call("stage")
	f.scratchDir(dir)
	f.staged = f.readFiles(dir)
	return nil
}

func (f *landingFacts) commit(dir string, who landingIdentity, stamp string, message []byte) error {
	f.call("commit")
	f.scratchDir(dir)
	if who != (landingIdentity{Name: "Wido Approver", Email: "wido@example.invalid"}) {
		f.t.Fatalf("landing identity = %+v", who)
	}
	if stamp != "1970-01-01T00:00:00Z" && stamp != f.stamp {
		f.t.Fatalf("commit date = %s, want %s", stamp, f.stamp)
	}
	if !reflect.DeepEqual(f.staged, f.readFiles(dir)) {
		f.t.Fatal("scratch changed after stage")
	}
	changes := cloneFiles(f.staged)
	for path := range f.nodes[f.scratchHead].files {
		if _, ok := changes[path]; !ok {
			changes[path] = nil
		}
	}
	f.scratchHead = f.add(f.scratchHead, string(Unit), "", changes)
	f.messages[f.scratchHead] = append([]byte(nil), message...)
	f.dates[f.scratchHead] = stamp
	return nil
}

func (f *landingFacts) exists(repo, snapshot, path string) error {
	f.requireRepo(repo, "goal-a")
	f.call("exists")
	if _, ok := f.snapshot(snapshot)[path]; !ok {
		return os.ErrNotExist
	}
	return nil
}

func (f *landingFacts) filter(repo, tree string, paths []string, prefixes bool) (string, error) {
	f.requireRepo(repo, "goal-a")
	if prefixes {
		f.call("filter-prefixes")
	} else {
		f.call("filter-exact")
	}
	if _, known := f.trees[tree]; !known {
		if _, known := f.nodes[tree]; !known {
			return "", fmt.Errorf("unknown tree or commit %s", tree)
		}
	}
	files := cloneFiles(f.snapshot(tree))
	for path := range files {
		for _, excluded := range paths {
			if path == excluded || prefixes && strings.HasPrefix(path, strings.TrimSuffix(excluded, "/")+"/") {
				delete(files, path)
				break
			}
		}
	}
	id := landingTree(files)
	f.trees[id] = files
	return id, nil
}

func (f *landingFacts) TopLevelForProjection(root string) (string, error) {
	f.call("top-level")
	if root != f.repo && root != filepath.Join(f.repo, "metasystem") {
		f.t.Fatalf("projection root = %s", root)
	}
	return f.repo, nil
}
func (f *landingFacts) PrefixForProjection(root string) (string, error) {
	f.call("prefix")
	if root == f.repo {
		return "", nil
	}
	if root == filepath.Join(f.repo, "metasystem") {
		return "metasystem/", nil
	}
	f.t.Fatalf("projection prefix root = %s", root)
	return "", nil
}

func (f *landingFacts) Paths(repo, before, after, onlyPath string) ([]byte, error) {
	f.call("contract-paths")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	raw := landingRaw(f.snapshot(before), f.snapshot(after))
	entries, err := parseRawEntries(after, raw)
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, entry := range entries {
		if onlyPath == "" || entry.Path == onlyPath {
			out = append(out, []byte(entry.Path)...)
			out = append(out, 0)
		}
	}
	return out, nil
}
func (f *landingFacts) Attributes(repo, tree string, paths []string) ([]byte, error) {
	f.call("attributes")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	files := f.snapshot(tree)
	var out []byte
	for _, path := range paths {
		value := "unspecified"
		if path == contractgit.TestingContractPath && bytes.Contains(files[".gitattributes"], []byte("merge=metasystem-testing")) {
			value = "metasystem-testing"
		}
		out = append(out, []byte(path+"\x00merge\x00"+value+"\x00")...)
	}
	return out, nil
}
func (f *landingFacts) OpenAttributeIndex() (string, func(), error) {
	f.call("attribute-open")
	index := filepath.Join(f.t.TempDir(), "attrs.index")
	f.attributeFiles = nil
	return index, func() { f.call("attribute-close"); f.attributeFiles = nil }, nil
}
func (f *landingFacts) LoadAttributeBase(repo, index, tree string) error {
	f.call("attribute-base")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	f.attributeFiles = cloneFiles(f.snapshot(tree))
	return nil
}
func (f *landingFacts) AttributeEntry(repo, index, commit, path string) ([]byte, error) {
	f.call("attribute-entry")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	data, ok := f.snapshot(commit)[path]
	if !ok {
		return nil, nil
	}
	return []byte(fmt.Sprintf("100644 blob %s\t%s\x00", blobHash(data), path)), nil
}
func (f *landingFacts) DropAttribute(repo, index, path string) error {
	f.call("attribute-drop")
	delete(f.attributeFiles, path)
	return nil
}
func (f *landingFacts) SetAttribute(repo, index, mode, blob, path string) error {
	f.call("attribute-set")
	if mode != "100644" {
		f.t.Fatalf("attribute mode = %s", mode)
	}
	for _, files := range f.trees {
		if data, ok := files[path]; ok && blobHash(data) == blob {
			f.attributeFiles[path] = data
			return nil
		}
	}
	f.t.Fatalf("attribute blob %s missing", blob)
	return nil
}
func (f *landingFacts) AttributeTree(repo, index string) (string, error) {
	f.call("attribute-tree")
	tree := landingTree(f.attributeFiles)
	f.trees[tree] = cloneFiles(f.attributeFiles)
	return tree, nil
}
func (f *landingFacts) File(repo, tree, path string) ([]byte, error) {
	f.call("contract-file")
	if repo != f.repo {
		f.scratchDir(repo)
	}
	data, ok := f.snapshot(tree)[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

var _ contractgit.CommitAccess = (*landingFacts)(nil)
var _ landing.ProjectionAccess = (*landingProjectionFacts)(nil)

type landingProjectionFacts struct{ f *landingFacts }

func (a *landingProjectionFacts) TopLevel(root string) (string, error) {
	return a.f.TopLevelForProjection(root)
}
func (a *landingProjectionFacts) Prefix(root string) (string, error) {
	return a.f.PrefixForProjection(root)
}
func (a *landingProjectionFacts) FilterPrefixes(repo, tree string, paths []string) (string, error) {
	return a.f.filter(repo, tree, paths, true)
}

func newCompositionFacts(t *testing.T) (*landingFacts, string, string) {
	t.Helper()
	f := newLandingFacts(t, map[string]string{"base.txt": "base", "metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"})
	f.composition = true
	f.stamp = "2026-09-17T10:00:00Z"
	plan := f.plan(f.base, "metasystem/plans/goal-a.md", "approved goal plan\n")
	u1 := f.unit(plan, "u1", "metasystem/one.go", "one\n")
	r1 := f.read(u1, u1, "u1")
	u2 := f.unit(r1, "u2", "metasystem/two.go", "two\n")
	r2 := f.read(u2, u2, "u2")
	between := f.plan(r2, "metasystem/plans/between.md", "folded into u3\n")
	u3 := f.unit(between, "u3", "metasystem/three.go", "three\n")
	r3 := f.read(u3, u3, "u3")
	tip := f.plan(r3, "metasystem/plans/tail.md", "tail fold\n")
	f.units = []string{u1, u2, u3}
	f.folds = [][]string{{plan, r1}, {r2}, {between, r3, tip}}
	f.writeFiles(f.repo, f.nodes[tip].files)
	files := cloneFiles(f.nodes[tip].files)
	for _, excluded := range landing.WorkspaceExclusions() {
		prefix := "metasystem/" + strings.TrimSuffix(excluded, "/")
		for path := range files {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				delete(files, path)
			}
		}
	}
	return f, tip, landingTree(files)
}

func (f *landingFacts) expectApply() {
	f.expect("patch", "index", "contract-paths", "attributes", "attribute-open", "attribute-base", "attribute-tree", "attributes", "attribute-close", "apply", "index", "contract-paths")
}

func (f *landingFacts) expectPass(count int) {
	f.expect("reset:" + f.base)
	for i := 0; i < count; i++ {
		f.expect("head")
		folds := f.folds[i]
		if count == 2 && i == 1 {
			folds = []string{f.folds[1][0], f.folds[2][0], f.folds[2][len(f.folds[2])-1]}
		}
		for _, fold := range folds {
			f.expect("index", "raw:"+fold)
			entries, err := parseRawEntries(fold, f.nodes[fold].raw)
			if err != nil {
				f.t.Fatal(err)
			}
			for _, entry := range entries {
				f.expect("entry:" + entry.Path)
			}
			f.expectApply()
		}
		unit := f.units[i]
		f.expect("index", "raw:"+unit)
		entries, err := parseRawEntries(unit, f.nodes[unit].raw)
		if err != nil {
			f.t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Path != contractgit.TestingContractPath {
				f.expect("entry:" + entry.Path)
			}
		}
		f.expectApply()
		if bytes.Contains(f.nodes[unit].raw, []byte(contractgit.TestingContractPath)) {
			f.expect("contract-file", "contract-file", "contract-file", "contract-file")
		}
		f.expect("index", "contract-paths")
		if bytes.Contains(f.nodes[unit].raw, []byte(contractgit.TestingContractPath)) {
			f.expect("contract-file", "contract-file", "contract-file", "contract-file")
			f.expect("transition", "transition", "transition", "transition", "transition", "transition")
		} else {
			f.expect("transition")
		}
		for _, fold := range folds {
			f.expect("raw:" + fold)
		}
		f.expect("message", "stage", "commit", "patch")
	}
}

func (f *landingFacts) expectProjection() {
	f.expect("prefix", "top-level", "prefix", "filter-prefixes")
}

func (f *landingFacts) expectPending(tip string, count int) {
	f.expectStatus(f.base, tip)
	f.expect("remote", "open:"+f.base)
	f.expectPass(count)
	f.expect("tree")
	f.expectProjection()
}

func (f *landingFacts) expectSuccess(tip string, count int) {
	f.expectPending(tip, count)
	f.expectPass(count)
	f.expect("tree", "head")
	f.expectProjection()
	f.expect("exists", "filter-exact", "push", "close")
}

func (f *landingFacts) expectRetry(tip string, count int, proof LandingProof, canary bool, push bool) {
	f.expectPending(tip, count)
	f.expectPass(count)
	f.expect("tree", "head")
	f.expectProjection()
	f.expect("exists", "file:"+tip+":"+landingRecordPath("goal-a"), "filter-exact")
	if proof.RetryIdentity == "" {
		f.expect("filter-exact")
	}
	if canary {
		f.expect("kind:" + proof.Fix)
		f.expect("ancestor:" + proof.Fix + ":" + proof.CanaryTip)
		if proof.CanaryTip != tip {
			f.expect("first-parent:"+proof.CanaryTip+":"+tip, "kind:"+tip)
		}
	}
	if push {
		f.expect("push")
	}
	f.expect("close")
}

func (f *landingFacts) assertRetryRefusal(out string, err error, code, tip, remote string) {
	f.t.Helper()
	requireLandingRefusal(f.t, err, code)
	f.consumed()
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		f.t.Fatalf("refusal output exists: %v", err)
	}
	if f.remote != remote {
		f.t.Fatalf("refusal moved landing ref: %s to %s", remote, f.remote)
	}
	if got := f.readFiles(f.repo); !reflect.DeepEqual(got, f.nodes[tip].files) {
		f.t.Fatal("refusal changed source snapshot")
	}
	if f.scratch != "" {
		if _, err := os.Stat(f.scratch); !os.IsNotExist(err) {
			f.t.Fatalf("scratch remains: %v", err)
		}
	}
}

func (f *landingFacts) record(parent string, proof LandingProof) string {
	f.t.Helper()
	tip := f.plan(parent, landingRecordPath("goal-a"), RenderLandingProof(proof)+"\n")
	f.writeFiles(f.repo, f.nodes[tip].files)
	return tip
}

func (f *landingFacts) retryReceipt(tip, attempt string) string {
	f.t.Helper()
	path := filepath.Join(f.t.TempDir(), "receipt.json")
	f.writeReceipt(path, f.projected(f.nodes[tip].tree), attempt)
	return path
}

func (f *landingFacts) assertLanding(result LandResult, out string, number int) {
	f.t.Helper()
	if result.ProofNumber != number || result.RetryIdentity == "" || f.remote != result.Landing || f.nodes[result.Landing].tree != result.Candidate {
		f.t.Fatalf("landing result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(out, "record-draft")); err != nil {
		f.t.Fatalf("landing artifact: %v", err)
	}
	if _, err := os.Stat(f.scratch); !os.IsNotExist(err) {
		f.t.Fatalf("scratch remains: %v", err)
	}
}

func (f *landingFacts) writeReceipt(path, tree, attempt string) {
	f.t.Helper()
	body := fmt.Sprintf(`{"schemaVersion":3,"tree":%q,"exitStatus":0,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":%q}}`, tree, attempt)
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *landingFacts) projected(tree string) string {
	files := cloneFiles(f.snapshot(tree))
	for _, excluded := range landing.WorkspaceExclusions() {
		prefix := "metasystem/" + strings.TrimSuffix(excluded, "/")
		for path := range files {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				delete(files, path)
			}
		}
	}
	return landingTree(files)
}

func (f *landingFacts) landed(tip string) []string {
	f.t.Helper()
	var reverse []string
	for id := tip; id != f.base; id = f.nodes[id].parent {
		if id == "" {
			f.t.Fatal("landing left graph")
		}
		reverse = append(reverse, id)
	}
	for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
		reverse[i], reverse[j] = reverse[j], reverse[i]
	}
	return reverse
}

func TestGoalLandingPreparationSeries(t *testing.T) {
	t.Parallel()
	f, tip, projected := newCompositionFacts(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receiptPath, projected, "attempt-deep")
	out := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	beforeRemote := f.remote
	if _, err := prepareLanding(f.request(f.base, tip, out, receiptPath), f.repository()); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing output refusal = %v", err)
	}
	if f.remote != beforeRemote {
		t.Fatalf("existing output moved landing ref: before=%s after=%s", beforeRemote, f.remote)
	}

	partialOut := filepath.Join(t.TempDir(), "partial")
	partial := f.request(f.base, tip, partialOut, receiptPath)
	partial.Last, partial.Through = false, f.units[1]
	partial.GoalPage = "- Next step: goal/goal-a last unit u2 commit " + f.units[1] + " is read clean\n"
	f.expectStatus(f.base, tip)
	_, err := prepareLanding(partial, f.repository())
	requireLandingRefusal(t, err, LandPartialCode)
	f.consumed()
	if _, err := os.Stat(partialOut); !os.IsNotExist(err) {
		t.Fatalf("partial output exists: %v", err)
	}

	through := partial
	through.Out = filepath.Join(t.TempDir(), "through")
	through.GoalPage = "- Next step: land through " + f.units[1] + "\n"
	discovery := filepath.Join(t.TempDir(), "through-discovery.json")
	f.writeReceipt(discovery, strings.Repeat("0", 40), "discover-through")
	through.TestReceipt = discovery
	f.expectPending(tip, 2)
	f.expectProjection()
	f.expect("close")
	_, discoveryErr := prepareLanding(through, f.repository())
	f.consumed()
	const marker = "candidate workspace "
	at := strings.LastIndex(fmt.Sprint(discoveryErr), marker)
	if at < 0 {
		t.Fatalf("partial candidate discovery = %v", discoveryErr)
	}
	throughProjected := strings.TrimSpace(fmt.Sprint(discoveryErr)[at+len(marker):])
	throughReceipt := filepath.Join(t.TempDir(), "through-receipt.json")
	f.writeReceipt(throughReceipt, throughProjected, "attempt-through")
	through.TestReceipt = throughReceipt
	f.expectSuccess(tip, 2)
	throughResult, err := prepareLanding(through, f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	throughCommits := f.landed(throughResult.Landing)
	if len(throughCommits) != 2 {
		t.Fatalf("partial landing commits = %v", throughCommits)
	}
	for _, commit := range throughCommits {
		if message := string(f.messages[commit]); strings.Contains(message, "Goal-Last:") {
			t.Fatalf("partial landing commit carries Goal-Last:\n%s", message)
		}
	}

	wrongReceipt := filepath.Join(t.TempDir(), "wrong.json")
	f.writeReceipt(wrongReceipt, f.projected(f.nodes[f.units[0]].tree), "attempt-interim")
	wrongOut := filepath.Join(t.TempDir(), "wrong")
	f.expectPending(tip, 3)
	f.expectProjection()
	f.expect("close")
	_, err = prepareLanding(f.request(f.base, tip, wrongOut, wrongReceipt), f.repository())
	requireLandingRefusal(t, err, LandUnprovenCode)
	f.consumed()
	if _, err := os.Stat(wrongOut); !os.IsNotExist(err) {
		t.Fatalf("wrong receipt output exists: %v", err)
	}

	unboundOut := filepath.Join(t.TempDir(), "unbound")
	unbound := f.request(f.base, tip, unboundOut, receiptPath)
	unbound.ApprovedBy = "human:Nobody"
	f.expect("claim", "human")
	_, err = prepareLanding(unbound, f.repository())
	requireLandingRefusal(t, err, LandAuthorUnboundCode)
	f.consumed()
	if _, err := os.Stat(unboundOut); !os.IsNotExist(err) {
		t.Fatalf("unbound output exists: %v", err)
	}

	out = filepath.Join(t.TempDir(), "prepared")
	f.expectSuccess(tip, 3)
	result, err := prepareLanding(f.request(f.base, tip, out, receiptPath), f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	if result.Endpoint != f.base || result.Attempt != "attempt-deep" || result.LastUnit != "u3" || result.ProofNumber != 1 || f.remote != result.Landing || f.nodes[result.Landing].tree != result.Candidate {
		t.Fatalf("landing result = %+v", result)
	}
	commits := f.landed(result.Landing)
	if len(commits) != 3 {
		t.Fatalf("landing commits = %v", commits)
	}
	for i, commit := range commits {
		message := string(f.messages[commit])
		for _, fragment := range []string{"Goal-Unit: goal-a/u", "Goal-Digest: ", "Goal-Source: " + f.units[i], "Landed-By: seat-a"} {
			if !strings.Contains(message, fragment) {
				t.Fatalf("message %d lacks %q:\n%s", i, fragment, message)
			}
		}
		if got := strings.Count(message, "Goal-Last: goal-a"); got != map[bool]int{true: 1, false: 0}[i == 2] {
			t.Fatalf("Goal-Last count on commit %d = %d", i, got)
		}
	}
	lastMessage := string(f.messages[commits[2]])
	for _, fold := range []string{"Goal-Fold: metasystem/plans/tail.md", "Goal-Fold: metasystem/plans/between.md"} {
		if !strings.Contains(lastMessage, fold) {
			t.Fatalf("last commit lacks %s:\n%s", fold, lastMessage)
		}
	}
	receipts := string(f.nodes[result.Landing].files["metasystem/memory/receipts.log"])
	if strings.Count(receipts, "|goal=goal-a|") != 3 || strings.Count(receipts, "|last_unit=u3|") != 3 || strings.Count(receipts, "|proof=attempt-deep|") != 3 {
		t.Fatalf("landing receipt rows:\n%s", receipts)
	}
	rows := strings.Split(strings.TrimSpace(receipts), "\n")
	if len(rows) != 4 {
		t.Fatalf("receipt row count = %d: %v", len(rows), rows)
	}
	for i, row := range rows[1:] {
		fields := strings.Split(row, "|")
		if len(fields) < 17 {
			t.Fatalf("receipt row %d incomplete: %s", i, row)
		}
		epoch, parseErr := strconv.ParseInt(fields[0], 10, 64)
		stamp, stampErr := time.Parse("2006-01-02T15:04:05Z", fields[1])
		if parseErr != nil || stampErr != nil || epoch != stamp.Unix() || fields[1] != f.stamp {
			t.Fatalf("receipt row %d time = %q|%q parse=%v/%v", i, fields[0], fields[1], parseErr, stampErr)
		}
		for _, field := range []string{"skills=none", "verify=clean", "corrections=0", "stop_loss=no", "delegate=none", "built_by=coordinator"} {
			if !strings.Contains("|"+row+"|", "|"+field+"|") {
				t.Fatalf("receipt row %d lacks %s: %s", i, field, row)
			}
		}
		oneRow := filepath.Join(t.TempDir(), fmt.Sprintf("receipt-%d.log", i))
		if err := os.WriteFile(oneRow, []byte(row+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stats := receipt.Stats(receipt.Options{File: oneRow, All: true})
		if stats.Code != 0 || !strings.Contains(strings.Join(stats.Out, " "), "receipts=1") || !strings.Contains(strings.Join(stats.Out, " "), "span_days=0.0") {
			t.Fatalf("receipt reader rejected row %d: %+v", i, stats)
		}
	}
	draft, err := os.ReadFile(filepath.Join(out, "record-draft"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"n=1", "endpoint=" + f.base, "candidate=" + result.Candidate, "landing=" + result.Landing, "attempt=attempt-deep"} {
		if !strings.Contains(string(draft), value) {
			t.Fatalf("record draft lacks %q: %s", value, draft)
		}
	}

	tip = f.plan(tip, "metasystem/plans/tail-two.md", "new candidate\n")
	f.folds[2] = append(f.folds[2], tip)
	f.writeFiles(f.repo, f.nodes[tip].files)
	f.writeReceipt(receiptPath, f.projected(f.nodes[tip].tree), "attempt-two")
	secondOut := filepath.Join(t.TempDir(), "second")
	f.expectSuccess(tip, 3)
	second, err := prepareLanding(f.request(f.base, tip, secondOut, receiptPath), f.repository())
	if err != nil || second.Landing == result.Landing || f.remote != second.Landing {
		t.Fatalf("replacement landing = %+v err=%v", second, err)
	}
	f.consumed()

	movedOut := filepath.Join(t.TempDir(), "moved")
	moved := f.request(f.base, tip, movedOut, receiptPath)
	intruder := f.add(f.base, "landing intruder", "", map[string][]byte{})
	moved.Hooks.AfterLandingRead = func() error { f.remote = intruder; return nil }
	f.expectSuccess(tip, 3)
	_, err = prepareLanding(moved, f.repository())
	requireLandingRefusal(t, err, LandBranchMovedCode)
	f.consumed()
	if _, err := os.Stat(movedOut); !os.IsNotExist(err) {
		t.Fatalf("moved output exists: %v", err)
	}
	if got := f.readFiles(f.repo); !reflect.DeepEqual(got, f.nodes[tip].files) {
		t.Fatal("source changed on moved landing")
	}
	if _, err := os.Stat(f.scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}

func TestGoalLandingCommitDatesComeFromReceiptStamp(t *testing.T) {
	t.Parallel()
	f, tip, projected := newCompositionFacts(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receiptPath, projected, "attempt-deep")
	f.expectSuccess(tip, 3)
	result, err := prepareLanding(f.request(f.base, tip, filepath.Join(t.TempDir(), "dated"), receiptPath), f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	for _, commit := range f.landed(result.Landing) {
		if dates := f.dates[commit]; dates != f.stamp {
			t.Fatalf("landing commit %s dates = %s", commit, dates)
		}
	}
}

func TestLandThroughIgnoresStopFenceReason(t *testing.T) {
	t.Parallel()
	f, tip, _ := newCompositionFacts(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receiptPath, strings.Repeat("0", 40), "attempt-deep")
	req := f.request(f.base, tip, filepath.Join(t.TempDir(), "stop-fence"), receiptPath)
	req.Last, req.Through = false, f.units[1]
	req.GoalPage = "- Stop fence: stopId=s1 reason=land through " + f.units[1] + "\n\nHistory:\n- 2026-09-17T10:00:00Z op stop actor=human:Wido reason=wait\n"
	f.expectStatus(f.base, tip)
	_, err := prepareLanding(req, f.repository())
	requireLandingRefusal(t, err, LandPartialCode)
	f.consumed()
	req.Out = filepath.Join(t.TempDir(), "history-word")
	req.GoalPage = "- Next step: wait\n\nHistory:\n- 2026-09-17T10:00:00Z op answer actor=human:Wido reason=land through " + f.units[1] + "\n"
	f.expectPending(tip, 2)
	f.expectProjection()
	f.expect("close")
	_, err = prepareLanding(req, f.repository())
	if err == nil {
		t.Fatal("history word unexpectedly matched the full-landing receipt")
	}
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code == LandPartialCode {
		t.Fatalf("history word was not accepted: %v", err)
	}
	f.consumed()
}

func TestGLENestedInstallationLandingProjection(t *testing.T) {
	t.Parallel()
	f, tip, projected := newCompositionFacts(t)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receipt, projected, "attempt-deep")
	request := f.request(f.base, tip, filepath.Join(t.TempDir(), "prepared"), receipt)
	request.Repo = filepath.Join(f.repo, "metasystem")
	f.expectSuccess(tip, 3)
	result, err := prepareLanding(request, f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	if result.Candidate == "" || result.Landing == "" {
		t.Fatalf("incomplete landing result: %+v", result)
	}
	if got := f.projected(result.Candidate); got != projected {
		t.Fatalf("nested projection = %s; want %s", got, projected)
	}
}

func TestGLEProjectRootLandingProjection(t *testing.T) {
	t.Parallel()
	f, tip, projected := newCompositionFacts(t)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receipt, projected, "attempt-deep")
	request := f.request(f.base, tip, filepath.Join(t.TempDir(), "prepared"), receipt)
	if request.Repo != f.repo {
		t.Fatalf("project-root caller changed: %s", request.Repo)
	}
	f.expectSuccess(tip, 3)
	result, err := prepareLanding(request, f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	if result.Candidate == "" || result.Landing == "" {
		t.Fatalf("incomplete landing result: %+v", result)
	}
	if got := f.projected(result.Candidate); got != projected {
		t.Fatalf("project-root projection = %s; want %s", got, projected)
	}
}

func landingContractFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			landingContractSurface("base", "base-group"),
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"base-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups: []testpolicy.Group{landingContractGroup("base-group", 1000)}, Always: testpolicy.Always{Canary: []string{}, Standard: []string{}},
		Unknown: []string{"base-group"}, Cadence: []string{}}
}

func landingContractGroup(id string, target int64) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{},
		Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: target,
		Packages: []string{"./example"}, Tests: json.RawMessage(`["TestExample"]`)}
}

func landingContractSurface(id, group string) testpolicy.Surface {
	return testpolicy.Surface{ID: id, Paths: []string{id + "/**"}, DependsOn: []string{}, Standard: []string{group}, Deep: []string{}, Critical: []string{}}
}

func landingContractAddition(contract testpolicy.Contract, name string, target int64) testpolicy.Contract {
	id := name + "-group"
	contract.Groups = append(contract.Groups, landingContractGroup(id, target))
	contract.Surfaces = append(contract.Surfaces, landingContractSurface(name, id))
	for index := range contract.Surfaces {
		if contract.Surfaces[index].ID == contract.Fallback {
			contract.Surfaces[index].Standard = append(contract.Surfaces[index].Standard, id)
		}
	}
	contract.Unknown = append(contract.Unknown, id)
	return contract
}

func landingContractBytes(t *testing.T, contract testpolicy.Contract) []byte {
	t.Helper()
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBranchLandingSyncMergesTestingContractBySurface(t *testing.T) {
	t.Parallel()
	const path = "metasystem/testing.json"
	baseContract := landingContractFixture()
	baseBytes := landingContractBytes(t, baseContract)
	goalBytes := landingContractBytes(t, landingContractAddition(landingContractFixture(), "goal", 1100))
	mainBytes := landingContractBytes(t, landingContractAddition(landingContractFixture(), "main", 1200))
	f := newLandingFacts(t, map[string]string{
		".gitattributes":                 "metasystem/testing.json merge=metasystem-testing\n",
		path:                             string(baseBytes),
		"metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n",
	})
	f.composition, f.stamp = true, "2026-09-17T10:00:00Z"
	unit := f.unit(f.base, "testing", path, string(goalBytes))
	tip := f.read(unit, unit, "testing")
	endpoint := f.add(f.base, "endpoint", "", map[string][]byte{path: mainBytes})
	f.base = endpoint
	f.units, f.folds = []string{unit}, [][]string{{tip}}
	f.writeFiles(f.repo, f.nodes[tip].files)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receipt, strings.Repeat("0", 40), "discover-sync")
	request := f.request(endpoint, tip, filepath.Join(t.TempDir(), "discover"), receipt)
	f.expectPending(tip, 1)
	f.expectProjection()
	f.expect("close")
	_, discoveryErr := prepareLanding(request, f.repository())
	f.consumed()
	const marker = "candidate workspace "
	markerAt := strings.LastIndex(fmt.Sprint(discoveryErr), marker)
	if markerAt < 0 {
		t.Fatalf("sync candidate discovery = %v", discoveryErr)
	}
	candidate := strings.TrimSpace(fmt.Sprint(discoveryErr)[markerAt+len(marker):])
	f.writeReceipt(receipt, candidate, "branch-sync")
	request.Out = filepath.Join(t.TempDir(), "prepared")
	f.expectSuccess(tip, 1)
	result, err := prepareLanding(request, f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	merged, err := testpolicy.Decode(f.snapshot(result.Candidate)[path])
	if err != nil {
		t.Fatal(err)
	}
	var groups []string
	for _, group := range merged.Groups {
		groups = append(groups, group.ID)
	}
	want := []string{"base-group", "main-group", "goal-group"}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("synced groups = %v", groups)
	}
	for _, surface := range merged.Surfaces {
		if surface.ID == "residual" {
			if !reflect.DeepEqual(surface.Standard, want) {
				t.Fatalf("synced residual groups = %v", surface.Standard)
			}
			return
		}
	}
	t.Fatal("residual surface is absent")
}

// verificationReader returns only the Git bytes needed to verify commits in
// this graph. A request outside that parent chain is a fixture failure.
func (f *landingFacts) verificationReader(tip string) landingGitReader {
	f.t.Helper()
	allowed := map[string]bool{f.base: true}
	for _, id := range f.landed(tip) {
		allowed[id] = true
	}
	return func(repo string, args ...string) ([]byte, error) {
		f.t.Helper()
		if repo != f.repo {
			f.t.Fatalf("verification repository = %s; want %s", repo, f.repo)
		}
		f.call("verify:" + strings.Join(args, " "))
		switch {
		case len(args) == 4 && args[0] == "show" && args[1] == "-s" && args[2] == "--format=%(trailers:only,unfold=true)" && allowed[args[3]]:
			message := f.messages[args[3]]
			_, body, ok := strings.Cut(string(message), "\n\n")
			if !ok {
				return []byte("\n"), nil
			}
			var trailers strings.Builder
			for _, line := range strings.Split(body, "\n") {
				if _, _, valid := strings.Cut(line, ": "); valid {
					trailers.WriteString(line + "\n")
				}
			}
			return []byte(trailers.String()), nil
		case len(args) == 5 && args[0] == "rev-list" && args[1] == "--parents" && args[2] == "-n" && args[3] == "1" && allowed[args[4]]:
			node := f.nodes[args[4]]
			if node.parent == "" {
				return nil, fmt.Errorf("commit %s has no parent", node.id)
			}
			return []byte(node.id + " " + node.parent + "\n"), nil
		case len(args) == 7 && args[0] == "diff-tree" && args[1] == "-r" && args[2] == "-z" && args[3] == "--no-renames" && args[4] == "--full-index" && allowed[args[6]]:
			node := f.nodes[args[6]]
			if args[5] == node.id+"^" && node.parent != "" {
				return append([]byte(nil), node.raw...), nil
			}
		case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^"):
			id := strings.TrimSuffix(args[1], "^")
			if allowed[id] && f.nodes[id].parent != "" {
				return []byte(f.nodes[id].parent + "\n"), nil
			}
		}
		f.t.Fatalf("undeclared verification request: %q", args)
		return nil, nil
	}
}

func TestGoalLandingKeepsBuildUnitList(t *testing.T) {
	t.Parallel()
	f := newLandingFacts(t, map[string]string{"metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"})
	f.composition, f.stamp = true, "2026-09-17T10:00:00Z"
	unit := f.unit(f.base, "5+6", "metasystem/multi.go", "multi\n")
	tip := f.read(unit, unit, "5+6")
	f.units, f.folds = []string{unit}, [][]string{{tip}}
	f.writeFiles(f.repo, f.nodes[tip].files)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receipt, f.projected(f.nodes[tip].tree), "attempt-multi")
	f.expectSuccess(tip, 1)
	result, err := prepareLanding(f.request(f.base, tip, filepath.Join(t.TempDir(), "prepared"), receipt), f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	message := string(f.messages[result.Landing])
	reader := f.verificationReader(result.Landing)
	f.expect("verify:show -s --format=%(trailers:only,unfold=true) "+result.Landing,
		"verify:rev-list --parents -n 1 "+result.Landing,
		"verify:diff-tree -r -z --no-renames --full-index "+result.Landing+"^ "+result.Landing)
	verified, verifyErr := verifyLandedWithGit(f.repo, result.Landing, reader)
	f.consumed()
	if result.LastUnit != "6" || !strings.Contains(message, "Goal-Unit: goal-a/5+6") || verifyErr != nil || verified.Units != "5+6" {
		t.Fatalf("multi-unit landing=%+v message=%q verified=%+v err=%v", result, message, verified, verifyErr)
	}
	f.expect("verify:show -s --format=%(trailers:only,unfold=true) "+result.Landing,
		"verify:show -s --format=%(trailers:only,unfold=true) "+result.Landing,
		"verify:rev-parse "+result.Landing+"^",
		"verify:show -s --format=%(trailers:only,unfold=true) "+f.base,
		"verify:show -s --format=%(trailers:only,unfold=true) "+result.Landing,
		"verify:rev-list --parents -n 1 "+result.Landing,
		"verify:diff-tree -r -z --no-renames --full-index "+result.Landing+"^ "+result.Landing)
	series, seriesErr := verifyLandedSeriesWithGit(f.repo, result.Landing, reader)
	f.consumed()
	if seriesErr != nil || len(series) != 1 || series[0].Units != "5+6" || series[0].Actual != series[0].Expected {
		t.Fatalf("multi-unit series=%+v err=%v", series, seriesErr)
	}
}

func TestGoalLandingPreimageRetryAndCanaryFence(t *testing.T) {
	t.Parallel()
	t.Run("endpoint preimage", func(t *testing.T) {
		t.Parallel()
		f, tip, _ := newCompositionFacts(t)
		endpoint := f.unit(f.base, "endpoint", "metasystem/one.go", "endpoint owns this path\n")
		out := filepath.Join(t.TempDir(), "preimage")
		receipt := f.retryReceipt(tip, "attempt-deep")
		f.expectStatus(endpoint, tip)
		f.expect("remote", "open:"+endpoint, "reset:"+endpoint, "head", "index", "raw:"+f.folds[0][0], "entry:metasystem/plans/goal-a.md")
		f.expectApply()
		fold := f.folds[0][1]
		f.expect("index", "raw:"+fold)
		entries, parseErr := parseRawEntries(fold, f.nodes[fold].raw)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, entry := range entries {
			f.expect("entry:" + entry.Path)
		}
		f.expectApply()
		f.expect("index", "raw:"+f.units[0], "entry:metasystem/one.go", "close")
		_, err := prepareLanding(f.request(endpoint, tip, out, receipt), f.repository())
		f.assertRetryRefusal(out, err, UnitRereadCode, tip, "")
	})

	t.Run("recorded red candidate", func(t *testing.T) {
		t.Parallel()
		f, tip, projected := newCompositionFacts(t)
		receipt := filepath.Join(t.TempDir(), "receipt.json")
		f.writeReceipt(receipt, projected, "attempt-deep")
		firstOut := filepath.Join(t.TempDir(), "first")
		f.expectSuccess(tip, 3)
		first, err := prepareLanding(f.request(f.base, tip, firstOut, receipt), f.repository())
		if err != nil {
			t.Fatal(err)
		}
		f.consumed()
		f.assertLanding(first, firstOut, 1)
		proof := LandingProof{Number: 1, Endpoint: first.Endpoint, Candidate: first.Candidate,
			Landing: first.Landing, Attempt: first.Attempt, Verdict: "red", Groups: []string{"deep"}}
		tip = f.record(tip, proof)
		f.folds[2] = append(f.folds[2], tip)
		receipt = f.retryReceipt(tip, "attempt-red-rerun")
		out := filepath.Join(t.TempDir(), "retry")
		remote := f.remote
		f.expectRetry(tip, 3, proof, false, false)
		_, err = prepareLanding(f.request(f.base, tip, out, receipt), f.repository())
		f.assertRetryRefusal(out, err, LandRetryCode, tip, remote)
	})

	t.Run("red proof needs clean canary", func(t *testing.T) {
		t.Parallel()
		f, tip, _ := newCompositionFacts(t)
		proof := LandingProof{Number: 1, Endpoint: f.base, Candidate: f.base, Landing: f.base,
			Attempt: "old", Verdict: "red", Groups: []string{"deep"}}
		tip = f.record(tip, proof)
		f.folds[2] = append(f.folds[2], tip)
		receipt := f.retryReceipt(tip, "attempt-after-red")
		uncheckedOut := filepath.Join(t.TempDir(), "unchecked")
		f.expectRetry(tip, 3, proof, false, false)
		_, err := prepareLanding(f.request(f.base, tip, uncheckedOut, receipt), f.repository())
		f.assertRetryRefusal(uncheckedOut, err, LandUncheckedCode, tip, "")

		proof.CanaryRun, proof.CanaryTip, proof.Fix = "canary-clean", tip, f.units[2]
		tip = f.record(tip, proof)
		f.folds[2] = append(f.folds[2], tip)
		receipt = f.retryReceipt(tip, "attempt-after-canary")
		checkedOut := filepath.Join(t.TempDir(), "checked")
		f.expectRetry(tip, 3, proof, true, true)
		result, err := prepareLanding(f.request(f.base, tip, checkedOut, receipt), f.repository())
		if err != nil || result.ProofNumber != 2 {
			t.Fatalf("checked landing = %+v err=%v", result, err)
		}
		f.consumed()
		f.assertLanding(result, checkedOut, 2)

		for _, row := range []struct {
			name, fix, canary              string
			kindErr, ancestorErr, rangeErr error
			kindErrorAt                    string
			want                           bool
		}{
			{name: "wrong fix kind", fix: tip, canary: tip},
			{name: "failed ancestry", fix: f.units[2], canary: f.base},
			{name: "kind error", fix: f.units[2], canary: tip, kindErr: errors.New("kind unavailable")},
			{name: "ancestor error", fix: f.units[2], canary: tip, ancestorErr: errors.New("ancestor unavailable")},
			{name: "range error", fix: f.units[2], canary: f.units[2], rangeErr: errors.New("range unavailable")},
			{name: "range classification error", fix: f.units[2], canary: proof.CanaryTip, kindErr: errors.New("plan kind unavailable"), kindErrorAt: tip},
			{name: "same tip", fix: f.units[2], canary: tip, want: true},
		} {
			p := proof
			p.Fix, p.CanaryTip = row.fix, row.canary
			f.kindError, f.kindErrorAt, f.ancestorError, f.firstParentError = row.kindErr, row.kindErrorAt, row.ancestorErr, row.rangeErr
			f.expect("kind:" + p.Fix)
			if (row.kindErr == nil || row.kindErrorAt != "") && row.fix != tip {
				f.expect("ancestor:" + p.Fix + ":" + p.CanaryTip)
				if row.ancestorErr == nil && row.canary != f.base && row.canary != tip {
					f.expect("first-parent:" + p.CanaryTip + ":" + tip)
					if row.kindErrorAt != "" {
						f.expect("kind:" + tip)
					}
				}
			}
			if got := redProofChecked(f.repository(), f.repo, tip, "goal-a", p); got != row.want {
				t.Fatalf("%s = %t, want %t", row.name, got, row.want)
			}
			f.consumed()
		}
		f.kindError, f.kindErrorAt, f.ancestorError, f.firstParentError = nil, "", nil, nil
		repository := f.repository()
		f.expect("first-parent:" + tip + ":" + tip)
		if empty, err := repository.firstParent(f.repo, tip, tip); err != nil || len(empty) != 0 {
			t.Fatalf("empty first-parent range = %q, err=%v", empty, err)
		}
		f.consumed()
		sibling := f.plan(f.base, "metasystem/plans/sibling.md", "sibling\n")
		f.expect("first-parent:" + sibling + ":" + tip)
		divergent, err := repository.firstParent(f.repo, sibling, tip)
		var want strings.Builder
		for _, node := range f.chain(f.base, tip) {
			fmt.Fprintln(&want, node.id)
		}
		if err != nil || string(divergent) != want.String() {
			t.Fatalf("divergent first-parent range = %q, err=%v; want %q", divergent, err, want.String())
		}
		f.consumed()
		unitAfter := f.unit(tip, "late", "metasystem/late.go", "late\n")
		p := proof
		p.CanaryTip = tip
		f.expect("kind:"+p.Fix, "ancestor:"+p.Fix+":"+p.CanaryTip, "first-parent:"+p.CanaryTip+":"+unitAfter, "kind:"+unitAfter)
		if redProofChecked(f.repository(), f.repo, unitAfter, "goal-a", p) {
			t.Fatal("unit after canary was accepted")
		}
		f.consumed()
	})
}

func TestGoalLandingRetryIdentitySurvivesASecondClone(t *testing.T) {
	t.Parallel()
	f, tip, projected := newCompositionFacts(t)
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	f.writeReceipt(receipt, projected, "attempt-deep")
	firstOut := filepath.Join(t.TempDir(), "first")
	f.expectSuccess(tip, 3)
	first, err := prepareLanding(f.request(f.base, tip, firstOut, receipt), f.repository())
	if err != nil {
		t.Fatal(err)
	}
	f.consumed()
	f.assertLanding(first, firstOut, 1)
	if first.RetryIdentity == "" {
		t.Fatal("prepared landing has no persisted retry identity")
	}
	proof := LandingProof{Number: 1, Endpoint: first.Endpoint, Candidate: first.Candidate, Landing: first.Landing,
		RetryIdentity: first.RetryIdentity, Attempt: first.Attempt, Verdict: "red", Groups: []string{"deep"}}
	tip = f.record(tip, proof)
	f.folds[2] = append(f.folds[2], tip)
	f.remote = ""

	fresh := newLandingFacts(t, map[string]string{"base.txt": "base", "metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"})
	fresh.composition, fresh.stamp = true, f.stamp
	for _, node := range f.chain(f.base, tip) {
		fresh.add(node.parent, string(node.kind), node.unit, node.files)
	}
	fresh.units = append([]string(nil), f.units...)
	for _, folds := range f.folds {
		fresh.folds = append(fresh.folds, append([]string(nil), folds...))
	}
	fresh.writeFiles(fresh.repo, fresh.nodes[tip].files)
	if _, ok := fresh.nodes[first.Landing]; ok {
		t.Fatal("old landing object reached fresh graph")
	}
	if _, ok := fresh.trees[first.Candidate]; ok {
		t.Fatal("old candidate object reached fresh graph")
	}
	receipt = fresh.retryReceipt(tip, "clone-b-retry")
	out := filepath.Join(t.TempDir(), "retry")
	fresh.expectRetry(tip, 3, proof, false, false)
	_, err = prepareLanding(fresh.request(fresh.base, tip, out, receipt), fresh.repository())
	fresh.assertRetryRefusal(out, err, LandRetryCode, tip, "")

	legacy := proof
	legacy.RetryIdentity = ""
	legacy.Candidate = first.Candidate
	legacyTip := fresh.record(tip, legacy)
	fresh.folds[2] = append(fresh.folds[2], legacyTip)
	receipt = fresh.retryReceipt(legacyTip, "clone-b-legacy")
	fresh.expectRetry(legacyTip, 3, legacy, false, false)
	legacyOut := filepath.Join(t.TempDir(), "legacy")
	_, err = prepareLanding(fresh.request(fresh.base, legacyTip, legacyOut, receipt), fresh.repository())
	if err == nil || !strings.Contains(err.Error(), "unknown tree") {
		t.Fatalf("missing legacy candidate = %v", err)
	}
	fresh.consumed()
	if _, err := os.Stat(legacyOut); !os.IsNotExist(err) {
		t.Fatalf("legacy refusal output exists: %v", err)
	}
	if _, err := os.Stat(fresh.scratch); !os.IsNotExist(err) {
		t.Fatalf("legacy refusal scratch remains: %v", err)
	}

	fix := fresh.unit(tip, "land-fix-1", "metasystem/fix.go", "fixed\n")
	fixRead := fresh.read(fix, fix, "land-fix-1")
	fresh.units = append(fresh.units, fix)
	fresh.folds[2] = fresh.folds[2][:2]
	fresh.folds = append(fresh.folds, []string{f.folds[2][2], tip, fixRead})
	proof.CanaryRun, proof.CanaryTip, proof.Fix = "clone-b-canary", fixRead, fix
	fixedTip := fresh.record(fixRead, proof)
	fresh.folds[3] = append(fresh.folds[3], fixedTip)
	receipt = fresh.retryReceipt(fixedTip, "clone-b-fixed")
	fixedOut := filepath.Join(t.TempDir(), "fixed")
	fresh.expectRetry(fixedTip, 4, proof, true, true)
	result, err := prepareLanding(fresh.request(fresh.base, fixedTip, fixedOut, receipt), fresh.repository())
	if err != nil || result.Landing == first.Landing || result.ProofNumber != 2 {
		t.Fatalf("second clone fixed landing=%+v err=%v", result, err)
	}
	fresh.consumed()
	fresh.assertLanding(result, fixedOut, 2)
}
