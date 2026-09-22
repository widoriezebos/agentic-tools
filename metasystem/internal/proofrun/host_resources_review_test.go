package proofrun

import (
	"context"
	"encoding/json"
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

func hostResourceReviewChild(t *testing.T, fixtureRoot, attemptID, directory, conf, class, resource, mode string, files []*os.File) error {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestHostResourceReviewSubprocess$", "--", directory, conf, class, resource, mode)
	command.ExtraFiles = files
	command.Env = append(os.Environ(), "GO_WANT_PROOF_ENTRYPOINT_HELPER=1",
		"METASYSTEM_PROOF_CONTROL_ROOT="+fixtureRoot, "METASYSTEM_PROOF_ATTEMPT="+attemptID,
		HostResourceFDEnvironment(files))
	output, err := command.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "FAIL") {
		t.Logf("resource child output: %s", output)
	}
	return err
}

func TestHostResourceForgedOpenPeerSlotDoesNotBorrowHeavy(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	fixture := newOwnershipFixture(t)
	parent, _ := fixture.reserve("nested-goal", "nested-plan", map[string]string{"native": strings.Repeat("a", 64)}, "", "", 0)
	cheap, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(cheap.Files()); _ = cheap.Close() }()
	peer, err := AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(peer.Files()); _ = peer.Close() }()
	unlocked, err := os.OpenFile(filepath.Join(directory, "slot-00"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unlocked.Close()
	files := append(append([]*os.File{}, cheap.Files()...), unlocked)
	if err := hostResourceReviewChild(t, fixture.root, parent.AttemptID, directory, conf, "heavy", "", "borrow", files); err == nil {
		t.Fatal("borrower accepted an unlocked descriptor to a peer-held heavy slot")
	}
}

func TestHostResourceForgedOpenPeerExclusiveDoesNotBorrowName(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	fixture := newOwnershipFixture(t)
	parent, _ := fixture.reserve("nested-goal", "nested-plan", map[string]string{"native": strings.Repeat("c", 64)}, "", "", 0)
	cheap, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(cheap.Files()); _ = cheap.Close() }()
	peer, err := AcquireHostResources(context.Background(), directory, conf, "cheap", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(peer.Files()); _ = peer.Close() }()
	unlocked, err := os.OpenFile(hostResourcePath(directory, "fixture-db"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unlocked.Close()
	files := append(append([]*os.File{}, cheap.Files()...), unlocked)
	if err := hostResourceReviewChild(t, fixture.root, parent.AttemptID, directory, conf, "cheap", "fixture-db", "borrow", files); err == nil {
		t.Fatal("borrower accepted an unlocked descriptor to a peer-held exclusive resource")
	}
}

// A nested proof completes while borrowing the outer lease. If the outer
// launcher and custodian are then lost, a descendant that deliberately did
// not inherit any lease descriptor still belongs to the uncleared phase.
// Completion of the nested proof must never clean the outer marker.
func TestHostResourceNestedCompletionCannotClearLostOuterCustody(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	directory, conf := isolatedHostResources(t)
	if err := os.WriteFile(filepath.Join(directory, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	processFile := filepath.Join(directory, "processes.json")
	if err := os.WriteFile(processFile, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ackPath := filepath.Join(directory, "nested-complete.pipe")
	if err := syscall.Mkfifo(ackPath, 0o600); err != nil {
		t.Fatal(err)
	}
	ackPipe, err := os.OpenFile(ackPath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer ackPipe.Close()
	releasePath := filepath.Join(directory, "grandchild-release.pipe")
	if err := syscall.Mkfifo(releasePath, 0o600); err != nil {
		t.Fatal(err)
	}
	releasePipe, err := os.OpenFile(releasePath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = releasePipe.Close() })
	acknowledged := make(chan error, 1)
	go func() {
		var ack [1]byte
		_, readErr := io.ReadFull(ackPipe, ack[:])
		if readErr == nil && ack[0] != 1 {
			readErr = fmt.Errorf("unexpected nested completion acknowledgement %d", ack[0])
		}
		acknowledged <- readErr
	}()
	log, err := os.Create(filepath.Join(directory, "nested-custody-helper.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	launcher := exec.Command(os.Args[0], "-test.run=^TestHostResourceNestedCustodySubprocess$", "--", "launcher", directory, conf, engine, processFile, ackPath)
	launcher.Env = append(os.Environ(), "METASYSTEM_NESTED_CUSTODY_HELPER=1", "METASYSTEM_CENSUS_PROCESS_FILE="+processFile,
		"METASYSTEM_NESTED_CUSTODY_RELEASE="+releasePath)
	launcher.Stdout, launcher.Stderr = log, log
	if err := launcher.Start(); err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	var launcherExact identity.Exact
	var worker, grandchild, custodian identity.Ref
	launcherWaited := false
	t.Cleanup(func() {
		if t.Failed() {
			for _, name := range []string{"nested-custody-helper.log", "outer.log", "nested.log", "control-root"} {
				data, _ := os.ReadFile(filepath.Join(directory, name))
				t.Logf("%s: %s", name, data)
			}
		}
		for _, ref := range []identity.Ref{grandchild, worker, custodian, launcherExact.Ref()} {
			if ref.Pid > 0 {
				_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
			}
		}
		if !launcherWaited {
			_ = launcher.Wait()
		}
	})
	launcherExact, state, err := prober.Probe(int64(launcher.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe launcher: %v (%s)", err, state)
	}
	select {
	case err := <-acknowledged:
		if err != nil {
			t.Fatalf("read nested completion acknowledgement: %v", err)
		}
	case <-t.Context().Done():
		t.Fatalf("nested completion was not acknowledged: %v", t.Context().Err())
	}
	nestedDone, err := os.ReadFile(filepath.Join(directory, "nested.done"))
	if err != nil || string(nestedDone) != "done" {
		t.Fatalf("nested completion acknowledgement lacks durable evidence: data=%q err=%v", nestedDone, err)
	}
	waitCustodyFile(t, filepath.Join(directory, "grandchild.ready"), 0)
	worker = waitCustodyRef(t, prober, filepath.Join(directory, "worker.pid"), 0)
	grandchild = waitCustodyRef(t, prober, filepath.Join(directory, "grandchild.pid"), 0)
	controlRootBytes, err := os.ReadFile(filepath.Join(directory, "control-root"))
	if err != nil {
		t.Fatal(err)
	}
	controlRoot := strings.TrimSpace(string(controlRootBytes))
	records, err := ReadRecords(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Suite == "review-outer" {
			custodian = record.Watchdog.Ref()
		}
	}
	if custodian.Pid <= 0 {
		t.Fatalf("outer custodian record missing: %+v", records)
	}
	markers, err := filepath.Glob(filepath.Join(directory, "lease-*"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("expected one outer marker: %v %v", markers, err)
	}
	assertDirty := func(stage string) {
		t.Helper()
		data, readErr := os.ReadFile(markers[0])
		if readErr != nil {
			t.Fatalf("%s: read outer marker: %v", stage, readErr)
		}
		var marker hostLeaseRecord
		if json.Unmarshal(data, &marker) != nil || marker.Cleared || marker.Class != "heavy" || len(marker.Resources) != 1 {
			t.Fatalf("%s: outer claim lost custody: %s", stage, data)
		}
	}
	assertDirty("after nested completion")
	for _, ref := range []identity.Ref{custodian, launcherExact.Ref(), worker} {
		if liveCustodyRef(prober, ref) {
			if err := identity.SignalExact(prober, ref, syscall.SIGKILL); err != nil {
				t.Fatalf("kill exact custody process %+v: %v", ref, err)
			}
		}
	}
	if err := launcher.Wait(); err == nil {
		t.Fatal("killed launcher exited successfully")
	}
	launcherWaited = true
	if !liveCustodyRef(prober, grandchild) {
		t.Fatal("ordinary no-descriptor grandchild did not survive lost custody")
	}
	assertDirty("after outer custody loss")
	refuse := func(stage string) {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		type outcome struct {
			lease *HostResourceLease
			err   error
		}
		finished := make(chan outcome, 1)
		observed := make(chan struct{})
		waitCtx := WithHostResourceWaitObserver(ctx, func() { close(observed) })
		go func() {
			contender, acquireErr := AcquireHostResources(waitCtx, directory, conf, "heavy", []string{"fixture-db"})
			finished <- outcome{lease: contender, err: acquireErr}
		}()
		select {
		case <-observed:
		case result := <-finished:
			if result.lease != nil {
				_ = result.lease.Close()
			}
			t.Fatalf("%s: contender returned before an occupied-slot scan: %v", stage, result.err)
		case <-t.Context().Done():
			t.Fatalf("%s: contender did not reach an occupied-slot scan: %v", stage, t.Context().Err())
		}
		if active, activeErr := activeHostResourceSlots(directory); activeErr != nil || active < 1 {
			t.Fatalf("%s: dirty outer phase did not occupy its slot: active=%d err=%v", stage, active, activeErr)
		}
		cancel()
		result := <-finished
		if result.lease != nil {
			_ = result.lease.Close()
			t.Fatalf("%s: contender acquired an uncleared outer phase", stage)
		}
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("%s: controlled cancellation returned %v", stage, result.err)
		}
	}
	refuse("live no-descriptor descendant")
	if !liveCustodyRef(prober, grandchild) {
		t.Fatal("grandchild exited before admission refusal was observed")
	}
	if err := identity.SignalExact(prober, grandchild, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitCustodyTerminal(t, prober, grandchild)
	assertDirty("after exact descendant death")
	refuse("after descendant death without proven cleanup")
}

func TestHostResourceNestedCustodySubprocess(t *testing.T) {
	if os.Getenv("METASYSTEM_NESTED_CUSTODY_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args)-separator != 7 {
		t.Fatal("invalid nested custody helper arguments")
	}
	mode, directory, conf, engine := os.Args[separator+1], os.Args[separator+2], os.Args[separator+3], os.Args[separator+4]
	processFile, ackPath := os.Args[separator+5], os.Args[separator+6]
	releasePath := os.Getenv("METASYSTEM_NESTED_CUSTODY_RELEASE")
	if releasePath == "" {
		t.Fatal("nested custody release descriptor path is unavailable")
	}
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	if mode == "launcher" {
		fixture := newOwnershipFixture(t)
		attempt, _ := fixture.reserve("nested-custody", "nested-custody-plan", map[string]string{"native": strings.Repeat("d", 64)}, "", "", 0)
		if err := os.WriteFile(filepath.Join(directory, "control-root"), []byte(fixture.root), 0o600); err != nil {
			t.Fatal(err)
		}
		lease, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
		if err != nil {
			t.Fatal(err)
		}
		code := LaunchSuite(LaunchOptions{Suite: "review-outer", Root: fixture.root, ControlRoot: fixture.root,
			AttemptID: attempt.AttemptID, Deadline: deadline, ConfPath: conf,
			ProgressPath: filepath.Join(directory, "outer.progress"), LogPath: filepath.Join(directory, "outer.log"),
			Banner: "outer nested custody", Silence: time.Second, SectionCap: time.Second,
			EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
			TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
			WatchdogExecutable: engine,
			Command:            []string{os.Args[0], "-test.run=^TestHostResourceNestedCustodySubprocess$", "--", "worker", directory, conf, engine, processFile, ackPath},
			Environment: append(os.Environ(), "METASYSTEM_NESTED_CUSTODY_HELPER=1", "METASYSTEM_CENSUS_PROCESS_FILE="+processFile,
				"METASYSTEM_NESTED_CUSTODY_RELEASE="+releasePath),
			HostResourceFiles: lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
		if code == 0 {
			t.Fatal("outer worker unexpectedly completed")
		}
		return
	}
	if mode != "worker" {
		t.Fatal("invalid nested custody mode")
	}
	controlRoot := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT")
	lease, err := AcquireHostResources(context.Background(), controlRoot, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if !lease.borrowed {
		t.Fatal("nested worker did not borrow the outer lease")
	}
	code := LaunchSuite(LaunchOptions{Suite: "review-nested", Root: controlRoot, ControlRoot: controlRoot,
		ConfPath: conf, ProgressPath: filepath.Join(directory, "nested.progress"), LogPath: filepath.Join(directory, "nested.log"),
		Banner: "nested completed phase", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
		TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond,
		WatchdogExecutable: engine, Command: []string{"sh", "-c", "exit 0"},
		Environment:       append(os.Environ(), "METASYSTEM_CENSUS_PROCESS_FILE="+processFile),
		HostResourceFiles: lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
	if code != 0 {
		t.Fatalf("nested proof did not complete: %d", code)
	}
	if err := os.WriteFile(filepath.Join(directory, "nested.done"), []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	ackPipe, err := os.OpenFile(ackPath, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ackPipe.Write([]byte{1}); err != nil {
		_ = ackPipe.Close()
		t.Fatal(err)
	}
	if err := ackPipe.Close(); err != nil {
		t.Fatal(err)
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	readyPath := filepath.Join(directory, "grandchild.ready")
	grandchild := exec.Command("sh", "-c", `exec 3<"$1"; printf ready >"$2"; exec cat <&3`, "sh", releasePath, readyPath)
	grandchild.Stdin, grandchild.Stdout, grandchild.Stderr = devnull, devnull, devnull
	if err := grandchild.Start(); err != nil {
		t.Fatal(err)
	}
	grandchildWaited := false
	t.Cleanup(func() {
		if !grandchildWaited {
			_ = grandchild.Process.Kill()
			_ = grandchild.Wait()
		}
	})
	for name, data := range map[string]string{
		"worker.pid": strconv.Itoa(os.Getpid()), "grandchild.pid": strconv.Itoa(grandchild.Process.Pid),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestHostResourceBorrowerCannotCleanOuterDirtyMarker(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	fixture := newOwnershipFixture(t)
	parent, _ := fixture.reserve("nested-goal", "nested-plan", map[string]string{"native": strings.Repeat("b", 64)}, "", "", 0)
	outer, err := AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	if err := MarkHostResourcesDirty(outer.Files()); err != nil {
		t.Fatal(err)
	}
	if err := hostResourceReviewChild(t, fixture.root, parent.AttemptID, directory, conf, "heavy", "", "borrow-clean", outer.Files()); err != nil {
		t.Fatalf("nested borrower could not complete: %v", err)
	}
	var marker *os.File
	for _, file := range outer.Files() {
		if strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			marker = file
			break
		}
	}
	if marker == nil {
		t.Fatal("outer lease has no marker")
	}
	data, err := os.ReadFile(marker.Name())
	if err != nil {
		t.Fatal(err)
	}
	var record hostLeaseRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Cleared {
		t.Fatal("nested completion cleaned the outer reservation")
	}
	if err := outer.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type outcome struct {
		lease *HostResourceLease
		err   error
	}
	finished := make(chan outcome, 1)
	observed := make(chan struct{})
	waitCtx := WithHostResourceWaitObserver(ctx, func() { close(observed) })
	go func() {
		contender, acquireErr := AcquireHostResources(waitCtx, directory, conf, "heavy", nil)
		finished <- outcome{lease: contender, err: acquireErr}
	}()
	select {
	case <-observed:
	case result := <-finished:
		if result.lease != nil {
			_ = result.lease.Close()
		}
		t.Fatalf("contender returned before scanning the dirty outer marker: %v", result.err)
	case <-t.Context().Done():
		t.Fatalf("contender did not scan the dirty outer marker: %v", t.Context().Err())
	}
	if active, activeErr := activeHostResourceSlots(directory); activeErr != nil || active != 1 {
		t.Fatalf("dirty outer marker did not occupy its slot: active=%d err=%v", active, activeErr)
	}
	cancel()
	result := <-finished
	if result.lease != nil {
		_ = result.lease.Close()
		t.Fatal("contender acquired after dirty outer custody was lost")
	}
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("dirty outer marker controlled cancellation: %v", result.err)
	}
}

func TestHostResourceLockedEmptyMarkerRefusesAdmission(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	marker, acquired, err := tryHostFile(filepath.Join(directory, "lease-heavy-00000000000000000000000000000000"))
	if err != nil || !acquired {
		t.Fatalf("lock empty marker: acquired=%v err=%v", acquired, err)
	}
	defer marker.Close()
	lease, err := AcquireHostResources(t.Context(), directory, conf, "heavy", nil)
	if lease != nil {
		_ = lease.Close()
		t.Fatal("locked empty marker was ignored")
	}
	if err == nil || !strings.Contains(err.Error(), "unreconciled proof resource marker") {
		t.Fatalf("locked unknown marker should refuse immediately: %v", err)
	}
}

func TestHostResourceAdmissionReclaimsOnlyValidatedCleanUnlockedMarkers(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	markerPath := func(lease *HostResourceLease) string {
		t.Helper()
		for _, file := range lease.Files() {
			if strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
				return file.Name()
			}
		}
		t.Fatal("resource lease has no marker")
		return ""
	}
	clean, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanPath := markerPath(clean)
	busy, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(busy.Files()); _ = busy.Close() }()
	busyPath := markerPath(busy)
	dirty, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	dirtyPath := markerPath(dirty)
	if err := MarkHostResourcesDirty(dirty.Files()); err != nil {
		t.Fatal(err)
	}
	if err := dirty.Close(); err != nil {
		t.Fatal(err)
	}
	if err := MarkHostResourcesClean(clean.Files()); err != nil {
		t.Fatal(err)
	}
	if err := clean.Close(); err != nil {
		t.Fatal(err)
	}
	trigger, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = MarkHostResourcesClean(trigger.Files()); _ = trigger.Close() }()
	if _, err := os.Stat(cleanPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean unlocked marker was not reclaimed: %v", err)
	}
	for _, path := range []string{busyPath, dirtyPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("busy or dirty marker was removed: %s: %v", path, err)
		}
	}
	unknownPath := filepath.Join(directory, "lease-heavy-ffffffffffffffffffffffffffffffff")
	unknown, acquired, err := tryHostFile(unknownPath)
	if err != nil || !acquired {
		t.Fatalf("lock unknown marker: acquired=%v err=%v", acquired, err)
	}
	defer unknown.Close()
	if contender, err := AcquireHostResources(context.Background(), directory, conf, "cheap", nil); err == nil {
		_ = contender.Close()
		t.Fatal("unknown locked marker was ignored")
	}
	if _, err := os.Stat(unknownPath); err != nil {
		t.Fatalf("unknown marker was removed: %v", err)
	}
}

func TestHostResourceReviewSubprocess(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return
	}
	if len(os.Args)-separator != 6 {
		t.Fatal("invalid resource review child arguments")
	}
	directory, conf, class, resource, mode := os.Args[separator+1], os.Args[separator+2], os.Args[separator+3], os.Args[separator+4], os.Args[separator+5]
	hostAdmissionDirectoryForTest = directory
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
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
		t.Fatal("child did not borrow parent lease")
	}
	if mode == "borrow-clean" {
		if err := MarkHostResourcesClean(lease.Files()); err != nil {
			t.Fatal(err)
		}
	}
}
