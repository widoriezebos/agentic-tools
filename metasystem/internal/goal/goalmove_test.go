package goal

import (
	"bytes"
	"testing"
)

func TestGoalRecordsOnly(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"goal-edit", "register-closure", "subfolder", "design-page", "testing-contract", "archive", "backlog", "unparseable-page", "unparseable-register", "empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			after := map[string][]byte{}
			for _, id := range []string{"ordinary-goal", "neighbour"} {
				f := vGoal(id, StateQueued)
				f.Intent = "Repair the ordinary goal."
				f.Revision++
				f.History = append(f.History, HistoryLine{At: "2026-10-04T12:01:00Z", Opid: f.History[0].Opid, Actor: "mac-a+lin-1", Verb: "edit", Targets: []string{id}, Keep: -1})
				after[livePath(id)] = bytes.TrimSpace(RenderFile(f))
			}
			entry := testTrunkRedEntry("red:unit:TestOne", "red:unit:TestOne", "2026-10-04T12:00:00Z")
			entry.Closed = &TrunkRedClosure{At: "2026-10-04T12:01:00Z", How: "hand", Opid: entry.Sightings[0].Opid, By: "wido", Why: "Resolved."}
			after[trunkRedPath] = bytes.TrimSpace(RenderTrunkRed([]TrunkRedEntry{entry}))
			for path, data := range after {
				var problems []Problem
				if path == trunkRedPath {
					_, problems = ParseTrunkRed(data)
				} else {
					_, problems = ParseFile(data)
				}
				if len(problems) != 0 {
					t.Fatalf("invalid fixture %s: %v", path, problems)
				}
			}
			want := false
			switch name {
			case "goal-edit":
				delete(after, trunkRedPath)
				want = true
			case "register-closure":
				after = map[string][]byte{trunkRedPath: after[trunkRedPath]}
				want = true
			case "subfolder":
				after["plans/goals/done/x.md"] = after[livePath("ordinary-goal")]
				want = true
			case "design-page":
				after["plans/designs/x.md"] = after[livePath("ordinary-goal")]
			case "testing-contract":
				after["testing.json"] = after[livePath("ordinary-goal")]
			case "archive":
				after["records/goals/x.md"] = bytes.TrimSpace(RenderFile(vGoal("x", StateDone)))
			case "backlog":
				after["plans/goals/backlog.md"] = []byte("# Backlog\n\nA list of goals.")
			case "unparseable-page":
				after[livePath("broken")] = []byte("not a goal page")
			case "unparseable-register":
				after[trunkRedPath] = []byte("not a register")
			case "empty":
				after = nil
			}
			if got := GoalRecordsOnly(after); got != want {
				t.Fatalf("goal-record admission = %v, want %v", got, want)
			}
		})
	}
}

func TestGoalRecordPathNamesOnlyTheRegisterAndGoalPages(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"plans/goals/trunk-red.json":          true,
		"plans/goals/a-flake-is-not-a-red.md": true,
		"plans/goals/backlog.md":              true,
		"plans/goals/notes.txt":               false,
		"plans/designs/fixture.md":            false,
		"testing.json":                        false,
	} {
		if got := GoalRecordPath(name); got != want {
			t.Errorf("GoalRecordPath(%q) = %v, want %v", name, got, want)
		}
	}
}
