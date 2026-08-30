package dispatch

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

type CustodyDeathOutcome string

const (
	CustodyDeathProven   CustodyDeathOutcome = "PROVEN-DEAD"
	CustodyDeathAlive    CustodyDeathOutcome = "ALIVE"
	CustodyDeathDeferred CustodyDeathOutcome = "DEFERRED"
)

type CustodyDeathResult struct {
	Outcome CustodyDeathOutcome
	Reason  string
}

// CustodyDeathDependencies are the kernel observations behind one death
// proof. Reapers inject this decision as one predicate and retain no signal
// authority.
type CustodyDeathDependencies struct {
	Reader     identity.VerificationReader
	PIDs       func() ([]int64, error)
	PGID       func(pid int64) (int64, error)
	TaggedScan func(tag string) census.TaggedProcessCensus
	MatchesTag func(argv []string, tag string) bool
}

func custodyDeathDependenciesWithDefaults(dependencies CustodyDeathDependencies) CustodyDeathDependencies {
	if dependencies.Reader == nil {
		dependencies.Reader = identity.KernelProber{}
	}
	if dependencies.PIDs == nil {
		dependencies.PIDs = identity.AllPids
	}
	if dependencies.PGID == nil {
		dependencies.PGID = func(pid int64) (int64, error) {
			group, err := unix.Getpgid(int(pid))
			return int64(group), err
		}
	}
	if dependencies.TaggedScan == nil && dependencies.MatchesTag != nil {
		dependencies.TaggedScan = func(tag string) census.TaggedProcessCensus {
			return census.ScanTaggedProcesses(tag, census.TaggedScanDependencies{
				PIDs: dependencies.PIDs, PGID: dependencies.PGID,
				Reader: dependencies.Reader, MatchesTag: dependencies.MatchesTag,
			})
		}
	}
	return dependencies
}

// ProveCustodyDeath closes the whole recorded custody set. Exact primary and
// custody identities are checked first, then every current member of the
// primary group. A readable foreign member is ignored only when no live
// pre-fork marker can still make it ours; every unreadable in-group member
// remains a deferral.
func ProveCustodyDeath(root string, record map[string]any, dependencies CustodyDeathDependencies) CustodyDeathResult {
	dependencies = custodyDeathDependenciesWithDefaults(dependencies)
	if dependencies.MatchesTag == nil || dependencies.TaggedScan == nil {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "tag-position-proof-unavailable"}
	}
	tag := asString(record["instanceTag"])
	if tag == "" {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "instance-tag-missing"}
	}
	primary, ok := identityRefFromObject(record)
	if !ok || !primary.NativeExact() {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "primary-identity-unprovable"}
	}
	recorded := []identity.Ref{primary}
	if items, ok := record["custodyProcesses"].([]any); ok {
		for _, item := range items {
			entry, valid := item.(map[string]any)
			if !valid {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "custody-entry-malformed"}
			}
			ref, valid := identityRefFromObject(entry)
			if !valid || !ref.NativeExact() {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "custody-identity-unprovable"}
			}
			recorded = append(recorded, ref)
		}
	}
	for index, ref := range recorded {
		switch recordedRefLiveness(dependencies.Reader, ref) {
		case identity.Alive:
			reason := "primary-custodian-alive"
			if index > 0 {
				reason = "custody-process-alive"
			}
			return CustodyDeathResult{Outcome: CustodyDeathAlive, Reason: reason}
		case identity.Unknown:
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "recorded-identity-unreadable"}
		}
	}

	marker, err := readPreforkMarker(root, record)
	if err != nil {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-marker-unreadable"}
	}
	primaryPGID, hasPrimaryPGID := numInt(record["pgid"])
	if !hasPrimaryPGID || primaryPGID < 2 {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "primary-group-unprovable"}
	}
	if marker != nil {
		if marker.intendedPGID != primaryPGID {
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-group-mismatch"}
		}
		switch recordedRefLiveness(dependencies.Reader, marker.supervisor) {
		case identity.Alive:
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-supervisor-alive"}
		case identity.Unknown:
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-supervisor-unreadable"}
		case identity.Dead:
			tagged := dependencies.TaggedScan(tag)
			if len(tagged.Tagged) > 0 {
				return CustodyDeathResult{Outcome: CustodyDeathAlive, Reason: "prefork-tagged-survivor"}
			}
			if !tagged.Complete() {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-tag-absence-unprovable"}
			}
			clear, reason := preforkNamedGroupClear(marker, tag, dependencies)
			if !clear {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: reason}
			}
			if removeErr := os.Remove(marker.path); removeErr != nil && !os.IsNotExist(removeErr) {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-marker-expiry-failed"}
			}
			// This pass began with advance-written evidence standing, so it
			// never turns that same observation into a death verdict. The
			// identity-bounded expiry is durable now; the next pass observes no
			// marker and may close a recycled group without elapsed-time rules.
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-marker-expired"}
		}
	}

	pids, err := dependencies.PIDs()
	if err != nil {
		return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "process-table-unreadable"}
	}
	for _, pid := range pids {
		group, groupErr := dependencies.PGID(pid)
		if groupErr != nil {
			if errors.Is(groupErr, unix.ESRCH) {
				continue
			}
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "group-membership-unreadable"}
		}
		if group != primaryPGID {
			continue
		}
		verification := identity.VerifyProcess(dependencies.Reader, pid, func(argv []string) bool {
			return dependencies.MatchesTag(argv, tag)
		})
		switch verification.Outcome {
		case identity.VerificationVerified:
			return CustodyDeathResult{Outcome: CustodyDeathAlive, Reason: "tagged-group-member-alive"}
		case identity.VerificationIndeterminate:
			return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "in-group-member-unproven"}
		case identity.VerificationNotOurs:
			if marker != nil {
				return CustodyDeathResult{Outcome: CustodyDeathDeferred, Reason: "prefork-child-unproven"}
			}
		case identity.VerificationDead:
		}
	}

	// Once a primary group exists, unknown observations outside it do not
	// defeat death. A positive nonce-tagged survivor anywhere still does:
	// nonce uniqueness makes it ours and the next reconciliation can add it.
	if tagged := dependencies.TaggedScan(tag); len(tagged.Tagged) > 0 {
		return CustodyDeathResult{Outcome: CustodyDeathAlive, Reason: "nonce-tagged-survivor"}
	}
	return CustodyDeathResult{Outcome: CustodyDeathProven, Reason: "custody-closed"}
}

