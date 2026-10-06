package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func claimLaneBed(t *testing.T) (*intentBed, intentOwners, *resolveVerbFixture) {
	t.Helper()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	announceProofFixtureHolder(t, bed.root())
	for _, id := range []string{"second-goal", "third-goal"} {
		file := *bed.goalFile(bedGoal)
		file.Id, file.State, file.Claimed = id, goal.StateApproved, nil
		file.History = slices.Clone(file.History)
		for index := range file.History {
			file.History[index].Targets = []string{id}
		}
		bed.addGoal(&file)
	}
	lane := newResolveVerbFixture(t)
	lane.owners.landing.person = func(string) (string, error) { return "", errors.New("no enrolled person") }
	owners := bed.owners()
	delivery := defaultIntentDeliveryOwners()
	delivery.laneRoot = func(string, time.Time) (string, bool, error) { return lane.root, true, nil }
	delivery.laneInstall = func(string) (string, error) { return lane.install, nil }
	delivery.laneLatest = func(install, id, main string) (plain.Entry, bool, error) {
		if install != lane.install || main == "" {
			t.Fatalf("lane read install=%q main=%q", install, main)
		}
		return plain.Latest(install, id)
	}
	delivery.laneContains = func(sha, main string) (bool, error) {
		if main == "" {
			t.Fatalf("lane containment read sha=%q main=%q", sha, main)
		}
		return false, nil
	}
	owners.delivery = delivery
	return bed, owners, lane
}

func queueClaimGoal(t *testing.T, install, id string) {
	t.Helper()
	if _, _, err := plain.HandIn(install, plain.Line{Goal: id, Branch: "goal/" + id, SHA: id + "-sha"}); err != nil {
		t.Fatal(err)
	}
}

func TestGoalClaimLaneWaitingThenReturned(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"named", "ready", "arc"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, owners, lane := claimLaneBed(t)
			queueClaimGoal(t, lane.install, bedGoal)
			before := bed.publications()
			args := []string{"goal", "claim"}
			if mode != "ready" {
				args = append(args, "second-goal")
			}
			if mode == "arc" {
				args = append(args, "--arc")
			}
			code, result := bed.runJSON(owners, args...)
			if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before+1 {
				t.Fatalf("claim while waiting: exit=%d result=%+v publications=%d, want %d", code, result, bed.publications(), before+1)
			}
			first, second := bed.goalFile(bedGoal), bed.goalFile("second-goal")
			if first.Landing == nil || second.Claimed == nil || second.Claimed.Machine != "mac-cli" {
				t.Fatalf("claim and landing slot not both recorded: first=%+v second=%+v", first, second)
			}
			ready, claimed := first.History[len(first.History)-1], second.History[len(second.History)-1]
			if ready.Verb != "land-ready" || claimed.Verb != "claim" || ready.Opid != claimed.Opid || first.Landing.Opid != claimed.Opid {
				t.Fatalf("landing slot and claim are not one operation: ready=%+v claimed=%+v", ready, claimed)
			}
			writeCauseProof(t, lane.install, "results.jsonl", plain.Result{Result: plain.Red, At: "2026-10-06T10:00:00Z",
				Goals: []plain.GoalSHA{{Goal: bedGoal, SHA: bedGoal + "-sha"}},
				Cause: &plain.Cause{Kind: "own", Goal: bedGoal, SHA: bedGoal + "-sha", Tests: []string{"TestWidget"}, Evidence: "proof.log"}})
			if code, output := lane.run(t, lane.root, "return", bedGoal, "--cause", "own", "--reason", "the widget check failed"); code != 0 {
				t.Fatalf("landing return exit=%d: %s", code, output)
			}
			before = bed.publications()
			args = []string{"goal", "claim"}
			if mode != "ready" {
				args = append(args, "third-goal")
			}
			if mode == "arc" {
				args = append(args, "--arc")
			}
			code, result = bed.runJSON(owners, args...)
			if code == 0 || result.Outcome != intentRefused || bed.publications() != before {
				t.Fatalf("returned goal allowed a claim or publication: exit=%d result=%+v", code, result)
			}
			if !strings.Contains(result.Summary, bedGoal) || !strings.Contains(result.Summary, "returned") || result.Next == nil ||
				!slices.Equal(result.Next.Argv, []string{"metasystem", "work", "land", bedGoal}) {
				t.Fatalf("returned goal did not count or name its remedy: %+v", result)
			}
		})
	}
}

