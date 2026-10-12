package plain

import (
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func shadowHome(t *testing.T, checkout, install string) string {
	t.Helper()
	home := t.TempDir()
	ensureShadowConfig(t, install)
	shadowRegister(t, home, lane.Record{Root: checkout, Install: install, CustodyEpoch: 1})
	return home
}

func shadowRegister(t *testing.T, home string, record lane.Record) {
	t.Helper()
	if record.CustodyEpoch == 0 {
		record.CustodyEpoch = 1
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0600); err != nil {
		t.Fatal(err)
	}
}

// These answers describe a synthetic checkout with no landed submissions.
func shadowSeams() ProveSeams {
	notAncestor := exec.Command("/usr/bin/false").Run()
	return ProveSeams{Now: func() time.Time { return bedNow }, Machine: func(string) (string, error) { return "fixture", nil },
		Git: func(_ string, args ...string) (string, error) {
			if args[0] == "merge-base" {
				return "", notAncestor
			}
			if args[0] == "rev-parse" {
				if slices.Contains(args, "HEAD^{commit}") {
					return "head", nil
				}
				return "main", nil
			}
			return "", nil
		},
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }}
}

func assertShadowAgrees(t *testing.T, home string, seams ProveSeams) {
	t.Helper()
	record, exists, err := lane.Read(home)
	if err != nil || !exists {
		t.Fatalf("lane registration: %v %v", exists, err)
	}
	if _, err := RefreshShadow(home, seams.now(), "agreement", seams); err != nil {
		if _, installErr := os.Stat(record.Install); errors.Is(installErr, os.ErrNotExist) && errors.Is(err, os.ErrNotExist) {
			return
		}
		// Damaged records are scenarios too: a failed observation must preserve
		// the last shadow rather than invent an answer for the unreadable field.
		view := lane.View{Root: &record.Root, Owner: lane.OwnerView{State: lane.OwnerIdle}}
		readonly := seams
		readonly.ReadOnly = true
		observed, raw := observe(home, record, view, readonly, checkoutGit(record.Root, readonly))
		if len(observed.Problems) == 0 && raw.Err == nil {
			t.Fatalf("shadow refresh: %v", err)
		}
		return
	}
	shadow, exists, err := state.Read(Dir(record.Install))
	if err != nil || !exists {
		t.Fatalf("shadow record: %v %v", exists, err)
	}
	equal := func(field string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("shadow %s: got %#v; raw records %#v", field, got, want)
		}
	}
	captured, err := captureObservation(home, &record, seams, nil)
	if err != nil || !captured.Coherent {
		t.Fatalf("agreement observation: coherent=%v error=%v", captured.Coherent, err)
	}
	observation, raw := captured.Status, captured.Status.Raw
	pause, paused := lane.ReadPause(home)
	word := "running"
	if paused {
		word = "paused"
	}
	if pause.Unreadable() || observation.Stop != nil {
		word = "stopped"
	}
	equal("lane.word", shadow.Lane.Word, word)
	agent, err := lane.ReadAgentState(home)
	if err != nil {
		t.Fatal(err)
	}
	equal("agent.session", shadow.Agent.Session, agent.Launch)
	agentWord := "ended"
	if raw.AgentLive != nil && *raw.AgentLive {
		agentWord = "idle"
		if p := observation.AgentProgress; p != nil && p.Step != nil {
			agentWord = "stepping"
			if p.ProcessGone {
				agentWord = "step-process-gone"
			}
		}
	}
	equal("agent.word", shadow.Agent.Word, agentWord)
	step := state.Step{}
	if p := observation.AgentProgress; p != nil {
		if p.Step != nil {
			step = state.Step{Verb: p.Step.Verb, Args: append([]string(nil), p.Step.Args...), StartedAt: p.Step.StartedAt}
		}
	}
	equal("agent.step", shadow.Agent.Step, step)
	batch, err := ReadBatch(record.Install)
	if err != nil {
		t.Fatal(err)
	}
	if batch != nil {
		equal("batch.id", shadow.Batch.ID, batch.ID)
		equal("batch.execution.word", shadow.Batch.Execution.Word, batch.State)
		admission := "sealed"
		if batch.Open {
			admission = "open"
		}
		equal("batch.admission.word", shadow.Batch.Admission.Word, admission)
		var members []state.Member
		for _, m := range batch.Members {
			members = append(members, state.Member{Goal: m.Goal, SHA: m.SHA})
		}
		equal("batch.members", shadow.Batch.Members, members)
	} else {
		equal("batch.id", shadow.Batch.ID, "")
	}
	entries, err := Entries(record.Install)
	if err != nil {
		t.Fatal(err)
	}
	if len(shadow.HandIns) != len(entries) {
		t.Fatalf("hand_ins count: got %d; queue has %d", len(shadow.HandIns), len(entries))
	}
	equal("hand_ins observation count", len(shadow.HandIns), len(observation.Queue))
	for i, e := range observation.Queue {
		prefix := fmt.Sprintf("hand_ins[%d]", i)
		equal(prefix+".goal", shadow.HandIns[i].Goal, entries[i].Goal)
		equal(prefix+".sha", shadow.HandIns[i].SHA, entries[i].SHA)
		equal(prefix+".word", shadow.HandIns[i].Word, e.State)
		equal(prefix+".ordinal", shadow.HandIns[i].Ordinal, raw.Ordinals[i])
		equal(prefix+".reason", shadow.HandIns[i].Reason, e.Reason)
	}
	if running := ReadRunningProof(record.Install, seams); running != nil {
		equal("proof.attempt", shadow.Proof.Attempt, running.Attempt)
		equal("proof.word", shadow.Proof.Word, running.State)
	} else if observation.LastProof != nil {
		equal("proof.attempt", shadow.Proof.Attempt, observation.LastProof.Attempt)
		equal("proof.word", shadow.Proof.Word, observation.LastProof.Result)
	} else {
		equal("proof.attempt", shadow.Proof.Attempt, "")
	}
	main := "unproven"
	for _, r := range raw.Results {
		if r.Trunk && r.Commit == raw.Main && (r.Result == Green || r.Result == Red) {
			main = r.Result
		}
	}
	equal("main.word", shadow.Main.Word, main)
	var incidents []string
	for _, incident := range raw.Incidents {
		if incident.Closed == nil {
			incidents = append(incidents, incident.ID)
		}
	}
	equal("main.open_incidents", shadow.Main.OpenIncidents, incidents)
}

