package proofrun

import (
	"bytes"
	"context"
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

// privateHostResources is an admission namespace and cap file of the test's
// own, for acquireHostResourcesIn; unlike isolatedHostResources it replaces no
// package default, so a parallel test may use it.
func privateHostResources(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(directory, "admission.conf")
	if err := os.WriteFile(conf, []byte(AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory, conf
}

func isolatedHostResources(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	previousDirectory := hostAdmissionDirectoryForTest
	hostAdmissionDirectoryForTest = directory
	t.Cleanup(func() {
		hostAdmissionDirectoryForTest = previousDirectory
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
	t.Run("fixture_namespace_census_skips_other_fixture_launchers", checkFixtureNamespaceCensusSkipsOtherFixtureLaunchers)
	t.Run("fixture_census_authority", func(t *testing.T) {
		previousDirectory, previousOptions := hostAdmissionDirectoryForTest, commandLoadOptions
		hostAdmissionDirectoryForTest = ""
		t.Cleanup(func() {
			hostAdmissionDirectoryForTest, commandLoadOptions = previousDirectory, previousOptions
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
		readers := deterministicTestLoadReaders()
		readers.launchers = func(int64) (int, bool) {
			realCalls++
			return realCount, true
		}
		commandLoadOptions = []loadSampleOption{withTestHostLoad("0")}
		t.Run("real_target_uses_real_census", func(t *testing.T) {
			before := realCalls
			count, known, err := resourceLegacyLauncherCount(readers, realRoot)
			if err != nil || !known || count != realCount || realCalls != before+1 {
				t.Fatalf("unrelated fake fixture masked real target census: count=%d known=%t calls=%d err=%v", count, known, realCalls-before, err)
			}
		})
		t.Run("no_namespace_uses_real_census", func(t *testing.T) {
			t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", "")
			before := realCalls
			count, known, err := resourceLegacyLauncherCount(readers, fakeRoot)
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
			// The boundary is the host's temporary roots, whatever TMPDIR
			// says (disk-lifetimes Part B U1b-2): a link inside TMPDIR to the
			// package's own source directory, outside every host temporary
			// root, is refused before anything is made there.
			parent := t.TempDir()
			admitted := filepath.Join(parent, "admitted-temp")
			if err := os.Mkdir(admitted, 0o700); err != nil {
				t.Fatal(err)
			}
			outside, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
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
			if _, err := hostAdmissionDirectory(); err == nil || !strings.Contains(err.Error(), "must be temporary") {
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
	t.Run("nested_wait_observers_compose_once_and_release_or_cancel", checkHostResourceWaitObserversComposeOnceAndReleaseOrCancel)
}

func checkHostResourceWaitObserversComposeOnceAndReleaseOrCancel(t *testing.T) {
	t.Run("release holder", func(t *testing.T) {
		directory, conf := isolatedHostResources(t)
		holder, err := AcquireHostResources(t.Context(), directory, conf, "heavy", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Close() }()

		outerCalls, innerCalls := 0, 0
		var releaseErr error
		ctx := WithHostResourceWaitObserver(t.Context(), func() {
			outerCalls++
			releaseErr = errors.Join(MarkHostResourcesClean(holder.Files()), holder.Close())
		})
		ctx = WithHostResourceWaitObserver(ctx, func() { innerCalls++ })
		lease, err := AcquireHostResources(ctx, directory, conf, "heavy", nil)
		if err != nil || releaseErr != nil {
			t.Fatalf("acquire after observer release: acquire=%v release=%v", err, releaseErr)
		}
		if outerCalls != 1 || innerCalls != 1 {
			t.Fatalf("nested observer calls = outer %d inner %d, want one each", outerCalls, innerCalls)
		}
		if err := MarkHostResourcesClean(lease.Files()); err != nil {
			t.Fatal(err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cancel wait", func(t *testing.T) {
		directory, conf := isolatedHostResources(t)
		holder, err := AcquireHostResources(t.Context(), directory, conf, "heavy", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = MarkHostResourcesClean(holder.Files()); _ = holder.Close() }()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		outerCalls, innerCalls := 0, 0
		ctx = WithHostResourceWaitObserver(ctx, func() { outerCalls++ })
		ctx = WithHostResourceWaitObserver(ctx, func() {
			innerCalls++
			cancel()
		})
		lease, err := AcquireHostResources(ctx, directory, conf, "heavy", nil)
		if lease != nil || !errors.Is(err, context.Canceled) {
			if lease != nil {
				_ = lease.Close()
			}
			t.Fatalf("cancelled nested observer acquire = lease %v error %v", lease, err)
		}
		if outerCalls != 1 || innerCalls != 1 {
			t.Fatalf("cancelled nested observer calls = outer %d inner %d, want one each", outerCalls, innerCalls)
		}
	})
}

func TestHostResourceNestedLeaseRequiresHeldSubset(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	f := newOwnershipFixture(t)
	attempt, _ := f.reserve("nested-goal", "nested-plan", map[string]string{"native": strings.Repeat("a", 64)}, "", "", 0)
	runChild := func(lease *HostResourceLease, class, resource string, legacy bool, expectedError string) {
		t.Helper()
		mode := "managed"
		if legacy {
			mode = "legacy"
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHostResourceNestedLeaseSubprocess$", "--", directory, conf, class, resource, mode)
		command.Env = append(os.Environ(), "GO_WANT_PROOF_ENTRYPOINT_HELPER=1",
			"METASYSTEM_PROOF_CONTROL_ROOT="+f.root, "METASYSTEM_PROOF_ATTEMPT="+attempt.AttemptID)
		if lease != nil {
			command.ExtraFiles = lease.Files()
			command.Env = append(command.Env, HostResourceFDEnvironment(lease.Files()))
		}
		output, err := command.CombinedOutput()
		if t.Context().Err() != nil {
			t.Fatalf("nested %s/%s reached its unsuccessful context guard: %v output=%s", class, resource, t.Context().Err(), output)
		}
		if expectedError == "" {
			if err != nil {
				t.Fatalf("nested %s/%s did not borrow its held subset: err=%v output=%s", class, resource, err, output)
			}
			return
		}
		if err == nil || !strings.Contains(string(output), expectedError) {
			t.Fatalf("nested %s/%s did not return %q: err=%v output=%s", class, resource, expectedError, err, output)
		}
	}
	runChild(nil, "heavy", "", false, "the parent test run holds no resource lease and no counted launcher")
	runChild(nil, "heavy", "", true, "")
	runChild(nil, "heavy", "fixture-db", true, "the older parent test run holds no named resource lease")
	cheap, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	runChild(cheap, "heavy", "", false, "the parent test run's lease does not match the capacity or resources passed down")
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
	runChild(heavy, "heavy", "fixture-db", false, "")
	runChild(heavy, "heavy", "new-db", false, `the parent test run's lease does not cover resource "new-db"`)
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
	readers := deterministicTestLoadReaders()
	if mode == "legacy" {
		readers.processes = identity.ListedProcessTable{int64(os.Getppid())}
		readers.prober = hostResourceLegacyProber{}
	}
	if os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT") == "" || os.Getenv("METASYSTEM_PROOF_ATTEMPT") == "" {
		t.Fatal("nested fixture proof locator was not inherited")
	}
	var exclusive []string
	if resource != "" {
		exclusive = []string{resource}
	}
	admission, err := hostAdmissionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	lease, err := acquireHostResourcesWith(context.Background(), readers, admission, os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), conf, class, exclusive, nil)
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

// The owner (hold) exits, the test kills the worker (middle) it launched, and
// the worker's native grandchild keeps the slot until it releases. The three
// helpers run without a testenv fixture custodian of their own (see
// hostResourceChainHelperInvocation): each one's custodian watches its
// parent and kills it one poll after that parent dies, so the owner's
// deliberate exit killed the worker before the test could ("no such
// process" under load) and the worker's death killed the grandchild whose
// survival is the claim. This test owns them: the release file ends the
// worker and the grandchild, and the parent's custodian reaps a leftover.
func TestHostResourceChildRetainsSlotAfterOwnerDies(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	if err := os.WriteFile(filepath.Join(directory, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(directory, "child-ready")
	release := filepath.Join(directory, "release-child")
	middlePID := filepath.Join(directory, "middle-pid")
	defer os.WriteFile(release, []byte("release"), 0o600)
	helper := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHostResourceSubprocess$", "--", "hold", directory, conf, ready, release, middlePID)
	helper.Env = append(os.Environ(), hostResourceChainHelperEnv+"=1")
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
	// Nothing else ends the worker, so it is alive at its exact identity
	// until the release file; the signal goes to that identity only.
	prober := identity.KernelProber{}
	worker, state, err := prober.Probe(int64(pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("worker %d is not alive after its owner exited: state=%s err=%v", pid, state, err)
	}
	if err := identity.SignalExact(prober, worker.Ref(), syscall.SIGKILL, syscall.Kill); err != nil {
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
	if err := os.WriteFile(filepath.Join(directory, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	processFile := filepath.Join(directory, "processes.json")
	if err := os.WriteFile(processFile, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processFile)
	lease, err := AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	leaseClosed := false
	t.Cleanup(func() {
		if !leaseClosed {
			_ = lease.Close()
		}
	})
	childPID := filepath.Join(directory, "native-child.pid")
	ready := filepath.Join(directory, "native-child.ready")
	release := filepath.Join(directory, "native-child.release")
	childRelease := filepath.Join(directory, "native-child-lifetime.release")
	closeFDs := ""
	for index := range lease.Files() {
		closeFDs += fmt.Sprintf("exec %d>&-; ", 3+index)
	}
	script := "(" + closeFDs + `exec >/dev/null 2>&1; printf ready > "$2"; while [ ! -f "$4" ]; do sleep 0.02; done) & echo $! > "$1"; ` +
		`while [ ! -f "$2" ]; do sleep 0.02; done; while [ ! -f "$3" ]; do sleep 0.02; done`
	var stdout, stderr bytes.Buffer
	runDone := make(chan struct{})
	var result int
	go func() {
		result = LaunchSuite(LaunchOptions{Suite: "native-custody", Root: directory, ConfPath: conf,
			ProgressPath: filepath.Join(directory, "progress.jsonl"), LogPath: filepath.Join(directory, "launcher.log"),
			Banner: "native custody fixture", Silence: time.Second, SectionCap: time.Second,
			EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
			TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
			WatchdogExecutable: engine, Command: []string{"sh", "-c", script, "sh", childPID, ready, release, childRelease},
			HostResourceFiles: lease.Files(), Output: &stdout, ErrorOutput: &stderr})
		close(runDone)
	}()
	t.Cleanup(func() {
		_ = os.WriteFile(release, []byte("release"), 0o600)
		_ = os.WriteFile(childRelease, []byte("release"), 0o600)
		<-runDone
	})
	waitCustodyFile(t, ready, 0)
	prober := identity.KernelProber{}
	child := waitCustodyRef(t, prober, childPID, 0)
	t.Cleanup(func() { _ = identity.SignalExact(prober, child, syscall.SIGKILL, syscall.Kill) })
	type acquireResult struct {
		lease *HostResourceLease
		err   error
	}
	acquired := make(chan acquireResult, 1)
	observed := make(chan struct{})
	waitCtx, cancelWait := context.WithCancel(t.Context())
	defer cancelWait()
	waitCtx = WithHostResourceWaitObserver(waitCtx, func() { close(observed) })
	go func() {
		next, acquireErr := AcquireHostResources(waitCtx, directory, conf, "heavy", nil)
		acquired <- acquireResult{lease: next, err: acquireErr}
	}()
	select {
	case result := <-acquired:
		if result.lease != nil {
			_ = result.lease.Close()
		}
		t.Fatalf("contender did not wait behind the live ordinary child: err=%v", result.err)
	case <-observed:
	case <-t.Context().Done():
		t.Fatalf("contender did not observe the live ordinary child's custody: %v", t.Context().Err())
	}
	if !liveCustodyRef(prober, child) {
		t.Fatalf("exact ordinary child %d died before the contender observed custody", child.Pid)
	}
	if active, activeErr := activeHostResourceSlots(directory); activeErr != nil || active != 1 {
		t.Fatalf("live ordinary child did not keep its slot occupied: active=%d err=%v", active, activeErr)
	}
	cancelWait()
	next := <-acquired
	if next.lease != nil {
		_ = next.lease.Close()
		t.Fatal("contender acquired while the ordinary child retained custody")
	}
	if !errors.Is(next.err, context.Canceled) {
		t.Fatalf("controlled contender cancellation returned %v", next.err)
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runDone:
	case <-t.Context().Done():
		t.Fatalf("native custody launch did not finish: %v", t.Context().Err())
	}
	diagnostics := stdout.String() + stderr.String()
	if result == 0 || !strings.Contains(diagnostics, "native descendants survived direct worker completion") {
		t.Fatalf("launcher did not retain the intended ordinary-child failure: exit=%d output=%s", result, diagnostics)
	}
	waitCustodyTerminal(t, prober, child)
	var marker hostLeaseRecord
	markerFound := false
	for _, file := range lease.Files() {
		if !strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			continue
		}
		markerFound = true
		var readErr error
		marker, _, readErr = readHostLeaseRecord(file)
		if readErr != nil {
			t.Fatalf("read native custody marker: %v output=%s", readErr, diagnostics)
		}
	}
	if !markerFound || !marker.Cleared {
		t.Fatalf("native custody returned without a clean marker: found=%t marker=%+v output=%s", markerFound, marker, diagnostics)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	leaseClosed = true
	nextLease, err := AcquireHostResources(t.Context(), directory, conf, "heavy", nil)
	if err != nil || nextLease == nil {
		t.Fatalf("capacity acquisition failed after exact cleanup: %v", err)
	}
	if err := MarkHostResourcesClean(nextLease.Files()); err != nil {
		_ = nextLease.Close()
		t.Fatal(err)
	}
	if err := nextLease.Close(); err != nil {
		t.Fatal(err)
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
		publishCustodyFile(t, middlePID, strconv.Itoa(os.Getpid()))
		// The test kills this worker; the release file ends it when the
		// test failed before that.
		for {
			if _, err := os.Stat(release); err == nil {
				os.Exit(0)
			}
			time.Sleep(20 * time.Millisecond)
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

// Two fixture admission namespaces on one host each mark their own waiting
// launchers managed, in a directory the other cannot see. The land bed runs
// its carried scenarios that way, one namespace each: when five or more
// reached the heavy phase together, every one counted the others' waiting
// test runs as unmanaged launchers against its cap of four, and all of them
// waited to their ceiling. Inside a selected namespace the census therefore
// skips launchers of other fake-runtime fixture roots and still counts every
// launcher it cannot place as a fixture.
func checkFixtureNamespaceCensusSkipsOtherFixtureLaunchers(t *testing.T) {
	previousDirectory, previousOptions := hostAdmissionDirectoryForTest, commandLoadOptions
	hostAdmissionDirectoryForTest = ""
	commandLoadOptions = nil
	t.Cleanup(func() {
		hostAdmissionDirectoryForTest, commandLoadOptions = previousDirectory, previousOptions
	})
	ownRoot, otherFixtureRoot, realRoot := t.TempDir(), t.TempDir(), t.TempDir()
	for _, fixture := range []struct{ root, mode string }{{ownRoot, "fake"}, {otherFixtureRoot, "fake"}, {realRoot, "real"}} {
		if err := os.WriteFile(filepath.Join(fixture.root, "metasystem.conf"), []byte("metasystem.runtimes="+fixture.mode+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	launcher := func(pid int64, argv ...string) identity.Exact {
		return identity.Exact{Pid: pid, StartedAt: started, Argv: append([]string{"/fixture/bin/metasystem"}, argv...), ArgvKnown: true}
	}
	census := &censusProber{processes: map[int64]identity.Exact{
		10: launcher(10, "internal", "test", "run", "--root", otherFixtureRoot, "--goal", "fx"),
		11: launcher(11, "internal", "test", "run", "--root="+otherFixtureRoot),
		12: launcher(12, "internal", "proof-run", "launch", "--suite", "validate-metasystem", "--root", realRoot),
		13: launcher(13, "test", "run", "--root", "relative/fixture"),
		14: launcher(14, "internal", "test", "run"),
	}, calls: map[int64]int{}}
	readers := deterministicTestLoadReaders()
	readers.launchers, readers.fixtureNamespaceLaunchers = nil, nil
	readers.prober = census
	readers.processes = identity.FixedProcessTable{
		{Pid: 10, Group: 10, Parent: 1}, {Pid: 11, Group: 11, Parent: 1}, {Pid: 12, Group: 12, Parent: 1},
		{Pid: 13, Group: 13, Parent: 1}, {Pid: 14, Group: 14, Parent: 1},
	}

	directory := filepath.Join(t.TempDir(), "host-admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", directory)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", ownRoot)
	if count, known, err := resourceLegacyLauncherCount(readers, ownRoot); err != nil || !known || count != 3 {
		t.Fatalf("selected fixture namespace census = %d known=%t err=%v, want 3 (the real launcher and the two it cannot place)", count, known, err)
	}
	if count, known, err := resourceLegacyLauncherCount(readers, realRoot); err != nil || !known || count != 5 {
		t.Fatalf("a real root in a fixture environment counted %d known=%t err=%v, want all 5", count, known, err)
	}
	if sample := sampleLoad(ownRoot, "proof-none", 0, started, withLoadReaders(readers)); !sample.OverlapKnown || sample.OverlappingHost != 3 {
		t.Fatalf("attempt admission inside the namespace saw %+v, want 3 host launchers", sample)
	}

	// Cap four admits the heavy phase beside three countable launchers; the
	// whole-host census (five) would have waited. The wait check ends a
	// regression at the first failed scan instead of on a clock.
	conf := filepath.Join(ownRoot, "admission.conf")
	if err := os.WriteFile(conf, []byte(AdmissionCapKey+"=4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waited := false
	ctx := WithHostResourceWaitObserver(t.Context(), func() { waited = true })
	admission, err := hostAdmissionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	lease, err := acquireHostResourcesWith(ctx, readers, admission, ownRoot, conf, "heavy", nil, func() error {
		if waited {
			return errors.New("the fixture namespace waited on another namespace's launchers")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkHostResourcesClean(lease.Files()); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", "")
	hostAdmissionDirectoryForTest = t.TempDir()
	if count, known, err := resourceLegacyLauncherCount(readers, ownRoot); err != nil || !known || count != 5 {
		t.Fatalf("a fixture root outside a selected namespace counted %d known=%t err=%v, want all 5", count, known, err)
	}
}

func TestFixtureLauncherArgvNeedsAnAbsoluteFakeRoot(t *testing.T) {
	t.Parallel()
	fakeRoot, realRoot := t.TempDir(), t.TempDir()
	for _, fixture := range []struct{ root, mode string }{{fakeRoot, "fake"}, {realRoot, "real"}} {
		if err := os.WriteFile(filepath.Join(fixture.root, "metasystem.conf"), []byte("metasystem.runtimes="+fixture.mode+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		argv []string
		want bool
	}{
		{[]string{"metasystem", "internal", "test", "run", "--root", fakeRoot}, true},
		{[]string{"metasystem", "internal", "test", "run", "--root=" + fakeRoot}, true},
		{[]string{"metasystem", "test", "run", "-root", fakeRoot}, true},
		{[]string{"metasystem", "internal", "test", "run", "--root", realRoot}, false},
		{[]string{"metasystem", "internal", "test", "run", "--root", "."}, false},
		{[]string{"metasystem", "internal", "test", "run", "--root"}, false},
		{[]string{"metasystem", "internal", "test", "run"}, false},
		{[]string{"metasystem", "internal", "test", "run", "--root", filepath.Join(fakeRoot, "absent")}, false},
	} {
		if got := fixtureLauncherArgv(tc.argv); got != tc.want {
			t.Fatalf("fixtureLauncherArgv(%q) = %t, want %t", tc.argv, got, tc.want)
		}
	}
}

// A relative --root names a directory only relative to the launcher's own
// working directory, which the census cannot see. Resolved against the
// census's working directory it would read the wrong checkout, so it is
// never discounted: from inside a fake-runtime checkout, "--root ." still
// counts.
func TestFixtureLauncherArgvRefusesARelativeRootInsideAFakeCheckout(t *testing.T) {
	fakeRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fakeRoot)
	for _, argv := range [][]string{
		{"metasystem", "internal", "test", "run", "--root", "."},
		{"metasystem", "internal", "test", "run", "--root=."},
	} {
		if fixtureLauncherArgv(argv) {
			t.Fatalf("fixtureLauncherArgv(%q) under a fake-runtime working directory = true, want false", argv)
		}
	}
}
