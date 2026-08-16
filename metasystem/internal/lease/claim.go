package lease

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// supervisionTagPrefixes mark an announcement as a supervision component
// rather than a writer.
var supervisionTagPrefixes = []string{"metasystem-supervision-"}

func isSupervisionTag(tag string) bool {
	for _, prefix := range supervisionTagPrefixes {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	return false
}

// claimer reconciles the checkout lease with a main's announcement under the
// lease lock. It emits lease lifecycle events as it goes.
type claimer struct {
	root    string
	paths   Paths
	emitter *events.Emitter
	probe   identity.FixtureProbe
}

func newClaimer(root string) (*claimer, error) {
	probe, err := fixtureProbe(root)
	if err != nil {
		return nil, err
	}
	self := int64(os.Getpid())
	started, _ := StartedAt(self, probe)
	return &claimer{
		root:    root,
		paths:   leasePaths(root),
		emitter: &events.Emitter{Component: "lease", Pid: self, PidStartedAt: started},
		probe:   probe,
	}, nil
}

// claim brings the lease into agreement with ann. It handles every case a
// main can present: a fresh checkout, its own renewal, a live holder (kept, or
// reconciled when it is the same process re-announcing — the KI-33 fix), a
// dead holder of the same lineage (succession, epoch preserved), and a dead
// holder of a foreign lineage (takeover, epoch bumped and predecessor swept).
func (c *claimer) claim(ann *Announcement) error {
	lock, err := acquireBounded(c.paths.Lock, "lease")
	if err != nil {
		return err
	}
	defer lock.release()

	// A supervision component announces so the census can see it, but it is
	// not a writer: it must never claim or disturb the checkout lease. This is
	// the KI-32 fix — a detached owner once claimed and refused its own parent.
	if isSupervisionTag(ann.InstanceTag) {
		return nil
	}

	current, err := loadLease(c.root, false)
	if err != nil {
		return err
	}

	switch {
	case current == nil:
		return c.freshClaim(ann)

	case current.HolderMainId == ann.MainId:
		return c.renewHeld(current, ann)

	case Live(current.Pid, current.PidStartedAt, c.probe):
		if current.Pid == ann.Pid && current.PidStartedAt == ann.PidStartedAt {
			// KI-33 fix: the same live process re-announced under a new mainId
			// (a --shutdown then re-arm). It still owns its own checkout, so
			// reconcile the lease to the new identity instead of stranding the
			// session as OWNED-ELSEWHERE against its own former mainId.
			return c.reconcileSameProcess(current, ann)
		}
		// A live holder that is a genuinely different process never loses the
		// lease, whatever the lineage.
		c.emitter.Emit(c.root, "lease-refused",
			fmt.Sprintf("live holder %s keeps the lease", current.HolderMainId),
			map[string]string{"holder": current.HolderMainId})
		return nil

	case leaseLineage(current) == announcementLineage(ann):
		return c.succeed(current, ann)

	default:
		return c.takeover(current, ann)
	}
}

// freshClaim claims an unheld checkout: epoch 1, revision 1, no predecessor.
func (c *claimer) freshClaim(ann *Announcement) error {
	claimed := nowStamp()
	lease := &Lease{
		HolderMainId: ann.MainId,
		OwnerLineage: announcementLineage(ann),
		Pid:          ann.Pid,
		PidStartedAt: ann.PidStartedAt,
		CommandHash:  ann.CommandHash,
		ClaimedAt:    claimed,
		RenewedAt:    claimed,
		Takeovers:    []Takeover{},
		Revision:     1,
		ClaimEpoch:   1,
	}
	if err := saveLease(c.root, lease); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "lease-claimed", "fresh claim", map[string]string{"epoch": "1"})
	return c.writeStamp(ann.MainId, 1)
}

// renewHeld renews the lease its own holder already owns, first finishing any
// sweep an earlier takeover left incomplete.
func (c *claimer) renewHeld(current *Lease, ann *Announcement) error {
	predecessor, reportInherited, err := c.completeInterruptedSweep(current)
	if err != nil {
		return err
	}
	renewed := *current
	renewed.OwnerLineage = announcementLineage(ann) // D-1c: reconcile on every renewal
	renewed.RenewedAt = nowStamp()
	renewed.Revision = current.Revision + 1
	expected := current.Revision
	if err := c.casSave(&renewed, &expected); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "lease-renewed", "same-process renewal",
		map[string]string{"epoch": itoa(renewed.ClaimEpoch)})
	if reportInherited {
		c.inheritedProtocolTotal(predecessor)
	}
	return nil
}

// reconcileSameProcess is the KI-33 fix: the same live process re-announced
// under a new mainId. Move the holder identity to the new mainId, preserve the
// epoch (its work is still valid), and bump the revision. Not a takeover.
func (c *claimer) reconcileSameProcess(current *Lease, ann *Announcement) error {
	reconciled := *current
	reconciled.HolderMainId = ann.MainId
	reconciled.CommandHash = ann.CommandHash
	reconciled.OwnerLineage = announcementLineage(ann)
	reconciled.RenewedAt = nowStamp()
	reconciled.Revision = current.Revision + 1
	expected := current.Revision
	if err := c.casSave(&reconciled, &expected); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "lease-renewed", "same-process identity reconciled (KI-33)",
		map[string]string{"epoch": itoa(reconciled.ClaimEpoch)})
	return nil
}

