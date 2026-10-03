package dispatch

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// TestAcceptedRiskClosesToALandableRead: a completed bound round whose only
// finding a person accepted as a risk (goal accept-risk) closes with the
// clean closure the landing takes, and the closure names the accepted risk.
// A chain an earlier engine closed without a closure records it when closed
// again. A refuted finding still closes without a closure.
func TestAcceptedRiskClosesToALandableRead(t *testing.T) {
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
	landed := func(t *testing.T, repo string, subject ReadSubject) {
		t.Helper()
		root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
		closure, present, err := ReadClosure(root)
		want := []readsubject.AcceptedRisk{{FindingID: "F-1", DecisionOpID: "decision-op", AcceptedDigest: acceptedSnapshot(t, repo, "F-1").Digest}}
		if err != nil || !present || closure.Round != 1 || !closure.Subject.Equal(subject) || !readsubject.SameAcceptedRisks(closure.AcceptedRisks, want) {
			t.Fatalf("accepted-risk closure = %+v present=%v err=%v", closure, present, err)
		}
		members, err := chainMembers(filepath.Join(repo, "artifacts", "agents", "jobs"), "critic")
		if err != nil {
			t.Fatal(err)
		}
		records := []map[string]any{}
		for _, member := range members {
			records = append(records, member.record)
		}
		if _, present, err := readsubject.ReadClosedClosure(filepath.Join(repo, "artifacts", "agents"), root, records); err != nil || !present {
			t.Fatalf("the landing's closure gate refuses the accepted-risk read: present=%v err=%v", present, err)
		}
		assertChainClosed(t, repo, "critic")
	}

	t.Run("closes", func(t *testing.T) {
		repo, subject := setup(t, "severe")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", acceptedSnapshot(t, repo, "F-1").Digest); err != nil {
			t.Fatal(err)
		}
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("register close = %q, %v", outcome, err)
		}
		closeReadyCriticChain(t, repo, "critic", "critic")
		for attempt := 1; attempt <= 2; attempt++ {
			if err := CritiqueChainClose(repo, "critic", false); err != nil {
				t.Fatalf("chain close attempt %d = %v", attempt, err)
			}
			landed(t, repo, subject)
		}
	})

	t.Run("closed-earlier-without-closure", func(t *testing.T) {
		repo, subject := setup(t, "unproven")
		if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op", acceptedSnapshot(t, repo, "F-1").Digest); err != nil {
			t.Fatal(err)
		}
		closeReadyCriticChain(t, repo, "critic", "critic")
		rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, rootPath)
		root["chainClosed"] = true
		if err := writeRecord(rootPath, root); err != nil {
			t.Fatal(err)
		}
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatalf("closing the closed chain again = %v", err)
		}
		landed(t, repo, subject)
	})

	t.Run("refuted-stays-without-closure", func(t *testing.T) {
		repo, _ := setup(t, "bounded")
		if err := CritiqueRegisterApplyDecisions(repo, "critic", map[string]string{"F-1": "refuted"}); err != nil {
			t.Fatal(err)
		}
		closeReadyCriticChain(t, repo, "critic", "critic")
		for attempt := 1; attempt <= 2; attempt++ {
			if err := CritiqueChainClose(repo, "critic", false); err != nil {
				t.Fatalf("chain close attempt %d = %v", attempt, err)
			}
			assertNoClosure(t, repo, "critic")
			assertChainClosed(t, repo, "critic")
		}
	})
}
