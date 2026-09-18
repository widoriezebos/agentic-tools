package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func commandSurvivorRef(pid, token int64) identity.Ref {
	if runtime.GOOS == "linux" {
		return identity.Ref{Pid: pid, StartTicks: token, BootID: "fixture-boot"}
	}
	return identity.Ref{Pid: pid, StartedAtSec: token, StartedAtUnixMicro: token * 1_000_000}
}

func waitFixtureRef(t *testing.T, ref identity.Ref, want identity.Liveness) {
	t.Helper()
	prober := identity.KernelProber{}
	ticker, deadline := time.NewTicker(10*time.Millisecond), time.NewTimer(10*time.Second)
	defer ticker.Stop()
	defer deadline.Stop()
	for {
		exact, state, err := prober.Probe(ref.Pid)
		same := state == identity.Alive && identity.SameIdentity(exact, ref)
		_, _, tagged := identity.FixtureTag(exact)
		if err == nil && (want == identity.Dead && (state == identity.Dead || state == identity.Alive && (!same || exact.Zombie)) || want == identity.Alive && same && !exact.Zombie && tagged) {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("pid %d state=%s zombie=%t same-identity=%t err=%v, want %s", ref.Pid, state, exact.Zombie, same, err, want)
		}
	}
}

func commandSurvivorProcess(pid, token int64) census.Process {
	process := census.Process{Pid: pid, PPID: 1, PGID: pid, Started: token, StartedExactMicro: token * 1_000_000, Argv: "/bin/sh", Alive: true}
	if runtime.GOOS == "linux" {
		process.StartTicks, process.BootID = token, "fixture-boot"
	}
	return process
}

func commandSurvivorTag(t *testing.T, key identity.FixtureKey) string {
	t.Helper()
	encoded, err := identity.EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return identity.FixtureOwnerEnv + "=" + encoded
}