func TestShadowAgreementAgentWordsAndRecordedStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		word              string
		alive, step, gone bool
	}{
		{"ended", false, true, true},
		{"idle", true, false, false},
		{"stepping", true, true, false},
		{"step-process-gone", true, true, true},
	} {
		t.Run(tc.word, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			seams := shadowSeams()
			seams.AgentProbe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: tc.alive}, nil }
			seams.Alive = func(Running) bool { return !tc.gone }
			if tc.step {
				b.lines("agent-step.json", AgentStep{Verb: "prove", PID: 1, StartedAt: bedNow.Format(time.RFC3339Nano)})
			}
			b.lines("stops.jsonl", Stop{Loop: "lane-proof", Subject: "A", Decision: "stop", Handoff: "ask lane", At: bedNow.Format(time.RFC3339Nano)})
			assertShadowAgrees(t, b.home, seams)
			r, _, err := state.Read(Dir(b.install))
			if err != nil || r.Agent.Word != tc.word || r.Lane.Word != "stopped" {
				t.Fatalf("words: agent=%s lane=%s error=%v", r.Agent.Word, r.Lane.Word, err)
			}
		})
	}
}

func TestShadowAgreementLifecycleAndEverySince(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	shadowRegister(t, b.home, b.record)
	seams := shadowSeams()
	seams.Alive = func(Running) bool { return true }
	seams.AgentRunning = func() (bool, error) { return true, nil }
	read := func() state.Record {
		t.Helper()
		assertShadowAgrees(t, b.home, seams)
		r, _, err := state.Read(Dir(b.install))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	transition := func(field string, got state.Since, stamp string) {
		t.Helper()
		want, _ := time.Parse(time.RFC3339Nano, stamp)
		if got.Observed || !got.At.Equal(want) || got.Source != "" {
			t.Fatalf("%s transition since: %+v; want %s", field, got, stamp)
		}
	}
	observation := func(field string, got state.Since, source string) {
		t.Helper()
		if !got.Observed || got.Source != source || !got.At.Equal(bedNow) {
			t.Fatalf("%s observation since: %+v", field, got)
		}
	}
	if _, err := lane.SetPauseBecause(b.home, "person", "maintenance", bedNow); err != nil {
		t.Fatal(err)
	}
	r := read()
	transition("lane", r.Lane.Since, bedNow.Format(time.RFC3339))
	if _, err := lane.ClearPause(b.home); err != nil {
		t.Fatal(err)
	}
	r = read()
	observation("lane", r.Lane.Since, "landing-lane.json")
	at := bedNow.Add(-time.Minute).Format(time.RFC3339)
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "a", At: at})
	r = read()
	transition("hand-in", r.HandIns[0].Since, at)
	if err := writeAgentRecord(b.install, "agent-step.json", AgentStep{Verb: "prove", StartedAt: at, PID: 1}); err != nil {
		t.Fatal(err)
	}
	r = read()
	transition("agent", r.Agent.Since, at)
	registered := b.record
	registered.CustodyEpoch = 1
	batch := &Batch{ID: "batch", Lane: registered, Members: []GoalSHA{{Goal: "A", SHA: "a"}}, Base: "main", State: BatchPrepared, CreatedAt: at, Open: true}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	r = read()
	transition("batch.execution", r.Batch.Execution.Since, at)
	transition("batch.admission", r.Batch.Admission.Since, at)
	batch.State = BatchRunning
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	r = read()
	observation("batch.execution", r.Batch.Execution.Since, "batch.json")
	if r.Batch.Admission.Word != "open" {
		t.Fatal(r.Batch)
	}
	batch.Open, batch.SealedAt = false, bedNow.Format(time.RFC3339)
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	r = read()
	transition("batch.admission", r.Batch.Admission.Since, batch.SealedAt)
	if r.Batch.Execution.Word != "running" {
		t.Fatal(r.Batch)
	}
	if err := writeRunning(b.install, Running{Attempt: "proof", Commit: "head", Since: at, Pid: 1}); err != nil {
		t.Fatal(err)
	}
	r = read()
	transition("proof", r.Proof.Since, at)
	if err := os.Remove(runningPath(b.install)); err != nil {
		t.Fatal(err)
	}
	b.lines("results.jsonl", Result{Attempt: "proof", Result: Green, Commit: "main", Trunk: true, At: at})
	r = read()
	transition("proof", r.Proof.Since, at)
	transition("main", r.Main.Since, at)
	b.lines("pushes.jsonl", Pushed{Commit: "main", Old: "before", At: bedNow.Format(time.RFC3339)})
	seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "rev-list" {
			return "a", nil
		}
		if args[0] == "rev-parse" {
			return "main", nil
		}
		return "", nil
	}
	r = read()
	transition("main", r.Main.Since, bedNow.Format(time.RFC3339))
	transition("landed hand-in", r.HandIns[0].Since, bedNow.Format(time.RFC3339))
	observation("holds", r.Holds.Since, "landing-agent-keeper.json")
}

