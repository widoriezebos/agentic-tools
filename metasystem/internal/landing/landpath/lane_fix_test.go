package landpath

import (
	"testing"
)

func TestGuardLaneFixCommitRule(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, commit, message string
		members               []string
		actor                 bool
		want                  int
	}{
		{"landing agent", "batch", "repair\n\nGoal-Unit: goal-a/lane-fix-1", []string{"goal-a"}, true, 0},
		{"launched build", "batch", "repair\n\nGoal-Unit: goal-a/lane-fix-2", []string{"goal-a"}, true, 0},
		{"another agent", "batch", "repair\n\nGoal-Unit: goal-a/lane-fix-1", []string{"goal-a"}, false, 1},
		{"non-batch HEAD", "other", "repair\n\nGoal-Unit: goal-a/lane-fix-1", []string{"goal-a"}, true, 1},
		{"no trailer", "batch", "repair", []string{"goal-a"}, true, 1},
		{"non-member", "batch", "repair\n\nGoal-Unit: outsider/lane-fix-1", []string{"goal-a"}, true, 1},
		{"ordinary unit", "batch", "repair\n\nGoal-Unit: goal-a/unit", []string{"goal-a"}, true, 1},
		{"body only", "batch", "Goal-Unit: goal-a/lane-fix-1\n\nrepair", []string{"goal-a"}, true, 1},
		{"duplicate trailer", "batch", "repair\n\nGoal-Unit: goal-a/lane-fix-1\nGoal-Unit: goal-a/lane-fix-2", []string{"goal-a"}, true, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newGuardBed(t)
			b.git.branch, b.git.head = "", "batch"
			b.stage("M", "code.go")
			b.owners.Classify = func(string, int64) (string, error) { return "MAIN", nil }
			b.owners.LaneFix = func(root, checkout string, caller int64) *LaneFixCommit {
				if root != b.root || checkout != b.root || caller != 900 {
					t.Fatal("wrong lane identity boundary")
				}
				if !test.actor {
					return nil
				}
				return &LaneFixCommit{Commit: test.commit, Message: test.message, Members: test.members}
			}
			b.expect(b.run(), test.want)
		})
	}
}

func TestGuardLaneFixStillFencesLedgerAndBackups(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"metasystem/plans/goals/g.json", "code.go.orig"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newGuardBed(t)
			b.git.branch, b.git.head = "", "batch"
			b.stage("M", path)
			b.owners.Classify = func(string, int64) (string, error) { return "MAIN", nil }
			b.owners.LaneFix = func(string, string, int64) *LaneFixCommit {
				return &LaneFixCommit{Commit: "batch", Message: "fix\n\nGoal-Unit: goal-a/lane-fix-1", Members: []string{"goal-a"}}
			}
			b.expect(b.run(), 1)
		})
	}
}

func TestGuardLaneFixMatchesRecordedRound(t *testing.T) {
	t.Parallel()
	for _, unit := range []string{"lane-fix-1", "lane-fix-2"} {
		t.Run(unit, func(t *testing.T) {
			t.Parallel()
			b := newGuardBed(t)
			b.git.branch, b.git.head = "", "batch"
			b.stage("M", "code.go")
			b.owners.Classify = func(string, int64) (string, error) { return "MAIN", nil }
			b.owners.LaneFix = func(string, string, int64) *LaneFixCommit {
				return &LaneFixCommit{Commit: "batch", Members: []string{"goal-a"}, Unit: "lane-fix-1", Message: "repair\n\nGoal-Unit: goal-a/" + unit}
			}
			want := 1
			if unit == "lane-fix-1" {
				want = 0
			}
			b.expect(b.run(), want)
		})
	}
}

func TestGuardLaneMergeRequiresTrailerAndSelectedMergeTip(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message, head, merge, selected string
		want                                 int
	}{
		{"merge", "merge\n\nGoal-Unit: goal-a/lane-merge-1", "batch", "tip", "tip", 0},
		{"fix during merge", "repair\n\nGoal-Unit: goal-a/lane-fix-1", "batch", "tip", "tip", 1},
		{"fix after abort", "repair\n\nGoal-Unit: goal-a/lane-fix-1", "batch", "", "tip", 1},
		{"no trailer", "merge", "batch", "tip", "tip", 1},
		{"missing merge", "merge\n\nGoal-Unit: goal-a/lane-merge-1", "batch", "", "tip", 1},
		{"other tip", "merge\n\nGoal-Unit: goal-a/lane-merge-1", "batch", "other", "tip", 1},
		{"other member", "merge\n\nGoal-Unit: goal-a/lane-merge-1", "batch", "tip", "other", 1},
		{"moved head", "merge\n\nGoal-Unit: goal-a/lane-merge-1", "other", "tip", "tip", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newGuardBed(t)
			b.git.branch, b.git.head = "", test.head
			b.git.on("rev-parse --verify MERGE_HEAD", func(GitCall) GitResult { return ok(test.merge) })
			b.stage("M", "code.go")
			b.owners.Classify = func(string, int64) (string, error) { return "MAIN", nil }
			b.owners.LaneFix = func(string, string, int64) *LaneFixCommit {
				return &LaneFixCommit{Commit: "batch", Message: test.message, Members: map[bool][]string{true: {"goal-a"}, false: {"other"}}[test.selected == "tip"], MergeTip: "tip"}
			}
			b.expect(b.run(), test.want)
		})
	}
}
