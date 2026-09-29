package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The steward reads landing batches and landing test receipts through local
// types of the fields that name attempts (the landing packages import the
// steward). This test fills the landing packages' own types by reflection,
// every string field named or tagged attempt or reuse with a fresh id, and
// requires the readers to name every one, so a new or renamed field fails
// here (Round B3-2 R3).
func TestAttemptKindsReadTheLandingPackagesOwnRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(path string, value any) {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fill := &attemptFill{}
	var record batch.Record
	fill.fill(reflect.ValueOf(&record).Elem(), "batch.Record", false, 0)
	record.State = batch.StateProving
	write(filepath.Join(root, "artifacts", "agents", "landing-batches", "b.json"), record)
	var receipt landing.TestReceipt
	fill.fill(reflect.ValueOf(&receipt).Elem(), "landing.TestReceipt", false, 0)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	receipt.Time = now.Format(time.RFC3339Nano)
	write(filepath.Join(root, "artifacts", "agents", "landing", "receipts", "tree.json"), receipt)
	var named []string
	for _, namer := range steward.LandingAttemptNamers([]string{root}, root, 14*24*time.Hour) {
		found, err := namer.Named(context.Background(), now)
		if err != nil {
			t.Fatal(err)
		}
		named = append(named, found...)
	}
	if len(fill.ids) < 8 {
		t.Fatalf("the fill reached only %d attempt fields", len(fill.ids))
	}
	if missing := fill.uncovered(named, landingNotAttemptIDs); len(missing) > 0 {
		t.Fatalf("attempt fields no reader names:\n%s", strings.Join(missing, "\n"))
	}
}

// landingNotAttemptIDs are the landing record fields the detector's name
// rule matches that carry no attempt id, each with why.
var landingNotAttemptIDs = map[string]string{
	"batch.Record.Proof.SourcesUnresolved":                 "a sentence saying why a proof's sources were not recorded",
	"landing.TestReceipt.Testing.EngineRearm.SourceCommit": "a git commit the engine was rebuilt from",
}
