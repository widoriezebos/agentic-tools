package supervise

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// reservationDuringCapAuthorityWait holds the repository cap-authority lock
// with a live owned process and swaps the arming clock for a fake one whose
// first sleep — the arming attempt waiting on that lock — writes a running
// job record reserving capMin and then releases the lock. It returns a
// function reporting whether the reservation was written during the wait.
func reservationDuringCapAuthorityWait(t *testing.T, root, job string, capMin int) func() bool {
	t.Helper()
	directory := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		t.Fatal(err)
	}
	holder := startOwnedHeldProcess(t, false, "sleep")
	holderPid := int64(holder.command.Process.Pid)
	if err := dispatch.OwnerLockClaim(directory, holderPid, "sleep"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dispatch.OwnerLockRelease(directory, holderPid, "sleep") })
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(5000, 0)
	reserved := false
	priorNow, priorSleep := armingNow, armingSleep
	armingNow = func() time.Time { return now }
	armingSleep = func(duration time.Duration) {
		now = now.Add(duration)
		if reserved {
			return
		}
		reserved = true
		record, _ := json.Marshal(map[string]any{"jobId": job, "status": "running", "capMin": capMin})
		if err := os.WriteFile(filepath.Join(jobs, job+".json"), append(record, '\n'), 0o600); err != nil {
			t.Error(err)
		}
		if err := dispatch.OwnerLockRelease(directory, holderPid, "sleep"); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { armingNow, armingSleep = priorNow, priorSleep })
	return func() bool { return reserved }
}

// restoreArmingClock puts the real arming clock back before a later arming
// that must wait on a real owner.
func restoreArmingClock(t *testing.T, prior func() time.Time, priorSleep func(time.Duration)) {
	t.Helper()
	armingNow, armingSleep = prior, priorSleep
}

// delegate-caps AUTH-R2-006, ordinary-establishment variant (U6b port): an
// establishing arm is a replacement authority too. A reservation that races
// into its cap-authority wait is seen once the lock is taken, and the lower
// config-derived ceiling refuses by the blocking job's name and reserved cap
// before any owner is launched.
func TestOrdinaryEstablishmentRefusesACeilingBelowAReservationThatRacedItsLockWait(t *testing.T) {
	root := t.TempDir()
	reserved := reservationDuringCapAuthorityWait(t, root, "ordinary-blocking-job", 400)
	options := armingOptions(root)
	options.WatcherCap = 230
	options.Command = func(...string) (*exec.Cmd, error) {
		t.Error("a refused establishment launched a supervision owner")
		return nil, errors.New("launch refused by the test")
	}
	_, err := EnsureArmed(options)
	if !reserved() {
		t.Fatal("the arming attempt never waited on the held cap-authority lock")
	}
	if err == nil || !strings.Contains(err.Error(), "ordinary-blocking-job") || !strings.Contains(err.Error(), "reserved cap 400m") {
		t.Fatalf("ordinary downward establishment was not refused by name: %v", err)
	}
	if _, statErr := os.Stat(ownerLockDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("a refused establishment retained the owner lock: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(SupervisionDir(root), "cap-authority.lock.d")); !os.IsNotExist(statErr) {
		t.Fatalf("a refused establishment kept the cap-authority lock: %v", statErr)
	}
}

// delegate-caps AUTH-R2-006 (U6b port): a downward re-arm of a live owner —
// the config-derived 230m ceiling replacing an armed 330m one — refuses
// while a running job reserves 400m, naming the job and its reserved cap,
// even when that reservation arrived while the re-arm waited for the
// cap-authority lock. The armed owner is left in place.
func TestDownwardRearmRefusesWhileARunningJobReservesAboveTheNewCeiling(t *testing.T) {
	reapArmingOwnerProcesses(t)
	root := t.TempDir()
	registryPath := isolatedArmingRegistry(t)
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	options := armingOptions(root)
	started, err := EnsureArmed(options)
	if err != nil || started.Action != "started" || !started.Inspection.Armed() {
		t.Fatalf("initial generation: result=%+v err=%v", started, err)
	}
	appendPreviousOwnerRows(t, registryPath, root, started)
	t.Cleanup(func() { _, _ = ShutdownAt(root, root, root, "metasystem-supervision-owner-test-", 1) })

	priorNow, priorSleep := armingNow, armingSleep
	reserved := reservationDuringCapAuthorityWait(t, root, "blocking-job", 400)
	downward := options
	downward.WatcherCap = 230
	_, err = EnsureArmed(downward)
	if !reserved() {
		t.Fatal("the re-arm never waited on the held cap-authority lock")
	}
	if err == nil || !strings.Contains(err.Error(), "blocking-job") || !strings.Contains(err.Error(), "reserved cap 400m") {
		t.Fatalf("downward re-arm bypass was accepted or unnamed: %v", err)
	}
	restoreArmingClock(t, priorNow, priorSleep)

	verified, err := EnsureArmed(options)
	if err != nil || verified.Action != "verified" || verified.Owner.Pid != started.Owner.Pid {
		t.Fatalf("the refused re-arm disturbed the armed owner: result=%+v err=%v", verified, err)
	}
	generation, err := ReadPublishedGeneration(root)
	if err != nil || generation.WatcherCap != 330 {
		t.Fatalf("published ceiling after the refusal = %+v err=%v, want 330", generation, err)
	}
}

// delegate-caps AUTH-R2-007's publication half (U6b port): the owner
// publishes the ceiling it was armed with as state.derivedWatcherCapMin.
func TestOwnerPublishesItsArmedCeilingAsTheDerivedWatcherCap(t *testing.T) {
	t.Parallel()
	checkout, root := diskCheckout(t)
	checkout.WatcherCap = 230
	writeOwner(t, root, 41, "owner-tag")
	if err := checkout.PublishState(nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(checkout.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var document stateDocument
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	if document.DerivedWatcherCapMin != 230 {
		t.Fatalf("derivedWatcherCapMin = %d, want the armed 230", document.DerivedWatcherCapMin)
	}
}
