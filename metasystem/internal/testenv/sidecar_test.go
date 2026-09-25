package testenv

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// startSidecarCustodian stands in for a fixture custodian the way
// startFixtureCustodian launches one: the log is locked exclusively and handed
// to the child as stderr. The returned function ends the child and joins it.
func startSidecarCustodian(t *testing.T, logPath string) func() {
	t.Helper()
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND|unix.O_NOFOLLOW, 0o600)
	checkTestenv(t, err)
	checkTestenv(t, unix.Flock(int(log.Fd()), unix.LOCK_EX))
	command := exec.Command("/bin/cat")
	stdin, err := command.StdinPipe()
	checkTestenv(t, err)
	command.Stderr = log
	checkTestenv(t, command.Start())
	checkTestenv(t, log.Close())
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		checkTestenv(t, stdin.Close())
		checkTestenv(t, command.Wait())
	}
	t.Cleanup(stop)
	return stop
}

func requireSidecars(t *testing.T, present bool, paths ...string) {
	t.Helper()
	for _, path := range paths {
		_, err := os.Lstat(path)
		if present && err != nil {
			t.Errorf("%s was removed: %v", filepath.Base(path), err)
		}
		if !present && !os.IsNotExist(err) {
			t.Errorf("%s survived cleanup: %v", filepath.Base(path), err)
		}
	}
}

func TestRegistrySidecarCleanupWaitsForInheritedLeaseAndCustodian(t *testing.T) {
	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	t.Setenv(supervisionRegistryHome, owner.path)
	t.Setenv(registryOwnerNonce, owner.nonce)
	inherited := inheritedRegistryHome()
	if inherited == nil {
		t.Fatal("inherited registry lease was not joined")
	}
	t.Cleanup(func() { _ = inherited.cleanup() })

	// The filename PID names the owner test binary, not the custodian process.
	runningRecords, runningLog := owner.path+".fixture-refs-4242", owner.path+".custodian-4242.log"
	// A custodian that settled has already removed its own records.
	finishedLog := owner.path + ".custodian-4343.log"
	checkTestenv(t, os.WriteFile(runningRecords, nil, 0o600))
	checkTestenv(t, os.WriteFile(runningLog, []byte("running custodian diagnostic\n"), 0o600))
	checkTestenv(t, os.WriteFile(finishedLog, []byte("finished custodian diagnostic\n"), 0o600))
	old := time.Now().Add(-8 * 24 * time.Hour)
	for _, path := range []string{runningRecords, runningLog} {
		checkTestenv(t, os.Chtimes(path, old, old))
	}
	stopCustodian := startSidecarCustodian(t, runningLog)

	var report bytes.Buffer
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, runningRecords, runningLog, finishedLog)

	checkTestenv(t, inherited.cleanup())
	checkTestenv(t, owner.cleanupReporting(&report))
	requireSidecars(t, false, finishedLog)
	// The owner marker outlives the owner while its custodian still runs.
	requireSidecars(t, true, owner.path, runningRecords, runningLog)
	if !strings.Contains(report.String(), "finished custodian diagnostic") || strings.Contains(report.String(), "running custodian") {
		t.Fatalf("report after owner cleanup = %q", report.String())
	}

	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, runningRecords, runningLog)

	// The custodian settles: it removes its own records, then exits.
	checkTestenv(t, os.Remove(runningRecords))
	stopCustodian()
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, false, owner.path, runningLog)
	if !strings.Contains(report.String(), "running custodian diagnostic") {
		t.Fatalf("custodian log was deleted without its diagnostic: %q", report.String())
	}
}

func TestRegistrySidecarCleanupReportsBoundedTailAndKeepsSentinels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	other, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	t.Cleanup(func() { _ = other.cleanup() })
	write := func(path, contents string) string {
		checkTestenv(t, os.WriteFile(path, []byte(contents), 0o600))
		return path
	}
	quietOwner := liveTestOwner(t)
	large := "HEAD-MARKER\n" + strings.Repeat("fixture-custodian action=observe filler\n", 2*fixtureCustodianLogReportLimit/40) + "TAIL-MARKER\n"
	removed := []string{
		write(owner.path+".custodian-1.log", large),
		write(owner.path+".custodian-2.log", identity.FixtureCustodianWatchLine(quietOwner)+identity.FixtureCustodianCompletionLine(quietOwner)),
	}
	linkTarget := write(filepath.Join(root, "log-link-target"), "target")
	symlinkLog := owner.path + ".custodian-7.log"
	checkTestenv(t, os.Symlink(linkTarget, symlinkLog))
	recordsDirectory := owner.path + ".fixture-refs-8"
	checkTestenv(t, os.Mkdir(recordsDirectory, 0o700))
	kept := []string{
		write(owner.path+"9.fixture-refs-5", ""),
		write(owner.path+"9.custodian-5.log", "prefix sibling"),
		write(other.path+".fixture-refs-6", ""),
		write(other.path+".custodian-6.log", "other registry"),
		write(filepath.Join(root, "other.custodian-1.log"), "unrelated"),
		write(owner.path+".fixture-refs-7", ""),
		// Records without a log are unproven, and pending sidecars keep the home.
		write(owner.path+".fixture-refs-3", ""),
		// Records beside an unlocked log are evidence of failed settlement.
		write(owner.path+".fixture-refs-4", "recorded fixture\n"),
		write(owner.path+".custodian-4.log", "FAILED-SETTLEMENT\n"),
		symlinkLog, linkTarget, recordsDirectory, owner.path,
	}

	var report bytes.Buffer
	checkTestenv(t, owner.cleanupReporting(&report))
	requireSidecars(t, false, removed...)
	requireSidecars(t, true, kept...)
	text := report.String()
	if !strings.Contains(text, "TAIL-MARKER") || strings.Contains(text, "HEAD-MARKER") {
		t.Fatalf("report is not the log's bounded tail: head=%t tail=%t", strings.Contains(text, "HEAD-MARKER"), strings.Contains(text, "TAIL-MARKER"))
	}
	if len(text) > fixtureCustodianLogReportLimit+512 {
		t.Fatalf("report is %d bytes, above the %d byte bound", len(text), fixtureCustodianLogReportLimit)
	}
	if strings.Contains(text, "custodian-2.log") {
		t.Fatalf("quiet custodian log was reported: %q", text)
	}
}

