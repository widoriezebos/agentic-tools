package proofrun

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestResourceCustodySettlesWatchdogBeforeFinalCensus(t *testing.T) {
	done := filepath.Join(t.TempDir(), "suite.done")
	watching := make(chan error, 1)
	releaseWatchdog := make(chan struct{})
	go func() {
		<-releaseWatchdog
		watching <- errors.New("watchdog verdict retained")
	}()
	type outcome struct {
		settled bool
		err     error
	}
	finishedWatchdog := make(chan outcome, 1)
	go func() {
		settled, err := settleResourceWatchdog(watching, done, time.Second)
		finishedWatchdog <- outcome{settled, err}
	}()
	close(releaseWatchdog)
	result := <-finishedWatchdog
	if !result.settled || result.err == nil || !strings.Contains(result.err.Error(), "watchdog verdict retained") {
		t.Fatalf("settled=%t err=%v; final census could precede watchdog shutdown", result.settled, result.err)
	}
	neverSettles := make(chan error)
	settled, err := settleResourceWatchdog(neverSettles, filepath.Join(t.TempDir(), "timeout.done"), 20*time.Millisecond)
	if settled || err == nil || !strings.Contains(err.Error(), "did not settle before final custody drain") {
		t.Fatalf("timeout settled=%t err=%v; marker must remain dirty", settled, err)
	}
	// The launcher may have published done first; the custodian's notification
	// must be idempotent while it still joins the watchdog before final census.
	existing := filepath.Join(t.TempDir(), "already-done")
	if err := os.WriteFile(existing, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	finished <- nil
	settled, err = settleResourceWatchdog(finished, existing, time.Second)
	if !settled || err != nil {
		t.Fatalf("existing done marker settled=%t err=%v", settled, err)
	}
}

func TestResourceCustodyClassifiesExactSuiteTerminalStates(t *testing.T) {
	t.Parallel()
	started := time.Unix(1700000000, 123000)
	suite := identity.Exact{Pid: 77, StartedAt: started}.Ref()
	for _, specimen := range []struct {
		name      string
		exact     identity.Exact
		state     identity.Liveness
		probeErr  error
		wantEnded bool
		wantError bool
	}{
		{name: "live", exact: identity.Exact{Pid: 77, StartedAt: started}, state: identity.Alive},
		{name: "zombie", exact: identity.Exact{Pid: 77, StartedAt: started, Zombie: true}, state: identity.Alive, wantEnded: true},
		{name: "exiting", exact: identity.Exact{Pid: 77, StartedAt: started, Exiting: true}, state: identity.Alive, wantEnded: true},
		{name: "dead", state: identity.Dead, wantEnded: true},
		{name: "replaced", exact: identity.Exact{Pid: 77, StartedAt: started.Add(time.Second)}, state: identity.Alive, wantEnded: true},
		{name: "unknown", state: identity.Unknown, probeErr: errors.New("probe denied"), wantError: true},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			ended, err := resourceCustodySuiteEnded(functionIdentityProber(func(int64) (identity.Exact, identity.Liveness, error) {
				return specimen.exact, specimen.state, specimen.probeErr
			}), suite)
			if ended != specimen.wantEnded || (err != nil) != specimen.wantError {
				t.Fatalf("ended=%t err=%v wantEnded=%t wantError=%t", ended, err, specimen.wantEnded, specimen.wantError)
			}
		})
	}
}

func TestResourceCustodianStopsDiscoveryBeforeReplacementGroup(t *testing.T) {
	t.Parallel()
	clock := time.Unix(1800000000, 0)
	old := identity.Exact{Pid: 101, StartedAt: time.Unix(1700000000, 0)}
	replacement := identity.Exact{Pid: 202, StartedAt: time.Unix(1700000100, 0)}
	oldDead, censuses := false, 0
	var signaled []int
	prober := functionIdentityProber(func(pid int64) (identity.Exact, identity.Liveness, error) {
		switch pid {
		case old.Pid:
			if oldDead {
				return identity.Exact{}, identity.Dead, nil
			}
			return old, identity.Alive, nil
		case replacement.Pid:
			return replacement, identity.Alive, nil
		default:
			return identity.Exact{}, identity.Dead, nil
		}
	})
	observed, err := drainResourceGroupWith(999, prober, time.Second, func(int64) ([]int64, error) {
		censuses++
		switch censuses {
		case 1:
			return []int64{old.Pid}, nil
		case 2, 3:
			return nil, nil
		default:
			// A reused numeric group exists only after the old owner completed
			// its stable empty census. The owner must never make this call.
			return []int64{replacement.Pid}, nil
		}
	}, func(pid int, signal syscall.Signal) error {
		signaled = append(signaled, pid)
		if int64(pid) == old.Pid && signal == syscall.SIGKILL {
			oldDead = true
		}
		return nil
	}, func() time.Time { return clock }, func() { clock = clock.Add(time.Millisecond) })
	seenReplacement, state, probeErr := prober.Probe(replacement.Pid)
	if err != nil || !observed || censuses != 3 || len(signaled) != 1 || int64(signaled[0]) != old.Pid ||
		probeErr != nil || state != identity.Alive || !identity.SameIdentity(seenReplacement, replacement.Ref()) {
		t.Fatalf("observed=%t censuses=%d signals=%v replacementState=%s probe=%v err=%v", observed, censuses, signaled, state, probeErr, err)
	}
}

