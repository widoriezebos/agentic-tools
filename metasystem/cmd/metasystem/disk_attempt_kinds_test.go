package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The steward reads landing batches and landing test receipts through local
// types of the fields that name attempts (the landing packages import the
// steward). This test marshals the landing packages' own types and requires
// the readers to find every attempt, so a renamed field fails here.
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
	ids := []string{"proof-a-0000000000000001", "proof-b-0000000000000002", "proof-c-0000000000000003", "proof-d-0000000000000004",
		"proof-e-0000000000000005", "proof-f-0000000000000006", "proof-g-0000000000000007", "proof-h-0000000000000008", "proof-i-0000000000000009"}
	record := batch.Record{State: batch.StateProving,
		Units:    []batch.Unit{{GoalID: "g", Admission: &batch.JoinAdmission{AttemptID: ids[0]}}},
		TrunkRed: &batch.TrunkRedHold{Red: batch.TrunkRed{AttemptID: ids[1]}},
		Proof:    &batch.Proof{AttemptID: ids[2], Sources: map[string]batch.Source{"g": {Kind: batch.SourceReused, Attempt: ids[3]}}},
		Receipts: map[string]batch.PrefixReceipt{"x": {AttemptID: ids[4], Reused: map[string]string{"g": ids[5]}}}}
	write(filepath.Join(root, "artifacts", "agents", "landing-batches", "b.json"), record)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	receipt := landing.TestReceipt{Time: now.Format(time.RFC3339Nano), AttemptIDs: []string{ids[6]}, Proof: &landing.TestReceiptProof{AttemptID: ids[7]},
		Testing: &proofrun.TestResult{Groups: []proofrun.GroupResult{{ID: "g", ReuseAttempt: ids[8]}}}}
	write(filepath.Join(root, "artifacts", "agents", "landing", "receipts", "tree.json"), receipt)
	var named []string
	for _, namer := range steward.LandingAttemptNamers([]string{root}, root, 14*24*time.Hour) {
		found, err := namer.Named(context.Background(), now)
		if err != nil {
			t.Fatal(err)
		}
		named = append(named, found...)
	}
	slices.Sort(named)
	if !slices.Equal(slices.Compact(named), ids) {
		t.Fatalf("named = %v, want %v", named, ids)
	}
}
