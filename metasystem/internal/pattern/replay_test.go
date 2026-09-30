package pattern

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

const replayBatch = "4gr18nm8t3nyev9sssda9jgtsq"

// replayRecord is the copied landing record of 2026-09-30 as it stood at
// now: the history entries written by then, the batch state they left, and
// the member's seat moved to this bed's seat.
func (b *bed) replayRecord(now time.Time) bool {
	b.t.Helper()
	data, err := os.ReadFile("testdata/replay-batch-" + replayBatch + ".json")
	if err != nil {
		b.t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		b.t.Fatal(err)
	}
	var kept []any
	state := ""
	for _, raw := range record["history"].([]any) {
		entry := raw.(map[string]any)
		at, err := time.Parse(time.RFC3339Nano, entry["at"].(string))
		if err != nil {
			b.t.Fatal(err)
		}
		if at.After(now) {
			break
		}
		kept = append(kept, entry)
		if to := entry["to"].(string); to != "joined" {
			state = to
		}
	}
	if len(kept) == 0 {
		return false
	}
	record["history"], record["state"] = kept, state
	for _, raw := range record["units"].([]any) {
		raw.(map[string]any)["seatRoot"] = b.seat
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		b.t.Fatal(err)
	}
	b.writeRaw(b.batchPath(replayBatch), encoded)
	return true
}

// The day's record replayed at the real 600 s cadence, not held from the
// join until the pause at 18:26 CEST (16:26Z), with the first cycle a full
// cadence after the join: one episode opens once two hours are counted, no
// later than 11:20Z; the fifth refusal since the batch reached proving
// (14:56:22Z) is appended to it at the first cycle after; one notification
// in all.
func TestReplay20260930Stagnation(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	pausedAt := time.Date(2026, 9, 30, 16, 26, 0, 0, time.UTC)
	fifth := "2026-09-30T14:56:22.432767Z"
	var openedAt, fifthAt time.Time
	for now := time.Date(2026, 9, 30, 9, 9, 51, 0, time.UTC); now.Before(time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)); now = now.Add(cadence) {
		if !b.replayRecord(now) {
			continue
		}
		if !now.Before(pausedAt) {
			b.pause(pausedAt)
		}
		b.cycle(now)
		open := b.open()
		if len(open) > 0 && openedAt.IsZero() {
			openedAt = now
			if !strings.HasPrefix(open[0].Message, "The landing lane has worked on batch 4gr18 for 2 hours without landing it.\n") {
				t.Fatalf("the episode says %q", open[0].Message)
			}
		}
		for _, episode := range open {
			for _, item := range episode.Evidence {
				if item.At == fifth && item.Fact == "prove-refused proving→sealed" && fifthAt.IsZero() {
					fifthAt = now
				}
			}
		}
	}
	if openedAt.IsZero() || openedAt.Before(time.Date(2026, 9, 30, 11, 0, 0, 0, time.UTC)) || openedAt.After(time.Date(2026, 9, 30, 11, 20, 0, 0, time.UTC)) {
		t.Fatalf("the episode opened at %s; want between 11:00Z and 11:20Z", openedAt)
	}
	wantFifth := time.Date(2026, 9, 30, 14, 59, 51, 0, time.UTC)
	if !fifthAt.Equal(wantFifth) {
		t.Fatalf("the fifth refusal was appended at %s; want the first cycle after 14:56:22Z, %s", fifthAt, wantFifth)
	}
	t.Logf("opened at %s (%s CEST), fifth refusal appended at %s", openedAt.Format(time.RFC3339), openedAt.Add(2*time.Hour).Format("15:04:05"), fifthAt.Format(time.RFC3339))
	episodes := b.episodes()
	if len(episodes) != 1 || len(b.sent) != 1 {
		t.Fatalf("%d episodes, notifications %q; want one of each", len(episodes), b.sent)
	}
	if episodes[0].Cleared || episodes[0].Standing != steward.StandingHeld || len(episodes[0].Evidence) != 20 {
		t.Fatalf("at 18:00Z the batch is diagnosing under the pause: %+v", episodes[0])
	}
}
