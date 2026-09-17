package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestBatchLedgerOwnerDefaultRefusesUnbound(t *testing.T) {
	owner, err := batchLedgerOwner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Record("op", batch.TrunkRed{})
	if err == nil || !strings.HasPrefix(err.Error(), "TRUNK_RED_OWNER_UNBOUND: ") {
		t.Fatalf("record error=%v, want named unbound-owner refusal", err)
	}
}