func TestGoalClaimLaneQuotaBoundAndStates(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"two waiting", "landed", "returned then records", "no entry", "no lane", "queue unreadable", "installation unreadable", "existing slot"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, owners, lane := claimLaneBed(t)
			delivery := owners.delivery
			wantOutcome := intentRefused
			wantGoal, wantRemedy := bedGoal, []string{"metasystem", "goal", "release", bedGoal + ",", "then", "metasystem", "goal", "claim", "second-goal"}
			switch scenario {
			case "two waiting", "existing slot":
				queueClaimGoal(t, lane.install, bedGoal)
				if scenario == "two waiting" {
					if code, result := bed.runJSON(owners, "goal", "claim", "second-goal"); code != 0 {
						t.Fatalf("second claim: exit=%d result=%+v", code, result)
					}
					queueClaimGoal(t, lane.install, "second-goal")
				} else {
					second := bed.goalFile("second-goal")
					second.State, second.Claimed = goal.StateClaimed, bed.goalFile(bedGoal).Claimed
					second.Landing = &goal.LandingRecord{At: second.Claimed.At, Opid: second.History[len(second.History)-1].Opid}
					bed.addGoal(second)
				}
			case "landed":
				queueClaimGoal(t, lane.install, bedGoal)
				first := bed.goalFile(bedGoal)
				first.Landing = &goal.LandingRecord{At: first.Claimed.At, Opid: first.History[len(first.History)-1].Opid}
				bed.addGoal(first)
				delivery.laneContains = func(string, string) (bool, error) {
					return true, nil
				}
				wantRemedy = []string{"metasystem", "goal", "done", bedGoal, "--reason", "TEXT"}
			case "returned then records":
				queueClaimGoal(t, lane.install, bedGoal)
				if _, changed, err := plain.Return(lane.install, bedGoal, "the widget check failed", time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)); err != nil || !changed {
					t.Fatalf("return: changed=%v err=%v", changed, err)
				}
				if _, added, err := plain.HandIn(lane.install, plain.Line{Goal: bedGoal, Branch: "records/" + bedGoal, SHA: "records-sha", Records: true}); err != nil || !added {
					t.Fatalf("records hand-in: added=%v err=%v", added, err)
				}
				wantRemedy = []string{"metasystem", "work", "land", bedGoal}
			case "no lane":
				delivery.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
			case "queue unreadable":
				if err := os.MkdirAll(filepath.Join(plain.Dir(lane.install), "queue.jsonl"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "installation unreadable":
				delivery.laneInstall = func(string) (string, error) { return "", errors.New("installation unavailable") }
			}
			before := bed.publications()
			id := "second-goal"
			if scenario == "two waiting" || scenario == "existing slot" {
				id = "third-goal"
			}
			code, result := bed.runJSON(owners, "goal", "claim", id)
			if code == 0 || result.Outcome != wantOutcome || bed.publications() != before {
				t.Fatalf("quota allowed a claim or publication: exit=%d result=%+v", code, result)
			}
			if !strings.Contains(result.Summary, wantGoal) {
				t.Fatalf("refusal did not name %q: %+v", wantGoal, result)
			}
			if scenario == "two waiting" || scenario == "existing slot" {
				if !strings.Contains(result.Summary, "second-goal") || !strings.Contains(result.Summary, "one landing slot") ||
					!slices.Contains(result.Details, "refusal code: "+goal.ClaimQuotaCode) {
					t.Fatalf("slot bound did not name both goals: %+v", result)
				}
			} else if scenario == "no entry" || scenario == "no lane" || scenario == "queue unreadable" || scenario == "installation unreadable" {
				if !strings.Contains(result.Summary, "run: "+strings.Join(wantRemedy, " ")) {
					t.Fatalf("historical quota remedy: %+v", result)
				}
			} else if result.Next == nil || !slices.Equal(result.Next.Argv, wantRemedy) {
				t.Fatalf("refusal remedy: %+v, want %v", result.Next, wantRemedy)
			}
			if scenario == "returned then records" && !strings.Contains(result.Summary, "returned") {
				t.Fatalf("records hid the goal's return: %+v", result)
			}
			if scenario == "queue unreadable" {
				code, result = bed.runJSON(owners, "goal", "claim")
				if code != 0 || result.Outcome != intentUnchanged || bed.publications() != before ||
					!strings.Contains(result.Summary, "continue it") {
					t.Fatalf("unreadable queue on ready claim: exit=%d result=%+v", code, result)
				}
			}
		})
	}
}

func TestGoalClaimLaneRootUnreadable(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"named", "ready", "arc"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, owners, _ := claimLaneBed(t)
			first := bed.goalFile(bedGoal)
			first.State, first.Claimed = goal.StateApproved, nil
			bed.addGoal(first)
			reads := 0
			owners.delivery.laneRoot = func(string, time.Time) (string, bool, error) {
				reads++
				return "", false, errors.New("lane root unavailable")
			}
			args := []string{"goal", "claim"}
			if mode != "ready" {
				args = append(args, bedGoal)
			}
			if mode == "arc" {
				args = append(args, "--arc")
			}
			before := bed.publications()
			code, result := bed.runJSON(owners, args...)
			if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before+1 || reads != 0 {
				t.Fatalf("first claim: exit=%d result=%+v publications=%d reads=%d", code, result, bed.publications(), reads)
			}
			heldID := result.Targets[0].ID
			if claimed := bed.goalFile(heldID); claimed.Claimed == nil || claimed.Claimed.Machine != "mac-cli" || claimed.Landing != nil {
				t.Fatalf("first claim not recorded: %+v", claimed)
			}
			args = []string{"goal", "claim", "third-goal"}
			if mode == "arc" {
				args = append(args, "--arc")
			}
			before = bed.publications()
			code, result = bed.runJSON(owners, args...)
			if code == 0 || result.Outcome != intentRefused || bed.publications() != before ||
				!strings.Contains(result.Summary, "already claims "+heldID) ||
				!strings.Contains(result.Summary, "a machine holds one claim at a time") ||
				!slices.Contains(result.Details, "refusal code: "+goal.ClaimQuotaCode) {
				t.Fatalf("unreadable lane changed the quota: exit=%d result=%+v", code, result)
			}
			if held := bed.goalFile(heldID); held.Landing != nil || bed.goalFile("third-goal").Claimed != nil {
				t.Fatalf("refused claim changed held work: %+v", held)
			}
		})
	}
}
