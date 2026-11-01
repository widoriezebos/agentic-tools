package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

// pageBed is the duty bed with the tick's page over counted sources: each
// read projects the ledger once and samples the load once.
type pageBed struct {
	*dutyBed
	board              string
	projections, loads int
	src                SituationSources
}

func newPageBed(t *testing.T) *pageBed {
	t.Helper()
	bed := &pageBed{dutyBed: newDutyBed(t), board: t.TempDir()}
	bed.src = withEmptyHost(t, SituationSources{LaneHome: func() (string, error) { return t.TempDir(), nil },
		BoardHome: func() (string, error) { return bed.board, nil }, Project: countedProject(&bed.projections)})
	load := bed.src.Load
	bed.src.Load = func(now time.Time) hostload.Sample { bed.loads++; return load(now) }
	bed.cfg.Situation = func(root string, now time.Time) Situation { return bed.read(root, now) }
	return bed
}

// read is the bed's sources read at now, through the test guard.
func (b *pageBed) read(root string, now time.Time) Situation {
	src := b.src
	src.Now = func() time.Time { return now }
	return readSituation(b.t, root, src)
}

// reads is how many reads ran since the last call, by both counters.
func (b *pageBed) reads() (projections, loads int) {
	projections, loads, b.projections, b.loads = b.projections, b.loads, 0, 0
	return projections, loads
}

func (b *pageBed) ask(text string) string {
	b.t.Helper()
	sent, err := board.Publish(b.board, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: text}, b.now)
	if err != nil {
		b.t.Fatal(err)
	}
	return sent.Message.ID
}

// saved is the page file as it decodes now.
func (b *pageBed) saved() SituationRecord {
	b.t.Helper()
	var record SituationRecord
	data, err := os.ReadFile(situationPagePath(b.root))
	if err == nil {
		err = json.Unmarshal(data, &record)
	}
	if err != nil {
		b.t.Fatalf("the page file: %v", err)
	}
	return record
}

func asked(record SituationRecord, id string) bool {
	for _, ask := range record.Situation.Asks {
		if ask.Thread.ID == id {
			return true
		}
	}
	return false
}

