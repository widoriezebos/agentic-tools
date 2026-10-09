package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// Real ancestry and proof executions distinguish selected members from later
// hand-ins; the model launcher records its effects without starting a provider.
func TestLandingExecutionAdapterRetriesSelectedBatchUnderHelmAndPause(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "person")
	helmMust(t, os.WriteFile(filepath.Join(b.installation, "go.mod"), []byte("module fixture\n"), 0600))
	a, second := b.seat(t, "a"), b.seat(t, "b")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	_, err := helm.Write(b.checkout, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Checkout: b.checkout, Reason: "manual"})
	helmMust(t, err)
	b.owners.landing.helm = helm.Active
	_, err = lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/claude", ProjectsRoot: t.TempDir()}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return b.now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return b.home, nil })}
	dead := true
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.manager = func() *launch.Manager { return manager }
		agent.now = func() time.Time { return b.now }
		agent.nonce = func() (string, error) { return "scoped", nil }
		agent.machine = func(string) (string, error) { return "fixture", nil }
		agent.settings = func(string) (launch.Settings, error) {
			if dead {
				return launch.Settings{}, errors.New("provider launcher unavailable")
			}
			settings := launch.DefaultSettings()
			settings.LandingRuntime = "claude"
			settings.LandingModel = "fixture-model"
			return settings, nil
		}
	})
	keeper := newLandingAgentKeeper(b.checkout, b.home, agent)
	b.owners.landing.keeper = func(string, string) lane.AgentKeeper { return keeper }
	b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
	// Automatic preparation asks without launching, even at the helm.
	_, _ = b.run(t, "landing", "run")
	questions, unread := channel.WalkOpenQuestions(b.installation)
	if len(unread) != 0 || len(questions) != 1 {
		t.Fatalf("proposal questions: %+v %v", questions, unread)
	}
	q := questions[0]
	command := strings.Fields(channel.LaneStopCommand(q))
	if len(command) == 0 {
		t.Fatal("selection remedy missing")
	}
	code, text := b.run(t, command[1:]...)
	if code != 0 || !strings.Contains(text, "selection recorded, execution not started") || !strings.Contains(text, "landing run") {
		t.Fatalf("dead launcher: %d %s", code, text)
	}
	selected := b.batch(t)
	if selected.Person == nil || selected.Person.StandingPause == nil || !slices.Equal(selected.Members, []plain.GoalSHA{{Goal: "a", SHA: a}, {Goal: "b", SHA: second}}) {
		t.Fatalf("selection: %+v", selected)
	}
	closed, err := channel.ReadQuestion(b.installation, q.ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("recorded effect question: %+v %v", closed, err)
	}
	if len(landingLaunches(t, store)) != 0 {
		t.Fatal("dead launcher claimed execution")
	}
	third := b.seat(t, "c")
	b.git(t, b.checkout, "fetch", "--quiet", "origin")
	dead = false
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, 30, person(), at)
	}
	// A standing provider outage still holds the recorded selection.
	_, err = outage.Observe(b.home, "claude", "landing-model", outage.ProviderLimit, "fixture", launch.LandingOwnerLineage, b.now)
	helmMust(t, err)
	b.success(t, "landing", "run")
	if len(landingLaunches(t, store)) != 0 {
		t.Fatal("Explicit bypassed the provider hold")
	}
	_, err = outage.Observe(b.home, "claude", "landing-model", "", "", "fixture-success", b.now.Add(time.Nanosecond))
	helmMust(t, err)
	b.success(t, "landing", "run")
	launches := landingLaunches(t, store)
	if len(launches) != 1 {
		t.Fatalf("scoped launches: %+v", launches)
	}
	var briefPath string
	helmMust(t, json.Unmarshal(launches[0].AdapterData["brief"], &briefPath))
	brief, err := os.ReadFile(briefPath)
	if err != nil || !strings.Contains(string(brief), selected.ID) || strings.Contains(string(brief), plain.WakeFullDue) {
		t.Fatalf("scoped brief: %s %v", brief, err)
	}
	if b.batch(t).ID != selected.ID || len(b.batch(t).Members) != 2 {
		t.Fatal("retry changed selected membership")
	}
	// The worker's proof invocation has no direct terminal authority.
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("ordinary worker")
	}
	// Drive the worker's public gate/full/push path on those exact real commits.
	b.assemble(t, a)
	b.success(t, "landing", "prove", "--gate", "--wait")
	b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", second)
	b.success(t, "landing", "prove", "--gate", "--wait")
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")
	head := b.git(t, b.checkout, "rev-parse", "origin/main")
	if _, err := plain.Git(b.checkout, "merge-base", "--is-ancestor", third, head); err == nil {
		t.Fatal("unselected later hand-in was pushed")
	}
	if b.executions(t) != 4 {
		t.Fatalf("baseline, two gates and full executions: %d", b.executions(t))
	}
	if _, paused := lane.ReadPause(b.home); !paused || !helm.Active(b.checkout).Active {
		t.Fatal("execution removed a standing fence")
	}
	if queueStates(b.status(t))["c"] != plain.StateWaiting {
		t.Fatal("later member did not remain waiting")
	}
}

