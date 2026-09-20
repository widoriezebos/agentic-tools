package proofrun

import (
	"context"
	"errors"
	"fmt"
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

func isolatedHostResources(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	previousDirectory, previousReaders := hostAdmissionDirectoryForTest, loadSeams
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	t.Cleanup(func() {
		hostAdmissionDirectoryForTest = previousDirectory
		loadSeams = previousReaders
	})
	conf := filepath.Join(directory, "admission.conf")
	if err := os.WriteFile(conf, []byte(AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory, conf
}

func buildResourceCustodyEngine(t *testing.T) string {
	t.Helper()
	engine := filepath.Join(t.TempDir(), "metasystem")
	command := exec.Command("go", "build", "-p=2", "-o", engine, "./cmd/metasystem")
	command.Dir = filepath.Clean(filepath.Join("..", ".."))
	command.Env = append(os.Environ(), "GOMAXPROCS=2")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build resource custodian engine: %v: %s", err, output)
	}
	return engine
}

func TestHostResourceCapacityWaitsWithoutOwningSlot(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	first, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	go func() {
		next, err := AcquireHostResources(ctx, directory, conf, "heavy", []string{"fixture-db"})
		if err != nil {
			errs <- err
			return
		}
		acquired <- next
	}()
	select {
	case next := <-acquired:
		next.Close()
		t.Fatal("contender acquired the held native slot")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(150 * time.Millisecond):
	}
	active, err := activeHostResourceSlots(directory)
	if err != nil || active != 1 {
		t.Fatalf("waiting contender consumed capacity: active=%d err=%v", active, err)
	}
	if err := MarkHostResourcesClean(first.Files()); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case next := <-acquired:
		if next.Waited() < 100*time.Millisecond {
			t.Fatalf("queue time was not recorded: %v", next.Waited())
		}
		_ = MarkHostResourcesClean(next.Files())
		next.Close()
	case err := <-errs:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("contender did not acquire after cleanup")
	}
}

func TestHostResourceNestedLeaseRequiresHeldSubset(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	f := newOwnershipFixture(t)
	attempt, _ := f.reserve("nested-goal", "nested-plan", map[string]string{"native": strings.Repeat("a", 64)}, "", "", 0)
	runChild := func(lease *HostResourceLease, class, resource string, success, legacy bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		mode := "managed"
		if legacy {
			mode = "legacy"
		}
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostResourceNestedLeaseSubprocess$", "--", directory, conf, class, resource, mode)
		command.Env = append(os.Environ(), "GO_WANT_PROOF_ENTRYPOINT_HELPER=1",
			"METASYSTEM_PROOF_CONTROL_ROOT="+f.root, "METASYSTEM_PROOF_ATTEMPT="+attempt.AttemptID)
		if lease != nil {
			command.ExtraFiles = lease.Files()
			command.Env = append(command.Env, HostResourceFDEnvironment(lease.Files()))
		}
		output, err := command.CombinedOutput()
		if success && err != nil || !success && err == nil {
			t.Fatalf("nested %s/%s success=%v: err=%v output=%s", class, resource, success, err, output)
		}
	}
	runChild(nil, "heavy", "", false, false)
	runChild(nil, "heavy", "", true, true)
	runChild(nil, "heavy", "fixture-db", false, true)
	cheap, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	runChild(cheap, "heavy", "", false, false)
	if err := MarkHostResourcesClean(cheap.Files()); err != nil {
		t.Fatal(err)
	}
	if err := cheap.Close(); err != nil {
		t.Fatal(err)
	}
	heavy, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(heavy.Files()); _ = heavy.Close() }()
	runChild(heavy, "heavy", "fixture-db", true, false)
	runChild(heavy, "heavy", "new-db", false, false)
	active, err := activeHostResourceSlots(directory)
	if err != nil || active != 1 {
		t.Fatalf("nested borrowing double-counted or lost capacity: %d %v", active, err)
	}
}

func TestHostResourceNestedLeaseSubprocess(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args)-separator != 6 {
		return
	}
	directory, conf, class, resource, mode := os.Args[separator+1], os.Args[separator+2], os.Args[separator+3], os.Args[separator+4], os.Args[separator+5]
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	if mode == "legacy" {
		loadSeams.pids = func() ([]int64, error) { return []int64{int64(os.Getppid())}, nil }
		loadSeams.prober = hostResourceLegacyProber{}
	}
	if os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT") == "" || os.Getenv("METASYSTEM_PROOF_ATTEMPT") == "" {
		t.Fatal("nested fixture proof locator was not inherited")
	}
	var exclusive []string
	if resource != "" {
		exclusive = []string{resource}
	}
	lease, err := AcquireHostResources(context.Background(), os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), conf, class, exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if !lease.borrowed {
		t.Fatal("nested process acquired an independent capacity slot")
	}
}

type hostResourceLegacyProber struct{}

func (hostResourceLegacyProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err == nil && state == identity.Alive {
		exact.Argv = []string{"metasystem", "test", "run"}
		exact.ArgvKnown = true
	}
	return exact, state, err
}

