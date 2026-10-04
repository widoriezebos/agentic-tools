package dispatch

import (
	"os"
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

// TestAcceptedUnboundReturnClosesTheChain: a round whose return names another
// tree than its persisted subject folds to the unbound-return finding. A
// person's acceptance of that finding closes the chain on the persisted
// subject with the acceptance among the closure's accepted risks, whether it
// is stamped after the chain closed (also when repeated) or before it closes.
// The finding left open gives no closure. A stamp whose closure cannot be
// computed still records the acceptance.
func TestAcceptedUnboundReturnClosesTheChain(t *testing.T) {
	t.Parallel()
	placeholder := syntheticUnboundFindingID("code-critic", "critic")
	rootPath := func(repo string) string {
		return filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	}
	// unbound is a completed round reviewed against tree-a whose persisted
	// subject is tree-b, folded and ready to close.
	unbound := func(t *testing.T) (string, ReadSubject) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
		setCriticSubjectFiles(t, repo, "critic", "implementer", "tree-a")
		subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-b", DiffDigest: "diff"}
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), subject); err != nil {
			t.Fatal(err)
		}
		if _, err := advanceWithPrefix(t, repo, "critic", "critic"); err != nil {
			t.Fatal(err)
		}
		if entry := registerEntryByID(t, readRegister(t, repo, "critic"), placeholder); entry["status"] != "open" {
			t.Fatalf("folded unbound-return finding = %v", entry)
		}
		bindCriticGoal(t, repo, "critic", "goal-a")
		closeReadyCriticChain(t, repo, "critic", "critic")
		return repo, subject
	}
	// closedOpen is that chain closed while its unbound-return finding is
	// open, as the steward's reaper closes it, so no closure was written.
	closedOpen := func(t *testing.T) (string, ReadSubject) {
		repo, subject := unbound(t)
		root := readJSONFile(t, rootPath(repo))
		root["chainClosed"] = true
		if err := writeRecord(rootPath(repo), root); err != nil {
			t.Fatal(err)
		}
		return repo, subject
	}
	gate := func(t *testing.T, repo string) (readsubject.Closure, bool, error) {
		t.Helper()
		members, err := chainMembers(filepath.Join(repo, "artifacts", "agents", "jobs"), "critic")
		if err != nil {
			t.Fatal(err)
		}
		records := []map[string]any{}
		for _, member := range members {
			records = append(records, member.record)
		}
		return readsubject.ReadClosedClosure(filepath.Join(repo, "artifacts", "agents"), readJSONFile(t, rootPath(repo)), records)
	}
	landed := func(t *testing.T, repo string, subject ReadSubject, digest string) {
		t.Helper()
		want := []readsubject.AcceptedRisk{{FindingID: placeholder, DecisionOpID: "decision-op", AcceptedDigest: digest}}
		closure, present, err := ReadClosure(readJSONFile(t, rootPath(repo)))
		if err != nil || !present || closure.Round != 1 || !closure.Subject.Equal(subject) || !readsubject.SameAcceptedRisks(closure.AcceptedRisks, want) {
			t.Fatalf("unbound closure = %+v present=%v err=%v", closure, present, err)
		}
		read, present, err := gate(t, repo)
		if err != nil || !present || !readsubject.SameAcceptedRisks(read.AcceptedRisks, want) {
			t.Fatalf("the landing's closure gate reads %+v present=%v err=%v", read, present, err)
		}
		assertChainClosed(t, repo, "critic")
	}

	t.Run("accepted-after-close", func(t *testing.T) {
		t.Parallel()
		repo, subject := closedOpen(t)
		digest := acceptedSnapshot(t, repo, placeholder).Digest
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", placeholder, "decision-op", digest); err != nil || !stamped {
			t.Fatalf("stamping the acceptance = %v, %v", stamped, err)
		}
		landed(t, repo, subject, digest)

		// An earlier engine stamped the acceptance without a closure.
		root := readJSONFile(t, rootPath(repo))
		delete(root, closureField)
		if err := writeRecord(rootPath(repo), root); err != nil {
			t.Fatal(err)
		}
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", placeholder, "decision-op", digest); err != nil || stamped {
			t.Fatalf("repeating the acceptance = %v, %v", stamped, err)
		}
		landed(t, repo, subject, digest)
	})

	t.Run("accepted-before-close", func(t *testing.T) {
		t.Parallel()
		repo, subject := unbound(t)
		digest := acceptedSnapshot(t, repo, placeholder).Digest
		if err := CritiqueRegisterAcceptRisk(repo, "critic", placeholder, "decision-op", digest); err != nil {
			t.Fatal(err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainOpen(t, repo, "critic")
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("register close = %q, %v", outcome, err)
		}
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatalf("chain close = %v", err)
		}
		landed(t, repo, subject, digest)
	})

	t.Run("open-finding-stays-without-closure", func(t *testing.T) {
		t.Parallel()
		repo, _ := closedOpen(t)
		root := readJSONFile(t, rootPath(repo))
		register, _, err := critiqueFindingRegister(root)
		if err != nil {
			t.Fatal(err)
		}
		if closure, earned, err := cleanClosure(loadCritiqueState(repo), "critic", root, register); err != nil || earned {
			t.Fatalf("closure with the finding open = %+v, %v, %v", closure, earned, err)
		}
		if read, present, err := gate(t, repo); err != nil || present {
			t.Fatalf("the landing's closure gate reads %+v present=%v err=%v", read, present, err)
		}
	})

	t.Run("acceptance-recorded-without-closure", func(t *testing.T) {
		t.Parallel()
		repo, _ := closedOpen(t)
		digest := acceptedSnapshot(t, repo, placeholder).Digest
		// The round's return is lost, so no closure can be computed.
		if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "return.json")); err != nil {
			t.Fatal(err)
		}
		if stamped, err := CritiqueRegisterStampAcceptedRisk(repo, "critic", placeholder, "decision-op", digest); err != nil || !stamped {
			t.Fatalf("stamping the acceptance = %v, %v", stamped, err)
		}
		entry := registerEntryByID(t, readRegister(t, repo, "critic"), placeholder)
		if entry["status"] != "accepted-risk" || entry["decisionOpid"] != "decision-op" || entry["acceptedDigest"] != digest {
			t.Fatalf("recorded acceptance = %v", entry)
		}
		assertNoClosure(t, repo, "critic")
	})
}
