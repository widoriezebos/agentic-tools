package dispatch

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func bindCriticGoal(t *testing.T, repo, root, goalID string) {
	t.Helper()
	path := filepath.Join(repo, "artifacts", "agents", "jobs", root+".json")
	record := readJSONFile(t, path)
	record["goalId"] = goalID
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
}

// acceptedSnapshot is what goal accept-risk shows the person: the register
// finding of chain "critic" with the digest an acceptance of it covers.
func acceptedSnapshot(t *testing.T, repo, findingID string) CritiqueDecisionFinding {
	t.Helper()
	finding, err := CritiqueRegisterDecisionFinding(repo, "critic", findingID, "goal-a")
	if err != nil {
		t.Fatal(err)
	}
	if finding.Digest == "" {
		t.Fatalf("decision finding %s carries no digest of the content it shows", findingID)
	}
	return finding
}

// TestAcceptedRiskCoversOnlyTheFindingSeen (F4): a person's accepted risk
// covers the finding content they saw (content only, Wido 2026-10-03). A
// later fold that re-reports the finding with changed facts at equal rigor
// reopens it, so the register is not landable and the chain does not close;
// a re-review of another tree reporting the same content keeps it, and the
// chain lands; stamping an acceptance from a snapshot whose content changed
// is refused.
func TestAcceptedRiskCoversOnlyTheFindingSeen(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T, class string) (string, ReadSubject) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1,
			[]any{registerFindingValue("F-1", true, "evidence")}, []any{registerRigor("F-1", class)})
		setCriticSubjectFiles(t, repo, "critic", "implementer", "tree-a")
		subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), subject); err != nil {
			t.Fatal(err)
		}
		if _, err := advanceWithPrefix(t, repo, "critic", "critic"); err != nil {
			t.Fatal(err)
		}
		bindCriticGoal(t, repo, "critic", "goal-a")
		return repo, subject
	}
	// refold folds round 2 on subject, re-reporting F-1 at class; changeFacts
	// crosses a proof boundary in its facts.
	refold := func(t *testing.T, repo string, subject ReadSubject, class string, changeFacts bool) {
		t.Helper()
		changed := registerRigor("F-1", class)
		facts := registerFacts()
		if changeFacts {
			facts["proofBoundaryCrossed"] = true
		}
		changed["facts"] = facts
		writeCriticRound(t, repo, "critic", "critic-2", 2, []any{registerFindingValue("F-1", true, "evidence")}, []any{changed})
		returnPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "return.json")
		result := readJSONFile(t, returnPath)
		result["reviewedTree"] = subject.ReviewedProjectTree
		if err := writeRecord(returnPath, result); err != nil {
			t.Fatal(err)
		}
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "subject.json"), subject); err != nil {
			t.Fatal(err)
		}
		if outcome, err := advanceWithPrefix(t, repo, "critic", "critic-2"); err != nil || outcome != "advanced" {
			t.Fatalf("round 2 fold = %q, %v", outcome, err)
		}
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
		if entry["rigorClass"] != class || entry["factsDigest"] != digestJSON(facts) {
			t.Fatalf("round 2 did not re-report F-1 at %s: %v", class, entry)
		}
	}
	// lands closes the chain and checks its closure names the acceptance.
	lands := func(t *testing.T, repo string, subject ReadSubject, digest string, members ...string) {
		t.Helper()
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("register close = %q, %v", outcome, err)
		}
		closeReadyCriticChain(t, repo, "critic", members...)
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatal(err)
		}
		root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
		closure, present, err := ReadClosure(root)
		want := []readsubject.AcceptedRisk{{FindingID: "F-1", DecisionOpID: "decision-op", AcceptedDigest: digest}}
		if err != nil || !present || !closure.Subject.Equal(subject) || !readsubject.SameAcceptedRisks(closure.AcceptedRisks, want) {
			t.Fatalf("accepted-risk closure = %+v present=%v err=%v", closure, present, err)
		}
	}

	t.Run("accepted-then-lands", func(t *testing.T) {
		repo, subject := setup(t, "severe")
		seen := acceptedSnapshot(t, repo, "F-1")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", seen.Digest); err != nil {
			t.Fatal(err)
		}
		lands(t, repo, subject, seen.Digest, "critic")
	})

	t.Run("rebased-tree-keeps-acceptance", func(t *testing.T) {
		// Round 2 reviews another tree and reports the same finding: the
		// person is asked again only when the problem changes.
		repo, subject := setup(t, "severe")
		seen := acceptedSnapshot(t, repo, "F-1")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", seen.Digest); err != nil {
			t.Fatal(err)
		}
		subject.ReviewedProjectTree = "tree-b"
		refold(t, repo, subject, "severe", false)
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
		if entry["status"] != "accepted-risk" || entry["decisionOpid"] != "decision-op" || entry["acceptedDigest"] != seen.Digest {
			t.Fatalf("a re-review of the same finding on another tree dropped the acceptance: %v", entry)
		}
		lands(t, repo, subject, seen.Digest, "critic", "critic-2")
	})

	// predate records F-1's acceptance as an engine before content binding
	// did: accepted-risk with its decision operation and no digest.
	predate := func(t *testing.T, repo string) string {
		t.Helper()
		seen := acceptedSnapshot(t, repo, "F-1")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", seen.Digest); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, path)
		delete(registerEntryByID(t, root[findingRegisterField].([]any), "F-1"), "acceptedDigest")
		if err := writeRecord(path, root); err != nil {
			t.Fatal(err)
		}
		return seen.Digest
	}
	// boundLine is the grandfathering record the fold left for F-1.
	boundLine := func(t *testing.T, repo string) map[string]any {
		t.Helper()
		root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
		lines, _ := root[acceptancesBoundField].([]any)
		if len(lines) != 1 {
			t.Fatalf("the fold recorded %d grandfathered acceptances: %v", len(lines), root[acceptancesBoundField])
		}
		return lines[0].(map[string]any)
	}

	t.Run("predating-acceptance-is-grandfathered", func(t *testing.T) {
		// R-142-m1e: a person's recorded decision is never overruled. An
		// acceptance without a digest covers the finding as it stands; the
		// first fold that meets it stamps that digest and says so.
		repo, subject := setup(t, "severe")
		digest := predate(t, repo)
		root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
		landable, risks, err := readsubject.LandableRegister(root[findingRegisterField])
		if err != nil || !landable || len(risks) != 1 || risks[0].AcceptedDigest != digest {
			t.Fatalf("a predating acceptance is not landable as the finding stands: %v %v %v", landable, risks, err)
		}
		subject.ReviewedProjectTree = "tree-b"
		refold(t, repo, subject, "severe", false)
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
		if entry["status"] != "accepted-risk" || entry["decisionOpid"] != "decision-op" || entry["acceptedDigest"] != digest {
			t.Fatalf("the fold did not keep and bind the predating acceptance: %v", entry)
		}
		if line := boundLine(t, repo); line["findingId"] != "F-1" || line["acceptedDigest"] != digest || !strings.Contains(asString(line["line"]), "predates") {
			t.Fatalf("grandfathering record = %v", line)
		}
		lands(t, repo, subject, digest, "critic", "critic-2")
	})

	t.Run("grandfathered-then-changed-reopens", func(t *testing.T) {
		repo, subject := setup(t, "severe")
		digest := predate(t, repo)
		refold(t, repo, subject, "severe", true)
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
		if entry["status"] != "open" || entry["decisionOpid"] != "" {
			t.Fatalf("changed content kept a grandfathered acceptance: %v", entry)
		}
		if line := boundLine(t, repo); line["acceptedDigest"] != digest {
			t.Fatalf("the acceptance was bound to content other than what it predated: %v", line)
		}
	})

	t.Run("changed-facts-reopen", func(t *testing.T) {
		repo, subject := setup(t, "severe")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", acceptedSnapshot(t, repo, "F-1").Digest); err != nil {
			t.Fatal(err)
		}
		refold(t, repo, subject, "severe", true)
		root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
		entry := registerEntryByID(t, root[findingRegisterField].([]any), "F-1")
		if entry["status"] != "open" || entry["resolution"] != "" || entry["decisionOpid"] != "" {
			t.Fatalf("an acceptance kept covering changed facts: %v", entry)
		}
		if landable, risks, err := readsubject.LandableRegister(root[findingRegisterField]); err != nil || landable || risks != nil {
			t.Fatalf("changed facts under an old acceptance are landable: %v %v %v", landable, risks, err)
		}
		// The reopened severe finding needs a new decision before the
		// register closes at all.
		if _, err := CritiqueRegisterClose(repo, "critic"); err == nil || !strings.Contains(err.Error(), "blocks close") {
			t.Fatalf("register close over a reopened finding = %v", err)
		}
		assertNoClosure(t, repo, "critic")
	})

	t.Run("stale-snapshot-refused", func(t *testing.T) {
		// The person was shown F-1 as unproven; round 2 re-reports it as
		// severe, which replaces its content before the stamp.
		repo, subject := setup(t, "unproven")
		stale := acceptedSnapshot(t, repo, "F-1")
		refold(t, repo, subject, "severe", false)
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", stale.Digest); err == nil || !strings.Contains(err.Error(), "changed since") {
			t.Fatalf("stamping a stale snapshot = %v", err)
		}
		// The repeated goal accept-risk stamps through the same owner.
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", "F-1", "decision-op", stale.Digest); stamped || !errors.Is(err, ErrAcceptedFindingChanged) {
			t.Fatalf("a repeated stamp from a stale snapshot = %v, %v", stamped, err)
		}
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
		if entry["status"] != "open" || entry["decisionOpid"] != "" {
			t.Fatalf("a stale snapshot stamped the register: %v", entry)
		}
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", ""); err == nil {
			t.Fatal("stamping without a snapshot digest was accepted")
		}
		fresh := acceptedSnapshot(t, repo, "F-1").Digest
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", "F-1", "decision-op", fresh); err != nil || !stamped {
			t.Fatalf("a fresh decision on the changed finding = %v, %v", stamped, err)
		}
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", "F-1", "decision-op", fresh); err != nil || stamped {
			t.Fatalf("repeating the fresh decision = %v, %v", stamped, err)
		}
	})
}
