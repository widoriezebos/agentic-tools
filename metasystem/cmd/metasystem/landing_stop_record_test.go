package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func newStopVerbBed(t *testing.T) *replayVerbBed {
	t.Helper()
	b := newReplayVerbBed(t)
	if err := os.WriteFile(filepath.Join(b.install, "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Check admission reconciles requests before the keeper tick; both use the same machine.
	b.owners.landing.machine = func(string) (string, error) { return "lane-machine", nil }
	return b
}

func syncStopQuestionHold(agent landingAgent, install string) (string, error) {
	if err := plain.SyncPolicyQuestion(install, agent.machine, agent.now()); err != nil {
		return "", err
	}
	return agent.questionHold(install)
}

func TestLandingStopAfterSecondRedShowsCommandAndGenericRunPreservesQuestion(t *testing.T) {
	t.Parallel()
	b := newStopVerbBed(t)
	b.fail = func(_ *exec.Cmd, only string) (string, error) {
		if only == "" {
			return replayFailure, errors.New("red")
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	b.owners.landing.view = func(string) lane.View {
		return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "the landing lane is idle"}
	}
	b.prove(t)
	if questions, _ := channel.WalkQuestions(b.install); len(questions) != 0 {
		t.Fatalf("the allowed repeat asked a question: %+v", questions)
	}
	b.prove(t)
	code, text := b.run(t, b.root, "status", "--json")
	var status struct{ Data plain.Status }
	if code != 0 || json.Unmarshal([]byte(text), &status) != nil || status.Data.Stop == nil {
		t.Fatalf("no recorded stop: %d %s", code, text)
	}
	stop := status.Data.Stop
	command := "metasystem landing return GOAL --cause unclassified --reason TEXT"
	if stop.Loop != "lane-proof" || stop.Attempt != 2 || stop.Budget != 2 || stop.Cause.Kind != "unclassified" || stop.Command() != command || !strings.Contains(stop.Subject, "a") || !strings.Contains(stop.Subject, "b") || stop.Evidence == "" {
		t.Fatalf("stop lost its decision or evidence: %+v", stop)
	}
	code, text = b.run(t, b.root, "status")
	if code != 0 || !strings.Contains(oneSpaced(text), "the lane stopped:") || !strings.Contains(oneSpaced(text), command) || !strings.Contains(oneSpaced(text), "TestBroken") {
		t.Fatalf("stop is not visible: %d %s", code, text)
	}
	// The keeper recovers the request already reconciled by check admission.
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.now = func() time.Time { return laneTestNow }
		agent.machine = func(string) (string, error) { return "lane-machine", nil }
	})
	if hold, err := syncStopQuestionHold(agent, b.install); err != nil || !strings.Contains(hold, command) {
		t.Fatalf("keeper did not ask and hold: %q %v", hold, err)
	}
	questions, _ := channel.WalkOpenQuestions(b.install)
	if len(questions) != 1 || questions[0].Facts[0] != command {
		t.Fatalf("the stop needs one command question: %+v", questions)
	}
	if instructions := channel.ReplyInstructions(questions[0]); !strings.HasPrefix(instructions, "Run "+command) || strings.Contains(instructions, "Reply") {
		t.Fatalf("the stop asks for an answer instead of an act: %q", instructions)
	}
	b.owners.processes.question = channel.ReadQuestion
	action, _ := findIntentAction("question", "show")
	var stdout, stderr bytes.Buffer
	code = runIntentIn(action, []string{"channel:" + questions[0].ID}, &stdout, &stderr, b.install, b.owners)
	if code != 0 || strings.SplitN(strings.TrimSpace(stdout.String()), "\n", 2)[0] != command || strings.Contains(stdout.String(), "question retry") {
		t.Fatalf("question show lost the stop command: %d %s %s", code, &stdout, &stderr)
	}
	if _, err := syncStopQuestionHold(agent, b.install); err != nil {
		t.Fatal(err)
	}
	if all, _ := channel.WalkQuestions(b.install); len(all) != 1 {
		t.Fatalf("a repeated tick asked again: %+v", all)
	}
	b.owners.landing.keeper = func(home, root string) lane.AgentKeeper {
		return lane.AgentKeeper{Home: home, Self: root, Now: agent.now, Running: func() (string, bool, error) { return "running-agent", true, nil }}
	}
	if code, text := b.run(t, b.root, "run"); code != 0 {
		t.Fatalf("landing run: %d %s", code, text)
	}
	if hold, err := syncStopQuestionHold(agent, b.install); err != nil || !strings.Contains(hold, command) {
		t.Fatalf("generic run changed the stopped subject: %q %v", hold, err)
	}
	if q, err := channel.ReadQuestion(b.install, questions[0].ID); err != nil || q.State == "closed" {
		t.Fatalf("generic run closed the question without its effect: %+v %v", q, err)
	}
	if result, ok, err := plain.LastResult(b.install); err != nil || !ok || result.LoopClosed {
		t.Fatalf("generic run reopened proof allowance: %+v %v", result, err)
	}
}

func TestLandingStopCauseChoosesTheHandoff(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"own", "main", "environment"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStopVerbBed(t)
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				if kind == "environment" {
					return "LANDING-NOT-RUN\tbusy\n", errors.New("busy")
				}
				_, exists := os.Stat(filepath.Join(cmd.Dir, "b"))
				if only == "" || kind == "main" || exists == nil {
					return replayFailure, errors.New("red")
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			b.prove(t)
			if kind == "environment" {
				b.prove(t)
			}
			stop, err := plain.NewestStop(b.install)
			if err != nil || stop == nil || stop.Cause.Kind != kind {
				t.Fatalf("no attributed stop: %+v %v", stop, err)
			}
			agent := newTestLandingAgent(func(agent *landingAgent) {
				agent.now = func() time.Time { return laneTestNow }
				agent.machine = func(string) (string, error) { return "lane-machine", nil }
			})
			if _, err := syncStopQuestionHold(agent, b.install); err != nil {
				t.Fatal(err)
			}
			questions, _ := channel.WalkQuestions(b.install)
			switch kind {
			case "own":
				if stop.Handoff != "return b own" || stop.Command() != "metasystem landing return b --cause own --reason TEXT" || stop.Attempt != 1 || len(questions) != 0 {
					t.Fatalf("own handoff: %+v questions=%+v", stop, questions)
				}
			case "main":
				if !strings.HasPrefix(stop.Handoff, "hold ") || len(questions) != 0 {
					t.Fatalf("main must hold without a lane question: %+v questions=%+v", stop, questions)
				}
			case "environment":
				// Decision 4 requires the check act; generic run grants no repeat authority.
				if stop.Attempt != 0 || stop.Command() != "metasystem landing prove" || len(questions) != 1 || questions[0].Facts[0] != stop.Command() {
					t.Fatalf("environment spent a full attempt or chose a return: %+v questions=%+v", stop, questions)
				}
			}
		})
	}
}

