package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestObservationRefreshClearsRecoveredFailure(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	seams := shadowSeams()
	if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err != nil {
		t.Fatal(err)
	}
	if err := state.NoteObservation(Dir(b.install), bedNow, []string{"queue unavailable"}); err != nil {
		t.Fatal(err)
	}
	before, _, err := state.Read(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshShadow(b.home, bedNow, "fixture", seams)
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := state.Read(Dir(b.install))
	events := shadowEvents(t, b.install)
	if err != nil || len(changed) != 0 || after.LastObservation != nil || after.Version != before.Version+1 || len(events) != 3 || events[2].Kind != "observation-recovered" || events[2].Rebuild {
		t.Fatalf("recovery: before=%+v after=%+v changed=%v events=%+v error=%v", before, after, changed, events, err)
	}
}

func TestObservationFingerprintUsesOnlyLaneInputs(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	landing := filepath.Join(b.home, "launch", "landing-session", "record.json")
	unrelated := filepath.Join(b.home, "launch", "build-session", "record.json")
	write(landing, `{"kind":"landing","state":"running"}`)
	write(unrelated, `{"kind":"unit","state":"running"}`)
	write(filepath.Join(b.install, "artifacts", "agents", "channel", "questions", "Q.json"), `{"id":"Q","state":"closed"}`)
	engine := filepath.Join(b.install, "bin", "metasystem")
	write(engine, "padding"+enginebuild.StampRecord("fixture-engine"))
	inputs := shadowInputs(b.home)
	if !slices.Contains(inputs, landing) {
		t.Fatal("landing launch is missing", inputs)
	}
	for _, path := range inputs {
		inLane := strings.HasPrefix(path, Dir(b.install)+string(filepath.Separator))
		hostRecord := filepath.Dir(path) == lane.HostDir(b.home) && strings.HasPrefix(filepath.Base(path), "landing-")
		if !inLane && !hostRecord && path != landing {
			t.Fatalf("unrelated input: %s", path)
		}
	}
	before := recordsFingerprint(b.home)
	write(engine, "different padding"+enginebuild.StampRecord("fixture-engine"))
	if got := recordsFingerprint(b.home); got != before {
		t.Fatal("engine bytes outside the stamp changed the fingerprint")
	}
	write(engine, enginebuild.StampRecord("new-engine"))
	if got := recordsFingerprint(b.home); got == before {
		t.Fatal("engine stamp is absent from the fingerprint")
	}
	seams := shadowSeams()
	calls := 0
	seams.Observed = func() {
		calls++
		write(unrelated, `{"kind":"unit","state":"finished"}`)
		write(filepath.Join(b.home, "launch", "new-build-session", "record.json"), `{"kind":"unit","state":"running"}`)
	}
	observation, err := captureObservation(b.home, &b.record, seams, nil)
	if err != nil || !observation.Coherent || calls != 1 {
		t.Fatalf("unrelated launch disturbed read: %+v %v calls=%d", observation, err, calls)
	}
	before = recordsFingerprint(b.home)
	write(landing, `{"kind":"landing","state":"finished"}`)
	if recordsFingerprint(b.home) == before {
		t.Fatal("landing launch change was ignored")
	}
}

func TestObservationRetakesUnreadableShadowBytes(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if err := os.MkdirAll(Dir(b.install), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(b.install), "state.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	calls := 0
	seams.Observed = func() {
		calls++
		if calls == 1 {
			if err := os.WriteFile(path, []byte(`{different broken json`), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	status := ObserveStatus(b.home, b.record, seams, nil)
	if calls != 2 || status.Health == nil || status.Shadow != nil || len(status.Problems) != 0 || !strings.Contains(strings.Join(status.Findings, ";"), "the next refresh replaces it") {
		t.Fatalf("unreadable bytes lost their identity: calls=%d status=%+v", calls, status)
	}
}

func shadowEvents(t *testing.T, install string) []state.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(install), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var events []state.Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e state.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	return events
}

func TestObservationExternalLandingKeepsFirstSeenTime(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "sha", At: bedNow.Format(time.RFC3339)})
	seams := shadowSeams()
	seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "rev-parse" {
			return "tip", nil
		}
		return "", nil
	}
	for _, now := range []time.Time{bedNow, bedNow.Add(10 * time.Minute)} {
		if _, err := RefreshShadow(b.home, now, "keeper", seams); err != nil {
			t.Fatal(err)
		}
	}
	got, _, _ := state.Read(Dir(b.install))
	if len(got.HandIns) != 1 || got.HandIns[0].Word != StateLanded || !got.HandIns[0].Since.Observed || !got.HandIns[0].Since.At.Equal(bedNow) || len(shadowEvents(t, b.install)) != 1 {
		t.Fatalf("landing drifted: %+v", got)
	}
}

func TestObservationFailedQueueAndQuestionKeepCompleteShadow(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"queue", "question"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			b.lines("queue.jsonl", Line{Goal: "A", SHA: "sha", At: bedNow.Format(time.RFC3339)})
			path := filepath.Join(b.install, "artifacts", "agents", "channel", "questions", "Q.json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			q := channel.Question{ID: "Q", About: "lane", State: "open", Machine: "fixture", OpenedAt: bedNow}
			data, _ := json.Marshal(q)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			seams := shadowSeams()
			if _, err := RefreshShadow(b.home, bedNow, "keeper", seams); err != nil {
				t.Fatal(err)
			}
			before, _, _ := state.Read(Dir(b.install))
			if name == "queue" {
				path = queuePath(b.install)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := RefreshShadow(b.home, bedNow.Add(10*time.Minute), "keeper", seams); err == nil {
				t.Fatal("failed read accepted")
			}
			got, _, _ := state.Read(Dir(b.install))
			if len(state.Diff(before, got)) != 0 || got.LastObservation == nil || !strings.Contains(strings.Join(got.LastObservation.Problems, ";"), path) {
				t.Fatalf("shadow replaced or error hidden: %+v", got)
			}
			events := shadowEvents(t, b.install)
			if len(events) != 2 || events[1].Kind != "observation-failed" {
				t.Fatal(events)
			}
			status := ObserveStatus(b.home, b.record, seams, nil)
			if status.Health == nil || healthRow(status.Health, "observation-failed") == nil || healthRow(status.Health, "drift") != nil || !strings.Contains(status.Health.Text(), path) || !strings.Contains(strings.Join(status.Problems, ";"), path) {
				t.Fatalf("health hid error: %+v", status)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestObservationRetakesPauseAndReferenceChanges(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"pause", "origin", "merge", "branch", "quiet"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			if _, err := lane.SetPause(b.home, "fixture", bedNow); err != nil {
				t.Fatal(err)
			}
			if change == "branch" {
				_, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Kind: "other", Machine: "fixture", Lineage: lane.AgentLineage, Facts: []string{"The checkout is not on main."}, Now: bedNow})
				if err != nil {
					t.Fatal(err)
				}
			}
			seam := shadowSeams()
			base := seam.Git
			tip, branch, merge := "old", "topic", "merge"
			seam.Git = func(dir string, args ...string) (string, error) {
				switch {
				case args[0] == "symbolic-ref":
					return branch, nil
				case slices.Contains(args, "refs/remotes/origin/main^{commit}"):
					return tip, nil
				case slices.Contains(args, "MERGE_HEAD^{commit}"):
					return merge, nil
				}
				return base(dir, args...)
			}
			calls := 0
			seam.Observed = func() {
				calls++
				if calls != 1 {
					return
				}
				switch change {
				case "pause":
					_, _ = lane.ClearPause(b.home)
				case "origin":
					tip = "new"
				case "merge":
					merge = ""
				case "branch":
					branch = "main"
				}
			}
			status := ObserveStatus(b.home, b.record, seam, nil)
			want := 2
			if change == "quiet" {
				want = 1
			}
			if calls != want || len(status.Problems) != 0 || status.Health == nil {
				t.Fatalf("calls=%d status=%+v", calls, status)
			}
			if change == "pause" && (status.Paused || status.Owner.State == lane.OwnerStopped) {
				t.Fatalf("mixed pause facts: %+v", status)
			}
			if change == "branch" && (len(status.Raw.Questions) != 1 || status.Raw.Questions[0].Premise != "gone") {
				t.Fatalf("branch premise used a fresh symbolic HEAD: %+v", status.Raw.Questions)
			}
			if status.Raw.Main != tip || status.Raw.Anchors.Branch != branch || status.Raw.Anchors.MergeHEAD != merge {
				t.Fatalf("mixed anchors: %+v", status.Raw.Anchors)
			}
		})
	}
}

