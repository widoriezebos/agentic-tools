package branch

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestTransferredSourceRequiresPublishedDestinationCoverage(t *testing.T) {
	t.Parallel()
	o := goal.ReviewObligation{OriginalEvidence: readsubject.Finding{ID: "read-1:1", Class: "regression", Severity: "high", Material: true, Where: "source.go", Claim: "Source requirement is incomplete", Evidence: "Required source behavior fails", Change: "Complete the source requirement"}, Finding: "read-1:1", Chain: "critic", SourceUnit: "source", TargetUnit: "destination", OriginalFinding: "read-1:1", OriginalRead: "read-1", SourceCommit: statusU1, State: "discharged", CoverageRead: "read-2", CoverageCommit: statusU2, TransferredOnce: true, StopReference: "stop-1"}
	att := Attestation{Unit: "destination", Subject: AttestationSubject{Commit: statusU2}, Source: AttestationSource{ReadLaunch: "read-2"}, CoversFindings: []string{"read-1:1"}, CoversCommits: []string{statusU1}}
	deps := statusDependencies{
		validatedRange: func(string, string, string, string) ([]Commit, error) {
			return []Commit{{ID: statusU1, Kind: Unit, Unit: "source", Units: []string{"source"}}, {ID: statusU2, Kind: Unit, Unit: "destination"}, {ID: statusR1, Kind: Read, Unit: "destination"}}, nil
		},
		kind:                func(string, string, string) (KindInfo, error) { return KindInfo{Kind: Read, CommitID: statusU2}, nil },
		attestation:         func(string, string, string, string, string, string) (Attestation, error) { return att, nil },
		transferObligations: func(string, string, string) []goal.ReviewObligation { return []goal.ReviewObligation{o} },
	}
	status, err := inspectStatus("fixture", statusBase, statusTip, "goal-a", deps)
	if err != nil || status.Prefix != 2 || status.Units[0].ReadState != "read transferred" {
		t.Fatalf("covered source status: %+v %v", status, err)
	}

	att.CoversFindings = nil
	status, err = inspectStatus("fixture", statusBase, statusTip, "goal-a", deps)
	if err != nil || status.Prefix != 0 || status.Units[0].ReadState != "built" {
		t.Fatalf("unrelated clean read cleared source: %+v %v", status, err)
	}
	att.CoversFindings = []string{"read-1:1"}
	att.CoversCommits = nil
	status, err = inspectStatus("fixture", statusBase, statusTip, "goal-a", deps)
	if err != nil || status.Prefix != 0 {
		t.Fatalf("missing retained change cleared source: %+v %v", status, err)
	}
	att.CoversCommits = []string{statusU1}
	o.State = "open"
	status, err = inspectStatus("fixture", statusBase, statusTip, "goal-a", deps)
	if err != nil || status.Prefix != 0 {
		t.Fatalf("unpublished discharge cleared source: %+v %v", status, err)
	}
}

func TestDestinationAttestationPublishesInheritedCoverage(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeReaderRecord()
	f.ancestors = map[string]bool{f.base + ":" + f.unit: true}
	req := f.request(false)
	req.CoversFindings, req.CoversCommits = []string{"read-1:1"}, []string{f.base}
	f.expectStart(f.root)
	f.expectAfterGate(f.root, false)
	for i, call := range f.expected {
		if call.method == "StagedPaths" {
			f.expected = append(f.expected, policyCall{})
			copy(f.expected[i+1:], f.expected[i:])
			f.expected[i] = policyCall{method: "IsAncestor", args: []string{f.root, f.base, f.unit}}
			break
		}
	}
	_, att, err := commitRead(req, f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	var published Attestation
	if err := json.Unmarshal(f.snapshots[f.tip][attestationPath("goal-a", f.unit)], &published); err != nil {
		t.Fatal(err)
	}
	if len(published.CoversFindings) != 1 || published.CoversFindings[0] != "read-1:1" || len(published.CoversCommits) != 1 || published.CoversCommits[0] != f.base {
		t.Fatalf("published coverage: %+v", published)
	}
	changed := att
	changed.CoversFindings = []string{"unrelated:1"}
	digest, err := digestAttestation(changed)
	if err != nil || digest == att.SHA256 {
		t.Fatalf("coverage omitted from attestation digest: %s %v", digest, err)
	}
}

func TestCanonicalReadBindsDigestCountAndSubject(t *testing.T) {
	t.Parallel()
	read := readsubject.Read{ID: "reader", Engine: "fixture-engine", Model: "reader-model", Subject: readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: statusU2, Tree: statusTip}, Findings: []readsubject.Finding{}}
	data, digest := read.Canonical()
	if err := validateCanonicalRead(data, digest, statusTip); err != nil {
		t.Fatal(err)
	}
	if err := validateCanonicalRead(data, digest, statusBase); err == nil {
		t.Fatal("foreign tree admitted")
	}
	if err := validateCanonicalRead(data, "changed", statusTip); err == nil {
		t.Fatal("corrupt digest admitted")
	}
	read.Material = 1
	data, digest = read.Canonical()
	if err := validateCanonicalRead(data, digest, statusTip); err == nil {
		t.Fatal("inconsistent material count admitted")
	}
	if err := validateCanonicalRead(nil, "", statusTip); err != nil {
		t.Fatalf("legacy evidence revoked: %v", err)
	}
}

