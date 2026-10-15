package plain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingAgentStatusRealReturnedQueue(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	data, err := os.ReadFile("testdata/agent-turn/queue.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(Dir(b.install), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queuePath(b.install), data, 0600); err != nil {
		t.Fatal(err)
	}
	since := "2026-10-11T14:30:00Z"
	b.view.Owner = lane.OwnerView{State: lane.OwnerRunning, Since: &since}
	now, _ := time.Parse(time.RFC3339, "2026-10-11T15:55:00Z")
	status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return now }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }, Git: func(string, ...string) (string, error) { return "main", nil }}, b.git(nil, nil))
	if len(status.Queue) != 1 || status.Queue[0].State != StateReturned || status.Queue[0].ReturnedAt != "2026-10-11T14:59:49Z" {
		t.Fatalf("queue: %+v", status.Queue)
	}
	if status.AgentProgress == nil || status.AgentProgress.Line != "agent: running, no step for 55 min" || status.AgentProgress.Finding == "" || status.Stop != nil {
		t.Fatalf("status: %+v", status)
	}
}

func TestLandingAgentStepLiveAndGone(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{true, false} {
		t.Run(map[bool]string{true: "live", false: "gone"}[live], func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			at, _ := time.Parse(time.RFC3339, "2026-10-11T14:32:00Z")
			b.lines("agent-step.json", AgentStep{Verb: "prove", Args: []string{"--gate"}, StartedAt: at.Format(time.RFC3339), PID: 4242})
			progress, err := agentProgress(b.install, b.view.Owner, ProveSeams{Now: func() time.Time { return at.Add(41 * time.Minute) }, Alive: func(r Running) bool {
				if r.Pid != 4242 {
					t.Errorf("pid=%d", r.Pid)
				}
				return live
			}})
			want := "agent: prove --gate since " + at.In(time.Local).Format("15:04") + " (41 min)"
			if !live {
				want += ", process gone"
			}
			if err != nil || progress.Line != want || progress.ProcessGone == live {
				t.Fatalf("progress=%+v error=%v", progress, err)
			}
		})
	}
}

func TestLandingKeeperEmptyBatchIdleBoundary(t *testing.T) {
	t.Parallel()
	for _, minutes := range []int{9, 10, 11} {
		t.Run(time.Duration(minutes).String(), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			at, _ := time.Parse(time.RFC3339, "2026-10-11T14:59:49Z")
			b.lines("agent-idle.json", agentIdle{EndedAt: at.Format(time.RFC3339)})
			p, err := agentProgress(b.install, lane.OwnerView{State: lane.OwnerRunning}, ProveSeams{Now: func() time.Time { return at.Add(time.Duration(minutes) * time.Minute) }})
			if err != nil {
				t.Fatal(err)
			}
			s := Status{AgentProgress: p, AgentAlive: true, Queue: []Entry{{State: StateReturned}}}
			if err := s.agentIdleFinding(b.install, ProveSeams{}); err != nil {
				t.Fatal(err)
			}
			if (p.Finding != "") != (minutes > 10) {
				t.Fatalf("%d min: %+v", minutes, p)
			}
			// A waiting member and an executing step each suppress the finding.
			p.Finding = ""
			s.Queue[0].State = StateWaiting
			if err := s.agentIdleFinding(b.install, ProveSeams{}); err != nil || p.Finding != "" {
				t.Fatalf("waiting: %+v %v", p, err)
			}
			s.Queue[0].State = StateReturned
			p.Step = &AgentStep{Verb: "push"}
			if err := s.agentIdleFinding(b.install, ProveSeams{}); err != nil || p.Finding != "" {
				t.Fatalf("step: %+v %v", p, err)
			}
		})
	}
}