func TestLandingStopQuestionClosesOnReturnOrHandInAndNotAnAnswer(t *testing.T) {
	t.Parallel()
	for _, act := range []string{"return", "hand-in"} {
		t.Run(act, func(t *testing.T) {
			t.Parallel()
			b := newStopVerbBed(t)
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				_, exists := os.Stat(filepath.Join(cmd.Dir, "b"))
				if only == "" || exists == nil {
					return replayFailure, errors.New("red")
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			b.prove(t)
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\nlanding.on-red=person\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if code, out := b.run(t, b.root, "return", "b", "--cause", "own"); code != 1 || !strings.Contains(out, "the return needs a person") {
				t.Fatalf("return request: %d %s", code, out)
			}
			// The observer synchronizes requests from fresh lane inputs (lane-reads-its-policies.md:140).
			agent := newTestLandingAgent(func(agent *landingAgent) {
				agent.proofEffects = b.owners.landing.plainProve
				agent.now = func() time.Time { return laneTestNow }
				agent.machine = func(string) (string, error) { return "lane-machine", nil }
			})
			if _, err := syncStopQuestionHold(agent, b.install); err != nil {
				t.Fatal(err)
			}
			questions, _ := channel.WalkOpenQuestions(b.install)
			if len(questions) != 1 || channel.LaneStopCommand(questions[0]) != "metasystem landing return b --cause own --reason TEXT" {
				t.Fatalf("questions=%+v", questions)
			}
			q := questions[0]
			q.State, q.Answer = "answered", &channel.Answer{Text: "return b"}
			data, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b.install, "artifacts", "agents", "channel", "questions", q.ID+".json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			keeper := newLandingAgentKeeper(b.root, b.home, agent)
			// This keeper continues the selection already proved by the fixture.
			keeper.Prepare = func(record lane.Record) error {
				_, err := plain.SelectBatch(record.Install, record.Root, record, b.owners.landing.plainProve)
				return err
			}
			keeper.Running = func() (string, bool, error) { return "", false, nil }
			keeper.Sources.Reasons = func(string) ([]string, error) { return []string{"queued"}, nil }
			keeper.Fingerprint = nil
			keeper.Holds[0] = func(string) (string, error) { return "", nil }
			keeper.Waiting = nil
			keeper.Start = func(string, lane.Wake) (string, error) { t.Fatal("an answer lifted the stop"); return "", nil }
			if run := keeper.Run(); run.Outcome != lane.AgentHeld || !strings.Contains(run.Line, "landing return") {
				t.Fatalf("an answer lifted the stop: %+v", run)
			}
			if act == "return" {
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
				b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
				if code, out := b.run(t, b.root, "return", "b", "--cause", "own", "--reason", "person chose b"); code != 0 {
					t.Fatalf("person return: %d %s", code, out)
				}
			} else if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "b", SHA: "new-sha"}); err != nil {
				t.Fatal(err)
			}
			// Reconciliation must happen even when the next launch is still alive.
			keeper.Running = func() (string, bool, error) { return "running-agent", true, nil }
			keeper.Run()
			if ended, err := channel.ReadQuestion(b.install, q.ID); err != nil || ended.State != "closed" {
				t.Fatalf("the later act did not withdraw: %+v %v", ended, err)
			}
			if open, err := plain.OpenStops(b.install); err != nil || len(open) != 0 {
				t.Fatalf("the goal act left its proof or return stop open: %+v %v", open, err)
			}
		})
	}
}

