package plain

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

type keeperClockBed struct {
	*scopeBed
	keeper                     lane.AgentKeeper
	record                     lane.Record
	batch                      *Batch
	alive                      bool
	agents, trunks, selections int
	trunkReads, candidates     int
	selectionErr               error
}

func newKeeperClockBed(t *testing.T) *keeperClockBed {
	t.Helper()
	b := &keeperClockBed{scopeBed: newScopeBed(t)}
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	b.record = lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1, RegisteredBy: "Wido"}
	shadowRegister(t, home, b.record)
	b.batch = &Batch{ID: "clock-batch", Open: true, Lane: b.record, State: BatchPrepared, Base: "main", CreatedAt: bedNow.Format(time.RFC3339), Members: []GoalSHA{{Goal: "g", SHA: "sha"}}, DepthClass: "cheap"}
	original := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify origin/main^{commit}":
			b.trunkReads++
			return "main", nil
		case "rev-parse --verify main^{tree}":
			return "main-tree", nil
		}
		return original(dir, args...)
	}
	b.seams.Alive = func(Running) bool { return b.alive }
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	b.seams.Executable = func() (string, error) { return "fixture-engine", nil }
	b.seams.Launch = func(argv []string, _, _ string) (int64, error) {
		if !strings.Contains(strings.Join(argv, " "), "--trunk") {
			t.Fatalf("unexpected launch: %v", argv)
		}
		b.trunks++
		b.alive = true
		return int64(os.Getpid()), nil
	}
	b.keeper = lane.AgentKeeper{Home: home, Self: b.checkout, Now: func() time.Time { return bedNow },
		Sources: lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{WakeQueued, WakeFullDue}, nil }},
		Running: func() (string, bool, error) { return "", false, nil },
		Prepare: func(lane.Record) error {
			b.selections++
			if b.selectionErr != nil {
				return b.selectionErr
			}
			if b.batch != nil {
				current, err := ReadBatch(b.install)
				if err != nil {
					return err
				}
				if current != nil && current.ID == b.batch.ID {
					return nil
				}
				return writeBatch(b.install, b.batch, b.seams)
			}
			return nil
		},
		Clock: func(record lane.Record, prepare func(lane.Record) error) (string, error) {
			return KeeperClock(record, prepare, b.seams)
		},
		Holds: []func(string) (string, error){func(string) (string, error) { return ProofHold(b.install, b.seams) }},
		Start: func(string, lane.Wake) (string, error) { b.agents++; return "agent", nil },
	}
	return b
}
func (b *keeperClockBed) wake(t *testing.T) lane.AgentRun {
	t.Helper()
	run := b.keeper.Run()
	if run.Outcome == lane.AgentFailed || strings.Contains(run.Line, "can't") {
		t.Fatalf("wake: %+v", run)
	}
	return run
}
func (b *keeperClockBed) candidate(t *testing.T, verdict string) Result {
	t.Helper()
	r := Result{BatchID: b.batch.ID, Tree: "candidate", Commit: "candidate-commit", Attempt: "20261001T220000.000000000Z", Result: verdict, Scope: "full", At: bedNow.Format(time.RFC3339), FullTree: "candidate", FullAt: bedNow.Format(time.RFC3339)}
	if err := appendLine(resultsPath(b.install), r, b.seams); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestKeeperClockCandidatePushOpportunity(t *testing.T) {
	t.Parallel()
	for _, pushed := range []bool{true, false} {
		t.Run(map[bool]string{true: "pushed", false: "push refused"}[pushed], func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.wake(t)
			selected, err := ReadBatch(b.install)
			if err != nil || !selected.ClockFull || b.agents != 1 || b.trunks != 0 {
				t.Fatalf("prepare first: %+v %v agents=%d trunks=%d", selected, err, b.agents, b.trunks)
			}
			depth, _ := BatchProofDepth(b.install, b.checkout, Running{BatchID: selected.ID, Tree: "candidate"}, b.seams)
			if depth != "full" {
				t.Fatalf("batch lost its full-depth mark: %s", depth)
			}
			for _, gate := range []bool{true, false} {
				agents := b.agents
				trunkReads := b.trunkReads
				selections := b.selections
				b.alive = true
				if err := writeRunning(b.install, Running{Gate: gate, BatchID: b.batch.ID, Tree: "candidate", Attempt: "active"}, b.seams); err != nil {
					t.Fatal(err)
				}
				b.wake(t)
				if b.trunks != 0 || b.agents != agents || b.trunkReads != trunkReads || b.selections != selections+1 {
					t.Fatalf("proof owner stolen: trunks=%d agents=%d", b.trunks, b.agents)
				}
				b.alive = false
				if err := removeRunning(b.install, b.seams); err != nil {
					t.Fatal(err)
				}
				if gate {
					if err := appendLine(gatesPath(b.install), Result{BatchID: b.batch.ID, Result: Green, Attempt: "gate", At: bedNow.Format(time.RFC3339)}, b.seams); err != nil {
						t.Fatal(err)
					}
					b.wake(t)
					if b.trunks != 0 || b.agents != 2 {
						t.Fatal("gate did not continue to candidate")
					}
				}
			}
			r := b.candidate(t, Green)
			b.wake(t)
			if b.trunks != 0 || b.agents != 3 {
				t.Fatal("green candidate lost its push wake")
			}
			if due, err := fullProofDue(b.install, bedNow); err != nil || !due {
				t.Fatalf("unpublished green credited: %v %v", due, err)
			}
			if pushed {
				writePushRecords(t, b.install, Pushed{Tree: r.Tree, BatchID: r.BatchID, At: r.At})
			}
			b.wake(t)
			if pushed && b.trunks != 0 || !pushed && b.trunks != 1 {
				t.Fatalf("push/fallback: pushed=%v trunks=%d", pushed, b.trunks)
			}
			if !pushed && b.selections != 3 {
				t.Fatalf("retried batch selection: %d", b.selections)
			}
		})
	}
}

