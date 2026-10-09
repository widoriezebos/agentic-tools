package phase_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
)

func TestRunReconcilesUnitStopWithoutChannel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Unix(10, 0)
	question, err := channel.Ask(channel.AskRequest{
		RepoRoot: root, Goal: "g", Kind: "stop", Machine: "m", Now: now,
		Facts: []string{"The reader needs a correction."},
		UnitStop: &channel.UnitStopQuestion{
			Loop: "unit-round", Subject: "g/u/run", Attempt: 2, Finding: "read:1",
			Review: "read", Needs: "metasystem work revise g --work u --reason 'correct the reader' --by Wido",
			AcceptableActs: []string{"work-revise"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	act := channel.UnitStopAct{
		ID: "successful-remedy", Goal: "g", Loop: "unit-round", Subject: "g/u/run",
		Attempt: 2, Findings: []string{"read:1"}, Kind: "work-revise",
		Reason: "correct the reader", At: now.Add(time.Second),
	}
	data, err := json.Marshal(act)
	if err != nil {
		t.Fatal(err)
	}
	// A saved act can survive an interruption before its question closes.
	dir := filepath.Join(root, "artifacts", "agents", "channel", "unit-stop-acts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, channel.Digest(act.ID)+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if count, err := phase.Run(context.Background(), root); err != nil || count != 0 {
			t.Fatalf("tick: undelivered=%d err=%v", count, err)
		}
		stored, err := channel.ReadQuestion(root, question.ID)
		if err != nil || stored.State != "closed" {
			t.Fatalf("successful remedy left its question open: %+v err=%v", stored, err)
		}
	}
}

func TestRunReturnsUnitStopReconciliationFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts", "agents", "channel", "unit-stop-acts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unreadable.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := phase.Run(context.Background(), root)
	if _, ok := err.(*json.SyntaxError); !ok || count != 1 {
		t.Fatalf("reconciliation failure: undelivered=%d err=%v; want one failure and its JSON error", count, err)
	}
}
