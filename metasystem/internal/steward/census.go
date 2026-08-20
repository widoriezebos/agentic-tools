package steward

// The census-backed worker set: enrolled sessions, delegate jobs,
// and runner-owned processes come from the same census the
// supervision watcher publishes. Any UNTRACKED live process, any
// census error, and any incomplete scan prevents a death proof —
// unknown dominates dead.

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// LiveWorkerCensus scans the live process table.
type LiveWorkerCensus struct {
	MetasystemRoot string
}

// stewardCensusInterval is the telemetry interval stamped into the
// verdict — the steward's own tick cadence.
const stewardCensusInterval = 600

func (c LiveWorkerCensus) Workers(repoRoot string) (Workers, error) {
	fingerprint, err := census.Fingerprint(c.MetasystemRoot, repoRoot)
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	verdict, err := census.RunProductionCensus(c.MetasystemRoot, repoRoot, fingerprint, stewardCensusInterval, time.Now())
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	return workersFromVerdict(verdict), nil
}

// workersFromVerdict maps a census verdict into the steward's worker
// summary. Owned live processes (custody or announced) count as
// live; untracked ones block a death proof; a failed census is an
// incomplete scan.
func workersFromVerdict(v census.Verdict) Workers {
	w := Workers{CensusComplete: v.Verdict == "SUCCESS"}
	for _, item := range v.Inventory {
		switch item.Class {
		case "CUSTODY", "ANNOUNCED":
			w.Live++
		case "UNTRACKED":
			w.Untracked++
		default:
			w.Unprovable++
		}
	}
	return w
}
