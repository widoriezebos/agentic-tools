package steward

import (
	"os"
	"path/filepath"
	"testing"
)

type spendEpisodeFixture struct {
	t        *testing.T
	root     string
	messages []string
}

func newSpendEpisodeFixture(t *testing.T) *spendEpisodeFixture {
	t.Helper()
	return &spendEpisodeFixture{t: t, root: t.TempDir()}
}

func (f *spendEpisodeFixture) deliver(root, message string) error {
	f.t.Helper()
	if root != f.root {
		f.t.Fatalf("spend delivery root = %q, want %q", root, f.root)
	}
	episodes, err := loadAlertEpisodesUnlocked(root)
	if err != nil {
		f.t.Fatal(err)
	}
	matches := 0
	for _, episode := range episodes {
		if episode.Owner != string(RoleSpendFence) || episode.Message != message || episode.TransportResult != TransportPending {
			continue
		}
		if len(episode.Attempts) != 1 || episode.Attempts[0].Result != TransportPending {
			f.t.Fatalf("delivery preceded a durable pending attempt: %+v", episode)
		}
		matches++
	}
	if matches != 1 {
		f.t.Fatalf("delivery message matched %d pending spend episodes: %q", matches, message)
	}
	f.messages = append(f.messages, message)
	return nil
}

func (f *spendEpisodeFixture) assertDelivered(episodes []AlertEpisode) {
	f.t.Helper()
	messageCounts := map[string]int{}
	for _, message := range f.messages {
		messageCounts[message]++
	}
	for _, episode := range episodes {
		if episode.Owner != string(RoleSpendFence) {
			continue
		}
		if episode.TransportResult != TransportSubmitted || len(episode.Attempts) != 1 || episode.Attempts[0].Result != TransportSubmitted {
			f.t.Fatalf("spend episode lacks one submitted attempt: %+v", episode)
		}
		messageCounts[episode.Message]--
	}
	for message, remaining := range messageCounts {
		if remaining != 0 {
			f.t.Fatalf("delivery count differs from submitted episodes by %d for %q", remaining, message)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, "artifacts", "agents", "steward", "alerts.flock")); err != nil {
		f.t.Fatalf("spend episode lock was not created: %v", err)
	}
}
