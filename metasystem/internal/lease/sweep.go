package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	c.emitter.Emit(c.root, "sweep-completed", fmt.Sprintf("epoch %d", epoch),
		map[string]string{"epoch": itoa(epoch), "sweptCount": itoa(int64(swept))})
	return nil
}

func (c *claimer) sweepOne(path, stem string, epoch int64, locksDir string) (bool, error) {
	lock, err := acquireRecordLock(filepath.Join(locksDir, stem+".lock"))
	if err != nil {
		return false, err
	}
	defer lock.release()

	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil // vanished under us; nothing to sweep
	}
	var job map[string]any
	if json.Unmarshal(data, &job) != nil {
		return false, nil
	}
	recordEpoch, ok := jsonInt(job["claimEpoch"])
	status, _ := job["status"].(string)
	if !ok || recordEpoch >= epoch || terminalStatuses[status] || !sweepableStatuses[status] {
		return false, nil
	}
	if err := c.stopStaleGroup(job, stem); err != nil {
		return false, err
	}
	job["status"] = "failed"
	job["phase"] = "claim-sweep"
	job["error"] = "stale-claim-epoch"
	if _, present := job["endedAt"].(string); !present || job["endedAt"] == "" {
		job["endedAt"] = nowStamp()
	}
	if err := atomicJSON(path, job); err != nil {
		return false, err
	}
	mission, _ := job["mission"].(string)
	c.emitter.Emit(c.root, "job-verdict", "stale-claim-epoch sweep", map[string]string{
		"jobId": stem, "verdict": "failed", "reason": "stale-claim-epoch", "missionId": mission,
	})
	return true, nil
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
	owned, provable := groupOwnsTag(pgid, tag)
	if !provable {
		return fmt.Errorf("claim sweep cannot prove ownership of stale job %s", stem)
	}
	if !owned {
		return nil
	}
	switch err := unix.Kill(int(-pgid), unix.SIGTERM); err {
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
func groupOwnsTag(pgid int64, tag string) (owned, provable bool) {
	pids, err := identity.AllPids()
	if err != nil {
		return false, false
	}
	for _, pid := range pids {
		pg, err := unix.Getpgid(int(pid))
		if err != nil || int64(pg) != pgid {
			continue
		}
		if command, ok := ProcessCommand(pid); ok && strings.Contains(command, tag) {
			return true, true
		}
	}
	return false, true
}

// acquireRecordLock takes a blocking exclusive lock on a job's record lock,
// serialising the sweep's rewrite against dispatch's own record writes.
func acquireRecordLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
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