func TestShadowMembershipDriftAndRebuildEvidence(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	shadowRegister(t, b.home, b.record)
	registered := b.record
	registered.CustodyEpoch = 1
	batch := &Batch{ID: "batch", Lane: registered, Base: "main", State: BatchRunning, Selector: PolicyValue{Value: "auto"}, Open: true, Members: []GoalSHA{{Goal: "A", SHA: "a"}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "a", At: bedNow.Format(time.RFC3339)}, Line{Goal: "B", SHA: "b", At: bedNow.Format(time.RFC3339)})
	if _, err := RefreshShadow(b.home, bedNow, "keeper", seams); err != nil {
		t.Fatal(err)
	}
	before, _, _ := state.Read(Dir(b.install))
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "a", At: bedNow.Format(time.RFC3339)}, Line{Goal: "B", SHA: "b", At: bedNow.Format(time.RFC3339)})
	seams.GoalTier = func(string, string) (uint8, error) { return 1, nil }
	if err := withLock(b.install, func() error { return extendBatchLocked(b.install, b.checkout, batch, seams) }); err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshShadow(b.home, bedNow, "keeper", seams)
	if err != nil {
		t.Fatal(err)
	}
	after, _, _ := state.Read(Dir(b.install))
	want := "batch.members: [{A a}] → [{A a} {B b}]"
	if !slices.Equal(changed, []string{want}) || !slices.Equal(state.Diff(before, after), changed) {
		t.Fatalf("membership drift: %q", changed)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var event state.Event
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	if !event.Rebuild || event.Evidence != want {
		t.Fatalf("rebuild: %+v", event)
	}
}

