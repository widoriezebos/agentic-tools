package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptCorrectVerbRejectsInvalidProvenanceValues(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "plans", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "1000|1970-01-01T00:16:40Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=goal-a|built_by=delegate|critique_waived=none|waiver_stream=none|note="
	if err := os.WriteFile(ledger, []byte(original+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(original)))
	for _, test := range []struct {
		field string
		was   string
		now   string
		want  string
	}{
		{field: "goal", was: "goal-a", now: "Invalid_goal", want: "invalid corrected goal value: Invalid_goal"},
		{field: "built_by", was: "delegate", now: "critic", want: "invalid corrected built_by value: critic"},
	} {
		_, stderr, code := captureRelay(t, func() int {
			return runReceipt([]string{
				"correct", "--file", ledger, "--ref-epoch", "1000", "--ref-sha1", digest,
				"--field", test.field, "--was", test.was, "--now", test.now, "--reason", "corrupt",
			})
		})
		if code != 2 || strings.TrimSpace(stderr) != test.want {
			t.Fatalf("invalid %s correction returned code=%d stderr=%q", test.field, code, stderr)
		}
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original+"\n" {
		t.Fatalf("refused correction changed the ledger: %s", data)
	}
}
