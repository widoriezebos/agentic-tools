package main

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestGoalShowSaysTheSavedClaimsAreas(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing", "malformed", "unreadable", "known", "known empty", "no reason", "no metadata"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, owners, _ := claimAreasBed(t)
			id := "second-goal"
			if scenario == "malformed" {
				claimAreaDesign(t, bed, id, "bad[")
			}
			if scenario == "known" || scenario == "known empty" {
				areas := "metasystem/owned/**"
				if scenario == "known empty" {
					areas = "[]"
				}
				claimAreaDesign(t, bed, id, areas)
			}
			code, result := bed.runJSON(owners, "goal", "claim", id)
			if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(id).Claimed == nil {
				t.Fatalf("claim: exit=%d result=%+v", code, result)
			}
			file := bed.goalFile(id)
			reason := "the claim recorded no reason"
			if len(file.Claimed.Warnings) > 0 {
				reason = strings.TrimLeft(strings.TrimPrefix(file.Claimed.Warnings[0], "areas unknown for goal "+id), ": ")
			}
			if scenario == "unreadable" {
				reason = "in design /x/d.md: open /x/d.md: permission denied"
				file.Claimed.AreaSnapshot = goal.AreaSnapshot{Warnings: []string{"areas unknown for goal " + id + " " + reason}}
			}
			if scenario == "no reason" {
				reason = "the claim recorded no reason"
				file.Claimed.AreaSnapshot = goal.AreaSnapshot{Source: "saved-design"}
			}
			other := "areas unknown for goal " + id + "-other: saved-other-reason"
			quota := "machine mac-cli also holds saved-held"
			file.Claimed.Warnings = append(file.Claimed.Warnings, other, quota)
			if scenario == "no metadata" {
				file.Claimed.AreaSnapshot = goal.AreaSnapshot{}
			}
			bed.addGoal(file)
			now, err := owners.commandNow(bed.root())
			if err != nil {
				t.Fatal(err)
			}
			env := layoutEnv(t, now, "", bed.root())
			owners.textEnv = func(io.Writer) textui.Env { return env }
			at, err := time.Parse(time.RFC3339, file.Claimed.At)
			if err != nil {
				t.Fatal(err)
			}
			before := bed.publications()
			for _, verbose := range []bool{false, true} {
				args := []string{"goal", "show", id}
				if verbose {
					args = append(args, "--verbose")
				}
				code, output, stderr := bed.run(owners, args...)
				if code != 0 || bed.publications() != before {
					t.Fatalf("show: exit=%d output=%s stderr=%s", code, output, stderr)
				}
				text := strings.Join(strings.Fields(output), " ")
				if strings.Contains(text, other) || strings.Contains(text, quota) {
					t.Fatalf("show repeated another saved warning: %s", output)
				}
				if scenario == "no metadata" {
					if strings.Contains(text, "Edit areas") {
						t.Fatalf("claim without metadata has an areas section: %s", output)
					}
				} else if file.Claimed.Known {
					if strings.Count(output, "\nEdit areas\n") != 1 || strings.Contains(text, "areas unknown") || scenario == "known" && !strings.Contains(text, "metasystem/owned/**") {
						t.Fatalf("known areas not shown: %s", output)
					}
				} else {
					want := "areas unknown since the claim at " + env.Time(at) + ": " + reason + "; overlap with other claims is not checked for it; the claim keeps what it read, and the next claim of this goal reads the design again"
					if strings.Count(text, "areas unknown") != 1 || !strings.Contains(text, want) || scenario == "malformed" && strings.Contains(text, "no accepted design") {
						t.Fatalf("show did not explain the saved areas: want %q; got %s", want, output)
					}
				}
				t.Logf("%v: %s", args, text)
			}
		})
	}
}

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
