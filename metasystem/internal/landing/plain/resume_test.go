package plain

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type resumeBed struct {
	home, install             string
	record                    lane.Record
	seams                     ProveSeams
	keeper                    lane.AgentKeeper
	merge, head, main, policy string
	owned, writer, agent      bool
	aborts, starts, proofs    int
	incidents                 []goal.TrunkRedEntry
	calls                     []string
}

func newResumeBed(t *testing.T) *resumeBed {
	t.Helper()
	b := &resumeBed{home: t.TempDir(), install: t.TempDir(), head: "base", main: "base", policy: "auto"}
	b.record = lane.Record{Root: b.install, Install: b.install, CustodyEpoch: 1}
	write := func(path string, value any) {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(lane.HostDir(b.home), "landing-lane.json"), b.record)
	if err := os.MkdirAll(filepath.Join(b.install, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	exit := func(code string) error { t.Helper(); return exec.Command("/bin/sh", "-c", "exit "+code).Run() }
	absent, outside := exit("128"), exit("1")
	b.seams = ProveSeams{BudgetHome: b.home, Now: func() time.Time { return bedNow }, NewID: func() string { return "selection" },
		Policy:    func(string) (PolicyValue, error) { return PolicyValue{Value: b.policy}, nil },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return b.incidents, nil },
		Alive:     func(Running) bool { return false }, Executable: func() (string, error) { return "fixture-engine", nil },
		Launch: func(argv []string, _, _ string) (int64, error) {
			if !strings.Contains(strings.Join(argv, " "), "--trunk") {
				t.Fatalf("not trunk: %v", argv)
			}
			b.proofs++
			return int64(os.Getpid()), nil
		},
		Git: func(_ string, args ...string) (string, error) {
			b.calls = append(b.calls, strings.Join(args, " "))
			switch args[0] {
			case "symbolic-ref":
				return "main", nil
			case "fetch", "cat-file":
				return "", nil
			case "rev-parse":
				if strings.Contains(args[len(args)-1], "MERGE_HEAD") {
					if b.merge == "" {
						return "", absent
					}
					return b.merge, nil
				}
				if strings.Contains(args[len(args)-1], "refs/remotes/") {
					return b.main, nil
				}
				return b.head, nil
			case "merge-base":
				return "", outside
			case "rev-list":
				if strings.Contains(args[len(args)-1], "base..new-main") {
					return "new-main", nil
				}
				return "", nil
			case "diff-tree":
				return "app.go\x00", nil
			case "merge":
				if reflect.DeepEqual(args, []string{"merge", "--abort"}) {
					b.aborts++
					b.merge = ""
					return "", nil
				}
			}
			t.Fatalf("unstubbed git %v", args)
			return "", nil
		},
	}
	b.keeper = lane.AgentKeeper{Home: b.home, Self: b.install, Now: func() time.Time { return bedNow },
		Running: func() (string, bool, error) { return "agent", b.agent, nil }, Start: func(string, lane.Wake) (string, error) { b.starts++; b.agent = true; return "agent", nil },
		Fingerprint: func(string) (string, error) { return "same", nil }, Sources: lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{WakeQueued}, nil }},
	}
	b.seams.ResolutionStore = &launch.Store{Root: t.TempDir()}
	b.seams.ResumeKeeper = &b.keeper
	if err := withLock(b.install, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.LockPath(b.home), nil, 0600); err != nil {
		t.Fatal(err)
	}
	return b
}
func (b *resumeBed) hold(t *testing.T) {
	t.Helper()
	data, _ := json.Marshal(lane.AgentState{Barren: 2, BarrenFingerprint: "same"})
	if err := os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func (b *resumeBed) queue(t *testing.T) {
	t.Helper()
	for i, g := range []string{"newer", "older"} {
		at := bedNow.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339)
		if _, _, err := HandIn(b.install, Line{Goal: g, SHA: g, At: at}); err != nil {
			t.Fatal(err)
		}
	}
}
func (b *resumeBed) resume(t *testing.T) ResumeRun {
	t.Helper()
	if b.merge != "" {
		state := launch.Completed
		if b.owned {
			state = launch.Running
		}
		if err := b.seams.ResolutionStore.Create(launch.Record{ID: "job", State: state, OutputOwnerUnproven: b.writer}); err != nil {
			t.Fatal(err)
		}
	}
	run, err := Resume(b.home, ActProvenance{Person: "Wido", Root: b.install, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 20}}, bedNow, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestResumeMergeIdentityAndWriters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, state, head, tip string
		owned, writer          bool
		want                   int
	}{
		{"abandoned", "abandoned", "base", "tip", false, false, 1},
		{"current", "resolving", "base", "tip", false, false, 0},
		{"different tip", "abandoned", "base", "another", false, false, 0},
		{"different checkpoint", "abandoned", "another", "tip", false, false, 0},
		{"owned abandoned", "abandoned", "base", "tip", true, false, 0},
		{"unsettled abandoned", "abandoned", "base", "tip", false, true, 0},
		{"ended with code move", "resolving", "base", "tip", false, false, 1},
		{"running with code move", "resolving", "base", "tip", true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newResumeBed(t)
			b.merge = "tip"
			b.owned, b.writer = tc.owned, tc.writer
			if strings.Contains(tc.name, "code move") {
				b.main = "new-main"
			}
			fix := &Fix{Attempt: "fix", Goal: "older", Job: "job", Units: []string{"p"}, State: tc.state, Commit: tc.head, Tip: tc.tip, Paths: []conflict.Path{{Path: "app.go"}}}
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(Dir(b.install), "fixes", "fix.json"))
			run := b.resume(t)
			if b.aborts != tc.want {
				t.Fatalf("aborts=%d want %d: %+v", b.aborts, tc.want, run)
			}
			after, _ := os.ReadFile(filepath.Join(Dir(b.install), "fixes", "fix.json"))
			if tc.want == 0 && string(before) != string(after) {
				t.Fatal("preserved merge record changed")
			}
			if tc.name == "ended with code move" {
				var got Fix
				json.Unmarshal(after, &got)
				if got.State != "abandoned" || got.Reason != "superseded by main's code move; rebuild" {
					t.Fatalf("fix=%+v", got)
				}
			}
			if tc.want == 0 && b.merge != "tip" {
				t.Fatal("merge lost")
			}
		})
	}
}
func TestResumeSelectionClearsHoldsOldestFirst(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"auto", "person"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b := newResumeBed(t)
			b.policy = policy
			b.queue(t)
			if policy == "auto" {
				b.hold(t)
			}
			run := b.resume(t)
			batch, err := ReadBatch(b.install)
			if err != nil || batch == nil || batch.Person == nil || batch.Members[0].Goal != "older" || b.starts != 1 {
				t.Fatalf("selection=%+v starts=%d run=%+v err=%v", batch, b.starts, run, err)
			}
			state, _ := lane.ReadAgentState(b.home)
			if state.Barren != 0 {
				t.Fatalf("hold=%+v", state)
			}
		})
	}
}
func TestResumeSealedBatchRestartsWithNewcomer(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.queue(t)
	b.main = "new-main"
	batch := &Batch{ID: "batch", Lane: b.record, Base: "base", Members: []GoalSHA{{Goal: "older", SHA: "older"}}, State: BatchRunning, SealedAt: bedNow.Format(time.RFC3339)}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	run := b.resume(t)
	got, _ := ReadBatch(b.install)
	if got == nil || got.Base != "new-main" || len(got.Members) != 2 || got.Person == nil || b.starts != 1 {
		t.Fatalf("batch=%+v run=%+v starts=%d", got, run, b.starts)
	}
}
func TestResumeTrunkProofOwedOrHoldEmpty(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                   string
		incident, hold, proved bool
		want                   int
	}{{"incident", true, false, false, 1}, {"already proved", true, false, true, 0}, {"empty hold", false, true, false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newResumeBed(t)
			if tc.incident {
				b.incidents = []goal.TrunkRedEntry{{Identity: "red", Opened: bedNow.Format(time.RFC3339)}}
			}
			if tc.hold {
				b.hold(t)
			}
			if tc.proved {
				if err := appendLine(resultsPath(b.install), Result{Trunk: true, Commit: b.main, Result: Green}); err != nil {
					t.Fatal(err)
				}
			}
			run := b.resume(t)
			if b.proofs != tc.want {
				t.Fatalf("proofs=%d run=%+v", b.proofs, run)
			}
		})
	}
}
func resumeSnapshot(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				out[path] = string(data)
			}
			return nil
		})
	}
	return out
}
func TestResumeRunningLaneChangesNoRecords(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.agent = true
	b.queue(t)
	before := resumeSnapshot(t, b.home, b.install)
	run := b.resume(t)
	after := resumeSnapshot(t, b.home, b.install)
	if !reflect.DeepEqual(before, after) || !strings.Contains(strings.Join(run.Lines, " "), "nothing to resume") || b.starts != 0 {
		t.Fatalf("changed running lane: %+v", run)
	}
}
func TestResumeNeverCancelsPersonProof(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.hold(t)
	b.queue(t)
	r := Running{Attempt: "person-proof", Commit: "base", Tree: "base", Person: &ActProvenance{Kind: "proof", Person: "Wido"}, Pid: 99}
	if err := writeRunning(b.install, r); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return true }
	b.seams.EndProof = func(Running) error { t.Fatal("cancelled person's proof"); return nil }
	before := resumeSnapshot(t, b.install, b.home)
	b.resume(t)
	if !reflect.DeepEqual(before, resumeSnapshot(t, b.install, b.home)) || b.starts+b.proofs != 0 {
		t.Fatal("changed running proof")
	}
}
func TestResumeResolutionLaunchState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := launch.Store{Root: root}
	for _, state := range []launch.State{launch.Running, launch.Completed} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			record := launch.Record{ID: string(state), State: state}
			dir, err := store.StateDir(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(record)
			if err := os.WriteFile(filepath.Join(dir, "record.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			s := ProveSeams{ResolutionStore: &store}
			got, _, err := resolutionSettled(Fix{Job: record.ID}, s)
			if err != nil || got != state.Terminal() {
				t.Fatalf("settled=%v err=%v", got, err)
			}
		})
	}
	if _, err := Resume(root, ActProvenance{}, bedNow, ProveSeams{}); err == nil {
		t.Fatal("missing lane accepted")
	}
}