func TestLandingAgentStepLifecycleAndSanitisation(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	at := time.Now().UTC().Truncate(time.Second)
	finish, err := BeginAgentStep(install, "prove", []string{"--gate", "--classify", "secret-attempt", "--reason=secret", "--reason", "secret"}, at)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(Dir(install), "agent-step.json"))
	if err != nil || strings.Contains(string(data), "secret") || !strings.Contains(string(data), "--gate") {
		t.Fatalf("record: %s %v", data, err)
	}
	p, err := agentProgress(install, lane.OwnerView{}, ProveSeams{Now: func() time.Time { return at }})
	if err != nil || p.ProcessGone {
		t.Fatalf("real process: %+v %v", p, err)
	}
	if err := finish(at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Dir(install), "agent-step.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("step not cleared: %v", err)
	}
	p, err = agentProgress(install, lane.OwnerView{}, ProveSeams{Now: func() time.Time { return at.Add(2 * time.Minute) }})
	if err != nil || p.Line != "agent: idle since "+at.Add(time.Minute).In(time.Local).Format("15:04") || p.Since != at.Add(time.Minute).Format(time.RFC3339) {
		t.Fatalf("idle: %+v %v", p, err)
	}
	if p.LastStep == nil || p.LastStep.Verb != "prove" || p.LastStep.StartedAt != at.Format(time.RFC3339) {
		t.Fatalf("the completed step was lost: %+v", p)
	}
}

func TestLandingAgentStepUnreadableIsAProblem(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.unreadable("agent-step.json")
	status := readStatus(b.home, b.record, b.view, ProveSeams{Git: func(string, ...string) (string, error) { return "main", nil }}, b.git(nil, nil))
	if !strings.Contains(strings.Join(status.Problems, " "), "agent step cannot be read") || status.AgentProgress != nil {
		t.Fatalf("status: %+v", status)
	}
}

func TestLandingAgentStepExitKeepsLaterOwner(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	at := time.Now().UTC().Truncate(time.Second)
	finish, err := BeginAgentStep(install, "prove", nil, at)
	if err != nil {
		t.Fatal(err)
	}
	later := AgentStep{Verb: "push", Args: []string{}, PID: int64(os.Getpid() + 1), StartedAt: at.Add(time.Minute).Format(time.RFC3339)}
	if err := writeAgentRecord(install, "agent-step.json", later); err != nil {
		t.Fatal(err)
	}
	if err := finish(at.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	var got AgentStep
	if err := readAgentRecord(install, "agent-step.json", &got); err != nil || got.Verb != "push" || got.PID != later.PID {
		t.Fatalf("later step erased: %+v %v", got, err)
	}
}

func TestLandingAgentIdleConfiguredThresholdAndInvalidValue(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"12", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("landing.agent-idle-minutes="+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			status := Status{AgentAlive: true, AgentProgress: &AgentProgress{Minutes: 11}}
			err := status.agentIdleFinding(install, ProveSeams{})
			if value == "invalid" {
				if err == nil {
					t.Fatal("invalid minutes accepted")
				}
				return
			}
			if err != nil || status.AgentProgress.Finding != "" {
				t.Fatalf("configured bound ignored: %+v %v", status.AgentProgress, err)
			}
			status.AgentProgress.Minutes = 13
			if err := status.agentIdleFinding(install, ProveSeams{}); err != nil || status.AgentProgress.Finding == "" {
				t.Fatalf("configured bound not exceeded: %+v %v", status.AgentProgress, err)
			}
		})
	}
}

func TestLandingAgentRetainedStepKeepsKeeperAgeAndWait(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "dead", true: "live"}[live], func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			started := time.Date(2026, 10, 13, 10, 26, 0, 0, time.UTC)
			since := started.Format(time.RFC3339)
			stepAt := started.Add(-time.Hour)
			b.lines("agent-step.json", AgentStep{Verb: "prove", StartedAt: stepAt.Format(time.RFC3339), PID: 4242})
			p, err := agentProgress(b.install, lane.OwnerView{State: lane.OwnerRunning, Since: &since}, ProveSeams{
				Now:   func() time.Time { return started.Add(21 * time.Minute) },
				Alive: func(Running) bool { return live },
			})
			if err != nil || p == nil || p.Since != since || p.Source != "landing-agent-keeper.json" || p.Step == nil || p.LastStep == nil || p.Step.Verb != "prove" || p.ProcessGone == live || p.Minutes != 81 || !strings.Contains(p.Line, "prove since") {
				t.Fatalf("retained step: %+v err=%v", p, err)
			}
		})
	}
}
