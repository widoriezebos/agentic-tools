package pattern

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

const cadence = 600 * time.Second

// At the default 600 s steward cadence (no tick-seconds set, no max-gap
// set) the observed not-held time reaches batch-hours=2 after twelve
// counted intervals: the episode opens at that cycle and not one earlier,
// with one notification. A max-gap below the cadence counts nothing; a
// threshold compared with > instead of >= opens one cycle late.
func TestAgeReachesThresholdAtDefaultCadence(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("a")
	b.writeBatch(id, []batch.HistoryEntry{step(bedStart, "open", "", "open"), step(bedStart, "join", "", "joined")}, b.seat)
	first := bedStart.Add(5 * time.Minute)
	for cycle := 0; cycle < 12; cycle++ {
		b.cycle(first.Add(time.Duration(cycle) * cadence))
	}
	if open := b.open(); len(open) != 0 {
		t.Fatalf("an episode opened after %s of counted time: %+v", b.active(id), open)
	}
	b.cycle(first.Add(12 * cadence))
	open := b.open()
	if len(open) != 1 || b.active(id) != 2*time.Hour {
		t.Fatalf("after twelve counted cycles: active %s, episodes %+v; want 2h and one episode", b.active(id), open)
	}
	if open[0].Owner != "pattern:stagnation" || open[0].ScopeID != id {
		t.Fatalf("episode identity %q %q", open[0].Owner, open[0].ScopeID)
	}
	if len(b.sent) != 1 || !strings.HasPrefix(b.sent[0], "The landing lane has worked on batch 00000 for 2 hours without landing it.\nrun: metasystem alert list") {
		t.Fatalf("notifications %q; want one, two plain lines", b.sent)
	}
	b.cycle(first.Add(13 * cadence))
	if len(b.open()) != 1 || len(b.sent) != 1 {
		t.Fatalf("a later finding opened or notified again: %d open, %q", len(b.open()), b.sent)
	}
}

// A retry that repeats sealed→proving after a refusal is not progress: five
// refusals since the high-water mark last rose report the batch, four do not.
func TestRetriesAreNotProgress(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	four, five := batchID("b"), batchID("c")
	b.writeBatch(four, refused(bedStart, 4), b.seat)
	b.writeBatch(five, refused(bedStart, 5), b.seat)
	b.cycle(bedStart.Add(10 * time.Minute))
	open := b.open()
	if len(open) != 1 || open[0].ScopeID != five {
		t.Fatalf("episodes %+v; want one, for the batch refused five times", open)
	}
	refusals := 0
	for _, item := range open[0].Evidence {
		if item.Fact == "prove-refused proving→sealed" {
			refusals++
		}
	}
	if refusals != 5 || !strings.HasPrefix(b.sent[0], "The landing lane tried batch 00000 5 times without moving it forward.") {
		t.Fatalf("evidence %+v, notification %q", open[0].Evidence, b.sent)
	}
	// A rise of the high-water mark is progress: the count starts over.
	steps := append(refused(bedStart, 5), step(bedStart.Add(20*time.Minute), "land", "sealed", "landing"))
	b.writeBatch(five, steps, b.seat)
	b.cycle(bedStart.Add(20 * time.Minute))
	b.cycle(bedStart.Add(30 * time.Minute))
	if open := b.open(); len(open) != 0 {
		t.Fatalf("the batch moved forward to landing, yet its episode stays open: %+v", open)
	}
}

// Only a Clear moves an episode toward cleared: an unreadable batch record,
// an unreadable pause and an unreadable member helm leave it open however
// long they last, and say so.
func TestUnknownNeverClears(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("d")
	b.writeBatch(id, refused(bedStart, 5), b.seat)
	at := bedStart.Add(10 * time.Minute)
	b.cycle(at)
	if len(b.open()) != 1 {
		t.Fatalf("no episode opened: %+v", b.episodes())
	}
	b.writeRaw(b.batchPath(id), []byte("{"))
	for range 4 {
		at = at.Add(cadence)
		b.cycle(at)
	}
	if open := b.open(); len(open) != 1 || open[0].Standing != steward.StandingUnreadable {
		t.Fatalf("an unreadable record moved the episode: %+v", b.episodes())
	}
	// The batch moved forward, which reads clear, but not while its pause is
	// unreadable.
	b.writeBatch(id, append(refused(bedStart, 5), step(at, "land", "sealed", "landing")), b.seat)
	b.writeRaw(b.pausePath(), []byte("not json"))
	for range 3 {
		at = at.Add(cadence)
		b.cycle(at)
	}
	if open := b.open(); len(open) != 1 || open[0].CleanCount != 0 {
		t.Fatalf("an unreadable pause moved the episode toward cleared: %+v", b.episodes())
	}
	b.resume()
	b.cycle(at.Add(cadence))
	if open := b.open(); len(open) != 1 || open[0].CleanCount != 1 {
		t.Fatalf("one clear cycle: %+v", b.episodes())
	}
	b.cycle(at.Add(2 * cadence))
	if open := b.open(); len(open) != 0 {
		t.Fatalf("two clear cycles left the episode open: %+v", open)
	}
	if len(b.sent) != 1 {
		t.Fatalf("clearing notified: %q", b.sent)
	}
}

// A member seat at the helm holds its batch whole: no episode opens however
// the batch looks, and the report comes once the seat returns the helm.
func TestHeldByMemberHelmNoEpisode(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("f")
	b.writeBatch(id, refused(bedStart, 6), b.seat)
	b.takeHelm(b.seat)
	at := bedStart.Add(10 * time.Minute)
	for range 15 {
		b.cycle(at)
		at = at.Add(cadence)
	}
	if episodes := b.episodes(); len(episodes) != 0 || len(b.sent) != 0 || b.active(id) != 0 {
		t.Fatalf("a batch held by its member's helm reported: %+v %q, active %s", episodes, b.sent, b.active(id))
	}
	b.returnHelm(b.seat)
	b.cycle(at)
	if open := b.open(); len(open) != 1 {
		t.Fatalf("the returned helm did not report the batch: %+v", b.episodes())
	}
}