func TestKeeperClockRedCheckpointPreserved(t *testing.T) {
	t.Parallel()
	for _, repair := range []bool{true, false} {
		t.Run(map[bool]string{true: "repair obligation", false: "returned"}[repair], func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.wake(t)
			red := b.candidate(t, Red)
			if repair {
				checkpoint := Running{BatchID: b.batch.ID, BatchMembers: b.batch.Members, Tree: red.Tree, Commit: red.Commit, Attempt: red.Attempt, Checkpoint: true}
				if err := writeRunning(b.install, checkpoint, b.seams); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 2; i++ {
					b.wake(t)
				}
				saved, ok, _, err := ReadRunning(b.install, b.seams)
				if err != nil || !ok || !saved.Checkpoint || saved.Attempt != checkpoint.Attempt || b.trunks != 0 {
					t.Fatalf("checkpoint overwritten: %+v %v", saved, err)
				}
				trunk := b.seams
				trunk.Trunk = true
				if _, _, err := Start(b.install, b.checkout, trunk); err != nil {
					t.Fatalf("agent trunk admission changed: %v", err)
				}
				b.alive = false
				if err := writeRunning(b.install, checkpoint, b.seams); err != nil {
					t.Fatal(err)
				}
				fix := Fix{Attempt: red.Attempt, Round: 1, Parent: red.Commit, Goal: "g", Units: []string{"test"}, Job: "repair", Read: "read", Commit: "repair-commit", State: "reviewing"}
				if err := WriteFix(b.install, &fix); err != nil {
					t.Fatal(err)
				}
				b.batch.State = BatchRunning
				if err := writeBatch(b.install, b.batch, b.seams); err != nil {
					t.Fatal(err)
				}
				if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "sha"}); err != nil {
					t.Fatal(err)
				}
				notAncestor := exec.Command("/usr/bin/false").Run()
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					switch args[0] {
					case "cat-file":
						return "", nil
					case "merge-base":
						if (args[3] == "repair-commit" || args[3] == "candidate-commit") && (args[2] == "main" || args[2] == "sha" || args[2] == "candidate-commit") {
							return "", nil
						}
						return "", notAncestor
					case "rev-list":
						if args[1] == "--first-parent" {
							return "candidate-commit main sha\nrepair-commit candidate-commit", nil
						}
					case "rev-parse":
						if args[len(args)-1] == "HEAD^{commit}" {
							return "repair-commit", nil
						}
						if args[len(args)-1] == "HEAD^{tree}" {
							return "repair-tree", nil
						}
						if args[len(args)-1] == "refs/remotes/origin/main^{commit}" {
							return "main", nil
						}
					case "show":
						if args[1] == "-s" {
							return "repair\n\nGoal-Unit: g/" + fix.Unit(), nil
						}
					}
					return git(dir, args...)
				}
				repairSeams := b.seams
				repairSeams.CommandForCommit = func(string) (string, error) { return "proof-command", nil }
				repairSeams.NewID = func() string { return "repair-proof" }
				repairSeams.Launch = func([]string, string, string) (int64, error) { b.alive = true; return int64(os.Getpid()), nil }
				admitted, _, err := Start(b.install, b.checkout, repairSeams)
				if err != nil || admitted.BatchID != b.batch.ID || admitted.Checkpoint || admitted.Admission == nil || admitted.Admission.Scope != "full" {
					t.Fatalf("repair not admitted: %+v %v", admitted, err)
				}
				b.wake(t)
				if b.trunks != 1 {
					t.Fatal("fallback stole repair proof")
				}
				b.alive = false
				result, err := Run(b.install, b.checkout, "proof-command", admitted.Attempt, b.output, repairSeams)
				if err != nil || result.Result != Green || result.Scope != "full" {
					t.Fatalf("repair proof completion: %+v %v", result, err)
				}
				if err := closeFix(b.install, "g", b.seams); err != nil {
					t.Fatal(err)
				}
				b.wake(t)
				if b.trunks != 1 {
					t.Fatal("repair green lost its push opportunity")
				}
			}
			b.wake(t)
			wantTrunks := 1
			if repair {
				wantTrunks++
			}
			if b.trunks != wantTrunks {
				t.Fatalf("settled red did not fall back: %d", b.trunks)
			}
		})
	}
}

