package applaunch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// censusProber answers as the kernel does for every process but one, whose
// answer the test chooses: the seam through which a member of a real group
// is made uncertain.
type censusProber struct {
	pid    int64
	answer func(identity.Exact) (identity.Exact, identity.Liveness, error)
}

func (p censusProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if pid == p.pid && err == nil && state == identity.Alive {
		return p.answer(exact)
	}
	return exact, state, err
}

func (p censusProber) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.KernelProber{}.ReadStart(pid)
}

// tableGroup is the kernel group reader over table alone: a test's group
// census reads the processes it started, never the host's.
func tableGroup(table identity.ProcessTable) GroupReader {
	return func(pgid int64) ([]Member, error) { return groupCensus(table, pgid, identity.KernelProber{}) }
}

// The group census reads only the table it is given: a live member outside
// it is never counted, whatever runs on the host.
func TestGroupCensusReadsOnlyItsTable(t *testing.T) {
	t.Parallel()
	pid := groupDescendant(t)
	if members, err := groupCensus(identity.ListedProcessTable{pid}, pid, identity.KernelProber{}); err != nil || len(members) != 1 || members[0].Pid != pid {
		t.Fatalf("a table holding the member = %+v, %v; want the member", members, err)
	}
	if members, err := groupCensus(identity.ListedProcessTable{}, pid, identity.KernelProber{}); err != nil || len(members) != 0 {
		t.Fatalf("an empty table = %+v, %v; want no member", members, err)
	}
}

// groupDescendant is a live process leading a group of its own, standing
// for a descendant the application left behind.
func groupDescendant(t *testing.T) int64 {
	t.Helper()
	command := exec.Command("sleep", "60")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	return int64(command.Process.Pid)
}

