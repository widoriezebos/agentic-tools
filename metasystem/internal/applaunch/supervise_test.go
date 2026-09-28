package applaunch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// superviseBed runs the supervisor in this process, with the contract and
// the application a test chooses. The subprocess fixture proves what a real
// detached supervisor does; this one holds the order of its acts.
type superviseBed struct {
	t         *testing.T
	stateRoot string
	contract  Contract
	options   SuperviseOptions
}

func newSuperviseBed(t *testing.T, contract map[string]any) *superviseBed {
	t.Helper()
	root := t.TempDir()
	contract["schemaVersion"] = 1
	path := filepath.Join(root, "launch.json")
	writeJSON(t, path, contract)
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("the fixture contract must validate: %v", err)
	}
	address, _ := contract["address"].(string)
	bed := &superviseBed{t: t, stateRoot: root, contract: loaded}
	bed.options = SuperviseOptions{
		StateRoot: root, Contract: loaded, ProjectRoot: root, Environment: os.Environ(),
		Seed: Record{Key: StandingKey, Address: address, StateRoot: root},
	}
	t.Cleanup(func() {
		record, err := ReadRecord(root, StandingKey)
		if err != nil {
			return
		}
		if ref, recorded, err := record.ChildRef(); recorded && err == nil {
			_ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL)
		}
	})
	return bed
}

func writeJSON(t *testing.T, path string, value map[string]any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The record is written before anything is spawned, and the application's
// own identity is the very next act after the spawn. The spawn itself
// observes the record, so the order is held where it happens.
func TestSuperviseWritesTheRecordBeforeTheSpawn(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	bed := newSuperviseBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{app, "--no-listen", "--exit-after", "10s"}},
		"stopMs": 2000, "readyMs": 5000})
	var atSpawn *Record
	bed.options.Spawn = func(spec ChildSpec) (Child, error) {
		atSpawn, _ = ReadRecord(bed.stateRoot, StandingKey)
		return ExecChild(spec)
	}
	ready := make(chan string, 1)
	bed.options.Ready = func(address string) { ready <- address }
	done := make(chan error, 1)
	go func() { done <- Supervise(bed.options) }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("the supervisor left before readiness: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("the supervisor never reported readiness")
	}
	if atSpawn == nil {
		t.Fatal("the record must exist before the application is spawned")
	}
	if atSpawn.Supervisor == "" || atSpawn.Group == 0 {
		t.Fatalf("the record written before the spawn names the supervisor and its group: %+v", atSpawn)
	}
	if atSpawn.Child != "" {
		t.Fatalf("no application can be recorded before it is spawned: %+v", atSpawn)
	}
	after, err := ReadRecord(bed.stateRoot, StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	if after.Child == "" {
		t.Fatal("the application's identity is written as the very next act after the spawn")
	}
	if after.ReadyAt == "" || after.Data != DataShared || after.ContractDigest == "" {
		t.Fatalf("the record carries the run's facts: %+v", after)
	}
	// End the run so the supervisor can finish.
	ref, _, err := after.ChildRef()
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the supervisor did not finish")
	}
	ended, err := ReadRecord(bed.stateRoot, StandingKey)
	if err != nil || ended.Ended == nil {
		t.Fatalf("a run ends into an ended record the supervisor never deletes: %+v %v", ended, err)
	}
}

// A start command that cannot be run at all leaves an ended record and a
// refusal, never a run that seems to be starting.
func TestSuperviseReportsASpawnThatCannotRun(t *testing.T) {
	t.Parallel()
	bed := newSuperviseBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{"./no-such-application"}},
		"stopMs": 2000, "readyMs": 2000})
	var failure string
	bed.options.Failed = func(message string) { failure = message }
	err := Supervise(bed.options)
	if err == nil || !strings.Contains(err.Error(), "could not be run") {
		t.Fatalf("a start command that cannot run is a refusal: %v", err)
	}
	if failure == "" {
		t.Fatal("the refusal is reported to whoever launched the supervisor")
	}
	record, readErr := ReadRecord(bed.stateRoot, StandingKey)
	if readErr != nil || record.Ended == nil {
		t.Fatalf("the record says the run ended before it began: %+v %v", record, readErr)
	}
}

