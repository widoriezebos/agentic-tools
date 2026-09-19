package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const batchProvenanceTestID = "01j5x00000000000000000baff"

type batchProvenanceBed struct {
	root, origin, baseCommit string
}

func newBatchProvenanceBed(t *testing.T, verdict string, branchMember bool) batchProvenanceBed {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(t.TempDir(), "origin.git")
	batchProvenanceGit(t, "init", "-q", "--bare", origin)
	batchProvenanceGit(t, "init", "-q", "-b", "main", root)
	batchProvenanceGit(t, "-C", root, "config", "user.name", "Fixture")
	batchProvenanceGit(t, "-C", root, "config", "user.email", "fixture@example.invalid")
	batchProvenanceWrite(t, filepath.Join(root, "base.txt"), "base\n", 0o644)
	batchProvenanceGit(t, "-C", root, "add", "base.txt")
	batchProvenanceGit(t, "-C", root, "commit", "-qm", "base")
	baseCommit := batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD")
	baseTree := batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD^{tree}")
	batchProvenanceGit(t, "-C", root, "remote", "add", "origin", origin)
	batchProvenanceGit(t, "-C", root, "push", "-q", "-u", "origin", "main")

	batchProvenanceGit(t, "-C", root, "switch", "-q", "-c", "source")
	batchProvenanceWrite(t, filepath.Join(root, "member.txt"), "member\n", 0o644)
	batchProvenanceGit(t, "-C", root, "add", "member.txt")
	batchProvenanceGit(t, "-C", root, "commit", "-qm", "source")
	sourceCommit := batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD")
	sourceTree := batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD^{tree}")
	batchProvenanceGit(t, "-C", root, "switch", "-q", "main")

	batchProvenanceWrite(t, filepath.Join(root, "scripts", "receipt.sh"), "#!/usr/bin/env bash\nexit 0\n", 0o755)
	wrapper := `#!/usr/bin/env bash
set -euo pipefail
chain=
attested=
snapshot=
base=
while (( $# )); do
  case "$1" in
    --chain) chain=$2; shift 2 ;;
    --attested) attested=$2; shift 2 ;;
    --attested-snapshot) snapshot=$2; shift 2 ;;
    --attested-base) base=$2; shift 2 ;;
    --goal|--test-receipt) shift 2 ;;
    *) break ;;
  esac
done
printf 'chain=%s attested=%s snapshot=%s base=%s\n' "$chain" "$attested" "$snapshot" "$base" >wrapper.args
if [[ -n $attested ]]; then
  provenance="attested=$attested goal=goal-a unit=10b critic=critic-fake/1 change=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
else
  provenance="chain=$chain change=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
fi
git commit -q "$@" \
  --trailer "Landing-Provenance: $provenance" \
  --trailer "Landing-Provenance-Verdict: ` + verdict + `"
`
	batchProvenanceWrite(t, filepath.Join(root, "scripts", "agents", "commit.sh"), wrapper, 0o755)

	if !branchMember {
		patch := batchProvenanceGit(t, "-C", root, "diff", "--binary", "--full-index", baseCommit, sourceCommit)
		batchProvenanceWrite(t, filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", "chain-a", "diff.patch"), patch+"\n", 0o644)
	}

	store := batch.NewStore(root, nil)
	if _, err := batch.FindOrCreateOpen(store, baseTree, batchProvenanceTestID, landingOwnerLineage, time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	claim := batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 1, AccountingRevision: 1}
	unit := batch.Unit{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined, AuthorName: "Approver", AuthorEmail: "approver@example.invalid"}
	if branchMember {
		unit.BranchTip = sourceCommit
		unit.CommitIDs = []string{sourceCommit}
		unit.LastUnit = "10b"
		unit.Builds = []batch.BranchBuild{{Units: []string{"10b"}, Commit: sourceCommit, Digest: "fixture-digest"}}
	}
	if err := store.Update(batchProvenanceTestID, func(record *batch.Record) error {
		record.TipTree = sourceTree
		record.PrefixTrees = []string{sourceTree}
		record.Units = append(record.Units, unit)
		record.Proof = &batch.Proof{Status: "green", AttemptID: "fixture-proof"}
		record.Transition(batch.StateLanding, time.Unix(2, 0), "prove", landingOwnerLineage, "green")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return batchProvenanceBed{root: root, origin: origin, baseCommit: baseCommit}
}

func TestBatchBranchCommitRequiresPassingProvenance(t *testing.T) {
	original := batchChildRunner
	originalNext := batchRecoveryGoalNext
	originalRearm := batchRecoveryRearm
	t.Cleanup(func() {
		batchChildRunner, batchRecoveryGoalNext, batchRecoveryRearm = original, originalNext, originalRearm
	})
	batchChildRunner = func(string, string, ...string) error { return nil }
	batchRecoveryGoalNext = func(string, string, time.Time) (string, error) { return "", nil }
	batchRecoveryRearm = func(string, string) error { return nil }

	t.Run("would-refuse ejects without a push", func(t *testing.T) {
		bed := newBatchProvenanceBed(t, "would-refuse code=chain-not-implementation", true)
		batchProvenanceGit(t, "-C", bed.root, "config", "metasystem.goal.machine", "landing-machine")
		seat := t.TempDir()
		batchProvenanceGit(t, "init", "-q", seat)
		t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-18T10:00:00Z")
		originalAcquire, originalRequire, originalTick := batchOwnerAcquire, batchOwnerRequire, batchOwnerTick
		t.Cleanup(func() {
			batchOwnerAcquire, batchOwnerRequire, batchOwnerTick = originalAcquire, originalRequire, originalTick
		})
		batchOwnerAcquire = func(root string) (batchOwnerLease, error) {
			return batchOwnerLease{root: root, pid: int64(os.Getpid()), epoch: 1}, nil
		}
		batchOwnerRequire = func(batchOwnerLease) error { return nil }
		batchOwnerTick = func(_ *batch.Owner, id string) error {
			return executeBatchLanding(bed.root, id, landingOwnerLineage, time.Unix(3, 0))
		}
		code, _, stderr := captureCommandOutput(t, false, true, func() int {
			return runBatchTick([]string{"--root", seat, "--landing-root", bed.root, "--max-wait", "1m", "--batch", batchProvenanceTestID})
		})
		if code != 1 || !strings.Contains(stderr, "goal goal-a") || !strings.Contains(stderr, "BATCH_LAND_UNPROVENANCED") ||
			!strings.Contains(stderr, "re-prove before the next landing") {
			t.Fatalf("ejection command result code=%d stderr=%q", code, stderr)
		}
		originTip := batchProvenanceGit(t, "-C", bed.origin, "rev-parse", "refs/heads/main")
		if originTip != bed.baseCommit {
			t.Fatalf("would-refuse branch commit reached origin/main: got %s, want base %s", originTip, bed.baseCommit)
		}
		record, err := batch.NewStore(bed.root, nil).Load(batchProvenanceTestID)
		if err != nil {
			t.Fatal(err)
		}
		want := "BATCH_LAND_UNPROVENANCED: goal goal-a commit "
		if record.Units[0].Outcome != batch.UnitEjected || !strings.Contains(record.Units[0].Failure, want) ||
			!strings.Contains(record.Units[0].Failure, "verdict would-refuse code=chain-not-implementation") || record.State != batch.StateDissolved {
			t.Fatalf("would-refuse branch member was not ejected with provenance failure: %+v", record.Units[0])
		}
		if head := batchProvenanceGit(t, "-C", bed.root, "rev-parse", "HEAD"); head != bed.baseCommit {
			t.Fatalf("landing branch head=%s, want base %s", head, bed.baseCommit)
		}
		if branch := batchProvenanceGit(t, "-C", bed.root, "branch", "--show-current"); branch != "landing/"+batchProvenanceTestID {
			t.Fatalf("landing branch=%q after reset", branch)
		}
		if err := exec.Command("git", "-C", bed.origin, "show-ref", "--verify", "--quiet", "refs/heads/landing/"+batchProvenanceTestID).Run(); err == nil {
			t.Fatal("would-refuse branch commit pushed a landing ref")
		}
	})

	t.Run("pass lands", func(t *testing.T) {
		bed := newBatchProvenanceBed(t, "pass bar=a", true)
		if err := executeBatchLanding(bed.root, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err != nil {
			t.Fatal(err)
		}
		originTip := batchProvenanceGit(t, "-C", bed.origin, "rev-parse", "refs/heads/main")
		if originTip == bed.baseCommit || batchProvenanceGit(t, "-C", bed.origin, "show", originTip+":member.txt") != "member" {
			t.Fatalf("passing branch commit did not land: base=%s tip=%s", bed.baseCommit, originTip)
		}
		record, err := batch.NewStore(bed.root, nil).Load(batchProvenanceTestID)
		if err != nil || record.State != batch.StateLanded {
			t.Fatalf("passing branch record state=%s error=%v", record.State, err)
		}
		args := strings.TrimSpace(string(batchProvenanceContents(t, filepath.Join(bed.root, "wrapper.args"))))
		if strings.Contains(args, "chain=chain-a") || !strings.Contains(args, "attested="+record.Units[0].CommitIDs[0]) ||
			!strings.Contains(args, "snapshot="+record.Units[0].BranchTip) || !strings.Contains(args, "base="+bed.baseCommit) {
			t.Fatalf("branch declaration args=%q", args)
		}
	})
}

func TestBatchChainCommitKeepsWouldRefuseVerdictBehavior(t *testing.T) {
	original := batchChildRunner
	originalNext := batchRecoveryGoalNext
	originalRearm := batchRecoveryRearm
	t.Cleanup(func() {
		batchChildRunner, batchRecoveryGoalNext, batchRecoveryRearm = original, originalNext, originalRearm
	})
	batchChildRunner = func(string, string, ...string) error { return nil }
	batchRecoveryGoalNext = func(string, string, time.Time) (string, error) { return "", nil }
	batchRecoveryRearm = func(string, string) error { return nil }
	bed := newBatchProvenanceBed(t, "would-refuse code=chain-not-implementation", false)
	if err := executeBatchLanding(bed.root, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	originTip := batchProvenanceGit(t, "-C", bed.origin, "rev-parse", "refs/heads/main")
	if originTip == bed.baseCommit {
		t.Fatal("chain member with a would-refuse verdict did not retain its landing behavior")
	}
	verdict := batchProvenanceGit(t, "-C", bed.origin, "show", "-s", "--format=%(trailers:key=Landing-Provenance-Verdict,valueonly)", originTip)
	if verdict != "would-refuse code=chain-not-implementation" {
		t.Fatalf("chain member verdict=%q", verdict)
	}
	record, err := batch.NewStore(bed.root, nil).Load(batchProvenanceTestID)
	if err != nil || record.State != batch.StateLanded {
		t.Fatalf("chain member record state=%s error=%v", record.State, err)
	}
}

func TestBatchRecoveryFindsProductionProvenanceShapes(t *testing.T) {
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

func TestBatchMixedBranchAndChainMembersLandInBothOrders(t *testing.T) {
	original := batchChildRunner
	originalNext := batchRecoveryGoalNext
	originalRearm := batchRecoveryRearm
	t.Cleanup(func() {
		batchChildRunner, batchRecoveryGoalNext, batchRecoveryRearm = original, originalNext, originalRearm
	})
	batchChildRunner = func(string, string, ...string) error { return nil }
	batchRecoveryGoalNext = func(string, string, time.Time) (string, error) { return "", nil }
	batchRecoveryRearm = func(string, string) error { return nil }
	for _, branchFirst := range []bool{true, false} {
		name := "chain then branch"
		if branchFirst {
			name = "branch then chain"
		}
		t.Run(name, func(t *testing.T) {
			bed := newBatchProvenanceBed(t, "pass bar=e", true)
			store := batch.NewStore(bed.root, nil)
			record, err := store.Load(batchProvenanceTestID)
			if err != nil {
				t.Fatal(err)
			}
			branchUnit := record.Units[0]
			digest, err := goalbranch.UnitDigest(bed.root, branchUnit.CommitIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			branchUnit.Builds[0].Digest = digest
			batchProvenanceGit(t, "-C", bed.root, "switch", "-q", "--detach", bed.baseCommit)
			batchProvenanceWrite(t, filepath.Join(bed.root, "chain.txt"), "chain\n", 0o644)
			batchProvenanceGit(t, "-C", bed.root, "add", "chain.txt")
			batchProvenanceGit(t, "-C", bed.root, "commit", "-qm", "chain source")
			chainCommit := batchProvenanceGit(t, "-C", bed.root, "rev-parse", "HEAD")
			patch := batchProvenanceGit(t, "-C", bed.root, "diff", "--binary", "--full-index", bed.baseCommit, chainCommit)
			batchProvenanceWrite(t, filepath.Join(bed.root, "artifacts", "agents", "landing-batches", "chains", "chain-b", "diff.patch"), patch+"\n", 0o644)
			chainUnit := batch.Unit{GoalID: "goal-chain", Chain: "chain-b", Claim: branchUnit.Claim, State: batch.UnitJoined,
				AuthorName: "Approver", AuthorEmail: "approver@example.invalid"}
			workspace := gittree.Workspace{Dir: bed.root}
			baseTree := record.BaseTree
			chainTree, err := workspace.Apply(baseTree, []byte(patch+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			member := batch.BranchMember{GoalID: branchUnit.GoalID, Tip: branchUnit.BranchTip, Last: branchUnit.GoalLast, Builds: branchUnit.Builds}
			branchTrees, err := batch.AssembleBranchMembers(bed.root, baseTree, []batch.BranchMember{member})
			if err != nil {
				t.Fatal(err)
			}
			units, prefixes := []batch.Unit{chainUnit, branchUnit}, []string{chainTree}
			var finalTrees []string
			if branchFirst {
				units, prefixes = []batch.Unit{branchUnit, chainUnit}, []string{branchTrees[0]}
				final, applyErr := workspace.Apply(branchTrees[0], []byte(patch+"\n"))
				err, finalTrees = applyErr, []string{final}
			} else {
				finalTrees, err = batch.AssembleBranchMembers(bed.root, chainTree, []batch.BranchMember{member})
			}
			if err != nil {
				t.Fatal(err)
			}
			prefixes = append(prefixes, finalTrees[0])
			record.Units, record.PrefixTrees, record.TipTree = units, prefixes, prefixes[1]
			record.Receipts = map[string]batch.PrefixReceipt{units[0].GoalID: {GoalID: units[0].GoalID, Tree: prefixes[0]}}
			data, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			batchProvenanceWrite(t, filepath.Join(bed.root, "artifacts", "agents", "landing-batches", batchProvenanceTestID+".json"), string(append(data, '\n')), 0o644)
			if err := executeBatchLanding(bed.root, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err != nil {
				t.Fatal(err)
			}
			tip := batchProvenanceGit(t, "-C", bed.origin, "rev-parse", "refs/heads/main")
			if batchProvenanceGit(t, "-C", bed.origin, "show", tip+":member.txt") != "member" || batchProvenanceGit(t, "-C", bed.origin, "show", tip+":chain.txt") != "chain" {
				t.Fatalf("mixed landing tip=%s", tip)
			}
		})
	}
}

func TestBatchRecoveryResolvesEachChainToItsOwnCommit(t *testing.T) {
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
	origin := filepath.Join(t.TempDir(), "origin.git")
	batchProvenanceGit(t, "init", "-q", "--bare", origin)
	batchProvenanceGit(t, "init", "-q", "-b", "main", root)
	batchProvenanceGit(t, "-C", root, "config", "user.name", "Fixture")
	batchProvenanceGit(t, "-C", root, "config", "user.email", "fixture@example.invalid")
	batchProvenanceGit(t, "-C", root, "commit", "--allow-empty", "-qm", "base")
	batchProvenanceGit(t, "-C", root, "remote", "add", "origin", origin)
	batchProvenanceGit(t, "-C", root, "push", "-q", "-u", "origin", "main")

	commits := make([]string, 0, len(commitTrailers))
	for index, trailers := range commitTrailers {
		args := []string{"-C", root, "commit", "--allow-empty", "-qm", "landing fixture " + string(rune('a'+index))}
		for _, trailer := range trailers {
			args = append(args, "--trailer", trailer)
		}
		batchProvenanceGit(t, args...)
		commits = append(commits, batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD"))
	}
	batchProvenanceGit(t, "-C", root, "push", "-q", "origin", "main")

	units := make([]batch.Unit, 0, len(chains))
	for index, chain := range chains {
		units = append(units, batch.Unit{
			GoalID: "goal-" + string(rune('a'+index)), Chain: chain, State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 1, AccountingRevision: 1},
		})
	}
	store := batch.NewStore(root, nil)
	record := batch.Record{
		Schema: 1, BatchID: batchProvenanceTestID, State: batch.StateLanding, Units: units,
		Landing: &batch.LandingProgress{PushComplete: true, PushedTip: batchProvenanceGit(t, "-C", root, "rev-parse", "HEAD")},
	}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}

	original := batchChildRunner
	originalNext := batchRecoveryGoalNext
	originalRearm := batchRecoveryRearm
	t.Cleanup(func() {
		batchChildRunner, batchRecoveryGoalNext, batchRecoveryRearm = original, originalNext, originalRearm
	})
	batchChildRunner = func(string, string, ...string) error { return nil }
	batchRecoveryGoalNext = func(string, string, time.Time) (string, error) { return "", nil }
	batchRecoveryRearm = func(string, string) error { return nil }
	if err := recoverBatchLanding(root, store, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Load(batchProvenanceTestID)
	if err != nil {
		t.Fatal(err)
	}
	return recovered, commits
}

func TestRecoveredBatchBranchFinalizationNamesLastSource(t *testing.T) {
	unit := batch.Unit{Chain: "branch-tip", CommitIDs: []string{"source-a", "source-b"}}
	if got := recoveredBatchNext(unit, "landed"); got != "landed commit:landed:source=source-b" {
		t.Fatalf("branch next=%q", got)
	}
	if got := recoveredBatchNext(batch.Unit{Chain: "chain-a"}, "landed"); got != "landed commit:landed:chain=chain-a" {
		t.Fatalf("chain next=%q", got)
	}
}

func TestBatchRecoveryLostFinalizeReplyDoesNotRepeatGoalEdit(t *testing.T) {
	root, upstream, _ := goalBranchCLIFixture(t, landingOwnerLineage)
	goalPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	file, problems := goal.ParseFile(batchProvenanceContents(t, goalPath))
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	file.Claimed.Lineage = landingOwnerLineage
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
			Claim: batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 3, AccountingRevision: 1}}},
		Landing: &batch.LandingProgress{PushComplete: true, PushedTip: landed}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}

	original := batchChildRunner
	originalRearm := batchRecoveryRearm
	t.Cleanup(func() { batchChildRunner, batchRecoveryRearm = original, originalRearm })
	batchRecoveryRearm = func(string, string) error { return nil }
	edits := 0
	batchChildRunner = func(_ string, _ string, args ...string) error {
		if len(args) < 2 || args[0] != "goal" || args[1] != "edit" {
			return nil
		}
		edits++
		code, _, stderr := captureCommandOutput(t, true, true, func() int { return runGoalEdit(args[2:]) })
		if code != 0 {
			return errors.New(stderr)
		}
		if edits == 1 {
			return errors.New("finalize reply lost after goal edit")
		}
		return nil
	}
	if err := recoverBatchLanding(root, store, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err == nil {
		t.Fatal("lost finalization reply unexpectedly completed recovery")
	}
	if err := recoverBatchLanding(root, store, batchProvenanceTestID, landingOwnerLineage, time.Unix(4, 0)); err != nil {
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
	if edits != 1 || rows != 1 || current.NextStep != recoveredBatchNext(record.Units[0], landed) {
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
			originalRearm := batchRecoveryRearm
			t.Cleanup(func() { batchRecoveryRearm = originalRearm })
			batchRecoveryRearm = func(string, string) error { return nil }
			root, upstream, base := goalBranchCLIFixture(t, "m1")
			exclude := batchProvenanceGit(t, "-C", root, "rev-parse", "--git-path", "info/exclude")
			if !filepath.IsAbs(exclude) {
				exclude = filepath.Join(root, exclude)
			}
			batchProvenanceWrite(t, exclude, "artifacts/agents/landing-batches/\nartifacts/agents/locks/landing-batches.lock\n", 0o644)
			batchProvenanceGit(t, "-C", root, "remote", "add", "origin", upstream)
			batchProvenanceWrite(t, filepath.Join(root, "metasystem", "batch-last.go"), "package fixture\n", 0o644)
			batchProvenanceGit(t, "-C", root, "add", "metasystem/batch-last.go")
			code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
				return runGoalBranch([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "last", "--root", root})
			})
			unit := strings.TrimSpace(stdout)
			if code != 0 || stderr != "" || len(unit) != 40 {
				t.Fatalf("unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			code, _, stderr = captureCommandOutput(t, true, true, func() int {
				return runGoalBranch([]string{"push", "--goal", "standing-validation", "--root", root, "--opid", "batch-last-push"})
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
			original := batchChildRunner
			originalSweep := batchGoalBranchSweep
			t.Cleanup(func() { batchChildRunner, batchGoalBranchSweep = original, originalSweep })
			batchChildRunner = func(string, string, ...string) error { return nil }
			sweeps := 0
			if test.failOnce {
				batchGoalBranchSweep = func(request goalbranch.SweepRequest) (goalbranch.SweepResult, error) {
					sweeps++
					if sweeps == 1 {
						return goalbranch.SweepResult{}, errors.New("fixture sweep refusal")
					}
					return originalSweep(request)
				}
				firstErr := recoverBatchLanding(root, store, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0))
				first, loadErr := store.Load(batchProvenanceTestID)
				if loadErr != nil || firstErr == nil || !strings.Contains(firstErr.Error(), "fixture sweep refusal") || first.Units[0].P6Done {
					t.Fatalf("first recovery error=%v load=%v record=%+v", firstErr, loadErr, first)
				}
				if refs := batchProvenanceGit(t, "-C", root, "ls-remote", "--heads", "upstream", "refs/heads/goal/standing-validation"); refs == "" {
					t.Fatal("failed sweep deleted the goal branch")
				}
			}
			if err := recoverBatchLanding(root, store, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0)); err != nil {
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
