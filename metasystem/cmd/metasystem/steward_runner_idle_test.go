package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStewardRunPublicVerbSleepsAndLogsATick(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, dir := range []string{filepath.Join(root, ".git", "metasystem"), filepath.Join(root, "artifacts", "agents", "steward")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "metasystem", "helm.json"), []byte(`{"schema":1,"by":"fixture","at":"2026-10-04T08:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	now, calls := start, 0
	stop := func() {
		if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	clock := &steward.HandoffClock{
		Now: func() time.Time {
			calls++
			if calls > 100 {
				stop()
			}
			return now
		},
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			if !now.Before(start.Add(time.Second)) {
				stop()
			}
		},
	}
	registered := families()
	for i := range registered {
		if registered[i].name != "steward" {
			continue
		}
		for j := range registered[i].verbs {
			if registered[i].verbs[j].name == "run" {
				registered[i].verbs[j].run = func(args []string, stdout, stderr io.Writer) int {
					return runStewardRunWithDependencies(args, stdout, stderr, clock, func(string) int { return 1 }, nil)
				}
			}
		}
	}
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", root}, &stdout, &stderr, registered, func(string) (string, error) { t.Fatal("runner verb called Git"); return "", nil })
	log, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.log"))
	if code != 0 || err != nil || now.Sub(start) != time.Second || string(log) != "tick 1: health 0.000ms census 0.000ms other 0.000ms, slept 1.000 s\n" {
		t.Fatalf("exit=%d elapsed=%s log=%q error=%v stderr=%s", code, now.Sub(start), log, err, stderr.String())
	}
	data, err := os.ReadFile(steward.ComponentEvidencePath(root, "steward-tick"))
	var attempt steward.ComponentEvidence
	if err == nil {
		err = json.Unmarshal(data, &attempt)
	}
	if err != nil || attempt.AttemptSeq != 1 || attempt.Result != steward.ComponentOK || attempt.Outcome != "HELM" {
		t.Fatalf("public verb did not complete one real tick: %+v %v", attempt, err)
	}
}