// A person's clear suppresses the same batch until its detector reads it
// clear once; a finding after that opens a new episode.
func TestManualClearSuppressesSameBatch(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("g")
	b.writeBatch(id, refused(bedStart, 5), b.seat)
	at := bedStart.Add(10 * time.Minute)
	b.cycle(at)
	first := b.open()
	if len(first) != 1 {
		t.Fatalf("no episode: %+v", b.episodes())
	}
	if _, changed, err := steward.ClearAlert(b.repo, first[0].EpisodeID, bedInvoker, at); err != nil || !changed {
		t.Fatalf("clear: %v %v", changed, err)
	}
	if _, changed, err := steward.ClearAlert(b.repo, first[0].EpisodeID, bedInvoker, at); err != nil || changed {
		t.Fatalf("a repeated clear must succeed and change nothing: %v %v", changed, err)
	}
	for range 4 {
		at = at.Add(cadence)
		b.cycle(at)
	}
	if open := b.open(); len(open) != 0 || len(b.sent) != 1 {
		t.Fatalf("a cleared batch reopened: %+v %q", open, b.sent)
	}
	// The batch moves forward: its detector reads it clear, which lifts the
	// suppression; new refusals after that are a new episode.
	moved := append(refused(bedStart, 5), step(at, "land", "sealed", "landing"))
	b.writeBatch(id, moved, b.seat)
	at = at.Add(cadence)
	b.cycle(at)
	for index := 0; index < 5; index++ {
		moved = append(moved, step(at.Add(time.Duration(index+1)*time.Second), "land-refused", "landing", "sealed"))
	}
	b.writeBatch(id, moved, b.seat)
	at = at.Add(cadence)
	b.cycle(at)
	open := b.open()
	if len(open) != 1 || open[0].EpisodeID == first[0].EpisodeID || len(b.sent) != 2 {
		t.Fatalf("after the suppression lifted: open %+v, sent %q", open, b.sent)
	}
}

// Another batch is new work: a person's clear of one batch never suppresses
// another, and two stagnant batches are two episodes.
func TestNewBatchOpensNewEpisode(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	one, two := batchID("h"), batchID("j")
	b.writeBatch(one, refused(bedStart, 5), b.seat)
	at := bedStart.Add(10 * time.Minute)
	b.cycle(at)
	first := b.open()
	if len(first) != 1 {
		t.Fatalf("no episode: %+v", b.episodes())
	}
	if _, _, err := steward.ClearAlert(b.repo, first[0].EpisodeID, bedInvoker, at); err != nil {
		t.Fatal(err)
	}
	b.writeBatch(two, refused(bedStart.Add(time.Minute), 5), b.seat)
	b.cycle(at.Add(cadence))
	open := b.open()
	if len(open) != 1 || open[0].ScopeID != two || len(b.sent) != 2 {
		t.Fatalf("the second batch: open %+v, sent %q", open, b.sent)
	}
}

// Exactly the observed not-held time counts across a 30-minute pause, its
// resume and a 20-minute steward outage.
func TestActiveTimeAcrossPauseResumeRestart(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("k")
	b.writeBatch(id, []batch.HistoryEntry{step(bedStart, "open", "", "open")}, b.seat)
	minute := func(n int) time.Time { return bedStart.Add(time.Duration(n) * time.Minute) }
	for _, n := range []int{0, 10, 20, 30, 40, 50} { // 50 minutes observed
		b.cycle(minute(n))
	}
	b.pause(minute(55))
	for _, n := range []int{60, 70, 80} {
		b.cycle(minute(n))
	}
	b.resume()                              // at 85
	for _, n := range []int{90, 100, 110} { // 80→90 began held; 90→110 counts
		b.cycle(minute(n))
	}
	for _, n := range []int{130, 140, 150} { // the outage 110→130 is not counted
		b.cycle(minute(n))
	}
	if got := b.active(id); got != 90*time.Minute {
		t.Fatalf("counted %s; want exactly the observed not-held 1h30m", got)
	}
}

// An interval with an unreadable hold at either end is not counted, and no
// episode opens while the hold cannot be read.
func TestUnreadableHoldIsNotCounted(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("m")
	b.writeBatch(id, refused(bedStart, 5), b.seat)
	b.setting("steward.pattern.stagnation.repeat=50")
	minute := func(n int) time.Time { return bedStart.Add(time.Duration(n) * time.Minute) }
	b.cycle(minute(0))
	b.cycle(minute(10)) // +10
	b.writeRaw(b.pausePath(), []byte("{"))
	b.cycle(minute(20)) // unreadable end: not counted
	b.resume()
	b.cycle(minute(30)) // unreadable start: not counted
	b.cycle(minute(40)) // +10
	if got := b.active(id); got != 20*time.Minute {
		t.Fatalf("counted %s; want 20m", got)
	}
	// A member seat whose checkout was removed holds no helm: it is not a
	// hold and not unreadable, so the batch counts and reports.
	c := newBed(t)
	c.writeBatch(id, refused(bedStart, 5), pathOf(c.t.TempDir(), "gone"))
	c.cycle(minute(10))
	c.cycle(minute(20))
	if open := c.open(); len(open) != 1 || c.active(id) != 10*time.Minute {
		t.Fatalf("a removed member seat blocked the batch: %+v %s", c.episodes(), c.active(id))
	}
}

func pathOf(parts ...string) string { return strings.Join(parts, "/") }
