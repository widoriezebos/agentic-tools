package plain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingStewardPersonActWithoutEffect(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(Health{PersonActs: []PersonAct{{Verb: "landing run", At: "stamp"}}, OwnRed: []OwnRed{{Goal: "g", SHA: "sha", Replay: "green"}}})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	for field, keys := range map[string][]string{"person_acts": {"verb", "at", "minutes"}, "own_red": {"goal", "sha", "replay"}} {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(fields[field], &rows); err != nil || len(rows) != 1 {
			t.Fatalf("health JSON %s=%s err=%v", field, fields[field], err)
		}
		for _, key := range keys {
			if _, ok := rows[0][key]; !ok {
				t.Fatalf("health JSON %s lacks lowercase %s: %s", field, key, fields[field])
			}
		}
	}
	stamp := bedNow.Format(time.RFC3339Nano)
	for _, scenario := range []string{"selection", "step", "last-step", "proof", "running-proof", "no-hold", "restart", "reopen", "pending-give-back", "give-back", "return"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: true}, nil }
			members := []GoalSHA{{Goal: "g", SHA: "sha"}}
			person := &ActProvenance{Kind: "selection", Person: "Wido", CheckedAt: bedNow, Root: b.checkout, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 42}, Destination: b.record, Subject: members}
			batch := &Batch{ID: "selected", Lane: b.record, Base: "main", State: BatchPrepared, Person: person, Members: members}
			verb := "landing run"
			switch scenario {
			case "restart", "reopen":
				batch.Actions = []BatchAction{{Kind: scenario, By: "Wido", At: stamp}}
				verb = "landing " + scenario
			case "pending-give-back":
				batch.Person = nil
				batch.Actions = []BatchAction{{Kind: "give-back", By: "Wido", At: stamp, Person: person, Pending: true}}
				verb = "landing give-back"
			case "give-back", "return":
				batch.Person = nil
				person.Kind = "return"
				cause := &Cause{Kind: "own", Name: "Wido"}
				verb = "landing return"
				if scenario == "give-back" {
					cause.Kind, verb = "unclassified", "landing give-back"
				}
				b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: stamp, Outcome: StateReturned, Cause: cause, ReturnOrigin: &ReturnOrigin{Person: person}})
			case "step", "last-step":
				step := AgentStep{Verb: "landing merge", StartedAt: bedNow.Add(3 * time.Minute).Format(time.RFC3339Nano), PID: 42}
				b.seams.Alive = func(Running) bool { return false }
				if scenario == "step" {
					b.lines("agent-step.json", step)
				} else {
					b.lines("agent-idle.json", agentIdle{EndedAt: bedNow.Add(4 * time.Minute).Format(time.RFC3339Nano), Step: &step})
				}
			case "proof":
				b.lines("results.jsonl", Result{Attempt: "proof", Result: Green, At: bedNow.Add(4 * time.Minute).Format(time.RFC3339Nano), StartedAt: stamp})
			case "running-proof":
				b.lines("running.json", Running{Attempt: "proof", Since: stamp, Pid: 42})
				b.seams.Alive = func(Running) bool { return true }
			}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			barren := 2
			if scenario == "no-hold" {
				barren = 0
			}
			data, err := json.Marshal(lane.AgentState{Barren: barren})
			if err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(filepath.Dir(lane.AgentStatePath(b.home)), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
				t.Fatal(err)
			}
			read := func(now time.Time) Status {
				b.seams.Now = func() time.Time { return now }
				return ReadStatus(b.home, b.record, b.view, b.seams)
			}
			if run := b.step(bedNow.Add(4 * time.Minute)); len(run.Interventions) != 0 || healthRow(read(bedNow.Add(4*time.Minute)).Health, "person-act-no-effect") != nil {
				t.Fatalf("person act reported before five minutes: %+v", run)
			}
			run := b.step(bedNow.Add(5 * time.Minute))
			want := !strings.Contains(scenario, "step") && scenario != "proof" && scenario != "running-proof" && scenario != "no-hold"
			// Before its five minutes the act that will fire waits, with its age.
			if pending := read(bedNow.Add(4 * time.Minute)).Health.PendingAct; (pending != nil) != want || want && (pending.Minutes == nil || *pending.Minutes != 4) {
				t.Fatalf("the act's wait at four minutes: %+v, want one with its minutes: %t", pending, want)
			}
			row := healthRow(read(bedNow.Add(5*time.Minute)).Health, "person-act-no-effect")
			if (row != nil) != want || len(run.Interventions) != map[bool]int{true: 1, false: 0}[want] {
				t.Fatalf("effect evidence: row=%+v run=%+v", row, run)
			}
			if want {
				e := run.Interventions[0]
				if e.Row != "person-refused" || e.Subject != verb+" "+stamp || e.Why != "person-act-no-effect" || e.What != "took no effect in 5 min: no agent step, no proof start, the hold is back" || e.How != "recorded as a defect; no second wake" {
					t.Fatalf("wrong person refusal: %+v", e)
				}
			}
			b.step(bedNow.Add(6 * time.Minute))
			history, err := readLines[Intervention](interventionsPath(b.install))
			if err != nil || len(history) != len(run.Interventions) || len(b.calls) != 0 {
				t.Fatalf("repeated refusal or a second wake: history=%+v calls=%v err=%v", history, b.calls, err)
			}
		})
	}
}

