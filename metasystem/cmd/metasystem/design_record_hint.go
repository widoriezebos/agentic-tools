package main

// The one line a seat is told at the moment a goal becomes work: whatever it
// designs for this goal is a record, and the gate says what a record is. It is
// said once, on standard error, after the act is confirmed — never on the JSON
// standard output a caller parses, and never after a refusal, which has its
// own words to say.

import (
	"fmt"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// designRecordHint is the whole of the hint, with the goal's own id in it and
// the gate's section named rather than described: one place says what a head
// carries and where a design lives, and this is not that place.
const designRecordHint = "A design for %s is a record: " +
	"see docs/design/design-obligation-gate.md, A design is a record.\n"

// hintDesignIsARecord prints that line for a confirmed mutation, and prints
// nothing for anything else. An unconfirmed outcome is a goal that was not
// opened or claimed, and a hint about designing for it would be advice about
// work that did not start.
func hintDesignIsARecord(stderr io.Writer, outcome goal.Outcome, id string) {
	if outcome != goal.OutcomeConfirmed || id == "" {
		return
	}
	fmt.Fprintf(stderr, designRecordHint, id)
}
