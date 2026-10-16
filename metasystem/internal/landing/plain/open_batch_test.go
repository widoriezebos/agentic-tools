package plain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func openBatchSelectionSeams(t *testing.T) ProveSeams {
	t.Helper()
	notAncestor := exec.Command("/usr/bin/false").Run()
	return ProveSeams{
		Now: func() time.Time { return bedNow }, NewID: func() string { return "batch" },
		AgentRunning: func() (bool, error) { return true, nil },
		Incidents:    func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch", "cat-file":
				return "", nil
			case "rev-parse":
				if args[len(args)-1] == "HEAD^{commit}" {
					return "fixed", nil
				}
				if args[len(args)-1] == "HEAD^{tree}" {
					return "tree", nil
				}
				return "main", nil
			case "merge-base":
				if args[3] == "fixed" && (args[2] == "tip" || args[2] == "main") {
					return "", nil
				}
				return "", notAncestor
			case "rev-list":
				if slices.Contains(args, "--parents") {
					return "merged main tip\nfixed merged", nil
				}
				return "fixed\nmerged\nmain", nil
			case "show":
				return "repair\n\nGoal-Unit: goal/lane-fix-20261011T040737-1", nil
			default:
				return "", fmt.Errorf("unexpected Git: %v", args)
			}
		},
	}
}

func TestOpenBatchRecordsOnlyHandInIsSelected(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	seams := openBatchSelectionSeams(t)
	if _, _, err := HandIn(install, Line{Goal: "records", SHA: "records-tip", Records: true}); err != nil {
		t.Fatal(err)
	}
	record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
	batch, err := SelectBatch(install, install, record, seams)
	want := []GoalSHA{{Goal: "records", SHA: "records-tip"}}
	if err != nil || batch == nil || batch.State != BatchPrepared || !batch.Open || !slices.Equal(batch.Members, want) {
		t.Fatalf("records-only selection: %+v %v", batch, err)
	}
	batch.Open = false
	batch.SealedAt = bedNow.Format(time.RFC3339Nano)
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	if _, _, err := HandIn(install, Line{Goal: "late-records", SHA: "late-records-tip", Records: true}); err != nil {
		t.Fatal(err)
	}
	status := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
	if !strings.Contains(status.Summary, "late-records waits for the next batch") {
		t.Fatal(status.Summary)
	}
}

func TestOpenBatchNewcomerWaitsUntilFixRoundEnds(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	seams := openBatchSelectionSeams(t)
	record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
	if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "tip"}); err != nil {
		t.Fatal(err)
	}
	batch, err := SelectBatch(install, install, record, seams)
	if err != nil {
		t.Fatal(err)
	}
	batch.State = BatchRunning
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	running := Running{Attempt: "20261011T040737.033326000Z", Commit: "merged", Tree: "red-tree", Gate: true, Checkpoint: true, BatchID: batch.ID, BatchMembers: batch.Members}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(gatesPath(install), Result{Result: Red, Attempt: running.Attempt, Commit: running.Commit, Tree: running.Tree, BatchID: batch.ID, BatchMembers: batch.Members}); err != nil {
		t.Fatal(err)
	}
	fix := &Fix{Attempt: running.Attempt, Round: 1, Parent: running.Commit, Commit: "fixed", Goal: "goal", Units: []string{"package"}, Job: "build", State: "reviewing"}
	if _, _, err := HandIn(install, Line{Goal: "late", SHA: "late-tip"}); err != nil {
		t.Fatal(err)
	}
	waiting, err := SelectBatch(install, install, record, seams)
	if err != nil {
		t.Fatal(err)
	}
	// The newcomer arrived before the builder could record its repair.
	beforeFix := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
	if err := WriteFix(install, fix); err != nil {
		t.Fatal(err)
	}
	// Admission must still accept the repair against the failed gate's members.
	if _, err := CheckBatch(install, install, fix.Commit, "main", true, seams); err != nil {
		t.Fatalf("newcomer wedged fix admission: %v", err)
	}
	if !strings.Contains(beforeFix.Summary, "late waits: a fix round is open") {
		t.Fatal(beforeFix.Summary)
	}
	status := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
	if !strings.Contains(status.Summary, "late waits: a fix round is open") {
		t.Fatal(status.Summary)
	}
	if !waiting.Open || !slices.Equal(waiting.Members, batch.Members) {
		t.Fatalf("fix round admitted newcomer: %+v", waiting)
	}
	next := Running{Attempt: "20261011T050737.033326000Z", Commit: fix.Commit, BatchID: batch.ID, BatchMembers: waiting.Members}
	if err := advanceFix(install, install, next, seams); err != nil {
		t.Fatal(err)
	}
	continued, err := FixForAttempt(install, next.Attempt)
	if err != nil || continued == nil || continued.State != "reviewing" || continued.ProofAttempt != running.Attempt || continued.Unit() != fix.Unit() {
		t.Fatalf("lost fix continuation: %+v %v", continued, err)
	}
	if err := writeRunning(install, next); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckBatch(install, install, fix.Commit, "main", true, seams); err != nil {
		t.Fatalf("follow-up proof lost repair admission: %v", err)
	}
	if err := closeFix(install, "goal"); err != nil {
		t.Fatal(err)
	}
	joined, err := SelectBatch(install, install, record, seams)
	if err != nil || !joined.Open || !slices.Equal(joined.Members, append(batch.Members, GoalSHA{Goal: "late", SHA: "late-tip"})) {
		t.Fatalf("closed round did not admit newcomer: %+v %v", joined, err)
	}
}

