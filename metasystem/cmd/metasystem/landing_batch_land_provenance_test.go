package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
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
	batchProvenanceWrite(t, filepath.Join(root, "plans", "goals", "backlog.md"), string(goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
	})), 0o644)
	budget, err := goal.NewBudget("100h", 10, 1000, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	opened, claimed := "2026-09-18T08:00:00Z", "2026-09-18T09:00:00Z"
	for index, id := range []string{"goal-a", "goal-chain"} {
		file := &goal.GoalFile{Id: id, State: goal.StateClaimed, Intent: "land " + id,
			Origin: goal.OriginHuman, OpenedAt: opened, Revision: 2, Budget: &budget,
			Claimed: &goal.ClaimRecord{Machine: "landing", Lineage: landingOwnerLineage, At: claimed,
				Revision: 2, AccountingRevision: 2, EpisodeAt: claimed, EpisodeRevision: 2,
				HandedOver: goal.HandedOver{FromMachine: "seat", FromLineage: "seat-lineage", FromEpoch: 1, Batch: batchProvenanceTestID}},
			History: []goal.HistoryLine{
				{At: opened, Opid: batchE2EOpid(20+index*2, "human", "wido"), Verb: "open", Actor: "human:wido", Keep: -1},
				{At: claimed, Opid: batchE2EOpid(21+index*2, "landing", landingOwnerLineage), Verb: "claim", Actor: "landing+" + landingOwnerLineage, Keep: -1},
			},
		}
		batchProvenanceWrite(t, filepath.Join(root, "plans", "goals", id+".md"), string(goal.RenderFile(file)), 0o644)
	}
	batchProvenanceGit(t, "-C", root, "add", "base.txt", "plans/goals")
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
	claim := batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
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
		record.Seal = map[string]batch.Claim{
			"goal-a":     {Revision: 2, AccountingRevision: 2},
			"goal-chain": {Revision: 2, AccountingRevision: 2},
		}
		record.Units = append(record.Units, unit)
		record.Proof = &batch.Proof{Status: "green", AttemptID: "fixture-proof"}
		record.Transition(batch.StateLanding, time.Unix(2, 0), "prove", landingOwnerLineage, "green")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return batchProvenanceBed{root: root, origin: origin, baseCommit: baseCommit}
}

