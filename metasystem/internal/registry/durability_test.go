package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B6's third outcome for the append families: a failed durability barrier is
// VISIBLE BUT UNCOMMITTED — the caller gets a plain error and claims no
// success, and the reader survives whatever reached the file.
func TestAppendFrameRefusesWhenNotDurable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.ndjson")
	if err := AppendFrame(path, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("clean append: %v", err)
	}
	original := syncFile
	syncFile = func(*os.File) error { return errors.New("injected: sync failed") }
	defer func() { syncFile = original }()

	err := AppendFrame(path, []byte(`{"b":2}`))
	if err == nil {
		t.Fatal("an append that could not be made durable reported success")
	}
	if !strings.Contains(err.Error(), "not durably written") {
		t.Fatalf("the failure does not say what happened: %v", err)
	}
	// Whatever reached the file, the reader must still make sense of it:
	// the first record survives and a later append repairs the tail.
	syncFile = original
	if err := AppendFrame(path, []byte(`{"c":3}`)); err != nil {
		t.Fatalf("append after a torn write: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `{"a":1}`) || !strings.Contains(string(data), `{"c":3}`) {
		t.Fatalf("records lost across the torn write: %s", data)
	}
}
