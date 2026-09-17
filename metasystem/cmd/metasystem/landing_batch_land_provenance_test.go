package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
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
while (( $# )); do
  case "$1" in
    --chain) chain=$2; shift 2 ;;
    --goal|--test-receipt) shift 2 ;;
    *) break ;;
  esac
done
git commit -q "$@" \
  --trailer "Landing-Provenance: chain=$chain" \
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
	t.Cleanup(func() { batchChildRunner = original })
	batchChildRunner = func(string, string, ...string) error { return nil }

	t.Run("would-refuse ejects without a push", func(t *testing.T) {
		bed := newBatchProvenanceBed(t, "would-refuse code=chain-not-implementation", true)
		_ = executeBatchLanding(bed.root, batchProvenanceTestID, landingOwnerLineage, time.Unix(3, 0))
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
	})
}

func TestBatchChainCommitKeepsWouldRefuseVerdictBehavior(t *testing.T) {
	original := batchChildRunner
	t.Cleanup(func() { batchChildRunner = original })
	batchChildRunner = func(string, string, ...string) error { return nil }
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

func batchProvenanceWrite(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
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