func TestKeeperClockSelectionFallbackOnce(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"person", "closed admission", "empty", "gate red"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			switch name {
			case "person":
				b.selectionErr = PolicyDecision(PolicyValue{Value: "person"}, nil)
				b.seams.Person = &ActProvenance{Kind: "proof", Person: "Wido", Destination: b.record}
			case "closed admission":
				b.selectionErr = errors.New("admission closed")
			case "empty":
				b.batch = nil
			}
			b.wake(t)
			if name == "gate red" {
				if err := appendLine(gatesPath(b.install), Result{BatchID: b.batch.ID, Result: Red, Attempt: "gate", At: bedNow.Format(time.RFC3339)}, b.seams); err != nil {
					t.Fatal(err)
				}
				b.wake(t)
			}
			b.wake(t)
			if b.trunks != 1 || b.selections != 2 {
				t.Fatalf("fallback attempts: trunks=%d selections=%d", b.trunks, b.selections)
			}
			if running, _, _, err := ReadRunning(b.install, b.seams); err != nil || running.Person != nil {
				t.Fatalf("automatic fallback borrowed a person's proof act: %+v %v", running, err)
			}
		})
	}
}

func TestKeeperClockRunningTrunkThenHandIn(t *testing.T) {
	t.Parallel()
	b := newKeeperClockBed(t)
	b.alive = true
	if err := writeRunning(b.install, Running{Trunk: true, Tree: "main-tree", Attempt: "trunk"}, b.seams); err != nil {
		t.Fatal(err)
	}
	b.wake(t)
	if b.agents != 0 || b.selections != 1 || b.trunks != 0 {
		t.Fatal("hand-in cancelled or shared the trunk proof")
	}
	b.alive = false
	if err := removeRunning(b.install, b.seams); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(resultsPath(b.install), Result{Trunk: true, Result: Red, CountedFull: true, Cause: &Cause{Kind: "main"}, At: bedNow.Format(time.RFC3339), Tree: "main-tree"}, b.seams); err != nil {
		t.Fatal(err)
	}
	b.wake(t)
	selected, err := ReadBatch(b.install)
	if err != nil || selected.ClockFull || b.agents != 1 || b.selections != 2 {
		t.Fatalf("batch after trunk: %+v %v", selected, err)
	}
	if due, err := fullProofDue(b.install, bedNow); err != nil || due {
		t.Fatalf("trunk result not credited: %v %v", due, err)
	}
	if !strings.Contains(ClockReason(b.install), "current") {
		t.Fatal("clock reason missing")
	}
	if _, err := os.Stat(filepath.Join(Dir(b.install), "clock.json")); err != nil {
		t.Fatal(err)
	}
}

