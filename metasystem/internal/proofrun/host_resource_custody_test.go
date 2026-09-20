package proofrun

import (
	"context"
	"fmt"
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

// A native grandchild can close every inherited lease descriptor and standard
// stream. Its lifetime must still hold the host slot when both the launcher
// and its direct worker are killed. Exact process references make cleanup safe
// even if a PID is recycled during this real-process test.
func TestGLEHostResourceKilledLauncherAndWorkerKeepOrdinaryGrandchildInCustody(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	directory, conf := isolatedHostResources(t)
	if err := os.WriteFile(filepath.Join(directory, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workerPID := filepath.Join(directory, "worker.pid")
	grandPID := filepath.Join(directory, "grandchild.pid")
	ready := filepath.Join(directory, "grandchild.ready")
	launcherLog, err := os.Create(filepath.Join(directory, "launcher-helper.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer launcherLog.Close()
	launcher := exec.Command(os.Args[0], "-test.run=^TestGLEHostResourceCustodyProcessHelper$", "--", "launcher", directory, conf, engine, workerPID, grandPID, ready)
	launcher.Env = append(os.Environ(),
		"METASYSTEM_HOST_CUSTODY_HELPER=1")
	launcher.Stdout, launcher.Stderr = launcherLog, launcherLog
	if err := launcher.Start(); err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	launcherExact, state, err := prober.Probe(int64(launcher.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe launcher: %v (%s)", err, state)
	}
	var worker, grandchild, watchdogProcess identity.Ref
	launcherWaited := false
	t.Cleanup(func() {
		for _, ref := range []identity.Ref{grandchild, worker, watchdogProcess, launcherExact.Ref()} {
			if ref.Pid > 0 {
				_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
			}
		}
		if !launcherWaited {
			_ = launcher.Wait()
		}
	})
	waitCustodyFile(t, ready, 8*time.Second)
	worker = waitCustodyRef(t, prober, workerPID, 3*time.Second)
	grandchild = waitCustodyRef(t, prober, grandPID, 3*time.Second)
	watchdogProcess = waitCustodyRecordWatchdog(t, directory, "custody-killed-launcher", 3*time.Second)
	assertDurableSpools := func() {
		deadline := time.Now().Add(3 * time.Second)
		for liveCustodyRef(prober, watchdogProcess) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if liveCustodyRef(prober, watchdogProcess) {
			t.Fatal("resource custodian remained live after capacity cleanup")
		}
		run, err := ReadLatestProgressRun(filepath.Join(directory, "progress.jsonl"))
		if err != nil || len(run.Header.LogPaths) != 5 {
			t.Fatalf("lost launcher spool inventory=%+v err=%v", run.Header, err)
		}
		for _, path := range run.Header.LogPaths[1:] {
			if _, err := os.ReadFile(path); err != nil {
				t.Fatalf("lost launcher spool %s is unreadable: %v", path, err)
			}
		}
	}
	if err := identity.SignalExact(prober, launcherExact.Ref(), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if err := launcher.Wait(); err == nil {
		t.Fatal("killed launcher exited successfully")
	}
	launcherWaited = true
	if identity.AliveRef(prober, worker) == identity.Alive {
		if err := identity.SignalExact(prober, worker, syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	go func() {
		lease, err := AcquireHostResources(ctx, directory, conf, "heavy", []string{"fixture-db"})
		if err != nil {
			errs <- err
			return
		}
		acquired <- lease
	}()
	if liveCustodyRef(prober, grandchild) {
		select {
		case lease := <-acquired:
			if liveCustodyRef(prober, grandchild) {
				_ = lease.Close()
				t.Fatal("capacity and named resource escaped while ordinary grandchild lived")
			}
			_ = lease.Close()
			assertDurableSpools()
			return
		case err := <-errs:
			t.Fatalf("contender refused instead of waiting: %v", err)
		case <-time.After(250 * time.Millisecond):
		}
	}
	select {
	case lease := <-acquired:
		if liveCustodyRef(prober, grandchild) {
			_ = lease.Close()
			t.Fatal("contender acquired while ordinary grandchild was still executing")
		}
		defer lease.Close()
		assertDurableSpools()
	case err := <-errs:
		t.Fatalf("contender did not acquire after cleanup: %v", err)
	case <-ctx.Done():
		active, activeErr := activeHostResourceSlots(directory)
		logBytes, _ := os.ReadFile(filepath.Join(directory, "launcher-helper.log"))
		custodyLog, _ := os.ReadFile(filepath.Join(directory, "launcher.log"))
		markers, _ := filepath.Glob(filepath.Join(directory, "lease-*"))
		markerData := ""
		for _, marker := range markers {
			data, _ := os.ReadFile(marker)
			markerData += filepath.Base(marker) + ":" + string(data) + " "
		}
		t.Fatalf("capacity and named resource stayed held after cleanup: active=%d err=%v worker=%s grandchild=%s watchdog=%s marker=%s helper=%s custodylog=%s", active, activeErr,
			identity.AliveRef(prober, worker), identity.AliveRef(prober, grandchild), identity.AliveRef(prober, watchdogProcess), markerData, logBytes, custodyLog)
	}
}

func liveCustodyRef(prober identity.Prober, ref identity.Ref) bool {
	exact, state, err := prober.Probe(ref.Pid)
	return err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) && !exact.Zombie
}

func waitCustodyRecordWatchdog(t *testing.T, root, suite string, bound time.Duration) identity.Ref {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		record, err := ReadRecord(root, suite)
		if err == nil && record.Watchdog.Pid > 0 {
			return record.Watchdog.Ref()
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("resource custodian record was not published")
	return identity.Ref{}
}

func waitCustodyFile(t *testing.T, path string, bound time.Duration) {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("custody barrier %s did not appear", filepath.Base(path))
}

func waitCustodyRef(t *testing.T, prober identity.Prober, path string, bound time.Duration) identity.Ref {
	t.Helper()
	waitCustodyFile(t, path, bound)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatalf("parse custody pid %q: %v", data, err)
	}
	exact, state, err := prober.Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe custody pid %d: %v (%s)", pid, err, state)
	}
	return exact.Ref()
}

func TestGLEHostResourceCustodyProcessHelper(t *testing.T) {
	if os.Getenv("METASYSTEM_HOST_CUSTODY_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args)-separator != 8 || os.Args[separator+1] != "launcher" {
		t.Fatal("invalid custody helper arguments")
	}
	directory, conf, watchdog, workerPID, grandPID, ready := os.Args[separator+2], os.Args[separator+3], os.Args[separator+4], os.Args[separator+5], os.Args[separator+6], os.Args[separator+7]
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	lease, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	closeFDs := ""
	for index := range lease.Files() {
		closeFDs += fmt.Sprintf("exec %d>&-; ", 3+index)
	}
	script := `echo $$ > "$1"; (` + closeFDs + `exec </dev/null >/dev/null 2>&1; : > "$3"; exec sleep 60) & echo $! > "$2"; trap '' TERM; while :; do sleep 1; done`
	code := LaunchSuite(LaunchOptions{Suite: "custody-killed-launcher", Root: directory, ConfPath: conf,
		ProgressPath: filepath.Join(directory, "progress.jsonl"), LogPath: filepath.Join(directory, "launcher.log"),
		Banner: "custody fixture", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
		TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
		WatchdogExecutable: watchdog, Command: []string{"sh", "-c", script, "sh", workerPID, grandPID, ready},
		HostResourceFiles: lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
	if code == 0 {
		t.Fatal("native worker unexpectedly completed successfully")
	}
}
