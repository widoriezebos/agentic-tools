package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type recoveryStarter struct {
	bed        *workBed
	crash      bool
	childReady chan struct{}
	ended      chan struct{}
	done       chan error
}

func (s *recoveryStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	r, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if r.Kind != "build" {
		return s.bed.starter.StartSupervisor(id, state)
	}
	if s.crash {
		panic("reservation crash")
	}
	go func() { _, err := s.bed.manager.Supervise(id); s.done <- err }()
	<-s.childReady
	if _, err := s.bed.manager.Supervise(id); !launch.IsCode(err, "LAUNCH_ALREADY_SUPERVISED") {
		return workProcessRef(10), errors.New("duplicate supervisor claim was admitted")
	}
	return workProcessRef(10), nil
}

type cancelRecoveryStarter struct{ manager *launch.Manager }

func (s cancelRecoveryStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	record, err := s.manager.Supervise(id)
	if err != nil && !record.State.Terminal() {
		return identity.Ref{}, err
	}
	return workProcessRef(99), nil
}

type recoveryProcesses struct {
	workProcesses
	mu                sync.Mutex
	children, signals int
	s                 *recoveryStarter
}

func (p *recoveryProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	p.mu.Lock()
	p.children++
	n := p.children
	p.mu.Unlock()
	if p.s == nil {
		return nil, identity.Ref{}, errors.New("unexpected child creation")
	}
	if n > 1 {
		close(p.s.ended)
	}
	var ready chan struct{}
	if n == 1 {
		ready = p.s.childReady
	}
	return recoveryChild{ended: p.s.ended, ready: ready}, workProcessRef(20), nil
}
func (p *recoveryProcesses) SignalGroup(int64, syscall.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signals++
	return nil
}
func (p *recoveryProcesses) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.children, p.signals
}

type recoveryChild struct {
	ended <-chan struct{}
	ready chan struct{}
}

func (c recoveryChild) Wait() (int, error) {
	if c.ready != nil {
		close(c.ready)
	}
	<-c.ended
	return 0, nil
}

type unknownRecoveryProber struct{}

func (unknownRecoveryProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Unknown, os.ErrPermission
}

type recoveryAdapter struct {
	workAdapter
	before func()
}

func (a recoveryAdapter) Command(r launch.Record, state string) (launch.Command, error) {
	if a.before != nil {
		a.before()
	}
	return a.workAdapter.Command(r, state)
}

