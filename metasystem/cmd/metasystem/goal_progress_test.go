package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func admissionBed(t *testing.T, route string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b, owners, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	b.owners.laneLatest = func(install, id, main string) (plain.Entry, bool, error) {
		entry, found, err := plain.Latest(install, id)
		if err == nil && found {
			derived, deriveErr := plain.Landed([]plain.Entry{entry}, func(sha string) (bool, error) { return sha == main, nil })
			entry, err = derived[0], deriveErr
		}
		return entry, found, err
	}
	if route == "hand" {
		b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
	}
	writeLandingUnits(b, "u1", "u2", "u3")
	return b, owners, install
}

func finishThirdUnit(owners *landingOwners) {
	commit := strings.Repeat("3", 40)
	owners.status.Status.Units = append(owners.status.Status.Units, branch.UnitStatus{Unit: "u3", Commit: commit})
	owners.status.Status.Prefix = 3
	owners.status.Sources = append(owners.status.Sources, "critic-root")
	owners.status.BranchTip = commit
	owners.status.Status.Commits = []branch.Commit{{ID: strings.Repeat("1", 40), Kind: branch.Unit}, {ID: strings.Repeat("2", 40), Kind: branch.Unit}, {ID: commit, Kind: branch.Unit}}
}

func TestWorkLandRequiresFinishedDeclaredGoal(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"lane", "hand"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, owners, install := admissionBed(t, route)
			code, result := b.do("work", "land", "standing-validation")
			expectOutcome(t, "missing third unit", code, result, intentRefused)
			if resultData(t, result)["code"] != "GOAL_NOT_FINISHED" || !strings.Contains(result.Summary, "u3 is not built") ||
				result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "build", "standing-validation", "--work", "u3", "--brief", "FILE", "--check", "COMMAND"}) {
				t.Fatalf("unfinished goal's remedy: %+v", result)
			}
			if entries, err := plain.Entries(install); err != nil || len(entries) != 0 || owners.candidates != 0 {
				t.Fatalf("unfinished goal changed delivery: %+v candidates=%d err=%v", entries, owners.candidates, err)
			}
			finishThirdUnit(owners)
			code, result = b.do("work", "land", "standing-validation", "--through", strings.Repeat("2", 40))
			expectOutcome(t, "partial selection", code, result, intentRefused)
			if resultData(t, result)["code"] != "GOAL_LAND_WHOLE" || owners.candidates != 0 {
				t.Fatalf("partial selection went through: %+v", result)
			}
			code, result = b.do("work", "land", "standing-validation", "--through", strings.Repeat("3", 40))
			expectOutcome(t, "whole finished goal", code, result, intentConfirmed)
			if route == "lane" {
				entries, err := plain.Entries(install)
				if err != nil || len(entries) != 1 || entries[0].SHA != owners.status.BranchTip {
					t.Fatalf("whole goal was not queued: %+v %v", entries, err)
				}
			} else if len(owners.pushes) != 1 {
				t.Fatal("finished goal was not pushed")
			}
		})
	}
}

func TestWorkLandRequiresDeclaredEnd(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"lane", "hand"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, owners, install := admissionBed(t, route)
			b.writeFile(filepath.Join(b.root(), "plans", "designs", "landing-work.md"),
				"# Landing work\n\n- Kind: design\n- Id: landing-work\n- Status: accepted\n- Goals: standing-validation\n")
			owners.status.Status.Units = owners.status.Status.Units[:1]
			owners.status.Status.Prefix = 1
			owners.status.Sources = owners.status.Sources[:1]
			owners.status.BranchTip = owners.status.Status.Units[0].Commit
			code, result := b.do("work", "land", "standing-validation")
			expectOutcome(t, "no declared end", code, result, intentRefused)
			if resultData(t, result)["code"] != "GOAL_NO_END" ||
				oneSpaced(result.Summary) != "goal standing-validation has no Units table and no unit built with --last, so nothing says it is finished. A goal lands whole, once." ||
				result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "build", "standing-validation", "--work", "NAME", "--last", "--brief", "FILE", "--check", "COMMAND"}) {
				t.Fatalf("goal without an end's remedy: %+v", result)
			}
			if entries, err := plain.Entries(install); err != nil || len(entries) != 0 || owners.candidates != 0 || len(owners.pushes) != 0 {
				t.Fatalf("goal without an end changed delivery: %+v candidates=%d pushes=%d err=%v", entries, owners.candidates, len(owners.pushes), err)
			}
		})
	}
}