func TestLandingExecutionLaterPauseStopsKeeperAndVerbs(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "person")
	a := b.seat(t, "a")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	b.success(t, "landing", "run", "--goals", "a")
	selected := b.batch(t)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	b.assemble(t, a)
	_, err := lane.SetPause(b.home, "Wido", b.now.Add(time.Minute))
	helmMust(t, err)
	before := b.starts
	code, text := b.run(t, "landing", "run")
	if code != 0 || b.starts != before || !strings.Contains(text, "execution not started") || !strings.Contains(text, "metasystem landing start") {
		t.Fatalf("later pause run: %d %s starts=%d", code, text, b.starts)
	}
	var pending intentResult
	helmMust(t, json.Unmarshal([]byte(text), &pending))
	if pending.Next == nil || !slices.Equal(pending.Next.Argv, []string{"metasystem", "landing", "start"}) {
		t.Fatalf("later pause next command: %+v", pending.Next)
	}
	for _, verb := range []string{"prove", "push", "resolve"} {
		code, text = b.run(t, "landing", verb)
		if code == 0 {
			t.Fatalf("later pause admitted %s: %s", verb, text)
		}
	}
	if b.executions(t) != 0 || b.launches != 0 || b.batch(t).ID != selected.ID {
		t.Fatal("later pause changed effects or selection")
	}
	if admission, _ := b.status(t)["admitted-batch"].(string); admission != "" {
		t.Fatalf("status retained canceled admission: %s", admission)
	}
}

func TestLandingExecutionDirectBackgroundProofUnderPause(t *testing.T) {
	t.Parallel()
	for _, trunk := range []bool{false, true} {
		t.Run(map[bool]string{false: "batch", true: "trunk"}[trunk], func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			if !trunk {
				a := b.seat(t, "a")
				b.success(t, "landing", "run")
				b.assemble(t, a)
			}
			_, err := lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			var child []string
			b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
				child = slices.Clone(argv[1:])
				b.launches++
				return int64(os.Getpid()), nil
			}
			args := []string{"landing", "prove"}
			if trunk {
				args = append(args, "--trunk")
			}
			b.success(t, args...)
			if len(child) == 0 || b.launches != 1 || b.executions(t) != 0 {
				t.Fatalf("background start: child=%v launches=%d executions=%d", child, b.launches, b.executions(t))
			}
			running, recorded, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
			if err != nil || !recorded || running.Attempt == "" {
				t.Fatalf("running proof: %+v %v", running, err)
			}
			registered, _, err := lane.Read(b.home)
			helmMust(t, err)
			if running.Person == nil || running.Person.Kind != "proof" || running.Person.Person != "Wido" || running.Person.Root != b.installation ||
				!running.Person.CheckedAt.Equal(b.now) || running.Person.TerminalGeneration == 0 || running.Person.Destination != registered {
				t.Fatalf("direct proof provenance: %+v", running.Person)
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				t.Fatal("background proof tried to re-prove the person")
				return humanauthority.Proof{}, errors.New("agent")
			}
			// The admission belongs to this attempt and proof mode alone.
			for _, bad := range [][]string{
				append(slices.Clone(args), "--wait", "--attempt", "another-attempt"),
				{"landing", "prove", "--wait", "--attempt", running.Attempt, "--gate"},
			} {
				if code, text := b.run(t, bad...); code == 0 || b.executions(t) != 0 {
					t.Fatalf("unmatched background admission: %v: %d %s", bad, code, text)
				}
			}
			b.success(t, child...)
			result, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || result.Result != plain.Green || result.Attempt != running.Attempt || result.Trunk != trunk || b.executions(t) != 1 {
				t.Fatalf("background result: %+v %v %v executions=%d", result, ok, err, b.executions(t))
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("background proof cleared the pause")
			}
		})
	}
}

func TestLandingExecutionDirectDetachedProofUnderPause(t *testing.T) {
	t.Parallel()
	for _, trunk := range []bool{false, true} {
		t.Run(map[bool]string{false: "batch", true: "trunk"}[trunk], func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			if !trunk {
				a := b.seat(t, "a")
				b.success(t, "landing", "run")
				b.assemble(t, a)
			}
			_, err := lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.owners.landing.plainProve = b.detachedEngine(t)
			args := []string{"landing", "prove"}
			if trunk {
				args = append(args, "--trunk")
			}
			b.success(t, args...)
			waitForEngineChildExit(t, b.exited)
			result, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || result.Result != plain.Green || result.Trunk != trunk || b.executions(t) != 1 {
				t.Fatalf("detached result: %+v %v %v executions=%d", result, ok, err, b.executions(t))
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("detached proof cleared the pause")
			}
		})
	}
}