func TestOpenBatchNewSHAForSelectedGoalWaitsForNextBatch(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	seams := openBatchSelectionSeams(t)
	record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
	if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "tip"}); err != nil {
		t.Fatal(err)
	}
	first, err := SelectBatch(install, install, record, seams)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "new-tip"}); err != nil {
		t.Fatal(err)
	}
	selected, err := SelectBatch(install, install, record, seams)
	if err != nil || !selected.Open || !slices.Equal(selected.Members, first.Members) {
		t.Fatalf("new hand-in replaced the selected commit: %+v %v", selected, err)
	}
	status := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
	if !strings.Contains(status.Summary, "goal waits for the next batch") {
		t.Fatal(status.Summary)
	}
}

func TestOpenBatchReusedGreenSealsWithoutNewAttempt(t *testing.T) {
	t.Parallel()
	for _, surface := range []string{"start", "run", "settled"} {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			seams := openBatchSelectionSeams(t)
			batch := &Batch{ID: "batch", Base: "main", State: BatchPrepared, Open: true, Selector: PolicyValue{Value: "auto"}, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal", SHA: "tip"}}}
			if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "tip"}); err != nil {
				t.Fatal(err)
			}
			if err := writeBatch(install, batch); err != nil {
				t.Fatal(err)
			}
			// This candidate is a merge, not the repair used by the fix-round bed.
			git := seams.Git
			seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "rev-list" {
					return "fixed main tip", nil
				}
				return git(dir, args...)
			}
			green := Result{Tree: "tree", Commit: "fixed", Result: Green, Scope: "full", FullTree: "tree", FullAt: bedNow.Format(time.RFC3339), At: bedNow.Format(time.RFC3339), Attempt: "saved"}
			if err := appendLine(resultsPath(install), green); err != nil {
				t.Fatal(err)
			}
			switch surface {
			case "start":
				if _, already, err := Start(install, install, seams); err != nil || !already {
					t.Fatalf("reuse start: %v %v", already, err)
				}
			case "run":
				if result, err := Run(install, install, "unused", "", io.Discard, seams); err != nil || result.Attempt != green.Attempt {
					t.Fatalf("reuse run: %+v %v", result, err)
				}
			case "settled":
				if _, found, err := Settled(install, install, seams); err != nil || !found {
					t.Fatalf("reuse settled: %v %v", found, err)
				}
			}
			sealed, err := ReadBatch(install)
			if err != nil || sealed.Open || sealed.SealedAt != bedNow.Format(time.RFC3339Nano) {
				t.Fatalf("reused green left window open: %+v %v", sealed, err)
			}
			if _, err := os.Stat(runningPath(install)); !os.IsNotExist(err) {
				t.Fatalf("reuse created a running attempt: %v", err)
			}
			if _, already, err := Start(install, install, seams); err != nil || !already {
				t.Fatalf("repeated reuse start: %v %v", already, err)
			}
			again, err := ReadBatch(install)
			if err != nil || again.SealedAt != sealed.SealedAt || len(again.Restarts) != 0 {
				t.Fatalf("repeated reuse changed the seal or restarted: %+v %v", again, err)
			}
			if _, err := os.Stat(runningPath(install)); !os.IsNotExist(err) {
				t.Fatalf("repeated reuse created a running attempt: %v", err)
			}
		})
	}
}