func TestResourceCustodyRepeatsDetachedFixtureCensusAfterKill(t *testing.T) {
	prober := identity.KernelProber{}
	owner, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("owner probe: %v (%s)", err, state)
	}
	type heldFixture struct {
		command *exec.Cmd
		release *os.File
		joined  bool
	}
	start := func() (*heldFixture, identity.Ref) {
		t.Helper()
		readyReader, readyWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		releaseReader, releaseWriter, err := os.Pipe()
		if err != nil {
			_ = readyReader.Close()
			_ = readyWriter.Close()
			t.Fatal(err)
		}
		command := exec.Command("sh", "-c", `printf R >&3; exec cat <&4`)
		command.ExtraFiles = []*os.File{readyWriter, releaseReader}
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := command.Start(); err != nil {
			_ = readyReader.Close()
			_ = readyWriter.Close()
			_ = releaseReader.Close()
			_ = releaseWriter.Close()
			t.Fatal(err)
		}
		subject := &heldFixture{command: command, release: releaseWriter}
		t.Cleanup(func() {
			_ = subject.release.Close()
			if !subject.joined {
				_ = subject.command.Wait()
				subject.joined = true
			}
		})
		_ = readyWriter.Close()
		_ = releaseReader.Close()
		var ready [1]byte
		_, readyErr := io.ReadFull(readyReader, ready[:])
		closeErr := readyReader.Close()
		if readyErr != nil || closeErr != nil || ready[0] != 'R' {
			t.Fatalf("fixture readiness=%q read=%v close=%v", ready, readyErr, closeErr)
		}
		exact, state, err := prober.Probe(int64(command.Process.Pid))
		if err != nil || state != identity.Alive || !exact.Ref().NativeExact() {
			t.Fatalf("fixture probe: %v (%s)", err, state)
		}
		return subject, exact.Ref()
	}
	firstSubject, first := start()
	lateSubject, late := start()
	fixture := func(ref identity.Ref) identity.FixtureSurvivor {
		return identity.FixtureSurvivor{Class: identity.FixtureSurvivorCertain, Ref: ref,
			Key: identity.FixtureKey{Owner: owner.Ref()}}
	}
	passes := 0
	observed, err := drainCustodyFixtureScans(ResourceCustodyOptions{Launcher: owner.Ref()}, prober,
		func() ([]identity.FixtureSurvivor, error) {
			passes++
			switch passes {
			case 1:
				return []identity.FixtureSurvivor{fixture(first)}, nil
			case 2:
				// This process became visible only after the first exact kill.
				return []identity.FixtureSurvivor{fixture(late)}, nil
			default:
				return nil, nil
			}
		})
	if err != nil || !observed || passes < 4 {
		t.Fatalf("fixture drain observed=%t passes=%d err=%v", observed, passes, err)
	}
	for name, subject := range map[string]*heldFixture{"first": firstSubject, "late": lateSubject} {
		waitErr := subject.command.Wait()
		subject.joined = true
		if waitErr == nil {
			t.Fatalf("%s fixture exited without the production custody signal", name)
		}
	}
	for name, ref := range map[string]identity.Ref{"first": first, "late": late} {
		exact, state, probeErr := prober.Probe(ref.Pid)
		if probeErr != nil || state == identity.Unknown || state == identity.Alive && identity.SameIdentity(exact, ref) {
			t.Fatalf("%s fixture was not reaped after custody drain: state=%s same-identity=%t err=%v", name,
				state, identity.SameIdentity(exact, ref), probeErr)
		}
	}
	passes = 0
	_, err = drainCustodyFixtureScans(ResourceCustodyOptions{Launcher: owner.Ref()}, prober,
		func() ([]identity.FixtureSurvivor, error) {
			passes++
			if passes == 1 {
				return nil, nil
			}
			return nil, errors.New("fixture census unreadable")
		})
	if err == nil || !strings.Contains(err.Error(), "fixture census unreadable") || passes != 2 {
		t.Fatalf("unreadable second census passes=%d err=%v; marker must remain dirty", passes, err)
	}
}