func TestLandingExecutionSelectedBatchDoesNotAdmitAgentTrunkProof(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"admission", "proof-owner"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "person")
			a := b.seat(t, "a")
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			_, err := lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			b.success(t, "landing", "run", "--goals", "a")
			b.assemble(t, a)
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			if boundary == "proof-owner" {
				_, err := lane.ClearPause(b.home)
				helmMust(t, err)
				b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "rev-parse" {
						_, err := lane.SetPause(b.home, "Wido", b.now)
						helmMust(t, err)
					}
					return plain.Git(dir, args...)
				}
			}
			code, text := b.run(t, "landing", "prove", "--trunk", "--wait")
			if code == 0 || b.executions(t) != 0 || b.launches != 0 {
				t.Fatalf("selected batch admitted agent trunk proof: %d %s executions=%d launches=%d", code, text, b.executions(t), b.launches)
			}
			if _, found, err := plain.LastResult(b.installation); err != nil || found {
				t.Fatalf("refused trunk proof wrote a result: found=%v err=%v", found, err)
			}
		})
	}
}

func TestLandingExecutionKeeperRechecksPauseAndReleasesLocks(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"wake", "start", "registration"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
			// Record the choice while readiness holds execution.
			b.owners.landing.ready = func(string) error { return errors.New("not armed") }
			if code, text := b.run(t, b.lane, "landing", "run", "--goals", "a"); code != 0 {
				t.Fatalf("record: %d %s", code, text)
			}
			k := b.keeper
			k.Continuation = func(r lane.Record) string { return plain.PersonBatchContinuation(r.Install, r, b.home) }
			k.Prepare = func(r lane.Record) error {
				// Queue-owner work can acquire the home lock in its normal order.
				queueLock, err := lock.File(filepath.Join(plain.Dir(r.Install), "lane.lock"), 0600, lock.TryExclusive)
				if err != nil {
					return err
				}
				defer queueLock.Release()
				homeLock, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
				if err != nil {
					return err
				}
				return homeLock.Release()
			}
			pause := func() { _, err := lane.SetPause(b.home, "Wido", b.now.Add(time.Minute)); helmMust(t, err) }
			canceled := 0
			k.Cancel = func(string) error { canceled++; return nil }
			if boundary == "wake" {
				k.Sources.Reasons = func(string) ([]string, error) { pause(); return []string{plain.WakeQueued}, nil }
			} else {
				k.Start = func(string, lane.Wake) (string, error) {
					if boundary == "registration" {
						record := b.record
						record.CustodyEpoch++
						data, err := json.Marshal(record)
						helmMust(t, err)
						helmMust(t, os.WriteFile(lane.RecordPath(b.home), data, 0600))
					} else {
						pause()
					}
					b.starts++
					return "agent", nil
				}
			}
			run := k.Run()
			want := lane.AgentPaused
			if boundary == "registration" {
				want = lane.AgentHeld
			}
			if run.Outcome != want || (boundary == "wake" && b.starts != 0) || (boundary != "wake" && canceled != 1) {
				t.Fatalf("%s recheck: %+v starts=%d cancels=%d", boundary, run, b.starts, canceled)
			}
		})
	}
}

func TestLandingExecutionDirectProofDoesNotAdmitLaterPush(t *testing.T) {
	t.Parallel()
	for _, badPause := range []bool{false, true} {
		t.Run(map[bool]string{false: "standing", true: "unreadable"}[badPause], func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			a := b.seat(t, "a")
			b.success(t, "landing", "run")
			b.assemble(t, a)
			_, err := lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			if badPause {
				helmMust(t, os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), []byte("{broken"), 0600))
			}
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.success(t, "landing", "prove", "--wait")
			if b.executions(t) != 1 {
				t.Fatalf("direct executions: %d", b.executions(t))
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			if code, text := b.run(t, "landing", "push"); code == 0 {
				t.Fatalf("direct proof admitted later push: %s", text)
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("direct proof cleared pause")
			}
		})
	}
}

func TestLandingExecutionInFlightProofRetainsEvidenceAfterLaterPause(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "person")
	a := b.seat(t, "a")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	_, err := lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	b.success(t, "landing", "run", "--goals", "a")
	b.assemble(t, a)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	b.owners.landing.plainProve.Command = func(command *exec.Cmd) error {
		helmMust(t, lanePauseAgain(b.home, b.now.Add(time.Minute)))
		return command.Run()
	}
	b.success(t, "landing", "prove", "--wait")
	result, ok, err := plain.LastResult(b.installation)
	if err != nil || !ok || result.Result != plain.Green || b.executions(t) != 1 {
		t.Fatalf("finished evidence: %+v %v %v", result, ok, err)
	}
	if code, text := b.run(t, "landing", "push"); code == 0 {
		t.Fatalf("later pause admitted push: %s", text)
	}
	retained, _, err := plain.LastResult(b.installation)
	if err != nil || retained.Attempt != result.Attempt || retained.Tree != result.Tree {
		t.Fatal("later pause discarded running proof evidence")
	}
}

