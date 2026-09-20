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
	log, err := os.Create(filepath.Join(directory, "nested-custody-helper.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	launcher := exec.Command(os.Args[0], "-test.run=^TestHostResourceNestedCustodySubprocess$", "--", "launcher", directory, conf, engine)
	launcher.Env = append(os.Environ(), "METASYSTEM_NESTED_CUSTODY_HELPER=1")
	launcher.Stdout, launcher.Stderr = log, log
	if err := launcher.Start(); err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	launcherExact, state, err := prober.Probe(int64(launcher.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe launcher: %v (%s)", err, state)
	}
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
	waitCustodyFile(t, filepath.Join(directory, "nested.done"), 12*time.Second)
	waitCustodyFile(t, filepath.Join(directory, "grandchild.ready"), 3*time.Second)
	worker = waitCustodyRef(t, prober, filepath.Join(directory, "worker.pid"), 3*time.Second)
	grandchild = waitCustodyRef(t, prober, filepath.Join(directory, "grandchild.pid"), 3*time.Second)
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
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		contender, acquireErr := AcquireHostResources(ctx, directory, conf, "heavy", []string{"fixture-db"})
		if contender != nil {
			_ = contender.Close()
			t.Fatalf("%s: contender acquired an uncleared outer phase", stage)
		}
		if !errors.Is(acquireErr, context.DeadlineExceeded) {
			t.Fatalf("%s: expected bounded refusal for dirty phase, got %v", stage, acquireErr)
		}
	}
	refuse("live no-descriptor descendant")
	if !liveCustodyRef(prober, grandchild) {
		t.Fatal("grandchild exited before admission refusal was observed")
	}
	if err := identity.SignalExact(prober, grandchild, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
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
	if separator < 0 || len(os.Args)-separator != 5 {
		t.Fatal("invalid nested custody helper arguments")
	}
	mode, directory, conf, engine := os.Args[separator+1], os.Args[separator+2], os.Args[separator+3], os.Args[separator+4]
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
			Command:            []string{os.Args[0], "-test.run=^TestHostResourceNestedCustodySubprocess$", "--", "worker", directory, conf, engine},
			Environment:        append(os.Environ(), "METASYSTEM_NESTED_CUSTODY_HELPER=1"),
			HostResourceFiles:  lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
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
		HostResourceFiles: lease.Files(), Output: os.Stdout, ErrorOutput: os.Stderr})
	if code != 0 {
		t.Fatalf("nested proof did not complete: %d", code)
	}
	if err := os.WriteFile(filepath.Join(directory, "nested.done"), []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	grandchild := exec.Command("sleep", "60")
	grandchild.Stdin, grandchild.Stdout, grandchild.Stderr = devnull, devnull, devnull
	if err := grandchild.Start(); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"worker.pid": strconv.Itoa(os.Getpid()), "grandchild.pid": strconv.Itoa(grandchild.Process.Pid),
		"grandchild.ready": fmt.Sprintf("%d", grandchild.Process.Pid),
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
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	contender, acquireErr := AcquireHostResources(ctx, directory, conf, "heavy", nil)
	if contender != nil {
		_ = contender.Close()
		t.Fatal("contender acquired after dirty outer custody was lost")
	}
	if !errors.Is(acquireErr, context.DeadlineExceeded) {
		t.Fatalf("dirty outer marker refusal: %v", acquireErr)
	}
}

func TestHostResourceLockedEmptyMarkerRefusesAdmission(t *testing.T) {
	directory, conf := isolatedHostResources(t)
	marker, acquired, err := tryHostFile(filepath.Join(directory, "lease-heavy-00000000000000000000000000000000"))
	if err != nil || !acquired {
		t.Fatalf("lock empty marker: acquired=%v err=%v", acquired, err)
	}
	defer marker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	lease, err := AcquireHostResources(ctx, directory, conf, "heavy", nil)
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