func preforkNamedGroupClear(marker *preforkMarker, tag string, dependencies CustodyDeathDependencies) (bool, string) {
	pids, err := dependencies.PIDs()
	if err != nil {
		return false, "prefork-process-table-unreadable"
	}
	for _, pid := range pids {
		group, groupErr := dependencies.PGID(pid)
		if groupErr != nil {
			if errors.Is(groupErr, unix.ESRCH) {
				continue
			}
			return false, "prefork-group-membership-unreadable"
		}
		if group != marker.intendedPGID {
			continue
		}
		verification := identity.VerifyProcess(dependencies.Reader, pid, func(argv []string) bool {
			return dependencies.MatchesTag(argv, tag)
		})
		switch verification.Outcome {
		case identity.VerificationDead:
			continue
		case identity.VerificationVerified:
			return false, "prefork-tagged-survivor"
		case identity.VerificationIndeterminate:
			return false, "prefork-child-unproven"
		case identity.VerificationNotOurs:
			if processDefinitelyPredatesSupervisor(verification.Identity, marker.supervisor) {
				continue
			}
			return false, "prefork-child-unproven"
		}
	}
	return true, ""
}

// A live member can be rejected as the marker's child only when its native
// start identity proves that it already existed before the supervisor. A
// readable but untagged process born later remains indistinguishable from the
// child in the fork-to-registration window and keeps the marker standing.
func processDefinitelyPredatesSupervisor(process identity.Exact, supervisor identity.Ref) bool {
	switch supervisor.Mode() {
	case identity.CompareLinuxTicksBootID:
		if process.BootID != supervisor.BootID {
			return process.BootID != ""
		}
		return process.StartTicks > 0 && process.StartTicks < supervisor.StartTicks
	case identity.CompareDarwinMicroseconds:
		return process.StartTicks == 0 && process.BootID == "" &&
			process.StartedAt.UnixMicro() < supervisor.StartedAtUnixMicro
	case identity.CompareLegacySeconds:
		return process.StartedAt.Unix() < supervisor.StartedAtSec
	default:
		return false
	}
}

func recordedRefLiveness(reader identity.StartReader, ref identity.Ref) identity.Liveness {
	exact, state, err := reader.ReadStart(ref.Pid)
	if err != nil || state == identity.Unknown {
		return identity.Unknown
	}
	if state == identity.Dead {
		return identity.Dead
	}
	if !identity.SameIdentity(exact, ref) {
		return identity.Dead
	}
	return identity.Alive
}

// CustodyGroupTargets returns every process group the kill-capable path must
// wind down. New custody entries persist their observed group; older entries
// may resolve it while their exact process is still live.
func CustodyGroupTargets(record map[string]any, groupID func(pid int64) (int64, error)) ([]int64, error) {
	groups := map[int64]bool{}
	if primary, ok := numInt(record["pgid"]); ok && primary >= 2 {
		groups[primary] = true
	} else {
		return nil, fmt.Errorf("dispatch: record has no primary process group")
	}
	items, _ := record["custodyProcesses"].([]any)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("dispatch: custody entry is malformed")
		}
		group, hasGroup := numInt(entry["pgid"])
		if !hasGroup || group < 2 {
			pid, hasPID := numInt(entry["pid"])
			if !hasPID || groupID == nil {
				return nil, fmt.Errorf("dispatch: custody entry has no process group")
			}
			var err error
			group, err = groupID(pid)
			if err != nil {
				if errors.Is(err, unix.ESRCH) {
					continue
				}
				return nil, fmt.Errorf("dispatch: custody process %d group is unavailable: %w", pid, err)
			}
		}
		if group >= 2 {
			groups[group] = true
		}
	}
	result := make([]int64, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}
