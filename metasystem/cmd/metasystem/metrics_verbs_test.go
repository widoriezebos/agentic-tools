package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
)

func TestO12GoalReportFailureWarnsWithoutChangingDoneOutcome(t *testing.T) {
	standing := generateMetricsReport
	generateMetricsReport = func(opts metrics.Options) (metrics.Result, error) {
		if opts.Root != "/fixture" || opts.GoalID != "goal-a" {
			t.Fatalf("wrong fast-path request: %+v", opts)
		}
		return metrics.Result{Target: "/fixture/artifacts/agents/metrics/goal-goal-a.md"}, errors.New("simulated write failure")
	}
	t.Cleanup(func() { generateMetricsReport = standing })

	var warnings bytes.Buffer
	doneOutcome := reportAfterConfirmedDone(0, "/fixture", "goal-a", &warnings)
	if doneOutcome != 0 {
		t.Fatalf("best-effort reporting changed the successful done outcome: %d", doneOutcome)
	}
	for _, want := range []string{"warning: goal goal-a concluded", "/fixture/artifacts/agents/metrics/goal-goal-a.md", "simulated write failure"} {
		if !strings.Contains(warnings.String(), want) {
			t.Fatalf("warning did not name %q: %s", want, warnings.String())
		}
	}
	warnings.Reset()
	if failedOutcome := reportAfterConfirmedDone(3, "/fixture", "goal-a", &warnings); failedOutcome != 3 || warnings.Len() != 0 {
		t.Fatalf("an unconfirmed done ran reporting or changed outcome: code=%d warning=%q", failedOutcome, warnings.String())
	}
}

func TestO12BothGoalDoneRoutesRequestTheGoalReport(t *testing.T) {
	standing := generateMetricsReport
	calls := map[string]int{}
	generateMetricsReport = func(opts metrics.Options) (metrics.Result, error) {
		calls[opts.GoalID]++
		return metrics.Result{Target: metrics.GoalReportTarget(opts.Root, opts.GoalID)}, nil
	}
	t.Cleanup(func() { generateMetricsReport = standing })

	t.Run("legacy mutation", func(t *testing.T) {
		root := t.TempDir()
		writeMetricsFixtureGuard(t, root)
		store := &goal.Store{Root: root}
		caller := goal.Caller{Class: "HUMAN"}
		if _, err := store.Open(caller, "legacy-goal", "Conclude through the legacy command.", "Appetite: 1h finish."); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Open(caller, "legacy-next", "Succeed the concluded goal.", "Appetite: 1h continue."); err != nil {
			t.Fatal(err)
		}
		stageHumanTerminal(t, root, int64(os.Getppid()))
		code := goalMutation(
			"done",
			[]string{"--root", root, "--id", "legacy-goal", "--conclude", "Legacy route done."},
			func(flags *flag.FlagSet) []*string {
				return []*string{
					flags.String("id", "", "goal id"),
					flags.String("conclude", "", "conclusion"),
				}
			},
			func(store *goal.Store, caller goal.Caller, values []string) (goal.Result, error) {
				return store.Done(caller, values[0], values[1], "legacy-next", false)
			},
		)
		if code != 0 {
			t.Fatalf("legacy done returned %d", code)
		}
		ledger, problems, err := store.ReadLedger()
		if err != nil || len(problems) != 0 || !legacyGoalIsDone(ledger, "legacy-goal") {
			t.Fatalf("legacy goal did not conclude: ledger=%+v problems=%v err=%v", ledger, problems, err)
		}
		if calls["legacy-goal"] != 1 {
			t.Fatalf("legacy done requested %d reports, want 1", calls["legacy-goal"])
		}
	})

	t.Run("synced mutation", func(t *testing.T) {
		root := syncedDoneFixture(t)
		code, handled := trySyncMutation("done", []string{
			"--root", root, "--id", "synced-goal", "--conclude", "Synced route done.", "--lineage", "fixture",
		})
		if !handled || code != 0 {
			t.Fatalf("synced done handled=%v code=%d", handled, code)
		}
		if calls["synced-goal"] != 1 {
			t.Fatalf("synced done requested %d reports, want 1", calls["synced-goal"])
		}
		metricsVerbGit(t, root, "cat-file", "-e", goal.AcceptedRef+":plans/goals/done/synced-goal.md")
	})
}

func legacyGoalIsDone(ledger *goal.Ledger, id string) bool {
	if ledger == nil {
		return false
	}
	for _, item := range ledger.Done {
		if item.Id == id {
			return true
		}
	}
	return false
}

func writeMetricsFixtureGuard(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func syncedDoneFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	metricsVerbGit(t, root, "init", "-q", "-b", "main")
	metricsVerbGit(t, root, "config", "user.name", "metrics-fixture")
	metricsVerbGit(t, root, "config", "user.email", "metrics-fixture@example.invalid")
	metricsVerbGit(t, root, "config", "metasystem.goal.machine", "machine-a")
	metricsVerbGit(t, root, "config", "goal.sync-remote", "local")
	writeMetricsFixtureGuard(t, root)
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}
	file := &goal.GoalFile{
		Id: "synced-goal", State: goal.StateQueued, Intent: "Conclude through the synced command.", Origin: goal.OriginMain,
		NextStep: "Appetite: 1h finish.", OpenedAt: "2026-08-20T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-20T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-machine-a-00000000",
			Verb: "open", Actor: "machine-a+fixture", Targets: []string{"synced-goal"}, Keep: -1,
		}},
	}
	for path, data := range map[string][]byte{
		filepath.Join(root, "plans", "goals", "backlog.md"):     goal.RenderRoot(rootRecord),
		filepath.Join(root, "plans", "goals", "synced-goal.md"): goal.RenderFile(file),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	metricsVerbGit(t, root, "add", "plans/goals", "scripts/agents/pre-commit-guard.sh")
	metricsVerbGit(t, root, "commit", "-q", "-m", "synced goal fixture")
	metricsVerbGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	metricsVerbGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func metricsVerbGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = environWithoutGitSteeringCLI()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