func TestWorkLandGoalLandsOnceOnEitherRoute(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"lane", "hand"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, owners, install := admissionBed(t, route)
			finishThirdUnit(owners)
			code, result := b.do("work", "land", "standing-validation")
			expectOutcome(t, "first hand-in", code, result, intentConfirmed)
			landed := owners.status.BranchTip
			owners.status.EndpointTip = landed
			owners.branchDeleted = false
			commit := strings.Repeat("4", 40)
			owners.status.BranchTip = commit
			owners.status.Status.Units = append(owners.status.Status.Units, branch.UnitStatus{Unit: "correction", Commit: commit})
			owners.status.Status.Commits = append(owners.status.Status.Commits, branch.Commit{ID: commit, Kind: branch.Unit})
			owners.status.Status.Prefix++
			owners.status.Sources = append(owners.status.Sources, "critic-root")
			code, result = b.do("work", "land", "standing-validation")
			expectOutcome(t, "new work after landing", code, result, intentRefused)
			if resultData(t, result)["code"] != "GOAL_LAND_ONCE" || !strings.Contains(result.Summary, "already landed at "+shortCommit(landed)) || result.Next == nil ||
				!slices.Equal(result.Next.Argv, []string{"metasystem", "goal", "done", "standing-validation", "--reason", "TEXT"}) {
				t.Fatalf("landed goal's remedy: %+v", result)
			}
			// Main may contain the old hand-in, leaving only later builds in
			// the branch reader's range.
			owners.status.Status.Units = owners.status.Status.Units[3:]
			owners.status.Status.Commits = owners.status.Status.Commits[3:]
			owners.status.Status.Prefix, owners.status.Sources = 1, []string{"critic-root"}
			code, result = b.do("work", "land", "standing-validation")
			expectOutcome(t, "new range after landing", code, result, intentRefused)
			if resultData(t, result)["code"] != "GOAL_LAND_ONCE" {
				t.Fatalf("landed history was lost outside the branch range: %+v", result)
			}
			if route == "lane" {
				entries, err := plain.Entries(install)
				if err != nil || len(entries) != 1 {
					t.Fatalf("landed goal was queued again: %+v %v", entries, err)
				}
			} else if len(owners.pushes) != 1 || owners.candidates != 1 {
				t.Fatal("landed goal was prepared or pushed again")
			}
		})
	}
}

func TestWorkLandTierOneRequiresAllDeclaredBuilds(t *testing.T) {
	t.Parallel()
	b, owners, install := admissionBed(t, "lane")
	file := b.goalFile("standing-validation")
	file.Tier, file.Budget.ReviewRoundLimit = 1, 0
	file.Risk.Severity, file.Risk.Novelty = 1, 1
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	b.addGoal(file)
	owners.status.ReadsWaived, owners.status.Status.Prefix, owners.status.Sources = true, 0, nil
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "missing waived unit", code, result, intentRefused)
	finishThirdUnit(owners)
	owners.status.Status.Prefix, owners.status.Sources = 0, nil
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "all units built without reads", code, result, intentConfirmed)
	if entries, err := plain.Entries(install); err != nil || len(entries) != 1 {
		t.Fatalf("waived goal was not queued: %+v %v", entries, err)
	}
}

func TestWorkLandWaitingGoalCanReplaceItsWholeTip(t *testing.T) {
	t.Parallel()
	b, owners, install := admissionBed(t, "lane")
	finishThirdUnit(owners)
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "first whole tip", code, result, intentConfirmed)
	owners.status.BranchTip = strings.Repeat("4", 40)
	owners.status.Status.Units = append(owners.status.Status.Units, branch.UnitStatus{Unit: "correction", Commit: owners.status.BranchTip, ReadState: "read clean"})
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "new whole tip", code, result, intentConfirmed)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 2 || entries[0].State != plain.StateSuperseded || entries[1].SHA != owners.status.BranchTip {
		t.Fatalf("waiting tip was not replaced: %+v %v", entries, err)
	}
}

func TestWorkLandGoalProgressMatchesBuildAdmission(t *testing.T) {
	t.Parallel()
	b, owners, _ := admissionBed(t, "lane")
	writeLandingUnits(b, "u2. second", "u1 first")
	path := filepath.Join(b.root(), "plans", "designs", "landing-work.md")
	noTable := filepath.Join(t.TempDir(), "no-table.md")
	if err := os.WriteFile(noTable, []byte("# No units\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	owners.status.Status.Prefix = 0
	owners.status.Status.Units[0].ReadState = "read clean"
	progress, err := goalProgress([]string{noTable, path}, owners.status)
	if err != nil || progress.NoEnd || progress.Declared != 2 || progress.Unit != "u2. second" || progress.NeedsBuild || progress.Index != 1 {
		t.Fatalf("table order or build row matching differs: %+v %v", progress, err)
	}
	owners.status.Status.Units = []branch.UnitStatus{{Unit: "u1+u2", Units: []string{"u1", "u2"}, Commit: strings.Repeat("1", 40), ReadState: "read clean"},
		{Unit: "extra", Commit: strings.Repeat("4", 40)}}
	progress, err = goalProgress([]string{path}, owners.status)
	if err != nil || progress.Unit != "extra" || progress.Index != 1 || progress.NeedsBuild {
		t.Fatalf("package build or extra unit's read was missed: %+v %v", progress, err)
	}
	progress, err = goalProgress([]string{noTable}, owners.status)
	if err != nil || !progress.NoEnd {
		t.Fatalf("missing declaration: %+v %v", progress, err)
	}
}

func TestWorkLandAdmissionRefusalIsTwoLines(t *testing.T) {
	t.Parallel()
	b, _, _ := admissionBed(t, "lane")
	owners := b.intentBed.owners()
	owners.delivery, owners.connection, owners.work = b.owners, b.connection, b.work
	code, stdout, stderr := b.run(owners, "work", "land", "standing-validation")
	want := "✗ goal standing-validation is not finished: its design declares 3 units; u3 is not built. A goal\n" +
		"  lands whole, once.\n" +
		"  → metasystem work build standing-validation --work u3 --brief FILE --check COMMAND\n"
	if code == 0 || stdout != "" || stderr != want {
		t.Fatalf("refusal must give its reason and one command: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
