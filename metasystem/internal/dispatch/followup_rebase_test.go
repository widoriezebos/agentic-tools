package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlanFollowUpRebaseBehindWithOverlap(t *testing.T) {
	repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || plan.BehindCount != 1 || plan.RebasedFrom != base || plan.RebasedTo != trunk ||
		!reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("overlap plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseBehindWithoutOverlap(t *testing.T) {
	repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "other.txt"), "trunk\n")
	facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/other.txt")}
	facts.tracked[worktree] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
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
	repo, worktree, head, _, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	plan, err := planFollowUpRebase(repo, "chain-a", worktree, head, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rebase || plan.BehindCount != 0 || plan.RebasedFrom != head || plan.RebasedTo != head || len(plan.OverlappingPaths) != 0 {
		t.Fatalf("current plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseRefusesUnreadableRound(t *testing.T) {
	repo, worktree, _, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	returnPath := filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "1", "return.json")
	writeFollowUpRebaseFile(t, returnPath, "{not-json\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "other.txt"), "trunk\n")

	_, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err == nil || !strings.Contains(err.Error(), "cannot decode chain chain-a round 1 return") {
		t.Fatalf("unreadable round error = %v", err)
	}
}

func TestPlanFollowUpRebaseSkipsRoundsWithoutBoundaries(t *testing.T) {
	repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "2", "return.json"), `{"findings":[]}`+"\n")
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "3"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || !reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("plan with boundary-less rounds = %+v", plan)
	}
}

func TestPlanFollowUpRebaseUsesDirtyPathAbsentFromBoundaries(t *testing.T) {
	repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/other.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "2", "return.json"), `{"findings":[]}`+"\n")
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "3"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	writeFollowUpRebaseFile(t, filepath.Join(repo, "metasystem", "shared.txt"), "trunk\n")
	facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}
	facts.tracked[worktree] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || !reflect.DeepEqual(plan.OverlappingPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("dirty-only overlap plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseReportsUnmergedPaths(t *testing.T) {
	repo, worktree, _, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	writeFollowUpRebaseFile(t, filepath.Join(worktree, "metasystem", "shared.txt"), "delegate\n")
	facts.heads[worktree] = postureFact[string]{value: trunk}
	facts.behind[postureRangeKey{worktree, trunk, trunk}] = postureFact[int64]{value: 0}
	facts.unmerged[worktree] = postureFact[map[string]struct{}]{value: posturePaths("metasystem/shared.txt")}

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rebase || plan.BehindCount != 0 || !reflect.DeepEqual(plan.UnmergedPaths, []string{"metasystem/shared.txt"}) {
		t.Fatalf("unmerged plan = %+v", plan)
	}
}

func TestPlanFollowUpRebaseCitedTrunkPath(t *testing.T) {
	repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt"})
	freshPath := "metasystem/plans/fresh.md"
	writeFollowUpRebaseFile(t, filepath.Join(repo, freshPath), "fresh trunk plan\n")
	facts.presence[posturePathKey{worktree, base, freshPath}] = postureFact[bool]{value: false}
	facts.presence[posturePathKey{worktree, trunk, freshPath}] = postureFact[bool]{value: true}
	brief := filepath.Join(t.TempDir(), "follow-up.md")
	writeFollowUpRebaseFile(t, brief, "Authority: "+freshPath+"\n")

	plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, brief, facts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rebase || !reflect.DeepEqual(plan.CitedTrunkPaths, []string{freshPath}) || len(plan.OverlappingPaths) != 0 ||
		plan.Reason != "the brief cites paths the trunk gained" {
		t.Fatalf("cited trunk path plan = %+v", plan)
	}
	withoutBrief, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, "", facts)
	if err != nil {
		t.Fatal(err)
	}
	if withoutBrief.Rebase || !reflect.DeepEqual(withoutBrief.CitedTrunkPaths, []string{}) {
		t.Fatalf("plan without brief = %+v", withoutBrief)
	}
}

