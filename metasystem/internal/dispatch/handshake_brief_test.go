package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// TestDelegateBriefCarriesThePeerCount (R26, U10f-1): a delegate job's brief
// carries one metasystem-authored line, the count of peer messages waiting
// for the seat and the command that reads them, never a message's text;
// counting marks nothing; nothing waiting is no line.
func TestDelegateBriefCarriesThePeerCount(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), ".metasystem")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	claims := func() (map[string]string, error) { return map[string]string{"goal-x": "m1b"}, nil }
	if count := peerMessagesCount(home, "m1b", claims, now); count != 0 || PeerMessagesPointer(count) != "" {
		t.Fatalf("an empty board counts %d and points %q", count, PeerMessagesPointer(count))
	}
	for text, to := range map[string]board.Address{"SECRET-one": {Machine: "m1b"}, "SECRET-two": {Machine: "m1b"}, "SECRET-goal": {Goal: "goal-x"}, "SECRET-other": {Machine: "m1c"}} {
		if _, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1a"}, To: to, Text: text}, now); err != nil {
			t.Fatal(err)
		}
	}
	count := peerMessagesCount(home, "m1b", claims, now)
	line := PeerMessagesPointer(count)
	if count != 3 || line != "\n3 peer messages wait: metasystem agent inbox\n" || strings.Contains(line, "SECRET") {
		t.Fatalf("count %d, line %q", count, line)
	}
	delivered, _ := filepath.Glob(filepath.Join(board.Dir(home), "*", "mailbox", "delivered", "*", "*"))
	if len(delivered) != 0 {
		t.Fatalf("counting marked %v", delivered)
	}
}
