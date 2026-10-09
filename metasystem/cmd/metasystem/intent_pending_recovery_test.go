package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// superviseUnitFixture uses the public supervisor handler with the fixture's authority owners.
func superviseUnitFixture(source *launch.Manager, owners intentOwners, id string) (launch.Record, error) {
	manager := *source
	var stderr bytes.Buffer
	code := runLaunchSuperviseIn([]string{"--id", id}, io.Discard, &stderr, func() *launch.Manager { return &manager }, owners)
	record, err := source.Store.Read(id)
	if err == nil && code != 0 {
		err = fmt.Errorf("supervisor exit %d: %s", code, stderr.String())
	}
	return record, err
}

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
	manager := *s.bed.manager
	go func() {
		code := runLaunchSuperviseIn([]string{"--id", id}, io.Discard, io.Discard, func() *launch.Manager { return &manager }, s.bed.workOwners())
		var err error
		if code != 0 {
			err = fmt.Errorf("supervisor exit %d", code)
		}
		s.done <- err
	}()
	<-s.childReady
	if _, err := s.bed.manager.Supervise(id); !launch.IsCode(err, "LAUNCH_ALREADY_SUPERVISED") {
		return workProcessRef(10), errors.New("duplicate supervisor claim was admitted")
	}
	return workProcessRef(10), nil
}

// deferredRecoveryStarter returns while the supervisor is still waiting to claim.
type deferredRecoveryStarter struct {
	bed     *workBed
	release chan struct{}
	done    chan int
	started bool
}

func (s *deferredRecoveryStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind != "build" {
		return s.bed.starter.StartSupervisor(id, state)
	}
	s.started = true
	go func() {
		<-s.release
		manager := *s.bed.manager
		// The detached supervisor cannot recover proof from its former parent.
		owners := s.bed.workOwners()
		owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, errors.New("parent invocation has ended")
		}
		s.done <- runLaunchSuperviseIn([]string{"--id", id}, io.Discard, io.Discard, func() *launch.Manager { return &manager }, owners)
	}()
	return workProcessRef(10), nil
}

type cancelRecoveryStarter struct {
	manager *launch.Manager
	bed     *workBed
}

