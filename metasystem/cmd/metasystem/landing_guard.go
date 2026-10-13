package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// guardLanding publishes an engine failure before returning the crashed act's refusal.
func guardLanding(install, verb string, args []string, stdout, stderr io.Writer, run func(io.Writer, io.Writer) int, home func() (string, error), effects ...plain.ProveSeams) (code int) {
	seams := plain.ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	now := time.Now
	if seams.Now != nil {
		now = seams.Now
	}
	defer func() {
		if value := recover(); value != nil {
			incident, err := plain.RecordEngineIncident(install, verb, args, now(), fmt.Sprint(value), string(debug.Stack()), seams)
			code = 2
			line := fmt.Sprintf("landing %s failed inside the engine: %s; incident %s records the stack", verb, oneLineLanding(fmt.Sprint(value)), incident.ID)
			if err != nil {
				line += "; the incident could not be written: " + oneLineLanding(err.Error())
			}
			fmt.Fprintln(stderr, line)
			return
		}
	}()
	if home == nil {
		home = batchowner.LandingLaneHome
	}
	if path, err := home(); err == nil {
		seams.BudgetHome = path
		if record, present, err := lane.Read(path); err == nil && present {
			install = record.Install
		}
	}
	return run(stdout, stderr)
}

func oneLineLanding(text string) string { return strings.Join(strings.Fields(text), " ") }
