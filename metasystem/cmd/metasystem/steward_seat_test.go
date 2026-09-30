package main

// The steward's seat (g1-s77 lane A) over the public goal verbs: the cap's
// reset by goal unapprove then goal approve (test 17's real-verb leg), and a
// review's clear-to-land or send-back starting the successor (test 22).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var seatBedNow = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// seatPlan is the seat ladder's selection over the bed's accepted ledger, as
// the steward's tick reads it, with the holder steps the gate names.
func seatPlan(t *testing.T, bed *intentBed, records []steward.SeatRecord, owned bool, now time.Time) (steward.Decision, *steward.SeatSelection, []goal.HolderStep) {
	t.Helper()
	endpoint, err := bed.dependencies().endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		t.Fatal(err)
	}
	work, err := goal.ClaimableWorkFromProjection(projection, "mac-cli", identity.KernelProber{})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := goal.ResolveGateSettings(filepath.Join(bed.root(), "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	// The bed's goal branch stands at the tip its review record names.
	world := steward.SeatWorldFrom(work, projection.Tree.Live, settings, map[string]string{bedGoal: reviewBedTip}, now)
	decision, selection := steward.PlanSeat(world, records, 3, owned)
	var steps []goal.HolderStep
	if file := projection.Tree.Live[bedGoal]; file != nil && file.Claimed != nil {
		steps = goal.HolderStepsDue([]*goal.GoalFile{file}, settings, now)
	}
	return decision, selection, steps
}

func noProgressSeats(id, opid string, n int) []steward.SeatRecord {
	var records []steward.SeatRecord
	for i := 0; i < n; i++ {
		records = append(records, steward.SeatRecord{Schema: 1, LaunchID: "seat-" + string(rune('a'+i)), Goal: id, ApprovalOpid: opid,
			StartedAt: seatBedNow.Add(time.Duration(i) * time.Minute).Format(time.RFC3339), ReapedAt: seatBedNow.Format(time.RFC3339),
			Outcome: steward.SeatNoProgress})
	}
	return records
}

// Test 17, the reset leg over the real verbs: an identical goal approve is
// the no-op of the approval owner and resets nothing; goal unapprove then
// goal approve is a new approval episode and starts again.
func TestSeatCapResetsOnlyOnUnapproveThenApprove(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	human := []string{"--fixture-human-authority", "--lineage", "m1"}
	if code, result := bed.runJSON(bed.owners(), append([]string{"goal", "approve", bedGoal}, human...)...); code != 0 {
		t.Fatalf("approve = %d %+v", code, result)
	}
	first := bed.goalFile(bedGoal).Approved.Opid
	records := noProgressSeats(bedGoal, first, 3)
	decision, selection, _ := seatPlan(t, bed, records, false, seatBedNow)
	if decision.Action != steward.ActNotify || selection != nil || !strings.Contains(decision.Reason, "3 seats ended without progress on "+bedGoal) {
		t.Fatalf("three no-progress seats cap the goal: %+v %+v", decision, selection)
	}

	bed.runJSON(bed.owners(), append([]string{"goal", "approve", bedGoal}, human...)...)
	if again := bed.goalFile(bedGoal).Approved.Opid; again != first {
		t.Fatalf("an identical approve moved the approval: %s -> %s", first, again)
	}
	if decision, selection, _ := seatPlan(t, bed, records, false, seatBedNow); decision.Action != steward.ActNotify || selection != nil {
		t.Fatalf("goal approve alone does not reset the cap: %+v %+v", decision, selection)
	}

	if code, result := bed.runJSON(bed.owners(), append([]string{"goal", "unapprove", bedGoal, "--reason", "the seat is stuck"}, human...)...); code != 0 {
		t.Fatalf("unapprove = %d %+v", code, result)
	}
	if code, result := bed.runJSON(bed.owners(), append([]string{"goal", "approve", bedGoal}, human...)...); code != 0 {
		t.Fatalf("approve after unapprove = %d %+v", code, result)
	}
	decision, selection, _ = seatPlan(t, bed, records, false, seatBedNow)
	if decision.Action != steward.ActRevive || selection == nil || selection.Goal != bedGoal || selection.ApprovalOpid == first {
		t.Fatalf("goal unapprove then goal approve starts again: %+v %+v", decision, selection)
	}
}

// seatLandingBed is the bed's goal built by the steward's seat and waiting
// to land at its tier (three, at or above human-from-tier).
func seatLandingBed(t *testing.T) *intentBed {
	t.Helper()
	bed := newIntentBed(t, false, func(file *goal.GoalFile) {
		waitingToLandBed(file)
		file.Claimed.Lineage = steward.SeatLineage
	})
	bed.lineage = steward.SeatLineage
	return bed
}

// Test 22.
func TestClearToLandAndSendBackStartTheSuccessor(t *testing.T) {
	t.Parallel()
	t.Run("before any word nothing starts", func(t *testing.T) {
		bed := seatLandingBed(t)
		decision, selection, steps := seatPlan(t, bed, nil, true, seatBedNow)
		if decision.Action != steward.ActNotify || selection != nil || len(steps) != 0 {
			t.Fatalf("a landing waiting for the person's word starts no successor: %+v %+v %+v", decision, selection, steps)
		}
	})
	t.Run("clear-to-land at the tip", func(t *testing.T) {
		bed := seatLandingBed(t)
		record := writeReviewRecord(t, bed.root(), "clear to land")
		if code, result := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "clear-to-land"); code != 0 {
			t.Fatalf("clear-to-land = %d %+v", code, result)
		}
		decision, selection, steps := seatPlan(t, bed, nil, true, seatBedNow)
		if decision.Action != steward.ActRevive || selection == nil || selection.Goal != bedGoal || !selection.Held {
			t.Fatalf("the next tick starts the successor naming G: %+v %+v", decision, selection)
		}
		if len(steps) != 1 || steps[0].Revise {
			t.Fatalf("its holder step is a landing: %+v", steps)
		}
	})
	t.Run("send-back with its brief", func(t *testing.T) {
		bed := seatLandingBed(t)
		record := writeReviewRecord(t, bed.root(), "send back")
		brief := filepath.Join(bed.root(), "fix.md")
		if err := os.WriteFile(brief, []byte("# Correction brief\n\n1. Read the reviewed tree (internal/owner.go:60).\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, result := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "send-back", "--brief", brief); code != 0 {
			t.Fatalf("send-back = %d %+v", code, result)
		}
		decision, selection, steps := seatPlan(t, bed, nil, true, seatBedNow)
		if decision.Action != steward.ActRevive || selection == nil || selection.Goal != bedGoal {
			t.Fatalf("the successor starts naming G: %+v %+v", decision, selection)
		}
		if len(steps) != 1 || !steps[0].Revise {
			t.Fatalf("its holder step is a revision: %+v", steps)
		}
		// The successor's Stop takes the revision from the published brief;
		// its send-back answer line ends the step.
		var calls []reviseCall
		owners := holderOwners(bed, &calls, func(int) intentResult { return attemptStarted(3) })
		if code, result := bed.runJSON(owners, "work", "revise", bedGoal); code != 0 && result.Outcome != intentInProgress {
			t.Fatalf("the successor's revision = %d %+v", code, result)
		}
		if len(calls) != 1 || !strings.Contains(calls[0].brief, "Read the reviewed tree") {
			t.Fatalf("the revision was not taken from the brief: %+v", calls)
		}
		decision, selection, steps = seatPlan(t, bed, nil, true, seatBedNow)
		if decision.Action == steward.ActRevive || selection != nil || len(steps) != 0 {
			t.Fatalf("after the send-back answer nothing is due and no successor starts: %+v %+v %+v", decision, selection, steps)
		}
	})
}

// The steward's launcher asks the launch lane for the seat kind in the
// checkout with the steward's id, brief and tag, and no goal; it reads a
// launch back as found, terminal, and its result document.
func TestStewardSeatLauncherStartsTheSeatKindInTheCheckout(t *testing.T) {
	t.Parallel()
	var asked []launch.StartSpec
	store := launch.Store{Root: t.TempDir()}
	launcher := stewardSeatLauncher{
		manager: func() *launch.Manager { return &launch.Manager{Store: store} },
		start: func(spec launch.StartSpec) (launch.Record, error) {
			asked = append(asked, spec)
			return launch.Record{ID: spec.ID}, nil
		},
		settings: func(string) (launch.Settings, error) {
			settings := launch.DefaultSettings()
			settings.SeatRuntime, settings.SeatModel = "claude", "claude-opus-5-5"
			return settings, nil
		},
		laneRoot: func() (string, bool, error) { return "", false, nil },
		repositoryTop: func(path string) (string, error) {
			if path != "/checkout/metasystem" {
				return "", fmt.Errorf("the checkout top is read from the state root, not %q", path)
			}
			return "/checkout", nil
		},
	}
	// The steward names its state root, the installation; the seat runs at
	// the checkout's top, where a person starts a session, and binds to the
	// fence the installation keeps.
	spec := steward.SeatLaunchSpec{ID: "seat-0011223344556677", StateRoot: "/checkout/metasystem", Brief: "/checkout/metasystem/brief.md", Tag: "0011223344556677"}
	if err := launcher.StartSeat(spec); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0].Kind != "seat" || asked[0].ID != spec.ID || asked[0].WorkingDirectory != "/checkout" ||
		asked[0].FenceRoot != spec.StateRoot || asked[0].Brief != spec.Brief || asked[0].Tag != spec.Tag || asked[0].Goal != "" {
		t.Fatalf("the seat start is the seat kind at the checkout top, fenced by the state root, naming no goal: %+v", asked)
	}
	unreadable := launcher
	unreadable.repositoryTop = func(string) (string, error) { return "", errors.New("not a git checkout") }
	if err := unreadable.StartSeat(spec); err == nil || len(asked) != 1 {
		t.Fatalf("a state root outside a checkout starts no seat: %v %+v", err, asked)
	}
	state, err := launcher.SeatLaunch(spec.ID)
	if err != nil || state.Found {
		t.Fatalf("a launch with no record is not found: %+v %v", state, err)
	}
	if err := store.Create(launch.Record{ID: spec.ID, Kind: "seat", State: launch.Completed, WorkingDirectory: "/checkout"}); err != nil {
		t.Fatal(err)
	}
	state, err = launcher.SeatLaunch(spec.ID)
	dir, _ := store.StateDir(spec.ID)
	if err != nil || !state.Found || !state.Terminal || state.State != "completed" || state.ResultPath != filepath.Join(dir, "result.json") {
		t.Fatalf("an ended launch reads terminal with its result document: %+v %v", state, err)
	}
}

