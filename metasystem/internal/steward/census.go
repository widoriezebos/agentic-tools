package steward

// The census-backed worker set: enrolled sessions, delegate jobs,
// and runner-owned processes come from the same census the
// supervision watcher publishes. Any UNTRACKED live process, any
// census error, and any incomplete scan prevents a death proof —
// unknown dominates dead.

import (
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// RuntimeWorkerCensus scans the live process table in production. Tests may
// provide the same guarded process fixture used by the supervision watcher;
// census.RunFixtureCensus refuses it unless the repository uses fake runtime.
type RuntimeWorkerCensus struct {
	MetasystemRoot string
	ProcessFile    string
}

// stewardCensusInterval is the telemetry interval stamped into the
// verdict — the steward's own tick cadence.
const stewardCensusInterval = 600

func (c RuntimeWorkerCensus) Workers(repoRoot string) (Workers, error) {
	fingerprint, err := census.Fingerprint(c.MetasystemRoot, repoRoot)
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	var verdict census.Verdict
	if c.ProcessFile != "" {
		verdict, err = census.RunFixtureCensus(c.MetasystemRoot, repoRoot, c.ProcessFile, fingerprint, stewardCensusInterval, time.Now())
	} else {
		verdict, err = census.RunProductionCensus(c.MetasystemRoot, repoRoot, fingerprint, stewardCensusInterval, time.Now())
	}
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	w := workersFromVerdict(verdict)
	// The runtime census sees runtime-shaped processes; runners,
	// monitored runs, and gates keep records it never reads.
	extraLive, extraUnprovable := supplementWorkers(repoRoot)
	w.Live += extraLive
	w.Unprovable += extraUnprovable
	return w, nil
}

// workersFromVerdict maps a census verdict into the steward's worker
// summary. Owned live processes (custody or announced) count as
// live; untracked ones block a death proof; a failed census is an
// incomplete scan.
func workersFromVerdict(v census.Verdict) Workers {
	w := Workers{CensusComplete: v.Verdict == "SUCCESS"}
	// A diagnostic the census could not fully account for (an
	// unresolved cwd, an unreadable run record, an unknown leader)
	// blocks a death proof. RACED-EXIT alone is benign: the process
	// PROVABLY exited during the scan, which is evidence of death,
	// not doubt about it.
	for _, d := range v.Diagnostics {
		if !strings.HasPrefix(d, "RACED-EXIT") {
			w.Unprovable++
		}
	}
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