func TestHostResourceChildRetainsSlotAfterOwnerDies(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	ready := filepath.Join(directory, "child-ready")
	release := filepath.Join(directory, "release-child")
	middlePID := filepath.Join(directory, "middle-pid")
	defer os.WriteFile(release, []byte("release"), 0o600)
	helperContext, stopHelper := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopHelper()
	helper := exec.CommandContext(helperContext, os.Args[0], "-test.run=^TestHostResourceSubprocess$", "--", "hold", directory, conf, ready, release, middlePID)
	if output, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("owner helper: %v: %s", err, output)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("native child did not start: %v", err)
	}
	pidData, err := os.ReadFile(middlePID)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	go func() {
		next, err := AcquireHostResources(ctx, directory, conf, "heavy", nil)
		if err != nil {
			errs <- err
			return
		}
		acquired <- next
	}()
	select {
	case next := <-acquired:
		next.Close()
		t.Fatal("killed worker released capacity while its grandchild lived")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case next := <-acquired:
		next.Close()
	case err := <-errs:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("surviving grandchild did not release capacity")
	}
}

// A native command may spawn an ordinary child that does not forward the
// proof descriptor. The launcher must prove that child cleaned up before its
// slot becomes available, even if the direct worker has already exited.
func TestHostResourceLaunchSuiteCleansUnforwardedGrandchildBeforeRelease(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	directory, conf := isolatedHostResources(t)
	lease, err := AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	childPID := filepath.Join(directory, "native-child.pid")
	ready := filepath.Join(directory, "native-child.ready")
	closeFDs := ""
	for index := range lease.Files() {
		closeFDs += fmt.Sprintf("exec %d>&-; ", 3+index)
	}
	script := "(" + closeFDs + `exec >/dev/null 2>&1; printf ready > "$2"; exec sleep 60) & echo $! > "$1"; ` +
		`while [ ! -f "$2" ]; do sleep 0.02; done`
	result := LaunchSuite(LaunchOptions{Suite: "native-custody", Root: directory, ConfPath: conf,
		ProgressPath: filepath.Join(directory, "progress.jsonl"), LogPath: filepath.Join(directory, "launcher.log"),
		Banner: "native custody fixture", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
		TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
		WatchdogExecutable: engine, Command: []string{"sh", "-c", script, "sh", childPID, ready},
		HostResourceFiles: lease.Files(), Output: io.Discard, ErrorOutput: io.Discard})
	if result == 0 {
		t.Error("launcher accepted a worker that left an ordinary native child alive")
	}
	pidData, err := os.ReadFile(childPID)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	child, state, err := prober.Probe(int64(pid))
	if err == nil && state == identity.Alive {
		t.Cleanup(func() { _ = identity.SignalExact(prober, child.Ref(), syscall.SIGKILL, syscall.Kill) })
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	contender, acquireErr := AcquireHostResources(ctx, directory, conf, "heavy", nil)
	if contender != nil {
		defer contender.Close()
	}
	childNow, childState, probeErr := prober.Probe(int64(pid))
	childLive := probeErr == nil && childState == identity.Alive && identity.SameIdentity(childNow, child.Ref()) && !childNow.Zombie
	if childLive && acquireErr == nil {
		t.Error("ordinary grandchild survived while its native capacity slot was released")
	}
	if childLive && acquireErr != nil && !errors.Is(acquireErr, context.DeadlineExceeded) {
		t.Fatalf("surviving grandchild had an unexpected admission result: %v", acquireErr)
	}
	if !childLive && acquireErr != nil {
		t.Fatalf("cleaned grandchild left capacity unavailable: %v", acquireErr)
	}
}

func TestHostResourceSubprocess(t *testing.T) {
	args := os.Args
	separator := -1
	for index, arg := range args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(args)-separator != 7 {
		return
	}
	mode := args[separator+1]
	directory, conf, ready, release, middlePID := args[separator+2], args[separator+3], args[separator+4], args[separator+5], args[separator+6]
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	switch mode {
	case "hold":
		lease, err := AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(os.Args[0], "-test.run=^TestHostResourceSubprocess$", "--", "middle", directory, conf, ready, release, middlePID)
		command.ExtraFiles = lease.Files()
		command.Env = append(os.Environ(), "GO_WANT_PROOF_ENTRYPOINT_HELPER=1", HostResourceFDEnvironment(lease.Files()))
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		for {
			_, readyErr := os.Stat(ready)
			_, pidErr := os.Stat(middlePID)
			if readyErr == nil && pidErr == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		os.Exit(0)
	case "middle":
		command := exec.Command(os.Args[0], "-test.run=^TestHostResourceSubprocess$", "--", "grandchild", directory, conf, ready, release, middlePID)
		command.Env = append(os.Environ(), "GO_WANT_PROOF_ENTRYPOINT_HELPER=1")
		closeInherited, err := InheritHostResourceLease(command)
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			closeInherited()
			t.Fatal(err)
		}
		closeInherited()
		if err := os.WriteFile(middlePID, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Second)
		}
	case "grandchild":
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := os.Stat(release); err == nil {
				var files []*os.File
				for _, entry := range strings.Split(os.Getenv(inheritedHostResourceFDs), ",") {
					fdText, name, ok := strings.Cut(entry, "@")
					fd, parseErr := strconv.Atoi(fdText)
					if !ok || parseErr != nil || fd < 3 {
						t.Fatal("grandchild lost its inherited lease manifest")
					}
					files = append(files, os.NewFile(uintptr(fd), filepath.Join(directory, name)))
				}
				if err := MarkHostResourcesClean(files); err != nil {
					t.Fatal(err)
				}
				os.Exit(0)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}