func TestAGiveBackThatEmptiesTheBatchHasTakenEffect(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"empty", "absent", "member-left"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: true}, nil }
			person := &ActProvenance{Kind: "return", Person: "Wido", CheckedAt: bedNow, Root: b.checkout, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 42}, Destination: b.record}
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: bedNow.Format(time.RFC3339Nano), Outcome: StateReturned, Cause: &Cause{Kind: "unclassified", Name: "Wido"}, ReturnOrigin: &ReturnOrigin{Person: person}})
			if scenario != "absent" {
				batch := &Batch{ID: "given-back", Lane: b.record, Base: "main", State: BatchPrepared}
				if scenario == "member-left" {
					batch.Members = []GoalSHA{{Goal: "other", SHA: "other-sha"}}
				}
				if err := writeBatch(b.install, batch, b.seams); err != nil {
					t.Fatal(err)
				}
			}
			data, err := json.Marshal(lane.AgentState{Barren: 2})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
				t.Fatal(err)
			}
			read := func(now time.Time) *Health {
				b.seams.Now = func() time.Time { return now }
				status := ReadStatus(b.home, b.record, b.view, b.seams)
				if status.Health == nil || len(status.Problems) != 0 {
					t.Fatalf("health unreadable: %v", status.Problems)
				}
				return status.Health
			}
			want := scenario == "member-left"
			pending := read(bedNow.Add(4 * time.Minute)).PendingAct
			run := b.step(bedNow.Add(6 * time.Minute))
			h := read(bedNow.Add(6 * time.Minute))
			row := healthRow(h, "person-act-no-effect")
			if (row != nil) != want || h.PendingAct != nil || len(run.Interventions) != map[bool]int{true: 1, false: 0}[want] || len(b.calls) != 0 {
				t.Fatalf("give-back effect: row=%+v pending=%+v run=%+v calls=%v", row, h.PendingAct, run, b.calls)
			}
			if want && (run.Interventions[0].Row != "person-refused" || run.Interventions[0].Why != "person-act-no-effect") {
				t.Fatalf("wrong give-back intervention: %+v", run.Interventions[0])
			}
			if (pending != nil) != want {
				t.Fatalf("pending give-back before five minutes: %+v, want pending: %t", pending, want)
			}
		})
	}
}

func TestLandingStewardReturnsOwnRedAsTheLane(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"own", "main", "flake", "wrong-goal", "wrong-sha", "newer-green", "gate"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.seams.Lane = func() (lane.Record, error) { return b.record, nil }
			git := b.seams.Git
			exitOne := exec.Command("false").Run()
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "merge-base" {
					return "", exitOne
				}
				return git(dir, args...)
			}
			cause := &Cause{Kind: kind, Goal: "g", SHA: "sha", Tests: []string{"TestOwn"}, Evidence: "named test red on g", Name: "original"}
			if kind == "wrong-goal" {
				cause.Kind, cause.Goal = "own", "other"
			}
			if kind == "wrong-sha" {
				cause.Kind, cause.SHA = "own", "old-sha"
			}
			if kind == "newer-green" || kind == "gate" {
				cause.Kind = "own"
			}
			replay := "named replay green on main: TestOwn"
			proof := Result{Attempt: "red", At: bedNow.Add(-time.Minute).Format(time.RFC3339Nano), Result: Red, Goals: []GoalSHA{{Goal: "g", SHA: "sha"}}, Cause: cause, Replay: replay}
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: bedNow.Add(-2 * time.Minute).Format(time.RFC3339Nano)})
			file := "results.jsonl"
			if kind == "gate" {
				file = "gates.jsonl"
			}
			b.lines(file, proof)
			if kind == "newer-green" {
				b.lines("gates.jsonl", Result{At: bedNow.Format(time.RFC3339Nano), Result: Green, Goals: proof.Goals})
			}
			b.lines("stops.jsonl", Stop{Loop: "lane-proof", Subject: "g", Decision: "stop", Handoff: "return g own", At: proof.At, ProofAttempt: proof.Attempt})
			run, stepErr := StewardStep(b.home, bedNow, b.seams)
			entry, ok, err := Latest(b.install, "g")
			want := kind == "own" || kind == "gate"
			if stepErr != nil || err != nil || !ok || (entry.State == StateReturned) != want || len(run.Interventions) != map[bool]int{true: 1, false: 0}[want] {
				t.Fatalf("own return: entry=%+v run=%+v err=%v stepErr=%v", entry, run, err, stepErr)
			}
			if want && (entry.Reason != replay || !reflect.DeepEqual(entry.Cause, cause) || run.Interventions[0].Outcome != "returned g") {
				t.Fatalf("lane changed the proven cause or replay: entry=%+v cause=%+v run=%+v", entry, cause, run)
			}
			questions, _ := channel.WalkQuestions(b.install)
			if len(questions) != 0 || len(b.calls) != 0 {
				t.Fatalf("return asked or woke: questions=%+v calls=%v", questions, b.calls)
			}
		})
	}
}

