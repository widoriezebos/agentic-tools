package branch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func policyID(c string) string      { return strings.Repeat(c, 40) }
func policyHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func policyRaw(p, a, b string) []byte {
	return []byte(":100644 100644 " + a + " " + b + " M\x00" + p + "\x00")
}

type policyCall struct {
	method string
	args   []string
}
type attestationPolicyFixture struct {
	t                                                                  *testing.T
	root, clone, base, unit, plan, tree, tip, index, path, record, job string
	subject                                                            readsubject.ReadSubject
	subjects                                                           map[string]readsubject.ReadSubject
	raw                                                                map[string][]byte
	ranges                                                             map[string][]Commit
	snapshots                                                          map[string]map[string][]byte
	transitions                                                        map[string][]byte
	treeEntries                                                        map[string]string
	expected                                                           []policyCall
	generated                                                          map[string][]byte
	stageErr, installErr, restoreErr                                   error
	detached                                                           string
	branchTip, unitName                                                string
	staged                                                             []string
}

func newAttestationPolicyFixture(t *testing.T, testPath string, plan bool) *attestationPolicyFixture {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &attestationPolicyFixture{t: t, root: root, base: policyID("a"), unit: policyID("b"),
		tree: policyID("c"), tip: policyID("d"), index: policyID("e"), path: testPath,
		record: "metasystem/records/misc/goal-a-u1-read.md", job: "critic-policy",
		raw: map[string][]byte{}, snapshots: map[string]map[string][]byte{}, transitions: map[string][]byte{}, treeEntries: map[string]string{},
		subjects: map[string]readsubject.ReadSubject{}, ranges: map[string][]Commit{}, unitName: "u1"}
	f.write("metasystem.conf", []byte("testing.contract=testing.json\n"))
	if plan {
		f.plan = policyID("f")
		f.raw[f.plan] = policyRaw("metasystem/plans/goal-a.md", policyID("1"), policyID("2"))
	}
	f.raw[f.unit] = policyRaw(testPath, policyID("3"), policyID("4"))
	parent := f.base
	if plan {
		parent = f.plan
	}
	f.subject = readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: f.unit, Parent: parent, Tree: f.tree, DiffDigest: policyHash([]byte("explicit unit patch\n"))}
	t.Cleanup(func() {
		if len(f.expected) > 0 {
			t.Errorf("%d unconsumed calls: %+v", len(f.expected), f.expected)
		}
	})
	return f
}
func (f *attestationPolicyFixture) expect(method string, args ...string) {
	f.expected = append(f.expected, policyCall{method, args})
}
func (f *attestationPolicyFixture) next(method string, args ...string) {
	f.t.Helper()
	got := policyCall{method, args}
	if len(f.expected) == 0 || !reflect.DeepEqual(got, f.expected[0]) {
		f.t.Fatalf("unexpected call %+v; next expectation %+v", got, f.expected)
	}
	f.expected = f.expected[1:]
}
func (f *attestationPolicyFixture) rangeFacts() []Commit {
	commits := []Commit{}
	if f.plan != "" {
		commits = append(commits, Commit{ID: f.plan, Kind: Plan})
	}
	return append(commits, Commit{ID: f.unit, Kind: Unit, Unit: "u1", Units: []string{"u1"}})
}
func (f *attestationPolicyFixture) ReadSubject(repo, commit string) (readsubject.ReadSubject, error) {
	f.next("ReadSubject", repo, commit)
	if subject, ok := f.subjects[commit]; ok {
		return subject, nil
	}
	return f.subject, nil
}
func (f *attestationPolicyFixture) RawEntries(repo, commit string) ([]byte, error) {
	f.next("RawEntries", repo, commit)
	raw, ok := f.raw[commit]
	if !ok {
		f.t.Fatalf("missing raw entries for %s", commit)
	}
	return raw, nil
}
func (f *attestationPolicyFixture) Range(repo, endpoint, tip, goal string) ([]Commit, error) {
	f.next("Range", repo, endpoint, tip, goal)
	if commits, ok := f.ranges[endpoint+":"+tip]; ok {
		return commits, nil
	}
	if len(f.ranges) != 0 {
		f.t.Fatalf("missing range facts for %s:%s", endpoint, tip)
	}
	return f.rangeFacts(), nil
}
func (f *attestationPolicyFixture) SnapshotFile(repo, snapshot, path string) ([]byte, error) {
	f.next("SnapshotFile", repo, snapshot, path)
	if snapshot == "" {
		f.t.Fatal("empty snapshot reached SnapshotFile")
	}
	data, ok := f.snapshots[snapshot][path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}
func (f *attestationPolicyFixture) TopLevel(repo string) (string, error) {
	f.next("TopLevel", repo)
	if repo == f.clone {
		return f.clone, nil
	}
	return f.root, nil
}
func (f *attestationPolicyFixture) CommitExists(repo, commit string) error {
	f.next("CommitExists", repo, commit)
	return nil
}
func (f *attestationPolicyFixture) Kind(repo, commit, goal string) (KindInfo, error) {
	f.next("Kind", repo, commit, goal)
	return KindInfo{Kind: Unit, Unit: "u1", Units: []string{"u1"}, CommitID: commit}, nil
}
func (f *attestationPolicyFixture) Prefix(repo string) (string, error) {
	f.next("Prefix", repo)
	if repo == filepath.Join(f.root, "metasystem") {
		return "metasystem/", nil
	}
	return "", nil
}
func (f *attestationPolicyFixture) Transition(repo, before, after string) ([]byte, error) {
	f.next("Transition", repo, before, after)
	raw, ok := f.transitions[before+":"+after]
	if !ok {
		f.t.Fatalf("missing transition %s:%s", before, after)
	}
	return raw, nil
}
func (f *attestationPolicyFixture) TreeEntry(repo, tree, path string) (string, error) {
	f.next("TreeEntry", repo, tree, path)
	return f.treeEntries[tree+":"+path], nil
}
func (f *attestationPolicyFixture) Subject(repo, commit string) (AttestationSubject, error) {
	f.next("BranchSubject", repo, commit)
	return AttestationSubject{Commit: f.unit, Parent: f.subject.Parent, Tree: f.tree, PatchDigest: f.subject.DiffDigest, UnitDigest: digestRawEntries(f.raw[f.unit])}, nil
}
func (f *attestationPolicyFixture) CommonDir(repo string) (string, error) {
	f.next("CommonDir", repo)
	return filepath.Join(f.root, ".git"), nil
}
func (f *attestationPolicyFixture) Entries(repo, commit string) ([]Entry, error) {
	f.next("BranchEntries", repo, commit)
	return parseRawEntries(commit, f.raw[commit])
}
func (f *attestationPolicyFixture) Detached(repo, commit string) (string, func() error, error) {
	f.next("Detached", repo, commit)
	dir, err := os.MkdirTemp(f.root, "detached-")
	if err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("declared unit tree\n"), 0o644); err != nil {
		return "", nil, err
	}
	f.detached = dir
	return dir, func() error { return os.RemoveAll(dir) }, nil
}
func (f *attestationPolicyFixture) write(path string, data []byte) {
	abs := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *attestationPolicyFixture) writeReaderRecord() []byte {
	data := []byte("Read commit " + f.unit + " with unit digest " + digestRawEntries(f.raw[f.unit]) + " and found it clean.\n")
	f.write(f.record, data)
	return data
}
func (f *attestationPolicyFixture) writeJob(status string, open bool) {
	findings := []any{}
	if open {
		findings = append(findings, map[string]any{"findingId": "defect-a", "critic": f.job, "rigorClass": "bounded", "factsDigest": strings.Repeat("0", 64), "status": "open", "evidenceDigest": strings.Repeat("1", 64), "multiplicity": 1})
	}
	record := map[string]any{"jobId": f.job, "role": "code-critic", "round": 1, "status": status, "reviews": "commit:" + f.unit,
		"goalId": "goal-a", "goalRevision": 1, "findingRegister": findings, "findingRegisterRound": 1, "findingRegisterSubjectDigest": f.subject.Digest()}
	if status == "completed" {
		record["chainClosed"] = true
		record["closure"] = map[string]any{"criticRoot": f.job, "round": 1, "subject": f.subject, "mechanism": "clean"}
	}
	f.write("artifacts/agents/jobs/"+f.job+".json", policyCanonical(f.t, record))
	if status == "completed" {
		f.write("artifacts/agents/"+f.job+"/rounds/1/subject.json", policyCanonical(f.t, f.subject))
		f.write("artifacts/agents/"+f.job+"/rounds/1/return.json", policyCanonical(f.t, map[string]any{"jobId": f.job, "round": 1, "reviewedTree": f.tree}))
	}
}
func (f *attestationPolicyFixture) request(critic bool) CommitReadRequest {
	req := CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "policy-read", CheckClaim: func() error { return nil }, GateRunID: "fast-policy", GateTree: f.tree}
	if critic {
		req.RootJob = f.job
	} else {
		req.ReaderRecord = f.record
	}
	return req
}
func (f *attestationPolicyFixture) effects() readCommitEffects {
	return readCommitEffects{
		Inspect: func(req CommitRequest) (commitBranchState, error) {
			f.next("Inspect")
			if (req.Repo != f.root && req.Repo != filepath.Join(f.root, "metasystem")) || req.Kind != Read || req.GoalID != "goal-a" || !reflect.DeepEqual(req.Units, []string{f.unitName}) {
				f.t.Fatalf("inspect request %+v", req)
			}
			baseTip := f.unit
			if f.branchTip != "" {
				baseTip = f.branchTip
			}
			return commitBranchState{baseTip: baseTip, localTip: baseTip}, nil
		},
		StagedPaths: func(repo string) ([]string, error) {
			f.next("StagedPaths", repo)
			return append([]string(nil), f.staged...), nil
		},
		AdoptionClean: func(CommitRequest, commitBranchState, []string) error {
			f.t.Fatal("unexpected adoption check")
			return nil
		},
		Patch: func(repo string, generated map[string][]byte, reader string) ([]byte, error) {
			f.next("Patch", repo, reader)
			f.generated = policySnapshotCopy(generated)
			var att Attestation
			if err := json.Unmarshal(generated[attestationPath("goal-a", f.unit)], &att); err != nil || att.SHA256 == "" {
				f.t.Fatalf("patch attestation %+v %v", att, err)
			}
			digest, err := digestAttestation(att)
			if err != nil || digest != att.SHA256 || !bytes.Equal(generated[attestationPath("goal-a", f.unit)], policyCanonical(f.t, att)) {
				f.t.Fatalf("patch attestation digest or canonical bytes: %+v, %v", att, err)
			}
			if att.Source.Kind == "reader-record" && att.Carry == nil && reader != att.Source.ReaderRecord {
				f.t.Fatalf("patch reader = %q, source = %+v", reader, att.Source)
			}
			wantPaths := 1
			if att.Source.ClosureSHA256 != "" {
				wantPaths++
				if policyHash(generated[closureBundlePath("goal-a", f.unit)]) != att.Source.ClosureSHA256 {
					f.t.Fatalf("patch closure bundle does not match %+v", att.Source)
				}
			}
			if len(generated) != wantPaths {
				f.t.Fatalf("patch generated paths = %v", generated)
			}
			return []byte("explicit prospective patch\n"), nil
		},
		Build: func(req CommitRequest, state commitBranchState, subject, trailer string, patch []byte) (string, error) {
			f.next("Build")
			baseTip := f.unit
			if f.branchTip != "" {
				baseTip = f.branchTip
			}
			if req.Kind != Read || req.GoalID != "goal-a" || !reflect.DeepEqual(req.Units, []string{f.unitName}) || state.baseTip != baseTip ||
				subject != "goal goal-a read "+f.unitName || trailer != "Goal-Read: goal-a/"+f.unitName+" "+f.unit || !bytes.Equal(patch, []byte("explicit prospective patch\n")) {
				f.t.Fatalf("build request %+v %q %q %q", state, subject, trailer, patch)
			}
			return f.tip, nil
		},
		IndexTree: func(repo string) (string, error) { f.next("IndexTree", repo); return f.index, nil },
		RestoreIndex: func(repo, tree string) error {
			f.next("RestoreIndex", repo, tree)
			return f.restoreErr
		},
		Stage: func(repo string, paths []string) error {
			f.next("Stage", append([]string{repo}, paths...)...)
			for path, data := range f.generated {
				actual, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
				if err != nil || !bytes.Equal(actual, data) {
					f.t.Fatalf("published %s bytes %q, err %v", path, actual, err)
				}
			}
			return f.stageErr
		},
		Install: func(req CommitRequest, state commitBranchState, tip string) error {
			f.next("Install", tip)
			baseTip := f.unit
			if f.branchTip != "" {
				baseTip = f.branchTip
			}
			if tip != f.tip || req.Kind != Read || req.GoalID != "goal-a" || !reflect.DeepEqual(req.Units, []string{f.unitName}) || state.baseTip != baseTip {
				f.t.Fatalf("install request %+v, state %+v, tip %s", req, state, tip)
			}
			if f.installErr != nil {
				return f.installErr
			}
			snap := map[string][]byte{}
			for path := range f.generated {
				data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
				if err != nil {
					f.t.Fatal(err)
				}
				snap[path] = data
			}
			if req.Kind == Read && f.record != "" {
				if data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(f.record))); err == nil {
					snap[f.record] = data
				}
			}
			f.snapshots[tip] = snap
			return nil
		},
	}
}
func (f *attestationPolicyFixture) expectStart(repo string) {
	f.expect("Inspect")
	f.expect("Range", repo, f.base, f.unit, "goal-a")
	f.expect("ReadSubject", repo, f.unit)
	f.expect("RawEntries", repo, f.unit)
}
func (f *attestationPolicyFixture) expectAfterGate(repo string, critic bool) {
	f.expect("RawEntries", repo, f.unit)
	f.expect("Range", repo, f.base, f.unit, "goal-a")
	if f.plan != "" {
		f.expect("RawEntries", repo, f.plan)
	}
	reader := ""
	if !critic {
		f.expect("TopLevel", repo)
		reader = f.record
	}
	f.expect("StagedPaths", repo)
	f.expect("Patch", repo, reader)
	f.expect("Build")
	f.expect("IndexTree", repo)
	f.expect("TopLevel", repo)
	paths := []string{repo, attestationPath("goal-a", f.unit)}
	if critic {
		paths = append(paths, closureBundlePath("goal-a", f.unit))
	} else {
		paths = append(paths, f.record)
	}
	f.expect("Stage", paths...)
	f.expect("Install", f.tip)
}
func (f *attestationPolicyFixture) expectValidation(repo, snapshot string, critic bool, legacy ...bool) {
	path := attestationPath("goal-a", f.unit)
	f.expect("SnapshotFile", repo, snapshot, path)
	f.expect("ReadSubject", repo, f.unit)
	f.expect("RawEntries", repo, f.unit)
	f.expect("Range", repo, f.base, f.unit, "goal-a")
	if f.plan != "" {
		f.expect("RawEntries", repo, f.plan)
	}
	f.expect("RawEntries", repo, f.unit)
	if critic && len(legacy) == 0 {
		f.expect("SnapshotFile", repo, snapshot, closureBundlePath("goal-a", f.unit))
	} else if !critic {
		f.expect("SnapshotFile", repo, snapshot, f.record)
	}
}
func (f *attestationPolicyFixture) expectBranchStart() {
	f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
	f.expect("BranchSubject", f.root, f.unit)
	f.expect("CommonDir", f.root)
}

