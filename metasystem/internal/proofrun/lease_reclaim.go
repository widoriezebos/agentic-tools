package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// reclaimedLeasesFile is the admission directory's append-only record of
// every dirty lease the admission path reclaimed. Its name must stay outside
// the lease-*, slot-* and resource-* patterns the scanners glob.
const reclaimedLeasesFile = "reclaimed-leases.jsonl"

// leaseRecensusPasses spaces the owner-scoped fixture census of a kept lease
// by admission passes, not by elapsed time: a kept lease is re-examined on
// every leaseRecensusPasses-th pass of one waiting acquisition, so a lease
// whose survivor later exits is still reclaimed while the waiter does not run
// a whole-table census every retry under admission.lock.
const leaseRecensusPasses = 50

// leaseReclaimer settles a dirty lease whose owner can no longer mark it
// clean. The caller holds admission.lock and the lease's own flock on the
// reloaded inode, so no owner, custodian or worker still holds the marker
// descriptor. That alone does not settle the lease: a custodian drains the
// owner's detached fixtures before it marks clean (resource_custody.go
// drainCustodyFixtures), and a killed custodian leaves that obligation open.
// The reclaimer therefore requires, in order: the owner exactly dead (a pid
// reused by another start time is dead), the owner's process group without a
// live member, and an owner-scoped fixture census with no survivor. Anything
// it cannot prove keeps the lease and reports one line once.
type leaseReclaimer struct {
	prober      identity.Prober
	groupLive   func(group int64, prober identity.Prober) (bool, error)
	survivors   func(prober identity.Prober, owner identity.Ref) ([]identity.FixtureSurvivor, error)
	report      func(line string)
	now         func() time.Time
	controlRoot string
	reported    map[string]bool
	skip        map[string]int
}

type leaseReclaimerKey struct{}

// withLeaseReclaimer lets a test replace the process, group and census
// seams of one acquisition without touching package state.
func withLeaseReclaimer(ctx context.Context, reclaimer *leaseReclaimer) context.Context {
	return context.WithValue(ctx, leaseReclaimerKey{}, reclaimer)
}

func leaseReclaimerFromContext(ctx context.Context, controlRoot string) *leaseReclaimer {
	if reclaimer, ok := ctx.Value(leaseReclaimerKey{}).(*leaseReclaimer); ok && reclaimer != nil {
		return reclaimer
	}
	return &leaseReclaimer{
		prober:      identity.KernelProber{},
		groupLive:   processGroupLive,
		survivors:   identity.FixtureSurvivorsOfDeadOwner,
		report:      func(line string) { fmt.Fprintln(os.Stderr, line) },
		now:         time.Now,
		controlRoot: controlRoot,
	}
}

// processGroupLive reports whether any process, the leader included, is a
// live, non-zombie member of group. Unknown membership is an error.
func processGroupLive(group int64, prober identity.Prober) (bool, error) {
	members, err := custodyGroupMembers(group)
	if err != nil {
		return false, err
	}
	// custodyGroupMembers omits the leader; a live leader is still a member.
	if leaderGroup, leaderErr := syscall.Getpgid(int(group)); leaderErr == nil && int64(leaderGroup) == group {
		members = append(members, group)
	} else if leaderErr != nil && !errors.Is(leaderErr, syscall.ESRCH) {
		return false, fmt.Errorf("group leader %d is uninspectable: %w", group, leaderErr)
	}
	for _, pid := range members {
		exact, state, probeErr := prober.Probe(pid)
		if probeErr != nil || state == identity.Unknown {
			return false, fmt.Errorf("group member %d is uninspectable: %v", pid, probeErr)
		}
		if state == identity.Alive && !exact.Zombie && !exact.Exiting {
			return true, nil
		}
	}
	return false, nil
}

type leaseReclaimRecord struct {
	At        string          `json:"at"`
	Lease     string          `json:"lease"`
	Owner     ProcessIdentity `json:"owner"`
	Class     string          `json:"class"`
	Slot      string          `json:"slot,omitempty"`
	Resources []string        `json:"resources"`
	OwnerDead string          `json:"ownerDead"`
	Group     string          `json:"group"`
	Fixtures  string          `json:"fixtures"`
	Flock     string          `json:"flock"`
	By        int             `json:"by"`
}

