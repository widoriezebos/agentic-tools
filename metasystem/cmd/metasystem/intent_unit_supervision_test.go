package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"golang.org/x/sys/unix"
)

// supervisedFreshRead holds one production supervision after its child is recorded.
type supervisedFreshRead struct {
	workProcesses
	b                    *workBed
	id                   string
	ready, release, done chan struct{}
	once                 sync.Once
	code                 int
	log                  bytes.Buffer
	started              bool
}

func (s *supervisedFreshRead) StartSupervisor(id, state string) (identity.Ref, error) {
	s.id, s.started = id, true
	go func() {
		s.code = runLaunchSupervise([]string{"--id", id}, &s.log, &s.log)
		close(s.done)
	}()
	select {
	case <-s.ready:
	case <-s.done:
		return identity.Ref{}, fmt.Errorf("supervision ended before its child was ready: %s", s.log.String())
	}
	return workProcessRef(10), nil
}

func (s *supervisedFreshRead) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	record, err := s.b.manager.Store.Read(s.id)
	if err != nil {
		return nil, identity.Ref{}, err
	}
	var paths []string
	if err := json.Unmarshal(record.AdapterData["declaredOutputs"], &paths); err != nil {
		return nil, identity.Ref{}, err
	}
	for index, path := range paths {
		content := "VERDICT: LAND\n"
		if index == 1 {
			content = `{"findings":[],"verdictMaterialCount":0}`
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return nil, identity.Ref{}, err
		}
	}
	return s, workProcessRef(20), nil
}

func (s *supervisedFreshRead) Wait() (int, error) {
	close(s.ready)
	<-s.release
	return 0, nil
}

func (s *supervisedFreshRead) end() {
	s.once.Do(func() { close(s.release) })
	if s.started {
		<-s.done
	}
}

func freshSupervisionBed(t *testing.T) (*workBed, string, intentOwners, *supervisedFreshRead, func() []string) {
	t.Helper()
	b := newStopWorkBed(t)
	b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{}}, corrupt: "missing-return"}
	run, built, _ := stopBuild(t, b, "auto")
	// The candidate is also an installation; its ledger must never receive collection.
	if err := os.WriteFile(filepath.Join(b.worktree, "metasystem.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	var mu sync.Mutex
	var resolved []string
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
		mu.Lock()
		resolved = append(resolved, path)
		mu.Unlock()
		for _, root := range []string{b.root(), b.worktree} {
			if top, err := fakeTop(root)(path); err == nil {
				return top, nil
			}
		}
		return "", fmt.Errorf("unexpected installation %s", path)
	}, noExecutable)
	s := &supervisedFreshRead{b: b, ready: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{})}
	b.manager.Supervisor, b.manager.Processes = s, s
	oldManager, oldOwners := launchManager, launchOwners
	launchManager = func() *launch.Manager { return b.manager }
	launchOwners = func() intentOwners { return owners }
	t.Cleanup(func() { s.end(); launchManager, launchOwners = oldManager, oldOwners })
	for _, step := range built.Rounds[0].Steps {
		execution, err := b.manager.Store.Read(step.LaunchID)
		if err != nil {
			t.Fatal(err)
		}
		assertUnitStamp(t, execution, run, b.root())
	}
	return b, run, owners, s, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), resolved...) }
}

func assertUnitStamp(t *testing.T, record launch.Record, run, installation string) {
	t.Helper()
	var stampedRun, stampedInstallation string
	_ = json.Unmarshal(record.AdapterData["unitRun"], &stampedRun)
	_ = json.Unmarshal(record.AdapterData["unitInstallation"], &stampedInstallation)
	if stampedRun != run || stampedInstallation != installation {
		t.Fatalf("wrong unit stamp: %+v", record)
	}
}

func reviewFresh(t *testing.T, b *workBed, owners intentOwners) (int, intentResult) {
	t.Helper()
	return b.runJSON(owners, "work", "review", b.id, "--work", "stopped")
}

