package retrodebt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnlyLaterRetroReceiptRowDischargesDebt(t *testing.T) {
	root := t.TempDir()
	receipts := filepath.Join(root, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(receipts), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receipts, []byte("1|before|RECEIPT|type=retro|outcome=shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Raise(root, KindArc, "arc-a:op-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if open, err := Open(root); err != nil || len(open) != 1 {
		t.Fatalf("an older retro receipt discharged new debt: %+v %v", open, err)
	}
	handle, err := os.OpenFile(receipts, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.WriteString("2|after|RECEIPT|type=review|note=type=retro\n3|after|RETRO|note=marker\n|after|RECEIPT|type=retro|outcome=missing-epoch\n"); err != nil {
		t.Fatal(err)
	}
	_ = handle.Close()
	if open, err := Open(root); err != nil || len(open) != 1 {
		t.Fatalf("non-receipt lookalikes discharged debt: %+v %v", open, err)
	}
	handle, err = os.OpenFile(receipts, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = handle.WriteString("4|after|RECEIPT|type=retro|outcome=shipped\n")
	_ = handle.Close()
	if open, err := Open(root); err != nil || len(open) != 0 {
		t.Fatalf("later retro receipt did not discharge debt: %+v %v", open, err)
	}
	if open, err := Open(root); err != nil || len(open) != 0 {
		t.Fatalf("recorded discharge was not stable: %+v %v", open, err)
	}
}

func TestOpenRefusesReceiptLedgerChangesBeneathDebt(t *testing.T) {
	for _, test := range []struct {
		name    string
		changed string
	}{
		{name: "shortened ledger", changed: "1|before|RECEIPT|type=review\n"},
		{name: "rewritten prefix", changed: "1|before|RECEIPT|type=reviex|outcome=shipped\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			receipts := filepath.Join(root, "memory", "receipts.log")
			if err := os.MkdirAll(filepath.Dir(receipts), 0o755); err != nil {
				t.Fatal(err)
			}
			original := "1|before|RECEIPT|type=review|outcome=shipped\n"
			if err := os.WriteFile(receipts, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Raise(root, KindObligation, "governed-1", time.Now()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(receipts, []byte(test.changed), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "restore the append-only ledger") {
				t.Fatalf("changed receipt prefix was accepted: %v", err)
			}
		})
	}
}

func TestRaiseRefusesUnknownDebtIdentity(t *testing.T) {
	for _, test := range []struct {
		kind   string
		source string
	}{
		{kind: "unknown", source: "battery-1"},
		{kind: KindObligation, source: ""},
		{kind: "milestone-battery", source: "retired"},
	} {
		if _, err := Raise(t.TempDir(), test.kind, test.source, time.Now()); err == nil {
			t.Fatalf("invalid debt identity kind=%q source=%q was accepted", test.kind, test.source)
		}
	}
}
