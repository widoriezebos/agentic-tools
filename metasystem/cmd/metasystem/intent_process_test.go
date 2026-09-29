package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// processBed is one isolated checkout with an installed engine file. The stop
// transition, the stop fence, the health preview, the launch store and the
// mission answer transition are the real owners over its files; the caller's
// terminal class, the arm sequence's steward and supervision effects and the
// channel transport are per-test fakes.
type processBed struct {
	*intentBed
	class    string
	armCalls int
	armErr   error
	// helpersRun makes every arm after the first find the helpers running.
	helpersRun bool
	enrolls    int
	launchDir  string
	families   []stoptransition.Family
	asked      []channelAskInput
	question   channel.Question
	// engineCalls are the engine verbs a public command ran.
	engineCalls [][]string
	// home is the HOME the bed's evidence root resolves under.
	home string
}

func newProcessBed(t *testing.T) *processBed {
	t.Helper()
	b := &processBed{intentBed: newIntentBed(t, false, nil), class: lease.ClassHuman, launchDir: t.TempDir(), home: t.TempDir()}
	b.writeEngine("engine build 1")
	return b
}

func (b *processBed) writeEngine(content string) {
	b.t.Helper()
	binary := filepath.Join(b.root(), "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := testexec.WriteFile(binary, []byte(content), 0o755); err != nil {
		b.t.Fatal(err)
	}
}

func (b *processBed) owners() intentOwners {
	owners := b.intentBed.owners()
	self := identity.Ref{Pid: int64(os.Getpid()), StartedAtSec: 1}
	owners.processes = processIntentOwners{
		process: processOwners{
			repositoryTop: fakeTop(b.root()),
			classify: func(string, string, int64) (lease.Classification, error) {
				return lease.Classification{Class: b.class}, nil
			},
			transition: func(scope processScope, scale int) *stoptransition.Transition {
				// No process family is running in the bed: the transition's
				// own fence, lock and report are what is exercised.
				return &stoptransition.Transition{Root: scope.Root, Checkout: scope.Checkout, ScaleMilli: scale, Families: b.families,
					Self: func() (identity.Ref, error) { return self, nil }}
			},
			evidenceRoot: func(conf string) (config.EvidenceRoot, error) {
				return config.ResolveEvidenceRoot(config.EvidenceRootParams{ConfPath: conf, LookupEnv: homeOnly(b.home)})
			},
			armSteps: func(processScope, int, processArmAuthority) (processArmResult, error) {
				b.armCalls++
				if b.armErr != nil {
					return processArmResult{lines: []string{"steward arm refused"}}, b.armErr
				}
				// Once armed, a bed that says its helpers keep running
				// answers a further arm as the real sequence does: the same
				// live runner kept and nothing started.
				if b.helpersRun && b.armCalls > 1 {
					return processArmResult{lines: []string{"already armed (runner pid 4242)"}, unchanged: true, runnerPid: 4242}, nil
				}
				return processArmResult{lines: []string{"steward armed"}}, nil
			},
		},
		up: func(options up.Options) up.Result {
			b.t.Errorf("a human start called the agent session start with %+v", options)
			return up.Result{}
		},
		health: func(repo, installation string, now time.Time) steward.HealthVerdict {
			return steward.PreviewHealthAt(repo, installation, now, nil)
		},
		healthNow: func(string) (time.Time, error) { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), nil },
		launches:  func() *launch.Manager { return &launch.Manager{Store: launch.Store{Root: b.launchDir}} },
		cancelDispatch: func(string, string) (map[string]any, int, error) {
			b.t.Error("a launch or unknown job reached the dispatch cancellation owner")
			return nil, 1, nil
		},
		sessionStop: authorizeSessionStop,
		enroll: func(root string, _ int64, _ humanauthority.Reader, by string, now time.Time) (humanauthority.Enrollment, error) {
			b.enrolls++
			return humanauthority.Enrollment{Human: by, EnrolledAt: now, Generation: 1}, nil
		},
		ask: func(root string, in channelAskInput) (channel.Question, []string, int, error) {
			b.asked = append(b.asked, in)
			return channel.Question{ID: "q-1", Goal: in.Goal, Kind: in.Kind, Wants: in.Wants, State: "open", Thread: &channel.MessageRef{ID: "m-1"}}, nil, 0, nil
		},
		question: func(root, id string) (channel.Question, error) {
			if id != b.question.ID {
				return channel.Question{}, os.ErrNotExist
			}
			return b.question, nil
		},
		mission: func(root, id string) (*missionrunner.Engine, error) { return missionrunner.NewEngine(root, id), nil },
		ui: func(string, lifecycle.Roots, uiIntentOptions) (uiLifecycleResult, error) {
			b.t.Error("the interface lifecycle was called")
			return uiLifecycleResult{}, errors.New("unexpected")
		},
		executable: os.Executable,
	}
	// Engine verbs (the mission runner's resume after an answer) run against
	// a stand-in engine that records them; the test binary is never run as
	// the engine.
	owners.delivery = processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			b.engineCalls = append(b.engineCalls, process.argv)
			return intentProcessResult{stdout: []byte(`{"outcome":"resumed"}`)}
		}})
	return owners
}

