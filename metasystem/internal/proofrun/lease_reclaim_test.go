package proofrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The fixtures below are the shape of the leases moved aside by hand on
// 2026-09-27 and 2026-09-28: heavy, slot-00, no resources, cleared=false,
// owner pid and group both dead.
const (
	reclaimOwnerPid   = 56449
	reclaimOwnerPgid  = 56440
	reclaimOwnerMicro = 1790530358214211
	reclaimLeaseName  = "lease-heavy-0a9b107b6b73a7cf070269f147e8a26d"
)

func reclaimExact(pid, micro int64) identity.Exact {
	exact := identity.Exact{Pid: pid, StartedAt: time.UnixMicro(micro)}
	if runtime.GOOS == "linux" {
		exact.StartTicks, exact.BootID = micro/10000, "boot-reclaim-test"
	}
	return exact
}

type reclaimProber map[int64]struct {
	exact identity.Exact
	state identity.Liveness
}

func (prober reclaimProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if row, ok := prober[pid]; ok {
		return row.exact, row.state, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

type countingProber struct {
	identity.Prober
	probes *int
}

func (prober countingProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	*prober.probes++
	return prober.Prober.Probe(pid)
}

func writeDirtyReclaimLease(t *testing.T, directory string, resources ...string) string {
	t.Helper()
	owner := processIdentity(reclaimExact(reclaimOwnerPid, reclaimOwnerMicro), reclaimOwnerPgid)
	claimed := []string{}
	for _, resource := range resources {
		claimed = append(claimed, filepath.Base(hostResourcePath(directory, resource)))
	}
	encoded, err := json.Marshal(hostLeaseRecord{Schema: 1, Owner: owner, Class: "heavy", Slot: "slot-00", Resources: claimed, Cleared: false})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, reclaimLeaseName)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type reclaimSeams struct {
	prober    identity.Prober
	groupLive bool
	groupErr  error
	survivors []identity.FixtureSurvivor
	censusErr error
	lines     *[]string
	mu        *sync.Mutex
}

func (seams reclaimSeams) reclaimer() *leaseReclaimer {
	return &leaseReclaimer{
		prober: seams.prober,
		groupLive: func(int64, identity.Prober) (bool, error) {
			return seams.groupLive, seams.groupErr
		},
		survivors: func(identity.Prober, identity.Ref) ([]identity.FixtureSurvivor, error) {
			return seams.survivors, seams.censusErr
		},
		report: func(line string) {
			seams.mu.Lock()
			defer seams.mu.Unlock()
			*seams.lines = append(*seams.lines, line)
		},
		now:         func() time.Time { return time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC) },
		controlRoot: "/checkout",
	}
}

func newReclaimSeams(prober identity.Prober) reclaimSeams {
	return reclaimSeams{prober: prober, lines: &[]string{}, mu: &sync.Mutex{}}
}

func reclaimRecords(t *testing.T, directory string) []leaseReclaimRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, reclaimedLeasesFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var records []leaseReclaimRecord
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record leaseReclaimRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("reclaim record is not JSON: %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func reclaimDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return directory
}

// Witness: dead owner, empty group, no survivors: the lease is reclaimed with
// a record, it no longer holds heavy capacity, and a waiter for its named
// resource is admitted on its first pass. The named resource keeps the waiter
// on the admission loop without the process-wide launcher census seam.
func TestDeadDirtyLeaseIsReclaimedAndTheWaiterAdmitted(t *testing.T) {
	t.Parallel()
	directory, conf := privateHostResources(t)
	path := writeDirtyReclaimLease(t, directory, "reclaim-witness")
	if active, err := activeHostResourceSlots(directory); err != nil || active != 1 {
		t.Fatalf("dead dirty lease does not hold heavy capacity before reclaim: active=%d err=%v", active, err)
	}
	seams := newReclaimSeams(reclaimProber{})
	ctx, cancel := context.WithCancel(withLeaseReclaimer(t.Context(), seams.reclaimer()))
	defer cancel()
	waited := false
	ctx = WithHostResourceWaitObserver(ctx, func() { waited = true; cancel() })
	lease, err := acquireHostResourcesIn(ctx, directory, directory, conf, "cheap", []string{"reclaim-witness"}, nil)
	if err != nil || waited {
		t.Fatalf("waiter was not admitted past the dead lease: waited=%t err=%v", waited, err)
	}
	defer lease.Close()
	if active, err := activeHostResourceSlots(directory); err != nil || active != 0 {
		t.Fatalf("reclaimed lease still holds heavy capacity: active=%d err=%v", active, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("dead lease was not removed: %v", statErr)
	}
	records := reclaimRecords(t, directory)
	if len(records) != 1 || records[0].Lease != reclaimLeaseName || records[0].Owner.Pid != reclaimOwnerPid ||
		records[0].Owner.Pgid != reclaimOwnerPgid || !strings.Contains(records[0].OwnerDead, "not running") ||
		records[0].At != "2026-09-28T15:00:00Z" {
		t.Fatalf("reclaim record does not name the lease, owner and proof: %+v", records)
	}
	if len(*seams.lines) != 0 {
		t.Fatalf("reclaim printed a keep line: %q", *seams.lines)
	}
}

func assertKept(t *testing.T, directory, path string) {
	t.Helper()
	dirty, _, err := hostLeaseStateWithReclaim(directory, false, nil)
	if err != nil || dirty != 1 {
		t.Fatalf("kept lease does not count against admission: dirty=%d err=%v", dirty, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("kept lease was removed: %v", statErr)
	}
	if records := reclaimRecords(t, directory); len(records) != 0 {
		t.Fatalf("kept lease has a reclaim record: %+v", records)
	}
}

// Witness: a live owner (exact identity matches) keeps its lease, silently.
func TestLiveOwnerKeepsItsDirtyLease(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	path := writeDirtyReclaimLease(t, directory)
	seams := newReclaimSeams(reclaimProber{reclaimOwnerPid: {reclaimExact(reclaimOwnerPid, reclaimOwnerMicro), identity.Alive}})
	if _, _, err := hostLeaseStateWithReclaim(directory, true, seams.reclaimer()); err != nil {
		t.Fatal(err)
	}
	assertKept(t, directory, path)
	if len(*seams.lines) != 0 {
		t.Fatalf("a live owner's lease printed a keep line: %q", *seams.lines)
	}
}

// Witness: the same pid with another start time is a different process; the
// recorded owner is dead and the lease is reclaimed.
func TestReusedOwnerPidIsTreatedAsDead(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	path := writeDirtyReclaimLease(t, directory)
	seams := newReclaimSeams(reclaimProber{reclaimOwnerPid: {reclaimExact(reclaimOwnerPid, reclaimOwnerMicro+5_000_000), identity.Alive}})
	dirty, _, err := hostLeaseStateWithReclaim(directory, true, seams.reclaimer())
	if err != nil || dirty != 0 {
		t.Fatalf("reused-pid lease still counts: dirty=%d err=%v", dirty, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("reused-pid lease was not removed: %v", statErr)
	}
	records := reclaimRecords(t, directory)
	if len(records) != 1 || !strings.Contains(records[0].OwnerDead, "reused") {
		t.Fatalf("reclaim record does not name pid reuse: %+v", records)
	}
}

// Witness: a detached fixture tagged to the dead owner keeps the lease, with
// one line naming the lease, owner, survivor and the settling verb.
func TestDetachedFixtureSurvivorKeepsTheLease(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	path := writeDirtyReclaimLease(t, directory)
	seams := newReclaimSeams(reclaimProber{})
	seams.survivors = []identity.FixtureSurvivor{{Class: identity.FixtureSurvivorCertain, Ref: reclaimExact(70001, reclaimOwnerMicro+1).Ref()}}
	if _, _, err := hostLeaseStateWithReclaim(directory, true, seams.reclaimer()); err != nil {
		t.Fatal(err)
	}
	assertKept(t, directory, path)
	if len(*seams.lines) != 1 || !strings.Contains((*seams.lines)[0], reclaimLeaseName) ||
		!strings.Contains((*seams.lines)[0], "pid 70001") || !strings.Contains((*seams.lines)[0], "fixture-survivors --owner") {
		t.Fatalf("survivor keep line is missing or incomplete: %q", *seams.lines)
	}
}

// Witness: an Unknown census keeps the lease and prints its remedy exactly
// once across many waiting passes, including a re-census.
func TestUnknownCensusKeepsTheLeaseWithOneRemedyLine(t *testing.T) {
	t.Parallel()
	for _, unknown := range []string{"census", "group", "owner"} {
		t.Run(unknown, func(t *testing.T) {
			t.Parallel()
			directory := reclaimDirectory(t)
			path := writeDirtyReclaimLease(t, directory)
			prober := identity.Prober(reclaimProber{})
			if unknown == "owner" {
				prober = reclaimProber{reclaimOwnerPid: {identity.Exact{}, identity.Unknown}}
			}
			seams := newReclaimSeams(prober)
			switch unknown {
			case "census":
				seams.censusErr = errors.New("process table unreadable")
			case "group":
				seams.groupErr = errors.New("sysctl failed")
			}
			reclaimer := seams.reclaimer()
			censuses := 0
			census := reclaimer.prober
			reclaimer.prober = countingProber{census, &censuses}
			for pass := 0; pass < 2*leaseRecensusPasses+1; pass++ {
				if _, _, err := hostLeaseStateWithReclaim(directory, true, reclaimer); err != nil {
					t.Fatal(err)
				}
			}
			assertKept(t, directory, path)
			if censuses != 3 {
				t.Fatalf("a kept lease was re-examined %d times in %d passes, want 3", censuses, 2*leaseRecensusPasses+1)
			}
			if len(*seams.lines) != 1 {
				t.Fatalf("want exactly one keep line, got %q", *seams.lines)
			}
			line := (*seams.lines)[0]
			if !strings.Contains(line, reclaimLeaseName) || !strings.Contains(line, "owner pid 56449") ||
				!strings.Contains(line, "unknown") && !strings.Contains(line, "failed") || !strings.Contains(line, "settle:") {
				t.Fatalf("keep line does not name lease, owner, unknown and remedy: %q", line)
			}
		})
	}
}

// Witness: two waiters meet the same dead lease; admission.lock serializes
// them, so exactly one reclaims it and both are admitted in turn.
func TestTwoWaitersReclaimADeadLeaseExactlyOnce(t *testing.T) {
	t.Parallel()
	directory, conf := privateHostResources(t)
	writeDirtyReclaimLease(t, directory, "reclaim-race")
	var settled atomic.Int32
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for waiter := 0; waiter < 2; waiter++ {
		group.Add(1)
		go func() {
			defer group.Done()
			seams := newReclaimSeams(reclaimProber{})
			reclaimer := seams.reclaimer()
			census := reclaimer.survivors
			reclaimer.survivors = func(prober identity.Prober, owner identity.Ref) ([]identity.FixtureSurvivor, error) {
				settled.Add(1)
				return census(prober, owner)
			}
			lease, err := acquireHostResourcesIn(withLeaseReclaimer(t.Context(), reclaimer), directory, directory, conf, "cheap", []string{"reclaim-race"}, nil)
			if err == nil {
				err = lease.Close()
			}
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if records := reclaimRecords(t, directory); len(records) != 1 || settled.Load() != 1 {
		t.Fatalf("dead lease was not reclaimed exactly once: records=%d censuses=%d", len(records), settled.Load())
	}
}

// Witness for the steward's proof-admission role: a dead dirty lease is
// reported while a proof holds admission.lock and reclaimed by the next
// inspection that takes it; an unknown one carries its remedy; a held or
// live-owned one is live.
func TestInspectHostLeasesReportsThenReclaimsADeadLease(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	path := writeDirtyReclaimLease(t, directory)
	seams := newReclaimSeams(reclaimProber{})
	guard, acquired, err := tryHostFile(filepath.Join(directory, "admission.lock"))
	if err != nil || !acquired {
		t.Fatalf("hold admission.lock: acquired=%t err=%v", acquired, err)
	}
	reports, err := inspectHostLeasesIn(directory, seams.reclaimer())
	if err != nil || len(reports) != 1 || reports[0].State != HostLeaseDead || reports[0].Owner.Pid != reclaimOwnerPid || reports[0].Since.IsZero() {
		t.Fatalf("busy admission.lock did not report the dead lease: %+v err=%v", reports, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("inspection reclaimed without admission.lock: %v", statErr)
	}
	if err := releaseHostProbe(guard); err != nil {
		t.Fatal(err)
	}
	reports, err = inspectHostLeasesIn(directory, seams.reclaimer())
	if err != nil || len(reports) != 1 || reports[0].State != HostLeaseReclaimed {
		t.Fatalf("free admission.lock did not reclaim the dead lease: %+v err=%v", reports, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) || len(reclaimRecords(t, directory)) != 1 {
		t.Fatalf("reclaimed lease remains or has no record: %v", statErr)
	}
	if reports, err = inspectHostLeasesIn(directory, seams.reclaimer()); err != nil || len(reports) != 0 {
		t.Fatalf("reclaimed lease is still reported: %+v err=%v", reports, err)
	}
}

func TestInspectHostLeasesNamesUnknownRemedyAndLiveOwners(t *testing.T) {
	t.Parallel()
	unknownDirectory := reclaimDirectory(t)
	unknownPath := writeDirtyReclaimLease(t, unknownDirectory)
	unknown := newReclaimSeams(reclaimProber{})
	unknown.censusErr = errors.New("process table unreadable")
	reports, err := inspectHostLeasesIn(unknownDirectory, unknown.reclaimer())
	if err != nil || len(reports) != 1 || reports[0].State != HostLeaseUnknown ||
		!strings.Contains(reports[0].Remedy, "fixture-survivors --owner") || !strings.Contains(reports[0].Reason, "census failed") {
		t.Fatalf("unknown lease report lacks its remedy: %+v err=%v", reports, err)
	}
	if _, statErr := os.Stat(unknownPath); statErr != nil {
		t.Fatalf("unknown lease was removed: %v", statErr)
	}

	liveDirectory := reclaimDirectory(t)
	writeDirtyReclaimLease(t, liveDirectory)
	live := newReclaimSeams(reclaimProber{reclaimOwnerPid: {reclaimExact(reclaimOwnerPid, reclaimOwnerMicro), identity.Alive}})
	reports, err = inspectHostLeasesIn(liveDirectory, live.reclaimer())
	if err != nil || len(reports) != 1 || reports[0].State != HostLeaseLive {
		t.Fatalf("live owner's lease is not live: %+v err=%v", reports, err)
	}

	heldDirectory := reclaimDirectory(t)
	heldPath := writeDirtyReclaimLease(t, heldDirectory)
	holder, acquired, err := tryHostFile(heldPath)
	if err != nil || !acquired {
		t.Fatalf("hold the lease flock: acquired=%t err=%v", acquired, err)
	}
	defer func() { _ = releaseHostProbe(holder) }()
	reports, err = inspectHostLeasesIn(heldDirectory, newReclaimSeams(reclaimProber{}).reclaimer())
	if err != nil || len(reports) != 1 || reports[0].State != HostLeaseLive {
		t.Fatalf("a held lease is not live: %+v err=%v", reports, err)
	}
	if _, statErr := os.Stat(heldPath); statErr != nil {
		t.Fatalf("a held lease was removed: %v", statErr)
	}
}
