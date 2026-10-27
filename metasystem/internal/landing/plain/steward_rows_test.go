package plain

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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

func contentionStewardBed(t *testing.T, kind string) (*stewardBed, string) {
	t.Helper()
	b := newStewardBed(t)
	b.incidents()
	b.seams.Steward.Trunk = nil
	b.seams.Trunk, b.seams.Fresh = true, true
	b.seams.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.seams.NewID = func() string { return "contention-proof" }
	b.seams.Lane = func() (lane.Record, error) { return b.record, nil }
	b.seams.CommandForCommit = func(string) (string, error) { return "the committed proof command", nil }
	b.seams.Alive = func(Running) bool { return len(b.calls) > 0 }
	b.seams.Launch = func(_ []string, _, log string) (int64, error) {
		if err := os.WriteFile(log, nil, 0600); err != nil {
			return 0, err
		}
		b.calls = append(b.calls, "launch")
		return int64(os.Getpid()), nil
	}
	root := t.TempDir()
	if kind == "missing" {
		root = filepath.Join(root, "missing")
	} else {
		if err := (launch.Store{Root: root}).Create(launch.Record{ID: "host-work", Kind: kind, State: launch.Running}); err != nil {
			t.Fatal(err)
		}
	}
	b.seams.HostWork = (&launch.Manager{Store: launch.Store{Root: root}}).HostWork
	return b, filepath.Join(root, "host-work", "record.json")
}

