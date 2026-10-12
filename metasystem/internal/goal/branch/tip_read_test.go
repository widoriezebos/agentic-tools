package branch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type tipSubjectRepository struct {
	BranchReadRepository
	commits []Commit
}

func (r tipSubjectRepository) Range(string, string, string, string) ([]Commit, error) {
	return r.commits, nil
}

func TestUnitReviewTipAfterRecordsKeepsItsSubject(t *testing.T) {
	t.Parallel()
	for _, kind := range []Kind{Plan, Read} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
			tip := policyID("9")
			repo := tipSubjectRepository{BranchReadRepository: branchPolicyRepository{f}, commits: append(f.rangeFacts(), Commit{ID: tip, Kind: kind})}
			req := BranchReadRequest{Repo: f.root, EndpointTip: f.base, BranchTip: tip, GoalID: "goal-a", UnitCommit: tip, Units: []string{"U1", "U5", "U8"}}
			info, err := branchReadSubject(repo, req)
			if err != nil || info.Unit != "u1" {
				t.Fatalf("tip following a %s could not be read: %+v %v", kind, info, err)
			}
			common := filepath.Join(f.root, ".git")
			path := filepath.Join(common, "metasystem", "goal-reads", "goal-a", tip+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := saveBranchReadRecord(common, path, branchReadRecord{SchemaVersion: 1, Goal: "goal-a", UnitCommit: tip, RootJob: "critic-tip", CoveredUnits: req.Units}); err != nil {
				t.Fatal(err)
			}
			req.Units, req.BranchTip = nil, policyID("8")
			f.expect("CommonDir", f.root)
			if _, err := branchReadSubject(repo, req); err != nil {
				t.Fatalf("flag-free collection after a later record: %v", err)
			}
		})
	}
}

func TestUnitReviewStoresAndFreezesCoveredUnits(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.expectBranchStart()
	f.expect("Detached", f.root, f.unit)
	f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
	units := []string{"U1", "U5", "U8"}
	req := BranchReadRequest{Repo: f.root, EndpointTip: f.base, BranchTip: f.unit, UnitCommit: f.unit, GoalID: "goal-a",
		Units: []string{"U8", "U1", "U5"}, Repository: branchPolicyRepository{f}, CheckClaim: func() error { return nil },
		Gate: func(string) (string, error) { return "green", nil }, NewID: func(string) (string, error) { return "tip-gate", nil },
		Delegate: func(brief, _, _, _, _ string) (string, error) {
			data, err := os.ReadFile(brief)
			if err != nil || !strings.Contains(string(data), "U1, U5, U8") || !strings.Contains(string(data), "git diff "+f.base+"..."+f.unit+" --") {
				t.Fatalf("critic did not receive complete coverage: %s %v", data, err)
			}
			f.writeJob("completed", false)
			return f.job, nil
		}}
	if result, err := RunBranchRead(req); err != nil || result.State != "dispatched" {
		t.Fatalf("dispatch: %+v %v", result, err)
	}
	recordPath := filepath.Join(f.root, ".git", "metasystem", "goal-reads", "goal-a", f.unit+".json")
	record, err := loadBranchReadRecord(recordPath)
	if err != nil || !slices.Equal(record.CoveredUnits, units) {
		t.Fatalf("record lost units: %+v %v", record, err)
	}
	f.expectBranchStart()
	req.Units = []string{"U5", "U8", "U1"}
	if result, err := RunBranchRead(req); err != nil || result.State != "closed" {
		t.Fatalf("reordering the same covered units changed the read: %+v %v", result, err)
	}
	f.expectBranchStart()
	req.Units = []string{"U1", "U5"}
	if _, err := RunBranchRead(req); err == nil || !strings.Contains(err.Error(), "other units") {
		t.Fatalf("started read was relabelled: %v", err)
	}
	req.Units, req.Collect = nil, true
	f.expectBranchStart()
	f.expect("BranchEntries", f.root, f.unit)
	req.Commit = func(got CommitReadRequest) (string, Attestation, error) {
		if !slices.Equal(got.CoveredUnits, units) || got.SubjectCommit != f.unit {
			t.Fatalf("flag-free collection lost stored coverage: %+v", got)
		}
		return f.tip, Attestation{}, nil
	}
	if result, err := RunBranchRead(req); err != nil || result.State != "collected" {
		t.Fatalf("collect: %+v %v", result, err)
	}
}