// A readiness timeout ends the application by its own recorded ref, never
// only the supervisor: a run must not outlive the engine's belief that it
// never started.
func TestSuperviseReadinessTimeoutEndsTheApplication(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	bed := newSuperviseBed(t, map[string]any{
		"start":   map[string]any{"argv": []string{app, "--no-listen"}},
		"ready":   map[string]any{"kind": "log", "pattern": "^NEVER$"},
		"readyMs": 700, "stopMs": 1500})
	var failure string
	bed.options.Failed = func(message string) { failure = message }
	var spawned int
	bed.options.Spawn = func(spec ChildSpec) (Child, error) {
		child, err := ExecChild(spec)
		if err == nil {
			spawned = child.Pid()
		}
		return child, err
	}
	err := Supervise(bed.options)
	if err == nil || !strings.Contains(err.Error(), "did not become ready") {
		t.Fatalf("a readiness timeout is a refusal: %v", err)
	}
	if !strings.Contains(failure, "did not become ready") {
		t.Fatalf("the refusal says what happened: %q", failure)
	}
	if spawned == 0 {
		t.Fatal("the application was never spawned")
	}
	exact, state, _ := identity.KernelProber{}.Probe(int64(spawned))
	if state == identity.Alive && exact.ArgvKnown {
		t.Fatal("the application must be ended by its own ref when readiness times out")
	}
}

// A signal to the supervisor ends the application it owns.
func TestSuperviseEndsTheApplicationOnASignal(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	bed := newSuperviseBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{app, "--no-listen"}},
		"stopMs": 2000, "readyMs": 5000})
	ctx, cancel := context.WithCancel(context.Background())
	bed.options.Context = ctx
	ready := make(chan string, 1)
	bed.options.Ready = func(address string) { ready <- address }
	done := make(chan error, 1)
	go func() { done <- Supervise(bed.options) }()
	select {
	case <-ready:
	case <-time.After(20 * time.Second):
		t.Fatal("the supervisor never reported readiness")
	}
	record, err := ReadRecord(bed.stateRoot, StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the supervisor did not finish after its signal")
	}
	ref, _, err := record.ChildRef()
	if err != nil {
		t.Fatal(err)
	}
	if identity.AliveRef(identity.KernelProber{}, ref) == identity.Alive {
		t.Fatal("a signal to the supervisor ends the application it owns")
	}
}

// The supervisor stays the owner while any process of its own group remains,
// and only it may signal that group.
func TestSuperviseStaysTheOwnerWhileItsGroupHasMembers(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	bed := newSuperviseBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{app, "--no-listen", "--exit-after", "300ms"}},
		"stopMs": 800, "readyMs": 5000})
	var emptied atomic.Bool
	var sent []string
	var sending sync.Mutex
	// The supervisor reads its group only once the application has ended,
	// so each read is an observation that it is still there as the owner.
	asked := make(chan struct{}, 64)
	bed.options.Group = func(pgid int64) ([]Member, error) {
		select {
		case asked <- struct{}{}:
		default:
		}
		if emptied.Load() {
			return nil, nil
		}
		return []Member{{Pid: 999999, Ref: "pid=999999;micro=1"}}, nil
	}
	bed.options.LeadsGroup = func() (int64, bool) { return 4242, true }
	bed.options.SendGroup = func(pgid int64, sig syscall.Signal) error {
		sending.Lock()
		defer sending.Unlock()
		sent = append(sent, "group:"+sig.String())
		emptied.Store(true)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	bed.options.Context = ctx
	done := make(chan error, 1)
	go func() { done <- Supervise(bed.options) }()
	// The application ends on its own; the supervisor must not leave while
	// the group still reports a member. Three reads of a group that still has
	// one, and it has not returned.
	for read := 0; read < 3; read++ {
		select {
		case <-asked:
		case <-done:
			t.Fatal("the supervisor left while a process of its group remained")
		}
	}
	select {
	case <-done:
		t.Fatal("the supervisor left while a process of its group remained")
	default:
	}
	cancel()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the supervisor did not finish")
	}
	sending.Lock()
	ended := append([]string(nil), sent...)
	sending.Unlock()
	if len(ended) == 0 || !strings.HasPrefix(ended[0], "group:") {
		t.Fatalf("only the group's leader may end it, and it must: %v", ended)
	}
	record, err := ReadRecord(bed.stateRoot, StandingKey)
	if err != nil || record.Ended == nil {
		t.Fatalf("the run ends into an ended record: %+v %v", record, err)
	}
}