func TestTransferredDestinationCriticRequiresExplicitCoverage(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	inherited := goal.ReviewObligation{OriginalEvidence: readsubject.Finding{ID: "source-read:1", Class: "regression", Severity: "high", Material: true, Where: "source.go", Claim: "Source requirement is incomplete", Evidence: "Required source behavior fails", Change: "Complete the source requirement"}, Finding: "source-read:1", Chain: "source-critic", SourceUnit: "source", TargetUnit: "u1", OriginalRead: "source-read", OriginalFinding: "source-read:1", SourceCommit: f.base, StopReference: "stop-1", TransferredOnce: true}
	req := BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.unit, GoalID: "goal-a", UnitCommit: f.unit, Repository: branchPolicyRepository{f}, InheritedFindings: []goal.ReviewObligation{inherited}, CheckClaim: func() error { return nil }, Gate: func(string) (string, error) { return "green", nil }, NewID: func(string) (string, error) { return "gate-transfer", nil }, Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
		data, err := os.ReadFile(brief)
		if err != nil || !strings.Contains(string(data), "git diff "+f.base+"^ "+f.unit) || !strings.Contains(string(data), "source-read:1") {
			t.Fatalf("inherited read brief: %s %v", data, err)
		}
		f.writeJob("completed", false)
		return f.job, nil
	}}
	f.expectBranchStart()
	f.expect("Detached", f.root, f.unit)
	f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
	if result, err := RunBranchRead(req); err != nil || result.State != "dispatched" {
		t.Fatalf("dispatch: %+v %v", result, err)
	}
	req.Collect = true
	f.expectBranchStart()
	f.expect("BranchEntries", f.root, f.unit)
	if _, err := RunBranchRead(req); err == nil || !strings.Contains(err.Error(), "does not explicitly cover inherited finding") {
		t.Fatalf("unrelated clean read admitted: %v", err)
	}
	f.write("artifacts/agents/"+f.job+"/rounds/1/return.json", policyCanonical(t, map[string]any{"jobId": f.job, "round": 1, "reviewedTree": f.tree, "findings": []any{map[string]any{"resolves": "source-read:1", "material": false}}}))
	req.Commit = func(got CommitReadRequest) (string, Attestation, error) {
		if len(got.CoversFindings) != 1 || got.CoversFindings[0] != inherited.OriginalFinding || len(got.CoversCommits) != 1 || got.CoversCommits[0] != f.base {
			t.Fatalf("collector discarded explicit coverage: %+v", got)
		}
		f.ancestors = map[string]bool{f.base + ":" + f.unit: true}
		f.expectStart(f.root)
		f.expectAfterGate(f.root, true)
		for i, call := range f.expected {
			if call.method == "StagedPaths" {
				f.expected = append(f.expected, policyCall{})
				copy(f.expected[i+1:], f.expected[i:])
				f.expected[i] = policyCall{method: "IsAncestor", args: []string{f.root, f.base, f.unit}}
				break
			}
		}
		return commitRead(got, f, f.effects())
	}
	f.expectBranchStart()
	f.expect("BranchEntries", f.root, f.unit)
	if result, err := RunBranchRead(req); err != nil || result.State != "collected" {
		t.Fatalf("covered read collection: %+v %v", result, err)
	}
}

type transferBindingFacts struct {
	*attestationPolicyFixture
	source string
}

func (f transferBindingFacts) Kind(repo, commit, goalID string) (KindInfo, error) {
	if commit == f.source {
		f.next("Kind", repo, commit, goalID)
		return KindInfo{Kind: Unit, Unit: "source", Units: []string{"source"}}, nil
	}
	return f.attestationPolicyFixture.Kind(repo, commit, goalID)
}