func (s cancelRecoveryStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	if s.bed != nil {
		record, err := s.manager.Store.Read(id)
		if err != nil {
			return identity.Ref{}, err
		}
		if record.Kind != "build" {
			return s.bed.starter.StartSupervisor(id, "")
		}
	}
	if s.bed != nil {
		manager := *s.manager
		runLaunchSuperviseIn([]string{"--id", id}, io.Discard, io.Discard, func() *launch.Manager { return &manager }, s.bed.workOwners())
		return workProcessRef(99), nil
	}
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
	for _, change := range []string{"input", "target", "registration", "revoked approval", "revoked claim", "stored person", "unreadable reservation"} {
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
			case "registration":
				root := t.TempDir()
				if _, _, err := lane.Register(b.manager.CapacityHome, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", b.manager.Now()); err != nil {
					t.Fatal(err)
				}
			case "revoked approval":
				file := b.goalFile(b.id)
				file.Approved = nil
				b.addGoal(file)
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
			if change == "unreadable reservation" {
				after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
				pending := launch.PendingWork(after)
				if err != nil || pending == nil || pending.Code != "UNIT_LAUNCH_HELD" || after.Rounds[0].Steps[0].State != launch.StepStarting {
					t.Fatalf("unreadable reservation lost its hold: pending=%v error=%v", pending != nil, err)
				}
			}
			if change == "revoked approval" || change == "revoked claim" {
				after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
				pending := launch.PendingWork(after)
				if err != nil || pending == nil || pending.Code != "UNIT_LAUNCH_UNAUTHORIZED" {
					t.Fatalf("current authority hold missing: pending=%v error=%v", pending != nil, err)
				}
			}
			if change == "target" || change == "registration" || change == "input" {
				after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
				if err != nil || after.Rounds[0].Steps[0].PendingAct.Waits[0].EndReason != "invalidated" {
					t.Fatalf("invalidation not retained: %v", err)
				}
				// Restoring the old facts cannot revive an invalidated operation.
				b.head = original.Base
				code, held, _ := pendingWork(t, b, "work", "build", "run:"+run)
				if code == 0 || len(b.starter.launched()) != 0 || held.Next == nil || !strings.Contains(held.Next.Reason, "fresh") {
					t.Fatalf("invalidated operation resumed or has no preparation remedy: %d %s next=%+v", code, held.Summary, held.Next)
				}
				b.personProof = person
				code, stopped, _ := pendingWork(t, b, held.Next.Argv[1:]...)
				if code != 0 {
					t.Fatalf("preparation stop: %d %s", code, stopped.Summary)
				}
				data, err := os.ReadFile(filepath.Join(b.root(), "artifacts", "agents", "jobs", operation+".json"))
				var spending map[string]any
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &spending); err != nil {
					t.Fatal(err)
				}
				if spending["unitExecuted"] != false || spending["status"] != "cancelled" {
					t.Fatalf("stopped request kept its execution charge: status=%v executed=%v", spending["status"], spending["unitExecuted"])
				}
				b.manager.Sleep(time.Second)
				code, repeated, _ := pendingWork(t, b, held.Next.Argv[1:]...)
				if code != 0 {
					t.Fatalf("repeat preparation stop: %d %s", code, repeated.Summary)
				}
				fresh := append([]string{"work", "build", b.id, "--work", "fresh", "--brief", brief, "--lines", "5"}, workCheck...)
				code, built, _ := pendingWork(t, b, fresh...)
				if code != 0 || len(b.starter.launched()) == 0 {
					t.Fatalf("fresh preparation failed: %d %s", code, built.Summary)
				}
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
				b.manager.Supervisor = cancelRecoveryStarter{manager: b.manager, bed: b}
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
	for _, authority := range []string{"revoked approval", "revoked claim", "person policy", "invocation scoped person", "unreadable authority", "automatic person terminal"} {
		t.Run("last authority "+authority, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			person := b.personProof
			agent := func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			conf := filepath.Join(b.root(), "metasystem.conf")
			settings, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf, append(settings, []byte("\nseat.driver=auto\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			personInvocation := false
			b.personProof = func(root string, pid int64, reader humanauthority.Reader, word, review string, now time.Time) (humanauthority.Proof, error) {
				if personInvocation {
					return person(root, pid, reader, word, review, now)
				}
				return agent(root, pid, reader, word, review, now)
			}
			b.manager.Supervisor = &recoveryStarter{bed: b, crash: true}
			argv := append([]string{"work", "build", b.id, "--work", "final-authority", "--brief", b.brief("authority.md", "Build this unit.\n"), "--lines", "5"}, workCheck...)
			func() {
				defer func() {
					if recover() != "reservation crash" {
						t.Fatal("no reservation crash")
					}
				}()
				pendingWork(t, b, argv...)
			}()
			runs, _, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
			if err != nil || len(runs) != 1 {
				t.Fatalf("runs=%d error=%v", len(runs), err)
			}
			run, operation := runs[0].Run, runs[0].Record.Rounds[0].Steps[0].LaunchID
			// Recovery upgrades an older reservation's authority context without retaining a permit.
			if _, err := b.manager.Store.Update(operation, func(r *launch.Record) error { delete(r.AdapterData, "unitAuthorityRoot"); return nil }); err != nil {
				t.Fatal(err)
			}
			ended := make(chan struct{})
			close(ended)
			p := &recoveryProcesses{s: &recoveryStarter{ended: ended}}
			b.manager.Processes = p
			b.manager.Supervisor = cancelRecoveryStarter{manager: b.manager, bed: b}
			if authority == "invocation scoped person" {
				personInvocation = true
			}
			a := recoveryAdapter{before: func() {
				switch authority {
				case "unreadable authority":
					if _, err := b.manager.Store.Update(operation, func(r *launch.Record) error { r.AdapterData["unitAuthorityRoot"] = json.RawMessage(`""`); return nil }); err != nil {
						t.Fatal(err)
					}
				case "revoked approval", "automatic person terminal":
					if authority == "automatic person terminal" {
						personInvocation = true
					}
					file := b.goalFile(b.id)
					file.Approved = nil
					b.addGoal(file)
				case "revoked claim":
					file := b.goalFile(b.id)
					file.Claimed = nil
					b.addGoal(file)
				case "person policy", "invocation scoped person":
					if err := os.WriteFile(conf, append(settings, []byte("\nseat.driver=person\n")...), 0600); err != nil {
						t.Fatal(err)
					}
					personInvocation = false
				}
			}}
			b.manager.Adapters["codex-exec"], b.manager.Adapters["claude-headless"] = a, a
			code, held, _ := pendingWork(t, b, "work", "build", "run:"+run)
			if authority == "invocation scoped person" {
				// The proof belongs to this invocation even after its parent has exited.
				execution, err := b.manager.Store.Read(operation)
				if n, signals := p.counts(); code != 3 || held.Outcome != intentInProgress || n != 1 || signals != 0 || err != nil || execution.Child == nil {
					t.Fatalf("person invocation lost authority: code=%d children=%d error=%v %s", code, n, err, held.Summary)
				}
				return
			}
			if n, signals := p.counts(); code == 0 || n != 0 || signals != 0 {
				t.Fatalf("final authority bypassed: code=%d children=%d signals=%d %s", code, n, signals, held.Summary)
			}
			execution, err := b.manager.Store.Read(operation)
			after, readErr := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			if err != nil || readErr != nil || execution.State != launch.Starting || execution.Supervisor != nil || execution.Child != nil || after.Rounds[0].Steps[0].State != launch.StepStarting {
				t.Fatalf("authority hold lost its reservation: state=%s error=%v read=%v", execution.State, err, readErr)
			}
			if authority != "revoked claim" {
				// Recovery is a separate proved invocation, even with approval or advisory policy held.
				personInvocation = true
				b.manager.Supervisor = cancelRecoveryStarter{manager: b.manager, bed: b}
				b.manager.Adapters["codex-exec"], b.manager.Adapters["claude-headless"] = workAdapter{}, workAdapter{}
				code, built, _ := pendingWork(t, b, "work", "build", "run:"+run)
				execution, err = b.manager.Store.Read(operation)
				if n, _ := p.counts(); code != 0 || n != 1 || err != nil || execution.Child == nil {
					t.Fatalf("person recovery failed: code=%d children=%d error=%v %s", code, n, err, built.Summary)
				}
			}
		})
	}
	t.Run("unknown process never reaped", func(t *testing.T) {
		t.Parallel()
		b := newWorkBed(t)
		b.starter.hold = "build"
		argv := append([]string{"work", "build", b.id, "--work", "unknown", "--brief", b.brief("unknown.md", "Build this unit.\n"), "--lines", "5"}, workCheck...)
		_, result, _ := pendingWork(t, b, argv...)
		run := resultData(t, result)["run"].(string)
		original, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
		if err != nil {
			t.Fatal(err)
		}
		operation := original.Rounds[0].Steps[0].LaunchID
		ref := workProcessRef(20)
		if _, err := b.manager.Store.Update(operation, func(r *launch.Record) error { r.ProcessGroup = &ref; return nil }); err != nil {
			t.Fatal(err)
		}
		p := &recoveryProcesses{}
		b.manager.Processes, b.manager.Prober = p, unknownRecoveryProber{}
		for range 2 {
			pendingWork(t, b, "work", "status", b.id)
			pendingWork(t, b, "work", "build", "run:"+run)
			code, stopped, _ := pendingWork(t, b, "work", "stop", b.id)
			if code == 0 || stopped.Outcome != intentPartial {
				t.Fatalf("unknown process stop succeeded: %d %s", code, stopped.Summary)
			}
			record, err := b.manager.Store.Read(operation)
			if err != nil || record.State != launch.Running || record.Supervisor == nil || record.Child == nil || record.ProcessGroup == nil || record.Child.Pid != 20 || record.FinishedAt != "" {
				t.Fatalf("unknown identity reaped or removed: state=%s error=%v", record.State, err)
			}
			if children, signals := p.counts(); children != 0 || signals != 0 {
				t.Fatalf("unknown identity children=%d signals=%d", children, signals)
			}
			if _, err := os.Stat(filepath.Join(b.unitRoot, run, "run.json")); err != nil {
				t.Fatalf("run deleted: %v", err)
			}
		}
	})

}

func TestDriverPendingRecoveryUnclaimedProof(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"admission refused", "capacity refused", "interrupted resume"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			person := b.personProof
			b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			conf := filepath.Join(b.root(), "metasystem.conf")
			settings, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf, append(append([]byte(nil), settings...), []byte("\nseat.driver=auto\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			b.manager.Supervisor = &recoveryStarter{bed: b, crash: true}
			argv := append([]string{"work", "build", b.id, "--work", "unclaimed-proof", "--brief", b.brief("unclaimed.md", "Build this unit.\n"), "--lines", "5"}, workCheck...)
			interrupt := func(args ...string) {
				t.Helper()
				defer func() {
					if got := recover(); got != "reservation crash" {
						t.Fatalf("interruption=%v", got)
					}
				}()
				pendingWork(t, b, args...)
			}
			interrupt(argv...)
			runs, _, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
			if err != nil || len(runs) != 1 {
				t.Fatalf("runs=%d error=%v", len(runs), err)
			}
			run, operation := runs[0].Run, runs[0].Record.Rounds[0].Steps[0].LaunchID
			b.personProof = person
			if err := os.WriteFile(conf, append(append([]byte(nil), settings...), []byte("\nseat.driver=person\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "admission refused":
				b.manager.Settings.BuildLinesCap = 1
			case "capacity refused":
				if err := os.Remove(filepath.Join(b.manager.Store.Root, "build-admission.lock")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(b.manager.Store.Root, "build-admission.lock"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "admission refused" {
				spec := *runs[0].Record.Rounds[0].Steps[0].Retained
				seed, err := b.manager.Store.Read(operation)
				if err != nil {
					t.Fatal(err)
				}
				spec.AdapterData = seed.AdapterData
				spec.ID, spec.Actor = operation, "person"
				spec.AdapterData["unitPersonInvocation"] = json.RawMessage(`true`)
				if _, err := b.manager.ResumePending(spec); !launch.IsCode(err, "LAUNCH_BUILD_OVERSIZE") {
					t.Fatalf("admission refusal=%v", err)
				}
			} else if failure == "interrupted resume" {
				interrupt("work", "build", "run:"+run)
			} else if code, result, _ := pendingWork(t, b, "work", "build", "run:"+run); code != 1 {
				t.Fatalf("refused recovery: code=%d %s", code, result.Summary)
			}
			execution, err := b.manager.Store.Read(operation)
			if err != nil || execution.State != launch.Starting || execution.Supervisor != nil || execution.Child != nil {
				t.Fatalf("unclaimed reservation=%+v error=%v", execution, err)
			}
			if _, present := execution.AdapterData["unitPersonInvocation"]; present {
				t.Error("unsupervised recovery retained the person's proof")
			}
			b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("the person invocation ended")
			}
			b.manager.Settings.BuildLinesCap = launch.DefaultSettings().BuildLinesCap
			ended := make(chan struct{})
			close(ended)
			p := &recoveryProcesses{s: &recoveryStarter{ended: ended}}
			b.manager.Processes = p
			var stderr bytes.Buffer
			code := runLaunchSuperviseIn([]string{"--id", operation}, io.Discard, &stderr, func() *launch.Manager { return b.manager }, b.workOwners())
			execution, err = b.manager.Store.Read(operation)
			if children, _ := p.counts(); code != 1 || err != nil || children != 0 || execution.Child != nil || !strings.HasPrefix(execution.Reason, "authority-held: ") {
				t.Fatalf("direct supervision: code=%d children=%d record=%+v error=%v %s", code, children, execution, err, stderr.String())
			}
		})
	}
}

func TestDriverPendingRecoveryAsyncAuthority(t *testing.T) {
	t.Parallel()
	for _, actor := range []string{"person", "agent"} {
		t.Run(actor, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			person := b.personProof
			agent := func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			b.personProof = agent
			conf := filepath.Join(b.root(), "metasystem.conf")
			settings, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			policy := func(value string) {
				t.Helper()
				if err := os.WriteFile(conf, append(append([]byte(nil), settings...), []byte("\nseat.driver="+value+"\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			policy("auto")
			b.manager.Supervisor = &recoveryStarter{bed: b, crash: true}
			argv := append([]string{"work", "build", b.id, "--work", "async-recovery", "--brief", b.brief("async.md", "Build this unit.\n"), "--lines", "5"}, workCheck...)
			func() {
				defer func() {
					if recover() != "reservation crash" {
						t.Fatal("no reservation crash")
					}
				}()
				pendingWork(t, b, argv...)
			}()
			runs, _, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
			if err != nil || len(runs) != 1 {
				t.Fatalf("runs=%d error=%v", len(runs), err)
			}
			run, operation := runs[0].Run, runs[0].Record.Rounds[0].Steps[0].LaunchID
			b.manager.Supervisor = cancelRecoveryStarter{manager: b.manager, bed: b}
			a := recoveryAdapter{before: func() { policy("person") }}
			b.manager.Adapters["codex-exec"], b.manager.Adapters["claude-headless"] = a, a
			code, held, _ := pendingWork(t, b, "work", "build", "run:"+run)
			reservation, err := b.manager.Store.Read(operation)
			if code != 1 || err != nil || !strings.HasPrefix(reservation.Reason, "authority-held: ") {
				t.Fatalf("initial hold: code=%d record=%+v error=%v %s", code, reservation, err, held.Summary)
			}
			if actor == "person" {
				b.personProof = person
			} else {
				policy("auto")
				// An unclaimed reservation can still carry a departed person's proof.
				if _, err := b.manager.Store.Update(operation, func(r *launch.Record) error {
					r.AdapterData["unitPersonInvocation"] = json.RawMessage(`true`)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			ended := make(chan struct{})
			close(ended)
			p := &recoveryProcesses{s: &recoveryStarter{ended: ended}}
			b.manager.Processes = p
			b.manager.Adapters["codex-exec"], b.manager.Adapters["claude-headless"] = workAdapter{}, workAdapter{}
			s := &deferredRecoveryStarter{bed: b, release: make(chan struct{}), done: make(chan int, 1)}
			b.manager.Supervisor = s
			b.manager.StartCap = time.Minute
			polled, supervisorExit := false, -1
			sleep := b.manager.Sleep
			b.manager.Sleep = func(d time.Duration) {
				if polled {
					sleep(d)
					return
				}
				polled = true
				policy("person")
				close(s.release)
				supervisorExit = <-s.done
			}
			code, result, _ := pendingWork(t, b, "work", "build", "run:"+run)
			if !polled {
				if !s.started {
					t.Fatalf("recovery did not start supervision: code=%d %s", code, result.Summary)
				}
				// Release the detached task even when a stale reason ends the caller early.
				close(s.release)
				<-s.done
				t.Fatalf("caller reported the old hold before the supervisor claimed: code=%d %s", code, result.Summary)
			}
			execution, err := b.manager.Store.Read(operation)
			n, signals := p.counts()
			if err != nil || signals != 0 {
				t.Fatalf("execution error=%v signals=%d", err, signals)
			}
			if _, retained := execution.AdapterData["unitPersonInvocation"]; retained {
				t.Fatal("consumed person proof remains reusable in the reservation")
			}
			if actor == "person" {
				if code != 0 || supervisorExit != 0 || n != 1 || execution.Child == nil || strings.HasPrefix(execution.Reason, "authority-held: ") {
					t.Fatalf("person recovery: code=%d supervisor=%d children=%d record=%+v %s", code, supervisorExit, n, execution, result.Summary)
				}
				code, _, _ = pendingWork(t, b, "work", "build", "run:"+run)
				if n, _ := p.counts(); code != 0 || n != 1 {
					t.Fatalf("repeated recovery: code=%d children=%d", code, n)
				}
			} else if code != 1 || supervisorExit != 1 || n != 0 || execution.Child != nil || !strings.HasPrefix(execution.Reason, "authority-held: ") {
				t.Fatalf("agent recovery: code=%d supervisor=%d children=%d record=%+v %s", code, supervisorExit, n, execution, result.Summary)
			}
		})
	}
}
