package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// The keeper fingerprint resolves the checkout through Git, so this adapter
// fixture exercises that boundary as well as the steward's stop cleanup.
func TestLandingStewardGitAdapterBarrenHoldKeepsStop(t *testing.T) {
	t.Parallel()
	for _, barren := range []int{2, 0} {
		t.Run(fmt.Sprint(barren), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			if out, err := exec.CommandContext(t.Context(), "git", "init", "--quiet", b.checkout).CombinedOutput(); err != nil {
				t.Fatalf("fixture checkout: %v %s", err, out)
			}
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fingerprint, err := KeeperFingerprint(b.checkout)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(lane.AgentState{Barren: barren, BarrenFingerprint: fingerprint})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
				t.Fatal(err)
			}
			stop := Stop{Loop: "lane-return", Subject: "lane", Attempt: 2, Decision: "stop", At: bedNow.Add(-time.Hour).Format(time.RFC3339Nano)}
			b.lines("stops.jsonl", stop)
			for tick := range 2 {
				b.step(bedNow.Add(time.Duration(tick) * time.Minute))
				open, err := OpenStops(b.install)
				if err != nil {
					t.Fatal(err)
				}
				lines, err := readLines[Stop](stopsPath(b.install))
				if err != nil {
					t.Fatal(err)
				}
				if barren == 2 {
					if len(open) != 1 || !reflect.DeepEqual(open[0], stop) || len(lines) != 1 {
						t.Fatalf("held stop changed: open=%+v lines=%+v", open, lines)
					}
				} else if len(open) != 0 || len(lines) != 2 || lines[1].Decision != "close" || lines[1].Handoff != "subject gone" || lines[1].At != bedNow.Format(time.RFC3339Nano) {
					t.Fatalf("cleared hold must close once: open=%+v lines=%+v", open, lines)
				}
			}
		})
	}
}

func TestLandingStewardOnlyPersonStopBounds(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(fmt.Sprint(person), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.staleQuestion()
			var stops []any
			for i := range 17 {
				stops = append(stops, Stop{Loop: "lane-proof", Subject: fmt.Sprintf("waiting-%d", i), Decision: "stop", At: bedNow.Add(-time.Hour).Format(time.RFC3339Nano)})
			}
			b.lines("stops.jsonl", stops...)
			if person {
				if _, err := lane.SetPauseBecause(b.home, "Wido", "maintenance", bedNow); err != nil {
					t.Fatal(err)
				}
				pause, present := lane.ReadPause(b.home)
				if !present || pause.By != "Wido" {
					t.Fatalf("person missing from stop record: %+v", pause)
				}
			}
			run := b.step(bedNow)
			if person && (len(b.calls) != 0 || !strings.Contains(run.Line, "bounded")) || !person && (len(b.calls) != 2 || len(run.Interventions) != 1) {
				t.Fatalf("person=%t: calls=%v run=%+v", person, b.calls, run)
			}
			open, err := OpenStops(b.install)
			if err != nil || len(open) != 17 {
				t.Fatalf("unknown subjects must retain their stops: %+v %v", open, err)
			}
		})
	}
}

func TestLandingStewardClosesGoneLoopStops(t *testing.T) {
	t.Parallel()
	for _, loop := range []string{"lane-proof", "lane-gate", "lane-return"} {
		t.Run(loop, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			// The landed subject is derived by the real status reader from Git's answer.
			b.seams.Git = func(_ string, args ...string) (string, error) {
				if args[0] == "merge-base" && args[len(args)-2] == "landed-sha" {
					return "", nil
				}
				if args[0] == "merge-base" {
					return "", exec.CommandContext(t.Context(), "false").Run()
				}
				return "main", nil
			}
			at := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
			b.lines("queue.jsonl",
				Line{Goal: "landed", SHA: "landed-sha", At: at},
				Line{Goal: "returned", SHA: "returned-sha", At: at},
				Line{Goal: "returned", SHA: "returned-sha", Outcome: StateReturned, At: at},
				Line{Goal: "superseded", SHA: "old-sha", At: at},
				Line{Goal: "superseded", SHA: "new-sha", At: at},
				Line{Goal: "waiting", SHA: "waiting-sha", At: at})
			stops := []any{
				Stop{Loop: loop, Subject: "landed", Tree: "landed-sha", Decision: "stop", At: at},
				Stop{Loop: loop, Subject: "returned", Tree: "returned-sha", Decision: "stop", At: at},
				Stop{Loop: loop, Subject: "superseded", Tree: "old-sha", ProofAttempt: "superseded-proof", Decision: "stop", At: at},
				Stop{Loop: loop, Subject: "waiting", Tree: "waiting-sha", ProofAttempt: "current", Decision: "stop", At: at, Class: "waiting goal needs proof"},
				Stop{Loop: "unit-round", Subject: "landed", Decision: "stop", At: at},
			}
			if loop != "lane-return" {
				stops = append(stops, Stop{Loop: loop, Subject: "waiting", ProofAttempt: "old", Decision: "stop", At: at}, Stop{Loop: loop, Subject: "landed, waiting", Decision: "stop", At: at})
			}
			b.lines("stops.jsonl", stops...)
			path := "results.jsonl"
			if loop == "lane-gate" {
				path = "gates.jsonl"
			}
			b.lines(path,
				Result{Attempt: "superseded-proof", Goals: []GoalSHA{{Goal: "superseded", SHA: "old-sha"}}, At: at},
				Result{Attempt: "old", Goals: []GoalSHA{{Goal: "waiting", SHA: "waiting-sha"}}, At: at},
				Result{Attempt: "current", Goals: []GoalSHA{{Goal: "waiting", SHA: "waiting-sha"}}, At: bedNow.Format(time.RFC3339Nano)})
			b.step(bedNow)
			open, err := OpenStops(b.install)
			want := 2
			if loop != "lane-return" {
				want++
			}
			if err != nil || len(open) != want {
				t.Fatalf("open=%+v err=%v", open, err)
			}
			lines, err := readLines[Stop](stopsPath(b.install))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range lines[len(stops):] {
				if s.Decision != "close" || s.Handoff != "subject gone" || s.StoppedAt == nil || s.At != bedNow.Format(time.RFC3339Nano) {
					t.Fatalf("wrong closure: %+v", s)
				}
			}
			status := ReadStatus(b.home, b.record, b.view, b.seams)
			if status.Health == nil || !strings.Contains(status.Health.Text(), "waiting goal needs proof") {
				t.Fatalf("health must explain the retained stop: %+v", status)
			}
			b.step(bedNow.Add(time.Minute))
			repeated, err := readLines[Stop](stopsPath(b.install))
			if err != nil || len(repeated) != len(lines) {
				t.Fatalf("cleanup replay appended closures: %+v %v", repeated, err)
			}
		})
	}
}