func TestDriverPendingRecoveryPublicStop(t *testing.T) {
	t.Parallel()
	t.Run("reservation restart competing resume and lost response", func(t *testing.T) {
		t.Parallel()
		b := newWorkBed(t)
		s := &recoveryStarter{bed: b, crash: true, childReady: make(chan struct{}), ended: make(chan struct{}), done: make(chan error, 3)}
		p := &recoveryProcesses{s: s}
		b.manager.Supervisor, b.manager.Processes = s, p
		argv := append([]string{"work", "build", b.id, "--work", "recover", "--brief", b.brief("recover.md", "Build the unit.\n"), "--lines", "5"}, workCheck...)
		func() {
			defer func() {
				if got := recover(); got != "reservation crash" {
					t.Fatalf("crash=%v", got)
				}
			}()
			pendingWork(t, b, argv...)
		}()
		runs, unknown, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
		if err != nil || len(unknown) != 0 || len(runs) != 1 {
			t.Fatalf("retained runs=%d unknown=%v error=%v", len(runs), unknown, err)
		}
		run := runs[0].Run
		operation := runs[0].Record.Rounds[0].Steps[0].LaunchID
		reservation, err := b.manager.Store.Read(operation)
		if err != nil || reservation.State != launch.Starting || reservation.Supervisor != nil {
			t.Fatalf("reservation=%s state=%s supervisor=%v error=%v", reservation.ID, reservation.State, reservation.Supervisor, err)
		}
		code, status, _ := pendingWork(t, b, "work", "status", b.id)
		if code != 0 || !strings.Contains(jsonText(status.Data), run) {
			t.Fatalf("status=%d %s", code, status.Summary)
		}
		s.crash = false
		personProof := b.personProof
		b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, errors.New("agent invocation")
		}
		conf := filepath.Join(b.root(), "metasystem.conf")
		settings, err := os.ReadFile(conf)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(conf, append(settings, []byte("\nseat.driver=person\n")...), 0600); err != nil {
			t.Fatal(err)
		}
		pendingWork(t, b, "work", "build", "run:"+run)
		if n, _ := p.counts(); n != 0 {
			t.Fatal("recovery inherited the previous person's authority under person policy")
		}
		b.personProof = personProof
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); pendingWork(t, b, "work", "build", "run:"+run) }()
		}
		wg.Wait()
		if n, _ := p.counts(); n != 1 {
			t.Fatalf("two resumes created %d children", n)
		}
		pendingWork(t, b, "work", "build", "run:"+run) // The first caller's response may be lost.
		after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
		if err != nil || after.Rounds[0].Steps[0].LaunchID != operation || len(after.Rounds[0].Steps[0].LaunchIDs) != 1 {
			t.Fatalf("recovery changed identity: run=%s rounds=%d error=%v", after.ID, len(after.Rounds), err)
		}
		b.manager.Prober = unknownRecoveryProber{}
		person := b.personProof
		code, stopped, _ := pendingWork(t, b, "work", "stop", b.id)
		if code == 0 || stopped.Outcome != intentPartial {
			t.Fatalf("unknown stop claimed success: %d %s", code, stopped.Summary)
		}
		if n, signals := p.counts(); n != 1 || signals != 0 {
			t.Fatalf("unknown identity children=%d signals=%d", n, signals)
		}
		pendingWork(t, b, "work", "build", "run:"+run)
		if n, _ := p.counts(); n != 1 {
			t.Fatal("cancelled run resurrected")
		}
		close(s.ended)
		if err := <-s.done; err != nil {
			t.Fatalf("supervisor: %v", err)
		}
		b.manager.Prober = &treeProber{dead: true}
		b.personProof = person
		for range 2 {
			code, result, _ := pendingWork(t, b, "work", "stop", b.id)
			if code != 0 {
				t.Fatalf("repeat stop: %d %s", code, result.Summary)
			}
		}
	})
	for _, change := range []string{"input", "revoked claim", "stored person", "unreadable reservation"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			person := b.personProof
			b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			conf := filepath.Join(b.root(), "metasystem.conf")
			settings, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf, append(settings, []byte("\nseat.driver=auto\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			waiting := true
			b.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				load := 0.0
				if waiting {
					load = 100
				}
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: load}
			}
			brief := b.brief("identity.md", "Build this unit.\n")
			argv := append([]string{"work", "build", b.id, "--work", "identity", "--brief", brief, "--lines", "5"}, workCheck...)
			code, result, _ := pendingWork(t, b, argv...)
			if code != 1 {
				t.Fatalf("seed wait: %d %s", code, result.Summary)
			}
			run := resultData(t, result)["run"].(string)
			original, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			operation := original.Rounds[0].Steps[0].LaunchID
			waiting = false
			switch change {
			case "target":
				b.head = "other-commit"
			case "unreadable reservation":
				if err := b.manager.Store.Create(launch.Record{ID: operation, Goal: b.id, Kind: "build", WorkingDirectory: b.worktree, State: launch.Starting}); err != nil {
					t.Fatal(err)
				}
			case "input":
				plan, err := launch.ReadUnitPlan(original.Plan)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(plan.Build.Brief, []byte("Changed input.\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "revoked claim":
				file := b.goalFile(b.id)
				file.Claimed = nil
				b.addGoal(file)
			case "stored person":
				original.Operation.Actor = "person"
				original.Rounds[0].Steps[0].Retained.Actor = "person"
				data, err := json.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
				waiting = true
			}
			code, result, _ = pendingWork(t, b, "work", "build", "run:"+run)
			if code == 0 || len(b.starter.launched()) != 0 {
				t.Fatalf("stale request launched: %s %d %s", change, code, result.Summary)
			}
			if _, err := b.manager.Store.Read(operation); change != "unreadable reservation" && !os.IsNotExist(err) {
				t.Fatalf("refusal left reservation: %v", err)
			}
			if change == "stored person" {
				b.personProof = person
				code, result, _ = pendingWork(t, b, "work", "build", "run:"+run)
				if code != 0 || len(b.starter.launched()) == 0 {
					t.Fatalf("fresh person invocation could not recover: %d %s", code, result.Summary)
				}
			}
		})
	}

	for _, when := range []string{"before admission", "last child check"} {
		t.Run(when, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			if when == "before admission" {
				b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("agent")
				}
				b.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
					return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 100}
				}
			} else {
				b.manager.Supervisor = &recoveryStarter{bed: b, crash: true}
			}
			argv := append([]string{"work", "build", b.id, "--work", "cancel", "--brief", b.brief("cancel.md", "Build the unit.\n"), "--lines", "5"}, workCheck...)
			if when == "last child check" {
				func() {
					defer func() {
						if recover() != "reservation crash" {
							t.Fatal("no reservation crash")
						}
					}()
					pendingWork(t, b, argv...)
				}()
			} else {
				code, result, _ := pendingWork(t, b, argv...)
				if code != 1 {
					t.Fatalf("wait: %d %s", code, result.Summary)
				}
			}
			runs, _, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
			if err != nil || len(runs) != 1 {
				t.Fatalf("runs=%d %v", len(runs), err)
			}
			run := runs[0].Run
			person := enrolledPersonProver(t, b.root(), b.manager.Now())
			b.personProof = person
			if when == "before admission" {
				for range 2 {
					code, result, _ := pendingWork(t, b, "work", "stop", b.id)
					if code != 0 {
						t.Fatalf("pending stop: %d %s", code, result.Summary)
					}
				}
				b.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
					return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true}
				}
				code, _, _ := pendingWork(t, b, argv...)
				if code == 0 || len(b.starter.launched()) != 0 {
					t.Fatal("cancel before admission relaunched")
				}
				after, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
				if after.Rounds[0].Steps[0].PendingAct.Waits[0].EndReason != "cancelled" {
					t.Fatal("cancellation did not close wait")
				}
			} else {
				p := &recoveryProcesses{}
				b.manager.Processes = p
				a := recoveryAdapter{before: func() {
					paths, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json.lock"))
					if err != nil || len(paths) != 1 {
						t.Fatalf("tree locks: %v %v", paths, err)
					}
					held, err := lock.File(paths[0], 0600, lock.TryExclusive)
					if err != nil {
						t.Fatal(err)
					}
					defer held.Release()
					code, result, _ := pendingWork(t, b, "work", "stop", "run:"+run)
					if code == 0 {
						t.Fatalf("live supervisor stop claimed quiescence: %s", result.Summary)
					}
				}}
				b.manager.Adapters["codex-exec"], b.manager.Adapters["claude-headless"] = a, a
				b.manager.Supervisor = cancelRecoveryStarter{b.manager}
				pendingWork(t, b, "work", "build", "run:"+run)
				if n, signals := p.counts(); n != 0 || signals != 0 {
					t.Fatalf("cancelled admission children=%d signals=%d", n, signals)
				}
				if _, err := os.Stat(filepath.Join(b.unitRoot, run, "cancelled")); err != nil {
					t.Fatal("stop tombstone missing")
				}
				code, _, _ := pendingWork(t, b, argv...)
				if code == 0 {
					t.Fatal("restart accepted cancelled operation")
				}
			}
		})
	}
}