func preparedClockCandidate(t *testing.T) *keeperClockBed {
	t.Helper()
	b := newKeeperClockBed(t)
	if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "sha"}); err != nil {
		t.Fatal(err)
	}
	b.wake(t)
	notAncestor := exec.Command("/usr/bin/false").Run()
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "cat-file":
			return "", nil
		case "merge-base":
			if args[3] == b.git.commit && (args[2] == "main" || args[2] == "sha") {
				return "", nil
			}
			return "", notAncestor
		case "rev-list":
			if args[1] == "--first-parent" {
				return b.git.commit + " main sha", nil
			}
		case "rev-parse":
			if args[len(args)-1] == "refs/remotes/origin/main^{commit}" {
				return "main", nil
			}
		}
		return git(dir, args...)
	}
	selected, err := ReadBatch(b.install)
	if err != nil {
		t.Fatal(err)
	}
	selected.Selector = PolicyValue{Value: "auto", Source: "fixture"}
	if err := writeBatch(b.install, selected, b.seams); err != nil {
		t.Fatal(err)
	}
	b.seams.CommandForCommit = func(string) (string, error) { return "proof-command", nil }
	b.seams.Launch = func(args []string, _, _ string) (int64, error) {
		if strings.Contains(strings.Join(args, " "), "--trunk") {
			b.trunks++
		} else {
			b.candidates++
		}
		b.alive = true
		return int64(os.Getpid()), nil
	}
	return b
}

func TestKeeperClockAdmitsFullCandidateAndCreditsPush(t *testing.T) {
	t.Parallel()
	b := preparedClockCandidate(t)
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || already || running.Trunk || running.Admission == nil || running.Admission.Scope != "full" {
		t.Fatalf("candidate admission: %+v %v %v", running, already, err)
	}
	selected, err := ReadBatch(b.install)
	if err != nil || selected.Open || selected.SealedAt == "" || !selected.ClockFull {
		t.Fatalf("admitted batch not sealed at full depth: %+v %v", selected, err)
	}
	b.wake(t)
	if b.trunks != 0 || b.candidates != 1 || b.agents != 1 {
		t.Fatal("keeper replaced the admitted proof")
	}
	b.alive = false
	result, err := Run(b.install, b.checkout, "proof-command", running.Attempt, b.output, b.seams)
	if err != nil || result.Result != Green || result.Scope != "full" {
		t.Fatalf("full candidate completion: %+v %v", result, err)
	}
	b.keeper.Start = func(string, lane.Wake) (string, error) {
		b.agents++
		writePushRecords(t, b.install, Pushed{BatchID: result.BatchID, Tree: result.Tree, At: result.At})
		return "push-agent", nil
	}
	b.wake(t)
	b.wake(t)
	if due, err := fullProofDue(b.install, bedNow); err != nil || due || b.trunks != 0 {
		t.Fatalf("pushed candidate did not discharge cadence: due=%v trunks=%d err=%v", due, b.trunks, err)
	}
}