func TestLandingStewardRunningAttemptAndBarrenProgressCloseStops(t *testing.T) {
	t.Parallel()
	for _, gate := range []bool{false, true} {
		t.Run(fmt.Sprint(gate), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			loop := "lane-proof"
			if gate {
				loop = "lane-gate"
			}
			at := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
			b.lines("running.json", Running{Attempt: "current", Trunk: true, Gate: gate, Pid: 42, Since: at})
			b.seams.Alive = func(Running) bool { return true }
			b.lines("stops.jsonl",
				Stop{Loop: loop, Subject: "main", Trunk: true, ProofAttempt: "old", Decision: "stop", At: at},
				Stop{Loop: loop, Subject: "main", Trunk: true, ProofAttempt: "current", Decision: "stop", At: at, Class: "current proof"},
				Stop{Loop: "lane-return", Subject: "lane", Attempt: 2, Decision: "stop", At: at})
			b.step(bedNow)
			open, err := OpenStops(b.install)
			if err != nil || len(open) != 1 || open[0].ProofAttempt != "current" {
				t.Fatalf("current attempt lost or obsolete stops retained: %+v %v", open, err)
			}
		})
	}
}

func TestLandingHealthLoopStopsNeverPause(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("stops.jsonl", Stop{Loop: "lane-proof", Subject: "waiting", Decision: "stop", At: bedNow.Format(time.RFC3339Nano), Class: "red proof"})
	status := readHealthBed(t, b, healthSeams(bedNow))
	if status.Health.State != "idle" || status.Health.Paused != nil || !strings.HasPrefix(status.Health.Line(), "lane idle;") {
		t.Fatalf("loop stop paused the lane: %+v", status.Health)
	}
}

func TestLandingStewardCleanupReadsCurrentAttemptUnderLock(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	at := bedNow.Format(time.RFC3339Nano)
	b.lines("running.json", Running{Attempt: "current", Trunk: true})
	b.lines("stops.jsonl",
		Stop{Loop: "lane-proof", Subject: "main", Trunk: true, ProofAttempt: "old", Decision: "stop", At: at},
		Stop{Loop: "lane-proof", Subject: "main", Trunk: true, ProofAttempt: "current", Decision: "stop", At: at})
	status := Status{RunningProof: &RunningProof{Attempt: "old", Trunk: true}}
	if err := closeGoneLoopStops(b.install, status, bedNow); err != nil {
		t.Fatal(err)
	}
	open, err := OpenStops(b.install)
	if err != nil || len(open) != 1 || open[0].ProofAttempt != "current" {
		t.Fatalf("stale status overrode the current proof: %+v %v", open, err)
	}
}

func TestLandingHealthReturnedGoalsUseDayWindow(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	var rows []any
	for name, age := range map[string]time.Duration{"old": 10 * 24 * time.Hour, "recent": time.Hour, "future": -time.Hour} {
		at := bedNow.Add(-age - time.Hour).Format(time.RFC3339Nano)
		rows = append(rows, Line{Goal: name, SHA: name, At: at}, Line{Goal: name, SHA: name, At: bedNow.Add(-age).Format(time.RFC3339Nano), Outcome: StateReturned})
	}
	b.lines("queue.jsonl", rows...)
	h := readHealthBed(t, b, healthSeams(bedNow)).Health
	if len(h.Goals) != 1 || h.Goals[0].Goal != "recent" || h.Goals[0].Minutes == nil || *h.Goals[0].Minutes != 60 {
		t.Fatalf("returned window: %+v", h.Goals)
	}
}

func TestLandingStewardLegacyStopKeepsCurrentWaitingGoal(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	at := bedNow.Format(time.RFC3339Nano)
	b.lines("queue.jsonl", Line{Goal: "waiting", SHA: "old", At: at}, Line{Goal: "waiting", SHA: "current", At: at})
	b.lines("results.jsonl", Result{Goals: []GoalSHA{{Goal: "waiting", SHA: "old"}}, At: at})
	b.lines("stops.jsonl", Stop{Loop: "lane-proof", Subject: "waiting", Decision: "stop", At: at})
	b.seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", exec.CommandContext(t.Context(), "false").Run()
		}
		return "main", nil
	}
	b.step(bedNow)
	open, err := OpenStops(b.install)
	if err != nil || len(open) != 1 {
		t.Fatalf("an unidentified older proof closed the waiting goal's stop: %+v %v", open, err)
	}
}
