package census

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// FixtureSurvivorSelection chooses a whole-table, key, or dead-owner scan.
type FixtureSurvivorSelection = identity.FixtureSurvivorSelection

type fixtureProcessTable map[int64]Process

func newFixtureProcessTable(processes []Process) fixtureProcessTable {
	table := make(fixtureProcessTable, len(processes))
	for _, process := range processes {
		table[process.Pid] = process
	}
	return table
}

func (table fixtureProcessTable) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	process, ok := table[pid]
	if !ok || !process.Alive {
		return identity.Exact{}, identity.Dead, nil
	}
	started := time.Time{}
	if process.StartedExactMicro > 0 {
		started = time.UnixMicro(process.StartedExactMicro)
	} else if process.Started > 0 {
		started = time.Unix(process.Started, 0)
	}
	exact := identity.Exact{
		Pid: process.Pid, StartedAt: started, StartTicks: process.StartTicks, BootID: process.BootID,
	}
	if !process.Unreadable {
		exact.Argv, exact.ArgvKnown = strings.Fields(process.Argv), true
		exact.Environ, exact.EnvironKnown = process.Environ, true
		exact.Exe, exact.ExeKnown = process.Exe, process.Exe != ""
	}
	return exact, identity.Alive, nil
}

func fixtureProcessScope(table fixtureProcessTable, pid int64) identity.FixtureProcessScope {
	process := table[pid]
	return identity.FixtureProcessScope{
		Pgid: process.PGID, Ppid: process.PPID, Signalable: process.Alive,
	}
}

// FixtureProcessProber returns a prober over a complete recorded process table.
func FixtureProcessProber(processes []Process) identity.Prober {
	return newFixtureProcessTable(processes)
}

// ScanFixtureSurvivors classifies only rows in processes, using prober for exact owner checks.
func ScanFixtureSurvivors(prober identity.Prober, processes []Process, selection FixtureSurvivorSelection) ([]identity.FixtureSurvivor, error) {
	table := newFixtureProcessTable(processes)
	pids := make([]int64, 0, len(processes))
	self := int64(os.Getpid())
	for _, process := range processes {
		if process.Pid == self {
			continue
		}
		pids = append(pids, process.Pid)
	}
	return identity.ScanFixtureSurvivors(pids, prober, func(pid int64) identity.FixtureProcessScope {
		return fixtureProcessScope(table, pid)
	}, selection)
}

// FixtureSurvivorLine renders one finding without exposing the process environment.
func FixtureSurvivorLine(prober identity.Prober, survivor identity.FixtureSurvivor) string {
	field := func(value int64) string {
		if value == 0 {
			return "?"
		}
		return strconv.FormatInt(value, 10)
	}
	since := "?"
	if !survivor.Started.IsZero() {
		since = survivor.Started.UTC().Format(time.RFC3339Nano)
	}
	owner, ownerState, key := "?", "?", "?"
	if survivor.Key.Owner.Pid != 0 {
		if encoded, err := identity.EncodeRef(survivor.Key.Owner); err == nil {
			owner = encoded
			ownerState = identity.AliveRef(prober, survivor.Key.Owner).String()
		}
		key = survivor.Key.Test + "/" + survivor.Key.Nonce
	}
	exe := "?"
	if survivor.ExeKnown {
		exe = survivor.Exe
	}
	argv := "?"
	if survivor.ArgvKnown {
		argv = strconv.Quote(strings.Join(survivor.Argv, " "))
	}
	carrier := "?"
	if survivor.Carrier != "" {
		carrier = string(survivor.Carrier)
	}
	return fmt.Sprintf("%s pid=%d pgid=%s ppid=%s since=%s owner=%s %s key=%s exe=%s argv=%s carrier=%s",
		survivor.Class, survivor.Ref.Pid, field(survivor.Pgid), field(survivor.Ppid), since,
		owner, ownerState, key, exe, argv, carrier)
}