func TestShadowRefreshReadOnlyInsideInstallLock(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	home := shadowHome(t, b.checkout, b.install)
	fix := &Fix{Attempt: "active", Round: 1, Units: []string{"conflict"}, Paths: []conflict.Path{{Path: "A"}}, Commit: b.originMain(), Goal: "A", State: "resolving", Job: "not-launched", Parent: b.originMain(), Tip: "missing"}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- withLock(b.install, func() error { _, err := RefreshShadow(home, bedNow, "fixture", ProveSeams{}); return err })
	}()
	var result error
	testenv.Await(t, "shadow refresh while the installation lock is held", func() bool {
		select {
		case result = <-done:
			return true
		default:
			return false
		}
	})
	if result != nil {
		t.Fatal(result)
	}

	after, err := os.ReadFile(path)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("read-only refresh changed fix: %v", err)
	}
}

func TestShadowReadPrecedesRawObservationAndDoesNotReenter(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	shadowRegister(t, b.home, b.record)
	sentinel := state.Record{Lane: state.Lane{Fact: state.Fact{Word: "running"}}}
	if _, err := state.Replace(Dir(b.install), bedNow, sentinel, "fixture", 0, state.Fingerprint{Current: func() string { return "" }}); err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	first := true
	observations := 0
	seams.Observed = func() {
		observations++
		if first {
			first = false
			if err := os.Remove(filepath.Join(Dir(b.install), "state.json")); err != nil {
				t.Fatal(err)
			}
		}
	}

	git := b.git(nil, nil)
	status := readStatus(b.home, b.record, b.view, seams, git)
	if observations != 1 || status.Shadow == nil || status.Shadow.Version != 1 || status.Health == nil || status.Health.Shadow != status.Shadow {
		t.Fatalf("shadow was not read first: %+v", status)
	}
	// One pause read and one projection remain possible while the host lock
	// is held; the refresh callback is entered before the keeper takes it.
	refreshes := 0
	seams.Now = func() time.Time { return bedNow }
	keeper := lane.AgentKeeper{Home: b.home, Self: b.checkout, Now: seams.Now, Running: func() (string, bool, error) { return "", false, nil },
		Refresh: func(now time.Time) {
			refreshes++
			held, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
			if err != nil {
				t.Fatal("refresh ran under host lock:", err)
			}
			held.Release()
			if _, err := RefreshShadow(b.home, now, "keeper", seams); err != nil {
				t.Fatal(err)
			}
		}}
	keeper.Run()
	if refreshes != 1 || observations != 2 {
		t.Fatalf("refreshes=%d observations=%d; want one per call", refreshes, observations)
	}
}