// A group is never signalled by a process that does not lead it: a group id
// whose leader is not provably ours may be a reused one.
func TestSuperviseNeverSignalsAGroupItDoesNotLead(t *testing.T) {
	t.Parallel()
	var sent []string
	options := SuperviseOptions{
		Contract:   Contract{Start: &Command{Argv: []string{"x"}}},
		LeadsGroup: func() (int64, bool) { return 4242, false },
		Group:      func(int64) ([]Member, error) { return []Member{{Pid: 7, Ref: "pid=7;micro=1"}}, nil },
		SendGroup:  func(pgid int64, sig syscall.Signal) error { sent = append(sent, sig.String()); return nil },
	}
	options.endGroup(Record{Group: 4242})
	if len(sent) != 0 {
		t.Fatalf("a group this process does not lead must not be signalled: %v", sent)
	}
}

// AwaitReady answers each form, and an application that exits before it is
// ready is reported as such rather than waited out.
func TestAwaitReadyFormsAndEarlyExit(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(logPath, []byte("starting\nREADY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	none := Contract{}
	if err := AwaitReady(context.Background(), none, "", "", 0, func() bool { return true }, time.Second); err != nil {
		t.Fatalf("the none form means alive is ready: %v", err)
	}
	if err := AwaitReady(context.Background(), none, "", "", 0, func() bool { return false }, time.Second); !ExitedBeforeReady(err) {
		t.Fatalf("a dead application is never ready: %v", err)
	}
	logForm := Contract{Ready: &Ready{Kind: ReadyLog, Pattern: "^READY$"}}
	if err := AwaitReady(context.Background(), logForm, "", logPath, 0, func() bool { return true }, time.Second); err != nil {
		t.Fatalf("the log form reads the log: %v", err)
	}
	// From this run's offset the earlier READY line is not this run's.
	offset := int64(len("starting\nREADY\n"))
	if err := AwaitReady(context.Background(), logForm, "", logPath, offset, func() bool { return true }, 300*time.Millisecond); !ReadyTimeout(err) {
		t.Fatalf("a line before this run's offset must not make it ready: %v", err)
	}
	httpForm := Contract{Address: "127.0.0.1:1", Ready: &Ready{Kind: ReadyHTTP, URL: "http://127.0.0.1:1/-/health"}}
	if err := AwaitReady(context.Background(), httpForm, "127.0.0.1:1", "", 0, func() bool { return true }, 300*time.Millisecond); !ReadyTimeout(err) {
		t.Fatalf("an http form that never answers times out: %v", err)
	}
	if err := ProbeOnce(logForm, ""); !NotProbed(err) {
		t.Fatalf("a log form has no probe to ask: %v", err)
	}
	if found, err := LogMatch(filepath.Join(t.TempDir(), "absent.log"), "x", 0); err != nil || found {
		t.Fatalf("a log that is not there has matched nothing: %v %v", found, err)
	}
}

// The supervisor's argument vector names the run it is to own.
func TestServeArgsNameTheRun(t *testing.T) {
	t.Parallel()
	joined := strings.Join(ServeArgs("/repo", "/repo/metasystem", "at-main-1234", "main", "g1", "127.0.0.1:7981"), " ")
	for _, want := range []string{"app serve", "--repo /repo", "--metasystem-root /repo/metasystem",
		"--key at-main-1234", "--ready-fd 3", "--at main", "--goal g1", "--address 127.0.0.1:7981"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the supervisor's argv must carry %q; got %q", want, joined)
		}
	}
	bare := strings.Join(ServeArgs("/repo", "/repo/metasystem", StandingKey, "", "", ""), " ")
	if strings.Contains(bare, "--at") || strings.Contains(bare, "--goal") || strings.Contains(bare, "--address") {
		t.Fatalf("the standing run names no ref, goal or address: %q", bare)
	}
}

// Records are per run and the standing one is listed first.
func TestRecordsAreListedStandingFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if keys, err := Keys(root); err != nil || len(keys) != 0 {
		t.Fatalf("a seat with no runs lists none: %v %v", keys, err)
	}
	self, _, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := identity.EncodeRef(self.Ref())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"at-main-abcd", StandingKey} {
		if err := WriteRecord(root, Record{Key: key, Supervisor: encoded, Group: 1}); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := Keys(root)
	if err != nil || len(keys) != 2 || keys[0] != StandingKey {
		t.Fatalf("the standing run is listed first: %v %v", keys, err)
	}
	if err := UpdateRecord(root, StandingKey, func(record *Record) { record.Address = "127.0.0.1:7979" }); err != nil {
		t.Fatal(err)
	}
	record, err := ReadRecord(root, StandingKey)
	if err != nil || record.Address != "127.0.0.1:7979" {
		t.Fatalf("a record is rewritten in place: %+v %v", record, err)
	}
	// A record that is not there is a run that was stopped, not an error.
	if err := UpdateRecord(root, "absent", func(*Record) {}); err != nil {
		t.Fatalf("updating a record that is gone is not an error: %v", err)
	}
	if err := RemoveRecord(root, StandingKey); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRecord(root, StandingKey); err != nil {
		t.Fatalf("removing a record twice is not an error: %v", err)
	}
	if RunDir(root, StandingKey) == "" || DefaultLogPath(root, StandingKey) == "" {
		t.Fatal("every run has a directory and a log")
	}
}

