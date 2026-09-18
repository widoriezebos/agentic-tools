package proofrun

import (
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func reapFixtureSurvivors(prober identity.Prober, processes []census.Process, signal identity.SignalFunc, launcherStart time.Time, verdict io.Writer) bool {
	survivors, err := census.ScanFixtureSurvivors(prober, processes, census.FixtureSurvivorSelection{})
	if err != nil {
		fmt.Fprintln(verdict, "suite launcher: fixture survivor scan failed:", err)
		return false
	}
	failed := false
	for _, survivor := range survivors {
		line := census.FixtureSurvivorLine(prober, survivor)
		if survivor.Class == identity.FixtureSurvivorCertain {
			_ = identity.SignalExact(prober, survivor.Ref, syscall.SIGKILL, signal)
			if !survivor.Started.IsZero() && survivor.Started.Before(launcherStart) {
				line += " stale"
			} else {
				failed = true
			}
		}
		fmt.Fprintln(verdict, line)
	}
	return failed
}