// deadRef is the identity of a process that has since ended.
func deadRef(t *testing.T) string {
	t.Helper()
	command := exec.Command("sleep", "60")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exact, _, err := identity.KernelProber{}.Probe(int64(command.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	return encoded
}

var (
	unreadableArgv = func(exact identity.Exact) (identity.Exact, identity.Liveness, error) {
		exact.Argv, exact.ArgvKnown = nil, false
		return exact, identity.Alive, nil
	}
	uncertainProbe = func(identity.Exact) (identity.Exact, identity.Liveness, error) {
		return identity.Exact{}, identity.Unknown, errors.New("identity: read refused")
	}
	finishedProbe = func(exact identity.Exact) (identity.Exact, identity.Liveness, error) {
		exact.Zombie = true
		return exact, identity.Alive, nil
	}
)

// A member that is alive but whose identity cannot be proved stays in the
// census, said as uncertain; only a member the probe says is finished may be
// left out.
func TestGroupCensusKeepsAnUncertainMember(t *testing.T) {
	t.Parallel()
	pid := groupDescendant(t)
	for name, answer := range map[string]func(identity.Exact) (identity.Exact, identity.Liveness, error){
		"unreadable argv": unreadableArgv, "uncertain probe": uncertainProbe,
	} {
		members, err := groupCensus(identity.ListedProcessTable{pid}, pid, censusProber{pid: pid, answer: answer})
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 1 || members[0].Pid != pid || !members[0].Uncertain {
			t.Fatalf("%s: an alive member whose identity is uncertain stays in the census: %+v", name, members)
		}
	}
	members, err := groupCensus(identity.ListedProcessTable{pid}, pid, censusProber{pid: pid, answer: finishedProbe})
	if err != nil || len(members) != 0 {
		t.Fatalf("a member the probe says is finished may be left out: %+v %v", members, err)
	}
}

// Stop's proof of death reads the same census, so an uncertain descendant
// keeps the run unproven and is never signalled.
func TestStopCannotProveAGroupWithAnUncertainMember(t *testing.T) {
	t.Parallel()
	pid := groupDescendant(t)
	for name, answer := range map[string]func(identity.Exact) (identity.Exact, identity.Liveness, error){
		"unreadable argv": unreadableArgv, "uncertain probe": uncertainProbe,
	} {
		root := t.TempDir()
		if err := WriteRecord(root, Record{Key: StandingKey, Supervisor: deadRef(t), Child: deadRef(t), Group: pid, StateRoot: root}); err != nil {
			t.Fatal(err)
		}
		var signalled []int
		result, err := Stop(root, StandingKey, Contract{StopMS: 100}, StopOptions{
			Wait: 300 * time.Millisecond,
			Group: func(pgid int64) ([]Member, error) {
				return groupCensus(identity.ListedProcessTable{pid}, pgid, censusProber{pid: pid, answer: answer})
			},
			Send: func(pid int, sig syscall.Signal) error { signalled = append(signalled, pid); return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Join(result.Lines, "\n")
		if result.Proven || result.Outcome == StoppedNow {
			t.Fatalf("%s: a group with an uncertain live member is not proven empty, got %s\n%s", name, result.Outcome, lines)
		}
		if !strings.Contains(lines, "cannot prove the group empty") {
			t.Fatalf("%s: stop says why it is unproven:\n%s", name, lines)
		}
		for _, sent := range signalled {
			if int64(sent) == pid {
				t.Fatalf("%s: an unidentified member was signalled", name)
			}
		}
	}
}

// A group that cannot be read is never an empty group: stop says it cannot
// prove the group empty.
func TestStopCannotProveAGroupItCannotRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := WriteRecord(root, Record{Key: StandingKey, Supervisor: deadRef(t), Child: deadRef(t), Group: 4242, StateRoot: root}); err != nil {
		t.Fatal(err)
	}
	result, err := Stop(root, StandingKey, Contract{StopMS: 100}, StopOptions{
		Wait:  300 * time.Millisecond,
		Group: func(int64) ([]Member, error) { return nil, errors.New("process table unreadable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(result.Lines, "\n")
	if result.Proven || !strings.Contains(lines, "cannot prove the group empty") {
		t.Fatalf("an unreadable group is not proven empty, got %s\n%s", result.Outcome, lines)
	}
}

// A supervisor whose group read errs keeps waiting, says why in the run's
// log, and writes no ended record.
func TestSuperviseKeepsWaitingWhenItsGroupCannotBeRead(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	bed := newSuperviseBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{app, "--no-listen", "--exit-after", "300ms"}},
		"stopMs": 800, "readyMs": 5000})
	var reads atomic.Int32
	var readable atomic.Bool
	// Every group read the supervisor makes is an event the test waits on,
	// so the test observes the supervisor's own loop and never a clock.
	read := make(chan struct{}, 1024)
	bed.options.Group = func(int64) ([]Member, error) {
		reads.Add(1)
		select {
		case read <- struct{}{}:
		default:
		}
		if readable.Load() {
			return nil, nil
		}
		return nil, errors.New("process table unreadable")
	}
	bed.options.LeadsGroup = func() (int64, bool) { return 4242, true }
	bed.options.SendGroup = func(int64, syscall.Signal) error { return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bed.options.Context = ctx
	done := make(chan error, 1)
	go func() { done <- Supervise(bed.options) }()
	for reads.Load() < 8 {
		select {
		case <-done:
			t.Fatal("the supervisor left while its group could not be read")
		case <-read:
		}
	}
	record, err := ReadRecord(bed.stateRoot, StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	if record.Ended != nil {
		t.Fatalf("a group read error is never an empty group; the record must stay unended: %+v", record.Ended)
	}
	log, _ := os.ReadFile(record.Log)
	if !strings.Contains(string(log), "could not be inspected") {
		t.Fatalf("the supervisor says why it keeps waiting:\n%s", log)
	}
	// A signal ends the group by the leader's own signal, and still the
	// supervisor does not write an ended record over a group it cannot read.
	cancel()
	// Twenty more reads of the group after the signal: the supervisor reads
	// every 200 milliseconds while it waits, so that is well past the group
	// signal's stopMs and both of finish's waits, measured by its own loop.
	for target := reads.Load() + 20; reads.Load() < target; {
		select {
		case <-done:
			t.Fatal("the supervisor left after its signal while its group could not be read")
		case <-read:
		}
	}
	if record, err := ReadRecord(bed.stateRoot, StandingKey); err != nil || record.Ended != nil {
		t.Fatalf("no ended record over a group that cannot be read: %+v %v", record, err)
	}
	readable.Store(true)
	// The join is the event: the supervisor's next read finds the group
	// empty and it finishes. No clock caps it; the test runner's own timeout
	// is the only bound, by the standing rule that tests wait on events.
	<-done
	if record, err = ReadRecord(bed.stateRoot, StandingKey); err != nil || record.Ended == nil {
		t.Fatalf("once the group reads empty, the run ends into an ended record: %+v %v", record, err)
	}
}

// A foreign service answering the probe is never the application: readiness
// is accepted only while the owned child is alive.
func TestAwaitReadyRefusesAForeignAnswerWhenTheChildHasExited(t *testing.T) {
	t.Parallel()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer foreign.Close()
	address := strings.TrimPrefix(foreign.URL, "http://")
	for _, form := range []Contract{
		{Address: address, Ready: &Ready{Kind: ReadyHTTP, URL: "http://${address}/-/health"}},
		{Address: address, Ready: &Ready{Kind: ReadyTCP, Address: "${address}"}},
	} {
		err := AwaitReady(context.Background(), form, address, "", 0, func() bool { return false }, time.Second)
		if !ExitedBeforeReady(err) {
			t.Fatalf("%s: a child that exited while a foreign listener answers is reported as exited: %v", form.ReadyKind(), err)
		}
	}
}

// An ended marker is trusted only once the group is checked: an ended record
// whose group still has a member reads as descendants alive, never finished.
func TestAnEndedRecordWithALiveGroupIsNotFinished(t *testing.T) {
	t.Parallel()
	pid := groupDescendant(t)
	root := t.TempDir()
	if err := WriteRecord(root, Record{Key: StandingKey, Supervisor: deadRef(t), Child: deadRef(t), Group: pid, StateRoot: root,
		Ended: &Ended{At: "2026-09-28T00:00:00Z", ExitStatus: "exit 0"}}); err != nil {
		t.Fatal(err)
	}
	status, err := Read(root, StandingKey, Contract{}, ReadOptions{Group: tableGroup(identity.ListedProcessTable{pid})})
	if err != nil {
		t.Fatal(err)
	}
	if status.State == Finished || !strings.Contains(status.Problem, "ended record, descendants alive") {
		t.Fatalf("an ended record whose group has a member is not finished, got %s: %s", status.State, status.Problem)
	}
}
