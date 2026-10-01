package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

const batchProvenanceTestID = "01j5x00000000000000000baff"

type batchProvenanceBed struct {
	root, origin, baseCommit string
}

func TestBatchRecoveryFindsProductionProvenanceShapes(t *testing.T) {
	t.Parallel()
	change := strings.Repeat("a", 64)
	certified := strings.Repeat("b", 64)
	for _, test := range []struct {
		name, chain, provenance string
	}{
		{name: "ordinary", chain: "ordinary-chain", provenance: "chain=ordinary-chain change=" + change},
		{name: "direct-fix", chain: "carried-chain", provenance: "chain=carried-chain direct-fix class=register-carriage change=" + change + " certified-change=" + certified},
	} {
		t.Run(test.name, func(t *testing.T) {
			record, commits := recoverBatchProvenanceFixture(t,
				[]string{test.chain},
				[][]string{{"Landing-Provenance: " + test.provenance}},
			)
			if record.State != batch.StateLanded || !record.Units[0].P6Done || record.Units[0].LandedCommit != commits[0] {
				t.Fatalf("recovery record=%+v, want chain commit %s landed", record, commits[0])
			}
		})
	}
}

func TestBatchRecoveryRequiresExactChainField(t *testing.T) {
	t.Parallel()
	change := strings.Repeat("c", 64)
	for _, test := range []struct {
		name, trailer string
	}{
		{name: "longer-chain", trailer: "Landing-Provenance: chain=ab change=" + change},
		{name: "certified-change", trailer: "Landing-Provenance: direct-fix class=register-carriage change=" + change + " certified-change=a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record, _ := recoverBatchProvenanceFixture(t, []string{"a"}, [][]string{{test.trailer}})
			if record.State != batch.StateLanding || record.Units[0].P6Done || record.Units[0].LandedCommit != "" {
				t.Fatalf("non-matching field landed chain a: %+v", record)
			}
		})
	}
}

func TestBatchRecoveryResolvesEachChainToItsOwnCommit(t *testing.T) {
	t.Parallel()
	change := strings.Repeat("d", 64)
	record, commits := recoverBatchProvenanceFixture(t,
		[]string{"chain-a", "chain-b"},
		[][]string{
			{"Landing-Provenance: chain=chain-a change=" + change},
			{"Landing-Provenance: chain=chain-b change=" + change},
		},
	)
	if record.State != batch.StateLanded || record.Units[0].LandedCommit != commits[0] || record.Units[1].LandedCommit != commits[1] {
		t.Fatalf("series recovery record=%+v, want commits %v", record, commits)
	}
}

func TestBatchRecoveryUsesNewestMatchingCommit(t *testing.T) {
	t.Parallel()
	change := strings.Repeat("e", 64)
	record, commits := recoverBatchProvenanceFixture(t,
		[]string{"repeated-chain"},
		[][]string{
			{"Landing-Provenance: chain=repeated-chain change=" + change},
			{"Landing-Provenance: chain=repeated-chain direct-fix class=register-carriage change=" + change + " certified-change=" + strings.Repeat("f", 64)},
		},
	)
	if record.State != batch.StateLanded || record.Units[0].LandedCommit != commits[1] {
		t.Fatalf("repeated chain landed commit=%q, want newest %s", record.Units[0].LandedCommit, commits[1])
	}
}