func TestTransferredSourceLandsThroughDestinationEvidence(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/correction.go", false)
	source := policyID("7")
	sourceTree := policyID("8")
	f.subjects[source] = readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: source, Parent: f.base, Tree: sourceTree, DiffDigest: policyHash([]byte("source patch"))}
	f.raw[source] = policyRaw("metasystem/source.go", policyID("1"), policyID("2"))
	f.ranges = map[string][]Commit{f.base + ":" + source: {{ID: source, Kind: Unit, Unit: "source", Units: []string{"source"}}}, f.base + ":" + f.unit: {{ID: source, Kind: Unit, Unit: "source", Units: []string{"source"}}, {ID: f.unit, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}}
	f.writeJob("completed", false)
	f.ancestors = map[string]bool{source + ":" + f.unit: true}
	req := f.request(true)
	req.CoversFindings = []string{"source-read:1"}
	req.CoversCommits = []string{source}
	f.expectStart(f.root)
	f.expectAfterGate(f.root, true)
	for i, call := range f.expected {
		if call.method == "StagedPaths" {
			f.expected = append(f.expected, policyCall{})
			copy(f.expected[i+1:], f.expected[i:])
			f.expected[i] = policyCall{method: "IsAncestor", args: []string{f.root, source, f.unit}}
			break
		}
	}
	if _, _, err := commitRead(req, f, f.effects()); err != nil {
		t.Fatal(err)
	}
	obligation := goal.ReviewObligation{OriginalEvidence: readsubject.Finding{ID: "source-read:1", Class: "regression", Severity: "high", Material: true, Where: "source.go", Claim: "Source requirement is incomplete", Evidence: "Required source behavior fails", Change: "Complete the source requirement"}, Finding: "source-read:1", Chain: "source-critic", Artifact: "source.go", Test: "source requirement covered", State: "discharged", SourceUnit: "source", TargetUnit: "u1", OriginalRead: "source-read", OriginalFinding: "source-read:1", StopReference: "stop-1", SourceCommit: source, CoverageRead: f.job, CoverageCommit: f.unit, TransferredOnce: true}
	page := goal.GoalFile{Id: "goal-a", State: goal.StateQueued, Intent: "Build the declared requirement", Origin: goal.OriginMain, OpenedAt: "2026-08-20T00:31:00Z", Revision: 1, History: []goal.HistoryLine{{At: "2026-08-20T00:31:00Z", Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Targets: []string{"goal-a"}, Keep: -1}}, ReviewObligations: []goal.ReviewObligation{obligation}}
	encoded := goal.RenderFile(&page)
	if _, problems := goal.ParseFile(encoded); len(problems) > 0 {
		t.Fatalf("fixture goal: %v", problems)
	}
	f.snapshots[f.base] = map[string][]byte{"metasystem/plans/goals/goal-a.md": encoded}
	before, after := policyID("5"), policyID("6")
	f.transitions[before+":"+after] = f.raw[source]
	f.expect("CommitExists", f.root, source)
	f.expect("Kind", f.root, source, "goal-a")
	f.expect("SnapshotFile", f.root, f.tip, attestationPath("goal-a", source))
	f.expect("SnapshotFile", f.root, f.base, "metasystem/plans/goals/goal-a.md")
	f.expect("Kind", f.root, source, "goal-a")
	f.expectValidation(f.root, f.tip, true)
	// Coverage is checked before the destination's saved critic files.
	i := len(f.expected) - 1
	f.expected = append(f.expected, policyCall{})
	copy(f.expected[i+1:], f.expected[i:])
	f.expected[i] = policyCall{method: "IsAncestor", args: []string{f.root, source, f.unit}}
	f.expect("ReadSubject", f.root, source)
	f.expect("RawEntries", f.root, source)
	f.expect("Range", f.root, f.base, source, "goal-a")
	f.expect("SnapshotFile", f.root, f.tip, closureBundlePath("goal-a", f.unit))
	f.expect("Prefix", f.root)
	f.expect("Transition", f.root, before, after)
	f.expect("Transition", f.root, before, after)
	bound, err := BindLandedUnitWithReads(transferBindingFacts{f, source}, f.root, f.tip, f.base, "goal-a", source, before, after)
	if err != nil || bound.Unit != "source" || bound.Digest != digestRawEntries(f.raw[source]) || bound.CriticRoot != f.job || bound.GoalRevision != 1 {
		t.Fatalf("transferred source binding: %+v %v", bound, err)
	}
	if _, exists := f.snapshots[f.tip][attestationPath("goal-a", source)]; exists {
		t.Fatal("source attestation written as clean")
	}
	delete(f.snapshots[f.base], "metasystem/plans/goals/goal-a.md")
	f.expect("SnapshotFile", f.root, f.base, "metasystem/plans/goals/goal-a.md")
	original := &LandedUnitError{Code: "unreadable", Err: os.ErrNotExist}
	if _, err := bindTransferredLandedUnit(transferBindingFacts{f, source}, f.root, f.tip, f.base, "goal-a", source, before, after, original); !errors.Is(err, original) {
		t.Fatalf("missing publication bypassed source read: %v", err)
	}
}

func TestCanonicalLiveReadKeepsSubjectThroughExactDiffPublication(t *testing.T) {
	t.Parallel()
	read := readsubject.Read{ID: "launch-read", Engine: "engine", Model: "reader", Subject: readsubject.ReadSubject{Kind: readsubject.SubjectLive, ImplementerRoot: "builder", ReviewedProjectTree: "frozen-snapshot", DiffDigest: "same-patch"}}
	data, digest := read.Canonical()
	if err := validateCanonicalRead(data, digest, statusTip, "same-patch"); err != nil {
		t.Fatalf("exact live diff cannot publish: %v", err)
	}
	if read.Subject.Kind != readsubject.SubjectLive || read.Subject.ReviewedProjectTree != "frozen-snapshot" {
		t.Fatal("immutable live subject rewritten")
	}
	if err := validateCanonicalRead(data, digest, statusTip, "different-patch"); err == nil {
		t.Fatal("different committed diff admitted")
	}
}
