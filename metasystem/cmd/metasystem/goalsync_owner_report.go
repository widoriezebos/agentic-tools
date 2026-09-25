package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// These report seams let one goal owner serve both callers: a legacy command
// prints exactly as it always has, and a public intent command, which sets
// dependencies.report, receives the same outcome typed instead.

// complain is an owner's one-line refusal on standard error, or the typed
// failure a public command renders.
func (d syncRequestDependencies) complain(parts ...any) {
	if d.report == nil {
		fmt.Fprintln(os.Stderr, parts...)
		return
	}
	d.report.failure = errors.New(strings.TrimSuffix(fmt.Sprintln(parts...), "\n"))
}

func (d syncRequestDependencies) complainf(format string, args ...any) {
	if d.report == nil {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	d.report.failure = errors.New(strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

// parseSyncFlags is the shared goal flag parser with its refusal routed like
// every other owner refusal.
func (d syncRequestDependencies) parseSyncFlags(name string, args []string) (*syncFlags, bool) {
	f, err := parseSyncFlagValues(name, args)
	if err != nil {
		d.complain(err)
		return nil, false
	}
	return f, true
}

// outcomeBeforeRefusal is an owner's unconfirmed publication, printed before
// its refusal sentence or kept for the public result.
func (d syncRequestDependencies) outcomeBeforeRefusal(res goal.PublishResult) {
	if d.report == nil {
		printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail})
		return
	}
	d.report.result = &res
}

// publishGrant prints a power of attorney's publication with the entry id a
// seat names with --under, or keeps both for the public result.
func (d syncRequestDependencies) publishGrant(res goal.PublishResult, entry string) int {
	if d.report == nil {
		printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "entry": entry, "detail": res.Detail})
		if res.Outcome != goal.OutcomeConfirmed {
			return 1
		}
		return 0
	}
	d.report.entry = entry
	return d.publish(res, nil)
}
