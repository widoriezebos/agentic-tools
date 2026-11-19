package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestRecoverySkipsGoneCheckoutAndReconcilesLiveCheckout(t *testing.T) {
	t.Parallel()
	owner := t.TempDir()
	home := testprovider.Register(t, owner)
	base := t.TempDir()
	live, gone := filepath.Join(base, "z-live"), filepath.Join(base, "a-gone", "metasystem")
	for _, root := range []string{live, gone} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	journal := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
	for i, root := range []string{gone, live} {
		tag := []string{"gone", "live"}[i]
		for _, event := range []string{registry.EventRelaunched, registry.EventExited} {
			payload, err := json.Marshal(map[string]any{"schemaVersion": 1, "event": event, "checkoutPath": root, "ownerTag": tag,
				"at": "2026-11-19T08:00:00Z", "generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0,
				"reason": "shutdown", "teardownComplete": true})
			if err != nil {
				t.Fatal(err)
			}
			if err := registry.AppendFrame(journal, payload); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.RemoveAll(filepath.Dir(gone)); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	q, _, err := channel.AskOrFind(channel.AskRequest{RepoRoot: live, About: "machine", Kind: "other", Machine: "live", Now: now,
		Wants: "recover seat", Facts: []string{"stale provider registration"}, Recovery: &channel.RecoverySubject{Host: "old-host", Seat: live, Launch: "failed"}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := TickConfig{Now: now, ProviderHome: home, RecoveryRegistry: journal, Seat: &fakeSeatLauncher{}}
	for range 2 {
		if err := ReconcileRecoveryRequests(owner, cfg, fakeCensus{}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Dir(gone)); !os.IsNotExist(err) {
			t.Fatalf("gone checkout was recreated: %v", err)
		}
		questions, unreadable := channel.WalkQuestions(live)
		if len(unreadable) != 0 || len(questions) != 1 || questions[0].ID != q.ID || questions[0].State != "closed" {
			t.Fatalf("live recovery was not reconciled: %+v, %v", questions, unreadable)
		}
	}
}