func TestKeeperClockRefusedSealedCandidateFallsBack(t *testing.T) {
	t.Parallel()
	b := preparedClockCandidate(t)
	b.seams.Policy = func(key string) (PolicyValue, error) {
		value := "auto"
		if key == "landing.proof" {
			value = "person"
		}
		return PolicyValue{Value: value, Source: "fixture"}, nil
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err == nil {
		t.Fatal("candidate proof was admitted without its person's act")
	}
	selected, err := ReadBatch(b.install)
	if err != nil || selected.SealedAt == "" || selected.Open || b.candidates != 0 {
		t.Fatalf("refused proof did not leave a sealed batch: %+v %v", selected, err)
	}
	b.seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: "auto", Source: "fixture"}, nil }
	b.wake(t)
	if b.trunks != 1 || b.selections != 1 {
		t.Fatalf("sealed refusal suppressed fallback: trunks=%d selections=%d", b.trunks, b.selections)
	}
}

func TestKeeperClockFullDepthMarkSurvivesLaterCredit(t *testing.T) {
	t.Parallel()
	b := newKeeperClockBed(t)
	b.wake(t)
	if err := appendLine(resultsPath(b.install), Result{Trunk: true, Result: Green, At: bedNow.Format(time.RFC3339)}, b.seams); err != nil {
		t.Fatal(err)
	}
	b.seams.BatchDepth = func(string, string, Running) (string, string) { return "impact", "low-tier work" }
	depth, _ := BatchProofDepth(b.install, b.checkout, Running{BatchID: b.batch.ID, Tree: "candidate"}, b.seams)
	if depth != "full" {
		t.Fatalf("batch recomputed away its recorded full depth: %s", depth)
	}
}

func TestKeeperClockRefusalDoesNotFreezeLane(t *testing.T) {
	t.Parallel()
	for _, refusal := range []string{"environment", "budget", "unready fix", "target changed"} {
		t.Run(refusal, func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			now := bedNow
			b.seams.Now = func() time.Time { return now }
			b.keeper.Now = b.seams.Now
			b.batch = nil
			refusals := 0
			if refusal == "environment" {
				b.save(t, Result{Trunk: true, Tree: "main-tree", Result: Red, Cause: &Cause{Kind: "environment"}, At: now.Add(-5 * time.Hour).Format(time.RFC3339)})
			} else {
				b.seams.FenceCheck = func() error { refusals++; return errors.New(refusal) }
			}
			b.wake(t)
			reason := ClockReason(b.install)
			if !strings.Contains(reason, "could not start") || !strings.Contains(reason, "at "+now.Local().Format("15:04")) {
				t.Fatalf("refusal missing from clock: %q", reason)
			}
			reads := b.trunkReads
			for i := 0; i < 3; i++ {
				now = now.Add(time.Minute)
				b.wake(t)
			}
			if b.agents != 4 || b.selections != 4 || b.trunkReads != reads || refusals > 1 || ClockReason(b.install) != reason {
				t.Fatalf("refusal froze or retried: agents=%d selections=%d reads=%d/%d refusals=%d reason=%q", b.agents, b.selections, b.trunkReads, reads, refusals, ClockReason(b.install))
			}
			if _, _, err := HandIn(b.install, Line{Goal: "later", SHA: "later-sha"}); err != nil {
				t.Fatal(err)
			}
			b.batch = &Batch{ID: "later-batch", Open: true, Lane: b.record, State: BatchPrepared, Base: "main", Members: []GoalSHA{{Goal: "later", SHA: "later-sha"}}}
			b.wake(t)
			selected, err := ReadBatch(b.install)
			if err != nil || selected == nil || selected.ID != "later-batch" || b.agents != 5 {
				t.Fatalf("later hand-in stranded: %+v %v agents=%d", selected, err, b.agents)
			}
			b.batch = nil
			if err := os.Remove(batchPath(b.install)); err != nil {
				t.Fatal(err)
			}
			now = bedNow.Add(4 * time.Hour)
			b.wake(t)
			if b.trunkReads <= reads {
				t.Fatal("next cadence did not retry the fallback")
			}
		})
	}
}

