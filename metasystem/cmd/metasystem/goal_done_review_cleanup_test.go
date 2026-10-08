package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type conclusionObserver struct {
	goal.Repository
	before func()
	after  func()
}

func (r conclusionObserver) Publish(parent, commit string) (goal.CASOutcome, error) {
	r.before()
	return r.Repository.Publish(parent, commit)
}

func (r conclusionObserver) AcceptedCAS(old, next string) error {
	if err := r.Repository.AcceptedCAS(old, next); err != nil {
		return err
	}
	if r.after != nil {
		r.after()
	}
	return nil
}

func TestGoalDoneClosesReviewsWithReasonAfterPublication(t *testing.T) {
	t.Parallel()
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "interrupted"}[interrupted], func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{})
			jobs := filepath.Join(bed.root, "artifacts", "agents", "jobs")
			read := func(path string) map[string]any {
				t.Helper()
				data, err := os.ReadFile(path)
				var record map[string]any
				if err != nil || json.Unmarshal(data, &record) != nil {
					t.Fatalf("record %s: %s %v", path, data, err)
				}
				return record
			}
			for _, row := range []struct {
				id, role, goal, parent string
				closed                 bool
			}{
				{"code", "code-critic", "fix-docs", "", false},
				{"design", "design-critic", "fix-docs", "", false},
				{"warden", "warden", "fix-docs", "", false},
				{"child", "code-critic", "fix-docs", "code", false},
				{"other", "code-critic", "ship-widget", "", false},
				{"clean", "code-critic", "fix-docs", "", true},
				{"builder", "implementer", "fix-docs", "", false},
			} {
				record := map[string]any{"jobId": row.id, "role": row.role, "goalId": row.goal, "status": "completed", "round": 1, "chainClosed": row.closed, "findingRegister": []any{map[string]any{"findingId": "read:1", "status": "open"}}}
				if row.parent != "" {
					record["parentJob"] = row.parent
				}
				if row.closed {
					record["closure"] = map[string]any{"retained": "original clean read"}
				}
				transferWriteJSON(t, filepath.Join(jobs, row.id+".json"), record)
			}
			goalTree := t.TempDir()
			bed.worktrees = []string{bed.root, goalTree}
			unitRoot := bed.unitRoot
			roundDir := filepath.Join(unitRoot, "run-one", "round-1")
			stop := &loopstop.Stop{Loop: "unit-round", Subject: "fix-docs/docs/run-one", Attempt: 1, Decision: "stop", Handoff: "ask"}
			unit := launch.UnitRunRecord{ID: "run-one", Goal: "fix-docs", Worktree: goalTree, Unit: "docs", State: "awaiting-judgement", Rounds: []launch.UnitRound{{Number: 1, Directory: roundDir, Material: 2, Stop: stop}}}
			unitPath := filepath.Join(unitRoot, unit.ID, "run.json")
			transferWriteJSON(t, unitPath, unit)
			foreignPath := filepath.Join(unitRoot, "foreign-run", "run.json")
			transferWriteJSON(t, foreignPath, launch.UnitRunRecord{ID: "foreign-run", Goal: "fix-docs", Worktree: t.TempDir(), State: "awaiting-judgement"})
			stopPath := filepath.Join(roundDir, "stop-register.json")
			transferWriteJSON(t, stopPath, map[string]any{"kind": "stop", "status": "open", "stop": stop})
			question, err := channel.Ask(channel.AskRequest{RepoRoot: bed.root, Goal: "fix-docs", Kind: "other", Facts: []string{"Two findings remain."}, Recommendation: "Conclude explicitly.", UnitStop: &channel.UnitStopQuestion{Loop: stop.Loop, Subject: stop.Subject, Attempt: 1, Finding: "read:1", Needs: "metasystem goal done fix-docs --reason TEXT --by Wido", AcceptableActs: []string{"goal-done"}}, Now: bed.clock()})
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			owners := bed.owners(&stdout, &stderr)
			observed := 0
			observer := conclusionObserver{Repository: bed.repo, before: func() {
				observed++
				if !strings.Contains(stderr.String(), "Impact:") || !strings.Contains(stderr.String(), "review chains") {
					t.Fatal("impact was not printed before publication")
				}
				paths, _ := filepath.Glob(filepath.Join(bed.root, "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
				if len(paths) != 1 {
					t.Fatalf("prior impact records: %v", paths)
				}
				impact := read(paths[0])
				if impact["reason"] != "Finished explicitly." || impact["who"] != "Wido" || impact["kind"] != "goal-done" || impact["at"] == nil || !strings.Contains(impact["impact"].(string), "fresh review") {
					t.Fatalf("impact missing its authority or consequences: %v", impact)
				}
				if read(filepath.Join(jobs, "code.json"))["chainClosed"] == true || read(unitPath)["reviewCloseReason"] != nil || read(stopPath)["status"] != "open" {
					t.Fatal("reviews closed before conclusion publication")
				}
			}}
			if interrupted {
				observer.after = func() {
					if err := os.Rename(jobs, jobs+"-saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(jobs, []byte("temporarily unavailable"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(unitRoot, unitRoot+"-saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(unitRoot, []byte("temporarily unavailable"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				endpoint, err := bed.endpoint(root)
				endpoint.Repository = observer
				return endpoint, err
			}
			command, args, _ := resolveIntentArgv([]string{"goal", "done", "fix-docs", "--reason", "Finished explicitly.", "--by", "Wido", "--json"})
			code := runIntentIn(command, args, &stdout, &stderr, bed.root, owners)
			if observed != 1 || !strings.Contains(bed.goalRecord("fix-docs"), "State: done") {
				t.Fatalf("conclusion did not publish once: exit=%d output=%s stderr=%s", code, &stdout, &stderr)
			}
			if interrupted {
				if code != 1 || !strings.Contains(stdout.String(), "cleanup is pending") || !strings.Contains(stdout.String(), "goal done") || read(filepath.Join(jobs+"-saved", "code.json"))["chainClosed"] == true {
					t.Fatalf("interruption not recoverable: exit=%d output=%s", code, &stdout)
				}
				shown := gcliLedgerMust(t, bed, "goal", "show", "fix-docs", "--json")
				if !strings.Contains(shown, "interrupted review cleanup") || !strings.Contains(shown, "Finished explicitly.") {
					t.Fatalf("concluded goal hides cleanup remedy: %s", shown)
				}
				if err := os.Remove(jobs); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(jobs+"-saved", jobs); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(unitRoot); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(unitRoot+"-saved", unitRoot); err != nil {
					t.Fatal(err)
				}
			} else if code != 0 {
				t.Fatalf("done: exit=%d output=%s stderr=%s", code, &stdout, &stderr)
			}
			shown := gcliLedgerMust(t, bed, "goal", "show", "fix-docs", "--json")
			if strings.Contains(shown, "interrupted review cleanup") != interrupted {
				t.Fatalf("cleanup remedy does not match pending records: %s", shown)
			}
			if shown := gcliLedgerMust(t, bed, "goal", "show", "port-engine", "--json"); strings.Contains(shown, "interrupted review cleanup") {
				t.Fatalf("completed goal offers unnecessary cleanup: %s", shown)
			}
			assertSingleImpact := func(errOut string) {
				t.Helper()
				paths, err := filepath.Glob(filepath.Join(bed.root, "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
				if err != nil || len(paths) != 1 || strings.Contains(errOut, "Impact:") {
					t.Fatalf("repeat recorded or printed another impact: records=%v error=%v stderr=%s", paths, err, errOut)
				}
			}
			bed.worktrees = []string{bed.root}
			if err := os.Remove(goalTree); err != nil {
				t.Fatal(err)
			}
			tip := bed.tip()
			code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Different repeat reason.", "--by", "Wido", "--json")
			if code != 0 || bed.tip() != tip {
				t.Fatalf("repeat republished or failed: exit=%d output=%s stderr=%s", code, out, errOut)
			}
			assertSingleImpact(errOut)
			if shown := gcliLedgerMust(t, bed, "goal", "show", "fix-docs", "--json"); strings.Contains(shown, "interrupted review cleanup") {
				t.Fatalf("completed cleanup still offers a repeat: %s", shown)
			}
			for _, id := range []string{"code", "design", "warden"} {
				record := read(filepath.Join(jobs, id+".json"))
				if record["chainClosed"] != true || record["chainCloseReason"] != "Finished explicitly." || record["closure"] != nil || record["findingRegister"].([]any)[0].(map[string]any)["status"] != "open" {
					t.Fatalf("conclusion fabricated clean evidence or lost reason: %v", record)
				}
			}
			for _, id := range []string{"other", "child", "builder"} {
				if read(filepath.Join(jobs, id+".json"))["chainClosed"] == true {
					t.Fatalf("closed unrelated record %s", id)
				}
			}
			if read(filepath.Join(jobs, "clean.json"))["closure"].(map[string]any)["retained"] != "original clean read" {
				t.Fatal("rewrote prior clean closure")
			}
			after, err := (&launch.UnitRunner{Root: unitRoot}).Status(unit.ID)
			if err != nil || after.ReviewCloseReason != "Finished explicitly." || !reflect.DeepEqual(after.Rounds, unit.Rounds) || after.State != unit.State || read(stopPath)["status"] != "closed" || read(stopPath)["reason"] != "Finished explicitly." {
				t.Fatalf("unit cleanup lost evidence: %+v %v", after, err)
			}
			if read(foreignPath)["reviewCloseReason"] != nil {
				t.Fatal("closed the same goal name in another repository")
			}
			questions, unreadable := channel.WalkOpenQuestions(bed.root)
			if len(questions) != 0 || len(unreadable) != 0 {
				t.Fatalf("question %s remains open: %v %v", question.ID, questions, unreadable)
			}
			before, _ := os.ReadFile(filepath.Join(jobs, "code.json"))
			code, out, errOut = bed.public("goal", "done", "fix-docs", "--reason", "Repeat.", "--by", "Wido")
			if code != 0 {
				t.Fatalf("completed cleanup repeat failed: exit=%d output=%s stderr=%s", code, out, errOut)
			}
			assertSingleImpact(errOut)
			afterBytes, _ := os.ReadFile(filepath.Join(jobs, "code.json"))
			if !bytes.Equal(before, afterBytes) || bed.tip() != tip {
				t.Fatal("cleanup repeat changed closed evidence")
			}
		})
	}
}

func TestGoalDoneKeepsAgentCompletionObligations(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{amend: func(files map[string]*goal.GoalFile) {
		files["fix-docs"].ReviewObligations = []goal.ReviewObligation{{OriginalEvidence: readsubject.Finding{ID: "read:1", Class: "regression", Severity: "high", Material: true, Where: "docs.md", Claim: "Requirement incomplete", Evidence: "Required behavior fails", Change: "Complete inherited requirement"}, Finding: "read:1", Chain: "source", Artifact: "docs.md", Test: "TestInherited", State: "open", SourceUnit: "source", TargetUnit: "destination", OriginalRead: "read", OriginalFinding: "read:1", StopReference: "stop-one", TransferredOnce: true}}
	}})
	path := filepath.Join(bed.root, "artifacts", "agents", "jobs", "source.json")
	transferWriteJSON(t, path, map[string]any{"jobId": "source", "role": "code-critic", "goalId": "fix-docs", "chainClosed": false})
	before, _ := os.ReadFile(path)
	tip := bed.tip()
	code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Finished.", "--lineage", bed.lineage, "--json")
	after, _ := os.ReadFile(path)
	if code == 0 || bed.tip() != tip || !bytes.Equal(before, after) || !strings.Contains(out, "open review obligation") {
		t.Fatalf("agent bypassed transferred work: exit=%d output=%s stderr=%s", code, out, errOut)
	}
}

func TestGoalDoneRejectsUnprovenPersonBeforeImpact(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.prove = humanauthority.ProveOrTemporaryGoalAuthority
	path := filepath.Join(bed.root, "artifacts", "agents", "jobs", "source.json")
	transferWriteJSON(t, path, map[string]any{"jobId": "source", "role": "code-critic", "goalId": "fix-docs", "chainClosed": false})
	tip := bed.tip()
	code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Finished.", "--by", "Wido", "--json")
	paths, _ := filepath.Glob(filepath.Join(bed.root, "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if code == 0 || bed.tip() != tip || len(paths) != 0 || strings.Contains(errOut, "Impact:") {
		t.Fatalf("unproven name admitted impact: exit=%d output=%s stderr=%s records=%v", code, out, errOut, paths)
	}
}

func TestGoalDoneCleanupSurvivesUnreadableUnitRecords(t *testing.T) {
	t.Parallel()
	for _, broken := range []string{"store", "run", "stop", "job-identity"} {
		t.Run(broken, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{})
			unitRoot := bed.unitRoot
			roundDir := filepath.Join(unitRoot, "run-one", "round-1")
			runPath := filepath.Join(unitRoot, "run-one", "run.json")
			stopPath := filepath.Join(roundDir, "stop-register.json")
			unit := launch.UnitRunRecord{ID: "run-one", Goal: "fix-docs", Worktree: bed.root, Unit: "docs", State: "awaiting-judgement", Rounds: []launch.UnitRound{{Number: 1, Directory: roundDir, Material: -1}}}
			transferWriteJSON(t, runPath, unit)
			transferWriteJSON(t, stopPath, map[string]any{"status": "open"})
			path := map[string]string{"store": unitRoot, "run": runPath, "stop": stopPath, "job-identity": filepath.Join(bed.root, "artifacts", "agents", "jobs", "review.json")}[broken]
			if broken == "store" {
				if err := os.Rename(unitRoot, unitRoot+"-saved"); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			brokenData := []byte("unreadable record")
			if broken == "job-identity" {
				brokenData = []byte("null")
			}
			if err := os.WriteFile(path, brokenData, 0600); err != nil {
				t.Fatal(err)
			}
			code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Finished explicitly.", "--by", "Wido", "--json")
			if code != 1 || !strings.Contains(out, "cleanup is pending") || !strings.Contains(bed.goalRecord("fix-docs"), "State: done") {
				t.Fatalf("advisory damage vetoed conclusion or hid pending cleanup: exit=%d output=%s stderr=%s", code, out, errOut)
			}
			tip := bed.tip()
			switch broken {
			case "store":
				if err := os.Remove(unitRoot); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(unitRoot+"-saved", unitRoot); err != nil {
					t.Fatal(err)
				}
			case "run":
				transferWriteJSON(t, runPath, unit)
			case "stop":
				transferWriteJSON(t, stopPath, map[string]any{"status": "open"})
			case "job-identity":
				transferWriteJSON(t, path, map[string]any{"jobId": "review", "role": "code-critic", "goalId": "fix-docs", "chainClosed": false})
			}
			gcliLedgerMust(t, bed, "goal", "done", "fix-docs", "--reason", "Another repeat reason.", "--by", "Wido")
			record, err := (&launch.UnitRunner{Root: unitRoot}).Status(unit.ID)
			data, readErr := os.ReadFile(stopPath)
			var stop map[string]any
			if json.Unmarshal(data, &stop) != nil {
				t.Fatalf("stop record: %s", data)
			}
			if err != nil || readErr != nil || bed.tip() != tip || record.ReviewCloseReason != "Finished explicitly." || record.Rounds[0].Material != -1 || stop["status"] != "closed" || stop["reason"] != "Finished explicitly." {
				t.Fatalf("repeat did not finish preserved cleanup: %+v %v %v %v", record, stop, err, readErr)
			}
		})
	}
}