func TestObservationRejectsRegistrationFromAnotherCheckout(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	other := b.record
	other.Root = t.TempDir()
	shadowRegister(t, b.home, other)
	_, err := captureObservation(b.home, &b.record, shadowSeams(), nil)
	var changed *Reregistered
	if !errors.As(err, &changed) || changed.Before.Root != b.checkout || changed.After.Root != other.Root {
		t.Fatalf("registration accepted: %v", err)
	}
	if _, exists, _ := state.Read(Dir(b.install)); exists {
		t.Fatal("wrote shadow on refusal")
	}
	shadowRegister(t, b.home, b.record)
}

func TestObservationStalledRefreshCannotRestorePause(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if _, err := lane.SetPause(b.home, "fixture", bedNow); err != nil {
		t.Fatal(err)
	}
	seam := shadowSeams()
	captured := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var once sync.Once
	seam.BeforeShadowPublish = func() { once.Do(func() { close(captured); <-release }) }
	done := make(chan error, 1)
	go func() { _, err := RefreshShadow(b.home, bedNow, "A", seam); done <- err }()
	<-captured
	if _, err := lane.ClearPause(b.home); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "B", shadowSeams()); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, _, _ := state.Read(Dir(b.install))
	if got.Lane.Word != "running" {
		t.Fatal(got.Lane)
	}
	for _, e := range shadowEvents(t, b.install) {
		if strings.Contains(e.Evidence, "→ paused") {
			t.Fatalf("stale pause published: %+v", e)
		}
	}
}