func TestRegistrySidecarCleanupReportsRemovalErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dead, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	checkTestenv(t, dead.lock.Close())
	stem := dead.path
	checkTestenv(t, os.WriteFile(stem+".custodian-1.log", []byte("diagnostic\n"), 0o600))
	checkTestenv(t, os.Chmod(root, 0o500))
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	var report bytes.Buffer
	removeDeadRegistryHomes(root, &report)
	if !strings.Contains(report.String(), "diagnostic") || !strings.Contains(report.String(), "remove registry sidecars") {
		t.Fatalf("removal error was not reported after the diagnostic: %q", report.String())
	}
	requireSidecars(t, true, stem, stem+".custodian-1.log")
}

func TestRegistrySidecarCleanupKeepsRecordsWhenLiveCustodianLogIsUnlinked(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	records, log := owner.path+".fixture-refs-4242", owner.path+".custodian-4242.log"
	checkTestenv(t, os.WriteFile(records, []byte("recorded fixture\n"), 0o600))
	old := time.Now().Add(-8 * 24 * time.Hour)
	checkTestenv(t, os.Chtimes(records, old, old))
	// The stand-in blocks on its stdin pipe, so it is alive until stopped.
	stopCustodian := startSidecarCustodian(t, log)
	checkTestenv(t, os.Remove(log))

	var report bytes.Buffer
	checkTestenv(t, owner.cleanupReporting(&report))
	requireSidecars(t, true, owner.path, records)
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, records)

	// A missing log stays unknown after the custodian exits too.
	stopCustodian()
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, records)

	// A settled custodian removes its own records and quiet log; the next
	// sweep then authenticates the dead home and removes it.
	checkTestenv(t, os.Remove(records))
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, false, owner.path)
	if report.Len() != 0 {
		t.Fatalf("unexpected report: %q", report.String())
	}
}

func TestRegistrySidecarCleanupRetainsFailedSettlement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	// The custodian exited, releasing its log lock, but kept its records
	// because its fixtures did not drain within the bound.
	records, log := owner.path+".fixture-refs-4242", owner.path+".custodian-4242.log"
	checkTestenv(t, os.WriteFile(records, []byte("recorded fixture\n"), 0o600))
	checkTestenv(t, os.WriteFile(log, []byte("run fixture custodian: identity: fixture custodian cleanup exceeded 5s\n"), 0o600))

	var report bytes.Buffer
	checkTestenv(t, owner.cleanupReporting(&report))
	requireSidecars(t, true, owner.path, records, log)
	removeDeadRegistryHomes(root, &report)
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, records, log)
	if report.Len() != 0 {
		t.Fatalf("unexpected report: %q", report.String())
	}
}

func TestRegistrySidecarCleanupReportsDiagnosticsAppendedBeforeLockRelease(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dead, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	checkTestenv(t, dead.lock.Close())
	logPath := dead.path + ".custodian-1.log"
	checkTestenv(t, os.WriteFile(logPath, nil, 0o600))

	// The custodian writes its last diagnostic after the log was opened and
	// inspected, and releases the lock just as it is acquired.
	var report bytes.Buffer
	settled, err := removeFixtureCustodianSidecarsLocking(dead.path, "1", &report, func(log *os.File) error {
		writer, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0)
		checkTestenv(t, err)
		_, err = writer.WriteString("FINAL-DIAGNOSTIC\n")
		checkTestenv(t, errors.Join(err, writer.Close()))
		return unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	})
	checkTestenv(t, err)
	if !settled || !strings.Contains(report.String(), "(last 17 of 17 bytes):\nFINAL-DIAGNOSTIC\n") {
		t.Fatalf("settled=%t report=%q", settled, report.String())
	}
	requireSidecars(t, false, logPath)
}
