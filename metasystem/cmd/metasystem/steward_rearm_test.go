package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStewardRunPublicVerbRefreshRefusalStillRevivesAndDelivers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := steward.QueueNotification(root, steward.PendingNotification{Nonce: "pending", Message: "pending before refresh"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) }}
	refreshes := 0
	refresh := func() (bool, error) {
		refreshes++
		if err := steward.MintIntent(root, steward.Intent{Nonce: "prepared", RepoIdentity: "fixture", InstallGen: 1, Goal: "fix-it", Runtime: "fake", Model: "fixture"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return false, errors.New("this checkout's engine is behind the landing branch, and its source has local edits")
	}
	registered := families()
	for i := range registered {
		if registered[i].name == "steward" {
			for j := range registered[i].verbs {
				if registered[i].verbs[j].name == "run" {
					registered[i].verbs[j].run = func(args []string, stdout, stderr io.Writer) int {
						return runStewardRunWithDependencies(args, stdout, stderr, steward.RunLoop, probeStewardRuntime, clock, func(string) int { return 1 }, refresh)
					}
				}
			}
		}
	}
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", root}, &stdout, &stderr, registered, func(string) (string, error) {
		t.Fatal("runner verb called Git")
		return "", nil
	})
	log, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "notifications.log"))
	if code != 0 || refreshes != 1 || err != nil || !strings.Contains(string(log), "pending before refresh") {
		t.Fatalf("exit=%d refreshes=%d deliveries=%q error=%v stderr=%s", code, refreshes, log, err, stderr.String())
	}
	ended, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "cancelled", "prepared.json"))
	if err != nil || !strings.Contains(string(ended), "cancelled:") {
		t.Fatalf("revival did not re-arbitrate the prepared intent: %q %v", ended, err)
	}
	if pending, err := steward.PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("runner left notifications pending: %+v %v", pending, err)
	}
}

func TestStewardStatusPublicVerbShowsDeferredRearm(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatal(stderr)
	}
	if err := steward.NoteDeferredRearm(bed.landingA, "main", "old", laneTestNow); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(t, "landing", "status")
	if code != 0 || !strings.Contains(stdout, "engine main, checkout old, re-arm deferred") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