func TestLandingStewardStopsAnActThatFailsTheSameWay(t *testing.T) {
	t.Parallel()
	failing := func(t *testing.T) *stewardBed {
		t.Helper()
		b := newStewardBed(t)
		b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
			var incidents []goal.TrunkRedEntry
			for _, id := range []string{"first", "second", "third"} {
				incidents = append(incidents, goal.TrunkRedEntry{ID: id, Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "old-main"}}})
			}
			return incidents, nil
		}
		b.seams.Steward.Trunk = func(string, string, ProveSeams) (string, error) {
			b.calls = append(b.calls, "trunk")
			return "", errors.New("launch failed")
		}
		return b
	}
	step := func(t *testing.T, b *stewardBed, minute int, wantFailure bool) StewardRun {
		t.Helper()
		run, err := StewardStep(b.home, bedNow.Add(time.Duration(minute)*time.Minute), b.seams)
		if (err != nil) != wantFailure || err != nil && !strings.Contains(err.Error(), "launch failed") {
			t.Fatalf("run=%+v err=%v", run, err)
		}
		return run
	}
	questions := func(t *testing.T, b *stewardBed) []channel.Question {
		t.Helper()
		qs, unreadable := channel.WalkQuestions(b.install)
		if len(unreadable) != 0 {
			t.Fatal(unreadable)
		}
		return qs
	}
	t.Run("bound-and-retirement", func(t *testing.T) {
		t.Parallel()
		b := failing(t)
		run := step(t, b, 0, true)
		qs := questions(t, b)
		if len(b.calls) != 2 || len(run.Interventions) != 3 || run.Interventions[2].Row != "steward-act-failed" || !strings.Contains(run.Line, "bounded: its last two acts failed the same way") || len(qs) != 1 {
			t.Fatalf("third subject was not bounded once: run=%+v calls=%v questions=%+v", run, b.calls, qs)
		}
		q := qs[0]
		decision := "The landing lane's steward stopped acting on unproven-incidents: its last two acts failed the same way"
		ifNothing := "the steward does not act on unproven-incidents again until you act or the outcome changes; this question is withdrawn when unproven-incidents no longer fires"
		tried := "tried: first: metasystem landing prove --trunk (prove main before asking anything) → failed: launch failed; second: metasystem landing prove --trunk (prove main before asking anything) → failed: launch failed"
		if q.State != "open" || q.About != "lane" || q.Kind != "other" || q.Machine != "landing" || q.Lineage != lane.AgentLineage || len(q.Facts) != 3 || q.Facts[0] != decision || q.Facts[2] != tried || q.Plain.Decision != decision || q.Plain.IfNothing != ifNothing || channel.LaneStopCommand(q) != "" {
			t.Fatalf("question omits what was tried or changes the ask: %+v", q)
		}
		if premise, err := LaneQuestionPremise(b.install, q, b.seams); err != nil || premise != "unknown" {
			t.Fatalf("steward premise=%q err=%v", premise, err)
		}
		for _, minute := range []int{1, 2, 61} {
			if run = step(t, b, minute, false); len(run.Interventions) != 0 || len(b.calls) != 2 || len(questions(t, b)) != 1 {
				t.Fatalf("same failure retried: %+v calls=%v", run, b.calls)
			}
		}
		history, err := readLines[Intervention](interventionsPath(b.install))
		if err != nil || len(history) != 3 {
			t.Fatalf("duplicate defect: %+v %v", history, err)
		}
		status := ReadStatus(b.home, b.record, b.view, b.seams)
		if row := healthRow(status.Health, "steward-act-failed"); row == nil || row.Subject != "unproven-incidents" || row.Act != "stopped acting on unproven-incidents: failed: launch failed; asked once" {
			t.Fatalf("health lost the defect: %+v", row)
		}
		b.seams.Incidents = healthSeams(bedNow).Incidents
		run = step(t, b, 62, false)
		closed, err := channel.ReadQuestion(b.install, q.ID)
		if err != nil || closed.State != "closed" || !strings.Contains(run.Line, "the situation ended: unproven-incidents no longer fires") {
			t.Fatalf("ended situation still asks: %+v %v", closed, err)
		}
	})
	t.Run("person-allows-one-retry", func(t *testing.T) {
		t.Parallel()
		b := failing(t)
		step(t, b, 0, true)
		qs := questions(t, b)
		if len(qs) != 1 {
			t.Fatalf("two failures did not ask once: %+v calls=%v", qs, b.calls)
		}
		q := qs[0]
		if _, err := channel.Withdraw(b.install, q.ID, "person acted", nil, channel.DestinationConfig{}); err != nil {
			t.Fatal(err)
		}
		run := step(t, b, 1, true)
		if len(b.calls) != 3 || len(run.Interventions) != 2 || run.Interventions[1].Row != "steward-act-failed" {
			t.Fatalf("person did not allow exactly one retry: %+v calls=%v", run, b.calls)
		}
		if len(questions(t, b)) != 2 {
			t.Fatal("new failure did not ask once after the person acted")
		}
	})
	t.Run("policy-question-is-not-withdrawable", func(t *testing.T) {
		t.Parallel()
		b := newStewardBed(t)
		stop := Stop{Loop: "lane-proof", Subject: "g", Decision: "stop", At: bedNow.Add(-time.Hour).Format(time.RFC3339Nano)}
		data, err := json.Marshal(stop)
		if err != nil {
			t.Fatal(err)
		}
		q, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Now: bedNow.Add(-time.Hour), Facts: []string{"Decide the lane stop", "lane stop: " + string(data), "evidence"}})
		if err != nil {
			t.Fatal(err)
		}
		closed := stop
		closed.Decision = "close"
		b.lines("stops.jsonl", stop, closed)
		status := ReadStatus(b.home, b.record, b.view, b.seams)
		if len(status.Health.Questions) != 1 || !status.Health.Questions[0].Policy || status.Health.Questions[0].Premise != "gone" || healthRow(status.Health, "stale-question") != nil {
			t.Fatalf("policy question fired stale-question: %+v", status.Health)
		}
		run := step(t, b, 0, false)
		got, err := channel.ReadQuestion(b.install, q.ID)
		if err != nil || got.State != "open" || len(b.calls) != 0 || len(run.Interventions) != 0 {
			t.Fatalf("policy question was withdrawn: %+v run=%+v calls=%v err=%v", got, run, b.calls, err)
		}
	})
	t.Run("unknown-is-kept-and-old-ended-question-retires", func(t *testing.T) {
		t.Parallel()
		b := newStewardBed(t)
		var qs []channel.Question
		for _, fact := range []string{"lane steward: {", `lane steward: {"ask":"cannot-heal","row":"overdue-goal","subject":"gone"}`} {
			q, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Now: bedNow.Add(-48 * time.Hour), Facts: []string{"A situation", fact, "tried: a verb"}})
			if err != nil {
				t.Fatal(err)
			}
			qs = append(qs, q)
		}
		run := step(t, b, 0, false)
		unknown, err := channel.ReadQuestion(b.install, qs[0].ID)
		if err != nil || unknown.State != "open" || !strings.Contains(run.Line, "steward question "+unknown.ID+" unreadable") {
			t.Fatalf("unknown was not preserved: %+v run=%+v err=%v", unknown, run, err)
		}
		ended, err := channel.ReadQuestion(b.install, qs[1].ID)
		if err != nil || ended.State != "closed" {
			t.Fatalf("old ended question not retired: %+v %v", ended, err)
		}
	})
}