func TestResourceCustodyDrainsDetachedFixtureOfLiveLauncher(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	owner, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("live owner probe: %v (%s)", err, state)
	}
	start := func(key identity.FixtureKey) identity.Ref {
		t.Helper()
		tag, err := identity.EncodeKey(key)
		if err != nil {
			t.Fatal(err)
		}
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		// The tag is an argv word: the shell waits in a builtin read, so it
		// remains the exact detached fixture process until custody kills it.
		command := exec.Command("sh", "-c", "read never", identity.FixtureOwnerEnv+"="+tag)
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdin = reader
		if err := command.Start(); err != nil {
			_ = reader.Close()
			_ = writer.Close()
			t.Fatal(err)
		}
		_ = reader.Close()
		exact, state, err := prober.Probe(int64(command.Process.Pid))
		if err != nil || state != identity.Alive {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("detached fixture probe: %v (%s)", err, state)
		}
		t.Cleanup(func() { _ = command.Process.Kill(); _ = writer.Close(); _ = command.Wait() })
		return exact.Ref()
	}
	owned := start(identity.FixtureKey{Owner: owner.Ref(), Test: "TestDetached", Nonce: "00000001"})
	foreignOwner := owner.Ref()
	foreignOwner.Pid += 1_000_000
	foreign := start(identity.FixtureKey{Owner: foreignOwner, Test: "TestForeign", Nonce: "00000002"})
	observed, err := drainCustodyFixtures(ResourceCustodyOptions{Launcher: owner.Ref(), ConfPath: conf}, prober)
	if err != nil || !observed || liveCustodyRef(prober, owned) || !liveCustodyRef(prober, foreign) {
		t.Fatalf("live-owner fixture drain observed=%t err=%v ownLive=%t foreignLive=%t", observed, err,
			liveCustodyRef(prober, owned), liveCustodyRef(prober, foreign))
	}
}

func TestResourceCustodyNativeLaunchDrainsLiveOwnerFixtureBeforeMarkerClean(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	workerSource := filepath.Join(root, "detached-worker.go")
	worker := filepath.Join(root, "detached-worker")
	const source = `package main
import("os";"os/exec";"strconv";"syscall";"time")
func main(){
 if len(os.Args)>1 && os.Args[1]=="--fixture" { for { time.Sleep(time.Hour) } }
 if len(os.Args)!=3 { os.Exit(2) }
 child:=exec.Command(os.Args[0],"--fixture","METASYSTEM_FIXTURE_OWNER="+os.Args[1]); child.SysProcAttr=&syscall.SysProcAttr{Setsid:true}
 if err:=child.Start(); err!=nil { os.Exit(3) }
 if err:=os.WriteFile(os.Args[2],[]byte(strconv.Itoa(child.Process.Pid)),0600); err!=nil { os.Exit(4) }
}`
	if err := os.WriteFile(workerSource, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-p=2", "-o", worker, workerSource)
	build.Env = append(os.Environ(), "GOMAXPROCS=2")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build detached worker: %v: %s", err, output)
	}
	prober := identity.KernelProber{}
	owner, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("launcher identity: %v (%s)", err, state)
	}
	tag, err := identity.EncodeKey(identity.FixtureKey{Owner: owner.Ref(), Test: "TestNativeDetached", Nonce: "00000003"})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	childPID := filepath.Join(root, "fixture.pid")
	logPath := filepath.Join(root, "launcher.log")
	status := LaunchSuite(LaunchOptions{Suite: "detached-live-owner", Root: root, ConfPath: conf,
		ProgressPath: filepath.Join(root, "progress.jsonl"), LogPath: logPath,
		Banner: "detached fixture custody", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
		TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
		WatchdogExecutable: engine, Command: []string{worker, tag, childPID},
		HostResourceFiles: lease.Files(), Output: io.Discard, ErrorOutput: io.Discard})
	data, err := os.ReadFile(childPID)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := prober.Probe(pid)
	if err == nil && state == identity.Alive {
		t.Cleanup(func() { _ = identity.SignalExact(prober, exact.Ref(), syscall.SIGKILL) })
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if status == 0 || liveCustodyRef(prober, exact.Ref()) || !strings.Contains(string(logData), "declared fixture descendants survived direct worker completion") {
		_, _, tagged := identity.FixtureTag(exact)
		t.Fatalf("native custody status=%d fixtureLive=%t tagged=%t argvKnown=%t argv=%q envKnown=%t log=%s", status,
			liveCustodyRef(prober, exact.Ref()), tagged, exact.ArgvKnown, exact.Argv, exact.EnvironKnown, logData)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireHostResources(t.Context(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatalf("clean fixture left capacity held: %v", err)
	}
	defer next.Close()
}
