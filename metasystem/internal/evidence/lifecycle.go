package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// The item's writer's own lease (3.12, DL4E-02, DL4F-01): a chain's mirror
// runs under the job's lifecycle lock, the directory lock
// artifacts/agents/record-locks/<job>.lifecycle.d the reaper holds around
// it. A disposer takes the lock of every job of the chain: the pass
// without waiting (held is pending), a person's verb with the reaper's own
// bound, so no mirror lands while an inventory is taken or a member is
// dropped, and a mirror that starts afterwards reads the tombstone.

// ReaperBound is the reaper's own wait for a lifecycle lock (reapOne's
// five seconds): a person's verb waits no longer.
const ReaperBound = 5 * time.Second

// LifecycleLockDir is a job's lifecycle lock in an installation.
func LifecycleLockDir(installation, job string) string {
	return filepath.Join(installation, "artifacts", "agents", "record-locks", job+".lifecycle.d")
}

// OwnerLocks takes lifecycle locks as the process (pid, tag): tag must be a
// word of the process's own command line (the lock's holder probe reads a
// live pid whose argv lacks the tag as a stranger). sleep and now bound a
// wait; the pass passes a zero wait.
func OwnerLocks(pid int64, tag string, now func() time.Time, sleep func(time.Duration)) Locks {
	return func(installation string, jobs []string, wait time.Duration) (func(), string, error) {
		var held []string
		release := func() {
			for index := len(held) - 1; index >= 0; index-- {
				_ = dispatch.OwnerLockRelease(held[index], pid, tag)
			}
		}
		if err := os.MkdirAll(filepath.Join(installation, "artifacts", "agents", "record-locks"), 0o755); err != nil {
			return nil, "", err
		}
		deadline := now().Add(wait)
		for _, job := range jobs {
			directory := LifecycleLockDir(installation, job)
			for {
				err := dispatch.OwnerLockClaim(directory, pid, tag)
				if err == nil {
					held = append(held, directory)
					break
				}
				if !errors.Is(err, dispatch.ErrOwnerLockBusy) {
					release()
					return nil, "", err
				}
				if wait <= 0 || !now().Before(deadline) {
					release()
					return nil, fmt.Sprintf("job %s's lifecycle lock is held by %s", job, lockHolder(directory)), nil
				}
				sleep(50 * time.Millisecond)
			}
		}
		return release, "", nil
	}
}

// lockHolder names a directory lock's recorded holder.
func lockHolder(directory string) string {
	data, err := os.ReadFile(filepath.Join(directory, "owner.json"))
	if err != nil {
		return "an unreadable holder"
	}
	var owner struct {
		Pid         int64  `json:"pid"`
		InstanceTag string `json:"instanceTag"`
	}
	if json.Unmarshal(data, &owner) != nil {
		return "an unreadable holder"
	}
	return fmt.Sprintf("pid=%d,tag=%s", owner.Pid, owner.InstanceTag)
}
