package steward

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func personSeatReservation(id, lineage string, epoch int64, approved bool) *goal.GoalFile {
	f := seatClaimedGoal(id, lineage)
	f.Claimed.By = "human:Wido"
	f.Claimed.EpisodeAt, f.Claimed.EpisodeRevision = f.Claimed.At, f.Claimed.Revision
	f.History[2].Actor = "human:Wido"
	f.History[2].AuthorityOutcome = goal.AuthorityOutcomeHumanAuthorityProven
	f.History[2].AuthorityGeneration = 1
	f.StopCapability.ClaimEpoch = epoch
	if !approved {
		f.Approved = nil
		f.Budget = nil
	}
	return f
}

func TestPersonClaimStewardWaitsBeforeAndAfterAdoption(t *testing.T) {
	t.Parallel()
	for _, epoch := range []int64{0, 7} {
		for _, budget := range []bool{false, true} {
			t.Run(time.Duration(epoch).String()+"/budget="+map[bool]string{false: "absent", true: "present"}[budget], func(t *testing.T) {
				t.Parallel()
				f := personSeatReservation("reserved", SeatLineage, epoch, false)
				if budget {
					f.Budget = stewardBudget()
				}
				before := goal.RenderFile(f)
				bed := newSeatBed(t, f)
				bed.now = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
				result := bed.tick(deadWorkers)
				if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "awaits a person's approval") || !strings.Contains(result.Decision.Reason, "this seat holds reserved") {
					t.Fatalf("reservation revived or lost held-work notification: %+v", result)
				}
				work, err := goal.ClaimableWorkFromProjection(bed.projection(bed.now), seatBedMachine, seatBusyDeadProber{})
				if err != nil {
					t.Fatal(err)
				}
				world := SeatWorldFrom(work, bed.goals, bed.settings, bed.tips, bed.now)
				if len(world.Held) != 1 || world.Held[0].StepDue || !strings.Contains(world.Held[0].Wait, "approval") {
					t.Fatalf("adopted unapproved ownership is executable or hidden: %+v", world)
				}
				bed.put(seatReadyGoal("unrelated-ready", "Build it."))
				result = bed.tick(deadWorkers)
				if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "this seat holds reserved") {
					t.Fatalf("unrelated ready work bypassed held reservation: %+v", result)
				}
				if !reflect.DeepEqual(before, goal.RenderFile(bed.goals["reserved"])) || len(bed.launcher.starts) != 0 {
					t.Fatal("waiting changed ownership/accounting or started a seat")
				}
				bed.drop("reserved")
				result = bed.tick(deadWorkers)
				if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "unrelated-ready" {
					t.Fatalf("removing held reservation did not restore ready selection: %+v", result)
				}
			})
		}
	}
}

func TestPersonClaimRunnerNeverGenericallyRevivesReservation(t *testing.T) {
	t.Parallel()
	for _, lineage := range []string{SeatLineage, "hand-started-owner"} {
		for _, epoch := range []int64{0, 7} {
			t.Run(lineage+"/"+time.Duration(epoch).String(), func(t *testing.T) {
				t.Parallel()
				f := personSeatReservation("reserved", lineage, epoch, false)
				bed := newSeatBed(t, f)
				bed.now = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
				before := goal.RenderFile(f)
				revivals, launches, ticks := 0, 0, 0
				deps := runnerLoopDependencies{
					Now: func() time.Time { return bed.now }, Sleep: func(time.Duration) {
						if err := stopRunnerLoop(bed.root); err != nil {
							t.Fatal(err)
						}
					},
					DeliverPending: func(string) (int, error) { return 0, nil },
					Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
						ticks++
						work := bed.dependencies()
						work.Seat = nil
						result, err := decideTickWithDependencies(bed.root, TickConfig{Now: bed.now}, fakeCensus{workers: deadWorkers}, Evidence{}, Marks{}, work)
						if err == nil && (result.Decision.Action != ActNotify || !strings.Contains(result.Decision.Reason, "awaits a person's approval")) {
							t.Fatalf("generic classification regained revival authority: %+v", result)
						}
						return result, err
					},
					StartSeat: func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
						launches++
						return SeatRecord{}, nil
					},
				}
				if err := runLoopWithDependencies(bed.root, fakeCensus{workers: deadWorkers}, func() error { revivals++; return nil }, time.Minute, TickConfig{Now: bed.now}, deps); err != nil {
					t.Fatal(err)
				}
				if ticks != 1 || revivals != 0 || launches != 0 || !reflect.DeepEqual(before, goal.RenderFile(f)) {
					t.Fatalf("reservation caused effects: ticks=%d delegates=%d seats=%d", ticks, revivals, launches)
				}
			})
		}
	}
}

func TestPersonClaimApprovedZeroEpochStartsItsSeatAndRechecksApproval(t *testing.T) {
	t.Parallel()
	for _, loseApproval := range []bool{false, true} {
		t.Run(map[bool]string{false: "starts adopter", true: "approval lost"}[loseApproval], func(t *testing.T) {
			t.Parallel()
			f := personSeatReservation("reserved", SeatLineage, 0, true)
			bed := newSeatBed(t, f)
			bed.now = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
			before := goal.RenderFile(f)
			delegates, ticks := 0, 0
			deps := runnerLoopDependencies{
				Now: func() time.Time { return bed.now }, Sleep: func(time.Duration) { _ = stopRunnerLoop(bed.root) }, DeliverPending: func(string) (int, error) { return 0, nil },
				Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
					ticks++
					r := bed.tick(deadWorkers)
					if r.Decision.Action != ActRevive || r.Seat == nil || r.Seat.Goal != "reserved" || !r.Seat.Held {
						t.Fatalf("approved zero reservation did not select its adopter: %+v", r)
					}
					if loseApproval {
						f.Approved = nil
					}
					return r, nil
				},
				StartSeat: func(_ string, _ TickConfig, census WorkerCensus, selection SeatSelection) (SeatRecord, error) {
					return bed.startUnder(selection, census)
				},
			}
			if err := runLoopWithDependencies(bed.root, fakeCensus{workers: deadWorkers}, func() error { delegates++; return nil }, time.Minute, TickConfig{Now: bed.now}, deps); err != nil {
				t.Fatal(err)
			}
			want := 1
			if loseApproval {
				want = 0
			}
			if ticks != 1 || len(bed.launcher.starts) != want || delegates != 0 {
				t.Fatalf("wrong reservation launch: ticks=%d seats=%d delegates=%d", ticks, len(bed.launcher.starts), delegates)
			}
			if !loseApproval && !reflect.DeepEqual(before, goal.RenderFile(f)) {
				t.Fatal("selecting the adopter changed ownership or accounting")
			}
		})
	}
}