func TestLandingStewardAsksOnceWhenTheLaneCannotHeal(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"waiting", "silent", "landed", "returned", "proving", "person-proof", "old-question", "recent-act", "young-step", "person-stop"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			stamp := bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp})
			if scenario == "silent" {
				b.lines("queue.jsonl")
				b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) {
					return lane.OwnerProbe{Alive: true, Since: bedNow.Add(-81 * time.Minute)}, nil
				}
			}
			if scenario == "returned" {
				b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp}, Line{Goal: "g", SHA: "tip", At: bedNow.Add(9 * time.Minute).Format(time.RFC3339Nano), Outcome: StateReturned})
			}
			if scenario == "proving" || scenario == "person-proof" {
				running := Running{Attempt: "proving", Commit: "candidate", Since: bedNow.Add(-time.Minute).Format(time.RFC3339Nano), Pid: 42}
				if scenario == "person-proof" {
					running.Person = &ActProvenance{Person: "Wido"}
				}
				b.lines("running.json", running)
				b.seams.Alive = func(Running) bool { return true }
			}
			if scenario == "person-stop" {
				if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "young-step" {
				b.lines("agent-step.json", AgentStep{Verb: "work wait", StartedAt: bedNow.Add(-time.Minute).Format(time.RFC3339Nano), PID: 42})
				b.seams.Alive = func(Running) bool { return true }
			}
			if scenario == "recent-act" {
				entries := []any{Intervention{At: bedNow.Add(-10 * time.Minute).Format(time.RFC3339Nano), Row: "overdue-goal", Subject: "g", Outcome: "reported"}}
				for i := 0; i < 6; i++ {
					entries = append(entries, Intervention{At: bedNow.Add(-time.Minute).Format(time.RFC3339Nano), Row: "other", Subject: "other"})
				}
				b.lines("interventions.jsonl", entries...)
			}
			questions := func() []channel.Question {
				t.Helper()
				qs, unreadable := channel.WalkQuestions(b.install)
				if len(unreadable) > 0 {
					t.Fatal(unreadable)
				}
				return qs
			}
			count := func() int {
				t.Helper()
				history, err := readLines[Intervention](interventionsPath(b.install))
				if err != nil {
					t.Fatal(err)
				}
				n := 0
				for _, p := range history {
					if p.Row == "cannot-heal" {
						n++
					}
				}
				return n
			}
			now := bedNow
			if scenario == "returned" {
				now = now.Add(9 * time.Minute)
			}
			run := b.step(now)
			qs := questions()
			if scenario == "returned" || scenario == "person-proof" || scenario == "recent-act" || scenario == "young-step" || scenario == "person-stop" {
				if len(qs) != 0 || count() != 0 {
					t.Fatalf("bounded or returned goal asked: %+v run=%+v", qs, run)
				}
				b.seams.Now = func() time.Time { return now }
				status := ReadStatus(b.home, b.record, b.view, b.seams)
				if (scenario == "returned" || scenario == "recent-act") && healthRow(status.Health, "cannot-heal") != nil {
					t.Fatalf("ineligible row fired: %+v", status.Health)
				}
				return
			}
			if scenario != "silent" {
				b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
			}
			delay := 31 * time.Minute
			if scenario == "silent" {
				delay = 61 * time.Minute
			}
			run = b.step(now.Add(delay))
			qs = questions()
			words := "more than 120 minutes"
			tried := "tried: nothing: the row has no act of its own"
			if scenario == "silent" {
				words = "more than 80 minutes"
				tried = "tried: with nothing to do: settle the landing agent; keeper wake (end the silent landing agent session and wake the keeper) → keeper woken"
			}
			if len(qs) != 1 || count() != 1 || !strings.Contains(qs[0].Plain.Decision, words) || qs[0].Plain.IfNothing != "the lane keeps reporting it; this question is withdrawn when the situation ends" || qs[0].Facts[2] != tried {
				t.Fatalf("cannot-heal did not ask with evidence: %+v run=%+v count=%d", qs, run, count())
			}
			if qs[0].State != "open" {
				t.Fatalf("the step that asked withdrew its own question: %+v run=%+v", qs[0], run)
			}
			if scenario == "landed" || scenario == "old-question" {
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if len(args) > 2 && strings.Join(args[:3], " ") == "rev-list --parents --ignore-missing" {
						return "main tip", nil
					}
					return git(dir, args...)
				}
				minute := 90
				if scenario == "old-question" {
					minute = 25 * 60
				}
				run = b.step(bedNow.Add(time.Duration(minute) * time.Minute))
				closed, err := channel.ReadQuestion(b.install, qs[0].ID)
				if err != nil || closed.State != "closed" || !strings.Contains(run.Line, "the situation ended") || count() != 1 {
					t.Fatalf("ended situation kept asking: %+v run=%+v err=%v", closed, run, err)
				}
				return
			}
			again := 61 * time.Minute
			if scenario == "silent" {
				again = 121 * time.Minute
			}
			run = b.step(bedNow.Add(again))
			if count() != 1 || len(questions()) != 1 || questions()[0].State != "open" || !strings.Contains(run.Line, "bounded: asked once for this occurrence") {
				t.Fatalf("same situation asked twice or its question closed: %+v count=%d questions=%+v", run, count(), questions())
			}
			b.seams.Now = func() time.Time { return bedNow.Add(70 * time.Minute) }
			if _, err := channel.Withdraw(b.install, qs[0].ID, "person closed it at minute 70", nil, channel.DestinationConfig{}); err != nil {
				t.Fatal(err)
			}
			b.step(bedNow.Add(131 * time.Minute))
			if count() != 1 || len(questions()) != 1 || questions()[0].State != "closed" {
				t.Fatalf("person's closed question was asked again: count=%d questions=%+v", count(), questions())
			}
		})
	}
}

