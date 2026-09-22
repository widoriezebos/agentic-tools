package fake

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSemanticClockAssignsDeterministicProtocolTimestamps(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 2, 3, 4, 5, 123456000, time.UTC)
	dir := t.TempDir()
	rows := []byte("{\"thread_ts\":\"root\",\"user\":\"UWIDO\",\"text\":\"slack\"}\n" +
		"{\"face\":\"telegram\",\"reply_to\":1,\"user\":7,\"text\":\"telegram\"}\n")
	if err := os.WriteFile(filepath.Join(dir, "replies.jsonl"), rows, 0o600); err != nil {
		t.Fatal(err)
	}
	server := &server{dir: dir, now: func() time.Time { return now }, counter: 1000000}
	server.loadNew()
	if len(server.slackAssigned) != 1 || server.slackAssigned[0].Timestamp != "1893553445.123456" {
		t.Fatalf("Slack assigned timestamps = %+v", server.slackAssigned)
	}
	if len(server.telegramAssigned) != 1 || server.telegramAssigned[0].Date != now.Unix() {
		t.Fatalf("Telegram assigned timestamps = %+v", server.telegramAssigned)
	}
	if next := server.nextTS(); next != "1893553445.123457" {
		t.Fatalf("monotonic Slack timestamp = %q", next)
	}
}
