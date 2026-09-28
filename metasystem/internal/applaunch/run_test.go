package applaunch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var (
	fixtureOnce sync.Once
	fixtureDir  string
	fixtureErr  error
)

// fixtures builds the application fixture and the real supervisor host once
// for the package. The supervisor host is the engine's own Supervise in a
// process of its own session and group, so that a test can interrupt it
// where a person's kill would land.
func fixtures(t *testing.T) (app, supervisor string) {
	t.Helper()
	fixtureOnce.Do(func() {
		fixtureDir, fixtureErr = os.MkdirTemp("", "applaunch-fixtures-")
		if fixtureErr != nil {
			return
		}
		for _, pair := range [][2]string{{"fixtureapp", "./testdata/fixtureapp"}, {"fixturesupervisor", "./testdata/supervisor"}} {
			build := exec.Command("go", "build", "-o", filepath.Join(fixtureDir, pair[0]), pair[1])
			if out, err := build.CombinedOutput(); err != nil {
				fixtureErr = fmt.Errorf("build %s: %v\n%s", pair[1], err, out)
				return
			}
		}
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return filepath.Join(fixtureDir, "fixtureapp"), filepath.Join(fixtureDir, "fixturesupervisor")
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

// bed is one seat: a state root, a project root and a written contract.
type bed struct {
	t         *testing.T
	root      string
	stateRoot string
	contract  Contract
	path      string
	app       string
	super     string
	address   string
}

func newBed(t *testing.T, contract map[string]any) *bed {
	t.Helper()
	app, super := fixtures(t)
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	contract["schemaVersion"] = 1
	body, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "launch.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("the fixture contract must validate: %v\n%s", err, body)
	}
	address, _ := contract["address"].(string)
	return &bed{t: t, root: root, stateRoot: stateRoot, contract: loaded, path: path, app: app, super: super, address: address}
}

// start launches a real detached supervisor for this bed and waits for its
// readiness answer, exactly as the engine's start does.
func (b *bed) start(key string) (string, int, error) {
	b.t.Helper()
	args := []string{"--state-root", b.stateRoot, "--key", key, "--contract", b.path,
		"--project-root", b.root, "--address", b.address, "--ready-fd", "3"}
	return b.startWith(key, args)
}

func (b *bed) startWith(key string, args []string) (string, int, error) {
	b.t.Helper()
	spec := LaunchSpec{Executable: b.super, Args: args, Dir: b.root,
		LogPath: filepath.Join(Dir(b.stateRoot), key+".launch.log")}
	address, pid, err := LaunchSupervisor(spec, ExecSpawn, 20*time.Second)
	if pid > 0 {
		// The engine's own start exits at once, so a supervisor it launched
		// is reparented and reaped by init. A test process outlives its
		// launches, so it reaps them itself; an unreaped zombie would read
		// as a living owner to every identity check below.
		go reap(pid)
	}
	b.t.Cleanup(func() { b.cleanup(key) })
	return address, pid, err
}

// cleanup ends whatever the test left behind, by identity, so that no
// fixture process outlives its test.
func (b *bed) cleanup(key string) {
	record, err := ReadRecord(b.stateRoot, key)
	if err != nil {
		return
	}
	prober := identity.KernelProber{}
	for _, encoded := range []string{record.Child, record.Supervisor} {
		if encoded == "" {
			continue
		}
		if ref, err := identity.ParseRef(encoded); err == nil {
			_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
		}
	}
	if members, err := KernelGroup(record.Group); err == nil {
		for _, member := range members {
			if ref, err := identity.ParseRef(member.Ref); err == nil && member.Pid != int64(os.Getpid()) {
				_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
			}
		}
	}
	// A KILL is delivered, not awaited: the test's temporary directory is
	// removed next, and a supervisor still writing its ended record into it
	// would fail that removal. Wait until nothing recorded is alive.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		alive := false
		for _, encoded := range []string{record.Child, record.Supervisor} {
			if ref, err := identity.ParseRef(encoded); err == nil && identity.AliveRef(prober, ref) != identity.Dead {
				alive = true
			}
		}
		if members, err := KernelGroup(record.Group); err == nil && len(members) > 0 {
			alive = true
		}
		if !alive {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func reap(pid int) {
	for attempt := 0; attempt < 6000; attempt++ {
		var status syscall.WaitStatus
		reaped, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if reaped == pid || err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (b *bed) status(key string) Status {
	b.t.Helper()
	status, err := Read(b.stateRoot, key, b.contract, ReadOptions{Probe: ProbeOnce})
	if err != nil {
		b.t.Fatal(err)
	}
	return status
}

func (b *bed) record(key string) *Record {
	b.t.Helper()
	record, err := ReadRecord(b.stateRoot, key)
	if err != nil {
		b.t.Fatalf("the run record must be readable: %v", err)
	}
	return record
}

func eventually(t *testing.T, what string, wait time.Duration, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func httpContract(app, address string, extra ...string) map[string]any {
	argv := append([]string{app, "--listen", "${address}"}, extra...)
	return map[string]any{
		"name":    "fixture",
		"address": address,
		"start":   map[string]any{"argv": argv},
		"ready":   map[string]any{"kind": "http", "url": "http://${address}/-/health"},
		"readyMs": 15000,
		"stopMs":  4000,
	}
}

// Start waits for readiness and records. The record names the supervisor,
// the group it leads and the application, and status says liveness and
// readiness separately.
func TestStartWaitsForReadyAndRecords(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address, "--ready-after", "600ms"))
	got, pid, err := b.start(StandingKey)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if got != address || pid <= 0 {
		t.Fatalf("start reported %q pid %d, want %q", got, pid, address)
	}
	record := b.record(StandingKey)
	if record.Supervisor == "" || record.Child == "" || record.Group <= 0 {
		t.Fatalf("the record must name the supervisor, its group and the application: %+v", record)
	}
	if record.ReadyAt == "" {
		t.Error("start returns only once readiness was observed, and the record says when")
	}
	if record.Data != DataShared {
		t.Errorf("a contract with no prepare says data shared with the standing run, got %q", record.Data)
	}
	status := b.status(StandingKey)
	if status.State != Running || status.Readiness != Answering {
		t.Fatalf("status must say running and answering, got %s/%s", status.State, status.Readiness)
	}
	lines := strings.Join(status.Lines(), "\n")
	if !strings.Contains(lines, "state: running") || !strings.Contains(lines, "readiness: answering") {
		t.Errorf("liveness and readiness are said separately:\n%s", lines)
	}
	if !strings.Contains(lines, "data: shared with the standing run") {
		t.Errorf("the status says the data word:\n%s", lines)
	}
}

func mustApp(t *testing.T) string {
	t.Helper()
	app, _ := fixtures(t)
	return app
}

// The one window the design leaves open: a supervisor killed between the
// spawn and the child's ref write. The record names its group and no child,
// status says exactly that, the application is discoverable in that group by
// its proven identity, and nothing is signalled by number.
func TestSupervisorInterruptedBetweenSpawnAndChildWrite(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	live := filepath.Join(t.TempDir(), "alive")
	b := newBed(t, httpContract(mustApp(t), address, "--live-file", live))
	args := []string{"--state-root", b.stateRoot, "--key", StandingKey, "--contract", b.path,
		"--project-root", b.root, "--address", b.address, "--ready-fd", "3", "--die-after-spawn"}
	began := time.Now()
	_, _, err := b.startWith(StandingKey, args)
	if err == nil {
		t.Fatal("a supervisor that dies in the window reports no readiness")
	}
	// The readiness pipe is the supervisor's alone: the application it
	// spawned must not hold it open, or a launcher whose supervisor was
	// killed waits out its whole wait instead of hearing the pipe close.
	if waited := time.Since(began); waited > 10*time.Second {
		t.Fatalf("the launcher waited %s for a supervisor that was already dead", waited)
	}
	eventually(t, "the application to be running", 10*time.Second, func() bool {
		_, statErr := os.Stat(live)
		return statErr == nil
	})
	record := b.record(StandingKey)
	if record.Group <= 0 {
		t.Fatalf("the record written before the spawn names the group: %+v", record)
	}
	if record.Child != "" {
		t.Fatalf("a supervisor killed in the window recorded no child: %+v", record)
	}
	status := b.status(StandingKey)
	if status.State != Stale {
		t.Fatalf("status must read the window as stale, got %s", status.State)
	}
	want := "supervisor gone, group " + strconv.FormatInt(record.Group, 10) + ", child not recorded"
	if status.Problem != want {
		t.Fatalf("status must say %q, got %q", want, status.Problem)
	}
	if len(status.Members) == 0 {
		t.Fatal("the application must be discoverable in the recorded group by its identity")
	}
	for _, member := range status.Members {
		if _, err := identity.ParseRef(member.Ref); err != nil {
			t.Fatalf("a member is listed by its native identity, not a number: %q", member.Ref)
		}
	}
	var signalled []string
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{
		Probe: ProbeOnce, Wait: time.Second,
		Send: func(pid int, sig syscall.Signal) error {
			signalled = append(signalled, strconv.Itoa(pid)+":"+sig.String())
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	if len(signalled) != 0 {
		t.Fatalf("nothing in that group may be signalled: %v", signalled)
	}
	lines := strings.Join(result.Lines, "\n")
	if !strings.Contains(lines, "no application is recorded for this run; nothing was signalled") {
		t.Errorf("stop must say it signalled nothing:\n%s", lines)
	}
	if !strings.Contains(lines, "none of it was signalled") {
		t.Errorf("stop must name the group's members and refuse to signal them:\n%s", lines)
	}
	if result.Proven || result.Outcome != StillRunning {
		t.Fatalf("an application the engine did not end is never reported as stopped: %s", result.Outcome)
	}
	// What status found is the application itself, by its proven identity:
	// the member named is the process that wrote its pid, and that identity,
	// re-proven at the signal, is what stops it.
	written, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	var found *Member
	for index, member := range status.Members {
		if strconv.FormatInt(member.Pid, 10) == strings.TrimSpace(string(written)) {
			found = &status.Members[index]
		}
	}
	if found == nil {
		t.Fatalf("the application (pid %s) must be among the members found by identity: %+v", written, status.Members)
	}
	ref, err := identity.ParseRef(found.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGTERM); err != nil {
		t.Fatalf("the application is stoppable by the identity status named: %v", err)
	}
	eventually(t, "the application to end by its identity", 10*time.Second, func() bool {
		return identity.AliveRef(identity.KernelProber{}, ref) == identity.Dead
	})
	signalled = nil
	result, err = Stop(b.stateRoot, StandingKey, b.contract, StopOptions{
		Probe: ProbeOnce, Wait: time.Second,
		Send: func(pid int, sig syscall.Signal) error {
			signalled = append(signalled, strconv.Itoa(pid)+":"+sig.String())
			return nil
		}})
	if err != nil || !result.Proven || result.Outcome != StoppedNow || len(signalled) != 0 {
		t.Fatalf("with the application ended the stop is proven, and still nothing was signalled by the engine: %v %s %v\n%s",
			err, result.Outcome, signalled, strings.Join(result.Lines, "\n"))
	}
}

// Status says starting while a live supervisor has not yet seen readiness:
// alive is said, and ready is not.
func TestStatusSaysStartingBeforeReadiness(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address, "--ready-after", "3s"))
	started := make(chan error, 1)
	go func() {
		_, _, err := b.start(StandingKey)
		started <- err
	}()
	eventually(t, "the application to be recorded", 10*time.Second, func() bool {
		record, err := ReadRecord(b.stateRoot, StandingKey)
		return err == nil && record.Child != ""
	})
	status, err := Read(b.stateRoot, StandingKey, b.contract, ReadOptions{Probe: ProbeOnce})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != Starting || status.Readiness != NotYet {
		t.Fatalf("a supervisor alive before readiness reads as starting, got %s/%s", status.State, status.Readiness)
	}
	lines := strings.Join(status.Lines(), "\n")
	if !strings.Contains(lines, "state: starting") || !strings.Contains(lines, "readiness: not yet ready") {
		t.Fatalf("liveness and readiness are said separately while starting:\n%s", lines)
	}
	if err := <-started; err != nil {
		t.Fatalf("start: %v", err)
	}
	if status := b.status(StandingKey); status.State != Running || status.Readiness != Answering {
		t.Fatalf("after readiness: %s/%s", status.State, status.Readiness)
	}
}

// Each readiness form is proven with the fixture.
func TestTheFourReadinessForms(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	t.Run("http", func(t *testing.T) {
		t.Parallel()
		address := freePort(t)
		b := newBed(t, httpContract(app, address, "--ready-after", "500ms"))
		if _, _, err := b.start(StandingKey); err != nil {
			t.Fatalf("http readiness: %v", err)
		}
		if status := b.status(StandingKey); status.Readiness != Answering {
			t.Fatalf("http readiness must answer, got %s", status.Readiness)
		}
	})
	t.Run("tcp", func(t *testing.T) {
		t.Parallel()
		address := freePort(t)
		b := newBed(t, map[string]any{
			"address": address,
			"start":   map[string]any{"argv": []string{app, "--listen", "${address}", "--listen-after", "400ms"}},
			"ready":   map[string]any{"kind": "tcp", "address": "${address}"},
			"readyMs": 15000, "stopMs": 4000})
		if _, _, err := b.start(StandingKey); err != nil {
			t.Fatalf("tcp readiness: %v", err)
		}
		if status := b.status(StandingKey); status.Readiness != Answering {
			t.Fatalf("tcp readiness must answer, got %s", status.Readiness)
		}
	})
	t.Run("log", func(t *testing.T) {
		t.Parallel()
		b := newBed(t, map[string]any{
			"start":   map[string]any{"argv": []string{app, "--no-listen", "--ready-line", "READY", "--ready-after", "400ms"}},
			"ready":   map[string]any{"kind": "log", "pattern": "^READY$"},
			"readyMs": 15000, "stopMs": 4000})
		if _, _, err := b.start(StandingKey); err != nil {
			t.Fatalf("log readiness: %v", err)
		}
		status := b.status(StandingKey)
		if status.State != Running || status.Readiness != ObservedOnce {
			t.Fatalf("a log form is a startup observation: got %s/%s", status.State, status.Readiness)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		b := newBed(t, map[string]any{
			"start":  map[string]any{"argv": []string{app, "--no-listen"}},
			"stopMs": 4000})
		if _, _, err := b.start(StandingKey); err != nil {
			t.Fatalf("the none form means alive is ready: %v", err)
		}
		if status := b.status(StandingKey); status.State != Running {
			t.Fatalf("the none form must read as running, got %s", status.State)
		}
	})
	t.Run("a start that exits before readiness", func(t *testing.T) {
		t.Parallel()
		address := freePort(t)
		b := newBed(t, httpContract(app, address, "--exit-now"))
		_, _, err := b.start(StandingKey)
		if err == nil {
			t.Fatal("a start command that exits before readiness is not a running application")
		}
		if !strings.Contains(err.Error(), "exited before") {
			t.Fatalf("the refusal must say the start command exited: %v", err)
		}
		if status := b.status(StandingKey); status.State == Running {
			t.Fatal("it must never be reported as running")
		}
	})
}

// A log readiness form is scoped to the offset this run's supervisor opened
// the log at: a line an earlier run wrote can never make this one ready.
func TestLogReadinessIsScopedToThisRun(t *testing.T) {
	t.Parallel()
	app := mustApp(t)
	b := newBed(t, map[string]any{
		"start":   map[string]any{"argv": []string{app, "--no-listen"}},
		"ready":   map[string]any{"kind": "log", "pattern": "^READY$"},
		"readyMs": 1500, "stopMs": 3000})
	logPath := DefaultLogPath(b.stateRoot, StandingKey)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("READY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.start(StandingKey); err == nil {
		t.Fatal("an earlier run's READY line must not make this run ready")
	}
}

// Status says running and not answering, with the time the last answer was
// observed, rather than calling a dark application stopped.
func TestRunningAndNotAnswering(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address, "--dark-after", "1s"))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	eventually(t, "the application to stop answering", 10*time.Second, func() bool {
		return b.status(StandingKey).Readiness == NotAnswering
	})
	status := b.status(StandingKey)
	if status.State != Running {
		t.Fatalf("a dark application is still running, got %s", status.State)
	}
	lines := strings.Join(status.Lines(), "\n")
	if !strings.Contains(lines, "readiness: not answering since ") {
		t.Fatalf("status must say since when it stopped answering:\n%s", lines)
	}
}

// A record whose process was killed behind the engine's back reads as such.
func TestAProcessKilledBehindTheEnginesBack(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	record := b.record(StandingKey)
	ref, _, err := record.ChildRef()
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end into an ended record", 10*time.Second, func() bool {
		status := b.status(StandingKey)
		return status.State == Finished || status.State == ChildEnded
	})
	status := b.status(StandingKey)
	if status.State == Running {
		t.Fatal("a killed application is never reported as running")
	}
}

// A recorded pid a different process has taken is refused by name and never
// signalled: the identity is re-proven immediately before every signal.
func TestARecordedPidReusedByAnUnrelatedProcessIsRefusedByName(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	record := b.record(StandingKey)
	ref, _, err := record.ChildRef()
	if err != nil {
		t.Fatal(err)
	}
	// The same number with a start time that is not this process's: exactly
	// what a reused pid looks like to a reader that re-proves identity.
	stale := ref
	if stale.StartedAtUnixMicro > 0 {
		stale.StartedAtUnixMicro -= 1_000_000
		stale.StartedAtSec = stale.StartedAtUnixMicro / 1_000_000
	} else {
		stale.StartTicks += 1000
	}
	encoded, err := identity.EncodeRef(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRecord(b.stateRoot, StandingKey, func(current *Record) { current.Child = encoded }); err != nil {
		t.Fatal(err)
	}
	var signalled []string
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{
		Probe: ProbeOnce, Wait: time.Second,
		Send: func(pid int, sig syscall.Signal) error {
			signalled = append(signalled, strconv.Itoa(pid)+":"+sig.String())
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	for _, sent := range signalled {
		if strings.HasPrefix(sent, strconv.FormatInt(ref.Pid, 10)+":") {
			t.Fatalf("a pid whose identity no longer matches was signalled: %v", signalled)
		}
	}
	lines := strings.Join(result.Lines, "\n")
	want := "the recorded application (pid " + strconv.FormatInt(ref.Pid, 10) + ") is gone"
	if !strings.Contains(lines, want) {
		t.Fatalf("stop must refuse it by name (%q):\n%s", want, lines)
	}
}

// Stop proves death for a child that ignores TERM, and reports nothing as
// stopped that is not.
func TestStopProvesDeathForAChildThatIgnoresTERM(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address, "--ignore-term"))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	record := b.record(StandingKey)
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{Probe: ProbeOnce, Wait: 25 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != StoppedNow || !result.Proven {
		t.Fatalf("stop must prove death, got %s\n%s", result.Outcome, strings.Join(result.Lines, "\n"))
	}
	lines := strings.Join(result.Lines, "\n")
	// A child that took TERM would have left the tree quiet at once; that
	// this stop had to go on to the supervisor is the proof TERM was ignored.
	if !strings.Contains(lines, "asked the supervisor to end its own group") {
		t.Errorf("a child that ignores TERM must be escalated past it:\n%s", lines)
	}
	if !strings.Contains(lines, "the readiness probe is dark") {
		t.Errorf("for a probed form, stopping is proven with the probe dark too:\n%s", lines)
	}
	prober := identity.KernelProber{}
	if ref, _, err := record.ChildRef(); err == nil && identity.AliveRef(prober, ref) == identity.Alive {
		t.Fatal("the application is still alive after a proven stop")
	}
	if record.Ended == nil && b.record(StandingKey).Ended == nil {
		t.Error("a stopped run ends into an ended record")
	}
}

// Stop against a foreground wrapper whose descendant survives TERM ends the
// descendant through the supervisor's own group, and the record is kept
// until the group is empty.
func TestStopEndsTheOwnedTreeThroughTheSupervisorsGroup(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	live := filepath.Join(t.TempDir(), "wrapper")
	b := newBed(t, httpContract(mustApp(t), address, "--spawn-descendant", "--live-file", live))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	descendant := live + ".descendant"
	eventually(t, "the descendant to be running", 10*time.Second, func() bool {
		_, err := os.Stat(descendant)
		return err == nil
	})
	record := b.record(StandingKey)
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{Probe: ProbeOnce, Wait: 25 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != StoppedNow {
		t.Fatalf("the owned tree must be ended, got %s\n%s", result.Outcome, strings.Join(result.Lines, "\n"))
	}
	lines := strings.Join(result.Lines, "\n")
	if !strings.Contains(lines, "asked the supervisor to end its own group") {
		t.Errorf("only the group's living leader may signal the group:\n%s", lines)
	}
	if members, err := KernelGroup(record.Group); err != nil || len(members) != 0 {
		t.Fatalf("the group must have no member left: %v %v", members, err)
	}
	if _, err := ReadRecord(b.stateRoot, StandingKey); err != nil {
		t.Fatalf("the record is kept until the caller has copied the run's evidence: %v", err)
	}
}

// An application that exits by itself leaves an ended record with its exit
// status, and its log is still readable.
func TestAnApplicationThatExitsByItselfLeavesAnEndedRecord(t *testing.T) {
	t.Parallel()
	b := newBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{mustApp(t), "--no-listen", "--exit-after", "700ms", "--exit-code", "3"}},
		"stopMs": 3000})
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	eventually(t, "the run to end into an ended record", 15*time.Second, func() bool {
		return b.status(StandingKey).State == Finished
	})
	record := b.record(StandingKey)
	if record.Ended == nil || record.Ended.ExitStatus != "exit 3" {
		t.Fatalf("the ended record carries the exit status: %+v", record.Ended)
	}
	if record.Ended.At == "" {
		t.Error("and the time it ended")
	}
	if lines, err := Tail(record.Log, 20); err != nil || len(lines) == 0 {
		t.Fatalf("log still reads an ended run's log: %v %v", lines, err)
	}
}

// A second start rejoins the run that is live and still waits for readiness.
func TestASecondStartRejoinsAndWaitsForReadiness(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address, "--ready-after", "500ms"))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	first := b.record(StandingKey)
	status, err := Rejoin(context.Background(), b.stateRoot, StandingKey, b.contract,
		ReadOptions{Probe: ProbeOnce}, 10*time.Second)
	if err != nil {
		t.Fatalf("a second start rejoins the live run: %v", err)
	}
	if status.Readiness != Answering {
		t.Fatalf("a rejoin still waits for readiness, got %s", status.Readiness)
	}
	if second := b.record(StandingKey); second.Child != first.Child {
		t.Fatal("a rejoin does not start a second application")
	}
}

