package proofrun

import (
	"context"
	"encoding/json"
	"errors"
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
	grandRelease := filepath.Join(directory, "grandchild.release")
	launcherLog, err := os.Create(filepath.Join(directory, "launcher-helper.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer launcherLog.Close()
	launcher := exec.Command(os.Args[0], "-test.run=^TestGLEHostResourceCustodyProcessHelper$", "--", "launcher", directory, conf, engine, workerPID, grandPID, ready, grandRelease)
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
		_ = os.WriteFile(grandRelease, []byte("release"), 0o600)
		for _, ref := range []identity.Ref{grandchild, worker, watchdogProcess, launcherExact.Ref()} {
			if ref.Pid > 0 {
				_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
			}
		}
		if !launcherWaited {
			_ = launcher.Wait()
		}
	})
	waitCustodyFile(t, ready, 0)
	worker = waitCustodyRef(t, prober, workerPID, 0)
	grandchild = waitCustodyRef(t, prober, grandPID, 0)
	watchdogProcess = waitCustodyRecordWatchdog(t, directory, "custody-killed-launcher")
	assertDurableSpools := func() {
		if liveCustodyRef(prober, watchdogProcess) {
			t.Fatal("resource custodian returned to a live state after its terminal witness")
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
	markers, err := filepath.Glob(filepath.Join(directory, "lease-*"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("expected one custody marker before cleanup: markers=%v err=%v", markers, err)
	}
	markerPath := markers[0]
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	observed := make(chan struct{})
	observationCtx, cancelObservation := context.WithCancel(t.Context())
	defer cancelObservation()
	waitCtx := WithHostResourceWaitObserver(observationCtx, func() {
		close(observed)
		<-observationCtx.Done()
	})
	go func() {
		lease, err := AcquireHostResources(waitCtx, directory, conf, "heavy", []string{"fixture-db"})
		if err != nil {
			errs <- err
			return
		}
		acquired <- lease
	}()
	select {
	case lease := <-acquired:
		_ = lease.Close()
		t.Fatalf("contender acquired before observing ordinary grandchild custody: grandchild=%s", identity.AliveRef(prober, grandchild))
	case err := <-errs:
		t.Fatalf("contender refused instead of waiting: %v", err)
	case <-observed:
	case <-t.Context().Done():
		t.Fatalf("contender did not observe held ordinary grandchild custody: %v", t.Context().Err())
	}
	if !liveCustodyRef(prober, grandchild) {
		t.Fatalf("exact grandchild %d died before the contender observed held custody", grandchild.Pid)
	}
	cancelObservation()
	select {
	case lease := <-acquired:
		_ = lease.Close()
		t.Fatal("observation-only contender acquired before custody cleanup")
	case err := <-errs:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("observation-only contender ended unexpectedly: %v", err)
		}
	case <-t.Context().Done():
		t.Fatalf("observation-only contender did not stop: %v", t.Context().Err())
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
	waitCustodyTerminal(t, prober, watchdogProcess)
	markerData, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read custody marker after custodian terminal: %v", err)
	}
	var marker hostLeaseRecord
	if err := json.Unmarshal(markerData, &marker); err != nil || !marker.Cleared {
		logBytes, _ := os.ReadFile(filepath.Join(directory, "launcher-helper.log"))
		custodyLog, _ := os.ReadFile(filepath.Join(directory, "launcher.log"))
		spoolData := ""
		if run, runErr := ReadLatestProgressRun(filepath.Join(directory, "progress.jsonl")); runErr == nil {
			for _, path := range run.Header.LogPaths {
				data, _ := os.ReadFile(path)
				spoolData += filepath.Base(path) + ":" + string(data) + " "
			}
		}
		t.Fatalf("resource custodian terminated without a clean marker: marker=%s err=%v helper=%s custodylog=%s spools=%s", markerData, err, logBytes, custodyLog, spoolData)
	}
	waitCustodyTerminal(t, prober, grandchild)
	lease, err := AcquireHostResources(t.Context(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		active, activeErr := activeHostResourceSlots(directory)
		logBytes, _ := os.ReadFile(filepath.Join(directory, "launcher-helper.log"))
		custodyLog, _ := os.ReadFile(filepath.Join(directory, "launcher.log"))
		spoolData := ""
		if run, runErr := ReadLatestProgressRun(filepath.Join(directory, "progress.jsonl")); runErr == nil {
			for _, path := range run.Header.LogPaths {
				data, _ := os.ReadFile(path)
				spoolData += filepath.Base(path) + ":" + string(data) + " "
			}
		}
		markers, _ := filepath.Glob(filepath.Join(directory, "lease-*"))
		markerData := ""
		for _, marker := range markers {
			data, _ := os.ReadFile(marker)
			markerData += filepath.Base(marker) + ":" + string(data) + " "
		}
		t.Fatalf("capacity and named resource stayed held after cleanup: acquire=%v context=%v active=%d err=%v worker=%s grandchild=%s watchdog=%s marker=%s helper=%s custodylog=%s spools=%s", err, t.Context().Err(), active, activeErr,
			identity.AliveRef(prober, worker), identity.AliveRef(prober, grandchild), identity.AliveRef(prober, watchdogProcess), markerData, logBytes, custodyLog, spoolData)
	}
	if err := MarkHostResourcesClean(lease.Files()); err != nil {
		_ = lease.Close()
		t.Fatal(err)
	}
	defer lease.Close()
	assertDurableSpools()
}

func liveCustodyRef(prober identity.Prober, ref identity.Ref) bool {
	exact, state, err := prober.Probe(ref.Pid)
	return err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) && !exact.Zombie
}

func waitCustodyRecordWatchdog(t *testing.T, root, suite string) identity.Ref {
	t.Helper()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	for {
		record, err := ReadRecord(root, suite)
		if err == nil && record.Watchdog.Pid > 0 {
			return record.Watchdog.Ref()
		}
		select {
		case <-poll.C:
		case <-t.Context().Done():
			t.Fatalf("resource custodian record was not published: %v", t.Context().Err())
		}
	}
}

func waitCustodyFile(t *testing.T, path string, bound time.Duration) {
	t.Helper()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	var deadline <-chan time.Time
	var timer *time.Timer
	if bound > 0 {
		timer = time.NewTimer(bound)
		deadline = timer.C
		defer timer.Stop()
	}
	exists := func() bool {
		_, err := os.Stat(path)
		return err == nil
	}
	for {
		if exists() {
			return
		}
		select {
		case <-poll.C:
		case <-t.Context().Done():
			if exists() {
				return
			}
			t.Fatalf("custody barrier %s did not appear: %v", filepath.Base(path), t.Context().Err())
		case <-deadline:
			if exists() {
				return
			}
			t.Fatalf("custody barrier %s did not appear", filepath.Base(path))
		}
	}
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

func waitCustodyTerminal(t *testing.T, prober identity.Prober, ref identity.Ref) {
	t.Helper()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	for {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, ref) || exact.Zombie)) {
			return
		}
		select {
		case <-poll.C:
		case <-t.Context().Done():
			t.Fatalf("exact custody process %d did not become terminal: state=%s err=%v", ref.Pid, state, err)
		}
	}
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
	if separator < 0 || len(os.Args)-separator != 9 || os.Args[separator+1] != "launcher" {
		t.Fatal("invalid custody helper arguments")
	}
	directory, conf, watchdog, workerPID, grandPID, ready, grandRelease := os.Args[separator+2], os.Args[separator+3], os.Args[separator+4], os.Args[separator+5], os.Args[separator+6], os.Args[separator+7], os.Args[separator+8]
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
	script := `echo $$ > "$1"; (` + closeFDs + `exec </dev/null >/dev/null 2>&1; : > "$3"; while [ ! -f "$4" ]; do sleep 0.02; done) & echo $! > "$2"; trap '' TERM; while :; do sleep 1; done`
	code := LaunchSuite(LaunchOptions{Suite: "custody-killed-launcher", Root: directory, ConfPath: conf,
		ProgressPath: filepath.Join(directory, "progress.jsonl"), LogPath: filepath.Join(directory, "launcher.log"),
		Banner: "custody fixture", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
		TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
		WatchdogExecutable: watchdog, Command: []string{"sh", "-c", script, "sh", workerPID, grandPID, ready, grandRelease},
		HostResourceFiles: lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
	if code == 0 {
		t.Fatal("native worker unexpectedly completed successfully")
	}
}
