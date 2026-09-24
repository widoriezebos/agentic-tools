package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
	t.Run("legacy mutation", func(t *testing.T) {
		root := t.TempDir()
		writeMetricsFixtureGuard(t, root)
		store := &goal.Store{Root: root}
		caller := goal.Caller{Class: "HUMAN"}
		if _, err := store.Open(caller, "legacy-goal", "Conclude through the legacy command.", "Finish."); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Open(caller, "legacy-next", "Succeed the concluded goal.", "Continue."); err != nil {
			t.Fatal(err)
		}
		stageHumanTerminal(t, root, int64(os.Getppid()))
		calls := 0
		code := goalMutationWithInputs(
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
			trySyncMutation,
			legacyMutationInputs{
				repositoryTop: func(got string) (string, error) {
					if got != root {
						t.Fatalf("repository root = %q", got)
					}
					return root, nil
				},
				ensureGuard: func(got string) error { return checkMetricsFixtureGuard(root, got) },
				reporter: func(opts metrics.Options) (metrics.Result, error) {
					calls++
					if opts.Root != root || opts.GoalID != "legacy-goal" {
						t.Fatalf("report options = %+v", opts)
					}
					ledger, problems, err := store.ReadLedger()
					if err != nil || len(problems) != 0 || !legacyGoalIsDone(ledger, "legacy-goal") {
						t.Fatalf("report ran before Store.Done: %v %v", problems, err)
					}
					return metrics.Result{Target: metrics.GoalReportTarget(opts.Root, opts.GoalID)}, nil
				},
			},
		)
		if code != 0 {
			t.Fatalf("legacy done returned %d", code)
		}
		ledger, problems, err := store.ReadLedger()
		if err != nil || len(problems) != 0 || !legacyGoalIsDone(ledger, "legacy-goal") {
			t.Fatalf("legacy goal did not conclude: ledger=%+v problems=%v err=%v", ledger, problems, err)
		}
		if calls != 1 {
			t.Fatalf("legacy done requested %d reports, want 1", calls)
		}
	})

	t.Run("synced mutation", func(t *testing.T) {
		fixture := syncedDoneFixture(t)
		calls := 0
		code, handled := fixture.done("Synced route done.", false, func(opts metrics.Options) (metrics.Result, error) {
			calls++
			fixture.checkReport(t, opts)
			return metrics.Result{Target: metrics.GoalReportTarget(opts.Root, opts.GoalID)}, nil
		}, func(string, goal.Endpoint) (string, error) {
			t.Fatal("branchless Done fetched endpoint")
			return "", nil
		})
		if !handled || code != 0 {
			t.Fatalf("synced done handled=%v code=%d", handled, code)
		}
		if calls != 1 {
			t.Fatalf("synced done requested %d reports, want 1", calls)
		}
		fixture.checkArchived(t)
	})
}

func TestGoalDoneWithoutLocalBranchSkipsUnreadableRemote(t *testing.T) {
	fixture := syncedDoneFixture(t)
	reports := 0

	var handled bool
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		var code int
		code, handled = fixture.done("Branchless goal done.", false, func(opts metrics.Options) (metrics.Result, error) {
			reports++
			fixture.checkReport(t, opts)
			return metrics.Result{}, nil
		}, func(string, goal.Endpoint) (string, error) {
			t.Fatal("branchless Done fetched endpoint")
			return "", nil
		})
		return code
	})
	if !handled || code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) || stderr != "" {
		t.Fatalf("branchless done read the unreadable remote: handled=%v code=%d stdout=%q stderr=%q", handled, code, stdout, stderr)
	}
	if reports != 1 {
		t.Fatalf("branchless Done requested %d reports, want 1", reports)
	}
	fixture.checkArchived(t)
}