func recoverBatchProvenanceFixture(t *testing.T, chains []string, commitTrailers [][]string) (batch.Record, []string) {
	t.Helper()
	root := t.TempDir()
	commits := make([]string, len(commitTrailers))
	var log strings.Builder
	for index := range commitTrailers {
		commits[index] = fmt.Sprintf("%040x", index+1)
	}
	for index := len(commitTrailers) - 1; index >= 0; index-- {
		log.WriteString(commits[index] + "\x00" + strings.Join(commitTrailers[index], "\n") + "\n\x00")
	}

	units := make([]batch.Unit, 0, len(chains))
	for index, chain := range chains {
		units = append(units, batch.Unit{
			GoalID: "goal-" + string(rune('a'+index)), Chain: chain, State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "landing", Lineage: lane.ClaimLineage, Epoch: 1, Revision: 1, AccountingRevision: 1},
		})
	}
	store := batch.NewStore(root, nil)
	record := batch.Record{
		Schema: 1, BatchID: batchProvenanceTestID, State: batch.StateLanding, Units: units,
		Landing: &batch.LandingProgress{PushComplete: true, PushedTip: commits[len(commits)-1]},
	}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	expected := make([]testgit.Expectation, len(chains))
	for index := range expected {
		expected[index] = testgit.Expectation{
			Call:   testgit.Call{Dir: root, Args: []string{"log", "--first-parent", "origin/main", "--format=%H%x00%B%x00"}},
			Result: testgit.Result{Stdout: []byte(log.String())},
		}
	}
	stub := testgit.New(t, expected...)
	gitRead := func(dir string, args ...string) (string, error) {
		result := stub.Run(testgit.Call{Dir: dir, Args: args})
		return string(result.Stdout), result.Err
	}
	seams := batchowner.RecoverySeams(root, batch.ModuleRoot(root), "", store, batchProvenanceTestID, time.Unix(3, 0), gitRead, &batchowner.BatchOwnerCalls, laneRecoveryInvocation)
	finalized := make(map[string]string)
	seams.Finalize = func(unit batch.Unit, commit string) error {
		validUnit, validCommit := false, false
		for index, chain := range chains {
			if unit.GoalID == "goal-"+string(rune('a'+index)) && unit.Chain == chain {
				validUnit = true
			}
		}
		for _, candidate := range commits {
			validCommit = validCommit || candidate == commit
		}
		if !validUnit || !validCommit || finalized[unit.GoalID] != "" {
			t.Fatalf("unexpected recovery finalization: unit=%+v commit=%q", unit, commit)
		}
		finalized[unit.GoalID] = commit
		return nil
	}
	cleanups := 0
	seams.Cleanup = func() error { cleanups++; return nil }
	if err := batch.RecoverPushedSeries(store, batchProvenanceTestID, lane.ClaimLineage, time.Unix(3, 0), seams); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Load(batchProvenanceTestID)
	if err != nil {
		t.Fatal(err)
	}
	landed := 0
	for _, unit := range recovered.Units {
		if unit.P6Done {
			landed++
			if finalized[unit.GoalID] != unit.LandedCommit {
				t.Fatalf("finalized commit for %s=%q, durable commit=%q", unit.GoalID, finalized[unit.GoalID], unit.LandedCommit)
			}
		} else if finalized[unit.GoalID] != "" {
			t.Fatalf("unrecognized unit %s was finalized", unit.GoalID)
		}
	}
	if len(finalized) != landed || cleanups != 0 && cleanups != 1 ||
		(cleanups == 1) != (landed == len(chains)) || (cleanups == 1) != recovered.Landing.CleanupDone ||
		(recovered.State == batch.StateLanded) != (landed == len(chains)) {
		t.Fatalf("recovery callbacks finalizations=%v cleanup=%d, record=%+v", finalized, cleanups, recovered)
	}
	return recovered, commits
}

func TestRecoveredBatchBranchFinalizationNamesLastSource(t *testing.T) {
	unit := batch.Unit{Chain: "branch-tip", CommitIDs: []string{"source-a", "source-b"}}
	if got := batchowner.RecoveredBatchNext(unit, "landed"); got != "landed commit:landed:source=source-b" {
		t.Fatalf("branch next=%q", got)
	}
	if got := batchowner.RecoveredBatchNext(batch.Unit{Chain: "chain-a"}, "landed"); got != "landed commit:landed:chain=chain-a" {
		t.Fatalf("chain next=%q", got)
	}
}

