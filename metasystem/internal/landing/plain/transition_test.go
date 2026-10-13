package plain

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestLaneRecordWriterTransitions(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	s.Alive = func(Running) bool { return true }
	now := bedNow
	s.Now = func() time.Time { return now }
	if _, err := RefreshShadow(b.home, now, "keeper", s); err != nil {
		t.Fatal(err)
	}
	step := func(by string, kinds []string, run func() error) {
		t.Helper()
		now = now.Add(time.Minute)
		before := len(shadowEvents(t, b.install))
		if err := run(); err != nil {
			t.Fatalf("%s: %v", by, err)
		}
		events := shadowEvents(t, b.install)[before:]
		if len(events) != len(kinds) {
			t.Fatalf("%s: %d events for %d raw writes: %+v", by, len(events), len(kinds), events)
		}
		for i, event := range events {
			wantBy := by
			if strings.HasPrefix(event.Kind, "batch.") {
				wantBy = "batch"
			}
			if event.Kind != kinds[i] || event.By != wantBy || event.Rebuild || event.Evidence == "" || !event.At.Equal(now) {
				t.Fatalf("%s event: %+v", by, event)
			}
		}
		if diff, err := RefreshShadow(b.home, now.Add(time.Second), "keeper", s); err != nil || len(diff) != 0 {
			t.Fatalf("%s refresh: %v %v", by, diff, err)
		}
		status := ObserveStatus(b.home, b.record, s, nil)
		if len(status.Problems) > 0 || status.Health == nil || healthRow(status.Health, "drift") != nil {
			t.Fatalf("%s health: %+v", by, status)
		}
		for _, event := range shadowEvents(t, b.install) {
			if event.Kind == "drift" {
				t.Fatalf("%s drift: %+v", by, event)
			}
		}
	}
	locked := func(run func() error) func() error { return func() error { return withLock(b.install, run) } }
	step("pause", []string{"lane"}, func() error { _, err := lane.SetPause(b.home, "person", now); return err })
	step("resume", []string{"lane"}, func() error { _, err := lane.ClearPauseAt(b.home, "person", now); return err })
	step("hand-in", []string{"hand_ins"}, func() error {
		_, _, err := HandInChecked(b.install, Line{Goal: "A", SHA: "a", At: now.Format(time.RFC3339Nano)}, "", s)
		return err
	})
	batch := &Batch{ID: "batch", Lane: b.record, Base: "main", State: BatchPrepared, Open: true, Members: []GoalSHA{{Goal: "A", SHA: "a"}}}
	step("batch", []string{"batch.execution"}, locked(func() error { batch.CreatedAt = now.Format(time.RFC3339Nano); return writeBatch(b.install, batch, s) }))
	step("hand-in", []string{"hand_ins"}, func() error {
		_, _, err := HandInChecked(b.install, Line{Goal: "B", SHA: "b", At: now.Format(time.RFC3339Nano)}, "", s)
		return err
	})
	step("batch", []string{"batch.execution"}, locked(func() error {
		batch.Members = append(batch.Members, GoalSHA{Goal: "B", SHA: "b"})
		return writeBatch(b.install, batch, s)
	}))
	step("gate", []string{"batch.execution"}, locked(func() error {
		batch.State, batch.RunningAt = BatchRunning, now.Format(time.RFC3339Nano)
		return writeBatch(b.install, batch, s)
	}))
	for _, scope := range []string{"gate", "full"} {
		s.Gate = scope == "gate"
		by := "gate"
		if scope == "full" {
			by = "prove"
		}
		if scope == "full" {
			step(by, []string{"batch.admission"}, locked(func() error { return sealBatchLocked(b.install, batch.ID, s) }))
		}
		step(by, []string{"proof"}, locked(func() error {
			return writeRunning(b.install, Running{Attempt: scope, Tree: "tree", Commit: "head", BatchID: batch.ID, BatchMembers: batch.Members, Gate: scope == "gate", Pid: 42, Since: now.Format(time.RFC3339Nano)}, s)
		}))
		step(by, nil, locked(func() error {
			return appendLine(s.resultsPath(b.install), Result{Attempt: scope, Tree: "tree", Commit: "head", Result: Green, Scope: scope, BatchID: batch.ID, BatchMembers: batch.Members, Goals: batch.Members, CountedFull: scope == "full", At: now.Format(time.RFC3339Nano)}, s)
		}))
		step("prove", []string{"proof"}, locked(func() error { return removeRunning(b.install, s) }))
	}
	batch, _ = ReadBatch(b.install)
	step("push", []string{"main", "batch.execution"}, locked(func() error { return completePush(b.install, b.checkout, batch, "before", "main", "tree", now, s) }))
	step("return", []string{"hand_ins"}, locked(func() error {
		return appendLine(queuePath(b.install), Line{Goal: "B", SHA: "b", Outcome: StateReturned, At: now.Format(time.RFC3339Nano), Reason: "needs correction"}, s)
	}))
	if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), []byte(`{"by":"hand edit","at":"2026-10-01T23:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if diff, err := RefreshShadow(b.home, now, "keeper", s); err != nil {
			t.Fatalf("hand edit refresh: %v %v", diff, err)
		}
	}
	count := 0
	for _, event := range shadowEvents(t, b.install) {
		if event.Kind == "drift" {
			count++
			if event.By != "keeper" {
				t.Fatal(event)
			}
		}
	}
	if count != 1 {
		t.Fatalf("hand edit drift count = %d", count)
	}
}

func TestLaneQuestionFixAndAgentWritersEmitTransitions(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome, s.Alive = b.home, func(Running) bool { return true }
	s.AgentProbe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: true}, nil }
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	check := func(kind, by string, run func() error) {
		t.Helper()
		before := len(shadowEvents(t, b.install))
		if err := run(); err != nil {
			t.Fatal(err)
		}
		events := shadowEvents(t, b.install)[before:]
		if len(events) != 1 || events[0].Kind != kind || events[0].By != by {
			t.Fatalf("%s: %+v", by, events)
		}
		if diff, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil || len(diff) != 0 {
			t.Fatalf("%s: %v %v", by, diff, err)
		}
	}
	var finish func(time.Time) error
	check("agent", "prove", func() (err error) { finish, err = BeginAgentStep(b.install, "prove", nil, bedNow, s); return err })
	check("agent", "prove", func() error { return finish(bedNow.Add(time.Minute)) })
	stop := Stop{Loop: "lane-return", Subject: "g", Decision: "stop", Handoff: "ask lane", At: bedNow.Format(time.RFC3339Nano)}
	check("stop", "prove", func() error {
		return withLock(b.install, func() error { return appendLine(stopsPath(b.install), stop, s) })
	})
	check("lane", "stop", func() error { _, err := lane.SetPauseBecause(b.home, "person", "maintenance", bedNow); return err })
	check("lane", "resume", func() error { _, err := lane.ClearPauseAt(b.home, "person", bedNow); return err })
	check("questions", "ask", func() error { return SyncPolicyQuestion(b.install, s.Machine, bedNow, s) })
	check("loop-stop", "keeper", func() error {
		return retireLifecycle(b.install, Status{Queue: []Entry{{Goal: "g", State: StateReturned}}}, bedNow, s, "loop-stop")
	})
	check("questions", "withdraw", func() error { return SyncPolicyQuestion(b.install, s.Machine, bedNow, s) })
	git := s.Git
	s.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify MERGE_HEAD^{commit}" {
			return "tip", nil
		}
		return git(dir, args...)
	}
	fix := &Fix{Attempt: "fix", Goal: "g", Tip: "tip", Commit: "head", State: "resolving", Paths: []conflict.Path{{Path: "app.go"}}, Units: []string{"merge"}}
	check("resolution", "resolve", func() error { return withLock(b.install, func() error { return WriteFix(b.install, fix, s) }) })
	fix.State = "abandoned"
	check("resolution", "resolve", func() error { return withLock(b.install, func() error { return WriteFix(b.install, fix, s) }) })
	check("fix", "keeper", func() error {
		return retireLifecycle(b.install, Status{Queue: []Entry{{Goal: "g", SHA: "tip", State: StateReturned}}}, bedNow, s, "fix")
	})
}

func TestLaggingLaneTransitionRecordsDefectWithoutFailingWrite(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	b.lines("queue.jsonl", Line{Goal: "old", SHA: "old", At: bedNow.Format(time.RFC3339Nano)})
	if _, _, err := HandInChecked(b.install, Line{Goal: "new", SHA: "new", At: bedNow.Format(time.RFC3339Nano)}, "", s); err != nil {
		t.Fatal(err)
	}
	entries, err := Entries(b.install)
	if err != nil || len(entries) != 2 {
		t.Fatalf("raw write: %+v %v", entries, err)
	}
	events := shadowEvents(t, b.install)
	if events[len(events)-1].Kind != "defect" || events[len(events)-1].By != "hand-in" {
		t.Fatal(events)
	}
	r, _, err := state.Read(Dir(b.install))
	if err != nil || len(r.HandIns) != 0 {
		t.Fatalf("refusal overwrote shadow: %+v %v", r, err)
	}
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
}

func TestFailedLaneRawWriteEmitsNoTransition(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	before := len(shadowEvents(t, b.install))
	failure := errors.New("fixture write failed")
	if err := withLock(b.install, func() error {
		return transitionWrite(b.install, "proof", "prove", bedNow, "", func() error { return failure }, s)
	}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(shadowEvents(t, b.install)) != before {
		t.Fatal("failed raw write emitted a transition")
	}
}

func TestLanePauseResumeKeepsReadinessIndependentOfPause(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.AgentReady = func(root string) error { return lane.UnarmedRefusal(root) }
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"stop", "resume"} {
		if verb == "stop" {
			if _, err := lane.SetPauseBecause(b.home, "person", "maintenance", bedNow); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := lane.ClearPauseAt(b.home, "person", bedNow); err != nil {
				t.Fatal(err)
			}
		}
		if diff, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "keeper", s); err != nil || len(diff) != 0 {
			t.Fatalf("%s: %v %v", verb, diff, err)
		}
		observation, err := captureObservation(b.home, &b.record, s, nil)
		if err != nil || !observation.Coherent {
			t.Fatalf("%s observation: %+v %v", verb, observation, err)
		}
		inputs := RenderInputsOf(observation.Status, observation.Status.Raw)
		if !inputs.Unready {
			t.Fatalf("%s lost readiness: %+v", verb, inputs)
		}
	}
}

func TestLaneProofProgressDoesNotCreateDrift(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome, s.Alive = b.home, func(Running) bool { return true }
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "proof.log")
	if err := os.WriteFile(log, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := withLock(b.install, func() error {
		return writeRunning(b.install, Running{Attempt: "proof", Tree: "tree", Commit: "head", Pid: 42, Since: bedNow.Format(time.RFC3339Nano), Log: log}, s)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, []byte("landing planned 2\nlanding package module package ok 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if diff, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "keeper", s); err != nil || len(diff) != 0 {
		t.Fatalf("progress drift: %v %v", diff, err)
	}
	status := ObserveStatus(b.home, b.record, s, nil)
	if status.RunningProof == nil || status.RunningProof.UnitsDone != 1 || status.RunningProof.UnitsTotal == nil || *status.RunningProof.UnitsTotal != 2 || !strings.Contains(status.ProofHeadline, "1 of 2 units") {
		t.Fatalf("lost live progress: %+v", status)
	}
}

func TestLaneProofExecutionLeavesNoDrift(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{Green, Red} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			home := shadowHome(t, b.checkout, b.install)
			s := b.seams
			git := s.Git
			s.Git = func(dir string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "symbolic-ref --quiet --short HEAD":
					return "main", nil
				case "rev-parse --verify MERGE_HEAD^{commit}":
					return "", nil
				case "rev-parse --verify --quiet refs/heads/main^{commit}":
					return "parent", nil
				}
				return git(dir, args...)
			}
			s.BudgetHome = home
			s.Incidents = shadowSeams().Incidents
			if _, err := RefreshShadow(home, bedNow, "keeper", s); err != nil {
				t.Fatal(err)
			}
			command := "printf 'LANDING-CHECKED\\t0\\n'"
			if outcome == Red {
				command += "; exit 127"
			}
			result, err := Run(b.install, b.checkout, command, "", io.Discard, s)
			if err != nil || result.Result != outcome {
				t.Fatalf("proof: %+v %v", result, err)
			}
			if diff, err := RefreshShadow(home, bedNow.Add(time.Minute), "keeper", s); err != nil || len(diff) != 0 {
				t.Fatalf("refresh: %v %v", diff, err)
			}
			transitions := 0
			for _, event := range shadowEvents(t, b.install) {
				if event.Kind == "drift" || event.Kind == "defect" {
					t.Fatalf("proof left drift: %+v", event)
				}
				if event.Kind == "proof" {
					transitions++
					if event.By != "prove" {
						t.Fatal(event)
					}
				}
			}
			if transitions != 2 {
				t.Fatalf("proof start and result must transition; removing the finished running record is unchanged: %d", transitions)
			}
		})
	}
}
