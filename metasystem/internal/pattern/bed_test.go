package pattern

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// bed is one host with a landing lane: a nested landing checkout (the
// checkout root holds .git and the batch store; its metasystem/ installation
// is the lane steward's repoRoot), the host home the lane record lives
// under, and one member seat. Every reader is the production one.
type bed struct {
	t                     *testing.T
	home, top, repo, seat string
	sent                  []string
	pass                  Pass
}

var bedStart = time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

func newBed(t *testing.T) *bed {
	t.Helper()
	base := t.TempDir()
	b := &bed{t: t, home: filepath.Join(base, "home", ".metasystem"), top: filepath.Join(base, "landing"), seat: filepath.Join(base, "seat")}
	b.repo = filepath.Join(b.top, "metasystem")
	for _, dir := range []string{b.repo, filepath.Join(b.seat, ".git")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.repo, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", b.top).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, _, err := lane.Register(b.home, b.top, "Wido", bedStart); err != nil {
		t.Fatal(err)
	}
	b.pass = Pass{Home: func() (string, error) { return b.home, nil },
		Deliver: func(_, message string) error { b.sent = append(b.sent, message); return nil }}
	return b
}

// setting writes one override into the lane installation's metasystem.conf.
func (b *bed) setting(line string) {
	b.t.Helper()
	path := filepath.Join(b.repo, "metasystem.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte(line+"\n")...), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func step(at time.Time, verb, from, to string) batch.HistoryEntry {
	return batch.HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: from, To: to, Actor: "landing-lane"}
}

// batchID is a valid lowercase ULID ending in the letter.
func batchID(letter string) string { return strings.Repeat("0", 25) + letter }

func (b *bed) batchPath(id string) string {
	return filepath.Join(b.top, "artifacts", "agents", "landing-batches", id+".json")
}

// writeBatch writes one batch record whose state is the last batch state
// its steps entered (a join's step names the unit's state).
func (b *bed) writeBatch(id string, steps []batch.HistoryEntry, seats ...string) {
	b.t.Helper()
	record := batch.Record{Schema: 1, BatchID: id, History: steps}
	for _, entry := range steps {
		if entry.To != batch.UnitJoined {
			record.State = entry.To
		}
	}
	for index, seat := range seats {
		goal := "goal-" + string(rune('a'+index))
		record.Units = append(record.Units, batch.Unit{GoalID: goal, Chain: goal, SeatRoot: seat, State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "m1e", Lineage: "lineage", Epoch: 1, Revision: 1, AccountingRevision: 1}})
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		b.t.Fatal(err)
	}
	b.writeRaw(b.batchPath(id), data)
}

func (b *bed) writeRaw(path string, data []byte) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// refused is a batch that joined at start, sealed, and was refused n times.
func refused(start time.Time, n int) []batch.HistoryEntry {
	steps := []batch.HistoryEntry{step(start, "open", "", "open"), step(start, "join", "", "joined"), step(start.Add(time.Minute), "seal", "open", "sealed")}
	for index := 0; index < n; index++ {
		at := start.Add(time.Duration(2+index) * time.Minute)
		steps = append(steps, step(at, "prove", "sealed", "proving"), step(at, "prove-refused", "proving", "sealed"))
	}
	return steps
}

func (b *bed) cycle(now time.Time) {
	b.t.Helper()
	if err := b.pass.Run(b.repo, now); err != nil {
		b.t.Fatalf("pattern pass at %s: %v", now.Format(time.RFC3339), err)
	}
}

func (b *bed) episodes() []steward.AlertEpisode {
	b.t.Helper()
	all, err := steward.AlertEpisodes(b.repo)
	if err != nil {
		b.t.Fatal(err)
	}
	var mine []steward.AlertEpisode
	for _, episode := range all {
		if steward.IsPatternOwner(episode.Owner) {
			mine = append(mine, episode)
		}
	}
	return mine
}

func (b *bed) open() []steward.AlertEpisode {
	var open []steward.AlertEpisode
	for _, episode := range b.episodes() {
		if !episode.Cleared {
			open = append(open, episode)
		}
	}
	return open
}

// active is a batch's counted active time as the pattern state holds it.
func (b *bed) active(id string) time.Duration {
	b.t.Helper()
	raw, err := os.ReadFile(steward.PatternStatePath(b.repo))
	if err != nil {
		b.t.Fatal(err)
	}
	current, err := decodeState(raw)
	if err != nil {
		b.t.Fatal(err)
	}
	return time.Duration(current.Batches[id].ActiveSeconds * float64(time.Second))
}

func (b *bed) pause(at time.Time) {
	b.t.Helper()
	if _, err := lane.SetPause(b.home, "Wido", at); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) pausePath() string { return filepath.Join(b.home, "host", "landing-lane-paused.json") }

// resume removes the pause record, readable or not.
func (b *bed) resume() {
	b.t.Helper()
	if err := os.Remove(b.pausePath()); err != nil && !os.IsNotExist(err) {
		b.t.Fatal(err)
	}
}

func (b *bed) takeHelm(root string) {
	b.t.Helper()
	if _, err := helm.Write(root, helm.Record{By: "Wido", At: bedStart.Format(time.RFC3339), Reason: "fixture"}); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) returnHelm(root string) {
	b.t.Helper()
	if _, err := helm.Remove(root); err != nil {
		b.t.Fatal(err)
	}
}

var bedInvoker = steward.AlertInvoker{Pid: 4242, PidStartedAt: 77, UID: 501}
