package partner

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The Fleet capture: the block is what the PAGE displayed, said as such, so
// "why is m1c unreachable" is answered from the rows the human was looking at
// rather than from a reading taken a minute later.

func fleetCapture() Page {
	return Page{
		Section: "Fleet", Path: "/fleet",
		Fleet: &FleetCapture{
			Source: "the interface", FetchedAt: "14:36",
			NeedsYou: []string{"tests-parallel-and-deterministic is held by m1c, unreachable since 09:40"},
			Machines: []FleetMachine{
				{Machine: "m1u", Standing: "reachable", Seen: "30 sec ago"},
				{Machine: "m1c", Standing: "unreachable", Seen: "6 h ago",
					Flag: "held by m1c, unreachable since 09:40", Holds: []string{"tests-parallel-and-deterministic"}},
			},
			Total: 2,
		},
	}
}

func TestTheFleetPageCarriesWhatItDisplayedAndSaysSo(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), fleetCapture(), "Wido", composedAt)

	testutil.Expect(t, "where the human is", strings.Contains(composed, "- Section: Fleet"), true)
	testutil.Expect(t, "the block is the page's own reading",
		strings.Contains(composed, "- The fleet as this page displayed it, from presence the interface, fetched 14:36:"), true)
	testutil.Expect(t, "the needs-you line travels",
		strings.Contains(composed,
			"  - needs a human: tests-parallel-and-deterministic is held by m1c, unreachable since 09:40"), true)
	testutil.Expect(t, "every row travels with its standing and what it was seen at",
		strings.Contains(composed, "  - m1c: unreachable, seen 6 h ago, holds tests-parallel-and-deterministic"), true)
	testutil.Expect(t, "with the flag beside it",
		strings.Contains(composed, "(held by m1c, unreachable since 09:40)"), true)
	testutil.Expect(t, "and the board's own block is not what a Fleet page gets",
		strings.Contains(composed, "Lanes on screen"), false)
}

// A capture that carried only some of a large fleet says how many it carried,
// so a Partner reading it knows to call the fleet tool for the rest.
func TestABoundedFleetCaptureSaysHowMuchOfItTravelled(t *testing.T) {
	t.Parallel()
	capture := fleetCapture()
	capture.Fleet.Total = 40
	composed := Compose(readingFacts(), capture, "Wido", composedAt)

	testutil.Expect(t, "the count and the whole are both named",
		strings.Contains(composed, "2 of 40 machines travelled with this question; the fleet tool reads the rest."), true)
}

// A page with no fleet in its capture is not a Fleet page, and falls back to
// the block it always had.
func TestAPageWithNoFleetInItsCaptureIsNotAFleetPage(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Fleet", Path: "/fleet"}, "Wido", composedAt)

	testutil.Expect(t, "nothing claims to be a displayed fleet",
		strings.Contains(composed, "The fleet as this page displayed it"), false)
}