func TestGoalDoneWithLocalBranchStillSweepsUnreadableRemote(t *testing.T) {
	fixture := syncedDoneFixture(t)
	fetched, cleaned, reports := false, false, 0
	endpointTip := func(root string, endpoint goal.Endpoint) (string, error) {
		if root != fixture.repository.root || endpoint.Remote != "local" || endpoint.Repository != fixture.repository {
			t.Fatalf("endpoint = %+v root=%q", endpoint, root)
		}
		temporary := ""
		return goalBranchEndpointTipWithGit(root, endpoint, func(got string, args ...string) (string, error) {
			if got != root {
				t.Fatalf("Git root = %q", got)
			}
			if len(args) == 5 && args[0] == "fetch" && args[1] == "--no-tags" && args[2] == "--refmap=" && args[3] == "local" && strings.HasPrefix(args[4], "+refs/heads/main:refs/metasystem/goals/endpoint/") && !fetched {
				temporary = strings.TrimPrefix(args[4], "+refs/heads/main:")
				fetched = true
				return "", errors.New("git fetch: unreadable remote")
			}
			if len(args) == 3 && args[0] == "update-ref" && args[1] == "-d" && args[2] == temporary && fetched && !cleaned {
				cleaned = true
				return "", nil
			}
			t.Fatalf("unexpected endpoint Git call: %v", args)
			return "", nil
		})
	}

	var handled bool
	code, _, stderr := captureCommandOutput(t, true, true, func() int {
		var code int
		code, handled = fixture.done("Branched goal done.", true, func(opts metrics.Options) (metrics.Result, error) {
			reports++
			fixture.checkReport(t, opts)
			return metrics.Result{}, nil
		}, endpointTip)
		return code
	})
	if !handled || code != 1 || !strings.Contains(stderr, "goal done confirmed but its branch was not swept: git fetch") {
		t.Fatalf("local branch bypassed the sweep refusal: handled=%v code=%d stderr=%q", handled, code, stderr)
	}
	if !fetched || !cleaned || reports != 0 {
		t.Fatalf("fetch=%v cleanup=%v reports=%d", fetched, cleaned, reports)
	}
	fixture.checkArchived(t)
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
	if err := testexec.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

type syncedDoneCommandFixture struct {
	repository   *proofAdmissionRepository
	dependencies syncRequestDependencies
	now          time.Time
	branchTip    string
	t            *testing.T
}

func syncedDoneFixture(t *testing.T) *syncedDoneCommandFixture {
	t.Helper()
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	file := &goal.GoalFile{
		Id: "synced-goal", State: goal.StateQueued, Intent: "Conclude through the synced command.", Origin: goal.OriginMain,
		NextStep: "Finish.", OpenedAt: "2026-08-20T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-20T00:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "mac-cli", "fixture"),
			Verb: "open", Actor: "mac-cli+fixture", Targets: []string{"synced-goal"}, Keep: -1,
		}},
	}
	repository.seed(map[string][]byte{"metasystem/plans/goals/synced-goal.md": goal.RenderFile(file)})
	return &syncedDoneCommandFixture{repository: repository, dependencies: repository.extendBudgetInputs(t), now: now, branchTip: repository.accepted, t: t}
}

func (f *syncedDoneCommandFixture) done(conclusion string, localPresent bool, reporter func(metrics.Options) (metrics.Result, error), endpointTip func(string, goal.Endpoint) (string, error)) (int, bool) {
	return trySyncMutationWithCompletion("done", []string{
		"--root", f.repository.root, "--id", "synced-goal", "--conclude", conclusion, "--lineage", "fixture",
	}, f.repository.commandNow(f.now), f.dependencies, goalParkBranchCheck, completionInputs{
		localTip: func(root, ref string) (string, bool, error) {
			if root != f.repository.root || ref != "refs/heads/goal/synced-goal" {
				f.t.Fatalf("local ref root=%q ref=%q", root, ref)
			}
			if localPresent {
				return f.branchTip, true, nil
			}
			return "", false, nil
		},
		endpointTip: endpointTip, reporter: reporter,
	})
}

func (f *syncedDoneCommandFixture) checkArchived(t *testing.T) {
	t.Helper()
	data := f.repository.rawFile(t, "metasystem/records/goals/synced-goal.md")
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file == nil || file.Id != "synced-goal" || file.State != goal.StateDone {
		t.Fatalf("accepted archived goal: file=%+v problems=%v", file, problems)
	}
}

func (f *syncedDoneCommandFixture) checkReport(t *testing.T, opts metrics.Options) {
	t.Helper()
	if opts.Root != f.repository.root || opts.GoalID != "synced-goal" || opts.PeriodEnd != "" || opts.Since != "" {
		t.Fatalf("report options = %+v", opts)
	}
	f.checkArchived(t)
}

func checkMetricsFixtureGuard(root, got string) error {
	if got != root {
		return fmt.Errorf("guard root = %q, want %q", got, root)
	}
	path := filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("guard is not executable: %s", path)
	}
	return nil
}

func TestGoalBranchEndpointTipUnreadableRemoteCleansTemporaryRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	metricsVerbGit(t, root, "init", "-q", "-b", "main")
	metricsVerbGit(t, root, "remote", "add", "local", filepath.Join(t.TempDir(), "unreadable.git"))
	_, err := goalBranchEndpointTip(root, goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch})
	if err == nil || !strings.Contains(err.Error(), "git fetch") {
		t.Fatalf("unreadable endpoint fetch error = %v", err)
	}
	if refs := metricsVerbGit(t, root, "for-each-ref", "--format=%(refname)", "refs/metasystem/goals/endpoint"); refs != "" {
		t.Fatalf("temporary endpoint refs remain: %q", refs)
	}
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
