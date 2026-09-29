package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestArmedCheckoutsKeepTheirError (R24, U10b-1): an unreadable registry
// returns its error, a registry that does not exist yet is an empty host
// with no error, and open owners are the armed checkouts, sorted, once each.
func TestArmedCheckoutsKeepTheirError(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	unreadable := filepath.Join(home, "unreadable.jsonl")
	if err := os.Mkdir(unreadable, 0o700); err != nil {
		t.Fatal(err)
	}
	if checkouts, err := ArmedCheckouts(unreadable); err == nil || checkouts != nil {
		t.Fatalf("an unreadable registry = %v, %v; want its error", checkouts, err)
	}
	if checkouts, err := ArmedCheckouts(filepath.Join(home, "absent.jsonl")); err != nil || len(checkouts) != 0 {
		t.Fatalf("an absent registry = %v, %v; want an empty host", checkouts, err)
	}
	path := filepath.Join(home, "armed-checkouts.jsonl")
	for _, row := range []struct{ checkout, tag string }{{"/c/m1e/metasystem", "tag-e"}, {"/c/m1b/metasystem", "tag-b"}, {"/c/m1b/metasystem", "tag-b2"}} {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": EventRelaunched, "checkoutPath": row.checkout, "ownerTag": row.tag,
			"at": "2026-09-29T08:00:00Z", "generation": 1, "watcherTag": row.tag + "-w", "reaperTag": row.tag + "-r", "retiredThrough": 0})
		if err := AppendFrame(path, payload); err != nil {
			t.Fatal(err)
		}
	}
	checkouts, err := ArmedCheckouts(path)
	if err != nil || !slices.Equal(checkouts, []string{"/c/m1b/metasystem", "/c/m1e/metasystem"}) {
		t.Fatalf("armed = %v, %v", checkouts, err)
	}
}
