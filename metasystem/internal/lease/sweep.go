package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// terminalStatuses are the job states the sweep must never touch — the job
// has already reached a verdict.
var terminalStatuses = map[string]bool{
	"completed": true, "failed": true, "timeout": true, "cancelled": true,
}

// sweepableStatuses are the in-flight states a stale job can be in when the
// sweep fails it.
var sweepableStatuses = map[string]bool{
	"pending-setup": true, "pending": true, "running": true,
}

// cleanupStaleJobs fails every job whose claim epoch predates the given epoch,
// stopping its process group first when ownership can be proven. It runs after
// a takeover (or succession) so a new generation never inherits a previous
// holder's live work. A job it cannot prove ownership of is a hard error: the
// sweep must not certify a generation it did not actually clear.
func (c *claimer) cleanupStaleJobs(epoch int64) error {
	jobsDir := filepath.Join(c.root, "artifacts/agents/jobs")
	locksDir := filepath.Join(c.root, "artifacts/agents/record-locks")
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		return err
	}
	paths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	sort.Strings(paths)
	swept := 0
	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		done, err := c.sweepOne(path, stem, epoch, locksDir)
		if err != nil {
			return err
		}
		if done {
			swept++
		}
	}
	if err := c.cleanupStaleRuns(epoch); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "sweep-completed", fmt.Sprintf("epoch %d", epoch),
		map[string]string{"epoch": itoa(epoch), "sweptCount": itoa(int64(swept))})
	return nil
}

// cleanupStaleRuns hands the run half of the takeover sweep to the run
// package, which owns the lock, the wrapped-only proof rule, the bounded
// drain, and the forced conclusion (critique finding 1); this side
// supplies the group-proof and kill seams the job sweep already tests.
func (c *claimer) cleanupStaleRuns(epoch int64) error {
	store := &run.Store{Root: c.root}
	return store.SweepStale(epoch,
		func(pgid int64, nonce string) (bool, bool) { return groupOwnsTag(pgid, nonce, nil) },
		func(pgid int64) error {
			switch err := sweepKill(pgid, unix.SIGTERM); err {
			case nil, unix.ESRCH:
				return nil
			default:
				return err
			}
		})
}

func (c *claimer) sweepOne(path, stem string, epoch int64, locksDir string) (bool, error) {
	lock, err := acquireRecordLock(filepath.Join(locksDir, stem+".lock"))
	if err != nil {
		return false, err
	}
	defer lock.release()

	data, err := os.ReadFile(path)
	if err != nil {
		// Only a record that VANISHED between glob and read is "nothing to
		// sweep" (review lease-census-2). Any other failure is a job this
		// sweep cannot prove cleared — and the function's own contract
		// forbids certifying a generation it did not actually clear.
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("claim sweep cannot read job record %s: %w", stem, err)
	}
	var job map[string]any
	if json.Unmarshal(data, &job) != nil {
		return false, fmt.Errorf("claim sweep cannot parse job record %s", stem)
	}
	status, _ := job["status"].(string)
	if !terminalStatuses[status] && !sweepableStatuses[status] {
		return false, fmt.Errorf("claim sweep refused: job record %s has an unknown status %q", stem, status)
	}
	// claimEpoch is NULLABLE by the writer's contract (dispatch's
	// nullableEpoch: a human-dispatched job carries null): a null or
	// absent epoch means the job belongs to NO lease generation, so an
	// epoch sweep has no claim over it and skipping is truthful. Only a
	// present, wrong-typed value is schema corruption the sweep must
	// refuse rather than certify past (the supervision canary caught the
	// stricter first cut of this refusing legitimate null-epoch records).
	rawEpoch, present := job["claimEpoch"]
	if !present || rawEpoch == nil {
		// One exception owns the null: a steward continuation was
		// launched precisely because NO holder was alive, so a new
		// generation must not inherit it — the arriving holder's
		// sweep clears it like any stale job.
		if role, _ := job["role"].(string); role == "steward-continuation" && !terminalStatuses[status] {
			return true, c.concludeStaleJob(path, stem, job)
		}
		return false, nil
	}
	recordEpoch, ok := jsonInt(rawEpoch)
	if !ok {
		return false, fmt.Errorf("claim sweep refused: job record %s has a noninteger claimEpoch", stem)
	}
	if recordEpoch >= epoch || terminalStatuses[status] {
		return false, nil
	}
	return true, c.concludeStaleJob(path, stem, job)
}

// concludeStaleJob stops a stale job's group and fails its record —
// the one conclusion both the epoch sweep and the steward-null
// exception share.
func (c *claimer) concludeStaleJob(path, stem string, job map[string]any) error {
	if err := c.stopStaleGroup(job, stem); err != nil {
		return err
	}
	job["status"] = "failed"
	job["phase"] = "claim-sweep"
	job["error"] = "stale-claim-epoch"
	if _, present := job["endedAt"].(string); !present || job["endedAt"] == "" {
		job["endedAt"] = nowStamp()
	}
	if err := atomicJSON(path, job); err != nil {
		return err
	}
	mission, _ := job["mission"].(string)
	c.emitter.Emit(c.root, "job-verdict", "stale-claim-epoch sweep", map[string]string{
		"jobId": stem, "verdict": "failed", "reason": "stale-claim-epoch", "missionId": mission,
	})
	return nil
}