func (b *processBed) fence() stopfence.Record {
	b.t.Helper()
	record, err := stopfence.Read(b.root())
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}

func TestIntentProcessAndAnswerTargets(t *testing.T) {
	t.Parallel()
	t.Run("an agent cannot stop the checkout and nothing is stopped", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		b.class = lease.ClassDelegate
		code, result := b.runJSON(b.owners(), "system", "stop")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "agent-free terminal") ||
			!strings.Contains(result.Summary, "human act at a terminal") {
			t.Fatalf("agent stop = %d %+v", code, result)
		}
		if record := b.fence(); record.State != stopfence.StateOpen || record.Generation != 0 {
			t.Fatalf("a refused stop changed the fence: %+v", record)
		}
		code, result = b.runJSON(b.owners(), "system", "restart")
		if code != 1 || result.Outcome != intentRefused || b.armCalls != 0 || b.fence().State != stopfence.StateOpen {
			t.Fatalf("agent restart = %d %+v, arm calls %d", code, result, b.armCalls)
		}
	})

	t.Run("stop, status and doctor read the same installation", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		code, result := b.runJSON(b.owners(), "system", "stop")
		if code != 0 || result.Outcome != intentConfirmed || result.Targets[0].Kind != "checkout" {
			t.Fatalf("human stop = %d %+v", code, result)
		}
		if record := b.fence(); record.State != stopfence.StateClosed {
			t.Fatalf("stop left the fence %+v", record)
		}
		_, status := b.runJSON(b.owners(), "status")
		if status.Outcome != intentConfirmed || status.Targets[0].ID != result.Targets[0].ID {
			t.Fatalf("status = %+v", status)
		}
		code, doctor := b.runJSON(b.owners(), "system", "check")
		encoded, _ := json.Marshal(doctor.Data)
		var preview steward.HookHealthPreview
		if err := json.Unmarshal(encoded, &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.Verdict.Stopped || doctor.Outcome != intentConfirmed || code != preview.ExitCode {
			t.Fatalf("doctor did not read the stopped fence the stop wrote: %d %+v", code, preview)
		}
		// doctor is the preview: it wrote no health observation record.
		if _, err := os.Stat(steward.HealthRecordPath(b.root())); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("doctor wrote health state: %v", err)
		}
	})

	t.Run("restart names the stopped state when its start refuses", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		b.armErr = errors.New("the steward refused to arm")
		code, result := b.runJSON(b.owners(), "system", "restart")
		record := b.fence()
		if code == 0 || result.Outcome != intentPartial || b.armCalls != 1 || !strings.Contains(result.Summary, "the stop fence is now "+record.State+"/"+record.Phase) {
			t.Fatalf("restart = %d %+v", code, result)
		}
		if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "system", "start"}) {
			t.Fatalf("restart next = %+v", result.Next)
		}
		if data := result.Data.(map[string]any); data["reached"] != "stopped" || data["fence"] != record.State+"/"+record.Phase {
			t.Fatalf("restart reached %v, fence %v; recorded %+v", data["reached"], data["fence"], record)
		}
		if record.Generation != 2 {
			t.Fatalf("the restart did not stop before it armed: %+v", record)
		}
		b.armErr = nil
		code, result = b.runJSON(b.owners(), "system", "restart")
		if code != 0 || result.Outcome != intentConfirmed || b.armCalls != 2 || b.fence().State != stopfence.StateOpen {
			t.Fatalf("repeated restart = %d %+v; fence %+v", code, result, b.fence())
		}
	})

	t.Run("a rebuilt engine keeps the live enrolled terminal", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		if code, result := b.runJSON(b.owners(), "system", "start"); code != 0 || result.Outcome != intentConfirmed || b.armCalls != 1 {
			t.Fatalf("start = %d %+v", code, result)
		}
		b.writeEngine("engine build 2")
		if code, result := b.runJSON(b.owners(), "system", "stop"); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("stop after rebuild = %d %+v", code, result)
		}
		if code, result := b.runJSON(b.owners(), "system", "start"); code != 0 || result.Outcome != intentConfirmed || b.armCalls != 2 {
			t.Fatalf("start after rebuild = %d %+v", code, result)
		}
		if b.enrolls != 0 || b.fence().State != stopfence.StateOpen {
			t.Fatalf("rebuild asked for enrollment %d times; fence %+v", b.enrolls, b.fence())
		}
	})

	t.Run("each target keeps its own authority", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		var sessionUp *up.Options
		owners.processes.up = func(options up.Options) up.Result {
			sessionUp = &options
			return up.Result{Outcome: "READY"}
		}
		code, result := b.runJSON(owners, "session", "start")
		if code != 0 || result.Outcome != intentConfirmed || sessionUp == nil || !samePath(sessionUp.Scope, b.root()) || b.armCalls != 0 {
			t.Fatalf("start session = %d %+v, up %+v, arm %d", code, result, sessionUp, b.armCalls)
		}
		for _, args := range [][]string{
			{"session", "start", "--temporary-human-word", "yes", "--review-by", "2026-10-01"},
			{"system", "stop", "--by", "Wido"},
			{"work", "stop"},
			{"session", "stop"},
			{"status", "fleet", "extra"},
		} {
			code, result := b.runJSON(owners, args...)
			if code == 0 || result.Outcome != intentRefused {
				t.Fatalf("%v = %d %+v", args, code, result)
			}
		}
		// The quiet session stop is the real owner: this test process is not
		// an attended human terminal, so it refuses and writes no marker.
		code, result = b.runJSON(owners, "session", "stop", "--by", "Wido")
		if code != 3 || result.Outcome != intentRefused || !strings.HasPrefix(result.Summary, "session stop refused") {
			t.Fatalf("agent session stop = %d %+v", code, result)
		}
		if entries, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "session-stop*")); len(entries) != 0 {
			t.Fatalf("refused session stop wrote %v", entries)
		}
		if b.fence().State != stopfence.StateOpen {
			t.Fatal("a session stop moved the checkout fence")
		}
	})

	t.Run("a job id names exactly one retained record", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		store := launch.Store{Root: b.launchDir}
		for _, record := range []launch.Record{{ID: "job-a", State: launch.Completed}, {ID: "job-b", State: launch.Failed}} {
			if err := store.Create(record); err != nil {
				t.Fatal(err)
			}
		}
		jobs := filepath.Join(b.root(), "artifacts", "agents", "jobs")
		if err := os.MkdirAll(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"job-a", "job-c"} {
			if err := os.WriteFile(filepath.Join(jobs, id+".json"), []byte(`{"status":"running","job":"`+id+`"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		owners := b.owners()
		code, ambiguous := b.runJSON(owners, "work", "stop", "job-a")
		if code != 1 || ambiguous.Outcome != intentRefused || !strings.Contains(ambiguous.Summary, "names 2 records (j1:job-a, j2:job-a)") ||
			strings.Contains(ambiguous.Decision, "internal") || !slices.Equal(ambiguous.Data.(map[string]any)["candidates"].([]any), []any{"j1:job-a", "j2:job-a"}) ||
			!strings.Contains(fmt.Sprint(ambiguous.Data.(map[string]any)["choices"]), "[metasystem work stop j1:job-a]") {
			t.Fatalf("ambiguous job = %d %+v", code, ambiguous)
		}
		// Each offered reference reaches exactly its own store.
		if code, result := b.runJSON(owners, "work", "stop", "j1:job-a"); code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "launch job-a already ended") {
			t.Fatalf("stop job j1:job-a = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "work", "status", "j2:job-a"); code != 0 || result.Summary != "dispatch job j2:job-a: running" || result.Targets[0].ID != "j2:job-a" {
			t.Fatalf("status job j2:job-a = %d %+v", code, result)
		}
		code, unknown := b.runJSON(owners, "work", "stop", "job")
		if code != 1 || unknown.Outcome != intentRefused || !strings.HasPrefix(unknown.Summary, "no launch or dispatch job or diagnostic read names job;") || unknown.Next == nil ||
			!slices.Equal(unknown.Next.Argv, []string{"metasystem", "work", "status", "--all"}) {
			t.Fatalf("unknown job = %d %+v", code, unknown)
		}
		code, listed := b.runJSON(owners, unknown.Next.Argv[1:]...)
		references := []string{}
		for _, job := range listed.Data.(map[string]any)["jobs"].([]any) {
			references = append(references, job.(map[string]any)["reference"].(string))
		}
		slices.Sort(references)
		if code != 0 || !slices.Equal(references, []string{"j1:job-a", "j1:job-b", "j2:job-a", "j2:job-c"}) {
			t.Fatalf("status work --all = %d %v %+v", code, references, listed)
		}
		code, running := b.runJSON(owners, "work", "status")
		runningRefs := []string{}
		for _, job := range running.Data.(map[string]any)["jobs"].([]any) {
			view := job.(map[string]any)
			runningRefs = append(runningRefs, view["reference"].(string))
			if view["stop"] == nil || view["wait"] == nil {
				t.Fatalf("a running job offers its wait and stop: %v", view)
			}
		}
		slices.Sort(runningRefs)
		if code != 0 || !slices.Equal(runningRefs, []string{"j2:job-a", "j2:job-c"}) || running.Next == nil {
			t.Fatalf("status work = %d %v %+v", code, runningRefs, running)
		}
		if code, result := b.runJSON(owners, "work", "stop", "job-b"); code != 0 || result.Outcome != intentUnchanged {
			t.Fatalf("ended launch = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "work", "status", "job-c"); code != 0 || result.Summary != "dispatch job j2:job-c: running" {
			t.Fatalf("dispatch status = %d %+v", code, result)
		}
		var cancelled []string
		owners.processes.cancelDispatch = func(checkout, job string) (map[string]any, int, error) {
			cancelled = append(cancelled, checkout+" "+job)
			return map[string]any{"outcome": "CANCELLED", "headline": "cancelled", "jobId": job}, 0, nil
		}
		if code, result := b.runJSON(owners, "work", "stop", "j2:job-c"); code != 0 || result.Outcome != intentConfirmed || len(cancelled) != 1 || !strings.HasSuffix(cancelled[0], " job-c") || !samePath(strings.TrimSuffix(cancelled[0], " job-c"), b.root()) {
			t.Fatalf("dispatch cancel = %d %+v %v", code, result, cancelled)
		}
		if code, result := b.runJSON(owners, "work", "status", "run:unit-z"); code != 1 || result.Outcome != intentRefused {
			t.Fatalf("unknown unit = %d %+v", code, result)
		}
	})

	t.Run("enrollment reports a local enrollment whose publication failed", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		owners.commandNow = func(string) (time.Time, error) { return time.Time{}, errors.New("the goal clock is unreadable") }
		code, result := b.runJSON(owners, "system", "enroll", "--name", "Wido")
		data, _ := result.Data.(map[string]any)
		if code == 0 || result.Outcome != intentPartial || b.enrolls != 1 || data["fleetPublished"] != false || data["enrollment"] == nil {
			t.Fatalf("partial enrollment = %d %+v", code, result)
		}
		if code, result := b.runJSON(b.owners(), "system", "enroll"); code != 2 || result.Next == nil || b.enrolls != 1 {
			t.Fatalf("nameless enrollment = %d %+v", code, result)
		}
	})

	t.Run("a mission answer advances or rolls back through its transition", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		askPath := parkedHostFailureMission(t, b.root())
		owners := b.owners()
		// An answered question resumes the mission through the runner; the
		// bed's runner confirms without starting a process.
		var resumed [][]string
		owners.delivery = processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
			process: func(process intentProcess) intentProcessResult {
				resumed = append(resumed, process.argv)
				return intentProcessResult{stdout: []byte(`{"outcome":"resumed"}`)}
			}})
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(string, string, string) error { return errors.New("anchor refused in the bed") }
			return engine, nil
		}
		code, result := b.runJSON(owners, "question", "answer", "demo/host-down", "retry: the host is back")
		if code != 3 || result.Outcome != intentRefused || missionAskAnswered(askPath) {
			t.Fatalf("rolled-back answer = %d %+v", code, result)
		}
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(string, string, string) error { return nil }
			return engine, nil
		}
		code, result = b.runJSON(owners, "question", "answer", "demo/host-down", "retry: the host is back")
		if code != 0 || result.Outcome != intentConfirmed || !missionAskAnswered(askPath) || len(resumed) != 1 ||
			!slices.Equal(resumed[0][1:4], []string{"internal", "mission", "resume"}) {
			t.Fatalf("answer = %d %+v resumed %v", code, result, resumed)
		}
		if code, result := b.runJSON(owners, "question", "answer", "demo/host-down", "again"); code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "already answered with a different answer") {
			t.Fatalf("second answer = %d %+v", code, result)
		}
	})

	t.Run("a channel question is answered in its channel", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		b.question = channel.Question{ID: "q-7", Goal: "g", State: "open", Wants: "resume g 1d/10/720m/1/3"}
		code, result := b.runJSON(b.owners(), "question", "answer", "q-7")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "Reply in this thread with this token verbatim") ||
			!strings.Contains(result.Decision, b.question.Wants) {
			t.Fatalf("channel answer = %d %+v", code, result)
		}
		b.question.Answer = &channel.Answer{}
		if code, result := b.runJSON(b.owners(), "question", "answer", "q-7"); code != 0 || result.Outcome != intentUnchanged {
			t.Fatalf("answered channel question = %d %+v", code, result)
		}
		code, result = b.runJSON(b.owners(), "question", "ask", "goal-a", "--question", "Resume it?", "--option", "yes: resume", "--kind", "stop", "--budget", "1d/10/720m/1/3")
		if code != 0 || result.Outcome != intentConfirmed || len(b.asked) != 1 || result.Next == nil {
			t.Fatalf("ask = %d %+v", code, result)
		}
		box, _ := goal.NewBudget("1d", 10, 720, 1, 3)
		if asked := b.asked[0]; asked.Wants != goal.ResumeApprovalToken("goal-a", box) || asked.Facts[0] != "Resume it?" || asked.Budget != nil {
			t.Fatalf("stop question = %+v", asked)
		}
		if code, result := b.runJSON(b.owners(), "question", "ask", "goal-a", "--question", "Why?", "--option", "a: b", "--budget", "1d/10/720m/1/3"); code != 2 || result.Outcome != intentRefused || len(b.asked) != 1 {
			t.Fatalf("budget without kind = %d %+v", code, result)
		}
	})

	t.Run("readings keep unknown and quoted paths", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		owners.processes.health = func(string, string, time.Time) steward.HealthVerdict {
			return steward.HealthVerdict{Aggregate: "unknown", Roles: []steward.RoleVerdict{{Role: steward.RoleStewardRunner, Status: steward.HealthUnknown, Reason: "no tick yet", Remedy: "metasystem system start"}}}
		}
		code, result := b.runJSON(owners, "system", "check")
		if code != 2 || result.Outcome != intentConfirmed {
			t.Fatalf("unknown doctor = %d %+v", code, result)
		}
		_, stdout, _ := b.run(owners, "system", "check")
		if !strings.Contains(stdout, "unknown: no tick yet; remedy: metasystem session start") {
			t.Fatalf("doctor text dropped the owner remedy: %q", stdout)
		}
		missing := filepath.Join(t.TempDir(), "a dir", "x")
		code, result = b.runJSON(owners, "status", "--repo", missing)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, shellCommand([]string{missing})) {
			t.Fatalf("quoted repo = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "system", "stop", "--installation", "a", "--installation", "b"); code != 2 || !strings.Contains(result.Summary, "given twice") {
			t.Fatalf("conflict = %d %+v", code, result)
		}
		if b.fence().State != stopfence.StateOpen || b.armCalls != 0 {
			t.Fatal("a refused reading changed the checkout")
		}
	})

	t.Run("old flag-only calls are refused at the top", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{{"stop", "--repo", "."}, {"stop", "--all"}, {"arm", "--repo", "."}, {"health", "--repo", "."}} {
			if code, _, stderr := routeWith(families(), args...); code != 2 || !strings.Contains(stderr, "nothing was done") {
				t.Fatalf("%v = %d %q; the flag-only machinery calls are internal", args, code, stderr)
			}
		}
	})
}

// parkedHostFailureMission writes a mission parked for a host failure with
// one open ask, through the mission state's own compare-and-write.
func parkedHostFailureMission(t *testing.T, root string) string {
	t.Helper()
	return parkedMission(t, root, "host-failure", map[string]any{"askId": "host-down", "streamId": "primary", "reasonClass": "host-failure", "question": "retry?"})
}

// parkedMission parks the demo mission for reason with one open ask.
func parkedMission(t *testing.T, root, reason string, ask map[string]any) string {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "missions", "demo")
	if err := os.MkdirAll(filepath.Join(dir, "asks"), 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(dir, "mission-demo.contract.md")
	if err := os.WriteFile(contractPath, []byte("# Intent\n\n```mission\ncandidate.branch=main\nstream.primary=Do the work\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath, ledgerPath := filepath.Join(dir, "state.json"), filepath.Join(dir, "ledger.md")
	if err := mission.InitLedger(ledgerPath, 8, 2); err != nil {
		t.Fatal(err)
	}
	origins := map[string]any{"headCommit": strings.Repeat("c", 40), "topTree": nil, "topStaged": nil,
		"refMap": map[string]any{}, "worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z"}
	if err := mission.InitStateWithBaseline(statePath, contractPath, ledgerPath, "", "main", strings.Repeat("b", 40), origins); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	hash, _ := state["integrity"].(map[string]any)["hash"].(string)
	state["status"], state["parkReason"], state["waitingList"] = "parked", reason, []any{ask["askId"]}
	proposed := filepath.Join(t.TempDir(), "proposed.json")
	encoded, _ := json.Marshal(state)
	if err := os.WriteFile(proposed, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mission.WriteState(statePath, proposed, hash); err != nil {
		t.Fatal(err)
	}
	askPath := filepath.Join(dir, "asks", ask["askId"].(string)+".json")
	for key, value := range map[string]any{"createdAt": "2026-09-25T00:00:00Z", "answeredAt": nil, "answer": nil, "supersedes": nil, "supersededBy": nil} {
		ask[key] = value
	}
	encodedAsk, _ := json.Marshal(ask)
	if err := os.WriteFile(askPath, encodedAsk, 0o644); err != nil {
		t.Fatal(err)
	}
	return askPath
}

func samePath(left, right string) bool {
	left, _ = filepath.EvalSymlinks(left)
	right, _ = filepath.EvalSymlinks(right)
	return left == right
}

// system start says where evidence goes, and an explicitly invalid root
// refuses by the owner's sentence before the fence moves; unset never does.
func TestSystemStartSaysTheEvidenceRoot(t *testing.T) {
	t.Parallel()
	t.Run("nothing configured prints the default line first", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		code, result := b.runJSON(b.owners(), "system", "start")
		data, _ := result.Data.(map[string]any)
		lines, _ := data["lines"].([]any)
		want := "evidence root: " + filepath.Join(b.home, "metasystem-evidence", filepath.Base(b.root())) + " (default; set evidence.root in metasystem.conf.local to change)"
		if code != 0 || len(lines) == 0 || lines[0] != want {
			t.Fatalf("start = %d %+v; want first line %q", code, result, want)
		}
	})
	t.Run("a relative .local root refuses before the fence", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		if code, result := b.runJSON(b.owners(), "system", "stop"); code != 0 || b.fence().State == stopfence.StateOpen {
			t.Fatalf("stop = %d %+v; fence %+v", code, result, b.fence())
		}
		if err := os.WriteFile(filepath.Join(b.root(), "metasystem.conf.local"), []byte("evidence.root=relative\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := b.fence()
		code, stdout, stderr := b.run(b.owners(), "system", "start")
		sentence := `evidence.root must be absolute (metasystem.conf.local reads "relative")`
		if code == 0 || !strings.Contains(stdout+stderr, sentence) || b.armCalls != 0 {
			t.Fatalf("start = %d stdout=%q stderr=%q arms=%d", code, stdout, stderr, b.armCalls)
		}
		if after := b.fence(); !reflect.DeepEqual(after, before) {
			t.Fatalf("the fence moved: %+v, then %+v", before, after)
		}
	})
}

// TestIntentWorkStatusPrintsTheBatchWaitLine (R23, U10b-2): work status G
// for a goal in a waiting batch prints the batch's one wait line, in local
// time, and the record's data names the batch.
func TestIntentWorkStatusPrintsTheBatchWaitLine(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	landing := t.TempDir()
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return landing, true, nil }
	three := 3
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000wa01", State: batch.StateOpen,
		Units: []batch.Unit{{GoalID: "standing-validation", Chain: "c", Claim: batch.Claim{Machine: "landing", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}, State: batch.UnitJoined}},
		Wait: &batch.WaitState{Reason: "r", ProofCost: 40 * time.Minute, Basis: "default", For: []batch.Waited{
			{Goal: "goal-x", Seat: "m1b", Stage: board.StageReview, Round: &board.Round{N: 2, Max: &three}, ExpectedAt: time.Date(2026, 9, 25, 12, 8, 0, 0, time.UTC)}}}}
	if err := batch.NewStore(landing, identity.KernelProber{}).Create(record); err != nil {
		t.Fatal(err)
	}
	code, result := b.do("work", "status", "standing-validation")
	want := "batch 01j5x00000000000000000wa01 waits for goal-x on m1b (review round 2 of 3, ~8 min); a separate proof costs ~40 min (default)"
	data, _ := result.Data.(map[string]any)
	if code != 0 || data == nil || data["batch"] == nil || data["batch"].(map[string]any)["line"] != want {
		t.Fatalf("work status: code %d data %+v", code, result.Data)
	}
}

// TestIntentWorkStatusSaysWhatTheWaitIsUsedFor (R27, R23, U10b-3): each
// member's work status and status print the batch's wait line with its
// meanwhile clause, here a partial red nobody was named for.
func TestIntentWorkStatusSaysWhatTheWaitIsUsedFor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	landing := t.TempDir()
	claim := batch.Claim{Machine: "landing", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000wa01", State: batch.StateOpen,
		Units: []batch.Unit{{GoalID: "standing-validation", Chain: "c", Claim: claim, State: batch.UnitJoined},
			{GoalID: "goal-b", Chain: "c", Claim: claim, State: batch.UnitJoined}},
		Wait: &batch.WaitState{Reason: "r", ProofCost: 40 * time.Minute, Basis: "default", For: []batch.Waited{
			{Goal: "goal-x", Seat: "m1b", Stage: board.StageBuild, ExpectedAt: now.Add(8 * time.Minute)}}},
		Early: &batch.Early{Shape: []string{"standing-validation", "goal-b"}, Tree: "tip", Cheap: "red",
			Finding: &batch.EarlyFinding{Group: "go-unit", Attempt: "proof-1", Log: "/logs/unit.log"}}}
	if err := batch.NewStore(landing, identity.KernelProber{}).Create(record); err != nil {
		t.Fatal(err)
	}
	want := "batch 01j5x00000000000000000wa01 waits for goal-x on m1b (build, ~8 min); a separate proof costs ~40 min (default); " +
		"meanwhile: partial red: go-unit on standing-validation+goal-b; decided at the batch proof"
	b := newDeliveryBed(t)
	b.owners.now = func() time.Time { return now }
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return landing, true, nil }
	code, result := b.do("work", "status", "standing-validation")
	data, _ := result.Data.(map[string]any)
	if code != 0 || data == nil || data["batch"] == nil || data["batch"].(map[string]any)["line"] != want {
		t.Fatalf("work status: code %d data %+v", code, result.Data)
	}
	p := newProcessBed(t)
	owners := p.owners()
	owners.delivery.now = func() time.Time { return now }
	owners.delivery.batchRoot = func(string, time.Time) (string, bool, error) { return landing, true, nil }
	owners.delivery.boardView = func(string, time.Time) board.View { return board.View{} }
	if code, stdout, _ := p.run(owners, "status"); code != 0 || !slices.Contains(strings.Split(strings.TrimRight(stdout, "\n"), "\n"), want) {
		t.Fatalf("status = %d:\n%s", code, stdout)
	}
}