func TestLandingProofStartHeldBesideBuilds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"build", "proof", "missing", "policy", "person"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			storeKind := kind
			if kind == "policy" || kind == "person" {
				storeKind = "build"
			}
			b, path := contentionStewardBed(t, storeKind)
			before, err := os.ReadFile(path)
			if err != nil && kind != "missing" {
				t.Fatal(err)
			}
			history := func() []Intervention {
				t.Helper()
				all, err := readLines[Intervention](interventionsPath(b.install))
				if err != nil {
					t.Fatal(err)
				}
				var holds []Intervention
				for _, p := range all {
					if p.Row == "proof-beside-builds" {
						holds = append(holds, p)
					}
				}
				return holds
			}
			read := func(now time.Time) *Health {
				b.seams.Now = func() time.Time { return now }
				status := ReadStatus(b.home, b.record, b.view, b.seams)
				if status.Health == nil || len(status.Problems) > 0 {
					t.Fatalf("status problems=%v running=%+v health=%+v", status.Problems, status.RunningProof, status.Health)
				}
				return status.Health
			}
			if kind == "person" {
				b.seams.Trunk = true
				b.seams.Person = &ActProvenance{Kind: "proof", Person: "Wido", CheckedAt: bedNow}
				if _, _, err := Start(b.install, b.checkout, b.seams); err != nil || len(b.calls) != 1 || len(history()) != 0 {
					t.Fatalf("person held: calls=%v history=%+v err=%v", b.calls, history(), err)
				}
				return
			}
			run := b.step(bedNow)
			records := history()
			if len(b.calls) != 0 || len(records) != 1 || len(run.Interventions) != 1 || !strings.HasPrefix(run.Interventions[0].Outcome, "held:") || !strings.HasPrefix(records[0].Outcome, "held: ") || !strings.Contains(records[0].Outcome, "load 2") {
				t.Fatalf("start not held once: calls=%v records=%+v run=%+v", b.calls, records, run)
			}
			if kind == "missing" && !strings.Contains(records[0].Outcome, "cannot be read") {
				t.Fatalf("lost store read as quiet: %+v", records)
			}
			end := bedNow.Add(30 * time.Minute)
			hold, err := KeeperContentionHold(b.install, b.seams)
			if err != nil || !strings.Contains(hold, end.Local().Format("15:04 MST")) {
				t.Fatalf("keeper missed hold end: %q %v", hold, err)
			}
			if _, err := KeeperClock(b.record, func(lane.Record) error { return nil }, b.seams); err != nil {
				t.Fatal(err)
			}
			clock, err := readClock(b.install)
			if err != nil || !clock.RetryAt.Equal(end) {
				t.Fatalf("clock retry=%v want=%v err=%v", clock.RetryAt, end, err)
			}
			run = b.step(bedNow.Add(10 * time.Minute))
			if len(run.Interventions) != 0 || len(history()) != 1 || len(b.calls) != 0 || !strings.Contains(run.Line, "bounded: the proof start is held beside builds until "+end.Local().Format("15:04 MST")) || healthRow(read(bedNow.Add(10*time.Minute)), "proof-beside-builds") == nil {
				t.Fatalf("held tick acted or lost row: %+v history=%+v calls=%v", run, history(), b.calls)
			}
			if kind == "policy" {
				b.seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: "person"}, nil }
			}
			now := bedNow.Add(31 * time.Minute)
			run, err = StewardStep(b.home, now, b.seams)
			records = history()
			if len(records) != 2 || !strings.HasPrefix(records[1].Outcome, "released after the 30-minute hold") {
				t.Fatalf("hold never released once: %+v run=%+v err=%v", records, run, err)
			}
			row := healthRow(read(now), "proof-beside-builds")
			if kind == "policy" {
				if err == nil || len(b.calls) != 0 || row == nil || !strings.Contains(row.Act, "no proof has started since") {
					t.Fatalf("release claimed a refused start: row=%+v calls=%v err=%v", row, b.calls, err)
				}
				b.seams.Now = func() time.Time { return now.Add(time.Minute) }
				_, _, err = Start(b.install, b.checkout, b.seams)
				if err == nil || len(history()) != 2 {
					t.Fatalf("retry duplicated release: %+v err=%v", history(), err)
				}
			} else {
				running, recorded, _, readErr := ReadRunning(b.install, b.seams)
				if err != nil || readErr != nil || !recorded || len(b.calls) != 1 || running.Since != now.Format(time.RFC3339Nano) || row == nil || row.Subject != running.Attempt || strings.Contains(row.Act, "held:") || strings.Contains(row.Act, "no proof has started") {
					t.Fatalf("release did not start/report proof: running=%+v row=%+v calls=%v err=%v/%v", running, row, b.calls, err, readErr)
				}
				if storeKind != "missing" && !strings.Contains(row.Act, "reported, not held") || storeKind == "missing" && !strings.Contains(row.Act, "unmeasured") {
					t.Fatalf("running work not reported: %+v", row)
				}
				if hold, err := KeeperContentionHold(b.install, b.seams); err != nil || hold != "" {
					t.Fatalf("released keeper still held: %q %v", hold, err)
				}
			}
			after, err := os.ReadFile(path)
			if kind != "missing" && (err != nil || string(before) != string(after)) {
				t.Fatalf("host work records changed: %v", err)
			}
		})
	}
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
			words := "more than 120 minutes"
			tried := "tried: nothing: the row has no act of its own"
			if scenario == "silent" {
				words = "more than 80 minutes"
				tried = "tried: with nothing to do: settle the landing agent; keeper wake (end the silent landing agent session and wake the keeper) → keeper woken"
			}
			if len(qs) != 1 || count() != 1 || !strings.Contains(qs[0].Plain.Decision, words) || qs[0].Plain.IfNothing != "the lane keeps reporting it; this question is withdrawn when the situation ends" || qs[0].Facts[2] != tried {
				t.Fatalf("cannot-heal did not ask with evidence: %+v run=%+v count=%d", qs, run, count())
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
			run = b.step(bedNow.Add(61 * time.Minute))
			if count() != 1 || len(questions()) != 1 || !strings.Contains(run.Line, "bounded: asked once for this situation") {
				t.Fatalf("same situation asked twice: %+v count=%d questions=%+v", run, count(), questions())
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
