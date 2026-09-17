package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func trunkRedNextFixture(t *testing.T) (string, goal.Projection) {
	t.Helper()
	root := syncedStoppedGoalFixture(t)
	p, err := goal.Project(goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return root, p
}

func TestGoalTrunkRedCommandsAndJSON(t *testing.T) {
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-17T10:00:00Z")
	root := syncedClaimedGoalFixture(t)
	emptyOutput, emptyCode := captureStdout(t, func() int {
		return runGoalList([]string{"--root", root, "--json"})
	})
	if emptyCode != 0 || !strings.Contains(emptyOutput, `"trunkRed":[]`) {
		t.Fatalf("empty JSON register code=%d output=%q", emptyCode, emptyOutput)
	}
	stamp := "2026-09-17T09:00:00Z"
	id := "tr-fast-command0001"
	register := goal.RenderTrunkRed([]goal.TrunkRedEntry{{
		ID: id, Identity: id, Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
			Opid: goal.Opid("01J5X0000000000000000000X1", "mac-cli", "m1")}},
		Owner: goal.TrunkRedOwner{Machine: "mac-cli", Since: stamp, How: "joiner"}, Holds: []string{"batch-1"}, Opened: stamp,
	}})
	registerPath := filepath.Join(root, "plans", "goals", "trunk-red.json")
	if err := os.WriteFile(registerPath, register, 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "plans/goals/trunk-red.json")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "trunk-red command fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	goalSyncMutationGit(t, root, "branch", "fix/red")

	output, code := captureStdout(t, func() int {
		return runGoalTrunkRed([]string{"own", "--root", root, "--id", id, "--goal", "standing-validation", "--branch", "fix/red", "--lineage", "m1"})
	})
	if code != 0 || !strings.Contains(output, `"outcome":"confirmed"`) {
		t.Fatalf("own command code=%d output=%q", code, output)
	}
	output, code = captureStdout(t, func() int {
		return runGoalList([]string{"--root", root, "--json"})
	})
	var listed struct {
		TrunkRed []goal.TrunkRedEntry `json:"trunkRed"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &listed) != nil || len(listed.TrunkRed) != 1 || listed.TrunkRed[0].FixBranch.Name != "fix/red" {
		t.Fatalf("json list code=%d output=%q parsed=%+v", code, output, listed)
	}
	stderr, code := captureStderr(t, func() int {
		return runGoalTrunkRed([]string{"close", "--root", root, "--id", id, "--why", "external outage", "--lineage", "m1"})
	})
	if code != 1 || !strings.Contains(stderr, "TRUNK_RED_CLOSE_IS_HUMAN") {
		t.Fatalf("close without human code=%d stderr=%q", code, stderr)
	}

	output, code = captureStdout(t, func() int {
		return runGoalTrunkRed([]string{"close", "--root", root, "--id", id, "--by", "Wido", "--why", "external outage", "--lineage", "human-line"})
	})
	if code != 0 || !strings.Contains(output, `"outcome":"confirmed"`) {
		t.Fatalf("close command code=%d output=%q", code, output)
	}
	stderr, code = captureStderr(t, func() int {
		return runGoalTrunkRed([]string{"own", "--root", root, "--id", id, "--goal", "standing-validation", "--to", "mac-other"})
	})
	if code != 2 || !strings.Contains(stderr, "--to only with --by") {
		t.Fatalf("--to edge code=%d stderr=%q", code, stderr)
	}
}
func renderTrunkRedNext(t *testing.T, root, machine string, p goal.Projection, labels ...string) string {
	t.Helper()
	out, code := captureStdout(t, func() int {
		return nextSyncedWithProjector(root, machine, false, func(goal.Endpoint, bool, time.Time) (goal.Projection, error) { return p, nil }, labels...)
	})
	if code != 0 {
		t.Fatalf("goal next code=%d output=%q", code, out)
	}
	return out
}
func TestTrunkRedEmptyFieldChangesNothing(t *testing.T) {
	root, p := trunkRedNextFixture(t)
	if problems := goal.ValidateTree(p.Tree); len(problems) != 0 {
		t.Fatalf("tree without trunk-red entries is invalid: %v", problems)
	}
	want := "single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal\n" +
		"FENCED standing-validation: breach-stopped by stop-standing-validation-r2-f1 (ELAPSED_LIMIT); only goal resume, a human act, clears it; the queue is open\n" +
		"no claimable goal for machine mac-cli; no matching eligible work\n"
	if without := renderTrunkRedNext(t, root, "mac-cli", p); without != want {
		t.Fatalf("no-entry output\ngot:  %q\nwant: %q", without, want)
	}
	tree := *p.Tree
	tree.TrunkRed = []goal.TrunkRedEntry{}
	p.Tree = &tree
	if with := renderTrunkRedNext(t, root, "mac-cli", p); with != want {
		t.Fatalf("empty field output\ngot:  %q\nwant: %q", with, want)
	}
}
func TestTrunkRedNextLinesAndPlacement(t *testing.T) {
	root, p := trunkRedNextFixture(t)
	tests := []struct {
		entry goal.TrunkRedEntry
		want  string
	}{
		{goal.TrunkRedEntry{ID: "failed", Group: "fast", Failures: []goal.TrunkRedFailure{{Report: "report", Classname: "Class", Name: "Test"}}, Sightings: []goal.TrunkRedSighting{{BaseCommit: "old"}, {BaseCommit: "base"}}, Owner: goal.TrunkRedOwner{Machine: "mac-cli", Since: "2026-09-17T01:00:00Z"}, FixGoal: "fix-red", FixBranch: goal.TrunkRedBranch{Name: "fix/red", Commit: "abc", State: goal.TrunkRedBranchOpen}, Holds: []string{"b1", "b2"}}, "trunk red failed: fast report/Class/Test on base, holds 2 batches, since 2026-09-17T01:00:00Z; fix it under fix-red on branch fix/red@abc (open)"},
		{goal.TrunkRedEntry{ID: "status", Group: "deep", Status: "not-run", Sightings: []goal.TrunkRedSighting{{BaseCommit: "base2"}}, Owner: goal.TrunkRedOwner{Machine: "mac-cli", Since: "2026-09-17T02:00:00Z"}}, "trunk red status: deep not-run on base2, holds 0 batches, since 2026-09-17T02:00:00Z; fix it under take it first"},
	}
	for _, test := range tests {
		if got := trunkRedOwnedLine(test.entry); got != test.want {
			t.Fatalf("owned line\ngot:  %q\nwant: %q", got, test.want)
		}
	}
	elsewhereTests := []struct {
		entry goal.TrunkRedEntry
		want  string
	}{
		{goal.TrunkRedEntry{ID: "elsewhere", Owner: goal.TrunkRedOwner{Machine: "m2", Since: "later"}}, "trunk red elsewhere owned by m2 since later"},
		{goal.TrunkRedEntry{ID: "unowned", Owner: goal.TrunkRedOwner{Since: "now"}}, "trunk red unowned owned by nobody since now"},
	}
	for _, elsewhere := range elsewhereTests {
		tree := *p.Tree
		tree.TrunkRed = []goal.TrunkRedEntry{tests[0].entry, tests[1].entry, elsewhere.entry}
		p.Tree = &tree
		out := renderTrunkRedNext(t, root, "mac-cli", p)
		ordered := []string{tests[0].want, tests[1].want, "FENCED standing-validation:", "no claimable goal for machine mac-cli", elsewhere.want}
		position := -1
		for _, text := range ordered {
			next := strings.Index(out, text)
			if next <= position {
				t.Fatalf("line %q is missing or misplaced in %q", text, out)
			}
			position = next
		}
	}
	tree := *p.Tree
	tree.TrunkRed = []goal.TrunkRedEntry{elsewhereTests[0].entry}
	p.Tree = &tree
	out := renderTrunkRedNext(t, root, "mac-cli", p, "missing")
	noMatch := strings.Index(out, "no goal matches --label missing")
	elsewhere := strings.Index(out, elsewhereTests[0].want)
	if noMatch < 0 || elsewhere <= noMatch {
		t.Fatalf("other-machine line must follow the label-no-match line: %q", out)
	}
}
func TestTrunkRedListCountsOpenEntriesOnly(t *testing.T) {
	grouped := map[string][]*goal.GoalFile{goal.StateQueued: {{Id: "goal", State: goal.StateQueued}}}
	plain := goalListSummary(grouped, syncedListStates, "tip", []string{"notice"}, false, goal.ApprovalHorizon{})
	empty := goalListSummary(grouped, syncedListStates, "tip", []string{"notice"}, false, goal.ApprovalHorizon{}, []goal.TrunkRedEntry{}...)
	wantEmpty := "claimed=0 approved=0 queued=1 parked=0 done=0 tip=tip\n" +
		"! notice\n" +
		"0:0 queued tier 0 goal pin=- claim=- :: \n"
	entries := []goal.TrunkRedEntry{{ID: "c", Group: "g3", Opened: "2", Owner: goal.TrunkRedOwner{Machine: "m3", Since: "three"}}, {ID: "closed", Closed: &goal.TrunkRedClosure{At: "closed"}}, {ID: "b", Group: "g2", Opened: "2", Owner: goal.TrunkRedOwner{Machine: "m2", Since: "two"}}, {ID: "z", Group: "g1", Opened: "1", Owner: goal.TrunkRedOwner{Since: "one"}, Holds: []string{"one"}}}
	out := goalListSummary(grouped, syncedListStates, "tip", []string{"notice"}, false, goal.ApprovalHorizon{}, entries...)
	ordered := []string{"trunk-red=3 tip=tip", "! notice", "! trunk red z g1 owned by nobody since one; holds 1 batches", "! trunk red b g2 owned by m2 since two; holds 0 batches", "! trunk red c g3 owned by m3 since three; holds 0 batches", "0:0 queued"}
	position := -1
	inOrder := true
	for _, text := range ordered {
		next := strings.Index(out, text)
		if next <= position {
			inOrder = false
		}
		position = next
	}
	headerEnd := strings.IndexByte(out, '\n') + 1
	footer := "... 1 more; run with --json > file for the records\n"
	row := "! trunk red z g1 owned by nobody since one; holds 1 batches\n"
	notice := strings.Repeat("x", goalListSummaryMaxBytes-headerEnd-len(row)-3)
	capped := goalListSummary(grouped, syncedListStates, "tip", []string{notice}, false, goal.ApprovalHorizon{}, entries[3])
	emptyGrouped := map[string][]*goal.GoalFile{}
	emptyHeader := "claimed=0 approved=0 queued=0 parked=0 done=0 trunk-red=1 tip=tip\n"
	noticeOnly := strings.Repeat("x", goalListSummaryMaxBytes-len(emptyHeader)-3)
	noticeCapped := goalListSummary(emptyGrouped, syncedListStates, "tip", []string{noticeOnly}, false, goal.ApprovalHorizon{}, entries[3])
	noticeFooter := "... 0 more; run with --json > file for the records\n"
	for name, ok := range map[string]bool{
		"no entries":                  plain == wantEmpty && empty == wantEmpty,
		"open and closed entries":     strings.Contains(out, "trunk-red=3 tip=tip") && !strings.Contains(out, "trunk red closed"),
		"empty owner":                 strings.Contains(out, "owned by nobody"),
		"opened and identifier order": inOrder,
		"cap and placement":           len(capped) <= goalListSummaryMaxBytes && strings.HasSuffix(capped, footer) && !strings.Contains(capped, "! trunk red") && !strings.Contains(capped, "0:0 queued"),
		"notice-side cap reserve":     len(noticeCapped) <= goalListSummaryMaxBytes && strings.HasSuffix(noticeCapped, noticeFooter),
	} {
		if !ok {
			t.Fatalf("%s: summary contract failed: output=%q capped-bytes=%d", name, out, len(capped))
		}
	}
}