func TestLandingStopBarrenHoldRecordsOneQuestionAndSelectionClearsIt(t *testing.T) {
	t.Parallel()
	b, keeper, now, starts, _ := landingRestartBed(t)
	install := b.landingA
	if _, _, err := plain.HandIn(install, plain.Line{Goal: "a", SHA: "sha-a"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if run := keeper.Run(); run.Outcome != lane.AgentStarted {
			t.Fatalf("launch: %+v", run)
		}
		b.alive = false
		*now = now.Add(time.Minute)
	}
	for range 2 {
		if run := keeper.Run(); run.Outcome != lane.AgentHeld || *starts != 2 {
			t.Fatalf("barren hold changed: %+v starts=%d", run, *starts)
		}
	}
	stop, err := plain.NewestStop(install)
	questions, _ := channel.WalkOpenQuestions(install)
	if err != nil || stop == nil || stop.Loop != "lane-return" || stop.Attempt != 2 || stop.Measure.Name != "lane fingerprint" || stop.Command() != "metasystem landing run --goals a" || stop.Evidence != lane.AgentStatePath(b.home) || len(questions) != 1 || questions[0].Facts[0] != stop.Command() {
		t.Fatalf("barren stop: %+v %v questions=%+v", stop, err, questions)
	}
	b.prove = enrolledPersonProver(t, b.landingA, *now)
	if code, stdout, stderr := b.run(t, "landing", "run", "--goals", "a"); code != 0 || *starts != 3 {
		t.Fatalf("run did not lift the hold: %d %s %s starts=%d", code, stdout, stderr, *starts)
	}
	if q, err := channel.ReadQuestion(install, questions[0].ID); err != nil || q.State != "closed" {
		t.Fatalf("run did not withdraw the barren question: %+v %v", q, err)
	}
}

func TestLandingUnreadableUnrelatedQuestionAllowsKeeperAndRun(t *testing.T) {
	t.Parallel()
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%v", explicit), func(t *testing.T) {
			t.Parallel()
			b, keeper, _, starts, _ := landingRestartBed(t)
			queueRestartWork(t, b)
			dir := filepath.Join(b.landingA, "artifacts", "agents", "channel", "questions")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "q-x.json"), []byte("{invalid"), 0o600); err != nil {
				t.Fatal(err)
			}
			if explicit {
				if code, out, stderr := b.run(t, "landing", "run"); code != 0 {
					t.Fatalf("run: %d %s %s", code, out, stderr)
				}
			} else if run := keeper.Run(); run.Outcome != lane.AgentStarted {
				t.Fatalf("keeper: %+v", run)
			}
			if *starts != 1 {
				t.Fatalf("starts=%d", *starts)
			}
		})
	}
}

