package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// askedReceipts is where the steward keeps what it asked, beside health.json.
func askedReceipts(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "asked")
}

// askOnce publishes one agent ask per subject. The message id is the
// subject's digest, so a retry is the same message; the receipt keeps the
// subject asked after the board sweeps an answered thread. It decides
// nothing about who is asked.
func askOnce(receipts, home string, from board.Sender, to board.Address, subject, text string, now time.Time) error {
	id := "steward-" + signalDigest(subject)[:24]
	receipt := filepath.Join(receipts, id+".json")
	if _, err := os.Stat(receipt); err == nil {
		return nil
	}
	request := board.Request{Kind: board.KindAsk, ID: id, From: from, To: to, Text: text}
	if _, err := board.Publish(home, request, now); err != nil && !errors.Is(err, board.ErrIDTaken) {
		return err
	}
	if err := os.MkdirAll(receipts, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Subject string        `json:"subject"`
		To      board.Address `json:"to"`
		At      time.Time     `json:"at"`
	}{subject, to, now.UTC()})
	if err != nil {
		return err
	}
	if _, err := atomicfile.Create(receipt, append(data, '\n'), 0o644, receipts); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
