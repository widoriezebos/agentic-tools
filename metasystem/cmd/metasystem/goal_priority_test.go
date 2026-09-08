package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestGoalPriorityAuthority(t *testing.T) {
	t.Run("wrong-terminal", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		args := []string{"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "supplied-lineage", "--priority", "1"}
		unobserved := func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, nil
		}
		stderr, code := captureStderr(t, func() int {
			return runGoalSetPriorityWithAuthority(args, unobserved)
		})
		if code == 0 || !strings.Contains(stderr, "freshly observed enrolled-terminal human authority") {
			t.Fatalf("a human name and supplied lineage reordered without observed authority: code=%d stderr=%q", code, stderr)
		}
		endpoint, err := goal.ResolveEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := goal.Project(endpoint, false, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		file := projection.Tree.Live["standing-validation"]
		if file.Priority != 0 || file.Sequence != 0 {
			t.Fatalf("refused authority changed the rank: %+v", file)
		}
	})

	t.Run("proven", func(t *testing.T) {
		root := syncedClaimedGoalFixture(t)
		args := []string{"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "fixture-lineage", "--priority", "1", "--fixture-human-authority"}
		var stdout string
		stderr, code := captureStderr(t, func() int {
			var inner int
			stdout, inner = captureStdout(t, func() int { return runGoalSetPriority(args) })
			return inner
		})
		if code != 0 || stderr != "" || !strings.Contains(stdout, `"outcome":"confirmed"`) {
			t.Fatalf("fixture-observed authority did not drive the handler: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		endpoint, _ := goal.ResolveEndpoint(root)
		projection, err := goal.Project(endpoint, false, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		file := projection.Tree.Live["standing-validation"]
		if file.Priority != 1 || file.Sequence != 1 || file.History[len(file.History)-1].Verb != "set-priority" {
			t.Fatalf("proven command did not publish the requested rank: %+v", file)
		}
	})
}

func TestGoalPriorityListing(t *testing.T) {
	t.Run("cross-state", func(t *testing.T) {
		root := priorityListingFixture(t)
		stdout, code := captureStdout(t, func() int { return runGoalList([]string{"--root", root}) })
		if code != 0 {
			t.Fatalf("JSON list failed: code=%d output=%q", code, stdout)
		}
		var listed struct {
			Open     []*goal.GoalFile `json:"open"`
			Queued   []*goal.GoalFile `json:"queued"`
			Claimed  []*goal.GoalFile `json:"claimed"`
			Parked   []*goal.GoalFile `json:"parked"`
			Approved []*goal.GoalFile `json:"approved"`
		}
		if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
			t.Fatalf("decode goal list: %v\n%s", err, stdout)
		}
		if got := listedGoalIDs(listed.Open); strings.Join(got, ",") != "z-ranked,standing-validation,a-unranked" {
			t.Fatalf("cross-state open order = %v", got)
		}
		if len(listed.Queued) != 1 || listed.Queued[0].Id != "z-ranked" || len(listed.Claimed) != 1 || listed.Claimed[0].Id != "standing-validation" || len(listed.Parked) != 1 || listed.Parked[0].Id != "a-unranked" {
			t.Fatalf("state arrays were not filtered from the ordered traversal: queued=%v claimed=%v parked=%v", listedGoalIDs(listed.Queued), listedGoalIDs(listed.Claimed), listedGoalIDs(listed.Parked))
		}
		if listed.Open[0].Priority != 1 || listed.Open[0].Sequence != 1 || listed.Open[2].Priority != 0 || listed.Open[2].Sequence != 0 {
			t.Fatalf("JSON did not expose ranked and unranked field values: %+v", listed.Open)
		}

		pretty, prettyCode := captureStdout(t, func() int { return runGoalList([]string{"--root", root, "--pretty"}) })
		if prettyCode != 0 || !strings.Contains(pretty, "PRIORITY") || !strings.Contains(pretty, "SEQUENCE") || !strings.Contains(pretty, "STATE") || !strings.Contains(pretty, "PIN") || !strings.Contains(pretty, "GOAL") {
			t.Fatalf("pretty list lacks the open-goal table: code=%d\n%s", prettyCode, pretty)
		}
		zAt, standingAt, aAt := strings.Index(pretty, "z-ranked"), strings.Index(pretty, "standing-validation"), strings.Index(pretty, "a-unranked")
		if zAt < 0 || standingAt <= zAt || aAt <= standingAt {
			t.Fatalf("pretty list grouped by state instead of rank: %s", pretty)
		}
		if !strings.Contains(pretty, "queued") || !strings.Contains(pretty, "claimed") || !strings.Contains(pretty, "parked") || !strings.Contains(pretty, "m2") || !regexp.MustCompile(`(?m)^-\s+-\s+parked\s+m3\s+a-unranked$`).MatchString(pretty) || !strings.Contains(pretty, "Ranked queue intent.") {
			t.Fatalf("pretty rows lost state, pin, unranked marker, or intent detail: %s", pretty)
		}
	})
}

func priorityListingFixture(t *testing.T) string {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	standingPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(standingPath)
	if err != nil {
		t.Fatal(err)
	}
	standing, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse standing fixture: %v", problems)
	}
	standing.Priority, standing.Sequence = 1, 2
	queued := commandPriorityGoal("z-ranked", goal.StateQueued)
	queued.Priority, queued.Sequence, queued.Pinned = 1, 1, "m2"
	queued.Intent = "Ranked queue intent."
	parked := commandPriorityGoal("a-unranked", goal.StateParked)
	parked.Pinned = "m3"
	parked.Parked = &goal.ParkRecord{By: "human:Wido", At: "2026-08-30T09:00:00Z", Because: "Waiting for input."}
	for path, contents := range map[string][]byte{
		standingPath: goal.RenderFile(standing),
		filepath.Join(root, "plans", "goals", queued.Id+".md"): goal.RenderFile(queued),
		filepath.Join(root, "plans", "goals", parked.Id+".md"): goal.RenderFile(parked),
	} {
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goalSyncMutationGit(t, root, "add", "plans/goals")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "priority listing fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func commandPriorityGoal(id, state string) *goal.GoalFile {
	file := &goal.GoalFile{
		Id: id, State: state, Tier: 3, Intent: "Intent for " + id + ".", Origin: goal.OriginMain,
		OpenedAt: "2026-08-30T09:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-30T09:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1"),
			Verb: "open", Actor: "mac-cli+m1", Targets: []string{id}, Keep: -1,
		}},
	}
	return file
}

func listedGoalIDs(files []*goal.GoalFile) []string {
	ids := make([]string, 0, len(files))
	for _, file := range files {
		ids = append(ids, file.Id)
	}
	return ids
}