// The runner is armed with the seat launcher.
func TestStewardRunIsArmedWithTheSeatLauncher(t *testing.T) {
	t.Parallel()
	var config steward.TickConfig
	wireStewardSeat(&config)
	launcher, ok := config.Seat.(stewardSeatLauncher)
	if !ok || launcher.repositoryTop == nil || launcher.settings == nil || launcher.laneRoot == nil {
		t.Fatalf("steward run starts seats through the launch lane, at the checkout top: %#v", config.Seat)
	}
}

// Amendment 1 (Wido, 2026-09-30): the launcher answers the steward's seat
// decision from the installation's own layered settings and the host's
// landing lane record, and refuses a seat start the answer forbids.
func TestStewardSeatLauncherStartsNoSeatWhenOffOrInTheLandingLane(t *testing.T) {
	t.Parallel()
	on := launch.DefaultSettings()
	on.SeatRuntime, on.SeatModel = "claude", "claude-opus-5-5"
	off := launch.DefaultSettings()
	for _, leg := range []struct {
		name     string
		settings launch.Settings
		lane     string
		reason   string
	}{
		{"off by default", off, "", "launch.seat.runtime=off"},
		{"the landing lane", on, "/checkout", "landing lane"},
		{"the landing lane named by its installation", on, "/checkout/metasystem", "landing lane"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			var asked []launch.StartSpec
			launcher := stewardSeatLauncher{
				start: func(spec launch.StartSpec) (launch.Record, error) {
					asked = append(asked, spec)
					return launch.Record{ID: spec.ID}, nil
				},
				repositoryTop: func(string) (string, error) { return "/checkout", nil },
				settings:      func(string) (launch.Settings, error) { return leg.settings, nil },
				laneRoot:      func() (string, bool, error) { return leg.lane, leg.lane != "", nil },
			}
			allowed, reason, err := launcher.SeatAllowed("/checkout/metasystem")
			if err != nil || allowed || !strings.Contains(reason, leg.reason) {
				t.Fatalf("SeatAllowed = %t %q %v, want refused naming %q", allowed, reason, err, leg.reason)
			}
			spec := steward.SeatLaunchSpec{ID: "seat-0011223344556677", StateRoot: "/checkout/metasystem", Brief: "/checkout/metasystem/brief.md", Tag: "0011223344556677"}
			if err := launcher.StartSeat(spec); err == nil || !strings.Contains(err.Error(), leg.reason) || len(asked) != 0 {
				t.Fatalf("a forbidden seat start = %v, asked %+v", err, asked)
			}
		})
	}
	t.Run("on, and not the landing lane", func(t *testing.T) {
		t.Parallel()
		launcher := stewardSeatLauncher{
			repositoryTop: func(string) (string, error) { return "/checkout", nil },
			settings:      func(string) (launch.Settings, error) { return on, nil },
			laneRoot:      func() (string, bool, error) { return "/elsewhere-landing", true, nil },
		}
		if allowed, reason, err := launcher.SeatAllowed("/checkout/metasystem"); err != nil || !allowed {
			t.Fatalf("a seat turned on outside the landing lane is allowed: %t %q %v", allowed, reason, err)
		}
	})
}