// A launcher that dies after the spawn leaves a supervisor that owns the run:
// the record was written before anything was spawned.
func TestALauncherThatDiesLeavesTheSupervisorOwningTheRun(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	// LaunchSupervisor has returned and released the child; the launcher is
	// out of the picture from here.
	record := b.record(StandingKey)
	prober := identity.KernelProber{}
	ref, err := record.SupervisorRef()
	if err != nil {
		t.Fatal(err)
	}
	if identity.AliveRef(prober, ref) != identity.Alive {
		t.Fatal("the supervisor owns the run after its launcher is gone")
	}
	if status := b.status(StandingKey); status.State != Running {
		t.Fatalf("the run is owned and running, got %s", status.State)
	}
}

// An engine that never got a supervisor started has left no run, and the
// next status says stopped.
func TestAnEngineKilledBeforeTheSupervisorStartedLeavesNoRun(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address))
	spec := LaunchSpec{Executable: filepath.Join(b.root, "no-such-engine"), Args: nil,
		LogPath: filepath.Join(Dir(b.stateRoot), "standing.launch.log")}
	if _, _, err := LaunchSupervisor(spec, ExecSpawn, time.Second); err == nil {
		t.Fatal("a supervisor that cannot be started is a refusal")
	}
	if status := b.status(StandingKey); status.State != Stopped {
		t.Fatalf("no run was left behind; status must say stopped, got %s", status.State)
	}
	if _, err := os.Stat(RecordPath(b.stateRoot, StandingKey)); !os.IsNotExist(err) {
		t.Fatal("no record was written")
	}
}