func TestShadowKeeperEveryOutcomeAndHandWrittenPause(t *testing.T) {
	t.Parallel()
	for _, outcome := range []lane.AgentOutcome{lane.AgentStarted, lane.AgentHeld, lane.AgentIdle, lane.AgentPaused} {
		t.Run(string(outcome), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			shadowRegister(t, b.home, b.record)
			seams := shadowSeams()
			keeper := lane.AgentKeeper{Home: b.home, Self: b.checkout, Now: seams.Now, Running: func() (string, bool, error) { return "", false, nil }, Start: func(string, lane.Wake) (string, error) { return "session", nil },
				Refresh: func(now time.Time) {
					if _, err := RefreshShadow(b.home, now, "keeper", seams); err != nil {
						t.Fatal(err)
					}
				}}
			if outcome == lane.AgentHeld {
				keeper.Holds = []func(string) (string, error){func(string) (string, error) { return "held", nil }}
			}
			if outcome == lane.AgentStarted || outcome == lane.AgentHeld {
				keeper.Sources.Reasons = func(string) ([]string, error) { return []string{"A waits"}, nil }
			}
			if outcome == lane.AgentPaused {
				if _, err := lane.SetPause(b.home, "person", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			if run := keeper.Run(); run.Outcome != outcome {
				t.Fatalf("tick: %+v; want %s", run, outcome)
			}
			if _, exists, err := state.Read(Dir(b.install)); err != nil || !exists {
				t.Fatalf("tick failed to write shadow: %v %v", exists, err)
			}
			data, err := os.ReadFile(filepath.Join(Dir(b.install), "events.jsonl"))
			if err != nil || strings.Count(string(data), "\n") != 1 {
				t.Fatalf("first tick events: %s %v", data, err)
			}
			pause := lane.Pause{By: "handwritten", At: bedNow.Add(time.Minute).Format(time.RFC3339)}
			encoded, _ := json.Marshal(pause)
			if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			keeper.Run()
			r, _, _ := state.Read(Dir(b.install))
			if r.Lane.Word != "paused" || r.Lane.By != "handwritten" {
				t.Fatal(r.Lane)
			}
			data, err = os.ReadFile(filepath.Join(Dir(b.install), "events.jsonl"))
			if err != nil || strings.Count(string(data), "\n") != 2 {
				t.Fatalf("second tick events: %s %v", data, err)
			}
		})
	}
}

func TestShadowRefreshFailureIsFindingAndKeepsRecord(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	shadowRegister(t, b.home, b.record)
	seams := shadowSeams()
	if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err != nil {
		t.Fatal(err)
	}
	before, _, _ := state.Read(Dir(b.install))
	seams.Git = func(string, ...string) (string, error) { return "", errors.New("raw reader failed") }
	if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err == nil {
		t.Fatal("refresh hid a read failure")
	}
	after, _, _ := state.Read(Dir(b.install))
	if len(state.Diff(before, after)) != 0 || after.LastObservation == nil {
		t.Fatal("failed refresh replaced shadow or omitted failure metadata")
	}
	status := readStatus(b.home, b.record, b.view, shadowSeams(), b.git(nil, nil))
	if !slices.ContainsFunc(status.Findings, func(line string) bool { return strings.Contains(line, "shadow refresh failed") }) {
		t.Fatalf("finding missing: %q", status.Findings)
	}
}

func TestShadowQueueOrdinalsTrackPhysicalLines(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if err := os.MkdirAll(Dir(b.install), 0700); err != nil {
		t.Fatal(err)
	}
	queue := "\n\n" + `{"goal":"A","sha":"a","at":"2026-10-01T21:00:00Z"}` + "\n" +
		`{"goal":"A","sha":"a","at":"2026-10-01T21:01:00Z","outcome":"returned","reason":"retry"}` + "\n\n" +
		`{"goal":"A","sha":"a","at":"2026-10-01T21:02:00Z"}` + "\n"
	if err := os.WriteFile(queuePath(b.install), []byte(queue), 0600); err != nil {
		t.Fatal(err)
	}
	assertShadowAgrees(t, b.home, shadowSeams())
	r, _, err := state.Read(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.HandIns) != 2 || r.HandIns[0].Ordinal != 3 || r.HandIns[1].Ordinal != 6 || r.HandIns[0].Word != StateReturned || r.HandIns[1].Word != StateWaiting {
		t.Fatalf("submission identities: %+v", r.HandIns)
	}
}

func TestShadowRefreshNeverCallsMergeWriter(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Attempt: "resolution", Goal: "A", State: "resolving", Commit: "head", Tip: "head", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "A"}}}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	writes, observations := 0, 0
	seams.RefreshMerge = func(string, string, *Fix, ProveSeams) error { writes++; return errors.New("merge writer called") }
	seams.Observed = func() { observations++ }
	if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err != nil {
		t.Fatal(err)
	}
	if writes != 0 || observations != 1 {
		t.Fatalf("merge writes=%d observations=%d; want 0 and 1", writes, observations)
	}
	after, err := os.ReadFile(path)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("resolution changed: %v", err)
	}
}

