package main

// The landing lane card's Land now, at the seam (goal fleet-card-can-land-now):
// the server runs landing run once, with --json, against this checkout, and
// reads back the verb's one-result envelope; an engine without the verb says
// so in words and runs nothing. The verb here is a fake answering the
// documented envelope (intentResult), because landing run is not on main yet.

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

type fakeLandingRun struct {
	found  bool
	stdout string
	stderr string
	asked  [][]string
	names  []string
}

func (fake *fakeLandingRun) find(name string) (intentCommand, bool) {
	fake.names = append(fake.names, name)
	if !fake.found {
		return intentCommand{}, false
	}
	return intentCommand{object: "landing", action: "run", name: name}, true
}

func (fake *fakeLandingRun) run(command intentCommand, args []string, stdout, stderr io.Writer) {
	fake.asked = append(fake.asked, append([]string{command.name}, args...))
	_, _ = io.WriteString(stdout, fake.stdout)
	_, _ = io.WriteString(stderr, fake.stderr)
}

const landNowCheckout = "/w/agentic-tools-ui"

func TestLandNowRunsLandingRunOnceWithJSONAgainstThisCheckout(t *testing.T) {
	t.Parallel()
	fake := &fakeLandingRun{found: true, stdout: `{
  "schemaVersion": 1,
  "verb": "landing run",
  "targets": [],
  "outcome": "confirmed",
  "summary": "started the landing agent for batch b-20 (2 members)",
  "next": {"argv": ["metasystem", "landing", "status"], "reason": "follows it"}
}`}

	answered, err := landNowRunWith(landNowCheckout, fake.find, fake.run)

	if err != nil {
		t.Fatalf("land now = %v", err)
	}
	if !reflect.DeepEqual(fake.names, []string{landNowVerb}) || landNowVerb != "landing run" {
		t.Fatalf("the seam looked for %v, want the one verb landing run", fake.names)
	}
	if want := [][]string{{"landing run", "--json", "--repo", landNowCheckout}}; !reflect.DeepEqual(fake.asked, want) {
		t.Fatalf("the verb was run as %v, want %v", fake.asked, want)
	}
	want := httpd.LandNowAnswer{Outcome: "confirmed", Summary: "started the landing agent for batch b-20 (2 members)",
		Next: &httpd.LandNowNext{Argv: []string{"metasystem", "landing", "status"}, Reason: "follows it"}}
	if !reflect.DeepEqual(answered, want) {
		t.Fatalf("answered %+v, want %+v", answered, want)
	}
}

// The verb's refusal is its answer, with both lines, and its next command
// exactly as the verb built it: the --repo naming this checkout stays, so the
// command copied from the browser works from any directory (Sol LN-01).
func TestLandNowPassesTheVerbsRefusalThroughWithItsNextCommandWhole(t *testing.T) {
	t.Parallel()
	fake := &fakeLandingRun{found: true, stdout: fmt.Sprintf(`{"schemaVersion":1,"verb":"landing run","targets":[],
"outcome":"refused","summary":"the landing lane is stopped by wido; nothing was started",
"next":{"argv":["metasystem","landing","start","--repo",%q],"reason":"resumes it"}}`, landNowCheckout)}

	answered, err := landNowRunWith(landNowCheckout, fake.find, fake.run)

	if err != nil {
		t.Fatalf("land now = %v", err)
	}
	want := httpd.LandNowAnswer{Outcome: "refused", Summary: "the landing lane is stopped by wido; nothing was started",
		Next: &httpd.LandNowNext{Argv: []string{"metasystem", "landing", "start", "--repo", landNowCheckout}, Reason: "resumes it"}}
	if !reflect.DeepEqual(answered, want) {
		t.Fatalf("answered %+v, want %+v", answered, want)
	}
}

