package fleet

// The launch records on the page, and the one event a changed record causes.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestThePageCarriesTheLaunchesItWasGiven(t *testing.T) {
	t.Parallel()
	records := []launch.Record{
		{Launch: "01M3BQAVYXE2AT6F0JG9YB64PG", Machine: "m1f", Outcome: launch.OutcomeRunning},
	}
	page := Compose(Inputs{This: "m1u", Launches: records}, time.Now().UTC())
	testutil.Expect(t, "the launches travel", len(page.Launches), 1)
	testutil.Expect(t, "the launch is the one it was given", page.Launches[0].Machine, "m1f")
}

// A page with no launches carries an empty list rather than a null, so the
// browser reads one shape.
func TestAPageWithNoLaunchesCarriesAnEmptyList(t *testing.T) {
	t.Parallel()
	page := Compose(Inputs{This: "m1u"}, time.Now().UTC())
	testutil.Expect(t, "never nil", page.Launches != nil, true)
	testutil.Expect(t, "and empty", len(page.Launches), 0)
}

// The fleet tool's reading prints the two launches a human can still act on,
// after the machines, with the owner's words under a failure. A launch that
// finished is the machine row above it and is not printed twice.
func TestTheReadingNamesTheStepALaunchStoppedAt(t *testing.T) {
	t.Parallel()
	page := Compose(Inputs{This: "m1u", Launches: []launch.Record{
		{
			Machine: "m1f", Destination: "/w/agentic-tools-m1f", Outcome: launch.OutcomeRunning,
			Steps: []launch.Step{{Step: launch.StepEngine, Outcome: launch.StepDone}},
		},
		{
			Machine: "m1g", Destination: "/w/agentic-tools-m1g", Outcome: launch.OutcomeFailed,
			Steps: []launch.Step{{Step: launch.StepEngine, Outcome: launch.StepFailed, Words: "go-build: refused: the fence holds"}},
		},
		{Machine: "m1h", Destination: "/w/agentic-tools-m1h", Outcome: launch.OutcomeDone},
	}}, time.Now().UTC())
	lines := page.Lines(time.Now().UTC())
	launched := []string{}
	for _, line := range lines {
		if len(line) > 10 && line[:10] == "- Launch: " {
			launched = append(launched, line)
		}
	}
	testutil.Require(t, "one line per launch worth acting on", len(launched), 2)
	testutil.Expect(t, "the running one says where it is",
		launched[0], "- Launch: m1f; running; into /w/agentic-tools-m1f; at engine")
	testutil.Expect(t, "the failed one carries the owner's words",
		launched[1], "- Launch: m1g; failed; into /w/agentic-tools-m1g; stopped at engine: go-build: refused: the fence holds")
}

func TestAChangedRecordAnnouncesAndAnUnchangedOneDoesNot(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	announced := 0
	watch := &LaunchWatch{
		Fingerprint: func() string { return FingerprintOf(directory) },
		Announce:    func() { announced++ },
	}
	// The first reading announces nothing: a server that starts with a launch
	// already on disk has changed nothing.
	testutil.Expect(t, "the first reading", watch.Consider(), false)
	testutil.Expect(t, "announced", announced, 0)
	testutil.Expect(t, "a second reading of the same directory", watch.Consider(), false)

	if err := os.WriteFile(filepath.Join(directory, "01M3BQAVYXE2AT6F0JG9YB64PG.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Expect(t, "a record appeared", watch.Consider(), true)
	testutil.Expect(t, "announced once", announced, 1)
	testutil.Expect(t, "and nothing moved since", watch.Consider(), false)

	if err := os.WriteFile(filepath.Join(directory, "01M3BQAVYXE2AT6F0JG9YB64PG.json"), []byte(`{"outcome":"done"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Expect(t, "the record was rewritten", watch.Consider(), true)
	testutil.Expect(t, "announced again", announced, 2)
}

// A directory that is not there reads as empty, which is what a host with no
// launches reads as: neither is a change to announce.
func TestAHostWithNoLaunchesFingerprintsAsEmpty(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "absent", FingerprintOf(filepath.Join(t.TempDir(), "nothing")), "")
	testutil.Expect(t, "empty", FingerprintOf(t.TempDir()), "")
}