// settle decides one dirty lease. It returns true only after the reclaim
// record is durable and the marker is marked cleared and removed.
func (reclaimer *leaseReclaimer) settle(directory, path string, locked *os.File, record hostLeaseRecord) (bool, error) {
	name := filepath.Base(path)
	if reclaimer.skip[name] > 0 {
		reclaimer.skip[name]--
		return false, nil
	}
	owner := record.Owner.Ref()
	exact, state, probeErr := reclaimer.prober.Probe(owner.Pid)
	var ownerDead string
	switch {
	case probeErr != nil || state == identity.Unknown:
		reclaimer.keep(name, record, fmt.Sprintf("owner liveness is unknown (%v)", probeErr), reclaimer.handRemedy(record))
		return false, nil
	case state == identity.Dead:
		ownerDead = fmt.Sprintf("pid %d is not running", owner.Pid)
	default:
		comparison := identity.Compare(exact, owner)
		switch {
		case comparison.Mode == identity.CompareInvalid:
			reclaimer.keep(name, record, "owner identity is not comparable", reclaimer.handRemedy(record))
			return false, nil
		case !comparison.Matches:
			ownerDead = fmt.Sprintf("pid %d was reused by a process with another start time", owner.Pid)
		case exact.Zombie:
			ownerDead = fmt.Sprintf("pid %d is a zombie", owner.Pid)
		default:
			// A live owner keeps its lease however long it runs.
			return false, nil
		}
	}
	if record.Owner.Pgid <= 0 {
		reclaimer.keep(name, record, "owner process group is not recorded", reclaimer.handRemedy(record))
		return false, nil
	}
	live, groupErr := reclaimer.groupLive(record.Owner.Pgid, reclaimer.prober)
	if groupErr != nil {
		reclaimer.keep(name, record, fmt.Sprintf("owner process group %d membership is unknown (%v)", record.Owner.Pgid, groupErr), reclaimer.handRemedy(record))
		return false, nil
	}
	if live {
		reclaimer.keep(name, record, fmt.Sprintf("owner process group %d still has live members, settlement is unknown", record.Owner.Pgid), reclaimer.handRemedy(record))
		return false, nil
	}
	survivors, censusErr := reclaimer.survivors(reclaimer.prober, owner)
	if censusErr != nil {
		reclaimer.keep(name, record, fmt.Sprintf("the owner's fixture census failed (%v)", censusErr), reclaimer.fixtureRemedy(record))
		return false, nil
	}
	if len(survivors) != 0 {
		what := fmt.Sprintf("%d owner-tagged fixture(s) survive, first pid %d (%s)", len(survivors), survivors[0].Ref.Pid, survivors[0].Class)
		reclaimer.keep(name, record, what, reclaimer.fixtureRemedy(record))
		return false, nil
	}
	_, data, err := readHostLeaseRecord(locked)
	if err != nil {
		return false, err
	}
	entry := leaseReclaimRecord{
		At: reclaimer.now().UTC().Format(time.RFC3339Nano), Lease: name, Owner: record.Owner,
		Class: record.Class, Slot: record.Slot, Resources: append([]string{}, record.Resources...),
		OwnerDead: ownerDead, Group: fmt.Sprintf("process group %d has no live member", record.Owner.Pgid),
		Fixtures: "owner-scoped fixture census found no survivor", Flock: "lease flock acquired under admission.lock; record reloaded",
		By: os.Getpid(),
	}
	if err := appendLeaseReclaimRecord(directory, entry); err != nil {
		return false, fmt.Errorf("record reclaim of %s: %w", name, err)
	}
	if err := setHostLeaseCleared(locked, data, true); err != nil {
		return false, fmt.Errorf("clear reclaimed lease %s: %w", name, err)
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove reclaimed lease %s: %w", name, err)
	}
	return true, nil
}

func (reclaimer *leaseReclaimer) keep(name string, record hostLeaseRecord, unknown, remedy string) {
	if reclaimer.skip == nil {
		reclaimer.skip = map[string]int{}
	}
	reclaimer.skip[name] = leaseRecensusPasses - 1
	if reclaimer.reported == nil {
		reclaimer.reported = map[string]bool{}
	}
	if reclaimer.reported[name] || reclaimer.report == nil {
		return
	}
	reclaimer.reported[name] = true
	reclaimer.report(fmt.Sprintf("proof admission: keeping dirty lease %s (owner pid %d, group %d): %s; settle: %s",
		name, record.Owner.Pid, record.Owner.Pgid, unknown, remedy))
}

func (reclaimer *leaseReclaimer) fixtureRemedy(record hostLeaseRecord) string {
	ref, err := identity.EncodeRef(record.Owner.Ref())
	root := reclaimer.controlRoot
	if root == "" {
		root = "<checkout>"
	}
	if err != nil {
		return reclaimer.handRemedy(record)
	}
	return fmt.Sprintf("metasystem internal proc fixture-survivors --owner '%s' --root %s --reap, then the next admission pass reclaims it", ref, root)
}

func (reclaimer *leaseReclaimer) handRemedy(record hostLeaseRecord) string {
	return fmt.Sprintf("no metasystem verb settles this; list it with `ps -axo pid,pgid,command | awk '$1==%d || $2==%d'`, stop what belongs to the lease, and the next admission pass reclaims it",
		record.Owner.Pid, record.Owner.Pgid)
}

func appendLeaseReclaimRecord(directory string, entry leaseReclaimRecord) error {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(directory, reclaimedLeasesFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	return errors.Join(writeErr, file.Sync(), file.Close())
}
