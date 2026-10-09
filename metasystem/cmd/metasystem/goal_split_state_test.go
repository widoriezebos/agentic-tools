package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	projectresolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	uiproject "github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

func splitStateBed(t *testing.T, amend func(map[string]*goal.GoalFile)) *goalCLIBed {
	t.Helper()
	return newGoalCLIBed(t, goalCLISeed{
		allowTerminalProof: true,
		amend: func(files map[string]*goal.GoalFile) {
			delete(files, "ship-widget")
			parent, child := files["fix-docs"], files["perf-pass"]
			parent.State = goal.StateSplit
			parent.Ratified = &goal.SplitRatification{Tier: goal.RatifierHuman, By: "Wido", DraftSHA256: strings.Repeat("a", 64)}
			parent.Tier = 1
			parent.Risk = &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture"}
			parent.Split = &goal.SplitRecord{Children: []string{child.Id}, Transaction: parent.History[0].Opid,
				PriorState: goal.StateParked, PriorParked: child.Parked}
			child.State, child.Parked = goal.StateQueued, nil
			child.Tier = 1
			child.Risk = &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture"}
			child.SplitFrom, child.Blocked = parent.Id, []string{parent.Id}
			if amend != nil {
				amend(files)
			}
		},
		rootRecord: func(root *goal.RootRecord) {
			root.Decomposed = []goal.DecomposedEntry{{Id: "port-engine", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FA5", "fixture-machine", "fixture-lineage"), At: goalCLISeedNow.Format("2006-01-02T15:04:05Z07:00")}}
		},
	})
}

func TestGoalShowKeepsSplitParentLive(t *testing.T) {
	t.Parallel()
	bed := splitStateBed(t, nil)
	designDir := filepath.Join(bed.root, "plans", "designs")
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatal(err)
	}
	design := "# Split source design\n\n- Kind: design\n- Id: split-source-design\n- Status: accepted\n- Goals: fix-docs\n"
	if err := os.WriteFile(filepath.Join(designDir, "split-source.md"), []byte(design), 0o644); err != nil {
		t.Fatal(err)
	}
	if listed := gcliLedgerMust(t, bed, "design", "list", "--goal", "fix-docs", "--json"); !strings.Contains(listed, "split-source-design") {
		t.Fatalf("design list lost the split source: %s", listed)
	}
	legacy := gcliLedgerMust(t, bed, "goal", "show", "port-engine", "--json")
	if strings.Contains(legacy, `"Split":`) || strings.Contains(legacy, `"SplitFrom":`) {
		t.Fatalf("new empty fields changed legacy JSON: %s", legacy)
	}
	shown := gcliLedgerMust(t, bed, "goal", "show", "fix-docs", "--json")
	if !strings.Contains(shown, "split-source-design") {
		t.Fatalf("show lost the source design: %s", shown)
	}
	var page struct {
		Data struct {
			Where string
			Goal  *goal.GoalFile
		}
	}
	if err := json.Unmarshal([]byte(shown), &page); err != nil {
		t.Fatal(err)
	}
	parent := page.Data.Goal
	if parent == nil || parent.Ratified == nil || parent.Ratified.DraftSHA256 != strings.Repeat("a", 64) || page.Data.Where != "live" || parent.State != goal.StateSplit || parent.Conclude != "" || parent.Claimed != nil || parent.Split == nil {
		t.Fatalf("show lost the unfinished split parent: %s", shown)
	}
	if parent.Split.PriorState != goal.StateParked || parent.Split.PriorParked == nil || parent.Split.PriorParked.Because != "Blocked on the vendor's profiler fix." || len(parent.Split.Children) != 1 || parent.Split.Children[0] != "perf-pass" || parent.Split.Transaction != goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FA3", "fixture-machine", "fixture-lineage") {
		t.Fatalf("show lost reversal data: %+v", parent.Split)
	}
	if strings.Contains(bed.accepted("plans/goals/backlog.md"), "- fix-docs opid=") || bed.accepted("records/goals/fix-docs.md") != "" {
		t.Fatal("split retired or archived its parent")
	}
	serialized, problems := goal.ParseFile([]byte(bed.goalRecord("fix-docs")))
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	roundtrip, problems := goal.ParseFile(goal.RenderFile(serialized))
	if len(problems) != 0 || !reflect.DeepEqual(roundtrip.Split, parent.Split) {
		t.Fatalf("split did not round-trip: %+v %v", roundtrip, problems)
	}
	text := gcliLedgerMust(t, bed, "goal", "show", "fix-docs")
	if !strings.Contains(text, "Split into goals") || !strings.Contains(text, "perf-pass") {
		t.Fatalf("plain show hid children: %s", text)
	}
	childShown := gcliLedgerMust(t, bed, "goal", "show", "perf-pass", "--json")
	if !strings.Contains(childShown, `"SplitFrom": "fix-docs"`) {
		t.Fatalf("show hid child lineage: %s", childShown)
	}
	if text := gcliLedgerMust(t, bed, "goal", "show", "perf-pass"); !strings.Contains(text, "Split from") || !strings.Contains(text, "fix-docs") {
		t.Fatalf("plain show hid parent: %s", text)
	}
	listing := gcliLedgerMust(t, bed, "goal", "list")
	if !strings.Contains(listing, "fix-docs") || !strings.Contains(listing, "Split") || !strings.Contains(listing, "2 open goals") {
		t.Fatalf("live listing hid split: %s", listing)
	}
	listing = gcliLedgerMust(t, bed, "goal", "list", "--all", "--json")
	if !strings.Contains(listing, `"State": "split"`) || !strings.Contains(listing, `"SplitFrom": "fix-docs"`) {
		t.Fatalf("JSON list lost lineage: %s", listing)
	}
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "perf-pass", "--budget", "norm"}, gcliLedgerHuman...)...)
	if ready := gcliLedgerMust(t, bed, "goal", "list", "--ready"); strings.Contains(ready, "next ready goal:") {
		t.Fatalf("split or held child became ready: %s", ready)
	}
	ep, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(ep, false, bed.clock())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Tree.Live["fix-docs"] == nil || projection.Tree.Done["fix-docs"] != nil {
		t.Fatal("projection concluded split")
	}
	frontier, err := goal.Next(projection, bed.machine)
	if err != nil || len(frontier.Ready) != 0 || len(frontier.Blocked) != 1 || frontier.Blocked[0] != "perf-pass" {
		t.Fatalf("split satisfied its child's dependency: %+v %v", frontier, err)
	}
	board := backlog.Project(projection.Tree, projection.Horizon, backlog.Admit(projection))
	var splitRow, childRow *backlog.Row
	for i := range board.Rows {
		if board.Rows[i].ID == "fix-docs" {
			splitRow = &board.Rows[i]
		}
		if board.Rows[i].ID == "perf-pass" {
			childRow = &board.Rows[i]
		}
	}
	if splitRow == nil || splitRow.Where != backlog.WhereLive || splitRow.Lane != backlog.LaneWaiting || splitRow.Phase != "split" || splitRow.Decomposed || splitRow.Concluded != "" || !reflect.DeepEqual(splitRow.Split, parent.Split) {
		t.Fatalf("board lost unfinished split: %+v", splitRow)
	}
	if childRow == nil || childRow.SplitFrom != "fix-docs" || len(childRow.OpenBlockers) != 1 || childRow.OpenBlockers[0] != "fix-docs" {
		t.Fatalf("board released or lost lineage: %+v", childRow)
	}
	if len(board.Closed) != 1 || !board.Closed[0].Decomposed {
		t.Fatalf("legacy decomposition lost its historical meaning: %+v", board.Closed)
	}
	read, err := projectresolver.Read(projectresolver.Roots{Checkout: bed.root, Installation: roots.Installation(bed.root), StateRoot: roots.State(bed.root)})
	if err != nil {
		t.Fatal(err)
	}
	if live := read.Goal("fix-docs"); live == nil || !live.Live() || !reflect.DeepEqual(live.Split, parent.Split) {
		t.Fatalf("project treated split as concluded: %+v", live)
	}
	pane, err := uiproject.ReadPane(uiproject.Roots{Checkout: bed.root, Installation: roots.Installation(bed.root), StateRoot: roots.State(bed.root)}, bed.clock())
	if err != nil {
		t.Fatal(err)
	}
	foundParent, foundChild := false, false
	for _, one := range pane.Goals {
		if one.ID == "fix-docs" {
			foundParent = one.State == goal.StateSplit && reflect.DeepEqual(one.Split, parent.Split)
		}
		if one.ID == "perf-pass" {
			foundChild = one.SplitFrom == "fix-docs"
		}
	}
	if !foundParent || !foundChild {
		t.Fatalf("project pane lost lineage: %+v", pane.Goals)
	}
	bed.announceHolder()
	gcliLedgerRefused(t, bed, "--reverse", "goal", "claim", "fix-docs")
	gcliLedgerRefused(t, bed, "--reverse", "goal", "resume", "fix-docs")
	gcliLedgerRefused(t, bed, "--reverse", "goal", "reopen", "fix-docs", "--next", "Restore.")
	gcliLedgerRefused(t, bed, "--reverse", append([]string{"goal", "approve", "fix-docs", "--budget", "norm"}, gcliLedgerHuman...)...)
	gcliLedgerRefused(t, bed, "never comes back", "goal", "reopen", "port-engine", "--next", "Revive.")
	// An agent cannot conclude the split even when its source was agent-origin.
	bed.prove = newGcliAuthorityCaller(t, bed, gcliAuthorityAgent).prove
	gcliLedgerRefused(t, bed, "human act", "goal", "done", "fix-docs", "--reason", "Agent must not retire responsibility.")
	bed.prove = fixedFixtureGoalAuthority
	gcliLedgerMust(t, bed, "goal", "done", "fix-docs", "--reason", "Person explicitly concluded the source.", "--by", "Wido")
	if done := gcliLedgerMust(t, bed, "goal", "show", "fix-docs", "--json"); !strings.Contains(done, `"State": "done"`) || !strings.Contains(done, `"children"`) {
		t.Fatalf("explicit conclusion lost lineage: %s", done)
	}
}