func TestObservationUnstablePublishesNothing(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	seam := shadowSeams()
	calls := 0
	seam.Observed = func() {
		calls++
		if calls%2 == 1 {
			data, _ := json.Marshal(lane.Pause{By: "fixture", At: bedNow.Format(time.RFC3339Nano)})
			if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Remove(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := RefreshShadow(b.home, bedNow, "keeper", seam); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal(calls)
	}
	if _, exists, _ := state.Read(Dir(b.install)); exists {
		t.Fatal("unstable observation published")
	}
	if events := shadowEvents(t, b.install); len(events) != 1 || events[0].Kind != "observation-unstable" {
		t.Fatal(events)
	}
	calls = 0
	if err := os.Remove(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json")); err != nil {
		t.Fatal(err)
	}
	status := ObserveStatus(b.home, b.record, seam, nil)
	if calls != 3 || status.Health == nil || len(status.Health.Rows()) != 1 || status.Health.Rows()[0].Row != "observation-unstable" || status.Health.Text() != (&Unstable{}).Error() {
		t.Fatalf("unstable health compared or hid changing inputs: %+v calls=%d", status, calls)
	}

}

func TestObservationAgentWordAndIdleClock(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	seam := shadowSeams()
	alive := true
	seam.Alive = func(Running) bool { return alive }
	seam.AgentProbe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: true, Since: bedNow}, nil }
	step := AgentStep{Verb: "prove", StartedAt: bedNow.Format(time.RFC3339), PID: 12}
	if err := writeAgentRecord(b.install, "agent-step.json", step); err != nil {
		t.Fatal(err)
	}
	read := func() state.Record {
		t.Helper()
		if _, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "keeper", seam); err != nil {
			t.Fatal(err)
		}
		r, _, _ := state.Read(Dir(b.install))
		return r
	}
	live := read()
	alive = false
	gone := read()
	if live.Agent.Word != "stepping" || gone.Agent.Word != "step-process-gone" || !gone.Agent.Since.Observed || gone.Agent.Since.Source != "agent-step.json" {
		t.Fatalf("live=%+v gone=%+v", live.Agent, gone.Agent)
	}
	if !slices.ContainsFunc(state.Diff(live, gone), func(s string) bool { return strings.HasPrefix(s, "agent.word:") }) {
		t.Fatal("agent death missing from diff")
	}
	var retained AgentStep
	if err := readAgentRecord(b.install, "agent-step.json", &retained); err != nil || !reflect.DeepEqual(retained, step) {
		t.Fatalf("raw step changed: %+v %v", retained, err)
	}
	if err := os.Remove(filepath.Join(Dir(b.install), "agent-step.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentRecord(b.install, "agent-idle.json", agentIdle{EndedAt: bedNow.Add(-time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	idle := read()
	if idle.Agent.Word != "idle" || !idle.Agent.Since.At.Equal(bedNow) {
		t.Fatal(idle.Agent)
	}
	if !reflect.DeepEqual(gone.Agent.Step, state.Step{}) {
		t.Fatal("retired step remained in the shadow")
	}
}

func TestObservationUsesSameTipForQueueAndQuestion(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	old := strings.Repeat("a", 40)
	tip := old
	b.lines("queue.jsonl", Line{Goal: "A", SHA: old, At: bedNow.Format(time.RFC3339)})
	_, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Kind: "other", Machine: "fixture", Lineage: lane.AgentLineage, Facts: []string{"Main is at commit " + old + "."}, Now: bedNow})
	if err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	base := seams.Git
	observations := 0
	advanced := false
	seams.Observed = func() { observations++ }
	seams.Git = func(dir string, args ...string) (string, error) {
		if slices.Contains(args, "refs/remotes/origin/main^{commit}") {
			return tip, nil
		}
		if args[0] == "merge-base" {
			if !advanced {
				advanced = true
				tip = strings.Repeat("b", 40)
			}
			return "", nil
		}
		return base(dir, args...)
	}
	status := ObserveStatus(b.home, b.record, seams, nil)
	if observations != 2 || len(status.Problems) != 0 || status.Queue[0].State != StateLanded || status.Raw.Main != tip || len(status.Raw.Questions) != 1 || status.Raw.Questions[0].Premise != "gone" {
		t.Fatalf("mixed containment/premise: observations=%d raw=%+v problems=%v", observations, status.Raw, status.Problems)
	}
}