func TestKeeperClockRepairObligationUsesCauseAndOpenFix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		cause   *Cause
		gate    bool
		openFix bool
		blocked bool
	}{
		{name: "nil cause", blocked: true},
		{name: "empty cause", cause: &Cause{}, blocked: true},
		{name: "unclassified", cause: &Cause{Kind: "unclassified"}, blocked: true},
		{name: "unknown", cause: &Cause{Kind: "unknown"}, blocked: true},
		{name: "own", cause: &Cause{Kind: "own"}, blocked: true},
		{name: "gate red", cause: &Cause{Kind: "unclassified"}, gate: true, blocked: true},
		{name: "main", cause: &Cause{Kind: "main"}},
		{name: "flake", cause: &Cause{Kind: "flake"}},
		{name: "environment", cause: &Cause{Kind: "environment"}},
		{name: "open fix", cause: &Cause{Kind: "main"}, openFix: true, blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.wake(t)
			red := Result{BatchID: b.batch.ID, Tree: "candidate", Commit: "candidate-commit", Attempt: "red-attempt", Result: Red, Cause: tc.cause, At: bedNow.Format(time.RFC3339)}
			mode := b.seams
			mode.Gate = tc.gate
			if tc.gate {
				red.Scope = "gate"
			}
			if err := appendLine(mode.resultsPath(b.install), red, b.seams); err != nil {
				t.Fatal(err)
			}
			checkpoint := Running{Checkpoint: true, Gate: tc.gate, BatchID: b.batch.ID, BatchMembers: b.batch.Members, Attempt: red.Attempt, Tree: red.Tree, Commit: red.Commit}
			if err := writeRunning(b.install, checkpoint, b.seams); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(runningPath(b.install))
			if err != nil {
				t.Fatal(err)
			}
			if tc.openFix {
				if err := WriteFix(b.install, &Fix{Attempt: red.Attempt, Goal: "g", Units: []string{"test"}, State: "building", Job: "repair"}); err != nil {
					t.Fatal(err)
				}
			}
			b.wake(t)
			if tc.blocked {
				saved, ok, _, err := ReadRunning(b.install, b.seams)
				if err != nil || !ok || saved.Attempt != red.Attempt || !saved.Checkpoint || b.trunks != 0 {
					t.Fatalf("repair checkpoint overwritten: %+v %v trunks=%d", saved, err, b.trunks)
				}
				after, err := os.ReadFile(runningPath(b.install))
				if err != nil || !bytes.Equal(before, after) || b.agents != 2 {
					t.Fatalf("repair wake changed checkpoint or withheld agent: before=%s after=%s err=%v agents=%d", before, after, err, b.agents)
				}
				if err := closeFix(b.install, "g", b.seams); err != nil {
					t.Fatal(err)
				}
				b.wake(t)
			}
			if b.trunks != 1 {
				t.Fatalf("cause=%s trunks=%d", tc.name, b.trunks)
			}
		})
	}
}

func TestKeeperClockAgentTrunkWithMainRedCheckpoint(t *testing.T) {
	t.Parallel()
	b := newKeeperClockBed(t)
	b.wake(t)
	red := b.candidate(t, Red)
	red.Cause = &Cause{Kind: "main"}
	if err := appendLine(resultsPath(b.install), red, b.seams); err != nil {
		t.Fatal(err)
	}
	if err := writeRunning(b.install, Running{Checkpoint: true, Attempt: red.Attempt, BatchID: red.BatchID, Tree: red.Tree}, b.seams); err != nil {
		t.Fatal(err)
	}
	b.keeper.Start = func(string, lane.Wake) (string, error) {
		seams := b.seams
		seams.Trunk = true
		running, _, err := Start(b.install, b.checkout, seams)
		if err != nil || !running.Trunk {
			t.Fatalf("agent trunk check refused: %+v %v", running, err)
		}
		return "agent", nil
	}
	// Explicit agent work bypasses the cadence decision.
	b.keeper.Explicit = true
	b.wake(t)
	if b.trunks != 1 {
		t.Fatal("agent did not re-prove main")
	}
}
