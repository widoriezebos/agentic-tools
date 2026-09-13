package landing

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func landingReviewPatch(t *testing.T, fixture *observeFixture, chain string, round int) []byte {
	t.Helper()
	patch, err := os.ReadFile(filepath.Join(fixture.root, "artifacts", "agents", chain, "rounds", strconv.Itoa(round), "diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	return patch
}

func writeLandingCriticClosure(t *testing.T, fixture *observeFixture, critic string, round int, subject readsubject.ReadSubject) {
	t.Helper()
	root := map[string]any{
		"jobId": critic, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": subject.ReviewedMember, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": round,
		"findingRegisterSubjectDigest": subject.Digest(),
		"closure":                      readsubject.Closure{CriticRoot: critic, Round: int64(round), Subject: subject, Mechanism: "clean"},
	}
	fixture.writeChainRecord(critic, root)
	selectedJob := critic
	for memberRound := 2; memberRound <= round; memberRound++ {
		selectedJob = critic + "-r" + strconv.Itoa(memberRound)
		parent := critic
		if memberRound > 2 {
			parent = critic + "-r" + strconv.Itoa(memberRound-1)
		}
		fixture.writeChainRecord(selectedJob, map[string]any{
			"jobId": selectedJob, "role": "code-critic", "round": memberRound,
			"parentJob": parent, "status": "completed",
		})
	}
	fixture.writeChainReviewSubject(critic, round, subject)
	fixture.writeJSONDocument(filepath.Join("artifacts", "agents", critic, "rounds", strconv.Itoa(round), "return.json"), map[string]any{
		"jobId": selectedJob, "round": round, "reviewedTree": subject.ReviewedProjectTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
}

func (f *observeFixture) writeJSONDocument(relative string, value any) {
	f.t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(relative, string(append(data, '\n')))
}

func (f *observeFixture) writeChainReviewSubject(critic string, round int, subject readsubject.ReadSubject) {
	f.t.Helper()
	f.writeJSONDocument(filepath.Join("artifacts", "agents", critic, "rounds", strconv.Itoa(round), "subject.json"), subject)
}

func newLandingClosureFixture(t *testing.T) (*observeFixture, map[string]any, string, []byte) {
	t.Helper()
	fixture := newObserveFixture(t)
	fixture.write("internal/output.go", "package internal\n")
	reviewedTree := fixture.tree()
	rootRecord := map[string]any{
		"jobId": "implementation", "parentJob": nil, "role": "implementer", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		"independentCritiqueJobRef": "critic",
	}
	fixture.writeChainRecord("implementation", rootRecord)
	fixture.writeChainReview("implementation", 1, "implementation", reviewedTree)
	patch := landingReviewPatch(t, fixture, "implementation", 1)
	subject := readsubject.ReadSubject{
		Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation",
		ReviewedProjectTree: reviewedTree, DiffDigest: landingPatchDigest(patch),
	}
	writeLandingCriticClosure(t, fixture, "critic", 1, subject)
	return fixture, rootRecord, reviewedTree, patch
}

func TestChainCertifiedOutputPrefersClosure(t *testing.T) {
	fixture := newObserveFixture(t)
	fixture.write("internal/output.go", "package internal // old\n")
	oldTree := fixture.tree()
	fixture.write("internal/output.go", "package internal // selected\n")
	selectedTree := fixture.tree()
	rootRecord := map[string]any{
		"jobId": "implementation", "parentJob": nil, "role": "implementer", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		"independentCritiqueJobRef": "critic",
	}
	fixture.writeChainRecord("implementation", rootRecord)
	fixture.writeChainRecord("implementation-r2", map[string]any{
		"jobId": "implementation-r2", "parentJob": "implementation", "role": "implementer",
		"round": 2, "status": "completed",
	})
	fixture.writeChainReview("implementation", 1, "implementation", oldTree)
	fixture.writeChainReview("implementation", 2, "implementation-r2", selectedTree)
	selectedPatch := landingReviewPatch(t, fixture, "implementation", 2)
	subject := readsubject.ReadSubject{
		Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation-r2",
		ReviewedProjectTree: selectedTree, DiffDigest: landingPatchDigest(selectedPatch),
	}
	writeLandingCriticClosure(t, fixture, "critic", 2, subject)
	fixture.writeJSONDocument("artifacts/agents/critic/rounds/1/return.json", map[string]any{
		"jobId": "critic", "round": 1, "reviewedTree": oldTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})

	output, err := chainCertifiedOutput(fixture.root, "implementation", rootRecord)
	if err != nil {
		t.Fatal(err)
	}
	if output.round != 2 || output.reviewedTree != selectedTree || !bytes.Equal(output.patch, selectedPatch) {
		t.Fatalf("selected output = round %d tree %s patch %x", output.round, output.reviewedTree, output.patch)
	}

	t.Run("equal subjects use numeric round order", func(t *testing.T) {
		equal := newObserveFixture(t)
		equal.write("internal/output.go", "package internal\n")
		tree := equal.tree()
		record := map[string]any{
			"jobId": "equal-implementation", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
			"independentCritiqueJobRef": "equal-critic",
		}
		equal.writeChainRecord("equal-implementation", record)
		equal.writeChainReview("equal-implementation", 10, "equal-implementation-r10", tree)
		equal.writeChainReview("equal-implementation", 2, "equal-implementation-r2", tree)
		patch := landingReviewPatch(t, equal, "equal-implementation", 2)
		subject := readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "equal-implementation", ReviewedMember: "equal-implementation-r10",
			ReviewedProjectTree: tree, DiffDigest: landingPatchDigest(patch),
		}
		writeLandingCriticClosure(t, equal, "equal-critic", 1, subject)
		output, err := chainCertifiedOutput(equal.root, "equal-implementation", record)
		if err != nil {
			t.Fatal(err)
		}
		if output.round != 2 {
			t.Fatalf("equal closure subjects selected round %d, want numeric first round 2", output.round)
		}
	})
}

func TestChainCertifiedOutputRejectsUnmatchedClosure(t *testing.T) {
	t.Run("sole output has another reviewed tree", func(t *testing.T) {
		fixture, rootRecord, _, patch := newLandingClosureFixture(t)
		subject := readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation",
			ReviewedProjectTree: strings.Repeat("b", 40), DiffDigest: landingPatchDigest(patch),
		}
		writeLandingCriticClosure(t, fixture, "critic", 1, subject)
		if _, err := chainCertifiedOutput(fixture.root, "implementation", rootRecord); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("unmatched closure tree error = %v", err)
		}
	})

	t.Run("equal tree has another patch digest", func(t *testing.T) {
		fixture, rootRecord, reviewedTree, _ := newLandingClosureFixture(t)
		subject := readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation",
			ReviewedProjectTree: reviewedTree, DiffDigest: strings.Repeat("c", 64),
		}
		writeLandingCriticClosure(t, fixture, "critic", 1, subject)
		if _, err := chainCertifiedOutput(fixture.root, "implementation", rootRecord); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("unmatched closure patch error = %v", err)
		}
	})

	t.Run("malformed closure cannot fall back", func(t *testing.T) {
		fixture, rootRecord, _, _ := newLandingClosureFixture(t)
		criticData, err := os.ReadFile(filepath.Join(fixture.root, "artifacts", "agents", "jobs", "critic.json"))
		if err != nil {
			t.Fatal(err)
		}
		var critic map[string]any
		if err := json.Unmarshal(criticData, &critic); err != nil {
			t.Fatal(err)
		}
		critic["closure"] = "malformed"
		fixture.writeChainRecord("critic", critic)
		if _, err := chainCertifiedOutput(fixture.root, "implementation", rootRecord); err == nil || !strings.Contains(err.Error(), "closure must be an object") {
			t.Fatalf("malformed closure error = %v", err)
		}
	})

	for _, test := range []struct {
		name  string
		stamp any
	}{
		{name: "null critic stamp cannot fall back", stamp: nil},
		{name: "empty critic stamp cannot fall back", stamp: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, rootRecord, _, _ := newLandingClosureFixture(t)
			rootRecord["independentCritiqueJobRef"] = test.stamp
			if _, err := chainCertifiedOutput(fixture.root, "implementation", rootRecord); err == nil || !strings.Contains(err.Error(), "critique reference") {
				t.Fatalf("malformed critic stamp error = %v", err)
			}
		})
	}

	t.Run("historical closure absence keeps the stamped return fallback", func(t *testing.T) {
		fixture := newObserveFixture(t)
		fixture.write("internal/output.go", "package internal // historical\n")
		historicalTree := fixture.tree()
		fixture.write("internal/output.go", "package internal // terminal\n")
		terminalTree := fixture.tree()
		record := map[string]any{
			"jobId": "historical-implementation", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
			"independentCritiqueJobRef": "historical-critic",
		}
		fixture.writeChainRecord("historical-implementation", record)
		fixture.writeChainRecord("historical-implementation-r2", map[string]any{
			"jobId": "historical-implementation-r2", "parentJob": "historical-implementation",
			"role": "implementer", "round": 2, "status": "completed",
		})
		fixture.writeChainReview("historical-implementation", 1, "historical-implementation", historicalTree)
		fixture.writeChainReview("historical-implementation", 2, "historical-implementation-r2", terminalTree)
		fixture.writeChainRecord("historical-critic", map[string]any{
			"jobId": "historical-critic", "parentJob": nil, "role": "code-critic", "round": 1,
			"reviews": "historical-implementation", "status": "completed", "chainClosed": true,
		})
		fixture.writeJSONDocument("artifacts/agents/historical-critic/rounds/1/return.json", map[string]any{
			"jobId": "historical-critic", "round": 1, "reviewedTree": historicalTree,
			"findings": []any{}, "verdictMaterialCount": 0,
		})
		output, err := chainCertifiedOutput(fixture.root, "historical-implementation", record)
		if err != nil || output.reviewedTree != historicalTree {
			t.Fatalf("historical output = %+v, %v", output, err)
		}
	})
}

