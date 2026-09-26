package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	class     string
	armCalls  int
	armErr    error
	enrolls   int
	launchDir string
	families  []stoptransition.Family
	asked     []channelAskInput
	question  channel.Question
}

func newProcessBed(t *testing.T) *processBed {
	t.Helper()
	b := &processBed{intentBed: newIntentBed(t, false, nil), class: lease.ClassHuman, launchDir: t.TempDir()}
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
			armSteps: func(processScope, int, processArmAuthority) ([]string, error) {
				b.armCalls++
				if b.armErr != nil {
					return []string{"steward arm refused"}, b.armErr
				}
				return []string{"steward armed"}, nil
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
		ui: func(string, lifecycle.Roots) (uiLifecycleResult, error) {
			b.t.Error("the interface lifecycle was called")
			return uiLifecycleResult{}, errors.New("unexpected")
		},
		executable: os.Executable,
	}
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
		code, result := b.runJSON(b.owners(), "stop")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "agent-free terminal") ||
			!strings.Contains(result.Summary, "human act at a terminal") {
			t.Fatalf("agent stop = %d %+v", code, result)
		}
		if record := b.fence(); record.State != stopfence.StateOpen || record.Generation != 0 {
			t.Fatalf("a refused stop changed the fence: %+v", record)
		}
		code, result = b.runJSON(b.owners(), "restart", "checkout")
		if code != 1 || result.Outcome != intentRefused || b.armCalls != 0 || b.fence().State != stopfence.StateOpen {
			t.Fatalf("agent restart = %d %+v, arm calls %d", code, result, b.armCalls)
		}
	})

	t.Run("stop, status and doctor read the same installation", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		code, result := b.runJSON(b.owners(), "stop", "checkout")
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
		code, doctor := b.runJSON(b.owners(), "doctor")
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
		code, result := b.runJSON(b.owners(), "restart", "checkout")
		record := b.fence()
		if code == 0 || result.Outcome != intentPartial || b.armCalls != 1 || !strings.Contains(result.Summary, "the stop fence is now "+record.State+"/"+record.Phase) {
			t.Fatalf("restart = %d %+v", code, result)
		}
		if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "start"}) {
			t.Fatalf("restart next = %+v", result.Next)
		}
		if data := result.Data.(map[string]any); data["reached"] != "stopped" || data["fence"] != record.State+"/"+record.Phase {
			t.Fatalf("restart reached %v, fence %v; recorded %+v", data["reached"], data["fence"], record)
		}
		if record.Generation != 2 {
			t.Fatalf("the restart did not stop before it armed: %+v", record)
		}
		b.armErr = nil
		code, result = b.runJSON(b.owners(), "restart", "checkout")
		if code != 0 || result.Outcome != intentConfirmed || b.armCalls != 2 || b.fence().State != stopfence.StateOpen {
			t.Fatalf("repeated restart = %d %+v; fence %+v", code, result, b.fence())
		}
	})

	t.Run("a rebuilt engine keeps the live enrolled terminal", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		if code, result := b.runJSON(b.owners(), "start"); code != 0 || result.Outcome != intentConfirmed || b.armCalls != 1 {
			t.Fatalf("start = %d %+v", code, result)
		}
		b.writeEngine("engine build 2")
		if code, result := b.runJSON(b.owners(), "stop"); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("stop after rebuild = %d %+v", code, result)
		}
		if code, result := b.runJSON(b.owners(), "start", "checkout"); code != 0 || result.Outcome != intentConfirmed || b.armCalls != 2 {
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
		code, result := b.runJSON(owners, "start", "session")
		if code != 0 || result.Outcome != intentConfirmed || sessionUp == nil || !samePath(sessionUp.Scope, b.root()) || b.armCalls != 0 {
			t.Fatalf("start session = %d %+v, up %+v, arm %d", code, result, sessionUp, b.armCalls)
		}
		for _, args := range [][]string{
			{"start", "session", "--temporary-human-word", "yes", "--review-by", "2026-10-01"},
			{"stop", "--by", "Wido"},
			{"stop", "job"},
			{"stop", "session"},
			{"status", "fleet"},
			{"restart"},
		} {
			code, result := b.runJSON(owners, args...)
			if code == 0 || result.Outcome != intentRefused {
				t.Fatalf("%v = %d %+v", args, code, result)
			}
		}
		// The quiet session stop is the real owner: this test process is not
		// an attended human terminal, so it refuses and writes no marker.
		code, result = b.runJSON(owners, "stop", "session", "--by", "Wido")
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
		if code, result := b.runJSON(owners, "stop", "job", "job-a"); code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "both a launch record and the dispatch job") {
			t.Fatalf("ambiguous job = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "stop", "job", "job"); code != 1 || result.Outcome != intentRefused || !strings.HasPrefix(result.Summary, "no job job:") {
			t.Fatalf("prefix job = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "stop", "job", "job-b"); code != 0 || result.Outcome != intentUnchanged {
			t.Fatalf("ended launch = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "status", "job", "job-c"); code != 0 || result.Summary != "dispatch job job-c: running" {
			t.Fatalf("dispatch status = %d %+v", code, result)
		}
		var cancelled []string
		owners.processes.cancelDispatch = func(checkout, job string) (map[string]any, int, error) {
			cancelled = append(cancelled, checkout+" "+job)
			return map[string]any{"outcome": "CANCELLED", "headline": "cancelled", "jobId": job}, 0, nil
		}
		if code, result := b.runJSON(owners, "stop", "job", "job-c"); code != 0 || result.Outcome != intentConfirmed || len(cancelled) != 1 || !strings.HasSuffix(cancelled[0], " job-c") || !samePath(strings.TrimSuffix(cancelled[0], " job-c"), b.root()) {
			t.Fatalf("dispatch cancel = %d %+v %v", code, result, cancelled)
		}
		if code, result := b.runJSON(owners, "status", "unit", "unit-z"); code != 1 || result.Outcome != intentRefused {
			t.Fatalf("unknown unit = %d %+v", code, result)
		}
	})

	t.Run("enrollment reports a local enrollment whose publication failed", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		owners.commandNow = func(string) (time.Time, error) { return time.Time{}, errors.New("the goal clock is unreadable") }
		code, result := b.runJSON(owners, "enroll", "--name", "Wido")
		data, _ := result.Data.(map[string]any)
		if code == 0 || result.Outcome != intentPartial || b.enrolls != 1 || data["fleetPublished"] != false || data["enrollment"] == nil {
			t.Fatalf("partial enrollment = %d %+v", code, result)
		}
		if code, result := b.runJSON(b.owners(), "enroll"); code != 2 || result.Next == nil || b.enrolls != 1 {
			t.Fatalf("nameless enrollment = %d %+v", code, result)
		}
	})

	t.Run("a mission answer advances or rolls back through its transition", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		askPath := parkedHostFailureMission(t, b.root())
		owners := b.owners()
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(string, string, string) error { return errors.New("anchor refused in the bed") }
			return engine, nil
		}
		code, result := b.runJSON(owners, "answer", "mission", "demo", "host-down", "retry: the host is back")
		if code != 3 || result.Outcome != intentRefused || missionAskAnswered(askPath) {
			t.Fatalf("rolled-back answer = %d %+v", code, result)
		}
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(string, string, string) error { return nil }
			return engine, nil
		}
		code, result = b.runJSON(owners, "answer", "mission", "demo", "host-down", "retry: the host is back")
		if code != 0 || result.Outcome != intentConfirmed || !missionAskAnswered(askPath) {
			t.Fatalf("answer = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "answer", "mission", "demo", "host-down", "again"); code != 3 || result.Outcome != intentRefused {
			t.Fatalf("second answer = %d %+v", code, result)
		}
	})

	t.Run("a channel question is answered in its channel", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		b.question = channel.Question{ID: "q-7", Goal: "g", State: "open", Wants: "resume g 1d/10/720m/1/3"}
		code, result := b.runJSON(b.owners(), "answer", "question", "q-7")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "Reply in this thread with this token verbatim") ||
			!strings.Contains(result.Decision, b.question.Wants) {
			t.Fatalf("channel answer = %d %+v", code, result)
		}
		b.question.Answer = &channel.Answer{}
		if code, result := b.runJSON(b.owners(), "answer", "question", "q-7"); code != 0 || result.Outcome != intentUnchanged {
			t.Fatalf("answered channel question = %d %+v", code, result)
		}
		code, result = b.runJSON(b.owners(), "ask", "goal-a", "--question", "Resume it?", "--option", "yes: resume", "--kind", "stop", "--budget", "1d/10/720m/1/3")
		if code != 0 || result.Outcome != intentConfirmed || len(b.asked) != 1 || result.Next == nil {
			t.Fatalf("ask = %d %+v", code, result)
		}
		box, _ := goal.NewBudget("1d", 10, 720, 1, 3)
		if asked := b.asked[0]; asked.Wants != goal.ResumeApprovalToken("goal-a", box) || asked.Facts[0] != "Resume it?" || asked.Budget != nil {
			t.Fatalf("stop question = %+v", asked)
		}
		if code, result := b.runJSON(b.owners(), "ask", "goal-a", "--question", "Why?", "--option", "a: b", "--budget", "1d/10/720m/1/3"); code != 2 || result.Outcome != intentRefused || len(b.asked) != 1 {
			t.Fatalf("budget without kind = %d %+v", code, result)
		}
	})

	t.Run("readings keep unknown and quoted paths", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		owners := b.owners()
		owners.processes.health = func(string, string, time.Time) steward.HealthVerdict {
			return steward.HealthVerdict{Aggregate: "unknown", Roles: []steward.RoleVerdict{{Role: steward.RoleStewardRunner, Status: steward.HealthUnknown, Reason: "no tick yet", Remedy: "metasystem start"}}}
		}
		code, result := b.runJSON(owners, "doctor")
		if code != 2 || result.Outcome != intentConfirmed {
			t.Fatalf("unknown doctor = %d %+v", code, result)
		}
		_, stdout, _ := b.run(owners, "doctor")
		if !strings.Contains(stdout, "unknown: no tick yet; remedy: metasystem start") {
			t.Fatalf("doctor text dropped the owner remedy: %q", stdout)
		}
		missing := filepath.Join(t.TempDir(), "a dir", "x")
		code, result = b.runJSON(owners, "status", "--repo", missing)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, shellCommand([]string{missing})) {
			t.Fatalf("quoted repo = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "stop", "--by", "a", "--by", "b"); code != 2 || !strings.Contains(result.Summary, "given twice") {
			t.Fatalf("conflict = %d %+v", code, result)
		}
		if b.fence().State != stopfence.StateOpen || b.armCalls != 0 {
			t.Fatal("a refused reading changed the checkout")
		}
	})

	t.Run("old calls keep their handlers", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{{}, {"--repo", "."}, {"--all"}, {"--installation", "x"}} {
			if !legacyProcessCall(args) {
				t.Fatalf("%v left the old handler", args)
			}
		}
		for _, args := range [][]string{{"checkout"}, {"job", "j"}, {"--json"}, {"--help"}} {
			if legacyProcessCall(args) {
				t.Fatalf("%v stayed on the old handler", args)
			}
		}
		if !legacyUICall([]string{"start"}) || !legacyUICall([]string{"serve"}) || legacyUICall(nil) {
			t.Fatal("ui routing")
		}
		if command, _ := findIntentCommand("start"); command.legacy != nil {
			t.Fatal("start has no old top-level call to keep")
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