// TestStatusShowsTheBoardAndEachUnfinishedBatch (R23, R24, U10d): status
// prints the board block from a direct read of the fixture host (one line
// per armed seat, in local time, saying bridge absent with no socket) and
// one line per unfinished batch, never a finished one; --verbose adds one
// line per goal; work status G prints the goal's card line, naming its seat,
// before its batch's wait line.
func TestStatusShowsTheBoardAndEachUnfinishedBatch(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	host := newPipelineHost(t)
	m1b := host.checkout(t, "b", "m1b", false, false)
	m1c := host.checkout(t, "c", "m1c", false, false)
	host.card(t, "m1b", m1b, "goal-x", board.StageBuild, now.Add(-10*time.Minute))
	host.card(t, "m1c", m1c, "standing-validation", board.StageJoined, now.Add(-5*time.Minute))
	host.card(t, "m1b", m1b, "goal-l", board.StageLanded, now.Add(-30*time.Minute))
	claims := map[string]string{"goal-x": "m1b", "standing-validation": "m1c", "goal-q": "m1c"}
	landing := t.TempDir()
	store := batch.NewStore(landing, identity.KernelProber{})
	claim := batch.Claim{Machine: "landing", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}
	for _, record := range []batch.Record{
		{Schema: 1, BatchID: "01j5x00000000000000000wa01", State: batch.StateOpen,
			Units: []batch.Unit{{GoalID: "standing-validation", Chain: "c", Claim: claim, State: batch.UnitJoined}},
			Wait: &batch.WaitState{Reason: "r", ProofCost: 40 * time.Minute, Basis: "default", For: []batch.Waited{
				{Goal: "goal-x", Seat: "m1b", Stage: board.StageBuild, ExpectedAt: now.Add(8 * time.Minute)}}}},
		{Schema: 1, BatchID: "01j5x00000000000000000wa02", State: batch.StateProving, StartReason: "nothing within reach",
			Units: []batch.Unit{{GoalID: "goal-p", Chain: "c", Claim: claim, State: batch.UnitJoined}}},
		{Schema: 1, BatchID: "01j5x00000000000000000wa03", State: batch.StateLanded, StartReason: "nothing within reach",
			Units: []batch.Unit{{GoalID: "goal-l", Chain: "c", Claim: claim, State: batch.UnitJoined}}},
	} {
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	view := func(string, time.Time) board.View { return host.source(claims).View(now) }
	local := func(at time.Time) string { return at.In(time.Local).Format("15:04") }

	t.Run("status", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		owners.delivery.now = func() time.Time { return now }
		owners.delivery.batchRoot = func(string, time.Time) (string, bool, error) { return landing, true, nil }
		owners.delivery.boardView = view
		code, stdout, _ := b.run(owners, "status")
		printed := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
		want := []string{
			"board: 2 seats on this host (bridge absent)",
			"  m1b: goal-x, build since " + local(now.Add(-10*time.Minute)) + " (1 finished)",
			"  m1c: standing-validation, joined since " + local(now.Add(-5*time.Minute)) + "; goal-q unknown: no card",
			"batch 01j5x00000000000000000wa01 waits for goal-x on m1b (build, ~8 min); a separate proof costs ~40 min (default)",
			"batch 01j5x00000000000000000wa02 started: nothing within reach",
		}
		if code != 0 || !slices.Equal(tailLines(printed, len(want)), want) {
			t.Fatalf("status = %d:\n%s\nwant the tail:\n%s", code, stdout, strings.Join(want, "\n"))
		}
		for _, line := range printed {
			if strings.Contains(line, "wa03") || strings.Contains(line, "{") {
				t.Errorf("status printed a finished batch or JSON: %q", line)
			}
		}
		_, verbose, _ := b.run(owners, "status", "--verbose")
		if !slices.Contains(strings.Split(verbose, "\n"), "    goal-l, landed since "+local(now.Add(-30*time.Minute))) {
			t.Errorf("status --verbose lacks the finished card's own line:\n%s", verbose)
		}
		if _, data := b.runJSON(owners, "status"); data.Data.(map[string]any)["board"] == nil {
			t.Errorf("status --json carries no board: %+v", data.Data)
		}
	})

	t.Run("work status G", func(t *testing.T) {
		t.Parallel()
		b := newDeliveryBed(t)
		b.owners.now = func() time.Time { return now }
		b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return landing, true, nil }
		b.owners.boardView = view
		owners := b.intentBed.owners()
		owners.delivery = b.owners
		code, stdout, _ := b.run(owners, "work", "status", "standing-validation")
		printed := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
		cardLine := "  board: standing-validation on m1c, joined since " + local(now.Add(-5*time.Minute))
		waitLine := "  batch 01j5x00000000000000000wa01 waits for goal-x on m1b (build, ~8 min); a separate proof costs ~40 min (default)"
		at := slices.Index(printed, cardLine)
		if code != 0 || at < 0 || at+1 >= len(printed) || printed[at+1] != waitLine {
			t.Fatalf("work status = %d:\n%s", code, stdout)
		}
	})
}

func tailLines(lines []string, n int) []string {
	if len(lines) < n {
		return lines
	}
	return lines[len(lines)-n:]
}
