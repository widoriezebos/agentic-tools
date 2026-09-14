package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	watchsurface "github.com/widoriezebos/agentic-tools/metasystem/internal/watch"
)

func TestWaitCompatibilityMappings(t *testing.T) {
	original := compatibilityWaitCommand
	defer func() { compatibilityWaitCommand = original }()

	cases := []struct {
		code    int
		outcome string
	}{
		{0, "green"}, {1, "red"}, {2, "ended-unknown"}, {3, "launch-failed"}, {4, "target-replaced"},
		{64, "registration-refused"}, {65, "storage-failure"}, {66, "uncertain-identity"},
		{67, "invalid-arguments"}, {124, "wait-deadline"}, {130, "interrupted"},
	}
	for _, test := range cases {
		compatibilityWaitCommand = func(_ []string, _ func(context.Context) error, _ int64, printResult func(metarun.WaitResult, bool)) int {
			printResult(metarun.WaitResult{ExitCode: test.code, SourceOutcome: test.outcome, TargetIncarnation: metarun.WaiterTarget{Generation: 1}}, false)
			return test.code
		}
		if output, code := captureStdout(t, func() int {
			return runRunWatch([]string{"--root", t.TempDir(), "--id", "compat", "--caller-pid", "1"})
		}); code != test.code {
			t.Fatalf("run watch wait exit %d mapped to %d (output %q)", test.code, code, output)
		}
		if code := runJobWatchVerb([]string{"--root", t.TempDir(), "--job", "compat", "--caller-pid", "1"}); code != test.code {
			t.Fatalf("job watch wait exit %d mapped to %d", test.code, code)
		}
	}

	dispatchSource, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "dispatch.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(dispatchSource)
	for _, mapping := range []string{
		`0) return 0 ;;`, `1) return 3 ;;`, `2) return 4 ;;`, `3) return 8 ;;`, `4) return 5 ;;`,
	} {
		if !strings.Contains(source, mapping) {
			t.Fatalf("delegate --wait compatibility mapping missing %q", mapping)
		}
	}
	if !strings.Contains(source, `unknown family "wait"`) || !strings.Contains(source, `wait_for_job_legacy "$job"`) {
		t.Fatal("delegate --wait lost its old-engine fallback")
	}
	start := strings.Index(source, "wait_for_job_legacy()")
	if start < 0 {
		t.Fatal("delegate wait functions could not be isolated")
	}
	end := strings.Index(source[start:], "aggregate_chain_usage()")
	if end < 0 {
		t.Fatal("delegate wait functions could not be isolated")
	}
	functionsPath := filepath.Join(t.TempDir(), "wait-functions.sh")
	if err := os.WriteFile(functionsPath, []byte(source[start:start+end]), 0o600); err != nil {
		t.Fatal(err)
	}
	harnessPath := filepath.Join(t.TempDir(), "harness.sh")
	harness := `#!/usr/bin/env bash
set -euo pipefail
root=$1
jobs=$2
heartbeats=$3
ms=$4
functions=$5
current_claim_epoch=1
lease_run_held() { return 0; }
json_field() { printf '%s\n' "${FIXTURE_STATUS:-completed}"; }
source "$functions"
wait_for_job compat
`
	if err := os.WriteFile(harnessPath, []byte(harness), 0o700); err != nil {
		t.Fatal(err)
	}
	stateRoot := t.TempDir()
	jobs := filepath.Join(stateRoot, "artifacts", "agents", "jobs")
	heartbeats := filepath.Join(stateRoot, "artifacts", "agents", "heartbeats")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(heartbeats, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "metasystem")
	for _, mapping := range []struct{ wait, delegate int }{{0, 0}, {1, 3}, {2, 4}, {3, 8}, {4, 5}} {
		stubSource := fmt.Sprintf("#!/bin/sh\nexit %d\n", mapping.wait)
		if err := os.WriteFile(stub, []byte(stubSource), 0o700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(harnessPath, stateRoot, jobs, heartbeats, stub, functionsPath)
		err := command.Run()
		got := 0
		if err != nil {
			got = err.(*exec.ExitError).ExitCode()
		}
		if got != mapping.delegate {
			t.Fatalf("delegate --wait mapped wait exit %d to %d, want %d", mapping.wait, got, mapping.delegate)
		}
	}
	if err := os.WriteFile(filepath.Join(jobs, "compat.json"), []byte("{\"status\":\"completed\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho 'metasystem: unknown family \"wait\"' >&2\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(harnessPath, stateRoot, jobs, heartbeats, stub, functionsPath).CombinedOutput(); err != nil {
		t.Fatalf("old-engine delegate fallback failed: %v %s", err, output)
	}
}

func TestWatchReadSurfaceAllTrackedClassesAndZeroWrite(t *testing.T) {
	root := watchFixture(t)
	before := watchTreeHash(t, root)
	out, code := captureStdout(t, func() int {
		return dispatch([]string{"watch", "--root", root, "--json"})
	})
	after := watchTreeHash(t, root)
	if before != after {
		t.Fatalf("watch changed the checkout tree: before=%s after=%s", before, after)
	}
	if code != 1 {
		t.Fatalf("known persisted failures must return attention exit 1: code=%d out=%s", code, out)
	}
	var snapshot watchsurface.Snapshot
	if err := json.Unmarshal([]byte(out), &snapshot); err != nil {
		t.Fatalf("watch JSON must parse: %v\n%s", err, out)
	}
	if snapshot.SchemaVersion != 1 || snapshot.Aggregate != watchsurface.AggregateAttention || snapshot.Empty {
		t.Fatalf("unexpected snapshot envelope: %+v", snapshot)
	}
	want := []watchsurface.TrackedClass{
		watchsurface.ClassJobs,
		watchsurface.ClassCompletedRounds,
		watchsurface.ClassCensus,
		watchsurface.ClassHealth,
		watchsurface.ClassDelivery,
		watchsurface.ClassAlerts,
		watchsurface.ClassIntents,
		watchsurface.ClassBreachRoutes,
	}
	if len(snapshot.Sections) != len(want) {
		t.Fatalf("the closed class list must always have %d sections: %+v", len(want), snapshot.Sections)
	}
	for index, section := range snapshot.Sections {
		if section.Class != want[index] {
			t.Fatalf("section %d drifted: got %s want %s", index, section.Class, want[index])
		}
		if section.Verdict == "" || len(section.Items) == 0 {
			t.Fatalf("fixture class %s must appear with a typed store verdict and items: %+v", section.Class, section)
		}
		for _, item := range section.Items {
			if item.Kind == "" || item.ID == "" || item.Verdict == "" || item.Evidence == "" {
				t.Fatalf("class %s has an untyped item: %+v", section.Class, item)
			}
		}
	}
	var noGoalFailure watchsurface.Item
	for _, item := range snapshot.Sections[0].Items {
		if item.ID == "no-goal-failure" {
			noGoalFailure = item
			break
		}
	}
	if noGoalFailure.GoalField != watchsurface.GoalFieldNull || noGoalFailure.Verdict != "failed" {
		t.Fatalf("explicit goalId null must remain distinct and visible: %+v", noGoalFailure)
	}
	completed := snapshot.Sections[1]
	if len(completed.Items) != 1 || completed.Items[0].Verdict != watchsurface.VerdictUnknownConsumption {
		t.Fatalf("completed return newer than its goal receipt must be typed unknown-consumption: %+v", completed)
	}
}

func TestWatchAbsentHealthIsDeadAndZeroWrite(t *testing.T) {
	root := t.TempDir()
	before := watchTreeHash(t, root)
	out, code := captureStdout(t, func() int {
		return dispatch([]string{"watch", "--root", root, "--json"})
	})
	after := watchTreeHash(t, root)
	if before != after {
		t.Fatalf("empty watch changed the checkout tree: before=%s after=%s", before, after)
	}
	if code != 1 {
		t.Fatalf("an absent steward health record must be dead: code=%d out=%s", code, out)
	}
	var snapshot watchsurface.Snapshot
	if err := json.Unmarshal([]byte(out), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Empty || snapshot.Aggregate != watchsurface.AggregateAttention || len(snapshot.Sections) != 8 {
		t.Fatalf("absent health fail-safe contract drifted: %+v", snapshot)
	}
	health := snapshot.Sections[3]
	if health.Class != watchsurface.ClassHealth || health.Verdict != watchsurface.SectionDegraded || len(health.Items) != 1 ||
		health.Items[0].Verdict != "dead" || !strings.Contains(health.Items[0].Problem, "age=unknown") {
		t.Fatalf("absent health record must be named as dead: %+v", health)
	}
}

func TestWatchStaleHealthAndGoalFailurePrintsDeadRecordAge(t *testing.T) {
	root := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), "global.gitconfig")
	if err := os.WriteFile(globalConfig, []byte("[metasystem \"steward\"]\n\ttick-seconds = 7200\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	command := exec.Command("git", "init", "--quiet", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("initialize isolated watch fixture: %v: %s", err, output)
	}
	command = exec.Command("git", "-C", root, "config", "metasystem.steward.tick-seconds", "600")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("set isolated watch cadence: %v: %s", err, output)
	}
	now := time.Now().UTC().Truncate(time.Second)
	watchWriteJSON(t, root, "artifacts/agents/jobs/goal-failure.json", map[string]any{
		"jobId": "goal-failure", "round": 1, "role": "implementer", "status": "failed", "goalId": "goal-one",
		"endedAt": now.Add(-30 * time.Minute).Format(time.RFC3339),
	})
	watchWriteJSON(t, root, "artifacts/agents/steward/health.json", map[string]any{
		"verdict": map[string]any{
			"schema": 1, "observedAt": now.Add(-time.Hour).Format(time.RFC3339), "observation": 1, "aggregate": "healthy",
			"roles": []map[string]any{{"role": "claimed-goal-delivery", "status": "alive"}},
		},
	})
	out, code := captureStdout(t, func() int {
		return dispatch([]string{"watch", "--root", root})
	})
	if code != 1 || !strings.Contains(out, "WATCH ATTENTION") ||
		!strings.Contains(out, "health-freshness health-record dead") ||
		!strings.Contains(out, "artifacts/agents/steward/health.json") || !strings.Contains(out, "age=") {
		t.Fatalf("stale health did not print a dead record and age: code=%d out=%s", code, out)
	}
}

func TestWatchPreservesJobWaitSelection(t *testing.T) {
	if !requestsJobWait([]string{"--job", "job-one"}) || !requestsJobWait([]string{"--job=job-one"}) {
		t.Fatal("both existing job waiter spellings must select the waiter")
	}
	if requestsJobWait([]string{"--root", "contains--job"}) || requestsJobWait([]string{"--json"}) {
		t.Fatal("snapshot flags must not select the waiter")
	}
}

func watchFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	stamp := now.Format(time.RFC3339)
	watchWriteJSON(t, root, "artifacts/agents/jobs/no-goal-failure.json", map[string]any{
		"jobId": "no-goal-failure", "status": "failed", "goalId": nil,
	})
	watchWriteJSON(t, root, "artifacts/agents/jobs/unconsumed-return.json", map[string]any{
		"jobId": "unconsumed-return", "role": "implementer", "round": 2, "status": "completed", "goalId": "goal-one",
		"endedAt": now.Add(-time.Minute).Format(time.RFC3339),
	})
	receiptPath := filepath.Join(root, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(receiptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	receiptAt := now.Add(-time.Hour)
	receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=goal-one\n", receiptAt.Unix(), receiptAt.Format(time.RFC3339))
	if err := os.WriteFile(receiptPath, []byte(receipt), 0o644); err != nil {
		t.Fatal(err)
	}
	watchWriteJSON(t, root, "artifacts/agents/supervision/last-census.json", map[string]any{
		"schemaVersion": 2, "writer": "watcher-pass", "verdict": "SUCCESS", "completedAt": stamp,
	})
	watchWriteJSON(t, root, "artifacts/agents/steward/health.json", map[string]any{
		"state": map[string]any{"sequence": 1, "observedAt": stamp, "unknownCounts": map[string]int{}, "failureCounts": map[string]int{}},
		"verdict": map[string]any{
			"schema": 1, "observedAt": stamp, "observation": 1, "aggregate": "unhealthy",
			"roles": []map[string]any{{"role": "claimed-goal-delivery", "status": "dead", "reason": "persisted delivery failure"}},
		},
	})
	watchWriteJSON(t, root, "artifacts/agents/steward/pending/notice-one.json", map[string]any{
		"nonce": "notice-one", "message": "operator notification awaiting submission",
	})
	watchWriteJSON(t, root, "artifacts/agents/steward/alerts/alert-one.json", map[string]any{
		"schema": 1, "episodeId": "alert-one", "digest": strings.Repeat("a", 64), "message": "persisted finding",
		"openedAt": stamp, "attempts": []any{}, "transportResult": "PENDING", "acknowledged": false,
		"resolved": false, "cleared": false,
	})
	watchWriteJSON(t, root, "artifacts/agents/steward/intents/intent-one.json", map[string]any{
		"nonce": "intent-one", "jobId": "repair-one", "goal": "goal-one",
	})
	if err := goal.WriteStopBatch(root, goal.StopBatch{
		StopID: "stop-one", GoalID: "goal-one", GoalRevision: 1, FenceEpoch: 1,
		CapabilityGeneration: 1, Machine: "machine-one", ClaimEpoch: 1,
		Reason: goal.StopReasonCorruptOverLimit, State: goal.StopBatchOpen,
		OpenedAt: stamp, UpdatedAt: stamp, Pending: []string{"job-one"},
	}); err != nil {
		t.Fatalf("write breach-stop fixture: %v", err)
	}
	return root
}

func watchWriteJSON(t *testing.T, root, relative string, value any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func watchTreeHash(t *testing.T, root string) string {
	t.Helper()
	paths := []string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write([]byte(target))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}