// routedQuestions observes the channel records and the full intervention log.
func routedQuestions(t *testing.T, b *stewardBed) ([]channel.Question, []Intervention) {
	t.Helper()
	qs, bad := channel.WalkQuestions(b.install)
	history, err := readLines[Intervention](interventionsPath(b.install))
	if len(bad) != 0 || err != nil {
		t.Fatalf("read route and questions: %v %v", bad, err)
	}
	return qs, history
}

func (b *stewardBed) askOccurrence(since, at time.Time) {
	b.t.Helper()
	d := humanauthority.Decision{At: at, Disposition: &humanauthority.Disposition{Row: "overdue-goal", Subject: "g", Occurrence: since.UTC().Format(time.RFC3339Nano), Kind: "ask"}}
	if err := humanauthority.AppendDecision(b.home, d); err != nil {
		b.t.Fatal(err)
	}
}

func TestANoActRowIsANoticeNotAQuestion(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)})
	b.step(bedNow)
	run := b.step(bedNow.Add(31 * time.Minute))
	qs, history := routedQuestions(t, b)
	if len(qs) != 0 || interventionCount(history, "cannot-heal") != 1 {
		t.Fatalf("no-act row asked: %+v history=%+v", qs, history)
	}
	notice := history[len(history)-1]
	if notice.How != "notice" || notice.Outcome != "noticed" || notice.Occurrence != bedNow.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("notice lost occurrence: %+v run=%+v", notice, run)
	}
	run = b.step(bedNow.Add(32 * time.Minute))
	if len(run.Interventions) != 0 || !strings.Contains(run.Line, "noticed once for this occurrence") {
		t.Fatalf("notice repeated: %+v", run)
	}
	b.askOccurrence(bedNow, bedNow.Add(33*time.Minute))
	b.step(bedNow.Add(34 * time.Minute))
	qs, _ = routedQuestions(t, b)
	if len(qs) != 1 || qs[0].State != "open" {
		t.Fatalf("explicit ask was suppressed: %+v", qs)
	}
}