func TestLandingUnreadableOwnStopQuestionHoldsKeeperButAllowsSelection(t *testing.T) {
	t.Parallel()
	b, keeper, now, starts, _ := landingRestartBed(t)
	queueRestartWork(t, b)
	dir := filepath.Join(b.landingA, "artifacts", "agents", "channel", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "q-x.json"), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if run := keeper.Run(); run.Outcome != lane.AgentStarted {
			t.Fatalf("start: %+v", run)
		}
		b.alive = false
		*now = now.Add(time.Minute)
	}
	if run := keeper.Run(); run.Outcome != lane.AgentHeld {
		t.Fatalf("hold: %+v", run)
	}
	questions, _ := channel.WalkQuestions(b.landingA)
	if len(questions) != 1 {
		t.Fatalf("questions=%+v", questions)
	}
	path := filepath.Join(b.landingA, "artifacts", "agents", "channel", "questions", questions[0].ID+".json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || !strings.Contains(run.Line, questions[0].ID) {
		t.Fatalf("own unreadable: %+v", run)
	}
	b.prove = enrolledPersonProver(t, b.landingA, *now)
	if code, out, stderr := b.run(t, "landing", "run", "--goals", "first,second"); code != 0 || *starts != 3 {
		t.Fatalf("explicit run: %d %s %s starts=%d", code, out, stderr, *starts)
	}
}

func TestLandingKeeperQuestionHoldDoesNotTakeInstallLock(t *testing.T) {
	t.Parallel()
	b, keeper, _, _, _ := landingRestartBed(t)
	if err := os.MkdirAll(plain.Dir(b.landingA), 0o755); err != nil {
		t.Fatal(err)
	}
	install, err := lock.File(filepath.Join(plain.Dir(b.landingA), "lane.lock"), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer install.Release()
	home, err := lock.File(lane.LockPath(b.home), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Release()
	done := make(chan error, 1)
	go func() { _, err := keeper.Holds[len(keeper.Holds)-1](b.landingA); done <- err }()
	var holdErr error
	testenv.AwaitOr(t, "the question hold to finish while both locks are held", func() bool {
		select {
		case holdErr = <-done:
			return true
		default:
			return false
		}
	}, func() string {
		_ = home.Release()
		_ = install.Release()
		<-done
		return "the question hold waited for the install lock while the home lock was held"
	})
	if holdErr != nil {
		t.Fatal(holdErr)
	}
}