func writeCommandProcessFile(t *testing.T, path string, processes []census.Process) {
	t.Helper()
	data, err := json.Marshal(processes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProcFixtureSurvivorsVerb(t *testing.T) {
	originalSignal := fixtureSurvivorSignal
	fixtureSurvivorSignal = func(int, syscall.Signal) error { return nil }
	t.Cleanup(func() { fixtureSurvivorSignal = originalSignal })
	_, root := runnableSeparateProcessScopeFixture(t)
	processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE")
	base := int64(os.Getpid()) * 10
	deadOwner := commandSurvivorRef(base+1, 101)
	liveOwner := commandSurvivorRef(base+2, 102)
	deadKey := identity.FixtureKey{Owner: deadOwner, Test: "TestDead", Nonce: "00000001"}
	otherKey := identity.FixtureKey{Owner: deadOwner, Test: "TestOther", Nonce: "00000003"}
	liveKey := identity.FixtureKey{Owner: liveOwner, Test: "TestLive", Nonce: "00000002"}
	dead := commandSurvivorProcess(base+10, 110)
	dead.Environ = []string{commandSurvivorTag(t, deadKey)}
	other := commandSurvivorProcess(base+13, 113)
	other.Environ = []string{commandSurvivorTag(t, otherKey)}
	live := commandSurvivorProcess(base+11, 111)
	live.Environ = []string{commandSurvivorTag(t, liveKey)}
	liveOwnerRow := commandSurvivorProcess(liveOwner.Pid, 102)
	unowned := commandSurvivorProcess(base+12, 112)
	unowned.Exe = filepath.Join(t.TempDir(), "go-tmp", "TestOther99", "child")
	writeCommandProcessFile(t, processFile, []census.Process{dead, live, liveOwnerRow, unowned, other})
	ownerText, _ := identity.EncodeRef(deadOwner)
	liveOwnerText, _ := identity.EncodeRef(liveOwner)
	keyText, _ := identity.EncodeKey(deadKey)

	run := func(args ...string) (string, string, int) {
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runFixtureSurvivors(append(args, "--root", root))
		})
		return stdout, stderr, code
	}
	stdout, stderr, code := run()
	if code != 1 || stderr != "" || !strings.Contains(stdout, "fixture-survivor pid=") || !strings.Contains(stdout, "unowned-in-cache pid=") || strings.Contains(stdout, fmt.Sprintf("pid=%d ", live.Pid)) {
		t.Fatalf("whole-table verb = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = run("--owner", ownerText)
	if code != 1 || stderr != "" || !strings.Contains(stdout, fmt.Sprintf("pid=%d ", dead.Pid)) || strings.Contains(stdout, "unowned-in-cache") {
		t.Fatalf("owner verb = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = run("--key", keyText)
	if code != 1 || stderr != "" || !strings.Contains(stdout, "key=TestDead/00000001") || strings.Contains(stdout, "TestLive") || strings.Contains(stdout, "TestOther") {
		t.Fatalf("key verb = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = run("--owner", liveOwnerText)
	if code != 2 || stdout != "" || !strings.Contains(stderr, liveOwnerText) || !strings.Contains(stderr, "alive") {
		t.Fatalf("live-owner refusal = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = run("--reap")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--reap is refused") {
		t.Fatalf("fixture reap refusal = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = run("--owner", ownerText, "--key", keyText)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "usage:") {
		t.Fatalf("two-selector refusal = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = run()
	if code != 2 || stdout != "" || !strings.Contains(stderr, "allowed only when metasystem.runtimes=fake") {
		t.Fatalf("unauthorized table = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(processFile, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = run()
	if code != 2 || stdout != "" || !strings.Contains(stderr, "process table is unreadable") {
		t.Fatalf("unreadable table = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	writeCommandProcessFile(t, processFile, []census.Process{live, liveOwnerRow})
	stdout, stderr, code = run()
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("no survivor = code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestProcFixtureSurvivorsReapsALiveSurvivor(t *testing.T) {
	directory := t.TempDir()
	fifo := filepath.Join(directory, "hold")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	owner := exec.Command("/bin/sh", "-c", `IFS= read -r tag; export METASYSTEM_FIXTURE_OWNER="$tag"; /usr/bin/perl -MPOSIX -e 'POSIX::setsid(); exec @ARGV' -- /bin/sh -c 'exec 3<"$1"; read line <&3' sh "$1" "METASYSTEM_FIXTURE_OWNER=$tag" & printf '%s\n' "$!"; wait`, "sh", fifo)
	owner.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	input, err := owner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	var ownerWaitOnce sync.Once
	var ownerWaitErr error
	ownerReaped := false
	waitOwner := func() error {
		ownerWaitOnce.Do(func() {
			ownerWaitErr = owner.Wait()
			ownerReaped = true
		})
		return ownerWaitErr
	}
	t.Cleanup(func() {
		if !ownerReaped {
			_ = syscall.Kill(-owner.Process.Pid, syscall.SIGKILL)
		}
		_ = waitOwner()
	})
	ownerExact, state, err := (identity.KernelProber{}).Probe(int64(owner.Process.Pid))
	ownerRef := ownerExact.Ref()
	if ownerRef.NativeExact() {
		t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ownerRef, syscall.SIGKILL) })
	}
	if err != nil || state != identity.Alive {
		t.Fatalf("owner probe state=%s err=%v", state, err)
	}
	key := identity.FixtureKey{Owner: ownerRef, Test: t.Name(), Nonce: "a1b2c3d4"}
	encodedKey, _ := identity.EncodeKey(key)
	if _, err := fmt.Fprintln(input, encodedKey); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	childExact, state, err := (identity.KernelProber{}).Probe(pid)
	childRef := childExact.Ref()
	if childRef.NativeExact() {
		t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, childRef, syscall.SIGKILL) })
	}
	if err != nil || state != identity.Alive {
		t.Fatalf("child probe state=%s err=%v", state, err)
	}
	waitFixtureRef(t, childRef, identity.Alive)
	if err := identity.SignalExact(identity.KernelProber{}, ownerRef, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = waitOwner()
	waitFixtureRef(t, ownerRef, identity.Dead)
	waitFixtureRef(t, childRef, identity.Alive)
	root := t.TempDir()
	ownerText, _ := identity.EncodeRef(ownerRef)
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runFixtureSurvivors([]string{"--owner", ownerText, "--reap", "--root", root}) })
	if code != 1 || stderr != "" || !strings.Contains(stdout, fmt.Sprintf("pid=%d ", childRef.Pid)) {
		t.Fatalf("reap = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	waitFixtureRef(t, childRef, identity.Dead)
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int { return runFixtureSurvivors([]string{"--owner", ownerText, "--root", root}) })
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("second scan = code %d stdout %q stderr %q", code, stdout, stderr)
	}
}