// A nothing-to-do answer carries its line 2 as next with an empty argv (m1e,
// every verb's --json): the seam passes it through as the reason alone.
func TestLandNowPassesANothingToDoReasonThrough(t *testing.T) {
	t.Parallel()
	fake := &fakeLandingRun{found: true, stdout: `{"schemaVersion":1,"verb":"landing run","targets":[],
"outcome":"unchanged","summary":"the landing lane at /w/landing has no queued work, so no landing agent was started",
"next":{"argv":[],"reason":"nothing to do; the lane is empty"}}`}

	answered, err := landNowRunWith(landNowCheckout, fake.find, fake.run)

	if err != nil {
		t.Fatalf("land now = %v", err)
	}
	want := httpd.LandNowAnswer{Outcome: "unchanged", Summary: "the landing lane at /w/landing has no queued work, so no landing agent was started",
		Next: &httpd.LandNowNext{Argv: []string{}, Reason: "nothing to do; the lane is empty"}}
	if !reflect.DeepEqual(answered, want) {
		t.Fatalf("answered %+v (next %+v), want %+v (next %+v)", answered, answered.Next, want, want.Next)
	}
}

// An engine built before landing run says so as the verb would, in two lines,
// and runs nothing.
func TestLandNowWithoutTheVerbSaysSoAndRunsNothing(t *testing.T) {
	t.Parallel()
	fake := &fakeLandingRun{}

	answered, err := landNowRunWith(landNowCheckout, fake.find, fake.run)

	if err != nil {
		t.Fatalf("land now = %v", err)
	}
	if len(fake.asked) != 0 {
		t.Fatalf("something ran: %v", fake.asked)
	}
	if answered.Outcome != "refused" || !strings.Contains(answered.Summary, "landing run") || answered.Next == nil || answered.Next.Reason == "" {
		t.Fatalf("answered %+v, want a refusal naming landing run with a line 2", answered)
	}
}

// A verb that printed nothing readable is a failure in its own words.
func TestLandNowWhoseVerbAnsweredNothingReadableFails(t *testing.T) {
	t.Parallel()
	fake := &fakeLandingRun{found: true, stdout: "", stderr: "the lane record cannot be read\n"}

	_, err := landNowRunWith(landNowCheckout, fake.find, fake.run)

	if err == nil || !strings.Contains(err.Error(), "the lane record cannot be read") {
		t.Fatalf("land now = %v, want a failure carrying the verb's words", err)
	}
}

// The seam against the real landing run (on main since 8b76c7cda), on the
// lane bed its own tests use — a registered lane whose keeper's supervisor is
// a stand-in, so no process runs: queued work starts one agent (confirmed,
// with landing status as line 2), a repeat is unchanged, and a stopped lane
// is refused with landing start as line 2.
func TestLandNowReadsTheRealLandingRun(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	bed.wake = []string{"queued"}
	run := func(command intentCommand, args []string, stdout, stderr io.Writer) {
		runIntentIn(command, args, stdout, stderr, bed.cwd, bed.owners())
	}
	press := func() httpd.LandNowAnswer {
		t.Helper()
		answered, err := landNowRunWith(bed.landingA, findIntentCommand, run)
		if err != nil {
			t.Fatalf("land now = %v", err)
		}
		return answered
	}

	started := press()
	launches := landingLaunches(t, store)
	if len(launches) != 1 || started.Outcome != "confirmed" || !strings.Contains(started.Summary, "started the landing agent "+launches[0].ID) ||
		started.Next == nil || !slices.Equal(started.Next.Argv[:3], []string{"metasystem", "landing", "status"}) || started.Next.Reason == "" {
		t.Fatalf("land now with queued work = %+v (next %+v), launches %d", started, started.Next, len(launches))
	}

	repeated := press()
	if repeated.Outcome != "unchanged" || !strings.Contains(repeated.Summary, "is already running") || len(landingLaunches(t, store)) != 1 ||
		repeated.Next == nil || len(repeated.Next.Argv) != 0 || !strings.HasPrefix(repeated.Next.Reason, "nothing to do") {
		t.Fatalf("land now again = %+v (next %+v); want unchanged with its nothing-to-do line 2, no second agent", repeated, repeated.Next)
	}

	if code, _, stderr := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 {
		t.Fatalf("stop = %d %q", code, stderr)
	}
	refused := press()
	if refused.Outcome != "refused" || !strings.Contains(refused.Summary, "stopped by Wido") ||
		refused.Next == nil || !slices.Equal(refused.Next.Argv[:3], []string{"metasystem", "landing", "start"}) {
		t.Fatalf("land now on a stopped lane = %+v (next %+v)", refused, refused.Next)
	}
}
