package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestReceiptAddFillsUsageFromTheLaunchRecord(t *testing.T) {
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	store := launch.Store{}
	var measurement launch.Measurement
	if err := json.Unmarshal([]byte(`{"calls":2,"toolCalls":3,"inputTokens":10,"cacheReadTokens":20,"cacheCreationTokens":30,"outputTokens":40,"peakContext":50}`), &measurement); err != nil {
		t.Fatal(err)
	}
	record := launch.Record{ID: "design-launch", Kind: "design", State: launch.Completed, Measurement: measurement}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	receiptRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(receiptRoot, "metasystem.conf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(receiptRoot, "memory", "receipts.log")
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runReceipt([]string{"add", "--root", receiptRoot, "--file", ledger, "--type", "design", "--outcome", "shipped", "--launch", record.ID})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "receipt recorded") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	want := "|design_tokens=100|design_calls=3|requests=2|tool_calls=3|peak_context=50|cache_read_tokens=20|output_tokens=40|"
	if !strings.Contains(string(data), want) {
		t.Fatalf("launch usage missing from receipt: %s", data)
	}
	missing := "missing-launch"
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runReceipt([]string{"add", "--root", receiptRoot, "--file", ledger, "--type", "design", "--outcome", "shipped", "--launch", missing})
	})
	if code != 2 || !strings.Contains(stderr, missing) {
		t.Fatalf("missing launch code=%d stderr=%q", code, stderr)
	}

	running := launch.Record{ID: "running-launch", Kind: "design", State: launch.Running}
	if err := store.Create(running); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runReceipt([]string{"add", "--root", receiptRoot, "--file", ledger, "--type", "design", "--outcome", "shipped", "--launch", running.ID})
	})
	if code != 2 || !strings.Contains(stderr, running.ID) || !strings.Contains(stderr, "not terminal") {
		t.Fatalf("running launch code=%d stderr=%q", code, stderr)
	}

	explicitLedger := filepath.Join(receiptRoot, "memory", "explicit.log")
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runReceipt([]string{"add", "--root", receiptRoot, "--file", explicitLedger, "--type", "design", "--outcome", "shipped", "--launch", record.ID,
			"--design-tokens", "999", "--design-calls", "8", "--requests", "9", "--tool-calls", "10", "--peak-context", "11", "--cache-read-tokens", "12", "--output-tokens", "13"})
	})
	data, err = os.ReadFile(explicitLedger)
	if code != 0 || stderr != "" || err != nil || !strings.Contains(string(data), "|design_tokens=999|design_calls=8|requests=9|tool_calls=10|peak_context=11|cache_read_tokens=12|output_tokens=13|") {
		t.Fatalf("explicit usage did not win: code=%d stderr=%q data=%q err=%v", code, stderr, data, err)
	}
}

func TestReceiptCorrectVerbRejectsInvalidProvenanceValues(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "memory", "receipts.log")
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