func TestLandingRejectsStaleCriticClosure(t *testing.T) {
	t.Run("later critic member invalidates the public observation", func(t *testing.T) {
		fixture, rootRecord, reviewedTree, _ := newLandingClosureFixture(t)
		fixture.writeChainRecord("critic-r2", map[string]any{
			"jobId": "critic-r2", "parentJob": "critic", "role": "code-critic",
			"round": 2, "status": "completed",
		})
		got := Observe(ObserveParams{RepoRoot: fixture.root, CandidateTree: reviewedTree, Chain: "implementation"})
		if got.Code != "chain-output-unreadable" || got.Bar != BarRefusal || !strings.Contains(got.Detail, "last critic round") {
			t.Fatalf("stale critic observation = %+v; root=%v", got, rootRecord)
		}
	})

	t.Run("base movement preserves patch replay", func(t *testing.T) {
		fixture, _, _, _ := newLandingClosureFixture(t)
		if err := os.Remove(filepath.Join(fixture.root, "internal", "output.go")); err != nil {
			t.Fatal(err)
		}
		fixture.write("docs/base-move.md", "unrelated base movement\n")
		fixture.git("add", "docs/base-move.md")
		fixture.git("commit", "-qm", "move landing base")
		fixture.write("internal/output.go", "package internal\n")
		candidate := fixture.tree()
		got := Observe(ObserveParams{RepoRoot: fixture.root, CandidateTree: candidate, Chain: "implementation"})
		if got.Code != "closed-chain" || got.Bar != BarChain || got.Verdict != "pass" {
			t.Fatalf("base-moved observation = %+v", got)
		}
	})

	t.Run("reviewed postimage drift still refuses", func(t *testing.T) {
		fixture, rootRecord, reviewedTree, patch := newLandingClosureFixture(t)
		fixture.write("internal/output.go", "package internal // drifted postimage\n")
		driftedTree := fixture.tree()
		fixture.write("internal/output.go", "package internal\n")
		fixture.writeJSONDocument("artifacts/agents/implementation/rounds/1/review.json", map[string]any{
			"diffArtifact": "diff.patch", "implementerJob": "implementation", "reviewedTree": driftedTree,
		})
		subject := readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation",
			ReviewedProjectTree: driftedTree, DiffDigest: landingPatchDigest(patch),
		}
		writeLandingCriticClosure(t, fixture, "critic", 1, subject)
		got := Observe(ObserveParams{RepoRoot: fixture.root, CandidateTree: reviewedTree, Chain: "implementation"})
		if got.Code != "chain-output-mismatch" || got.Bar != BarRefusal {
			t.Fatalf("postimage drift observation = %+v; root=%v", got, rootRecord)
		}
	})
}
