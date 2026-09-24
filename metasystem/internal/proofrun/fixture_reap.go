package proofrun

import (
	"fmt"
	"io"
	"slices"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixtureAttemptOwnership struct {
	owner     identity.Ref
	attemptID string
	startedAt time.Time
}

func reapFixtureSurvivors(prober identity.Prober, processes []census.Process, signal identity.SignalFunc, ownership fixtureAttemptOwnership, verdict io.Writer) bool {
	survivors, err := census.ScanFixtureSurvivors(prober, processes, census.FixtureSurvivorSelection{})
	if err != nil {
		fmt.Fprintln(verdict, "suite launcher: fixture survivor scan failed:", err)
		return false
	}
	failed := false
	for _, survivor := range survivors {
		line := census.FixtureSurvivorLine(prober, survivor)
		if ownership.attemptID != "" && !fixtureSurvivorOwnedByAttempt(prober, survivor, ownership) {
			fmt.Fprintln(verdict, line+" foreign")
			continue
		}
		if survivor.Class == identity.FixtureSurvivorCertain {
			_ = identity.SignalExact(prober, survivor.Ref, syscall.SIGKILL, signal)
			if !survivor.Started.IsZero() && survivor.Started.Before(ownership.startedAt) {
				line += " stale"
			} else {
				failed = true
			}
		}
		fmt.Fprintln(verdict, line)
	}
	return failed
}

func fixtureSurvivorOwnedByAttempt(prober identity.Prober, survivor identity.FixtureSurvivor, ownership fixtureAttemptOwnership) bool {
	survivorOwner, survivorOwnerErr := identity.EncodeRef(survivor.Key.Owner)
	attemptOwner, attemptOwnerErr := identity.EncodeRef(ownership.owner)
	if survivorOwnerErr == nil && attemptOwnerErr == nil && survivorOwner == attemptOwner {
		return true
	}
	exact, state, err := prober.Probe(survivor.Ref.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, survivor.Ref) {
		return false
	}
	tag := identity.FixtureAttemptEnv + "=" + ownership.attemptID
	return slices.Contains(exact.Argv, tag) || slices.Contains(exact.Environ, tag)
}