func TestALaunchEndBooksItsRound(t *testing.T) {
	// Regression. Serial: replaces launchManager and launchOwners for one supervision.
	for _, busy := range []bool{false, true} {
		t.Run(fmt.Sprintf("run-lock-busy=%t", busy), func(t *testing.T) {
			b, run, owners, s, resolved := freshSupervisionBed(t)
			code, result := reviewFresh(t, b, owners)
			if code != 3 || result.Summary != "the fresh read of attempt 1 is running" {
				t.Fatalf("review: %d %+v", code, result)
			}
			record, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			step := record.Rounds[0].Steps[len(record.Rounds[0].Steps)-1]
			execution, _ := b.manager.Store.Read(step.LaunchID)
			assertUnitStamp(t, execution, run, b.root())
			before := len(resolved())
			var lock *os.File
			if busy {
				var err error
				lock, err = os.OpenFile(filepath.Join(b.unitRoot, run, ".lock"), os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
				if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			}
			s.end()
			if s.code != 0 || s.log.Len() != 0 {
				t.Fatalf("supervisor: %d %s", s.code, s.log.String())
			}
			paths := resolved()[before:]
			if len(paths) == 0 {
				t.Fatal("supervisor did not resolve its installation")
			}
			for _, path := range paths {
				if path != b.root() {
					t.Fatalf("supervisor composed from %s instead of %s", path, b.root())
				}
			}
			reservation := filepath.Join(b.root(), "artifacts", "agents", "jobs", step.LaunchID+".json")
			if busy {
				record, _ = (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
				if record.Rounds[0].Steps[len(record.Rounds[0].Steps)-1].State != launch.StepRunning {
					t.Fatal("busy run was booked")
				}
				if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
					t.Fatal(err)
				}
				code, result = b.runJSON(owners, "work", "wait", b.id, "--work", "stopped")
				if code != 0 {
					t.Fatalf("fallback wait: %d %+v", code, result)
				}
			}
			assertLateReadBooked(t, b, run, step, 0)
			job, err := dispatchcore.ReadRecordObject(reservation)
			if err != nil || !dispatchcore.TerminalStatus(fmt.Sprint(job["status"])) {
				t.Fatalf("uncollected reservation: %+v %v", job, err)
			}
			duplicates, _ := filepath.Glob(filepath.Join(b.worktree, "artifacts", "agents", "jobs", "*.json"))
			if len(duplicates) != 0 {
				t.Fatalf("candidate installation received reservations: %v", duplicates)
			}
		})
	}
}

func TestAReviewFollowsARoundBookedDuringItsWait(t *testing.T) {
	// U2 mutation: fails if the review does not reread after UNIT_WAIT_RETRY.
	// Serial: replaces launchManager and launchOwners for one supervision.
	b, run, owners, s, _ := freshSupervisionBed(t)
	reviewed := false
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) {
		reviewed = true
		return "", errors.New("round review reached the endpoint")
	}
	sleep := b.manager.Sleep
	b.manager.Sleep = func(d time.Duration) {
		// StartSupervisor returns only after Wait signals ready. This Sleep
		// therefore belongs to Manager.Wait, with the command locks released.
		if s.started {
			s.end()
		}
		sleep(d)
	}
	code, result := reviewFresh(t, b, owners)
	if code != 1 || !reviewed || !strings.Contains(resultWords(result), "round review reached the endpoint") {
		t.Fatalf("review did not follow the booked round: %d %+v", code, result)
	}
	record, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	assertLateReadBooked(t, b, run, record.Rounds[0].Steps[len(record.Rounds[0].Steps)-1], 0)
	if s.code != 0 || s.log.Len() != 0 {
		t.Fatalf("supervisor: %d %s", s.code, s.log.String())
	}
}

func TestAnEndedUnitLaunchIsNoRunningJob(t *testing.T) {
	t.Parallel()
	// Regression for completed; preservation for running, unreadable, and --all.
	for _, state := range []launch.State{launch.Completed, launch.Running, "missing"} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			manager := &launch.Manager{Store: launch.Store{Root: b.launchDir}}
			if state != "missing" {
				if err := manager.Store.Create(launch.Record{ID: "unit-job", State: state}); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(b.root(), "artifacts", "agents", "jobs", "unit-job.json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`{"jobId":"unit-job","unitRun":"one-run","operationId":"unit-launch:unit-job","status":"pending"}`), 0600); err != nil {
				t.Fatal(err)
			}
			b.families = stoptransition.LocalFamilies(stoptransition.LocalConfig{Root: b.root(), Checkout: b.root(), LaunchStatus: engineHost{launches: manager}.UnitLaunchStatus})[1:2]
			owners := b.owners()
			for _, all := range []bool{false, true} {
				args := []string{"work", "status"}
				if all {
					args = append(args, "--all")
				}
				code, result := b.runJSON(owners, args...)
				data, _ := json.Marshal(result.Data)
				if code != 0 || bytes.Contains(data, []byte("j2:unit-job")) != (all || state != launch.Completed) {
					t.Fatalf("work status all=%t: %d %+v", all, code, result)
				}
			}
			code, result := b.runJSON(owners, "system", "status")
			data, _ := json.Marshal(result.Data)
			if code != 0 || bytes.Contains(data, []byte("job unit-job")) != (state != launch.Completed) {
				t.Fatalf("system status: %d %+v", code, result)
			}
			inv := &intentInvocation{owners: owners, cwd: b.root(), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
			lines := strings.Join(inv.helmReport(helm.Seat{Checkout: b.root()}, helmNow), "\n")
			count := 1
			if state == launch.Completed {
				count = 0
			}
			if !strings.Contains(lines, fmt.Sprintf("running dispatch jobs: %d", count)) {
				t.Fatalf("helm: %s", lines)
			}
		})
	}
}
