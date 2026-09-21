package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

type batchBranchPatchBed struct {
	root, origin, base string
}

func newBatchBranchPatchBed(t *testing.T) batchBranchPatchBed {
	t.Helper()
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, root, "config", "user.name", "Batch Fixture")
	goalSyncMutationGit(t, root, "config", "user.email", "batch@example.invalid")
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem", "base.go"), []byte("package fixture\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem", "trunkgone", "value.go"), []byte("package trunkgone\n"), 0o644)
	goalSyncMutationGit(t, root, "add", ".")
	goalSyncMutationGit(t, root, "commit", "-qm", "base")
	origin := filepath.Join(t.TempDir(), "origin.git")
	goalSyncMutationGit(t, filepath.Dir(origin), "init", "-q", "--bare", origin)
	goalSyncMutationGit(t, root, "remote", "add", "origin", origin)
	goalSyncMutationGit(t, root, "push", "-q", "origin", "main")
	return batchBranchPatchBed{root: root, origin: origin, base: goalSyncMutationGit(t, root, "rev-parse", "HEAD")}
}

func addBatchBranchUnit(t *testing.T, bed batchBranchPatchBed, unit, path, body string) string {
	t.Helper()
	writeTestingFixtureFile(t, filepath.Join(bed.root, filepath.FromSlash(path)), []byte(body), 0o644)
	goalSyncMutationGit(t, bed.root, "add", "--", path)
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{
		Repo: bed.root, Remote: "origin", EndpointTip: bed.base, GoalID: "goal-a", Unit: unit,
		OpID: "build-" + unit, Kind: goalbranch.Unit, CheckClaim: func() error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: bed.root, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("read subject present=%v err=%v", present, err)
	}
	job := "critic-" + unit
	rootRecord := map[string]any{
		"jobId": job, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"},
	}
	writeBatchBranchJSON(t, filepath.Join(bed.root, "artifacts", "agents", "jobs", job+".json"), rootRecord)
	writeBatchBranchJSON(t, filepath.Join(bed.root, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	writeBatchBranchJSON(t, filepath.Join(bed.root, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	if _, _, err := goalbranch.CommitRead(goalbranch.CommitReadRequest{
		Repo: bed.root, Remote: "origin", EndpointTip: bed.base, GoalID: "goal-a", Unit: unit, OpID: "read-" + unit,
		RootJob: job, GateRunID: "fast-" + unit, GateTree: subject.Tree, CheckClaim: func() error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
	return commit
}

func writeBatchBranchJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, path, append(data, '\n'), 0o644)
}

func pushBatchGoalBranch(t *testing.T, bed batchBranchPatchBed) {
	t.Helper()
	goalSyncMutationGit(t, bed.root, "push", "-q", "origin", "refs/heads/goal/goal-a:refs/heads/goal/goal-a")
}

func addBatchBranchPlan(t *testing.T, bed batchBranchPatchBed, path string) {
	t.Helper()
	writeTestingFixtureFile(t, filepath.Join(bed.root, filepath.FromSlash(path)), []byte("later plan\n"), 0o644)
	goalSyncMutationGit(t, bed.root, "add", "--", path)
	if _, err := goalbranch.CommitStaged(goalbranch.CommitRequest{
		Repo: bed.root, Remote: "origin", EndpointTip: bed.base, GoalID: "goal-a",
		OpID: "later-plan", Kind: goalbranch.Plan, CheckClaim: func() error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
}

func assertBatchBranchPatchGate(t *testing.T, bed batchBranchPatchBed, request batchJoinRequest, forbiddenPackage string, forbiddenPaths ...string) batch.Unit {
	t.Helper()
	member, patch, err := productionBatchBranchMember(request)
	if err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, bed.root, "fetch", "-q", "origin", "main")
	endpoint := goalSyncMutationGit(t, bed.root, "rev-parse", "FETCH_HEAD")
	endpointTree := goalSyncMutationGit(t, bed.root, "rev-parse", endpoint+"^{tree}")
	prefixes, err := batch.AssembleBranchMembers(bed.root, endpointTree, []batch.BranchMember{member})
	if err != nil || len(prefixes) != 1 {
		t.Fatalf("assembled prefixes=%v err=%v", prefixes, err)
	}
	unit := batch.BindBranchMember(batch.Unit{GoalID: "goal-a"}, member)
	for path := range batch.ChangedPaths(patch) {
		unit.ChangedPaths = append(unit.ChangedPaths, path)
	}
	slices.Sort(unit.ChangedPaths)
	for _, path := range forbiddenPaths {
		if slices.Contains(unit.ChangedPaths, path) {
			t.Fatalf("changed paths include endpoint or suffix path %s: %v", path, unit.ChangedPaths)
		}
	}
	for _, path := range unit.ChangedPaths {
		if strings.HasPrefix(path, "metasystem/"+strings.TrimPrefix(forbiddenPackage, "./")+"/") {
			t.Fatalf("branch member included forbidden package %s in %v", forbiddenPackage, unit.ChangedPaths)
		}
	}
	return unit
}

func TestBatchThroughJoinIgnoresLaterPackage(t *testing.T) {
	bed := newBatchBranchPatchBed(t)
	through := addBatchBranchUnit(t, bed, "prefix", "metasystem/prefix/value.go", "package prefix\n")
	addBatchBranchUnit(t, bed, "later", "metasystem/later/value.go", "package later\n")
	addBatchBranchPlan(t, bed, "metasystem/plans/later.md")
	pushBatchGoalBranch(t, bed)
	unit := assertBatchBranchPatchGate(t, bed, batchJoinRequest{SeatRoot: bed.root, GoalID: "goal-a", Through: through}, "./later",
		"metasystem/later/value.go", "metasystem/plans/later.md")
	if len(unit.Builds) != 1 || unit.Builds[0].Commit != through {
		t.Fatalf("through member=%+v", unit.Builds)
	}
}

func TestBatchLaggingBranchExcludesEndpointChanges(t *testing.T) {
	bed := newBatchBranchPatchBed(t)
	addBatchBranchUnit(t, bed, "member", "metasystem/member/value.go", "package member\n")
	pushBatchGoalBranch(t, bed)
	goalSyncMutationGit(t, bed.root, "switch", "--quiet", "main")
	if err := os.Remove(filepath.Join(bed.root, "metasystem", "trunkgone", "value.go")); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(bed.root, "metasystem", "trunk-only.txt"), []byte("endpoint\n"), 0o644)
	goalSyncMutationGit(t, bed.root, "add", "-A", "metasystem/trunkgone/value.go", "metasystem/trunk-only.txt")
	goalSyncMutationGit(t, bed.root, "commit", "-qm", "move endpoint")
	goalSyncMutationGit(t, bed.root, "push", "-q", "origin", "main")
	assertBatchBranchPatchGate(t, bed, batchJoinRequest{SeatRoot: bed.root, GoalID: "goal-a", Last: true}, "./trunkgone",
		"metasystem/trunkgone/value.go", "metasystem/trunk-only.txt")
}