// Restart replaces the process and the record.
func TestRestartReplacesTheProcessAndTheRecord(t *testing.T) {
	t.Parallel()
	address := freePort(t)
	b := newBed(t, httpContract(mustApp(t), address))
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	first := b.record(StandingKey)
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{Probe: ProbeOnce, Wait: 25 * time.Second})
	if err != nil || !result.Proven {
		t.Fatalf("stop before start: %v %s", err, result.Outcome)
	}
	if err := RemoveRecord(b.stateRoot, StandingKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("restart: %v", err)
	}
	second := b.record(StandingKey)
	if second.Child == first.Child || second.Supervisor == first.Supervisor {
		t.Fatal("a restart replaces both the process and the record")
	}
}

// A log readiness form is proven stopped by death alone: nothing asks a
// pattern that cannot unmatch whether it has gone dark.
func TestALogReadinessFormStopsOnDeathAlone(t *testing.T) {
	t.Parallel()
	b := newBed(t, map[string]any{
		"start":   map[string]any{"argv": []string{mustApp(t), "--no-listen", "--ready-line", "READY"}},
		"ready":   map[string]any{"kind": "log", "pattern": "^READY$"},
		"readyMs": 15000, "stopMs": 4000})
	if _, _, err := b.start(StandingKey); err != nil {
		t.Fatalf("start: %v", err)
	}
	probed := false
	result, err := Stop(b.stateRoot, StandingKey, b.contract, StopOptions{
		Wait:  25 * time.Second,
		Probe: func(Contract, string) error { probed = true; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if probed {
		t.Error("a log form has no probe to ask")
	}
	if result.Outcome != StoppedNow {
		t.Fatalf("death alone proves it stopped, got %s\n%s", result.Outcome, strings.Join(result.Lines, "\n"))
	}
}

// log prints the captured tail and follows it.
func TestLogTailAndFollow(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, 2)
	if err != nil || strings.Join(lines, ",") != "two,three" {
		t.Fatalf("tail: %v %v", lines, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out strings.Builder
	done := make(chan error, 1)
	go func() { done <- Follow(ctx, path, &out, 20*time.Millisecond) }()
	time.Sleep(100 * time.Millisecond)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("four\n"); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	eventually(t, "follow to print the new line", 3*time.Second, func() bool {
		return strings.Contains(out.String(), "four")
	})
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