func TestACannotHealRowIsRoutedBeforeItIsAsked(t *testing.T) {
	t.Parallel()
	for _, elsewhere := range []bool{false, true} {
		t.Run(fmt.Sprint(elsewhere), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)})
			calls := 0
			b.seams.Steward.Decisions = func(home string) ([]humanauthority.Decision, int, error) {
				calls++
				return humanauthority.ReadDecisions(home)
			}
			run := b.step(bedNow)
			qs, history := routedQuestions(t, b)
			id := bedNow.UTC().Format(time.RFC3339Nano)
			if len(qs) != 0 || len(history) != 1 || history[0].Row != "routed" || history[0].Subject != "overdue-goal g" || history[0].Occurrence != id || history[0].At != id || !strings.Contains(run.Line, "routed to its owner; asks at "+bedNow.Add(30*time.Minute).Local().Format("15:04")) {
				t.Fatalf("route before question: %+v %+v %s", qs, history, run.Line)
			}
			if elsewhere {
				d := humanauthority.Decision{At: bedNow.Add(5 * time.Minute), Disposition: &humanauthority.Disposition{Row: "other", Subject: "g", Occurrence: "elsewhere", Kind: "wait", WaitUntil: bedNow.Add(time.Hour)}}
				if err := humanauthority.AppendDecision(b.home, d); err != nil {
					t.Fatal(err)
				}
			}
			b.step(bedNow.Add(29 * time.Minute))
			if qs, _ := routedQuestions(t, b); len(qs) != 0 {
				t.Fatal("asked before thirty minutes")
			}
			b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
			b.step(bedNow.Add(31 * time.Minute))
			b.step(bedNow.Add(40 * time.Minute))
			qs, history = routedQuestions(t, b)
			var facts stewardQuestion
			if len(qs) != 1 || json.Unmarshal([]byte(strings.TrimPrefix(qs[0].Facts[1], "lane steward: ")), &facts) != nil || facts.Occurrence != id || facts.Row != "overdue-goal" || facts.Subject != "g" || calls != 4 {
				t.Fatalf("question lost occurrence or repeated reads: %+v %+v calls=%d", qs, history, calls)
			}
		})
	}
	t.Run("silent count is evidence", func(t *testing.T) {
		t.Parallel()
		b := newStewardBed(t)
		b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) {
			return lane.OwnerProbe{Alive: true, Since: bedNow.Add(-81 * time.Minute)}, nil
		}
		stamp := bedNow.Format(time.RFC3339Nano)
		b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp})
		b.step(bedNow)
		b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp}, Line{Goal: "h", SHA: "tip", At: stamp})
		b.step(bedNow.Add(5 * time.Minute))
		_, history := routedQuestions(t, b)
		routes := 0
		for _, entry := range history {
			if entry.Row == "cleared" {
				t.Fatalf("count change cleared silence: %+v", history)
			}
			if entry.Row == "routed" {
				routes++
				if entry.Subject != "silent-agent landing-agent" {
					t.Fatalf("unstable silent key: %+v", entry)
				}
			}
		}
		if routes != 1 {
			t.Fatalf("silence routed %d times: %+v", routes, history)
		}
	})
}

func TestADispositionDecidesTheQuestion(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"wait", "refused", "confirmed", "partial", "unchanged", "ask", "newest", "cleared", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			waiting := Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)}
			b.lines("queue.jsonl", waiting)
			b.step(bedNow)
			id := bedNow.UTC().Format(time.RFC3339Nano)
			if kind == "unreadable" {
				if err := os.MkdirAll(humanauthority.DecisionLogPath(b.home), 0700); err != nil {
					t.Fatal(err)
				}
			} else if kind != "cleared" {
				d := humanauthority.Decision{At: bedNow.Add(5 * time.Minute), Outcome: kind, Disposition: &humanauthority.Disposition{Row: "overdue-goal", Subject: "g", Occurrence: id, Kind: "act"}}
				if kind == "wait" || kind == "ask" {
					d.Disposition.Kind = kind
					d.Disposition.WaitUntil = bedNow.Add(60 * time.Minute)
				}
				if err := humanauthority.AppendDecision(b.home, d); err != nil {
					t.Fatal(err)
				}
				if kind == "newest" {
					older := d
					older.At, older.Outcome = bedNow.Add(time.Minute), "confirmed"
					if err := humanauthority.AppendDecision(b.home, older); err != nil {
						t.Fatal(err)
					}
				}
			}
			b.step(bedNow.Add(5 * time.Minute))
			want := 0
			if kind == "ask" {
				want = 1
			}
			if qs, _ := routedQuestions(t, b); len(qs) != want {
				t.Fatalf("%s at five minutes: %+v", kind, qs)
			}
			b.step(bedNow.Add(31 * time.Minute))
			if qs, _ := routedQuestions(t, b); len(qs) != want {
				t.Fatalf("%s at thirty-one minutes: %+v", kind, qs)
			}
			if kind == "wait" {
				b.step(bedNow.Add(61 * time.Minute))
				if qs, history := routedQuestions(t, b); len(qs) != 0 || interventionCount(history, "cannot-heal") != 1 {
					t.Fatal("expired wait did not produce one notice")
				}
			}
			if kind == "confirmed" || kind == "partial" || kind == "unchanged" {
				b.step(bedNow.Add(90 * time.Minute))
				if qs, _ := routedQuestions(t, b); len(qs) != 0 {
					t.Fatal("handled act asked")
				}
			}
			if kind == "cleared" {
				b.lines("queue.jsonl")
				b.step(bedNow.Add(40 * time.Minute))
				b.lines("queue.jsonl", waiting)
				b.step(bedNow.Add(50 * time.Minute))
				b.step(bedNow.Add(79 * time.Minute))
				qs, history := routedQuestions(t, b)
				routed, cleared := 0, 0
				for _, entry := range history {
					if entry.Row == "routed" {
						routed++
					}
					if entry.Row == "cleared" {
						cleared++
						if entry.Occurrence != id {
							t.Fatal("cleared wrong occurrence")
						}
					}
				}
				if len(qs) != 0 || routed != 2 || cleared != 1 {
					t.Fatalf("new occurrence was not routed afresh: %+v %+v", qs, history)
				}
				b.step(bedNow.Add(81 * time.Minute))
				if qs, history := routedQuestions(t, b); len(qs) != 0 || interventionCount(history, "cannot-heal") != 2 {
					t.Fatalf("new occurrence did not get its own notice: %+v %+v", qs, history)
				}
			}
		})
	}
}

// settleLaneAnswer starts at the channel's authenticated answer checkpoint;
// its production settlement path records and closes the goal-less answer.
func settleLaneAnswer(t *testing.T, b *stewardBed, q channel.Question, at time.Time) {
	t.Helper()
	q.Answer = &channel.Answer{Text: "keep waiting", Phase: "matched", At: at, Machine: "landing"}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "artifacts/agents/channel/questions", q.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.SettleReplies(context.Background(), channel.PollConfig{RepoRoot: b.install, Now: at}, func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("no ledger read needed") }); err != nil {
		t.Fatal(err)
	}
}

