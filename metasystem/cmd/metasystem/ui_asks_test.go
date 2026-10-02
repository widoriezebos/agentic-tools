package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

// TestUIAsksNamesTheQuestionRecordsItCouldNotRead: the interface's question
// reader answers every open question it could read, and the records it
// could not beside them, rather than dropping them as though nobody asked.
func TestUIAsksNamesTheQuestionRecordsItCouldNotRead(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	dir := filepath.Join(checkout, "artifacts", "agents", "channel", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	open := channel.Question{ID: "q-1", Goal: "g", Machine: "m1f", OpenedAt: time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC), State: "open", Facts: []string{"Land it?"}}
	data, err := json.Marshal(open)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "q-1.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	asks := uiAsks(checkout)
	if read, err := asks(); err != nil || len(read) != 1 {
		t.Fatalf("a whole channel = %v, %v; want the one question and no error", read, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "q-2.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	read, err := asks()

	var unread *httpd.UnreadQuestions
	if !errors.As(err, &unread) || len(unread.Records) != 1 {
		t.Fatalf("err = %v; want the torn record named", err)
	}
	if len(read) != 1 || read[0].ID != "q-1" {
		t.Fatalf("questions = %v; want the readable one kept", read)
	}
}
