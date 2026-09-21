package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/trunkredmap"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEBatchSplitNativeRedReprovesAndPublishesIndependentSurvivor(t *testing.T) {
	fixture := newPortableProofFixture(t)
	fixture.contract.Groups = append(fixture.contract.Groups, fixture.group("app-b", "b"))
	fixture.contract.Surfaces = []testpolicy.Surface{
		{ID: "app-a", Paths: []string{"app/a.txt"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}},
		{ID: "app-b", Paths: []string{"app/b.txt"}, Standard: []string{"app-b"}, Critical: []string{"app-b-observed"}},
		{ID: "control", Paths: []string{"testing.json", "plans/goals/**", "scripts/**"}, Standard: []string{"app-a"}},
	}
	fixture.contract.Cadence = []string{"app-a", "app-b"}
	fixture.write("app/b.txt", "green b\n", 0o644)
	fixture.writeContract()
	fixture.git("add", "-A")
	fixture.git("commit", "-qm", "declare two independent checks")
	baseCommit := fixture.git("rev-parse", "HEAD")
	baseTree := fixture.git("rev-parse", "HEAD^{tree}")
	fixture.git("update-ref", "refs/remotes/origin/main", baseCommit)
	fixture.git("update-ref", goal.AcceptedRef, baseCommit)
	origin := filepath.Join(t.TempDir(), "origin.git")
	batchE2EGit(t, "", "init", "-q", "--bare", origin)
	fixture.git("remote", "add", "origin", origin)
	fixture.git("push", "-q", "origin", baseCommit+":refs/heads/main")

	fixture.write("app/a.txt", "red a\n", 0o644)
	patchA := fixture.gitBytes("diff", "--binary", "HEAD", "--", "app/a.txt")
	fixture.git("add", "app/a.txt")
	fixture.git("commit", "-qm", "A fails its native check")
	firstTree := fixture.git("rev-parse", "HEAD^{tree}")
	fixture.write("app/b.txt", "green b changed\n", 0o644)
	patchB := fixture.gitBytes("diff", "--binary", "HEAD", "--", "app/b.txt")
	fixture.git("add", "app/b.txt")
	fixture.git("commit", "-qm", "B stays green")
	tipTree := fixture.git("rev-parse", "HEAD^{tree}")
	for chain, patch := range map[string][]byte{"chain-a": patchA, "chain-b": patchB} {
		fixture.writeBytes("artifacts/agents/landing-batches/chains/"+chain+"/diff.patch", patch, 0o644)
	}
	readResult := func(path string) proofrun.TestResult {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var result proofrun.TestResult
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	const batchID = "01j5x00000000000000000ba01"
	tipPath := filepath.Join(t.TempDir(), "tip.json")
	status, output := fixture.command("test", "run", "--root", fixture.root, "--goal", "goal-b", "--tree", tipTree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-a", "app-b"}), "--result", tipPath)
	if status == 0 {
		t.Fatalf("native red tip passed: %s", output)
	}
	tip := readResult(tipPath)
	red := trunkredmap.ResultToRedGroups(tip)
	if len(red) != 1 || red[0].ID != "app-a" || tip.Delivery.Sufficient {
		t.Fatalf("native tip failure was not isolated to A: red=%+v delivery=%+v", red, tip.Delivery)
	}
	claim := batch.Claim{Machine: "portable", Lineage: "portable-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	store := batch.NewStore(fixture.root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateProving, TipTree: tipTree,
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined, SelectedGroups: []string{"app-a"}, ChangedPaths: []string{"app/a.txt"}},
			{GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoined, SelectedGroups: []string{"app-b"}, ChangedPaths: []string{"app/b.txt"}}},
		Proof:    &batch.Proof{Status: "planned", Tree: tipTree, SelectedGroups: []string{"app-a", "app-b"}},
		BaseTree: baseTree, PrefixTrees: []string{firstTree, tipTree}}); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(10, 0)
	if err := batch.FinishProof(store, batchID, "owner", tip, errors.New("native test failed"), at); err != nil {
		t.Fatal(err)
	}
	redRecord, err := store.Load(batchID)
	if err != nil || redRecord.State != batch.StateDiagnosing || len(redRecord.Proof.RedGroups) != 1 {
		t.Fatalf("native red proof was not persisted: record=%+v error=%v", redRecord, err)
	}
	diagnosticPath := filepath.Join(t.TempDir(), "diagnostic.json")
	if err := batch.DiagnoseRed(store, batchID, "owner", redRecord.Proof.RedGroups, "", at.Add(time.Second), batch.RedSeams{
		Run: func(request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
			if request.Tree != baseTree || !request.NeverReuse {
				t.Fatalf("base diagnostic was not fresh: %+v", request)
			}
			status, output := fixture.command("test", "run", "--root", fixture.root, "--goal", request.GoalID, "--tree", request.Tree,
				"--mode", "canary", "--purpose", "diagnostic", "--groups", "app-a", "--no-reuse", "--result", diagnosticPath)
			if status != 0 {
				return batch.DiagnosticResult{}, errors.New(output)
			}
			result := readResult(diagnosticPath)
			return batch.DiagnosticResult{AttemptID: result.AttemptID, Groups: trunkredmap.ResultToRedGroups(result)}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	reassembled, err := store.Load(batchID)
	if err != nil || reassembled.State != batch.StateOpen || reassembled.Units[0].State != batch.UnitReturnPending || reassembled.Units[1].State != batch.UnitJoined {
		t.Fatalf("red A did not leave only B: record=%+v error=%v", reassembled, err)
	}
	patchPath := filepath.Join(fixture.root, "artifacts", "agents", "landing-batches", "chains", "chain-b", "diff.patch")
	fixture.git("switch", "-q", "-C", "split-candidate", baseCommit)
	fixture.git("apply", "--index", "--binary", patchPath)
	survivorPath := filepath.Join(t.TempDir(), "survivor.json")
	fixture.requireCommand("test", "run", "--root", fixture.root, "--goal", "goal-b", "--tree", reassembled.TipTree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-b"}), "--result", survivorPath)
	fixture.requireCommand("test", "verify", "--root", fixture.root, "--goal", "goal-b", "--tree", reassembled.TipTree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-b"}))
	survivor := readResult(survivorPath)
	if !survivor.Delivery.Sufficient {
		t.Fatalf("survivor native proof was insufficient: %+v", survivor.Delivery)
	}
	if err := store.Update(batchID, func(record *batch.Record) error {
		record.State = batch.StateProving
		record.Proof = &batch.Proof{Status: "planned", Tree: record.TipTree, SelectedGroups: []string{"app-a", "app-b"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := batch.FinishProof(store, batchID, "owner", survivor, nil, at.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	land := batch.LandSeams{
		Prepare: func(string) error {
			fixture.git("reset", "--hard", baseCommit)
			fixture.git("switch", "-q", "-C", "split-publish", baseCommit)
			return nil
		},
		Apply:         func(batch.Unit) error { fixture.git("apply", "--index", "--binary", patchPath); return nil },
		AppendReceipt: func(batch.Unit, batch.PrefixReceipt) error { return nil },
		Commit: func(batch.Unit, batch.PrefixReceipt) (string, error) {
			fixture.git("commit", "-qm", "land independent B\n\nGoal-Unit: goal-b/u1")
			return fixture.git("rev-parse", "HEAD"), nil
		},
		Held: func(_, tip string) error {
			if fixture.git("rev-parse", tip+"^{tree}") != reassembled.TipTree {
				return errors.New("published commit does not match proved survivor tree")
			}
			return nil
		},
		Push: func(_, tip string) error { fixture.git("push", "-q", "origin", tip+":refs/heads/main"); return nil },
	}
	if err := batch.LandSeries(store, batchID, "owner", at.Add(3*time.Second), land); err != nil {
		t.Fatal(err)
	}
	publishedTree := batchE2EGit(t, origin, "rev-parse", "refs/heads/main^{tree}")
	if publishedTree != reassembled.TipTree || batchE2EGit(t, origin, "show", "refs/heads/main:app/a.txt") != "green a" ||
		!strings.Contains(batchE2EGit(t, origin, "show", "refs/heads/main:app/b.txt"), "green b changed") {
		t.Fatalf("remote published wrong split: tree=%s want=%s", publishedTree, reassembled.TipTree)
	}
}
