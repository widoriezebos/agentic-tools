// Package laneprobe is the landing custody barrier's reads on this host:
// the kernel's process reads and the lane installation's proof leases as
// proofrun classifies them. It sits above proofrun, which binds the groups
// a proof launcher makes to the custody store below it.
package laneprobe

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Production is the host's reads for the lane whose installation is
// installation: the kernel prober, group membership by signal 0, and the
// lane's proof leases as proofrun classifies them (read only: nothing is
// reclaimed). proving adds the host proving lock, which the batch owner's
// proofs hold; a caller holding that lock itself (landing engine advance)
// leaves it out. An installation that is not known fails closed: the
// leases read as an error, so custody reads unknown.
func Production(home, installation string, proving bool) custody.Probes {
	probes := custody.Probes{Prober: identity.KernelProber{}, Group: custody.GroupMembers, Now: time.Now,
		Leases: func() ([]custody.Lease, error) { return laneLeases(installation, proofrun.ReadHostLeases) }}
	if proving {
		probes.Proving = func() (string, bool, error) { return lane.ProbeProving(home) }
	}
	return probes
}

// laneLeases are the proof leases taken for the lane's installation, and
// every lease whose record can't be read (it may be the lane's). A lease
// an older engine took names no installation: it is judged by the older
// rules, not the lane's.
func laneLeases(installation string, inspect func(string) ([]proofrun.HostLeaseReport, error)) ([]custody.Lease, error) {
	if installation == "" || !filepath.IsAbs(installation) {
		return nil, fmt.Errorf("the lane's installation is not known (%q), so which test-run leases are the lane's can't be read", installation)
	}
	reports, err := inspect(installation)
	if err != nil {
		return nil, err
	}
	conf := filepath.Join(installation, "metasystem.conf")
	var leases []custody.Lease
	for _, report := range reports {
		unreadable := report.Owner.Pid == 0 && report.State == proofrun.HostLeaseUnknown
		if !unreadable && filepath.Clean(report.Conf) != conf {
			continue
		}
		leases = append(leases, custody.Lease{Name: report.Lease, State: report.State, Reason: report.Reason})
	}
	return leases, nil
}
