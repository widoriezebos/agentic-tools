package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestInheritedV6ResolutionFoldsAndClosesFromPublishedEvidence(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"valid", "unknown", "class", "path", "source-commit"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			repo, root, job, dir := examinationReadFixture(t)
			commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
			subject := readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: commit, Parent: strings.Repeat("c", 40), Tree: tree, DiffDigest: strings.Repeat("d", 64)}
			writeJSONFile(t, dir, "subject.json", subject)
			rootPath := filepath.Join(repo, "artifacts/agents/jobs", root+".json")
			record := readJSONFile(t, rootPath)
			record[findingRegisterRoundField] = 1
			record["reviews"] = "commit:" + commit
			if err := writeRecord(rootPath, record); err != nil {
				t.Fatal(err)
			}
			evidence := readsubject.Finding{ID: "source-read:2", Class: "missing-reader", Severity: "high", Material: true, Claim: "The inherited reader misses retained evidence.", Evidence: "Observed the source path.", Where: "metasystem/test.go", Change: "Read the source evidence."}
			obligation := goal.ReviewObligation{Finding: evidence.ID, Chain: "source-critic", SourceUnit: "source", TargetUnit: "destination", OriginalRead: "source-read", OriginalFinding: evidence.ID, OriginalEvidence: evidence, SourceCommit: strings.Repeat("a", 40), StopReference: "source-stop", TransferredOnce: true}
			if mutation == "source-commit" {
				obligation.SourceCommit = ""
			}
			if err := CritiqueInheritFindings(repo, root, []goal.ReviewObligation{obligation}); err != nil {
				if mutation == "source-commit" {
					return
				}
				t.Fatal(err)
			}
			result := readJSONFile(t, filepath.Join(dir, "return.json"))
			result["reviewedTree"] = tree
			f := result["findings"].([]any)[0].(map[string]any)
			f["material"], f["resolves"] = false, evidence.ID
			switch mutation {
			case "unknown":
				f["resolves"] = "unpublished:1"
			case "class":
				f["class"] = "regression"
			case "path":
				f["where"] = "metasystem/another.go"
			}
			result["schemaVersion"], result["verdictMaterialCount"], result["rigor"] = 6, 0, []any{}
			writeJSONFile(t, dir, "return.json", result)
			if err := os.WriteFile(filepath.Join(dir, "return.md"), []byte("VERDICT: LAND\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if outcome, err := advanceWithFacts(t, repo, root, job, declaredPaths(repo, commit, evidence.Where), declaredTree(repo, commit, tree)); err != nil {
				if mutation != "valid" {
					return
				}
				t.Fatalf("valid inherited resolution: %s,%v", outcome, err)
			} else if mutation != "valid" {
				t.Fatalf("%s inherited evidence admitted", mutation)
			}
			record = readJSONFile(t, rootPath)
			register, err := decodeFindingRegister(record[findingRegisterField])
			if err != nil {
				t.Fatal(err)
			}
			if len(register) != 1 || register[0].FindingID != evidence.ID || register[0].Status != "resolved" || register[0].Class != evidence.Class || register[0].Where != evidence.Where {
				t.Fatalf("original identity or evidence discarded: %+v", register)
			}
			state := loadCritiqueState(repo)
			closure, earned, err := cleanClosure(state, root, record, register)
			if err != nil || !earned || closure.Mechanism != "clean" || closure.Round != 2 {
				t.Fatalf("inherited clean read could not close: %+v earned=%v err=%v", closure, earned, err)
			}
			if _, err := inheritedObligations("corrupt"); err == nil {
				t.Fatal("corrupt inherited provenance ignored")
			}
		})
	}
}

func TestFreshUnknownExaminationKeepsGoalAllowanceAndCriticCount(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeBudgetJob(t, repo, "original", "original-op", 3, 30, "completed", budgetJobLife{})
	originalPath := filepath.Join(repo, "artifacts/agents/jobs/original.json")
	original := readJSONFile(t, originalPath)
	original["role"], original["parentJob"], original["round"] = "code-critic", nil, 1
	original[reviewRoundLimitField], original[criticRoundsConsumedField], original[findingRegisterRoundField] = 3, 0, 0
	if err := writeRecord(originalPath, original); err != nil {
		t.Fatal(err)
	}
	if err := ReserveUnknownExaminationRetry(repo, "original"); err != nil {
		t.Fatal(err)
	}
	writeBudgetJob(t, repo, "fresh", "fresh-op", 3, 30, "running", budgetJobLife{})
	freshPath := filepath.Join(repo, "artifacts/agents/jobs/fresh.json")
	fresh := readJSONFile(t, freshPath)
	fresh["role"], fresh["parentJob"], fresh["round"], fresh["examinationRetryOf"] = "code-critic", "original", 2, "original"
	if err := writeRecord(freshPath, fresh); err != nil {
		t.Fatal(err)
	}
	projection := ProjectBudget(repo, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 0 || projection.ActiveJobs != 1 {
		data, _ := json.Marshal(projection)
		t.Fatalf("retry spent goal allowance or lost live slot: %s", data)
	}
	state := loadCritiqueState(repo)
	root := state.records["original"]
	delete(root, criticRoundsConsumedField)
	accounting, err := critiqueRoundAccounting(repo, state, "original", root)
	if err != nil || accounting.consumed != 0 {
		t.Fatalf("unknown transport execution counted as completed critique: %+v,%v", accounting, err)
	}
	if round, _ := numInt(root[findingRegisterRoundField]); round != 1 {
		t.Fatal("unavailable input left retry behind the fold order gate")
	}
	if root["unknownExaminationRetryFrom"] != "original" {
		t.Fatal(fmt.Sprint(root))
	}
}
