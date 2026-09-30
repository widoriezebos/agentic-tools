package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Idempotency rows for the alert verbs (behaviour patterns, design §1): a
// repeated ack or clear succeeds and records nothing more.
func init() {
	registerIdempotency("alert list", idemRead, "", nil)
	registerIdempotency("alert ack", idemStateful, "an alert already marked seen keeps its first acknowledgment", witnessAlertAct("ack"))
	registerIdempotency("alert clear", idemStateful, "a cleared alert keeps its first clear", witnessAlertAct("clear"))
}

func witnessAlertAct(act string) func(*testing.T) {
	return func(t *testing.T) {
		bed := newAlertBed(t)
		episode := bed.open(t, bed.landing, "4gr18nm8t3nyev9sssda9jgtsq")
		id := "lane/" + episode.EpisodeID
		if code, stdout, stderr := bed.run(t, bed.seat, "alert", act, id, "--json"); code != 0 || !strings.Contains(stdout, `"outcome": "confirmed"`) {
			t.Fatalf("first %s = %d %q %q", act, code, stdout, stderr)
		}
		first, err := steward.AlertEpisodes(bed.store(bed.landing))
		if err != nil {
			t.Fatal(err)
		}
		if code, stdout, stderr := bed.run(t, bed.seat, "alert", act, id, "--json"); code != 0 || !strings.Contains(stdout, `"outcome": "unchanged"`) {
			t.Fatalf("repeated %s = %d %q %q", act, code, stdout, stderr)
		}
		again, err := steward.AlertEpisodes(bed.store(bed.landing))
		if err != nil || len(again) != 1 || again[0].AcknowledgedAt != first[0].AcknowledgedAt || again[0].ClearedAt != first[0].ClearedAt {
			t.Fatalf("the repeat changed the record: %+v -> %+v (%v)", first, again, err)
		}
	}
}