// stopStaleGroup SIGTERMs a stale job's process group, but only after proving
// the group is still ours (its pgid is live and carries the job's tag). If
// ownership cannot be proven, the sweep refuses rather than kill blindly.
func (c *claimer) stopStaleGroup(job map[string]any, stem string) error {
	pgid, ok := jsonInt(job["pgid"])
	tag, tagOK := job["instanceTag"].(string)
	if !ok || pgid <= 1 || !tagOK || tag == "" {
		return nil
	}
	// SIGNAL authorization is KERNEL-ONLY (B1 critique finding 1): a
	// fixture row must never be the evidence that TERMs a real process
	// group. The claimer's probe serves classification reads, not this.
	owned, provable := groupOwnsTag(pgid, tag, nil)
	if !provable {
		return fmt.Errorf("claim sweep cannot prove ownership of stale job %s", stem)
	}
	if !owned {
		return nil
	}
	switch err := sweepKill(pgid, unix.SIGTERM); err {
	case nil, unix.ESRCH:
		return nil
	case unix.EPERM:
		return fmt.Errorf("claim sweep cannot stop stale job %s", stem)
	default:
		return fmt.Errorf("claim sweep cannot stop stale job %s: %w", stem, err)
	}
}

// groupOwnsTag reports whether any live process in the given process group
// carries tag in its command line. provable is false only when the process
// table cannot be read at all, which the sweep treats as inability to prove
// ownership.
// The sweep's process-table reads go through seams so the refusal rows —
// the branches standing between a takeover sweep and SIGTERM-ing a
// recycled group — are testable (review lease-census-10).
var (
	sweepAllPids = identity.AllPids
	sweepGetpgid = func(pid int64) (int64, error) {
		pg, err := unix.Getpgid(int(pid))
		return int64(pg), err
	}
	sweepProcessCommand = ProcessCommand
	sweepKill           = func(pgid int64, sig unix.Signal) error {
		return unix.Kill(int(-pgid), sig)
	}
)

func groupOwnsTag(pgid int64, tag string, probe identity.FixtureProbe) (owned, provable bool) {
	pids, err := sweepAllPids()
	if err != nil {
		return false, false
	}
	for _, pid := range pids {
		pg, err := sweepGetpgid(pid)
		if err != nil {
			// Only ESRCH proves the member is gone (review
			// lease-census-2); any other inspection failure means this
			// group's ownership cannot be PROVEN either way, and an
			// unproven group must never be ruled "provably not owned".
			if err == unix.ESRCH {
				continue
			}
			return false, false
		}
		if pg != pgid {
			continue
		}
		command, ok := sweepProcessCommand(pid, probe)
		if !ok {
			// A live member whose identity cannot be read: ownership is
			// unprovable, not disproven.
			return false, false
		}
		if strings.Contains(command, tag) {
			return true, true
		}
	}
	return false, true
}

// acquireRecordLock takes a blocking exclusive lock on a job's record lock,
// serialising the sweep's rewrite against dispatch's own record writes.
// acquireRecordLock is BOUNDED like the lease lock one layer up, and for
// the documented reason there (review lease-census-9): the sweep holds the
// lease lock while taking record locks, so a wedged-alive record-lock
// holder — the stale-job class this sweep exists to handle — would turn
// every claim, renew, and succession into an unexplained hang. Bounded
// acquisition turns it into a loud refusal naming the record.
func acquireRecordLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	wait := lockWaitSeconds()
	deadline := time.Now().Add(time.Duration(wait * float64(time.Second)))
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return &fileLock{f: f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("claim sweep cannot lock job record %s after %gs; a wedged holder keeps it", filepath.Base(path), wait)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// protocolCounts totals the distinct protocol-error keys per root job chain —
// the inheritance a successor reports so a new main knows what its predecessor
// left unresolved.
func protocolCounts(root string) map[string]int {
	jobsDir := filepath.Join(root, "artifacts/agents/jobs")
	paths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	records := map[string]map[string]any{}
	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var value map[string]any
		if json.Unmarshal(data, &value) != nil {
			continue
		}
		if id, _ := value["jobId"].(string); id == stem {
			records[stem] = value
		}
	}
	keys := map[string]map[string]bool{}
	for job, value := range records {
		rootJob, ok := chainRoot(records, job)
		if !ok {
			continue
		}
		protoErr, isObj := value["protocolError"].(map[string]any)
		if !isObj {
			continue
		}
		key, isStr := protoErr["key"].(string)
		if !isStr {
			continue
		}
		if keys[rootJob] == nil {
			keys[rootJob] = map[string]bool{}
		}
		keys[rootJob][key] = true
	}
	counts := map[string]int{}
	for job, set := range keys {
		counts[job] = len(set)
	}
	return counts
}

// chainRoot walks parentJob links to the root of a job's chain. ok is false
// on a broken link or a cycle.
func chainRoot(records map[string]map[string]any, job string) (string, bool) {
	seen := map[string]bool{}
	for {
		value, present := records[job]
		if !present || seen[job] {
			return "", false
		}
		seen[job] = true
		parent, hasParent := value["parentJob"]
		if parent == nil && !hasParent {
			return job, true
		}
		if parent == nil {
			return job, true
		}
		parentStr, isStr := parent.(string)
		if !isStr {
			return "", false
		}
		job = parentStr
	}
}

// inheritedProtocolTotal reports, on stderr, the protocol errors a successor
// inherits from its predecessor — a witness line, never fatal.
func (c *claimer) inheritedProtocolTotal(predecessor string) int {
	counts := protocolCounts(c.root)
	total := 0
	for _, n := range counts {
		total += n
	}
	if total > 0 {
		fmt.Fprintf(os.Stderr, "INHERITED-PROTOCOL-ERRORS predecessor=%s total=%d\n", predecessor, total)
	}
	return total
}