func TestShadowOfProjectsRecordedPayloadsAndHonestClocks(t *testing.T) {
	t.Parallel()
	root := "an observation with no filesystem"
	started := bedNow.Add(-time.Minute)
	stamp := started.Format(time.RFC3339Nano)
	observation := Status{View: lane.View{Root: &root}, AgentAlive: true,
		AgentProgress: &AgentProgress{Step: &AgentStep{Verb: "prove", Args: []string{"--gate"}, StartedAt: stamp}},
		Queue:         []Entry{{Goal: "A", SHA: "a", State: StateReturned, At: stamp, ReturnedAt: stamp, Reason: "own failure"}, {Goal: "A", SHA: "a", State: StateLanded, At: stamp}},
		RunningProof:  &RunningProof{Attempt: "proof", State: "running", Since: stamp}}
	raw := RawFacts{Now: bedNow, Agent: lane.AgentState{Launch: "session", Barren: 2}, Ordinals: []int{1, 3}, Main: "external", Engine: "recorded-engine",
		Batch:   &Batch{ID: "batch", State: BatchClosed, Open: false, SealedAt: stamp, Base: "base", Members: []GoalSHA{{Goal: "A", SHA: "a"}}},
		Running: Running{Gate: true}, Fix: Fix{Attempt: "resolution", Goal: "A", Job: "job", State: "resolving"},
		Incidents: []goal.TrunkRedEntry{{ID: "incident"}}, Questions: []state.Question{{ID: "question", Premise: "current", Since: state.Since{At: started}}},
		Stops: []Stop{{Loop: "lane-return", Subject: "lane", Attempt: 2, At: stamp}}}
	got := ShadowOf(observation, raw)
	if got.Agent.Session != "session" || got.Agent.Step.Verb != "prove" || !slices.Equal(got.Agent.Step.Args, []string{"--gate"}) || got.Agent.Since.Observed || !got.Agent.Since.At.Equal(started) {
		t.Fatalf("agent: %+v", got.Agent)
	}
	if got.Batch.Execution.Word != BatchClosed || !got.Batch.Execution.Since.Observed || got.Batch.Execution.Since.Source != "batch.json" || got.Batch.Admission.Word != "sealed" || got.Batch.Admission.Since.Observed || !got.Batch.Admission.Since.At.Equal(started) || got.Batch.Base != "base" {
		t.Fatalf("batch: %+v", got.Batch)
	}
	if got.HandIns[0].Reason != "own failure" || got.HandIns[0].Since.Observed || !got.HandIns[0].At.Equal(started) || !got.HandIns[1].Since.Observed || got.HandIns[1].Since.Source != "queue.jsonl" {
		t.Fatalf("hand-ins: %+v", got.HandIns)
	}
	if got.Resolution.Goal != "A" || got.Resolution.Job != "job" || !got.Resolution.Since.Observed || got.Resolution.Since.Source != "fixes/resolution.json" {
		t.Fatalf("resolution: %+v", got.Resolution)
	}
	if got.Proof.Attempt != "proof" || got.Proof.Scope != "gate" || got.Proof.Since.Observed || !got.Proof.Since.At.Equal(started) || got.Proof.LoadAtStart != nil {
		t.Fatalf("proof: %+v", got.Proof)
	}
	if got.Main.Word != "unproven" || got.Main.Commit != "external" || !got.Main.Since.Observed || got.Main.Since.Source != "refs/remotes/origin/main" || !slices.Equal(got.Main.OpenIncidents, []string{"incident"}) {
		t.Fatalf("main: %+v", got.Main)
	}
	if got.Holds.Barren != 2 || got.Holds.Since.Observed || !got.Holds.Since.At.Equal(started) || !reflect.DeepEqual(got.Questions, raw.Questions) || got.Engine.Stamp != raw.Engine {
		t.Fatalf("payloads: %+v", got)
	}
	raw.Previous, raw.Now = &got, bedNow.Add(time.Minute)
	if changed := state.Diff(got, ShadowOf(observation, raw)); len(changed) != 0 {
		t.Fatalf("unchanged observations moved first-seen times: %q", changed)
	}
	observation.RunningProof.State = "died"
	dead := ShadowOf(observation, raw)
	if !dead.Proof.Since.Observed || dead.Proof.Since.Source != "running.json" || !dead.Proof.Since.At.Equal(raw.Now) {
		t.Fatalf("death borrowed the proof start: %+v", dead.Proof)
	}
	observation.RunningProof = nil
	observation.LastProof = &Result{Attempt: "finished", Result: Red, Scope: "full", At: stamp, LoadStart: &hostload.Sample{Available: true, Load1m: 7, Cores: 8}}
	finished := ShadowOf(observation, raw)
	if finished.Proof.LoadAtStart == nil || finished.Proof.LoadAtStart.Load1m != 7 || finished.Proof.Word != Red || finished.Proof.Since.Observed {
		t.Fatalf("finished proof: %+v", finished.Proof)
	}
}

func TestShadowObservationClocksUseUTC(t *testing.T) {
	t.Parallel()
	root := "fixture"
	now := bedNow.In(time.FixedZone("fixture", 2*60*60))
	shadow := ShadowOf(Status{View: lane.View{Root: &root}}, RawFacts{Now: now})
	if !shadow.Lane.Since.At.Equal(now) || shadow.Lane.Since.At.Location() != time.UTC {
		t.Fatalf("observation clock changed instant or retained local zone: %+v", shadow.Lane.Since)
	}
}

func ensureShadowConfig(t *testing.T, install string) {
	t.Helper()
	if info, err := os.Stat(install); err == nil && info.IsDir() {
		path := filepath.Join(install, "metasystem.conf")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, []byte("metasystem.template=true\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