func interventionCount(history []Intervention, row string) (count int) {
	for _, p := range history {
		if p.Row == row {
			count++
		}
	}
	return count
}

func TestAnAnsweredOccurrenceNeverAsksAgain(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"steady", "restart", "silent"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			waiting := Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)}
			if scenario == "silent" {
				waiting.At = bedNow.Format(time.RFC3339Nano)
				b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) {
					return lane.OwnerProbe{Alive: true, Since: bedNow.Add(-81 * time.Minute)}, nil
				}
			}
			b.lines("queue.jsonl", waiting)
			b.step(bedNow)
			if scenario != "silent" {
				b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
			}
			b.step(bedNow.Add(31 * time.Minute))
			qs, _ := routedQuestions(t, b)
			if len(qs) != 1 {
				t.Fatalf("initial question: %+v", qs)
			}
			settleLaneAnswer(t, b, qs[0], bedNow.Add(time.Hour))
			if scenario == "silent" {
				b.lines("queue.jsonl", waiting, Line{Goal: "h", SHA: "tip", At: waiting.At})
			}
			if scenario != "restart" {
				b.step(bedNow.Add(61 * time.Minute))
			}
			minutesToStep := []int{151, 240, 600, 900, 1500, 1591}
			if scenario == "silent" {
				minutesToStep = []int{70}
			}
			for _, minutes := range minutesToStep {
				b.step(bedNow.Add(time.Duration(minutes) * time.Minute))
			}
			qs, history := routedQuestions(t, b)
			if len(qs) != 1 || interventionCount(history, "asked-closed") != 1 {
				t.Fatalf("answer rearmed or not reconciled: %+v %+v", qs, history)
			}
			for _, p := range history {
				if p.Row == "asked-closed" && (p.What != "keep waiting" || p.Occurrence != bedNow.UTC().Format(time.RFC3339Nano)) {
					t.Fatalf("answer lost: %+v", p)
				}
				if scenario == "silent" && p.Row == "cleared" {
					t.Fatalf("queue count cleared silence: %+v", history)
				}
			}
			if scenario == "silent" {
				return
			}
			b.lines("queue.jsonl")
			b.step(bedNow.Add(1600 * time.Minute))
			b.lines("queue.jsonl", waiting)
			b.step(bedNow.Add(1601 * time.Minute))
			b.askOccurrence(bedNow.Add(1601*time.Minute), bedNow.Add(1631*time.Minute))
			b.step(bedNow.Add(1632 * time.Minute))
			if qs, _ := routedQuestions(t, b); len(qs) != 2 {
				t.Fatalf("new occurrence not asked: %+v", qs)
			}
		})
	}
}

func TestAMachineWithdrawalRearmsTheFailSafe(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"wait", "act"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.seams.Steward.Route = func(Intervention) error { return nil }
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)})
			b.step(bedNow)
			b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
			b.step(bedNow.Add(31 * time.Minute))
			d := humanauthority.Decision{At: bedNow.Add(36 * time.Minute), Outcome: "confirmed", Disposition: &humanauthority.Disposition{Row: "overdue-goal", Subject: "g", Occurrence: bedNow.UTC().Format(time.RFC3339Nano), Kind: kind, WaitUntil: bedNow.Add(90 * time.Minute)}}
			if err := humanauthority.AppendDecision(b.home, d); err != nil {
				t.Fatal(err)
			}
			b.step(bedNow.Add(37 * time.Minute))
			qs, history := routedQuestions(t, b)
			if len(qs) != 1 || qs[0].State != "closed" || interventionCount(history, "withdrawn") != 1 || interventionCount(history, "asked-closed") != 0 {
				t.Fatalf("machine withdrawal: %+v %+v", qs, history)
			}
			if kind == "act" {
				d.At, d.Outcome = bedNow.Add(89*time.Minute), "refused"
				if err := humanauthority.AppendDecision(b.home, d); err != nil {
					t.Fatal(err)
				}
			}
			b.askOccurrence(bedNow, bedNow.Add(90*time.Minute))
			b.step(bedNow.Add(91 * time.Minute))
			qs, _ = routedQuestions(t, b)
			if len(qs) != 2 {
				t.Fatalf("machine withdrawal did not rearm: %+v", qs)
			}
			for _, q := range qs {
				if q.State == "open" {
					if _, err := channel.Withdraw(b.install, q.ID, "noise: the proof was running", nil, channel.DestinationConfig{}, bedNow.Add(92*time.Minute)); err != nil {
						t.Fatal(err)
					}
				}
			}
			b.step(bedNow.Add(93 * time.Minute))
			b.step(bedNow.Add(26 * time.Hour))
			qs, history = routedQuestions(t, b)
			if len(qs) != 2 || interventionCount(history, "asked-closed") != 1 || interventionCount(history, "judgment-withdrawn") != 1 {
				t.Fatalf("person withdrawal not final: %+v %+v", qs, history)
			}
			for _, p := range history {
				if p.Row == "asked-closed" || p.Row == "judgment-withdrawn" {
					if p.What != "noise: the proof was running" {
						t.Fatalf("withdrawal reason lost: %+v", p)
					}
				}
			}
		})
	}
}

