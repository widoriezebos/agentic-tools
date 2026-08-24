package steward

// The provider-outage posture (provider-outage-posture, Wido
// 2026-08-24): a standing outage mark pauses the patience clocks and
// holds revival, because the provider's weather is nobody's failure —
// while progress, visibility, and the mark's own horizon keep every
// pause honest.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func markOutage(t *testing.T, root string) {
	t.Helper()
	if _, err := outage.Record(root, "overloaded", "API Error: 529", "test", time.Now()); err != nil {
		t.Fatal(err)
	}
}

// A standing outage freezes TicksSinceAdvance; clearing the mark lets
// the clock age again from where it stopped. The narration says why.
func TestProviderOutagePausesTheAging(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	cfg := TickConfig{StaleTicks: 2}
	r := tickN(t, root, cfg, census, 2)
	if r.Evidence.TicksSinceAdvance != 1 {
		t.Fatalf("the second quiet tick ages to 1: %+v", r.Evidence)
	}
	markOutage(t, root)
	r = tickN(t, root, cfg, census, 3)
	if r.Evidence.TicksSinceAdvance != 1 {
		t.Fatalf("the clock must pause during a standing outage: %+v", r.Evidence)
	}
	if !r.ProviderOutage || r.Decision.Verdict != VerdictHealthy {
		t.Fatalf("a paused clock below the threshold stays healthy: %+v", r)
	}
	narration, err := os.ReadFile(NarrationPath(root))
	if err != nil || !strings.Contains(string(narration),
		"the model provider is overloaded; local work continues; the clocks are paused") {
		t.Fatalf("the narration must say the clocks are paused: %v\n%s", err, narration)
	}
	if err := outage.Clear(root); err != nil {
		t.Fatal(err)
	}
	r = tickN(t, root, cfg, census, 1)
	if r.Evidence.TicksSinceAdvance != 2 || r.ProviderOutage {
		t.Fatalf("a cleared outage resumes the aging where it stopped: %+v", r)
	}
	if r.Decision.Verdict != VerdictStalledIdle {
		t.Fatalf("the resumed clock reaches the threshold honestly: %+v", r.Decision)
	}
}

// The pause never eats progress: a commit during the outage resets the
// clock exactly as it would on a clear day.
func TestProgressStillResetsDuringOutage(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	cfg := TickConfig{StaleTicks: 5}
	tickN(t, root, cfg, census, 2)
	markOutage(t, root)
	cmd := exec.Command("git", "-C", root, "commit", "-q", "--allow-empty", "-m", "progress")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("progress commit: %v\n%s", err, out)
	}
	r := tickN(t, root, cfg, census, 1)
	if r.Evidence.TicksSinceAdvance != 0 {
		t.Fatalf("progress during an outage still resets: %+v", r.Evidence)
	}
}

// A provably dead worker during an outage is notified, never revived:
// a continuation spawned into the outage would burn a launch on a
// certain failure — and its dry-revival count with it.
func TestOutageHoldsRevival(t *testing.T) {
	dead := Snapshot{
		Work:           WorkOwned,
		Workers:        Workers{CensusComplete: true},
		StaleTicks:     5,
		MaxRevivals:    3,
		ProviderOutage: true,
	}
	d := Decide(dead)
	if d.Verdict != VerdictStalledDead || d.Action != ActNotify ||
		!strings.Contains(d.Reason, "holding revival until the provider recovers") {
		t.Fatalf("an outage holds revival with the reason on record: %+v", d)
	}
	dead.ProviderOutage = false
	if d := Decide(dead); d.Action != ActRevive {
		t.Fatalf("without the outage the same snapshot revives: %+v", d)
	}
	// The dry-revival cap still outranks the outage: an operator who is
	// already needed stays needed.
	dead.ProviderOutage = true
	dead.DryRevivals = 3
	if d := Decide(dead); !strings.Contains(d.Reason, "operator needed") {
		t.Fatalf("the exhausted-revivals message outranks the outage hold: %+v", d)
	}
}

// The live tick path honors the hold end to end: a dead worker with a
// standing outage decides notify, and the same world revives once the
// mark clears.
func TestTickHoldsRevivalDuringOutage(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	markOutage(t, root)
	r := tickN(t, root, TickConfig{StaleTicks: 3}, census, 1)
	if r.Decision.Verdict != VerdictStalledDead || r.Decision.Action != ActRevive {
		if r.Decision.Action != ActNotify ||
			!strings.Contains(r.Decision.Reason, "holding revival") {
			t.Fatalf("a dead worker during an outage is notified, not revived: %+v", r.Decision)
		}
	} else {
		t.Fatalf("the tick must not revive during a standing outage: %+v", r.Decision)
	}
	if err := outage.Clear(root); err != nil {
		t.Fatal(err)
	}
	r = tickN(t, root, TickConfig{StaleTicks: 3}, census, 1)
	if r.Decision.Action != ActRevive {
		t.Fatalf("clearing the mark releases the revival: %+v", r.Decision)
	}
}

// A LONG outage reaches the human: the standing-outage noticing fires
// once the mark's age crosses the threshold, rides the durable notify
// queue, and is not silenced by a notify-tick decision. A fresh
// outage stays quiet.
func TestLongOutageNoticingReachesTheHuman(t *testing.T) {
	fresh := TickResult{
		ProviderOutage: true,
		Outage: outage.Mark{
			ConsecutiveFailures: 2,
			Since:               time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
		},
	}
	if items := noticings(fresh, TickConfig{}); len(items) != 0 {
		t.Fatalf("a fresh outage must stay quiet: %v", items)
	}
	long := TickResult{
		ProviderOutage: true,
		Decision:       Decision{VerdictStalledDead, ActNotify, "holding revival"},
		Outage: outage.Mark{
			ConsecutiveFailures: 7,
			Since:               time.Now().Add(-15 * time.Minute).UTC().Format(time.RFC3339),
		},
	}
	items := noticings(long, TickConfig{})
	if len(items) != 1 || items[0].Key != "provider-outage-standing" {
		t.Fatalf("a long outage must produce its one noticing even on a notify tick: %v", items)
	}
	if !strings.Contains(items[0].Line, "overloaded for 15 minutes") ||
		!strings.Contains(items[0].Line, "7 failure(s)") {
		t.Fatalf("the noticing must carry the duration and the count: %q", items[0].Line)
	}
}

// The live tick composes the alert end to end: a backdated standing
// mark carries the noticing into the narration (the queue side rides
// ReachTheHuman's own pinned mechanics).
func TestTickCarriesTheLongOutageNoticing(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	if _, err := outage.Record(root, "overloaded", "API Error: 529", "test",
		time.Now().Add(-15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	tickN(t, root, TickConfig{StaleTicks: 5}, census, 1)
	narration, err := os.ReadFile(NarrationPath(root))
	if err != nil || !strings.Contains(string(narration), "provider has been overloaded for") {
		t.Fatalf("the narration must carry the long-outage noticing: %v\n%s", err, narration)
	}
}
