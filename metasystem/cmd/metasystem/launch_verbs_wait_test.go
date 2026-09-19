package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLaunchWaitNamesTheCapAndThePendingState(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 19, 6, 0, 0, 0, time.UTC)
	now := start
	manager := &launch.Manager{
		Store: launch.Store{Root: t.TempDir()},
		Now:   func() time.Time { return now },
		Sleep: func(d time.Duration) { now = now.Add(d) },
		Poll:  time.Second,
		Settings: launch.Settings{WaitCapSeconds: 2, Values: []launch.Setting{{
			Key: launch.WaitCapKey, Value: "2", Source: "fixture",
		}}},
	}
	if err := manager.Store.Create(launch.Record{
		ID: "pending", Kind: "build", Adapter: "codex-exec", WorkingDirectory: t.TempDir(),
		State: launch.Running, StartedAt: start.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatal(err)
	}

	now = start
	var stdout, stderr bytes.Buffer
	rc := launchWaitWith(manager, []string{"--id", "pending", "--timeout", "3h"}, &stdout, &stderr)
	wantStderr := "launch wait: --timeout 3h0m0s exceeds launch.wait.cap.seconds=2; waiting 2s\n" +
		"launch wait: pending still running after 2s; run launch wait again or launch status\n"
	if rc != 3 || stdout.String() != "" || stderr.String() != wantStderr || now.Sub(start) != 2*time.Second {
		t.Fatalf("rc=%d stdout=%q stderr=%q elapsed=%s", rc, stdout.String(), stderr.String(), now.Sub(start))
	}

	now = start
	stdout.Reset()
	stderr.Reset()
	rc = launchWaitWith(manager, []string{"--id", "pending", "--timeout", "1s"}, &stdout, &stderr)
	wantStderr = "launch wait: pending still running after 1s; run launch wait again or launch status\n"
	if rc != 3 || stdout.String() != "" || stderr.String() != wantStderr || now.Sub(start) != time.Second {
		t.Fatalf("rc=%d stdout=%q stderr=%q elapsed=%s", rc, stdout.String(), stderr.String(), now.Sub(start))
	}

	if _, err := manager.Store.Update("pending", func(record *launch.Record) error {
		record.State = launch.Completed
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	now = start
	stdout.Reset()
	stderr.Reset()
	rc = launchWaitWith(manager, []string{"--id", "pending", "--timeout", "1s"}, &stdout, &stderr)
	if rc != 0 || stderr.String() != "" || !strings.HasPrefix(stdout.String(), "id=pending state=completed") || !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}
