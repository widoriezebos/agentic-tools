package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestWorkLandTipReadCoversRecordedGoalUnits(t *testing.T) {
	t.Parallel()
	var records []struct {
		Goal  string
		Units []branch.UnitStatus
	}
	data, err := os.ReadFile("testdata/tip-unit-reads.json")
	if err != nil || json.Unmarshal(data, &records) != nil {
		t.Fatalf("read recorded unit shapes: %v", err)
	}
	for _, record := range records {
		t.Run(record.Goal, func(t *testing.T) {
			t.Parallel()
			var names []string
			var table strings.Builder
			table.WriteString("| Unit | Lines |\n| --- | --- |\n")
			for _, unit := range record.Units {
				names = append(names, unit.Unit)
				table.WriteString("| " + unit.Unit + " | 100 |\n")
			}
			page := filepath.Join(t.TempDir(), "design.md")
			if err := os.WriteFile(page, []byte(table.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			tip := record.Units[len(record.Units)-1].Commit
			state := intentBranchState{BranchTip: tip, Sources: make([]string, len(record.Units)), Status: branch.Status{Units: slices.Clone(record.Units), CleanReads: []branch.Attestation{{CoveredUnits: slices.Clone(names)}}}}
			// Status marks the builds the tip read covers as read clean (they are at or before the read's subject).
			for i := range state.Status.Units {
				state.Status.Units[i].ReadState = "read clean"
			}
			state.Status.Prefix = len(state.Status.Units)
			if subject, _, refusal := handLandingSubject(nil, record.Goal, "", state, false, true, page); refusal != nil || subject != tip {
				t.Fatalf("tip read did not cover hand-written goal: %s %+v", subject, refusal)
			}
			state.Status.CleanReads[0].CoveredUnits = names[:len(names)-1]
			// Status leaves the build the read does not cover unread.
			state.Status.Units[len(state.Status.Units)-1].ReadState = "needs read"
			state.Status.Prefix = len(state.Status.Units) - 1
			_, _, refusal := handLandingSubject(nil, record.Goal, "", state, false, true, page)
			if refusal == nil || !strings.Contains(refusal.Summary, names[len(names)-1]) || !slices.Equal(refusal.next,
				[]string{"metasystem", "work", "review", "--commit", tip, "--goal", record.Goal, "--units", names[len(names)-1]}) {
				t.Fatalf("missing unit was admitted or remedy lost its tip: %+v", refusal)
			}
		})
	}
}

func TestWorkLandTipReadCannotReplaceMissingBuild(t *testing.T) {
	t.Parallel()
	b, owners, _ := admissionBed(t, "hand")
	writeLandingUnits(b, "U1", "U2", "U3", "U4")
	finishThirdUnit(owners)
	for i := range owners.status.Status.Units {
		owners.status.Status.Units[i].Unit = "U" + string(rune('1'+i))
	}
	owners.status.Status.CleanReads = []branch.Attestation{{CoveredUnits: []string{"U1", "U2", "U3", "U4"}}}
	code, result := b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "U4 is not built") || result.Next == nil ||
		!slices.Equal(result.Next.Argv, []string{"metasystem", "work", "build", "standing-validation", "--work", "U4", "--brief", "FILE", "--check", "COMMAND"}) || len(owners.pushes) != 0 || owners.candidates != 0 {
		t.Fatalf("a tip read admitted missing work or offered another read: %d %+v", code, result)
	}
	owners.status.Status.Prefix = 0
	owners.status.Status.CleanReads[0].CoveredUnits = []string{"U2", "U3", "U4"}
	// Status marks the builds the read covers as read clean; U1, which the read does not cover, stays unread.
	for i := range owners.status.Status.Units {
		if owners.status.Status.Units[i].Unit != "U1" {
			owners.status.Status.Units[i].ReadState = "read clean"
		}
	}
	code, result = b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	if code == 0 || !strings.Contains(result.Summary, "U4 is not built") || !strings.Contains(result.Summary, "U1 has no clean read") || result.Next == nil ||
		!slices.Equal(result.Next.Argv, []string{"metasystem", "work", "build", "standing-validation", "--work", "U4", "--brief", "FILE", "--check", "COMMAND"}) {
		t.Fatalf("an earlier unread build hides the missing build's remedy: %d %+v", code, result)
	}
}

func TestWorkLandUnreadCompositeRemedyNamesEachUnit(t *testing.T) {
	t.Parallel()
	tip := strings.Repeat("b", 40)
	state := intentBranchState{BranchTip: tip, Status: branch.Status{Units: []branch.UnitStatus{{Unit: "U1+U2", Commit: tip}}}}
	_, _, refused := handLandingSubject(nil, "goal-a", "", state, false, true)
	if refused == nil || !slices.Equal(refused.next, []string{"metasystem", "work", "review", "--commit", tip, "--goal", "goal-a", "--units", "U1,U2"}) {
		t.Fatalf("composite names cannot be passed to --units: %+v", refused)
	}
}

func TestWorkReviewCommitForwardsCoveredUnits(t *testing.T) {
	t.Parallel()
	b := newDeliveryBedWith(t, nil, true)
	b.publicationSubject(strings.Repeat("b", 40))
	b.writeJob(map[string]any{"jobId": "tip-reader", "role": "code-critic", "status": "running", "round": 1})
	var received []string
	b.owners.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		received = slices.Clone(args)
		return branch.BranchReadResult{State: "open", RootJob: "tip-reader"}, 0, nil
	}
	code, result := b.do("work", "review", "--commit", strings.Repeat("b", 40), "--goal", "standing-validation", "--units", "U1,U5,U8")
	if code != 1 || result.Outcome != intentInProgress || flagValue(received, "--units") != "U1,U5,U8" {
		t.Fatalf("covered units did not reach read owner: %d %+v %v", code, result, received)
	}
}
