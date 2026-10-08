package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func TestPersonClaimFreshStartAdoptsEveryRemoteReservation(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"session", "up"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			bed, owners := personClaimsBed(t)
			first := bed.goalFile(bedGoal)
			first.Id = "a-current"
			bed.addGoal(first)
			reservePersonGoal(t, bed, owners, first.Id)
			owners = adoptingHolder(owners, launch.SeatOwnerLineage, 9)
			adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 9)
			old := bed.repo.accepted
			second := bed.goalFile(bedGoal)
			second.Id = "z-second"
			bed.addGoal(second)
			reservationOwners := owners
			reservationOwners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
				return lease.CurrentHolderView{}, lease.ErrLeaseAbsent
			}
			for _, id := range []string{bedGoal, second.Id} {
				reservePersonGoal(t, bed, reservationOwners, id)
			}
			reserved := map[string]*goal.GoalFile{}
			for _, id := range []string{first.Id, bedGoal, second.Id} {
				reserved[id] = bed.goalFile(id)
				wantEpoch := int64(0)
				if id == first.Id {
					wantEpoch = 9
				}
				if capability := reserved[id].StopCapability; capability == nil || capability.ClaimEpoch != wantEpoch {
					t.Fatalf("reservation %s must start at epoch %d: %+v", id, wantEpoch, capability)
				}
			}
			remote := bed.repo.accepted
			// The remote contains reservations this checkout has not observed.
			bed.repo.accepted = old
			attempts := 0
			endpoint := owners.dependencies.endpoint
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				e, err := endpoint(root)
				e.Repository = freshCommandRepository{Repository: e.Repository, attempts: &attempts}
				return e, err
			}
			processes := defaultProcessIntentOwners()
			processes.process.repositoryTop = fakeTop(bed.root())
			processes.executable = os.Executable
			processes.adoptionReads = &claimAdoptionReads{dependencies: owners.dependencies, clock: owners.commandNow, project: goal.Project}
			var actual up.StopCapabilityRestampResult
			processes.up = func(options up.Options) up.Result {
				var err error
				actual, err = options.RestampStopCapability(bed.root(), launch.SeatOwnerLineage, 9)
				if err != nil {
					t.Fatal(err)
				}
				return up.Result{Outcome: "armed", Adoption: &actual, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "restamped", Adoption: &actual}}}
			}
			var out, diagnostic bytes.Buffer
			if code := (hookOwners{processes: &processes}).Up(hooks.UpRequest{MetasystemRoot: stateroot.Installation(bed.root()), Repo: bed.root()}, &out, &diagnostic); code != 0 || attempts != 0 || len(actual.Goals) != 1 || bed.repo.accepted != old {
				t.Fatalf("hook fetched remote reservations: exit=%d attempts=%d result=%+v out=%s err=%s", code, attempts, actual, &out, &diagnostic)
			}
			invoke := func() int {
				out.Reset()
				diagnostic.Reset()
				if verb == "up" {
					return runUpWithProcessOwners([]string{"--repo", bed.root(), "--metasystem-root", bed.root(), "--json"}, processes, &out, &diagnostic)
				}
				owners.processes = processes
				code, result := bed.runJSON(owners, "session", "start")
				if result.Outcome != intentConfirmed {
					t.Fatalf("session preparation: %+v", result)
				}
				return code
			}
			if code := invoke(); code != 0 || attempts != 1 || actual.Status != "complete" || actual.Pending || actual.Observation == nil || actual.Observation.Tip != remote || actual.Observation.Outcome != "fresh" || len(actual.Goals) != 3 {
				t.Fatalf("explicit %s missed remote reservations: exit=%d attempts=%d result=%+v out=%s err=%s", verb, code, attempts, actual, &out, &diagnostic)
			}
			for i, id := range []string{first.Id, bedGoal, second.Id} {
				want := "restamped"
				if i == 0 {
					want = "current"
				}
				file := bed.goalFile(id)
				if actual.Goals[i].GoalID != id || actual.Goals[i].Outcome != want || file.StopCapability.ClaimEpoch != 9 || !reflect.DeepEqual(file.Claimed, reserved[id].Claimed) || !reflect.DeepEqual(file.Budget, reserved[id].Budget) || !reflect.DeepEqual(file.Approved, reserved[id].Approved) {
					t.Fatalf("adoption changed ownership or accounting for %s: result=%+v file=%+v", id, actual, file)
				}
				if !strings.Contains(strings.Join((up.Result{Adoption: &actual, Components: []up.ComponentOutcome{{Adoption: &actual}}}).Lines(), "\n"), "goal="+id+" adoption="+want) {
					t.Fatalf("adoption output omitted %s: %+v", id, actual)
				}
			}
			published := bed.publications()
			if code := invoke(); code != 0 || attempts != 2 || actual.Pending || len(actual.Goals) != 3 || bed.publications() != published {
				t.Fatalf("repeat %s changed the claim episode: exit=%d attempts=%d result=%+v", verb, code, attempts, actual)
			}
			for _, outcome := range actual.Goals {
				if outcome.Outcome != "current" {
					t.Fatalf("repeat adoption was not idempotent: %+v", actual)
				}
			}
		})
	}
}