// These fixtures exercise the production commit/provenance and endpoint path
// from an already green batch. Prefix policy/execution is covered separately;
// replanning it here would turn a synthetic proof into an unrelated native run.
func landBatchProvenanceBed(t *testing.T, bed batchProvenanceBed) error {
	t.Helper()
	store := batch.NewStore(bed.root, nil)
	record, err := store.Load(batchProvenanceTestID)
	if err != nil {
		return err
	}
	seams := batchLandSeams(bed.root, batchProvenanceTestID, record, bed.baseCommit, landingOwnerLineage)
	seams.VerifySeries = func(units []batch.Unit, _ map[string]string) error {
		if len(units) == 0 || record.Proof == nil || record.Proof.Status != "green" {
			return errors.New("provenance fixture has no declared green series")
		}
		return nil
	}
	at := time.Unix(3, 0)
	if err := batch.LandSeries(store, batchProvenanceTestID, landingOwnerLineage, at, seams); err != nil {
		return err
	}
	return finishBatchLanding(bed.root, store, batchProvenanceTestID, landingOwnerLineage, at)
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
			if id != batchProvenanceTestID {
				return errors.New("provenance fixture received another batch")
			}
			return landBatchProvenanceBed(t, bed)
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
		if err := landBatchProvenanceBed(t, bed); err != nil {
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
	if err := landBatchProvenanceBed(t, bed); err != nil {
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

func TestBatchLandingVerdictPolicyWithoutGit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, verdict string
		branch        bool
		refused       bool
	}{
		{name: "branch would-refuse ejects", branch: true, verdict: "would-refuse code=chain-not-implementation", refused: true},
		{name: "branch pass pushes", branch: true, verdict: "pass bar=a"},
		{name: "chain keeps would-refuse behavior", verdict: "would-refuse code=chain-not-implementation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
				t.Fatalf("fixture contains Git metadata: %v", err)
			}

			wrapper := filepath.Join(root, "scripts", "agents", "commit.sh")
			argsFile := filepath.Join(root, "wrapper.args")
			if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := testexec.WriteFile(wrapper, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> wrapper.args\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			store := batch.NewStore(root, nil).WithReassembly(
				func(string, []batch.Unit) ([]string, error) { t.Fatal("unexpected survivor assembly"); return nil, nil },
				func(string, string) error { t.Fatal("unexpected branch deletion"); return nil },
				func(string, string, string, string, []batch.Unit) (string, error) {
					t.Fatal("unexpected branch rebuild")
					return "", nil
				},
			)
			at := time.Unix(3, 0)
			if _, err := batch.FindOrCreateOpen(store, "base-tree", batchProvenanceTestID, landingOwnerLineage, at); err != nil {
				t.Fatal(err)
			}
			unit := batch.Unit{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
				Claim:      batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2},
				AuthorName: "Approver", AuthorEmail: "approver@example.invalid"}
			if tc.branch {
				unit.BranchTip = "source-tip"
				unit.CommitIDs = []string{"source-commit"}
				unit.Builds = []batch.BranchBuild{{Units: []string{"unit-a"}, Commit: "source-commit"}}
			}
			if err := store.Update(batchProvenanceTestID, func(record *batch.Record) error {
				record.Units = []batch.Unit{unit}
				record.TipTree = "tip-tree"
				record.PrefixTrees = []string{"tip-tree"}
				record.Proof = &batch.Proof{Status: "green", AttemptID: "green-proof"}
				record.Transition(batch.StateLanding, at, "prove", landingOwnerLineage, "green")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			record, err := store.Load(batchProvenanceTestID)
			if err != nil {
				t.Fatal(err)
			}

			reads := []struct {
				args  []string
				value string
			}{
				{[]string{"rev-parse", "HEAD"}, "base"},
				{[]string{"rev-parse", "HEAD"}, "landed"},
				{[]string{"rev-parse", "landed^"}, "base"},
				{[]string{"rev-parse", "HEAD"}, "landed"},
			}
			if tc.branch {
				reads = append(reads, struct {
					args  []string
					value string
				}{
					[]string{"show", "-s", "--format=%(trailers:key=Landing-Provenance-Verdict,valueonly)", "landed"}, tc.verdict,
				})
			}
			readCount := 0
			readGit := func(gitRoot string, args ...string) (string, error) {
				if readCount >= len(reads) || gitRoot != root || !reflect.DeepEqual(args, reads[readCount].args) {
					t.Fatalf("unexpected Git read %d: root=%q args=%v; chain trailer would be %q", readCount, gitRoot, args, tc.verdict)
				}
				if readCount == 0 {
					if _, err := os.Stat(argsFile); !os.IsNotExist(err) {
						t.Fatalf("wrapper ran before pre-HEAD read: %v", err)
					}
				} else if _, err := os.Stat(argsFile); err != nil {
					t.Fatalf("wrapper has not run at Git read %d: %v", readCount, err)
				}
				value := reads[readCount].value
				readCount++
				return value, nil
			}
			seams := batchLandSeamsWithRead(root, batchProvenanceTestID, record, "base", landingOwnerLineage, readGit)
			calls := map[string]int{}
			call := func(name string) { calls[name]++ }
			seams.Origin = func() (string, error) { call("origin"); return "base", nil }
			seams.OriginTree = func(string) (string, error) { t.Fatal("unexpected origin tree read"); return "", nil }
			seams.Prepare = func(base string) error {
				if base != "base-tree" {
					t.Fatalf("prepare base=%q", base)
				}
				call("prepare")
				return nil
			}
			seams.Apply = func(got batch.Unit) error {
				if got.GoalID != unit.GoalID {
					t.Fatalf("apply unit=%q", got.GoalID)
				}
				call("apply")
				return nil
			}
			seams.AppendReceipt = func(got batch.Unit, receipt batch.PrefixReceipt) error {
				if got.GoalID != unit.GoalID || receipt.Tree != "tip-tree" {
					t.Fatalf("chain receipt=%+v", receipt)
				}
				call("receipt")
				return nil
			}
			seams.ApplyBuild = func(got batch.Unit, build batch.BranchBuild) error {
				if got.GoalID != unit.GoalID || build.Commit != "source-commit" {
					t.Fatalf("build=%+v", build)
				}
				call("apply-build")
				return nil
			}
			seams.AppendBuildReceipt = func(got batch.Unit, build batch.BranchBuild, receipt batch.PrefixReceipt) error {
				if got.GoalID != unit.GoalID || build.Commit != "source-commit" || receipt.Tree != "tip-tree" {
					t.Fatalf("build receipt=%+v", receipt)
				}
				call("build-receipt")
				return nil
			}
			seams.Held = func(base, tip string) error {
				if base != "base-tree" || tip != "landed" {
					t.Fatalf("held base=%q tip=%q", base, tip)
				}
				call("held")
				return nil
			}
			seams.VerifySeries = func(units []batch.Unit, commits map[string]string) error {
				if len(units) != 1 || units[0].GoalID != unit.GoalID || commits[unit.GoalID] != "landed" {
					t.Fatalf("verify units=%+v commits=%v", units, commits)
				}
				call("verify")
				return nil
			}
			seams.PublishBranch = func(expected, tip string) error {
				if expected != "" || tip != "landed" {
					t.Fatalf("publish expected=%q tip=%q", expected, tip)
				}
				call("publish")
				return nil
			}
			seams.Push = func(base, tip string) error {
				if base != "base-tree" || tip != "landed" {
					t.Fatalf("push base=%q tip=%q", base, tip)
				}
				call("push")
				return nil
			}
			seams.Reset = func(base string) error {
				if base != "base-tree" {
					t.Fatalf("reset base=%q", base)
				}
				call("reset")
				return nil
			}
			seams.Cleanup = func() error { call("cleanup"); return nil }
			seams.Abandon = func(string, string) error { t.Fatal("unexpected abandon"); return nil }
			seams.SeriesOnOrigin = func(string, string) (bool, error) { t.Fatal("unexpected origin membership check"); return false, nil }
			seams.RecoverPush = func(string, string, string) (batch.PushRecovery, error) {
				t.Fatal("unexpected push recovery")
				return batch.PushRecovery{}, nil
			}

			landErr := batch.LandSeries(store, batchProvenanceTestID, landingOwnerLineage, at, seams)
			landedRecord, loadErr := store.Load(batchProvenanceTestID)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if tc.refused {
				if landErr == nil || !strings.Contains(landErr.Error(), "BATCH_LAND_UNPROVENANCED") ||
					landedRecord.State != batch.StateDissolved || landedRecord.Units[0].State != batch.UnitReturnPending ||
					landedRecord.Units[0].Outcome != batch.UnitEjected || !strings.Contains(landedRecord.Units[0].Failure, tc.verdict) {
					t.Fatalf("branch refusal error=%v record=%+v", landErr, landedRecord)
				}
			} else if landErr != nil || landedRecord.State != batch.StateLanding || landedRecord.Landing == nil || !landedRecord.Landing.PushComplete || landedRecord.Landing.Commits[unit.GoalID] != "landed" || landedRecord.Units[0].State != batch.UnitJoined {
				t.Fatalf("passing landing error=%v record=%+v", landErr, landedRecord)
			}
			wantCalls := map[string]int{"origin": 1, "prepare": 1}
			if tc.branch {
				wantCalls["apply-build"], wantCalls["build-receipt"] = 1, 1
			} else {
				wantCalls["apply"], wantCalls["receipt"] = 1, 1
			}
			if tc.refused {
				wantCalls["reset"] = 1
			} else {
				wantCalls["publish"], wantCalls["held"], wantCalls["verify"], wantCalls["push"], wantCalls["cleanup"] = 1, 1, 1, 1, 1
			}
			if !reflect.DeepEqual(calls, wantCalls) || readCount != len(reads) {
				t.Fatalf("effects=%v want=%v Git reads=%d want=%d", calls, wantCalls, readCount, len(reads))
			}
			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(args)), "\n")
			declaration := []string{"--chain", "chain-a"}
			if tc.branch {
				declaration = []string{"--attested", "source-commit", "--attested-snapshot", "source-tip", "--attested-base", "base"}
			}
			wantPrefix := append(declaration, "--goal", "goal-a", "--test-receipt", filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", batchProvenanceTestID+".json"), "-F")
			if len(lines) != len(wantPrefix)+1 || !reflect.DeepEqual(lines[:len(wantPrefix)], wantPrefix) || !strings.HasPrefix(lines[len(wantPrefix)], filepath.Join(root, ".batch-commit-message-")) {
				t.Fatalf("wrapper argv=%q want prefix=%q and message path", lines, wantPrefix)
			}
		})
	}
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
			if err := landBatchProvenanceBed(t, bed); err != nil {
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
			Claim: batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 1, AccountingRevision: 1},
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
	seams := batchRecoverySeamsWithGit(root, store, batchProvenanceTestID, time.Unix(3, 0), gitRead)
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
	rearms, cleanups := 0, 0
	seams.Rearm = func(tip string) error {
		if tip != commits[len(commits)-1] {
			t.Fatalf("rearm tip=%q, want %q", tip, commits[len(commits)-1])
		}
		rearms++
		return nil
	}
	seams.Cleanup = func() error { cleanups++; return nil }
	if err := batch.RecoverPushedSeries(store, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0), seams); err != nil {
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
	if len(finalized) != landed || rearms != 0 && rearms != 1 || cleanups != rearms ||
		(rearms == 1) != (landed == len(chains)) ||
		(rearms == 1) != recovered.Landing.RearmComplete || (cleanups == 1) != recovered.Landing.CleanupDone ||
		(recovered.State == batch.StateLanded) != (landed == len(chains)) {
		t.Fatalf("recovery callbacks finalizations=%v rearm=%d cleanup=%d, record=%+v", finalized, rearms, cleanups, recovered)
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