func TestUnitReviewRejectsInvalidCoveredNamesBeforeGate(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{{"U1", "U1"}, {"U1", ""}, {"../U1"}} {
		t.Run(strings.Join(names, ","), func(t *testing.T) {
			t.Parallel()
			f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
			f.expectBranchStart()
			_, err := RunBranchRead(BranchReadRequest{Repo: f.root, EndpointTip: f.base, BranchTip: f.unit, UnitCommit: f.unit, GoalID: "goal-a", Units: names,
				Repository: branchPolicyRepository{f}, CheckClaim: func() error { return nil }})
			if err == nil || !strings.Contains(err.Error(), "unit names") {
				t.Fatalf("invalid coverage reached the gate: %v", err)
			}
		})
	}
}

func TestUnitReviewAttestationRetainsCoveredUnits(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeJob("completed", false)
	req := f.request(true)
	req.CoveredUnits, req.SubjectCommit = []string{"U1", "U2", "U3"}, f.unit
	f.ancestors = map[string]bool{f.unit + ":" + f.unit: true}
	f.expect("Inspect")
	f.expect("Range", f.root, f.base, f.unit, "goal-a")
	f.expect("IsAncestor", f.root, f.unit, f.unit)
	f.expect("ReadSubject", f.root, f.unit)
	f.expect("RawEntries", f.root, f.unit)
	f.expectAfterGate(f.root, true)
	_, att, err := commitRead(req, f, f.effects())
	if err != nil || !slices.Equal(att.CoveredUnits, req.CoveredUnits) {
		t.Fatalf("attestation lost coverage: %+v %v", att, err)
	}
	var stored Attestation
	if err := json.Unmarshal(f.generated[attestationPath("goal-a", f.unit)], &stored); err != nil || !slices.Equal(stored.CoveredUnits, req.CoveredUnits) {
		t.Fatalf("committed record lost coverage: %+v %v", stored, err)
	}
}

func TestUnitReviewCoverageRequiresCleanAncestorRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source   string
		failed, future bool
	}{
		{name: "clean", source: "critic-root"},
		{name: "failed", source: "critic-root", failed: true},
		{name: "reader prose", source: "reader-record"},
		{name: "subject outside ancestry", source: "critic-root", future: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			commits := []Commit{{ID: statusU1, Kind: Unit, Unit: "U1", Units: []string{"U1"}},
				{ID: statusU2, Kind: Unit, Unit: "U2", Units: []string{"U2"}}, {ID: statusTip, Kind: Plan},
				{ID: statusR3, Kind: Read, Unit: "U2"}, {ID: statusNext, Kind: Unit, Unit: "U1", Units: []string{"U1"}}}
			subject := statusTip
			if tc.future {
				subject = statusBase
			}
			facts := []statusFact{rangeFact(repo, statusBase, statusR3, "goal-a", commits, nil),
				kindFact(repo, statusR3, "goal-a", KindInfo{Kind: Read, CommitID: subject}, nil)}
			var readErr error
			if tc.failed {
				readErr = errors.New("findings remain open")
			}
			if !tc.future {
				facts = append(facts, attestFact(repo, statusR3, statusBase, "goal-a", "U2", subject,
					Attestation{CoveredUnits: []string{"U1", "U2"}, Source: AttestationSource{Kind: tc.source}}, readErr))
			}
			f := newStatusFacts(t, facts...)
			status, err := inspectStatus(repo, statusBase, statusR3, "goal-a", f.dependencies())
			want := 0
			if tc.name == "clean" {
				want = 2
			}
			if err != nil || status.Prefix != want || status.Units[2].Resolved() || len(status.CleanReads) != want/2 {
				t.Fatalf("coverage crossed its clean ancestry: %+v %v", status, err)
			}
		})
	}
}

func TestUnitReviewNamedCoverageNeedsANewReadInsteadOfCarry(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeJob("completed", false)
	req := f.request(true)
	req.CoveredUnits = []string{"U1", "U5", "U8"}
	f.expectStart(f.root)
	f.expectAfterGate(f.root, true)
	if _, _, err := commitRead(req, f, f.effects()); err != nil {
		t.Fatal(err)
	}
	req.CoveredUnits, req.Carry = nil, f.unit
	f.expectStart(f.root)
	f.expect("RawEntries", f.root, f.unit)
	f.expect("Range", f.root, f.base, f.unit, "goal-a")
	f.expect("TopLevel", f.root)
	f.expect("ReadSubject", f.root, f.unit)
	f.expect("RawEntries", f.root, f.unit)
	f.expect("Range", f.root, f.base, f.unit, "goal-a")
	f.expect("RawEntries", f.root, f.unit)
	f.expect("TopLevel", f.root)
	if _, _, err := commitRead(req, f, f.effects()); err == nil || !strings.Contains(err.Error(), "new examination") || !strings.Contains(err.Error(), "--units U1,U5,U8") {
		t.Fatalf("a carried per-unit digest certified the other units: %v", err)
	}
}