// Stop runs the contract's own stop command where there is one.
func TestStopRunsTheContractsOwnStopCommand(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	scratch := t.TempDir()
	marker, live := filepath.Join(scratch, "stopped"), filepath.Join(scratch, "live")
	// The stop command ends this run's own application by the pid it wrote,
	// and nothing else: a fixture that reached for every process of its own
	// name would end another test's run beside it.
	b := newBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{app, "--no-listen", "--live-file", live}},
		"stop":   map[string]any{"argv": []string{"sh", "-c", "touch " + marker + "; kill $(cat " + live + ") 2>/dev/null || true"}},
		"stopMs": 4000})
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{Wait: 20 * time.Second, ProjectRoot: b.root, Environment: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("the contract's own stop command must run: %v", statErr)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "ran the contract's stop command") {
		t.Fatalf("stop says what it did:\n%s", strings.Join(result.Lines, "\n"))
	}
}

// A rejoin refuses a run that is not starting rather than waiting for one.
func TestRejoinRefusesARunThatIsNotStarting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract := Contract{Start: &Command{Argv: []string{"x"}}}
	status, err := Rejoin(context.Background(), root, StandingKey, contract, ReadOptions{}, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("a rejoin of nothing is a refusal: %v %v", status.State, err)
	}
}

// A tool is said with the executable found and, where it printed one, its
// version line.
func TestToolLineSaysTheExecutableAndItsVersion(t *testing.T) {
	t.Parallel()
	if got := (ToolLine{ID: "go", Executable: "/usr/bin/go", Version: "go version go1"}).Line(); got != "go: /usr/bin/go (go version go1)" {
		t.Errorf("a tool with a version line: %q", got)
	}
	if got := (ToolLine{ID: "shell", Executable: "/bin/sh"}).Line(); got != "shell: /bin/sh" {
		t.Errorf("a tool without one: %q", got)
	}
}

// The readiness descriptor is the supervisor's own: the application it
// spawns never inherits it.
func TestReadinessPipeIsNeverInherited(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	duplicate, err := syscall.Dup(int(writer.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	pipe := ReadinessPipe(duplicate)
	defer pipe.Close()
	flags, err := unix.FcntlInt(uintptr(duplicate), unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("the readiness descriptor must be close-on-exec before anything is spawned")
	}
}