func TestResumeNaturallyResolvingMergeAfterJobEnds(t *testing.T) {
	t.Parallel()
	for _, owned := range []bool{true, false} {
		t.Run(map[bool]string{true: "running", false: "ended"}[owned], func(t *testing.T) {
			t.Parallel()
			source := newResolveFixture(t)
			source.contract.Generated = nil
			source.seams.AsSeat = true
			source.seams.Proof.NewID = func() string { return "natural-fix" }
			out, err := source.resolve()
			if err != nil || out.Outcome != "resolving" {
				t.Fatalf("resolve=%+v err=%v", out, err)
			}
			fix, err := ReadFix(source.install)
			if err != nil || fix == nil || fix.State != "resolving" {
				t.Fatalf("fix=%+v err=%v", fix, err)
			}
			b := newResumeBed(t)
			b.install = source.install
			b.record = lane.Record{Root: source.checkout, Install: source.install, CustodyEpoch: 1}
			data, _ := json.Marshal(b.record)
			if err := os.WriteFile(lane.RecordPath(b.home), data, 0600); err != nil {
				t.Fatal(err)
			}
			b.keeper.Self = source.checkout
			b.head = fix.Commit
			b.merge = fix.Tip
			b.main = "new-main"
			b.owned = owned
			fix.Job = "job"
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "rev-list" && len(args) == 3 && args[1] == "--reverse" {
					return "new-main", nil
				}
				return git(dir, args...)
			}
			run := b.resume(t)
			want := 1
			if owned {
				want = 0
			}
			if b.aborts != want {
				t.Fatalf("aborts=%d want %d run=%+v", b.aborts, want, run)
			}
			got, err := readFix(b.install, true)
			if err != nil || got == nil || !owned && (got.State != "abandoned" || got.Reason != "superseded by main's code move; rebuild") {
				t.Fatalf("fix=%+v err=%v", got, err)
			}
		})
	}
}