func TestWorkLandExceptionCandidateNeedsNoReviewedBuild(t *testing.T) {
	t.Parallel()
	f := newLandingFacts(t, map[string]string{"base.txt": "base", "metasystem/memory/receipts.log": "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"})
	f.composition = true
	tip := f.unit(f.base, "U1", "metasystem/code.go", "manual work\n")
	f.units, f.folds = []string{tip}, [][]string{{}}
	req := f.request(f.base, tip, "", "")
	req.CandidateOnly, req.ExceptionCandidate, req.LandingReady = true, true, false
	f.expectPending(tip, 1)
	f.expect("close")
	if result, err := prepareLanding(req, f.repository()); err != nil || result.Candidate != f.projected(f.nodes[tip].tree) || result.Endpoint != f.base {
		t.Fatalf("exception candidate did not compose the branch tip: %+v %v", result, err)
	}
	f.consumed()
	req.CandidateOnly = false
	if _, err := prepareLanding(req, f.repository()); err == nil || !strings.Contains(err.Error(), "cannot prepare") {
		t.Fatalf("exception composition granted delivery: %v", err)
	}
}

func TestUnitReviewRangeAcceptsOnlyNamedRecordTip(t *testing.T) {
	t.Parallel()
	for _, named := range []bool{true, false} {
		t.Run(map[bool]string{true: "named", false: "unnamed"}[named], func(t *testing.T) {
			t.Parallel()
			unit, plan := strings.Repeat("a", 40), strings.Repeat("b", 40)
			raw := rawTree("metasystem/code.go")
			calls := rangeTranscript("base", "tip", "base", unit+" base\n"+plan+" "+unit+"\ntip "+plan+"\n")
			calls = append(calls, rangeOutput("Goal-Unit: goal-a/u1\n", "show", "-s", "--format=%(trailers:only,unfold=true)", unit))
			calls = append(calls, treeCalls(unit, "base", raw)...)
			calls = append(calls, treeCalls(unit, "base", raw)...)
			calls = append(calls, rangeOutput("Goal-Plan: goal-a\n", "show", "-s", "--format=%(trailers:only,unfold=true)", plan))
			calls = append(calls, treeCalls(plan, unit, rawTree("metasystem/plans/note.md"))...)
			calls = append(calls, rangeOutput("Goal-Read: goal-a/u1 "+plan+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
			calls = append(calls, treeCalls("tip", plan, rawTree(attestationPath("goal-a", plan)))...)
			calls = append(calls, rangeOutput("Goal-Plan: goal-a\n", "show", "-s", "--format=%(trailers:only,unfold=true)", plan))
			att := Attestation{Subject: AttestationSubject{Commit: plan}}
			if named {
				att.CoveredUnits = []string{"U1", "U5", "U8"}
			}
			data, err := json.Marshal(att)
			if err != nil {
				t.Fatal(err)
			}
			calls = append(calls, rangeOutput(string(data), "show", "tip:"+attestationPath("goal-a", plan)))
			stub := testgit.New(t, calls...)
			commits, err := ValidateRangeWithGit(rangeRepo, "base", "tip", "goal-a", rangeReader(stub))
			if named && (err != nil || len(commits) != 3) || !named && err == nil {
				t.Fatalf("record tip coverage was ignored: named=%v commits=%+v err=%v", named, commits, err)
			}
		})
	}
}

func TestUnitReviewCoverageNeedsEveryCompositeName(t *testing.T) {
	t.Parallel()
	read := Attestation{CoveredUnits: []string{"U1"}}
	for _, unit := range []UnitStatus{{Unit: "U1+U5"}, {Unit: "U1+U5", Units: []string{"U1", "U5"}}} {
		if read.CoversUnit(unit) {
			t.Fatal("partial coverage admitted a composite build")
		}
		read.CoveredUnits = []string{"U1", "U5"}
		if !read.CoversUnit(unit) {
			t.Fatal("complete coverage rejected a composite build")
		}
		read.CoveredUnits = []string{"U1"}
	}
}
