package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLandingDesignCheckDigestOnce(t *testing.T) {
	t.Parallel()
	fixture := laneFixture{channelFixture: newUnservedChannelFixture(t), home: t.TempDir()}
	writeLaneRecord(t, fixture.home, fixture.root)
	check := plain.DesignCheck{Goal: "g", Commit: "commit", Verdict: "design-changed", Reason: "warning: goal g was built against a design that changed; the landing goes on", At: laneSilentT0.Format(time.RFC3339)}
	if err := plain.RecordDesignCheck(fixture.root, check); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (stateroot.Layout, error) {
		return stateroot.Layout{InstallationRoot: fixture.root, GitRoot: fixture.root}, nil
	}
	appendDigest := func(root string, entries []narratordigest.Entry, now time.Time) error {
		return narratordigest.AppendWithLayoutReader(root, entries, now, resolve)
	}
	var output strings.Builder
	ledger := fixtureSpendLedger()
	dependencies := tickHealthDependencies{
		evaluate: tickHealthRoles(t, fixture.root, ledger.Machine, func(string, string, time.Time) (spend.Ledger, error) { return ledger, nil }),
		now:      time.Now, deliver: func(string, string) error { return nil },
		lane: func(self string, now time.Time) LaneSilence {
			return readLaneSilenceWithDesignChecks(fixture.home, self, now, appendDigest, &output)
		},
	}
	for _, now := range []time.Time{laneSilentT0, laneSilentT0.Add(time.Minute)} {
		result := TickResult{}
		if err := completeTickHealthWithDependencies(fixture.root, &result, 1, identity.Ref{Pid: 1, StartedAtSec: 1}, now, dependencies); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := os.ReadFile(filepath.Join(fixture.root, "records", "narrator-digest.log"))
	if err != nil || strings.Count(string(digest), check.Reason) != 1 || !strings.Contains(string(digest), "design-gate-landing g@commit") || output.Len() != 0 {
		t.Fatalf("digest=%q output=%q err=%v", digest, output.String(), err)
	}
	reads := 0
	carryLaneDesignChecks(fixture.home, t.TempDir(), laneSilentT0, func(string, []narratordigest.Entry, time.Time) error { reads++; return nil }, &output)
	if reads != 0 {
		t.Fatal("another checkout carried the lane's warning")
	}
	carryLaneDesignChecks(fixture.home, fixture.root, laneSilentT0, func(string, []narratordigest.Entry, time.Time) error { return errors.New("digest unavailable") }, &output)
	if !strings.Contains(output.String(), "digest unavailable") || strings.Count(output.String(), "\n") != 2 || !strings.Contains(output.String(), "the tick goes on") {
		t.Fatalf("digest failure pair: %q", output.String())
	}
}

func TestLandingDesignCheckCarryReadFailures(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"lane", "checks"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			fixture := laneFixture{channelFixture: newUnservedChannelFixture(t), home: t.TempDir()}
			writeLaneRecord(t, fixture.home, fixture.root)
			if failure == "lane" {
				if err := os.WriteFile(lane.RecordPath(fixture.home), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Join(plain.Dir(fixture.root), "design-gate.jsonl"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			appends := 0
			appendDigest := func(string, []narratordigest.Entry, time.Time) error { appends++; return nil }
			for _, self := range []string{fixture.root, t.TempDir()} {
				var output strings.Builder
				carryLaneDesignChecks(fixture.home, self, laneSilentT0, appendDigest, &output)
				if failure == "checks" && self == fixture.root {
					if !strings.Contains(output.String(), "could not reach the narrator") || strings.Count(output.String(), "\n") != 2 {
						t.Fatalf("check read failure pair: %q", output.String())
					}
				} else if output.Len() != 0 {
					t.Fatalf("unowned lane read printed a warning: %q", output.String())
				}
			}
			if appends != 0 {
				t.Fatalf("carried unreadable design checks %d times", appends)
			}
		})
	}
}

var laneSilentT0 = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

// laneFixture is a host lane whose flat installation is the steward root of
// a channel fixture: the steward that keeps the lane is the one that reads it.
type laneFixture struct {
	channelFixture
	home string
}

func newLaneFixture(t *testing.T) laneFixture {
	t.Helper()
	fixture := laneFixture{channelFixture: newChannelFixture(t), home: t.TempDir()}
	writeLaneRecord(t, fixture.home, fixture.root)
	return fixture
}

func writeLaneRecord(t *testing.T, home, root string) {
	t.Helper()
	record := lane.Record{Root: root, Install: root, CustodyEpoch: 1, RegisteredBy: "fixture", At: laneSilentT0.Add(-24 * time.Hour).Format(time.RFC3339)}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f laneFixture) handIn(t *testing.T, goal string, at time.Time) {
	t.Helper()
	if _, _, err := plain.HandIn(f.root, plain.Line{Goal: goal, Branch: "goal/" + goal, SHA: strings.Repeat("a", 40), Seat: "seat", At: at.UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
}

func (f laneFixture) appendLaneLine(t *testing.T, name string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(plain.Dir(f.root), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func (f laneFixture) proofEnded(t *testing.T, at time.Time) {
	f.appendLaneLine(t, "results.jsonl", plain.Result{Tree: "tree", Commit: "commit", Result: plain.Red, At: at.UTC().Format(time.RFC3339)})
}

func (f laneFixture) pushed(t *testing.T, at time.Time) {
	f.appendLaneLine(t, "pushes.jsonl", plain.Pushed{Old: "old", Commit: "new", Tree: "tree", At: at.UTC().Format(time.RFC3339)})
}

// proofRunning records a proof whose process is this test process, so it
// reads as running.
func (f laneFixture) proofRunning(t *testing.T, since time.Time) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("the test process identity can't be read: %v", err)
	}
	ref, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plain.Running{Attempt: "a1", Tree: "tree", Commit: "commit", Since: since.UTC().Format(time.RFC3339), Pid: int64(os.Getpid()), Process: ref})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plain.Dir(f.root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(f.root), "running.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// laneQuestion writes a question record as the landing agent's
// `question ask --about lane` leaves it in the lane installation.
func (f laneFixture) laneQuestion(t *testing.T, state string, opened time.Time) {
	t.Helper()
	dir := filepath.Join(f.root, "artifacts", "agents", "channel", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"id": "q-lane-1", "goal": "", "about": "lane", "kind": "other", "machine": "bed-m1", "openedAt": opened.UTC(), "state": state}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "q-lane-1.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// tick runs the tick's health completion, where the lane-silent episode
// lives beside the spend episodes, at now.
func (f laneFixture) tick(t *testing.T, now time.Time) {
	t.Helper()
	tickLaneHealth(t, f.root, f.home, now)
}

func tickLaneHealth(t *testing.T, root, home string, now time.Time) {
	t.Helper()
	ledger := fixtureSpendLedger()
	measure := func(string, string, time.Time) (spend.Ledger, error) { return ledger, nil }
	dependencies := tickHealthDependencies{
		evaluate: tickHealthRoles(t, root, ledger.Machine, measure), now: time.Now,
		deliver: func(string, string) error { return nil },
		lane:    func(self string, at time.Time) LaneSilence { return readLaneSilence(home, self, at) },
	}
	result := TickResult{}
	if err := completeTickHealthWithDependencies(root, &result, 1, identity.Ref{Pid: 1, StartedAtSec: 1}, now, dependencies); err != nil {
		t.Fatal(err)
	}
}

const laneSilentCommand = "metasystem landing status"

func TestLaneSilentOpensAtTwentyMinutes(t *testing.T) {
	t.Parallel()
	fixture := newLaneFixture(t)
	fixture.handIn(t, "queued-goal", laneSilentT0)
	fixture.tick(t, laneSilentT0.Add(19*time.Minute))
	if posts := fixture.posts(t); len(posts) != 0 {
		t.Fatalf("19 minutes without progress posted: %q", posts)
	}
	fixture.tick(t, laneSilentT0.Add(20*time.Minute))
	posts := fixture.posts(t)
	if len(posts) != 1 {
		t.Fatalf("20 minutes without progress posted %d times, want once: %q", len(posts), posts)
	}
	assertTwoLineNotice(t, posts[0], "landing lane", laneSilentCommand)
	fixture.tick(t, laneSilentT0.Add(25*time.Minute))
	if posts := fixture.posts(t); len(posts) != 1 {
		t.Fatalf("the standing silence posted again: %q", posts)
	}
	if owned := episodesOwnedBy(t, fixture.root, laneSilentAlertOwner); len(owned) != 1 || owned[0].TransportResult != TransportSubmitted {
		t.Fatalf("want one submitted lane-silent episode: %+v", owned)
	}

	// Progress ends the episode; a new silence is a new episode.
	fixture.pushed(t, laneSilentT0.Add(26*time.Minute))
	fixture.tick(t, laneSilentT0.Add(27*time.Minute))
	owned := episodesOwnedBy(t, fixture.root, laneSilentAlertOwner)
	if len(owned) != 1 || !owned[0].Cleared {
		t.Fatalf("lane progress did not clear the silent episode: %+v", owned)
	}
	fixture.tick(t, laneSilentT0.Add(46*time.Minute))
	if posts := fixture.posts(t); len(posts) != 2 {
		t.Fatalf("a new 20-minute silence after progress did not post once more: %q", posts)
	}
}

func TestLaneSilentAfterAProofEndedWithWorkQueued(t *testing.T) {
	t.Parallel()
	fixture := newLaneFixture(t)
	fixture.handIn(t, "queued-goal", laneSilentT0.Add(-2*time.Hour))
	fixture.proofEnded(t, laneSilentT0.Add(-30*time.Minute))
	fixture.tick(t, laneSilentT0)
	if posts := fixture.posts(t); len(posts) != 1 {
		t.Fatalf("a proof that ended 30 minutes ago with work queued and no push posted %d times: %q", len(posts), posts)
	}
}

func TestLaneSilentHeldByRunningProofPauseOrLaneQuestion(t *testing.T) {
	t.Parallel()
	for _, hold := range []struct {
		name  string
		apply func(*testing.T, laneFixture)
	}{
		{"proof running", func(t *testing.T, f laneFixture) { f.proofRunning(t, laneSilentT0.Add(-time.Hour)) }},
		{"lane paused", func(t *testing.T, f laneFixture) {
			if _, err := lane.SetPause(f.home, "wido", laneSilentT0.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
		}},
		{"lane question open", func(t *testing.T, f laneFixture) { f.laneQuestion(t, "open", laneSilentT0.Add(-time.Hour)) }},
	} {
		t.Run(hold.name, func(t *testing.T) {
			t.Parallel()
			fixture := newLaneFixture(t)
			fixture.handIn(t, "queued-goal", laneSilentT0.Add(-2*time.Hour))
			hold.apply(t, fixture)
			fixture.tick(t, laneSilentT0)
			if posts := fixture.posts(t); len(posts) != 0 {
				t.Fatalf("%s still posted lane silent: %q", hold.name, posts)
			}
		})
	}
}

func TestLaneQuestionOpenedCountsAsLaneProgress(t *testing.T) {
	t.Parallel()
	fixture := newLaneFixture(t)
	fixture.handIn(t, "queued-goal", laneSilentT0.Add(-2*time.Hour))
	fixture.laneQuestion(t, "answered", laneSilentT0.Add(-10*time.Minute))
	fixture.tick(t, laneSilentT0)
	if posts := fixture.posts(t); len(posts) != 0 {
		t.Fatalf("a lane question opened 10 minutes ago did not count as progress: %q", posts)
	}
}

func TestLaneSilentNeedsQueuedWorkTheLaneStewardAndAChannel(t *testing.T) {
	t.Parallel()
	t.Run("nothing queued", func(t *testing.T) {
		t.Parallel()
		fixture := newLaneFixture(t)
		fixture.tick(t, laneSilentT0)
		if posts := fixture.posts(t); len(posts) != 0 {
			t.Fatalf("an empty lane posted: %q", posts)
		}
	})
	t.Run("another steward", func(t *testing.T) {
		t.Parallel()
		// The lane, with work silent for two hours, is another checkout's.
		fixture := newLaneFixture(t)
		other := t.TempDir()
		if _, _, err := plain.HandIn(other, plain.Line{Goal: "queued-goal", SHA: strings.Repeat("a", 40), At: laneSilentT0.Add(-2 * time.Hour).Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
		writeLaneRecord(t, fixture.home, other)
		fixture.tick(t, laneSilentT0)
		if posts := fixture.posts(t); len(posts) != 0 {
			t.Fatalf("a steward that does not keep the lane posted: %q", posts)
		}
	})
	t.Run("no channel", func(t *testing.T) {
		t.Parallel()
		root, home := t.TempDir(), t.TempDir()
		writeLaneRecord(t, home, root)
		if _, _, err := plain.HandIn(root, plain.Line{Goal: "queued-goal", SHA: strings.Repeat("a", 40), At: laneSilentT0.Add(-2 * time.Hour).Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
		tickLaneHealth(t, root, home, laneSilentT0)
		if owned := episodesOwnedBy(t, root, laneSilentAlertOwner); len(owned) != 0 {
			t.Fatalf("with no channel the silent lane opened an episode: %+v", owned)
		}
	})
}