func TestGoalSplitLineageRefusesMalformedRecords(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, want string
		amend      func(map[string]*goal.GoalFile)
	}{
		{"missing child backlink", "does not link back", func(f map[string]*goal.GoalFile) { f["perf-pass"].SplitFrom = "" }},
		{"missing parent backlink", "does not link back", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split = nil; f["fix-docs"].State = goal.StateQueued }},
		{"missing child", "does not link back", func(f map[string]*goal.GoalFile) { delete(f, "perf-pass") }},
		{"empty children", "requires children", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.Children = nil }},
		{"bad transaction", "valid transaction", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.Transaction = "unknown" }},
		{"bad prior state", "invalid prior state", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.PriorState = goal.StateDone }},
		{"missing prior park", "prior park", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.PriorParked = nil }},
		{"broken prior park", "prior park", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.PriorParked.At = "unknown" }},
		{"repeated child", "repeated", func(f map[string]*goal.GoalFile) { f["fix-docs"].Split.Children = []string{"perf-pass", "perf-pass"} }},
		{"live claim", "Claimed", func(f map[string]*goal.GoalFile) {
			f["fix-docs"].Claimed = &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "fixture-lineage", At: f["fix-docs"].OpenedAt, Revision: 1}
		}},
		{"lineage cycle", "lineage cycle", func(f map[string]*goal.GoalFile) {
			f["fix-docs"].SplitFrom = "perf-pass"
			f["perf-pass"].Split = &goal.SplitRecord{Children: []string{"fix-docs"}, Transaction: f["perf-pass"].History[0].Opid, PriorState: goal.StateQueued}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := splitStateBed(t, tc.amend)
			gcliLedgerRefused(t, bed, tc.want, "goal", "show", "fix-docs", "--json")
		})
	}
	t.Run("retired legacy id", func(t *testing.T) {
		t.Parallel()
		bed := splitStateBed(t, nil)
		gcliLedgerRefused(t, bed, "was used by a goal that was split", append([]string{"goal", "open", "port-engine", "--intent", "Reuse a retired id.", "--next", "Stop.", "--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture"}, gcliLedgerHuman...)...)
	})
	t.Run("unreadable serialization", func(t *testing.T) {
		t.Parallel()
		bed := splitStateBed(t, nil)
		file, problems := goal.ParseFile([]byte(bed.goalRecord("fix-docs")))
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		body := strings.SplitN(string(goal.RenderFile(file)), "Integrity:", 2)[0]
		body = regexp.MustCompile(`(?m)^- Split: .*`).ReplaceAllString(body, "- Split: {broken json}")
		data := []byte(body + fmt.Sprintf("Integrity: sha256=%s\n", goal.IntegrityDigest([]byte(body))))
		gcliLedgerInstall(t, bed, "fixture-malformed-split", goal.Change{Path: "plans/goals/fix-docs.md", Content: data})
		gcliLedgerRefused(t, bed, "Split", "goal", "show", "fix-docs", "--json")
	})
}