func TestResumeOpenBatchAdmitsWaitingGoal(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.queue(t)
	batch := &Batch{ID: "batch", Lane: b.record, Base: b.main, Members: []GoalSHA{{Goal: "older", SHA: "older"}}, State: BatchPrepared, Open: true, Selector: PolicyValue{Value: "auto"}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	run := b.resume(t)
	got, err := ReadBatch(b.install)
	if err != nil || got == nil || len(got.Members) != 2 || len(got.Actions) != 1 || got.Actions[0].Kind != "reopen" {
		t.Fatalf("batch=%+v run=%+v err=%v", got, run, err)
	}
}

func TestResumePausedPersonProofPreservesAttempt(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	_, err := lane.SetPause(b.home, "Wido", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	r := Running{Attempt: "person", Tree: "tree", Commit: "base", Person: &ActProvenance{Kind: "proof", Person: "Wido"}, Pid: 99}
	if err := writeRunning(b.install, r); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return true }
	before, _ := os.ReadFile(runningPath(b.install))
	run := b.resume(t)
	after, _ := os.ReadFile(runningPath(b.install))
	_, paused := lane.ReadPause(b.home)
	if paused || string(before) != string(after) || b.starts+b.proofs != 0 {
		t.Fatalf("paused=%v run=%+v", paused, run)
	}
}

func TestResumePreservedMergeContinuesAgent(t *testing.T) {
	t.Parallel()
	for _, job := range []string{"job", ""} {
		t.Run("job="+job, func(t *testing.T) {
			t.Parallel()
			b := newResumeBed(t)
			b.merge = "tip"
			fix := &Fix{Attempt: "fix", Goal: "older", Job: job, Units: []string{"p"}, State: "resolving", Commit: b.head, Tip: b.merge, Paths: []conflict.Path{{Path: "app.go"}}}
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
				t.Fatal(err)
			}
			b.queue(t)
			if _, _, err := SetDrain(b.install, Drain{By: "Wido", At: bedNow.Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			b.policy = "person"
			b.incidents = []goal.TrunkRedEntry{{Identity: "red", Opened: bedNow.Format(time.RFC3339)}}
			before := resumeSnapshot(t, filepath.Join(Dir(b.install), "fixes"))
			run := b.resume(t)
			_, paused := lane.ReadPause(b.home)
			drain, err := ReadDrain(b.install)
			batch, batchErr := ReadBatch(b.install)
			if paused || drain != nil || err != nil || batchErr != nil || batch != nil || b.starts != 1 || b.proofs != 0 || b.aborts != 0 || b.merge != "tip" || !reflect.DeepEqual(before, resumeSnapshot(t, filepath.Join(Dir(b.install), "fixes"))) {
				t.Fatalf("paused=%v drain=%+v batch=%+v starts=%d proofs=%d aborts=%d run=%+v errors=%v/%v", paused, drain, batch, b.starts, b.proofs, b.aborts, run, err, batchErr)
			}
			if job == "" && !strings.Contains(strings.Join(run.Lines, "; "), "the resolution of older has no job; the agent continues it") {
				t.Fatalf("missing job reported as running: %+v", run)
			}
		})
	}
}

func TestResumeFetchesMainBeforeStaleMergeDecision(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.merge = "tip"
	if err := WriteFix(b.install, &Fix{Attempt: "fix", Goal: "older", Job: "job", Units: []string{"p"}, State: "resolving", Commit: b.head, Tip: b.merge, Paths: []conflict.Path{{Path: "app.go"}}}); err != nil {
		t.Fatal(err)
	}
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			b.main = "new-main"
		}
		return git(dir, args...)
	}
	run := b.resume(t)
	fix, err := readFix(b.install, true)
	if err != nil || fix == nil || fix.State != "abandoned" || fix.Reason != "superseded by main's code move; rebuild" || b.aborts != 1 || b.merge != "" || b.starts != 1 {
		t.Fatalf("fix=%+v aborts=%d merge=%q starts=%d run=%+v err=%v", fix, b.aborts, b.merge, b.starts, run, err)
	}
	if len(b.calls) == 0 || !strings.HasPrefix(b.calls[0], "fetch ") {
		t.Fatalf("merge decision preceded fetch: %v", b.calls)
	}
}