func lanePauseAgain(home string, at time.Time) error {
	if _, err := lane.ClearPause(home); err != nil {
		return err
	}
	_, err := lane.SetPause(home, "Wido", at)
	return err
}

func TestLandingExecutionPauseBeforeProofOwnerPreventsExecution(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "person")
	a := b.seat(t, "a")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	b.success(t, "landing", "run", "--goals", "a")
	b.assemble(t, a)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	paused := false
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if !paused && args[0] == "rev-parse" {
			paused = true
			_, err := lane.SetPause(b.home, "Wido", b.now.Add(time.Minute))
			helmMust(t, err)
		}
		return plain.Git(dir, args...)
	}
	code, text := b.run(t, "landing", "prove", "--wait")
	if code == 0 || b.executions(t) != 0 || !paused {
		t.Fatalf("pause before proof owner: %d %s executions=%d", code, text, b.executions(t))
	}
}

func TestLandingExecutionDirectTrunkProofUnderHelmAndPause(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "person")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	_, err := helm.Write(b.checkout, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Checkout: b.checkout, Reason: "manual"})
	helmMust(t, err)
	b.owners.landing.helm = helm.Active
	_, err = lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	b.success(t, "landing", "prove", "--trunk", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	if err != nil || !ok || !proof.Trunk || proof.Commit != b.main || proof.Result != plain.Green || b.executions(t) != 1 {
		t.Fatalf("explicit trunk: %+v %v %v checks=%d", proof, ok, err, b.executions(t))
	}
	if _, paused := lane.ReadPause(b.home); !paused || !helm.Active(b.checkout).Active || b.starts != 0 {
		t.Fatal("direct trunk proof removed fences or started an agent")
	}
	if batch, err := plain.ReadBatch(b.installation); err != nil || batch != nil {
		t.Fatalf("direct trunk created batch permission: %+v %v", batch, err)
	}
}

func TestLandingExecutionReadinessKeepsSelectionAndRealRemedy(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	b.owners.landing.ready = func(string) error { return lane.UnarmedRefusal(b.lane) }
	code, text := b.run(t, b.lane, "landing", "run", "--goals", "a")
	if code != 0 || !strings.Contains(text, "selection recorded, execution not started") || !strings.Contains(text, "system start --repo "+b.lane) || b.starts != 0 {
		t.Fatalf("readiness: %d %s starts=%d", code, text, b.starts)
	}
	selected := b.batch(t)
	b.owners.landing.ready = func(string) error { return nil }
	startProofs := 0
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		startProofs++
		return humanauthority.Proof{}, errors.New("the retry is an agent act")
	}
	if code, text := b.run(t, b.lane, "landing", "run"); code != 0 || b.starts != 1 || b.batch(t).ID != selected.ID || startProofs != 1 {
		t.Fatalf("readiness retry: %d %s starts=%d startProofs=%d", code, text, b.starts, startProofs)
	}
}

func TestLandingExecutionPersonProofBelongsToOneInvocation(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	prove := enrolledPersonProver(t, b.lane, b.now)
	proofs, holds := 0, 0
	b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, runtime, sessions string, at time.Time) (humanauthority.Proof, error) {
		proofs++
		if proofs > 1 {
			return humanauthority.Proof{}, errors.New("the later invocation is an agent act")
		}
		return prove(root, pid, reader, runtime, sessions, at)
	}
	b.keeper.ProviderHold = func(string) (string, error) {
		holds++
		return "the model provider is limited", nil
	}
	if code, text := b.run(t, b.lane, "landing", "run", "--goals", "a"); code != 0 || b.starts != 1 || proofs != 1 || holds != 0 {
		t.Fatalf("person selection and start: %d %s starts=%d proofs=%d holds=%d", code, text, b.starts, proofs, holds)
	}
	selected := b.batch(t)
	if code, text := b.run(t, b.lane, "landing", "run", "--batch", selected.ID, "--json"); code != 0 || !strings.Contains(text, "selection recorded, execution not started") || !strings.Contains(text, "provider is limited") || b.starts != 1 || proofs != 2 || holds != 1 || b.batch(t).ID != selected.ID {
		t.Fatalf("agent continuation inherited person authority: %d %s starts=%d proofs=%d holds=%d", code, text, b.starts, proofs, holds)
	}
}
