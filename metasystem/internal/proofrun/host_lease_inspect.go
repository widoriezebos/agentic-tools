package proofrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
)

// Heavy lease states the steward reports.
const (
	HostLeaseLive      = "live"
	HostLeaseDead      = "reclaimable-dead"
	HostLeaseUnknown   = "unknown"
	HostLeaseReclaimed = "reclaimed"
)

// HostLeaseReport is one dirty heavy admission lease as the steward sees it.
// Since is the marker's last write, which is when it was marked dirty.
type HostLeaseReport struct {
	Lease  string
	Owner  ProcessIdentity
	Since  time.Time
	State  string
	Reason string
	Remedy string
}

// InspectHostLeases reports every dirty heavy lease on this host. When
// admission.lock is free it takes it without waiting and hands each provably
// dead, settled lease to the same reclaim the admission path uses; when a
// proof holds the lock it only classifies, and the admission pass reclaims.
func InspectHostLeases(controlRoot string) ([]HostLeaseReport, error) {
	// A fake-runtime checkout never inspects or reclaims the host's real
	// leases; it sees only an explicitly selected test namespace.
	if os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR") == "" && fixtureauth.FixtureModeRoot(controlRoot) {
		return nil, nil
	}
	directory, err := hostAdmissionDirectory()
	if err != nil {
		return nil, err
	}
	reclaimer := leaseReclaimerFromContext(context.Background(), controlRoot)
	reclaimer.report = nil
	return inspectHostLeasesIn(directory, reclaimer)
}

func inspectHostLeasesIn(directory string, reclaimer *leaseReclaimer) ([]HostLeaseReport, error) {
	guard, locked, err := tryHostFile(filepath.Join(directory, "admission.lock"))
	if err != nil {
		return nil, err
	}
	if locked {
		defer func() { _ = releaseHostProbe(guard) }()
	}
	paths, err := filepath.Glob(filepath.Join(directory, "lease-heavy-*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var reports []HostLeaseReport
	for _, path := range paths {
		report, include, err := inspectHostLease(directory, path, locked, reclaimer)
		if err != nil {
			return reports, err
		}
		if include {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func inspectHostLease(directory, path string, locked bool, reclaimer *leaseReclaimer) (HostLeaseReport, bool, error) {
	report := HostLeaseReport{Lease: filepath.Base(path)}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return report, false, nil
	}
	if err != nil {
		return report, false, err
	}
	report.Since = info.ModTime().UTC()
	reader, err := os.Open(path)
	if os.IsNotExist(err) {
		return report, false, nil
	}
	if err != nil {
		return report, false, err
	}
	record, _, readErr := readHostLeaseRecord(reader)
	_ = reader.Close()
	if readErr != nil {
		report.State, report.Reason = HostLeaseUnknown, readErr.Error()
		report.Remedy = fmt.Sprintf("no metasystem verb settles an unreadable lease; inspect %s by hand", path)
		return report, true, nil
	}
	report.Owner = record.Owner
	if record.Cleared {
		return report, false, nil
	}
	file, acquired, err := tryHostFile(path)
	if err != nil {
		return report, false, err
	}
	if !acquired {
		report.State, report.Reason = HostLeaseLive, "a live owner, custodian or worker holds the lease"
		return report, true, nil
	}
	defer func() { _ = releaseHostProbe(file) }()
	record, _, err = readHostLeaseRecord(file)
	if err != nil {
		return report, false, err
	}
	if record.Cleared {
		return report, false, nil
	}
	verdict := reclaimer.judge(path, record)
	switch {
	case verdict.live:
		report.State, report.Reason = HostLeaseLive, fmt.Sprintf("owner pid %d is alive", record.Owner.Pid)
	case verdict.unknown != "":
		report.State, report.Reason, report.Remedy = HostLeaseUnknown, verdict.unknown, verdict.remedy
	case !locked:
		report.State, report.Reason = HostLeaseDead, verdict.ownerDead+"; admission.lock is busy, the next admission pass reclaims it"
	default:
		if err := reclaimer.commit(directory, path, file, record, verdict); err != nil {
			return report, false, err
		}
		report.State, report.Reason = HostLeaseReclaimed, verdict.ownerDead
	}
	return report, true, nil
}