func TestPlanFollowUpRebaseCitedPathsThatDoNotTrigger(t *testing.T) {
	cases := []struct {
		name, trunkPath, brief string
	}{
		{"already in worktree", "trunk-unrelated.txt", "Authority: metasystem/plans/base.md\n"},
		{"absent from trunk", "metasystem/plans/unrelated.md", "Authority: metasystem/plans/never.md\n"},
		{"runtime path", "artifacts/agents/x.md", "Authority: artifacts/agents/x.md\n"},
		{"declared output", "metasystem/plans/made.md", "Create: metasystem/plans/made.md\n"},
		{"malformed bounds", "metasystem/plans/fresh.md", "Boundary: not-json\nAuthority: metasystem/plans/fresh.md\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repo, worktree, base, trunk, facts := newFollowUpRebaseFixture(t, []string{"metasystem/shared.txt", "metasystem/other.txt"})
			writeFollowUpRebaseFile(t, filepath.Join(repo, test.trunkPath), "trunk change\n")
			facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths(test.trunkPath)}
			switch test.name {
			case "already in worktree":
				facts.presence[posturePathKey{worktree, base, "metasystem/plans/base.md"}] = postureFact[bool]{value: true}
			case "absent from trunk":
				facts.presence[posturePathKey{worktree, base, "metasystem/plans/never.md"}] = postureFact[bool]{value: false}
				facts.presence[posturePathKey{worktree, trunk, "metasystem/plans/never.md"}] = postureFact[bool]{value: false}
			}
			brief := filepath.Join(t.TempDir(), "follow-up.md")
			writeFollowUpRebaseFile(t, brief, test.brief)

			plan, err := planFollowUpRebase(repo, "chain-a", worktree, trunk, brief, facts)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Rebase || !reflect.DeepEqual(plan.CitedTrunkPaths, []string{}) {
				t.Fatalf("non-triggering cited path plan = %+v", plan)
			}
		})
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

func newFollowUpRebaseFixture(t *testing.T, boundary []string) (repo, worktree, base, trunk string, facts *postureFacts) {
	t.Helper()
	fixture := t.TempDir()
	repo = filepath.Join(fixture, "repo")
	worktree = filepath.Join(fixture, "worktree")
	base, trunk = "base-tip", "trunk-tip"
	for _, dir := range []string{repo, worktree} {
		writeFollowUpRebaseFile(t, filepath.Join(dir, "metasystem", "shared.txt"), "base\n")
		writeFollowUpRebaseFile(t, filepath.Join(dir, "metasystem", "other.txt"), "base\n")
		writeFollowUpRebaseFile(t, filepath.Join(dir, "metasystem", "plans", "base.md"), "base\n")
	}
	returnPath := filepath.Join(repo, "artifacts", "agents", "chain-a", "rounds", "1", "return.json")
	encoded, err := json.Marshal(map[string]any{"diffBoundary": boundary})
	if err != nil {
		t.Fatal(err)
	}
	writeFollowUpRebaseFile(t, returnPath, string(encoded)+"\n")
	facts = newPostureFacts()
	facts.heads[worktree] = postureFact[string]{value: base}
	facts.refs[postureRefKey{worktree, trunk}] = postureFact[string]{value: trunk}
	facts.refs[postureRefKey{worktree, base}] = postureFact[string]{value: base}
	facts.behind[postureRangeKey{worktree, base, trunk}] = postureFact[int64]{value: 1}
	facts.behind[postureRangeKey{worktree, base, base}] = postureFact[int64]{value: 0}
	facts.unmerged[worktree] = postureFact[map[string]struct{}]{value: posturePaths()}
	facts.tracked[worktree] = postureFact[map[string]struct{}]{value: posturePaths()}
	facts.untracked[worktree] = postureFact[map[string]struct{}]{value: posturePaths()}
	facts.touched[postureRangeKey{worktree, base, trunk}] = postureFact[map[string]struct{}]{value: posturePaths()}
	facts.prefixes[repo] = postureFact[string]{value: ""}
	facts.directories[postureTreeKey{worktree, trunk}] = postureFact[map[string]bool]{value: map[string]bool{"metasystem": true, "artifacts": true}}
	facts.directories[postureTreeKey{worktree, trunk + ":metasystem"}] = postureFact[map[string]bool]{value: map[string]bool{"plans": true}}
	return repo, worktree, base, trunk, facts
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