func TestObservationMergeAbortBeforePublicationIsRetaken(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Attempt: "merge", Goal: "A", State: "resolving", Tip: "tip", Commit: "head", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "file.go", Class: conflict.Builder}}}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	seam := shadowSeams()
	base := seam.Git
	merge := "tip"
	publications := 0
	observations := 0
	seam.Git = func(dir string, args ...string) (string, error) {
		if slices.Contains(args, "MERGE_HEAD^{commit}") {
			return merge, nil
		}
		return base(dir, args...)
	}
	seam.Observed = func() { observations++ }
	seam.BeforeShadowPublish = func() {
		publications++
		if publications == 1 {
			merge = ""
		}
	}
	if _, err := RefreshShadow(b.home, bedNow, "keeper", seam); err != nil {
		t.Fatal(err)
	}
	r, _, _ := state.Read(Dir(b.install))
	if observations != 2 || publications != 2 || r.Resolution.Word != "abandoned" || len(shadowEvents(t, b.install)) != 1 {
		t.Fatalf("aborted merge published resolving: %+v observations=%d publications=%d", r.Resolution, observations, publications)
	}
}

func TestObservationNeverFetchesIncidentHeldQueue(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "sha", At: bedNow.Format(time.RFC3339)})
	if _, err := lane.SetPause(b.home, "fixture", bedNow); err != nil {
		t.Fatal(err)
	}
	seams := shadowSeams()
	base := seams.Git
	fetches := 0
	seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			fetches++
			return "", errors.New("origin unavailable")
		}
		return base(dir, args...)
	}
	seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{ID: "incident"}}, nil
	}
	if _, err := RefreshShadow(b.home, bedNow, "keeper", seams); err != nil {
		t.Fatal(err)
	}
	before, _, _ := state.Read(Dir(b.install))
	status := ObserveStatus(b.home, b.record, seams, nil)
	after, _, _ := state.Read(Dir(b.install))
	if fetches != 0 || len(status.Problems) != 0 || !reflect.DeepEqual(before, after) || len(before.HandIns) != 1 {
		t.Fatalf("observation fetched or wrote: fetches=%d problems=%v", fetches, status.Problems)
	}
}

func TestObservationRetakesWhenAnAbsentShadowAppears(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	calls := 0
	status := ObserveStatus(b.home, b.record, shadowSeams(), func(ProveSeams) lane.View {
		calls++
		if calls == 1 {
			if err := os.MkdirAll(Dir(b.install), 0700); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(state.Record{})
			if err := os.WriteFile(filepath.Join(Dir(b.install), "state.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return b.view
	})
	if calls != 2 || status.Shadow == nil || len(status.Problems) != 0 {
		t.Fatalf("captured absence was mistaken for version zero: calls=%d status=%+v", calls, status)
	}
}
