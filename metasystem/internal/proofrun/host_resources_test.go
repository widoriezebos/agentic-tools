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
	if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
		t.Fatalf("holder did not own the only native slot: active=%d err=%v", active, err)
	}
	if named, available, err := tryHostFile(hostResourcePath(directory, "fixture-db")); err != nil || available {
		if named != nil {
			_ = named.Close()
		}
		t.Fatalf("holder did not own the named resource: available=%t err=%v", available, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	retried := make(chan struct{})
	waitCtx := WithHostResourceWaitObserver(ctx, func() { close(retried) })
	go func() {
		next, err := AcquireHostResources(waitCtx, directory, conf, "heavy", []string{"fixture-db"})
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
	case <-retried:
	case <-ctx.Done():
		t.Fatal("contender did not reach its first failed capacity scan")
	}
	select {
	case next := <-acquired:
		_ = next.Close()
		t.Fatal("contender acquired the held native slot after its failed scan")
	case err := <-errs:
		t.Fatal(err)
	default:
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
		if next.Waited() <= 0 {
			t.Fatalf("queue time was not recorded: %v", next.Waited())
		}
		_ = MarkHostResourcesClean(next.Files())
		next.Close()
	case err := <-errs:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("contender did not acquire after cleanup")
	}
	t.Run("caller_fence_closes_while_queued", func(t *testing.T) {
		checkHostCapacityWaitObservesCallerFence(t, ctx)
	})
	t.Run("fixture_census_authority", func(t *testing.T) {
		previousDirectory, previousReaders, previousOptions := hostAdmissionDirectoryForTest, loadSeams, commandLoadOptions
		hostAdmissionDirectoryForTest = ""
		t.Cleanup(func() {
			hostAdmissionDirectoryForTest, loadSeams, commandLoadOptions = previousDirectory, previousReaders, previousOptions
		})
		fakeRoot, realRoot := t.TempDir(), t.TempDir()
		for _, fixture := range []struct{ root, mode string }{{fakeRoot, "fake"}, {realRoot, "real"}} {
			if err := os.WriteFile(filepath.Join(fixture.root, "metasystem.conf"), []byte("metasystem.runtimes="+fixture.mode+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		directory := filepath.Join(t.TempDir(), "host-admission")
		conf := filepath.Join(fakeRoot, "admission.conf")
		if err := os.WriteFile(conf, []byte(AdmissionCapKey+"=2\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", directory)
		t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", fakeRoot)
		realCount, realCalls := 2, 0
		loadSeams.launchers = func(int64) (int, bool) {
			realCalls++
			return realCount, true
		}
		commandLoadOptions = []loadSampleOption{withTestHostLoad("0")}
		t.Run("real_target_uses_real_census", func(t *testing.T) {
			before := realCalls
			count, known, err := resourceLegacyLauncherCount(realRoot)
			if err != nil || !known || count != realCount || realCalls != before+1 {
				t.Fatalf("unrelated fake fixture masked real target census: count=%d known=%t calls=%d err=%v", count, known, realCalls-before, err)
			}
		})
		t.Run("no_namespace_uses_real_census", func(t *testing.T) {
			t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", "")
			before := realCalls
			count, known, err := resourceLegacyLauncherCount(fakeRoot)
			if err != nil || !known || count != realCount || realCalls != before+1 {
				t.Fatalf("unselected fixture masked real census: count=%d known=%t calls=%d err=%v", count, known, realCalls-before, err)
			}
		})
		t.Run("invalid_fixture_count_refuses", func(t *testing.T) {
			commandLoadOptions = []loadSampleOption{withTestHostLoad("invalid")}
			lease, err := AcquireHostResources(t.Context(), fakeRoot, conf, "heavy", nil)
			if lease != nil || err == nil || !strings.Contains(err.Error(), "fixture host launcher count is invalid") {
				if lease != nil {
					_ = lease.Close()
				}
				t.Fatalf("invalid fixture count admitted or fell back to real census: lease=%v err=%v", lease, err)
			}
			if active, err := activeHostResourceSlots(directory); err != nil || active != 0 {
				t.Fatalf("invalid fixture count retained a slot: active=%d err=%v", active, err)
			}
		})
		t.Run("temporary_symlink_escape_refuses", func(t *testing.T) {
			parent := t.TempDir()
			admitted, outside := filepath.Join(parent, "admitted-temp"), filepath.Join(parent, "outside")
			for _, path := range []string{admitted, outside} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TMPDIR", admitted)
			link := filepath.Join(admitted, "escape")
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(link, ".host-resource-fixture-escape")
			if _, err := os.Lstat(selected); !os.IsNotExist(err) {
				t.Fatalf("escape target exists before test: %v", err)
			}
			t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", selected)
			if _, err := hostAdmissionDirectory(); err == nil || !strings.Contains(err.Error(), "temporary path") {
				t.Fatalf("symlink ancestor escaped temporary admission boundary: %v", err)
			}
			if _, err := os.Lstat(selected); !os.IsNotExist(err) {
				t.Fatalf("rejected symlink escape created a directory outside temp: %v", err)
			}
		})
		t.Run("max_int_fixture_cannot_overflow_cap", func(t *testing.T) {
			realCount = 0
			commandLoadOptions = []loadSampleOption{withTestHostLoad("0")}
			holder, err := AcquireHostResources(t.Context(), fakeRoot, conf, "heavy", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = MarkHostResourcesClean(holder.Files()); _ = holder.Close() }()
			if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
				t.Fatalf("cap-two fixture did not hold one real slot: active=%d err=%v", active, err)
			}
			commandLoadOptions = []loadSampleOption{withTestHostLoad(strconv.Itoa(int(^uint(0) >> 1)))}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := make(chan struct{})
			waitCtx := WithHostResourceWaitObserver(ctx, func() { close(observed) })
			type outcome struct {
				lease *HostResourceLease
				err   error
			}
			finished := make(chan outcome, 1)
			go func() {
				lease, err := AcquireHostResources(waitCtx, fakeRoot, conf, "heavy", nil)
				finished <- outcome{lease: lease, err: err}
			}()
			select {
			case <-observed:
			case result := <-finished:
				if result.lease != nil {
					_ = result.lease.Close()
				}
				t.Fatalf("MaxInt launcher count did not wait: %+v", result)
			case <-ctx.Done():
				t.Fatal("MaxInt launcher count did not reach a failed scan")
			}
			if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
				t.Fatalf("overflow contender displaced held slot: active=%d err=%v", active, err)
			}
			cancel()
			result := <-finished
			if result.lease != nil || !errors.Is(result.err, context.Canceled) {
				if result.lease != nil {
					_ = result.lease.Close()
				}
				t.Fatalf("canceled overflow contender admitted: %+v", result)
			}
		})
	})
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
	helper := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHostResourceSubprocess$", "--", "hold", directory, conf, ready, release, middlePID)
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
	if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
		t.Fatalf("surviving grandchild did not retain capacity: active=%d err=%v", active, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	acquired := make(chan *HostResourceLease, 1)
	errs := make(chan error, 1)
	retried := make(chan struct{})
	waitCtx := WithHostResourceWaitObserver(ctx, func() { close(retried) })
	go func() {
		next, err := AcquireHostResources(waitCtx, directory, conf, "heavy", nil)
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
	case <-retried:
	case <-ctx.Done():
		t.Fatal("contender did not reach its first failed capacity scan")
	}
	select {
	case next := <-acquired:
		_ = next.Close()
		t.Fatal("contender acquired capacity while the grandchild retained custody")
	case err := <-errs:
		t.Fatal(err)
	default:
	}
	if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
		t.Fatalf("contender displaced the surviving grandchild: active=%d err=%v", active, err)
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