func TestBatchRecoveryLostFinalizeReplyDoesNotRepeatGoalEdit(t *testing.T) {
	root, upstream, _ := goalBranchCLIFixture(t, lane.ClaimLineage)
	goalPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	file, problems := goal.ParseFile(batchProvenanceContents(t, goalPath))
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	file.Claimed.Lineage = lane.ClaimLineage
	batchProvenanceWrite(t, goalPath, string(goal.RenderFile(file)), 0o644)
	batchProvenanceGit(t, "-C", root, "add", "plans/goals/standing-validation.md")
	batchProvenanceGit(t, "-C", root, "commit", "-qm", "handover fixture goal")
	batchProvenanceGit(t, "-C", root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	batchProvenanceGit(t, "-C", root, "update-ref", goal.AcceptedRef, "HEAD")
	batchProvenanceGit(t, "-C", root, "push", "-q", "upstream", "HEAD:main")
	batchProvenanceGit(t, "-C", root, "remote", "add", "origin", upstream)

	change := strings.Repeat("a", 64)
	batchProvenanceGit(t, "-C", root, "commit", "--allow-empty", "-qm", "land fixture",
		"--trailer", "Landing-Provenance: chain=chain-a change="+change)
	landed := batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD")
	batchProvenanceGit(t, "-C", root, "push", "-q", "upstream", "HEAD:main")
	batchProvenanceGit(t, "-C", root, "update-ref", "refs/remotes/origin/main", landed)

	store := batch.NewStore(root, nil)
	record := batch.Record{Schema: 1, BatchID: batchProvenanceTestID, State: batch.StateLanding,
		Units: []batch.Unit{{GoalID: "standing-validation", Chain: "chain-a", State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "landing", Lineage: lane.ClaimLineage, Epoch: 1, Revision: 3, AccountingRevision: 1}}},
		Landing: &batch.LandingProgress{PushComplete: true, PushedTip: landed}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}

	edits := 0
	realEdit := batchowner.BatchOwnerCalls.EditNext
	stubBatchOwnerCalls(t, func(invocation ownercall.Invocation, args ...string) error {
		if len(args) < 3 || args[0] != "internal" || args[1] != "goal" || args[2] != "edit" {
			return nil
		}
		edits++
		// The real owner runs in this process under the owner's context.
		if err := realEdit(invocation, args[4], args[6], args[8]); err != nil {
			return err
		}
		if edits == 1 {
			return errors.New("finalize reply lost after goal edit")
		}
		return nil
	})
	if err := recoverBatchLanding(root, store, time.Unix(3, 0)); err == nil {
		t.Fatal("lost finalization reply unexpectedly completed recovery")
	}
	if err := recoverBatchLanding(root, store, time.Unix(4, 0)); err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(goal.Endpoint{Root: root, Remote: "upstream", Branch: "refs/heads/main"}, true, time.Unix(5, 0))
	if err != nil {
		t.Fatal(err)
	}
	current := projection.Tree.Live["standing-validation"]
	rows := 0
	for _, history := range current.History {
		if history.Verb == "edit" {
			rows++
		}
	}
	if edits != 1 || rows != 1 || current.NextStep != batchowner.RecoveredBatchNext(record.Units[0], landed) {
		t.Fatalf("finalize calls=%d edit history rows=%d next=%q", edits, rows, current.NextStep)
	}
}

