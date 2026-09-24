package main

// The seat family: what this checkout can see of the other seats. Today that
// is presence — one record per machine on one git ref per machine, published
// by each machine's steward tick and read here. Presence authorizes nothing:
// a silent holder is flagged, never displaced.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// seatFleetNow is the reader's clock; the fixture boundary injects it so the
// verb's output is testable without the wall.
var seatFleetNow = func() time.Time { return time.Now().UTC() }

// runSeatFleet prints one line per machine, this machine first.
func runSeatFleet(args []string) int {
	flags := flag.NewFlagSet("seat fleet", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	fetch := flags.Bool("fetch", false, "fetch presence into this read's own namespace instead of reading the tick's copy")
	asJSON := flags.Bool("json", false, "print the standings as JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "seat fleet: --root is required")
		return 2
	}
	report, err := seatFleetReport(*root, *fetch, seatFleetNow())
	if err != nil {
		fmt.Fprintf(os.Stderr, "seat fleet: %v\n", err)
		return 1
	}
	if *asJSON {
		encoded, err := report.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "seat fleet: %v\n", err)
			return 1
		}
		os.Stdout.Write(encoded)
		return 0
	}
	fmt.Print(report.Text())
	return 0
}

// seatFleetReport reads presence once and derives every standing from it.
func seatFleetReport(root string, fetch bool, now time.Time) (seat.Report, error) {
	transport, err := seat.NewGit(root)
	if err != nil {
		return seat.Report{}, err
	}
	report := seat.Report{}
	report.SetNow(now)
	window := seatPresenceWindow(root)
	report.WindowSeconds = int(window / time.Second)

	namespace, source := seat.TickNamespace, "the tick"
	if fetch {
		id, err := goal.NewOperationULID()
		if err != nil {
			return seat.Report{}, err
		}
		namespace = seat.FetchNamespacePrefix + "/" + id
		source = "this read"
		// A stale namespace left by an interrupted read is harmless and
		// collected by the next run; this one is deleted before the verb
		// exits whatever happens after the fetch.
		defer func() { _ = transport.DeleteNamespace(namespace) }()
		if err := transport.Fetch(namespace); err != nil {
			report.CopyProblem = err.Error()
		}
	}
	if transport.Local {
		source = "local refs"
	}
	report.CopySource = source

	fleetCopy, err := transport.Read(namespace)
	if err != nil {
		report.CopyProblem = err.Error()
		fleetCopy = seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}
	}

	machine, enrolled := seat.Machine(root)
	report.This, report.NoNickname = machine, !enrolled
	claims, unavailable := seat.Claims(root)
	report.ClaimsUnavailable = unavailable

	previous, _, err := seat.LoadStandings(root)
	if err != nil {
		previous = seat.StandingsState{Machines: map[string]seat.Observation{}}
	}
	report.CopyReadAt = previous.ReadAt
	if fetch {
		report.CopyReadAt = seat.FormatTime(now)
	}
	report.Machines = seat.Fleet(seat.FleetInput{
		This: machine, Copy: fleetCopy, Claims: claims, ClaimsUnavailable: unavailable,
		Previous: previous.Machines, Now: now, Window: window,
	})
	if state, readable, err := seat.LoadPublicationState(root); err == nil && readable {
		held := state
		report.Publication = &held
	}
	return report, nil
}

// seatPresenceWindow reads the reader's own stale window, the same key the
// steward's health role reads.
func seatPresenceWindow(root string) time.Duration {
	minutes, err := config.SeatPresenceStaleMinutes(filepath.Join(root, "metasystem.conf"))
	if err != nil || minutes == 0 {
		minutes = config.DefaultSeatPresenceStaleMinutes
	}
	return time.Duration(minutes) * time.Minute
}