// succeed hands the lease from a dead predecessor of the same lineage to its
// successor process, preserving the epoch so the predecessor's in-flight jobs
// stay valid. Any interrupted sweep is finished first.
func (c *claimer) succeed(current *Lease, ann *Announcement) error {
	if !c.stampComplete(current) {
		if err := c.cleanupStaleJobs(current.ClaimEpoch); err != nil {
			return err
		}
	}
	successor := *current
	// The WHOLE holder identity moves: liveness is (pid, pidStartedAt), so a
	// new pid beside the predecessor's start time would test as dead.
	successor.HolderMainId = ann.MainId
	successor.Pid = ann.Pid
	successor.PidStartedAt = ann.PidStartedAt
	successor.CommandHash = ann.CommandHash
	successor.OwnerLineage = announcementLineage(ann)
	successor.RenewedAt = nowStamp()
	successor.Revision = current.Revision + 1
	// claimEpoch preserved; takeovers not appended: this is not a seizure.
	expected := current.Revision
	if err := c.casSave(&successor, &expected); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "lease-renewed",
		fmt.Sprintf("same-lineage succession from %s", current.HolderMainId),
		map[string]string{"epoch": itoa(successor.ClaimEpoch)})
	return c.writeStamp(ann.MainId, current.ClaimEpoch)
}

// takeover seizes the lease from a dead holder of a foreign lineage: a new
// epoch, a recorded takeover, and a sweep of the predecessor's stale jobs.
func (c *claimer) takeover(current *Lease, ann *Announcement) error {
	claimed := nowStamp()
	epoch := current.ClaimEpoch + 1
	history := append([]Takeover{}, current.Takeovers...)
	history = append(history, Takeover{
		FromMainId: current.HolderMainId,
		ToMainId:   ann.MainId,
		ClaimEpoch: epoch,
		TakenAt:    claimed,
		Reason:     "holder-death",
	})
	lease := &Lease{
		HolderMainId: ann.MainId,
		OwnerLineage: announcementLineage(ann),
		Pid:          ann.Pid,
		PidStartedAt: ann.PidStartedAt,
		CommandHash:  ann.CommandHash,
		ClaimedAt:    claimed,
		RenewedAt:    claimed,
		Takeovers:    history,
		Revision:     current.Revision + 1,
		ClaimEpoch:   epoch,
	}
	expected := current.Revision
	if err := c.casSave(lease, &expected); err != nil {
		return err
	}
	c.emitter.Emit(c.root, "lease-takeover", fmt.Sprintf("predecessor %s", current.HolderMainId),
		map[string]string{"epoch": itoa(epoch), "reason": "holder-death", "predecessor": current.HolderMainId})
	if err := c.cleanupStaleJobs(epoch); err != nil {
		return err
	}
	if err := c.writeStamp(ann.MainId, epoch); err != nil {
		return err
	}
	c.inheritedProtocolTotal(current.HolderMainId)
	return nil
}

// casSave re-reads the lease under the held lock and refuses to write if it
// changed since the caller loaded it, then persists the new record.
func (c *claimer) casSave(lease *Lease, expected *int64) error {
	current, err := loadLease(c.root, false)
	if err != nil {
		return err
	}
	if err := verifyRevision(current, expected); err != nil {
		return err
	}
	return saveLease(c.root, lease)
}

// completeInterruptedSweep finishes a takeover's sweep that crashed after
// writing the lease but before stamping it. Returns the predecessor to report
// inherited protocol errors for, if any.
func (c *claimer) completeInterruptedSweep(current *Lease) (predecessor string, report bool, err error) {
	if c.stampComplete(current) {
		return "", false, nil
	}
	if err := c.cleanupStaleJobs(current.ClaimEpoch); err != nil {
		return "", false, err
	}
	if err := c.writeStamp(current.HolderMainId, current.ClaimEpoch); err != nil {
		return "", false, err
	}
	if n := len(current.Takeovers); n > 0 {
		last := current.Takeovers[n-1]
		if last.ToMainId == current.HolderMainId && last.ClaimEpoch == current.ClaimEpoch {
			return last.FromMainId, true, nil
		}
	}
	return "", false, nil
}

// stampComplete reports whether the reaped-after-claim stamp certifies the
// current holder's epoch sweep is done.
func (c *claimer) stampComplete(current *Lease) bool {
	stamp, err := readObject(c.paths.Stamp)
	if err != nil || stamp == nil {
		return false
	}
	holder, _ := stamp["holderMainId"].(string)
	epoch, _ := jsonInt(stamp["claimEpoch"])
	return holder == current.HolderMainId && epoch == current.ClaimEpoch
}

func (c *claimer) writeStamp(holder string, epoch int64) error {
	return atomicJSON(c.paths.Stamp, map[string]any{
		"holderMainId": holder,
		"claimEpoch":   epoch,
		"reapedAt":     nowStamp(),
	})
}

func jsonInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