type branchPolicyRepository struct{ *attestationPolicyFixture }

func (r branchPolicyRepository) Range(repo, endpoint, tip, goal string) ([]Commit, error) {
	r.next("BranchRange", repo, endpoint, tip, goal)
	return r.rangeFacts(), nil
}
func TestAttestationRequiresFastGateAndNamesChangedTests(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code_test.go", false)
	f.writeReaderRecord()
	req := f.request(false)
	f.expect("Inspect")
	if _, _, err := commitRead(func() CommitReadRequest { copy := req; copy.GateRunID = ""; return copy }(), f, f.effects()); err == nil || !strings.Contains(err.Error(), ReadUngatedCode) {
		t.Fatalf("ungated read: %v", err)
	}
	f.expectStart(f.root)
	wrong := req
	wrong.GateTree = f.base
	if _, _, err := commitRead(wrong, f, f.effects()); err == nil || !strings.Contains(err.Error(), ReadUngatedCode) {
		t.Fatalf("wrong-tree gate: %v", err)
	}
	f.expectStart(f.root)
	f.expect("RawEntries", f.root, f.unit)
	if _, _, err := commitRead(req, f, f.effects()); err == nil || !strings.Contains(err.Error(), ReadTestsUnnamedCode) {
		t.Fatalf("unnamed changed test: %v", err)
	}
	req.TestsChanged = []TestChange{{Path: f.path, ReaderWord: "assertions still prove the intended behavior"}}
	f.expectStart(f.root)
	f.expectAfterGate(f.root, false)
	tip, att, err := commitRead(req, f, f.effects())
	if err != nil || tip != f.tip || len(att.TestsChanged) != 1 || att.TestsChanged[0] != req.TestsChanged[0] {
		t.Fatalf("named read tip=%s att=%+v err=%v", tip, att, err)
	}
	if len(f.snapshots[f.tip]) != 2 {
		t.Fatalf("published snapshot=%v", f.snapshots[f.tip])
	}
}
func TestReadInstallRefusalLeavesNothingStaged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		stage, install, restore error
		want                    string
	}{
		{name: "install", install: errors.New("install refused"), want: "install refused"},
		{name: "stage", stage: errors.New("stage refused"), want: "stage refused"},
		{name: "stage and restore failure", stage: errors.New("stage refused"), restore: errors.New("index restore refused"), want: "stage refused"},
		{name: "restore failure retains install cause", install: errors.New("install refused"), restore: errors.New("index restore refused"), want: "install refused"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
			original := f.writeReaderRecord()
			f.stageErr, f.installErr, f.restoreErr = tc.stage, tc.install, tc.restore
			f.expectStart(f.root)
			f.expectAfterGate(f.root, false)
			if tc.stage != nil {
				f.expected = f.expected[:len(f.expected)-1]
			}
			f.expect("RestoreIndex", f.root, f.index)
			_, _, err := commitRead(f.request(false), f, f.effects())
			if err == nil || !strings.Contains(err.Error(), tc.want) || (tc.restore != nil && !strings.Contains(err.Error(), "index restore refused")) {
				t.Fatalf("rollback error %v", err)
			}
			for path := range f.generated {
				if _, statErr := os.Stat(filepath.Join(f.root, filepath.FromSlash(path))); !os.IsNotExist(statErr) {
					t.Fatalf("generated %s remains: %v", path, statErr)
				}
			}
			actual, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(f.record)))
			if readErr != nil || !bytes.Equal(actual, original) {
				t.Fatalf("reader record %q, %v", actual, readErr)
			}
		})
	}
}
func TestGoalBranchReadRedGateAndUncleanClosureDispatchNothingFurther(t *testing.T) {
	t.Parallel()
	t.Run("red gate", func(t *testing.T) {
		t.Parallel()
		f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
		f.expectBranchStart()
		f.expect("Detached", f.root, f.unit)
		delegates := 0
		_, err := RunBranchRead(BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.unit,
			GoalID: "goal-a", UnitCommit: f.unit, Repository: branchPolicyRepository{f}, CheckClaim: func() error { return nil },
			Gate: func(dir string) (string, error) {
				if dir != f.detached {
					t.Fatalf("gate workspace %s", dir)
				}
				return "go gate: staticcheck red", errors.New("exit 1")
			},
			Delegate: func(string, string, string, string, string) (string, error) { delegates++; return f.job, nil }})
		if err == nil || !strings.Contains(err.Error(), ReadUngatedCode) || !strings.Contains(err.Error(), "staticcheck red") || delegates != 0 {
			t.Fatalf("red gate err=%v delegates=%d", err, delegates)
		}
		if _, statErr := os.Stat(f.detached); !os.IsNotExist(statErr) {
			t.Fatalf("detached workspace remains: %v", statErr)
		}
	})
	t.Run("open finding", func(t *testing.T) {
		t.Parallel()
		f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
		f.expectBranchStart()
		f.expect("Detached", f.root, f.unit)
		f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
		delegates := 0
		req := BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.unit, GoalID: "goal-a", UnitCommit: f.unit,
			Repository: branchPolicyRepository{f}, CheckClaim: func() error { return nil }, Gate: func(string) (string, error) { return "green", nil },
			NewID: func(prefix string) (string, error) {
				if prefix != "goal-read-gate" {
					t.Fatalf("id prefix %s", prefix)
				}
				return "fast-open", nil
			},
			Delegate: func(string, string, string, string, string) (string, error) {
				delegates++
				f.writeJob("completed", true)
				return f.job, nil
			}}
		if result, err := RunBranchRead(req); err != nil || result.State != "dispatched" {
			t.Fatalf("dispatch result=%+v err=%v", result, err)
		}
		f.expectBranchStart()
		f.expect("BranchEntries", f.root, f.unit)
		req.Collect = true
		req.Commit = func(got CommitReadRequest) (string, Attestation, error) {
			if got.RootJob != f.job || got.GateTree != f.tree || got.GateRunID != "fast-open" {
				t.Fatalf("collect request %+v", got)
			}
			f.expectStart(f.root)
			f.expect("RawEntries", f.root, f.unit)
			f.expect("Range", f.root, f.base, f.unit, "goal-a")
			return commitRead(got, f, f.effects())
		}
		if _, err := RunBranchRead(req); err == nil || !strings.Contains(err.Error(), "clean") {
			t.Fatalf("open finding collect: %v", err)
		}
		if delegates != 1 {
			t.Fatalf("delegate calls %d", delegates)
		}
		if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(attestationPath("goal-a", f.unit)))); !os.IsNotExist(err) {
			t.Fatalf("unclean closure wrote attestation: %v", err)
		}
	})
}
func (f *attestationPolicyFixture) expectBind(repo, snapshot, before, after string, plan, complete bool, legacy ...bool) {
	f.expect("CommitExists", repo, f.unit)
	f.expect("Kind", repo, f.unit, "goal-a")
	f.expect("SnapshotFile", repo, snapshot, attestationPath("goal-a", f.unit))
	f.expectValidation(repo, snapshot, true, legacy...)
	if len(legacy) == 0 {
		f.expect("SnapshotFile", repo, snapshot, closureBundlePath("goal-a", f.unit)) // goal revision
	}
	f.expect("Prefix", repo)
	f.expect("Transition", repo, before, after)
	if plan {
		f.expect("RawEntries", repo, f.plan)
		path := "plans/goal-a.md"
		f.expect("TreeEntry", repo, before, path)
		f.expect("TreeEntry", repo, after, path)
		f.expect("TreeEntry", repo, f.plan, "metasystem/"+path)
	}
	if complete {
		f.expect("Transition", repo, before, after)
	}
}
func TestGoalBranchReadRunsGateDispatchesAndCollectsClosedCritic(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	gateCalls, delegateCalls := 0, 0
	req := BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.unit, GoalID: "goal-a", UnitCommit: f.unit,
		Repository: branchPolicyRepository{f}, CheckClaim: func() error { return nil },
		Gate: func(dir string) (string, error) {
			gateCalls++
			data, err := os.ReadFile(filepath.Join(dir, "code.go"))
			if dir != f.detached || err != nil || string(data) != "declared unit tree\n" {
				t.Fatalf("gate workspace %s", dir)
			}
			return "go gate: fast mode passed", nil
		},
		Delegate: func(brief, goal, commit, runtime, model string) (string, error) {
			delegateCalls++
			body, err := os.ReadFile(brief)
			if err != nil || goal != "goal-a" || commit != f.unit || runtime != "" || model != "" || !strings.Contains(string(body), "git diff "+f.unit+"^ "+f.unit) || strings.Contains(string(body), "goals-live-on-branches-design.md") {
				t.Fatalf("brief=%q goal=%s commit=%s err=%v", body, goal, commit, err)
			}
			f.writeJob("running", false)
			return f.job, nil
		}, NewID: func(prefix string) (string, error) {
			if prefix != "goal-read-gate" {
				t.Fatalf("id prefix %s", prefix)
			}
			return "fast-unit-tree", nil
		}}
	f.expectBranchStart()
	f.expect("Detached", f.root, f.unit)
	f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
	result, err := RunBranchRead(req)
	if err != nil || result.State != "dispatched" || result.RootJob != f.job || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("dispatch result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	if _, err := os.Stat(f.detached); !os.IsNotExist(err) {
		t.Fatalf("temporary workspace remains: %v", err)
	}
	req.Collect = true
	f.expectBranchStart()
	result, err = RunBranchRead(req)
	if err != nil || result.State != "open" || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("open result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	f.writeJob("completed", false)
	f.expectBranchStart()
	f.expect("BranchEntries", f.root, f.unit)
	req.Commit = func(got CommitReadRequest) (string, Attestation, error) {
		if got.RootJob != f.job || got.GateRunID != "fast-unit-tree" || got.GateTree != f.tree || len(got.TestsChanged) != 0 {
			t.Fatalf("collect request %+v", got)
		}
		f.expectStart(f.root)
		f.expectAfterGate(f.root, true)
		return commitRead(got, f, f.effects())
	}
	result, err = RunBranchRead(req)
	if err != nil || result.State != "collected" || result.AttestationCommit != f.tip || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("collect result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	f.expectValidation(f.root, f.tip, true)
	att, err := validateAttestation(f, f.root, f.tip, f.base, "goal-a", "u1", f.unit, map[string]bool{})
	if err != nil || att.Source.RootJob != f.job || att.Gate.RunID != "fast-unit-tree" {
		t.Fatalf("attestation=%+v err=%v", att, err)
	}
	before, after, extra := policyID("5"), policyID("6"), policyID("7")
	f.transitions[before+":"+after] = f.raw[f.unit]
	f.transitions[before+":"+extra] = append(append([]byte(nil), f.raw[f.unit]...), policyRaw("extra.txt", policyID("8"), policyID("9"))...)
	f.expectBind(f.root, f.tip, before, after, false, true)
	bound, err := bindLandedUnit(f, f.root, f.tip, f.base, "goal-a", f.unit, before, after)
	if err != nil || bound.CriticRoot != f.job || bound.GoalRevision != 1 || bound.Digest != att.Subject.UnitDigest {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
	f.expectBind(f.root, f.tip, before, extra, false, true)
	if _, err := bindLandedUnit(f, f.root, f.tip, f.base, "goal-a", f.unit, before, extra); err == nil || !strings.Contains(err.Error(), "does not match attested digest") {
		t.Fatalf("extra candidate path: %v", err)
	}
	nested := filepath.Join(f.root, "metasystem")
	nestedBefore, nestedAfter := policyID("0"), policyID("8")
	f.transitions[nestedBefore+":"+nestedAfter] = policyRaw("code.go", policyID("3"), policyID("4"))
	f.expectBind(nested, f.tip, nestedBefore, nestedAfter, false, true)
	bound, err = bindLandedUnit(f, nested, f.tip, f.base, "goal-a", f.unit, nestedBefore, nestedAfter)
	if err != nil || bound.Digest != att.Subject.UnitDigest {
		t.Fatalf("nested bound=%+v err=%v", bound, err)
	}
}
func TestBindLandedUnitRefusesOmittedFold(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", true)
	f.writeJob("completed", false)
	f.expectStart(f.root)
	f.expectAfterGate(f.root, true)
	_, _, err := commitRead(f.request(true), f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(f.root, "metasystem")
	before, omitted, applied := policyID("5"), policyID("6"), policyID("7")
	for _, after := range []string{omitted, applied} {
		f.transitions[before+":"+after] = policyRaw("code.go", policyID("3"), policyID("4"))
	}
	planPath := "plans/goal-a.md"
	f.treeEntries[before+":"+planPath] = "100644 blob " + policyID("1")
	f.treeEntries[omitted+":"+planPath] = "100644 blob " + policyID("1")
	f.treeEntries[applied+":"+planPath] = "100644 blob " + policyID("2")
	f.treeEntries[f.plan+":metasystem/"+planPath] = "100644 blob " + policyID("2")
	f.expectBind(nested, f.tip, before, omitted, true, false)
	if _, err := bindLandedUnit(f, nested, f.tip, f.base, "goal-a", f.unit, before, omitted); err == nil || !strings.Contains(err.Error(), planPath) {
		t.Fatalf("omitted fold: %v", err)
	}
	f.expectBind(nested, f.tip, before, applied, true, true)
	bound, err := bindLandedUnit(f, nested, f.tip, f.base, "goal-a", f.unit, before, applied)
	if err != nil || !bound.HasPlan || !reflect.DeepEqual(bound.FoldPaths, []string{planPath}) {
		t.Fatalf("applied fold=%+v err=%v", bound, err)
	}
}
func policyCanonical(t *testing.T, value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
func policySnapshotCopy(source map[string][]byte) map[string][]byte {
	copy := map[string][]byte{}
	for path, data := range source {
		copy[path] = append([]byte(nil), data...)
	}
	return copy
}
func TestCriticAttestationSurvivesFreshCloneWithoutJobStore(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeJob("completed", false)
	f.expectStart(f.root)
	f.expectAfterGate(f.root, true)
	_, att, err := commitRead(f.request(true), f, f.effects())
	if err != nil || att.Source.ClosureSHA256 == "" {
		t.Fatalf("portable read source=%+v err=%v", att.Source, err)
	}
	f.clone = filepath.Join(t.TempDir(), "fresh")
	if err := os.Mkdir(f.clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.clone, "artifacts", "agents")); !os.IsNotExist(err) {
		t.Fatalf("fresh clone has job store: %v", err)
	}
	before, after := policyID("5"), policyID("6")
	f.transitions[before+":"+after] = f.raw[f.unit]
	f.expectBind(f.clone, f.tip, before, after, false, true)
	if _, err := bindLandedUnit(f, f.clone, f.tip, f.base, "goal-a", f.unit, before, after); err != nil {
		t.Fatalf("portable closure in fresh clone: %v", err)
	}
	bundlePath := closureBundlePath("goal-a", f.unit)
	attPath := attestationPath("goal-a", f.unit)
	tampered := policyID("7")
	f.snapshots[tampered] = policySnapshotCopy(f.snapshots[f.tip])
	f.snapshots[tampered][bundlePath] = append(f.snapshots[tampered][bundlePath], ' ')
	f.expectValidation(f.clone, tampered, true)
	if _, err := validateAttestation(f, f.clone, tampered, f.base, "goal-a", "u1", f.unit, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "fails its digest") {
		t.Fatalf("tampered bundle: %v", err)
	}
	for i, unsafe := range []string{"../escape.json", "/absolute.json", "C:/absolute.json"} {
		snapshot := policyID([]string{"8", "9", "0"}[i])
		f.snapshots[snapshot] = policySnapshotCopy(f.snapshots[f.tip])
		var bundle closureBundle
		if err := json.Unmarshal(f.snapshots[snapshot][bundlePath], &bundle); err != nil {
			t.Fatal(err)
		}
		bundle.Files[unsafe] = "{}\n"
		malicious := policyCanonical(t, bundle)
		f.snapshots[snapshot][bundlePath] = malicious
		changed := att
		changed.Source.ClosureSHA256 = policyHash(malicious)
		changed.SHA256 = ""
		changed.SHA256, err = digestAttestation(changed)
		if err != nil {
			t.Fatal(err)
		}
		f.snapshots[snapshot][attPath] = policyCanonical(t, changed)
		f.expectValidation(f.clone, snapshot, true)
		if _, err := validateAttestation(f, f.clone, snapshot, f.base, "goal-a", "u1", f.unit, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "unsafe path") {
			t.Fatalf("unsafe path %q: %v", unsafe, err)
		}
	}
	legacy := policyID("f")
	f.snapshots[legacy] = policySnapshotCopy(f.snapshots[f.tip])
	old := att
	old.Source.ClosureSHA256 = ""
	old.SHA256 = ""
	old.SHA256, err = digestAttestation(old)
	if err != nil {
		t.Fatal(err)
	}
	f.snapshots[legacy][attPath] = policyCanonical(t, old)
	delete(f.snapshots[legacy], bundlePath)
	nested := filepath.Join(f.root, "nested")
	for _, repo := range []string{f.root, f.clone, nested} {
		if repo == nested {
			t.Run("nested request repository owns critic job store", func(t *testing.T) {
				agents := filepath.Join("artifacts", "agents")
				if err := os.MkdirAll(filepath.Join(nested, "artifacts"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(f.root, agents), filepath.Join(nested, agents)); err != nil {
					t.Fatal(err)
				}
				source, bundle, err := directSource(f, CommitReadRequest{Repo: nested, RootJob: f.job}, att.Subject, f.subject)
				if err != nil || source.ClosureSHA256 == "" || source.Round != 1 || len(bundle) == 0 {
					t.Fatalf("nested direct critic source=%+v bundle=%d err=%v", source, len(bundle), err)
				}
				if _, _, err := directSource(f, CommitReadRequest{Repo: f.root, RootJob: f.job}, att.Subject, f.subject); err == nil {
					t.Fatal("parent unexpectedly resolved the nested critic job")
				}
				f.expectBind(nested, legacy, before, after, false, true, true)
				bound, err := bindLandedUnit(f, nested, legacy, f.base, "goal-a", f.unit, before, after)
				if err != nil || bound.GoalRevision != 1 || bound.CriticRoot != f.job || bound.Digest != old.Subject.UnitDigest {
					t.Fatalf("nested legacy binding=%+v err=%v", bound, err)
				}
			})
			continue
		}
		f.expectValidation(repo, legacy, true, true)
		_, err := validateAttestation(f, repo, legacy, f.base, "goal-a", "u1", f.unit, map[string]bool{})
		if repo != f.clone && err != nil {
			t.Fatalf("legacy with local job files: %v", err)
		}
		if repo == f.clone && (err == nil || !strings.Contains(err.Error(), "code-critic root")) {
			t.Fatalf("legacy without job files: %v", err)
		}
	}
}
