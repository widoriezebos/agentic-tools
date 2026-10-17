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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// RuntimeWorkerCensus scans the live process table in production. Tests may
// provide the same guarded process fixture used by the supervision watcher;
// census.RunFixtureCensus refuses it unless the repository uses fake runtime.
type RuntimeWorkerCensus struct {
	MetasystemRoot string
	ProcessFile    string
	Prober         identity.Prober
	Now            func() time.Time
}

// stewardCensusInterval is the telemetry interval stamped into the
// verdict — the steward's own tick cadence.
const stewardCensusInterval = 600

func (c RuntimeWorkerCensus) Workers(repoRoot string) (Workers, error) {
	fingerprint, err := census.Fingerprint(c.MetasystemRoot, repoRoot)
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	prober := c.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	timestamp := now()
	var verdict census.Verdict
	if c.ProcessFile != "" {
		verdict, err = census.RunFixtureCensusProbed(c.MetasystemRoot, repoRoot, c.ProcessFile, fingerprint, stewardCensusInterval, timestamp, prober)
	} else {
		verdict, err = census.RunProductionCensus(c.MetasystemRoot, repoRoot, fingerprint, stewardCensusInterval, timestamp)
	}
	if err != nil {
		return Workers{Unprovable: 1}, nil
	}
	w := workersFromVerdict(verdict)
	// The runtime census sees runtime-shaped processes; runners,
	// monitored runs, and gates keep records it never reads.
	extraLive, extraLiveSeatMains, extraUnprovable := supplementWorkers(repoRoot, prober)
	w.Live += extraLive
	w.LiveSeatMains += extraLiveSeatMains
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
	// blocks a death proof. HUMAN-TERMINAL records a repair; RACED-EXIT
	// is benign because the process
	// PROVABLY exited during the scan, which is evidence of death,
	// not doubt about it.
	for _, d := range v.Diagnostics {
		if !strings.HasPrefix(d, "RACED-EXIT") && !strings.HasPrefix(d, census.HumanTerminalNotice+" ") {
			w.Unprovable++
		}
	}
	for _, item := range v.Inventory {
		switch item.Class {
		case "CUSTODY":
			w.Live++
		case "ANNOUNCED":
			w.Live++
			w.LiveSeatMains++
		case "UNTRACKED":
			w.Untracked++
		default:
			w.Unprovable++
		}
	}
	return w
}