func (b *pageBed) timing() string {
	b.t.Helper()
	data, err := os.ReadFile(runnerLogPath(b.root))
	if err != nil {
		b.t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return lines[len(lines)-1]
}

func TestTheTickWritesOnePagePerCycleAndTheVerbReadsIt(t *testing.T) {
	t.Parallel()
	bed := newPageBed(t)
	pass := func() time.Time {
		t.Helper()
		at := bed.now
		bed.run(1, func(int) dutyCycle { return dutyCycle{} }, nil)
		return at
	}
	page := func(now time.Time) SituationRecord {
		t.Helper()
		record, err := SituationPage(bed.root, func(at time.Time) Situation { return bed.read(bed.root, at) }, now, SituationFresh)
		if err != nil {
			t.Fatalf("the page at %v: %v", now, err)
		}
		return record
	}

	first := pass()
	if saved := bed.saved(); !saved.Composition.ReadAt.Equal(first) || saved.Composition.Machine != "m1e" || saved.Composition.Digest == "" {
		t.Fatalf("the pass saved %+v, want this machine's page read at %v", saved.Composition, first)
	}
	if entry, ran := bed.duties()["situation"]; !ran || entry.Error != "" {
		t.Fatalf("the situation duty: ran %v, error %q", ran, entry.Error)
	}
	if line := bed.timing(); !strings.Contains(line, " situation ") || !strings.Contains(line, "ms, slept") {
		t.Fatalf("the timing line lacks the page: %q", line)
	}
	if projections, loads := bed.reads(); projections != 1 || loads != 1 {
		t.Fatalf("one pass read the ledger %d and the load %d times, want once each", projections, loads)
	}

	// An ask after the pass: a reader within the freshness gets the saved
	// page and reads nothing; the next pass reads it, as its page is never
	// fresh to itself.
	one := bed.ask("Is it built?")
	if cached := page(first.Add(time.Minute)); asked(cached, one) || !cached.Composition.ReadAt.Equal(first) {
		t.Fatalf("a fresh page was read again: read at %v, the new ask %v", cached.Composition.ReadAt, asked(cached, one))
	}
	if projections, loads := bed.reads(); projections != 0 || loads != 0 {
		t.Fatalf("a fresh page read the ledger %d and the load %d times, want none", projections, loads)
	}
	second := pass()
	if saved := bed.saved(); !asked(saved, one) || !saved.Composition.ReadAt.Equal(second) {
		t.Fatalf("the pass after the ask kept the old page: read at %v, the ask %v", saved.Composition.ReadAt, asked(saved, one))
	}
	if projections, loads := bed.reads(); projections != 1 || loads != 1 {
		t.Fatalf("the second pass read the ledger %d and the load %d times, want once each", projections, loads)
	}
	two := bed.ask("Is it proven?")
	if stale := page(second.Add(3 * time.Minute)); !asked(stale, two) || !stale.Composition.ReadAt.Equal(second.Add(3*time.Minute)) {
		t.Fatalf("a stale page was kept: read at %v, the second ask %v", stale.Composition.ReadAt, asked(stale, two))
	}
	if saved := bed.saved(); !asked(saved, two) || !saved.Composition.ReadAt.Equal(second.Add(3*time.Minute)) {
		t.Fatalf("the stale page's read was not saved: read at %v", saved.Composition.ReadAt)
	}
	if projections, loads := bed.reads(); projections != 1 || loads != 1 {
		t.Fatalf("a stale page read the ledger %d and the load %d times, want once each", projections, loads)
	}

	// At the helm the pass still writes the page.
	takeHelmFixture(t, bed.root)
	if helm := pass(); !bed.saved().Composition.ReadAt.Equal(helm) {
		t.Fatalf("at the helm the page was read at %v, want %v", bed.saved().Composition.ReadAt, helm)
	}

	// A save that fails is the duty's health row, at the helm too, and a
	// later save clears it.
	path := situationPagePath(bed.root)
	aside := filepath.Join(filepath.Dir(path), "situation-aside")
	if err := os.Rename(path, aside); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "blocked"), 0o755); err != nil {
		t.Fatal(err)
	}
	pass()
	failed := pass()
	if row := bed.role(bed.cycle, dutyTestRole("situation")); row.Status != HealthDead || !strings.HasPrefix(row.Reason, "situation failed: ") {
		t.Fatalf("at %v the failed save reads %+v, want the situation duty dead", failed, row)
	}
	if line := dutyLine("situation", errors.New("x")); line != "situation page: x" {
		t.Fatalf("the failure is said as %q", line)
	}
	if err := os.Rename(path, filepath.Join(filepath.Dir(path), "situation-folder")); err != nil {
		t.Fatal(err)
	}
	pass()
	pass()
	if row := bed.role(bed.cycle, dutyTestRole("situation")); row.Status != HealthAlive {
		t.Fatalf("after a save the row reads %+v, want it alive", row)
	}

	// A malformed page reads as none and is replaced; another machine's page
	// is stale to this one.
	bed.reads()
	for _, content := range []string{"{", `{"Composition":{"Machine":"m1x","ReadAt":"` + bed.now.Format(time.RFC3339Nano) + `"}}`} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := page(bed.now); got.Composition.Machine != "m1e" || !bed.saved().Composition.ReadAt.Equal(bed.now) {
			t.Fatalf("the page over %q: %+v", content, got.Composition)
		}
		if projections, loads := bed.reads(); projections != 1 || loads != 1 {
			t.Fatalf("the page over %q read the ledger %d and the load %d times, want once each", content, projections, loads)
		}
	}
	if _, err := SituationPage(t.TempDir(), nil, bed.now, SituationFresh); err == nil || err.Error() != "no situation reader" {
		t.Fatalf("a nil reader with no page: %v", err)
	}
}

func TestATickWithoutAPageReaderWritesNone(t *testing.T) {
	t.Parallel()
	bed := newDutyBed(t)
	bed.run(1, func(int) dutyCycle { return dutyCycle{} }, nil)
	if _, err := os.Stat(situationPagePath(bed.root)); !os.IsNotExist(err) {
		t.Fatalf("a tick with no reader wrote a page: %v", err)
	}
	if _, ran := bed.duties()["situation"]; ran {
		t.Fatal("a tick with no reader recorded the situation duty")
	}
	data, err := os.ReadFile(runnerLogPath(bed.root))
	if err != nil || !strings.Contains(string(data), " situation 0.000ms, slept") {
		t.Fatalf("the timing line: %q %v", data, err)
	}
}
