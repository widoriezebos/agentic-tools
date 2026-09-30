package custody

import (
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// ProductionProbes are the host's reads for the lane whose installation is
// installation: the kernel prober, group membership by signal 0, and the
// lane's proof leases as proofrun classifies them (read only: nothing is
// reclaimed). proving adds the host proving lock,
// which the batch owner's proofs hold; a caller holding that lock itself
// (landing engine advance) leaves it out.
func ProductionProbes(home, installation string, proving bool) Probes {
	probes := Probes{Prober: identity.KernelProber{}, Group: GroupMembers, Now: time.Now,
		Leases: func() ([]Lease, error) { return laneLeases(installation, proofrun.ReadHostLeases) }}
	if proving {
		probes.Proving = func() (string, bool, error) { return lane.ProbeProving(home) }
	}
	return probes
}

// laneLeases are the proof leases taken for the lane's installation, and
// every lease whose record can't be read (it may be the lane's). A lease
// an older engine took names no installation: it is judged by the older
// rules, not the lane's.
func laneLeases(installation string, inspect func(string) ([]proofrun.HostLeaseReport, error)) ([]Lease, error) {
	reports, err := inspect(installation)
	if err != nil {
		return nil, err
	}
	conf := filepath.Join(installation, "metasystem.conf")
	var leases []Lease
	for _, report := range reports {
		unreadable := report.Owner.Pid == 0 && report.State == proofrun.HostLeaseUnknown
		if !unreadable && filepath.Clean(report.Conf) != conf {
			continue
		}
		leases = append(leases, Lease{Name: report.Lease, State: report.State, Reason: report.Reason})
	}
	return leases, nil
}
