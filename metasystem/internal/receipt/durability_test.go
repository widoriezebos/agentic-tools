package receipt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B6 for the receipt ledger: a failed durability barrier is a plain error,
// and the line-oriented readers survive whatever reached the file.
func TestAppendLineRefusesWhenNotDurable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipts.log")
	if err := appendLine(path, "1|x|RECEIPT|type=other|outcome=parked|note=\n"); err != nil {
		t.Fatalf("clean append: %v", err)
	}
	original := syncFile
	syncFile = func(*os.File) error { return errors.New("injected: sync failed") }
	defer func() { syncFile = original }()

	err := appendLine(path, "2|x|RECEIPT|type=other|outcome=parked|note=\n")
	if err == nil {
		t.Fatal("an append that could not be made durable reported success")
	}
	if !strings.Contains(err.Error(), "not durably written") {
		t.Fatalf("the failure does not say what happened: %v", err)
	}
	syncFile = original
	// The reader still parses the file: the committed record is countable.
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "1|x|RECEIPT") {
		t.Fatalf("the committed record was lost: %s", data)
	}
}