func TestAWithdrawnAskRouteFailureDoesNotStopRows(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)})
	b.step(bedNow)
	b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
	b.step(bedNow.Add(31 * time.Minute))
	qs, _ := routedQuestions(t, b)
	if len(qs) != 1 {
		t.Fatalf("initial question: %+v", qs)
	}
	q := qs[0]
	if _, err := channel.Withdraw(b.install, q.ID, "noise: proof running", nil, channel.DestinationConfig{}, bedNow.Add(32*time.Minute)); err != nil {
		t.Fatal(err)
	}
	routes := 0
	b.seams.Steward.Route = func(Intervention) error {
		routes++
		return errors.New("board refuses the message")
	}
	b.incidents()
	before := len(b.calls)
	run, err := StewardStep(b.home, bedNow.Add(33*time.Minute), b.seams)
	if len(b.calls) != before+1 || b.calls[before] != "trunk" || interventionCount(run.Interventions, "unproven-incidents") != 1 {
		t.Fatalf("route failure stopped the firing row: %+v calls=%v err=%v", run, b.calls, err)
	}
	fact := "Steward question " + q.ID + " could not be routed: board refuses the message."
	if err != nil || routes != 1 || strings.Count(run.Line, fact) != 1 || interventionCount(run.Interventions, "judgment-route-failed") != 1 {
		t.Fatalf("route failure not named once: %+v routes=%d err=%v", run, routes, err)
	}
	run = b.step(bedNow.Add(34 * time.Minute))
	_, history := routedQuestions(t, b)
	if routes != 1 || strings.Contains(run.Line, fact) || interventionCount(run.Interventions, "judgment-route-failed") != 0 || interventionCount(history, "judgment-route-failed") != 1 || interventionCount(history, "asked-closed") != 1 || interventionCount(history, "judgment-withdrawn") != 0 {
		t.Fatalf("route failure repeated or closure lost: %+v history=%+v routes=%d", run, history, routes)
	}
	for _, p := range history {
		if p.Row == "judgment-route-failed" && (p.Outcome != q.ID || p.What != fact) {
			t.Fatalf("failure lost question or cause: %+v", p)
		}
	}
}

func TestAWithdrawnAskIsDesignInputOnce(t *testing.T) {
	t.Parallel()
	for _, declared := range []bool{false, true} {
		t.Run(fmt.Sprint(declared), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			registry, coordinator := filepath.Dir(b.home), t.TempDir()
			const ledger = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			if declared {
				if _, err := brain.Declare(brain.DeclareOptions{StateRoot: coordinator, RegistryHome: registry, LedgerIdentity: ledger, Machine: "coordinator", DeclaredBy: "Wido", Now: bedNow}); err != nil {
					t.Fatal(err)
				}
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "show" && args[1] == goal.AcceptedRef+":./plans/goals/backlog.md" {
					return string(goal.RenderRoot(&goal.RootRecord{Identity: ledger, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})), nil
				}
				return git(dir, args...)
			}
			waiting := Line{Goal: "g", SHA: "tip", At: bedNow.Add(-121 * time.Minute).Format(time.RFC3339Nano)}
			b.lines("queue.jsonl", waiting)
			b.step(bedNow)
			b.askOccurrence(bedNow, bedNow.Add(30*time.Minute))
			b.step(bedNow.Add(31 * time.Minute))
			qs, _ := routedQuestions(t, b)
			if len(qs) != 1 {
				t.Fatalf("initial question: %+v", qs)
			}
			if _, err := channel.Withdraw(b.install, qs[0].ID, "noise: proof running", nil, channel.DestinationConfig{}, bedNow.Add(32*time.Minute)); err != nil {
				t.Fatal(err)
			}
			b.step(bedNow.Add(33 * time.Minute))
			b.step(bedNow.Add(34 * time.Minute))
			messages, err := board.Threads(b.home)
			want := 0
			if declared {
				want = 1
			}
			_, history := routedQuestions(t, b)
			if err != nil || len(messages) != want || interventionCount(history, "judgment-withdrawn") != 1 {
				t.Fatalf("design input: %+v %v history=%+v", messages, err, history)
			}
			if declared {
				m := messages[0].Root
				if m.From.Machine != "landing" || m.To.Machine != "coordinator" || m.Kind != board.KindAsk || !strings.Contains(m.PeerText(), "overdue-goal g: noise: proof running") || !strings.Contains(m.PeerText(), "no act is asked") {
					t.Fatalf("wrong design route: %+v", m)
				}
				// A repeated publication after a lost local marker is the same ask.
				if err := stewardRoute(b.home, ledger, "landing", Intervention{Row: "overdue-goal", Subject: "g", Occurrence: bedNow.UTC().Format(time.RFC3339Nano), What: "changed reason"}, bedNow.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				if threads, _ := board.Threads(b.home); len(threads) != 1 {
					t.Fatal("duplicate design input")
				}
			}
			b.lines("queue.jsonl")
			b.step(bedNow.Add(35 * time.Minute))
			b.lines("queue.jsonl", waiting)
			b.step(bedNow.Add(36 * time.Minute))
			b.askOccurrence(bedNow.Add(36*time.Minute), bedNow.Add(66*time.Minute))
			b.step(bedNow.Add(67 * time.Minute))
			if qs, _ := routedQuestions(t, b); len(qs) != 2 {
				t.Fatalf("withdrawal suppressed new occurrence: %+v", qs)
			}
		})
	}
}
