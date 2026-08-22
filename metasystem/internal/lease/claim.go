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
// reconciled when it is the same process re-announcing), a
// dead holder of the same lineage (succession, epoch preserved), and a dead
// holder of a foreign lineage (takeover, epoch bumped and predecessor swept).
func (c *claimer) claim(ann *Announcement) error {
	lock, err := acquireBounded(c.paths.Lock, "lease")
	if err != nil {
		return err
	}
	defer lock.release()

	// A supervision component announces so the census can see it, but it is
	// not a writer: it must never claim or disturb the checkout lease — a
	// detached owner that claimed would refuse its own parent as
	// OWNED-ELSEWHERE.
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

	case LiveRef(current.Pid, current.PidStartedAt, current.PidStartTicks, current.BootID, c.probe):
		if sameLeaseProcess(current, ann) {
			// The same live process re-announced under a new mainId
			// (a --shutdown then re-arm). It still owns its own checkout, so
			// reconcile the lease to the new identity instead of stranding the
			// session as OWNED-ELSEWHERE against its own former mainId.
			return c.reconcileSameProcess(current, ann)
		}
		// THE ONE EXCEPTION to "a live holder never yields":
		// a mission's own HOST TURN succeeds the mission's LIVE
		// RUNNER. Unattended, the runner is the main that armed the
		// checkout and holds the lease for the mission's whole life;
		// without this edge every host turn's dispatch refuses
		// OWNED-ELSEWHERE and the mission can never spend a job. The
		// edge is fenced three ways: same mission lineage, the holder's
		// own announcement carries the runner tag, and the claimant is
		// NOT itself a runner. A foreign main — different lineage —
		// stays refused, and the runner re-announces at turn
		// conclusion to succeed the then-dead host via the ordinary
		// dead-holder rule below; the epoch is PRESERVED across every
		// succession (it is not a seizure), which is what keeps
		// inherited delegates swept correctly.
		if leaseLineage(current) == announcementLineage(ann) &&
			announcementLineage(ann) != "" &&
			holderIsMissionRunner(c.root, current) &&
			descendsFrom(ann.Pid, current.Pid) {
			// Kernel ancestry is the whole fence: the mission's
			// host turn AND the runner's own detached loop are both real
			// children of the runner-side holder; a twin runner or an
			// impostor session, whatever its tags claim, is not.
			return c.succeed(current, ann)
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
		HolderMainId:  ann.MainId,
		OwnerLineage:  announcementLineage(ann),
		Pid:           ann.Pid,
		PidStartedAt:  ann.PidStartedAt,
		PidStartTicks: ann.PidStartTicks,
		BootID:        ann.BootID,
		CommandHash:   ann.CommandHash,
		ClaimedAt:     claimed,
		RenewedAt:     claimed,
		Takeovers:     []Takeover{},
		Revision:      1,
		ClaimEpoch:    1,
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
	renewed.OwnerLineage = announcementLineage(ann) // the lease's lineage follows the announcement on every renewal
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

// reconcileSameProcess handles the same live process re-announcing
// under a new mainId: move the holder identity to the new mainId, preserve the
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
	successor.PidStartTicks = ann.PidStartTicks
	successor.BootID = ann.BootID
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
		HolderMainId:  ann.MainId,
		OwnerLineage:  announcementLineage(ann),
		Pid:           ann.Pid,
		PidStartedAt:  ann.PidStartedAt,
		PidStartTicks: ann.PidStartTicks,
		BootID:        ann.BootID,
		CommandHash:   ann.CommandHash,
		ClaimedAt:     claimed,
		RenewedAt:     claimed,
		Takeovers:     history,
		Revision:      current.Revision + 1,
		ClaimEpoch:    epoch,
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

// sameLeaseProcess decides whether the lease holder and an announcement are
// one process: the clock-step-immune pair when both carry it
// (same-process reconciliation must not fail on a drifted second the moment
// LiveRef has proven the holder live), else the legacy seconds rule.
func sameLeaseProcess(current *Lease, ann *Announcement) bool {
	if current.Pid != ann.Pid {
		return false
	}
	if current.PidStartTicks > 0 && current.BootID != "" && ann.PidStartTicks > 0 && ann.BootID != "" {
		return current.PidStartTicks == ann.PidStartTicks && current.BootID == ann.BootID
	}
	return current.PidStartedAt == ann.PidStartedAt
}

// holderIsMissionRunner reports whether the lease holder's own announcement
// carries the mission-runner tag AND names the same kernel identity the
// lease holds — a structurally valid announcement file that merely reuses
// the holder's mainId proves nothing. Together with the
// kernel-ancestry check at the call site, the succession edge
// cannot be reached by forgeable declarations: lineage and tags are text,
// but parentage and the holder's identity pair are the kernel's.
func holderIsMissionRunner(root string, current *Lease) bool {
	records, err := readAnnouncements(root, false)
	if err != nil {
		return false
	}
	for _, rec := range records {
		if rec.Ann.MainId != current.HolderMainId {
			continue
		}
		if rec.Ann.InstanceTag != "mission-runner.sh" || rec.Ann.Pid != current.Pid {
			return false
		}
		if rec.Ann.PidStartTicks > 0 && rec.Ann.BootID != "" &&
			current.PidStartTicks > 0 && current.BootID != "" {
			return rec.Ann.PidStartTicks == current.PidStartTicks && rec.Ann.BootID == current.BootID
		}
		return rec.Ann.PidStartedAt == current.PidStartedAt
	}
	return false
}

// descendsFrom reports whether pid's kernel ancestry reaches ancestor — the
// unforgeable half of the succession edge: a mission's host turn is spawned
// BY the runner, so its parent chain includes the runner's pid, and no
// same-user impostor can fake its own parentage. Bounded walk; a broken
// chain answers false, and false never yields a live holder's lease.
func descendsFrom(pid, ancestor int64) bool {
	current := pid
	for depth := 0; depth < 64; depth++ {
		parent, ok := ParentPid(current)
		if !ok {
			return false
		}
		if parent == ancestor {
			return true
		}
		current = parent
	}
	return false
}
