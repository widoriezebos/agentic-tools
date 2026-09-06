package dispatch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlanFollowUpRebaseBehindWithOverlap(t *testing.T) {
	repo, worktree, base := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/shared.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "trunk overlap")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")

	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || plan.BehindCount != 1 || plan.RebasedFrom != base || plan.RebasedTo != trunk ||
		!reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("overlap plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseBehindWithoutOverlap(t *testing.T) {
	repo, worktree, base := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "other.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/other.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "unrelated trunk change")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")

	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rebase || plan.BehindCount != 1 || plan.RebasedFrom != base || plan.RebasedTo != trunk || len(plan.OverlappingPaths) != 0 {
		t.Fatalf("non-overlap plan = %+v", plan)
	}
	if !strings.Contains(plan.Reason, "no trunk commit touched this chain's files") {
		t.Fatalf("non-overlap reason = %q", plan.Reason)
	}
}

func TestPlanFollowUpRebaseNotBehind(t *testing.T) {
	repo, worktree, head := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, head)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rebase || plan.BehindCount != 0 || plan.RebasedFrom != head || plan.RebasedTo != head || len(plan.OverlappingPaths) != 0 {
		t.Fatalf("current plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseRefusesUnreadableRound(t *testing.T) {
	repo, worktree, _ := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	returnPath := filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "1", "return.json")
	writeFollowUpRebaseFile(t, returnPath, "{not-json\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "other.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/other.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "trunk movement")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")

	_, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err == nil || !strings.Contains(err.Error(), "cannot decode chain chain-a round 1 return") {
		t.Fatalf("unreadable round error = %v", err)
	}
}

func TestPlanFollowUpRebaseSkipsRoundsWithoutBoundaries(t *testing.T) {
	repo, worktree, _ := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "2", "return.json"), `{"findings":[]}`+"\n")
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "3"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/shared.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "trunk boundary overlap")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")

	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || !reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("plan with boundary-less rounds = %+v", plan)
	}
}

func TestPlanFollowUpRebaseUsesDirtyPathAbsentFromBoundaries(t *testing.T) {
	repo, worktree, _ := newFollowUpRebaseFixture(t, []string{"metasystem/other.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "2", "return.json"), `{"findings":[]}`+"\n")
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "3"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/shared.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "trunk dirty-path overlap")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")

	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || !reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("dirty-only overlap plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseReportsUnmergedPaths(t *testing.T) {
	repo, worktree, _ := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	gitFollowUpRebase(t, worktree, "stash", "push", "-qm", "unmerged-path-fixture")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	gitFollowUpRebase(t, repo, "add", "metasystem/shared.txt")
	gitFollowUpRebase(t, repo, "commit", "-qm", "trunk conflict")
	trunk := gitFollowUpRebase(t, repo, "rev-parse", "HEAD")
	gitFollowUpRebase(t, worktree, "merge", "--ff-only", "-q", trunk)
	command := exec.Command("git", "-C", worktree, "stash", "apply", "-q", "stash@{0}")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("stash apply unexpectedly avoided a conflict: %s", output)
	}

	plan, err := PlanFollowUpRebase(repo, "chain-a", worktree, trunk)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rebase || plan.BehindCount != 0 || !reflect.DeepEqual(plan.UnmergedPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("unmerged plan = %+v", plan)
	}
}

func TestValidateFollowUpRebaseFields(t *testing.T) {
	objectA := strings.Repeat("a", 40)
	objectB := strings.Repeat("b", 40)
	valid := []BuildFollowRecordParams{
		{},
		{RebasedFrom: objectA, RebasedTo: objectB},
		{RebasedFrom: objectA, RebasedTo: objectB, ConflictedPaths: []string{"metasystem/conflict.txt"}},
		{RebasedTo: objectB, ConflictedPaths: []string{"metasystem/conflict.txt"}},
	}
	for index := range valid {
		if err := validateFollowUpRebaseFields(&valid[index]); err != nil {
			t.Errorf("valid shape %d: %v", index, err)
		}
	}

	invalid := []BuildFollowRecordParams{
		{RebasedFrom: objectA},
		{RebasedTo: objectB},
		{ConflictedPaths: []string{"metasystem/conflict.txt"}},
		{RebasedFrom: objectA, ConflictedPaths: []string{"metasystem/conflict.txt"}},
	}
	for index := range invalid {
		err := validateFollowUpRebaseFields(&invalid[index])
		if err == nil || !strings.Contains(err.Error(), "rebasedFrom, rebasedTo, and conflictedPaths") {
			t.Errorf("invalid shape %d error = %v", index, err)
		}
	}
}

func newFollowUpRebaseFixture(t *testing.T, boundary []string) (repo, worktree, base string) {
	t.Helper()
	fixture := t.TempDir()
	repo = filepath.Join(fixture, "repo")
	worktree = filepath.Join(fixture, "worktree")
	if err := os.MkdirAll(filepath.Join(repo, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFollowUpRebase(t, repo, "init", "-q", "-b", "main")
	gitFollowUpRebase(t, repo, "config", "user.name", "fixture")
	gitFollowUpRebase(t, repo, "config", "user.email", "fixture@example.invalid")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "base\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "other.txt"), "base\n")
	gitFollowUpRebase(t, repo, "add", "metasystem")
	gitFollowUpRebase(t, repo, "commit", "-qm", "base")
	base = gitFollowUpRebase(t, repo, "rev-parse", "HEAD")
	gitFollowUpRebase(t, repo, "worktree", "add", "-q", "--detach", worktree, base)

	returnPath := filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "1", "return.json")
	encoded, err := json.Marshal(map[string]any{"diffBoundary": boundary})
	if err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, returnPath, string(encoded)+"\n")
	return repo, worktree, base
}

func writeFollowUpRebaseFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitFollowUpRebase(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