func TestResumeFetchFailurePreservesPausedMerge(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.merge = "tip"
	if err := WriteFix(b.install, &Fix{Attempt: "fix", Goal: "older", Job: "job", Units: []string{"p"}, State: "resolving", Commit: b.head, Tip: b.merge, Paths: []conflict.Path{{Path: "app.go"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
		t.Fatal(err)
	}
	before := resumeSnapshot(t, b.home, b.install)
	fetchErr := errors.New("origin unavailable")
	b.seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] != "fetch" {
			t.Fatalf("merge inspected after failed fetch: %v", args)
		}
		return "", fetchErr
	}
	run, err := Resume(b.home, ActProvenance{}, bedNow, b.seams)
	if !errors.Is(err, fetchErr) || !reflect.DeepEqual(before, resumeSnapshot(t, b.home, b.install)) || b.starts+b.aborts+b.proofs != 0 {
		t.Fatalf("fetch failure changed lane: run=%+v err=%v", run, err)
	}
}

func TestRestartBindsAndClosesOnlyCurrentBarrenStop(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.queue(t)
	batch := &Batch{ID: "batch", Lane: b.record, Base: b.main, Members: []GoalSHA{{Goal: "older", SHA: "older"}}, State: BatchPrepared, SealedAt: bedNow.Format(time.RFC3339Nano)}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	at := bedNow.Add(-time.Minute).Format(time.RFC3339Nano)
	stops := []Stop{
		{Loop: "lane-return", Subject: "lane", Attempt: 2, Decision: "stop", At: at},
		{Loop: "lane-return", Subject: "older", Attempt: 2, Decision: "stop", At: at},
		{Loop: "lane-proof", Subject: "lane", Attempt: 2, Decision: "stop", At: at},
	}
	for _, stop := range stops {
		if err := appendLine(stopsPath(b.install), stop); err != nil {
			t.Fatal(err)
		}
	}
	person := ActProvenance{Person: "Wido", Root: b.install, CheckedAt: bedNow, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 20}, Destination: b.record}
	got, message, err := ActOnBatch(b.install, "restart", "", person, b.seams)
	if err != nil || message != "" || got.Person == nil || got.Person.BarrenStopAt != at || len(got.Actions) != 1 || got.Actions[0].Kind != "restart" {
		t.Fatalf("restart=%+v message=%q err=%v", got, message, err)
	}
	open, err := OpenStops(b.install)
	if err != nil || len(open) != 2 || slices.ContainsFunc(open, func(stop Stop) bool { return stop.Loop == "lane-return" && stop.Subject == "lane" }) {
		t.Fatalf("wrong closures: %+v err=%v", open, err)
	}
	stored, err := ReadBatch(b.install)
	if err != nil || !reflect.DeepEqual(got, stored) {
		t.Fatalf("restart lost durable selection: %+v err=%v", stored, err)
	}
}