func TestOpenBatchSavedHandInsAdmitEveryWaitingGoal(t *testing.T) {
	t.Parallel()
	for _, gap := range []time.Duration{9 * time.Second, 20 * time.Second} {
		t.Run(gap.String(), func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			now := bedNow
			notAncestor := exec.Command("/usr/bin/false").Run()
			seams := ProveSeams{
				Now: func() time.Time { return now }, NewID: func() string { return "selection" },
				AgentRunning: func() (bool, error) { return true, nil },
				GoalTier: func(_, id string) (uint8, error) {
					if gap == 20*time.Second && id == "lane-rehearsal-before-landing" {
						return 1, nil
					}
					return 3, nil
				},
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
				Git: func(_ string, args ...string) (string, error) {
					switch args[0] {
					case "fetch", "cat-file":
						return "", nil
					case "rev-parse":
						return "main", nil
					case "merge-base":
						return "", notAncestor
					default:
						return "", fmt.Errorf("unexpected Git: %v", args)
					}
				},
			}
			var lines []Line
			for _, name := range []string{"lane-rehearsal-before-landing", "shared-state-in-the-ledger"} {
				data, err := os.ReadFile(filepath.Join("testdata", "open-batch", name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var line Line
				if err := json.Unmarshal(data, &line); err != nil {
					t.Fatal(err)
				}
				lines = append(lines, line)
			}
			var err error
			now, err = time.Parse(time.RFC3339Nano, lines[0].At)
			if err != nil {
				t.Fatal(err)
			}
			record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
			selectBatch := func() *Batch {
				t.Helper()
				batch, err := SelectBatch(install, install, record, seams)
				if err != nil || batch == nil {
					t.Fatalf("selection: %+v %v", batch, err)
				}
				return batch
			}
			if _, _, err := HandIn(install, lines[0]); err != nil {
				t.Fatal(err)
			}
			first := selectBatch()
			if !first.Open || first.SealedAt != "" || len(first.Members) != 1 {
				t.Fatalf("prepared batch: %+v", first)
			}
			now = now.Add(gap)
			lines[1].At = now.Format(time.RFC3339Nano)
			if _, _, err := HandIn(install, lines[1]); err != nil {
				t.Fatal(err)
			}
			for _, line := range []Line{
				{Goal: "held", SHA: "held-tip"}, {Goal: "returned", SHA: "returned-tip"}, {Goal: "records", SHA: "records-tip", Records: true},
			} {
				if _, _, err := HandIn(install, line); err != nil {
					t.Fatal(err)
				}
			}
			if err := appendLine(queuePath(install), Line{Goal: "held", SHA: "held-tip", Outcome: StateWaiting, Held: true, After: first.Members}); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(queuePath(install), Line{Goal: "returned", SHA: "returned-tip", Outcome: StateReturned}); err != nil {
				t.Fatal(err)
			}
			batch := selectBatch()
			want := []GoalSHA{{Goal: lines[0].Goal, SHA: lines[0].SHA}, {Goal: lines[1].Goal, SHA: lines[1].SHA}, {Goal: "records", SHA: "records-tip"}}
			if batch.ID != first.ID || batch.Base != first.Base || !batch.Open || batch.DepthClass != "full" || !slices.Equal(batch.Members, want) {
				t.Fatalf("newcomer changed identity or eligibility: %+v", batch)
			}
			if again := selectBatch(); !slices.Equal(again.Members, want) {
				t.Fatalf("repeated selection duplicated members: %+v", again)
			}
			// A fresh selection must also include both simultaneous waiting goals.
			if err := os.Remove(batchPath(install)); err != nil {
				t.Fatal(err)
			}
			batch = selectBatch()
			if !slices.Equal(batch.Members, want) {
				t.Fatalf("initial selection omitted a waiting goal: %+v", batch)
			}
			batch.State, batch.Open, batch.SealedAt = BatchRunning, false, now.Format(time.RFC3339Nano)
			if err := writeBatch(install, batch); err != nil {
				t.Fatal(err)
			}
			if _, _, err := HandIn(install, Line{Goal: "late", SHA: "late-tip"}); err != nil {
				t.Fatal(err)
			}
			if sealed := selectBatch(); sealed.Open || !slices.Equal(sealed.Members, want) {
				t.Fatalf("sealed selection admitted a newcomer: %+v", sealed)
			}
			status := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
			if !strings.Contains(status.Summary, "batch selected "+now.Local().Format("15:04")+": 3 goals") || !strings.Contains(status.Summary, "late waits for the next batch") {
				t.Fatalf("sealed status: %s", status.Summary)
			}
		})
	}
}

func TestOpenBatchGateAdmissionLeavesWindowUntilFullProof(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	now := bedNow
	notAncestor := exec.Command("/usr/bin/false").Run()
	seams := ProveSeams{
		Now:        func() time.Time { return now },
		BudgetHome: t.TempDir(),
		Executable: func() (string, error) { return "fixture-engine", nil },
		Launch:     func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil },
		Incidents:  func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "cat-file":
				return "", nil
			case "rev-parse":
				if args[len(args)-1] == "HEAD^{commit}" {
					return "candidate", nil
				}
				if args[len(args)-1] == "HEAD^{tree}" {
					return "tree", nil
				}
				return "main", nil
			case "rev-list":
				return "candidate main tip", nil
			case "merge-base":
				if args[3] == "candidate" && (args[2] == "main" || args[2] == "tip") {
					return "", nil
				}
				return "", notAncestor
			default:
				return "", fmt.Errorf("unexpected Git: %v", args)
			}
		},
	}
	if _, _, err := HandIn(install, Line{Goal: "goal", SHA: "tip"}); err != nil {
		t.Fatal(err)
	}
	batch := &Batch{ID: "batch", Base: "main", State: BatchPrepared, Open: true, Selector: PolicyValue{Value: "auto"}, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal", SHA: "tip"}}}
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	seams.Gate = true
	if _, already, err := Start(install, install, seams); err != nil || already {
		t.Fatalf("start gate: already=%v err=%v", already, err)
	}
	gate, err := ReadBatch(install)
	if err != nil || !gate.Open || gate.State != BatchRunning || gate.SealedAt != "" {
		t.Fatalf("gate closed admission: %+v %v", gate, err)
	}
	lines, err := BatchLines(gate, nil, install, seams)
	if err != nil || lines[0] != "batch batch: open" {
		t.Fatalf("gate described as sealed: %v %v", lines, err)
	}
	// The isolated gate has finished before the full proof is admitted.
	if err := os.Remove(runningPath(install)); err != nil {
		t.Fatal(err)
	}
	seams.Gate = false
	now = now.Add(time.Minute)
	if _, err := checkBatchLocked(install, install, "candidate", "main", false, true, seams); err != nil {
		t.Fatal(err)
	}
	checked, err := ReadBatch(install)
	if err != nil || !checked.Open || checked.SealedAt != "" {
		t.Fatalf("candidate validation sealed without a proof attempt: %+v %v", checked, err)
	}
	if _, already, err := Start(install, install, seams); err != nil || already {
		t.Fatalf("start full proof: already=%v err=%v", already, err)
	}
	sealed, err := ReadBatch(install)
	if err != nil || sealed.Open || sealed.State != BatchRunning || sealed.SealedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("full proof did not record its seal: %+v %v", sealed, err)
	}
	now = now.Add(time.Minute)
	if _, err := checkBatchLocked(install, install, "candidate", "main", false, true, seams); err != nil {
		t.Fatal(err)
	}
	if err := recordBatchProofAttemptLocked(install, Running{BatchID: batch.ID}, seams); err != nil {
		t.Fatal(err)
	}
	again, err := ReadBatch(install)
	if err != nil || again.SealedAt != sealed.SealedAt {
		t.Fatalf("repeated admission rewrote the seal: %+v %v", again, err)
	}
}