func TestBatchLastBranchLandingSweepsGoalBranch(t *testing.T) {
	for _, test := range []struct {
		name     string
		last     bool
		failOnce bool
		want     bool
	}{
		{name: "last deletes branch", last: true},
		{name: "through keeps branch", last: false, want: true},
		{name: "failed sweep retries", last: true, failOnce: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, upstream, base := goalBranchCLIFixture(t, "m1")
			exclude := batchProvenanceGit(t, "-C", root, "rev-parse", "--git-path", "info/exclude")
			if !filepath.IsAbs(exclude) {
				exclude = filepath.Join(root, exclude)
			}
			batchProvenanceWrite(t, exclude, "artifacts/agents/landing-batches/\nartifacts/agents/locks/landing-batches.lock\n", 0o644)
			batchProvenanceGit(t, "-C", root, "remote", "add", "origin", upstream)
			batchProvenanceWrite(t, filepath.Join(root, "metasystem", "batch-last.go"), "package fixture\n", 0o644)
			batchProvenanceGit(t, "-C", root, "add", "metasystem/batch-last.go")
			code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
				return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "last", "--root", root}, stdout, stderr)
			})
			unit := strings.TrimSpace(stdout)
			if code != 0 || stderr != "" || len(unit) != 40 {
				t.Fatalf("unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			code, _, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
				return goalBranchTestCommand([]string{"push", "--goal", "standing-validation", "--root", root, "--opid", "batch-last-push"}, stdout, stderr)
			})
			if code != 0 || stderr != "" {
				t.Fatalf("goal push: code=%d stderr=%q", code, stderr)
			}
			digest, err := goalbranch.UnitDigest(root, unit)
			if err != nil {
				t.Fatal(err)
			}
			message := "land batch member\n\nGoal-Unit: standing-validation/last\nGoal-Digest: " + digest + "\nGoal-Source: " + unit + "\n"
			if test.last {
				message += "Goal-Last: standing-validation\n"
			}
			landed := batchProvenanceGit(t, "-C", root, "commit-tree", unit+"^{tree}", "-p", base, "-m", message)
			batchProvenanceGit(t, "-C", root, "push", "-q", "upstream", landed+":refs/heads/main")
			batchProvenanceGit(t, "-C", root, "update-ref", "refs/remotes/origin/main", landed)

			store := batch.NewStore(root, nil)
			record := batch.Record{Schema: 1, BatchID: batchProvenanceTestID, State: batch.StateLanding,
				Units: []batch.Unit{{GoalID: "standing-validation", Chain: unit, State: batch.UnitJoined, CommitIDs: []string{unit},
					BranchTip: unit, LastUnit: "last", GoalLast: test.last,
					Claim: batch.Claim{Machine: "mac-cli", Lineage: "m1", Epoch: 1, Revision: 3, AccountingRevision: 1}}},
				Landing: &batch.LandingProgress{PushComplete: true, PushedTip: landed}}
			if err := store.Create(record); err != nil {
				t.Fatal(err)
			}
			originalSweep := batchowner.BatchGoalBranchSweep
			t.Cleanup(func() { batchowner.BatchGoalBranchSweep = originalSweep })
			stubBatchOwnerCalls(t, func(ownercall.Invocation, ...string) error { return nil })
			sweeps := 0
			if test.failOnce {
				batchowner.BatchGoalBranchSweep = func(request goalbranch.SweepRequest) (goalbranch.SweepResult, error) {
					sweeps++
					if sweeps == 1 {
						return goalbranch.SweepResult{}, errors.New("fixture sweep refusal")
					}
					return originalSweep(request)
				}
				firstErr := recoverBatchLanding(root, store, time.Unix(3, 0))
				first, loadErr := store.Load(batchProvenanceTestID)
				if loadErr != nil || firstErr == nil || !strings.Contains(firstErr.Error(), "fixture sweep refusal") || first.Units[0].P6Done {
					t.Fatalf("first recovery error=%v load=%v record=%+v", firstErr, loadErr, first)
				}
				if refs := batchProvenanceGit(t, "-C", root, "ls-remote", "--heads", "upstream", "refs/heads/goal/standing-validation"); refs == "" {
					t.Fatal("failed sweep deleted the goal branch")
				}
			}
			if err := recoverBatchLanding(root, store, time.Unix(3, 0)); err != nil {
				t.Fatal(err)
			}
			if test.failOnce && sweeps != 2 {
				t.Fatalf("sweep calls=%d, want 2", sweeps)
			}
			refs := batchProvenanceGit(t, "-C", root, "ls-remote", "--heads", "upstream", "refs/heads/goal/standing-validation")
			if present := refs != ""; present != test.want {
				t.Fatalf("goal branch present=%v, want %v: %s", present, test.want, refs)
			}
		})
	}
}

func batchProvenanceWrite(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func batchProvenanceContents(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func batchProvenanceGit(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// laneRecoveryInvocation is the authority a recovery's goal edits run under
// in these beds: the lane's claim lineage, from this process.
func laneRecoveryInvocation() (ownercall.Invocation, error) {
	return ownercall.FromThisProcess(lane.ClaimLineage), nil
}

// recoverBatchLanding runs the lane's landed-trailer recovery of the bed's
// batch through the production seams (lane design r10 §1 step 3), reading
// origin's main in the bed's checkout.
func recoverBatchLanding(root string, store batch.Store, at time.Time) error {
	seams := batchowner.RecoverySeams(root, batch.ModuleRoot(root), "", store, batchProvenanceTestID, at, batchowner.GitOutput, &batchowner.BatchOwnerCalls, laneRecoveryInvocation)
	return batch.RecoverPushedSeries(store, batchProvenanceTestID, lane.ClaimLineage, at, seams)
}
