package uitools

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
)

// The fleet tool: that it is registered everywhere a tool has to be, that its
// source names both readings, and that its result is bounded like every other.

// readFleet is one composed page with a silent holder in it.
func readFleet() fleet.Page {
	tip := readObservation()
	return fleet.Compose(fleet.Inputs{
		This: "m1u",
		Presence: seat.Copy{
			Records: map[string]seat.Record{
				// m1e is the fixture ledger's own holder, and it has gone quiet.
				"m1e": {
					PresenceSchema: seat.RecordSchema, Machine: "m1e", RepoIdentity: "r",
					Generation: 4, Engine: "3f9c1e2abc", ArmedLineage: seat.NoLease, TickSeconds: 600,
					TickAt: seat.FormatTime(readAt.Add(-6 * time.Hour)),
				},
				"m1u": {
					PresenceSchema: seat.RecordSchema, Machine: "m1u", RepoIdentity: "r",
					Generation: 4, Engine: "8a01d77abc", ArmedLineage: seat.NoLease, TickSeconds: 600,
					TickAt: seat.FormatTime(readAt.Add(-2 * time.Minute)),
				},
			},
			Malformed: map[string]string{},
		},
		Copy:        fleet.Copy{Source: fleet.SourceInterface, SucceededAt: seat.FormatTime(readAt.Add(-40 * time.Second))},
		Observation: tip,
		Board:       backlog.Project(tip.Tree, tip.Horizon, tip.Admission),
		Previous: map[string]seat.Observation{
			"m1e": {Standing: seat.Unreachable, Since: seat.FormatTime(readAt.Add(-5 * time.Hour))},
		},
		Window: seat.DefaultStaleMinutes * time.Minute,
	}, readAt)
}

func fleetReaders(t *testing.T) Readers {
	t.Helper()
	readers := fixture(t)
	readers.Fleet = func() (fleet.Page, error) { return readFleet(), nil }
	return readers
}

func TestTheFleetToolIsRegisteredEverywhereAToolHasToBe(t *testing.T) {
	t.Parallel()

	testutil.Expect(t, "the operation is named", Names(OpFleet), true)
	listed := false
	for _, operation := range Operations {
		if operation == OpFleet {
			listed = true
		}
	}
	testutil.Expect(t, "and listed among the operations", listed, true)
	published := map[string]Tool{}
	for _, tool := range Catalogue() {
		published[tool.Name] = tool
	}
	tool, offered := published[OpFleet]
	testutil.Require(t, "the catalogue publishes it", offered, true)
	testutil.Expect(t, "with a description a runtime can choose it by",
		strings.Contains(tool.Description, "standings"), true)
	testutil.Expect(t, "and the cursor every bounded tool takes",
		tool.InputSchema["properties"].(map[string]any)["cursor"] != nil, true)
	testutil.Expect(t, "the catalogue and the operations are the same list",
		len(Catalogue()), len(Operations))
}

func TestTheFleetToolStampsBothReadings(t *testing.T) {
	t.Parallel()
	result := fleetReaders(t).Answer(OpFleet, Args{})

	testutil.Expect(t, "the presence copy's provenance is named",
		strings.Contains(result.Source, "presence from the interface, fetched "), true)
	testutil.Expect(t, "and the tip the holders were read at",
		strings.Contains(result.Source, "claims from the accepted tip "+observedTip), true)
}

func TestTheFleetToolReadsThisSeatThenTheMachines(t *testing.T) {
	t.Parallel()
	result := fleetReaders(t).Answer(OpFleet, Args{})

	testutil.Expect(t, "nothing failed", result.Failed(), false)
	testutil.Expect(t, "every line is supplied", result.Supplied, result.Total)
	testutil.Expect(t, "this seat is read before the other machines",
		strings.Index(result.Body, "- This seat: m1u") < strings.Index(result.Body, "- m1e"), true)
	testutil.Expect(t, "and the silent holder's flag travels",
		strings.Contains(result.Body, "held by m1e, unreachable since "), true)
}

// The needs-you lines come first, because they are the one thing a human is
// being asked to do something about.
func TestTheFleetToolLeadsWithTheNeedsYouLines(t *testing.T) {
	t.Parallel()
	result := fleetReaders(t).Answer(OpFleet, Args{})

	testutil.Expect(t, "the first line names a goal that needs a human",
		strings.HasPrefix(result.Body, "- Needs you: running is held by m1e"), true)
	testutil.Expect(t, "and says what a human does about it",
		strings.Contains(result.Body, "a human steals or resumes it at a terminal"), true)
}

func TestTheFleetToolIsBoundedAndCarriesACursor(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	readers.Fleet = func() (fleet.Page, error) {
		page := readFleet()
		// A fleet no result could carry whole: the bound is the tool's, not
		// the fleet's, so a big enough one has to hand back a cursor.
		for at := 0; at < maxRows+50; at++ {
			page.Machines = append(page.Machines, fleet.Machine{
				Machine:  "m" + strings.Repeat("x", 40) + strconv.Itoa(at),
				Standing: "unknown", Reason: "no presence record", Holds: []fleet.Held{},
			})
		}
		return page, nil
	}

	first := readers.Answer(OpFleet, Args{})
	testutil.Expect(t, "not everything fits", first.Supplied < first.Total, true)
	testutil.Require(t, "so a cursor comes back", first.Cursor != "", true)
	testutil.Expect(t, "and the text says how to continue",
		strings.Contains(first.Text(), "More remains: call this tool again with cursor"), true)

	second := readers.Answer(OpFleet, Args{"cursor": first.Cursor})
	testutil.Expect(t, "the second page starts where the first stopped",
		second.Supplied > first.Supplied, true)
	testutil.Expect(t, "over the same whole", second.Total, first.Total)
}

func TestABuildWithNoFleetReaderSaysSoRatherThanAnsweringEmpty(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "no reader, no reading",
		fixture(t).Answer(OpFleet, Args{}).Problem, "this build cannot read the fleet")

	readers := fixture(t)
	readers.Fleet = func() (fleet.Page, error) {
		return fleet.Page{}, errors.New("the presence namespace could not be read")
	}
	testutil.Expect(t, "a reader that failed says why",
		readers.Answer(OpFleet, Args{}).Problem, "the presence namespace could not be read")
}