func TestResumeRebuildPreservesUnownedMerge(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.merge = "tip"
	if err := WriteFix(b.install, &Fix{Attempt: "fix", Goal: "older", Job: "job", Units: []string{"merge"}, State: "rebuild", Commit: b.head, Tip: b.merge}); err != nil {
		t.Fatal(err)
	}
	run := b.resume(t)
	if b.aborts != 0 || b.merge != "tip" || b.starts != 1 || !strings.Contains(strings.Join(run.Lines, "\n"), "the current merge is preserved") {
		t.Fatalf("aborts=%d merge=%q starts=%d run=%+v", b.aborts, b.merge, b.starts, run)
	}
}

func TestResumePersonProvesMainDuringCandidateRepair(t *testing.T) {
	t.Parallel()
	b := newResumeBed(t)
	b.incidents = []goal.TrunkRedEntry{{Identity: "red", Opened: bedNow.Format(time.RFC3339)}}
	checkpoint := Running{Attempt: "candidate", Tree: "candidate-tree", Commit: "candidate", Checkpoint: true}
	if err := writeRunning(b.install, checkpoint); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(resultsPath(b.install), Result{Attempt: checkpoint.Attempt, Tree: checkpoint.Tree, Commit: checkpoint.Commit, Result: Red}); err != nil {
		t.Fatal(err)
	}
	if err := WriteFix(b.install, &Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}); err != nil {
		t.Fatal(err)
	}
	run := b.resume(t)
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || !running.Trunk || running.Person == nil || b.proofs != 1 || !run.Changed {
		t.Fatalf("resume: %+v running=%+v proofs=%d error=%v", run, running, b.proofs, err)
	}
}
