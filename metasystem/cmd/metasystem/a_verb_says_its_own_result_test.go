package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestAnActSaysOnlyItsOwnResult(t *testing.T) {
	t.Parallel()
	warnings := []string{"areas unknown for goal other-goal", "machine mac-cli already claims gone-goal, conclude it before claiming another"}
	bed := newIntentBed(t, false, func(file *goal.GoalFile) {
		file.Claimed.Warnings = slices.Clone(warnings)
	})
	before := bed.publications()
	code, result := bed.runJSON(bed.terminalOwners(), "goal", "notes", bedGoal, "--read", "r2", "--add", "help wraps at 80 columns")
	if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before+1 {
		t.Fatalf("note not published: exit=%d result=%+v", code, result)
	}
	file := bed.goalFile(bedGoal)
	if !strings.Contains(string(goal.RenderFile(file)), "help wraps at 80 columns") {
		t.Fatalf("published goal has no note: %+v", file)
	}
	for _, warning := range warnings {
		if strings.Contains(result.Summary, warning) {
			t.Fatalf("notes repeated a saved claim warning: %s", result.Summary)
		}
	}
}

func TestAHandInSaysOnlyItsOwnWarnings(t *testing.T) {
	t.Parallel()
	bed, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	file := bed.goalFile(bedGoal)
	warnings := []string{"areas unknown for goal old-other", "machine mac-cli already claims gone-goal"}
	file.Claimed.AreaSnapshot = goal.AreaSnapshot{Warnings: slices.Clone(warnings)}
	bed.addGoal(file)
	code, result := bed.do("work", "land", bedGoal)
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	entry, seen, err := plain.Latest(install, bedGoal)
	if err != nil || !seen || !slices.Equal(entry.Warnings, warnings) {
		t.Fatalf("queue lost the saved snapshot: entry=%+v seen=%t err=%v", entry, seen, err)
	}
	if strings.Contains(result.Summary, "warning:") {
		t.Fatalf("hand-in repeated saved warnings: %s", result.Summary)
	}
}

func TestAnArcClaimSaysTheClaimsItWrote(t *testing.T) {
	t.Parallel()
	bed, owners, _ := claimLaneBed(t)
	first := bed.goalFile(bedGoal)
	first.Arc = "a"
	first.Claimed.Warnings = []string{"areas unknown for goal old-other"}
	bed.addGoal(first)
	second := bed.goalFile("second-goal")
	second.Arc = first.Arc
	bed.addGoal(second)
	code, result := bed.runJSON(owners, "goal", "claim", bedGoal, "--arc")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile("second-goal").State != goal.StateClaimed {
		t.Fatalf("arc member not claimed: exit=%d result=%+v", code, result)
	}
	if !strings.Contains(result.Summary, "goal second-goal: areas unknown for goal second-goal") || strings.Contains(result.Summary, "old-other") {
		t.Fatalf("arc claim did not say only the new claim's warnings: %s", result.Summary)
	}
}

func TestAClaimNeverRepeatsTheWaitingGoalsWarnings(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"named", "arc"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, owners, lane := claimLaneBed(t)
			first := bed.goalFile(bedGoal)
			first.Claimed.Warnings = []string{"areas unknown for goal old-frozen"}
			bed.addGoal(first)
			queueClaimGoal(t, lane.install, bedGoal)
			args := []string{"goal", "claim", "second-goal"}
			if mode == "arc" {
				args = append(args, "--arc")
			}
			code, result := bed.runJSON(owners, args...)
			if code != 0 || result.Outcome != intentConfirmed {
				t.Fatalf("claim while waiting: exit=%d result=%+v", code, result)
			}
			first, second := bed.goalFile(bedGoal), bed.goalFile("second-goal")
			if second.State != goal.StateClaimed || second.Claimed == nil {
				t.Fatalf("second goal was not claimed: %+v", second)
			}
			ready, claimed := first.History[len(first.History)-1], second.History[len(second.History)-1]
			if ready.Verb != "land-ready" || claimed.Verb != "claim" || claimed.Opid == "" || ready.Opid != claimed.Opid {
				t.Fatalf("waiting goal and claim are not one operation: ready=%+v claimed=%+v", ready, claimed)
			}
			t.Logf("claim confirmed with a matching land-ready row; summary: %s", result.Summary)
			if strings.Contains(result.Summary, "old-frozen") || strings.Contains(result.Summary, "goal "+bedGoal+":") {
				t.Fatalf("claim repeated the waiting goal's saved warnings: %s", result.Summary)
			}
		})
	}
}
