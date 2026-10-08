package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type unitStopProber struct{ alive *bool }

func (p unitStopProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if *p.alive {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

func TestIntentUnitLaunchBreachStopCancelsExecution(t *testing.T) {
	t.Parallel()
	for _, prior := range []string{"pending", "cancelled"} {
		t.Run(prior, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			bed.starter.hold = "build"
			brief := bed.brief("stop.md", "Build the unit.\n")
			code, result, _ := bed.work(append([]string{"work", "build", bed.id, "stop", "--brief", brief, "--lines", "5"}, workCheck...)...)
			if code != 3 {
				t.Fatalf("held build: %d %+v", code, result)
			}
			run := resultData(t, result)["run"].(string)
			unit, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil {
				t.Fatal(err)
			}
			id := unit.Rounds[0].Steps[0].LaunchID
			path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", id+".json")
			reservation, err := dispatchcore.ReadRecordObject(path)
			if err != nil {
				t.Fatal(err)
			}
			reservation["status"] = prior
			transferWriteJSON(t, path, reservation)
			file := bed.goalFile(bed.id)
			stamp := bed.manager.Now().UTC().Format(time.RFC3339)
			batch := goal.StopBatch{StopID: "stop-held-unit", GoalID: bed.id, GoalRevision: file.Claimed.Revision,
				FenceEpoch: 1, CapabilityGeneration: file.StopCapability.Generation, Machine: file.Claimed.Machine,
				ClaimEpoch: file.StopCapability.ClaimEpoch, Reason: goal.StopReasonElapsedLimit,
				State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp}
			if err := goal.WriteStopBatch(bed.root(), batch); err != nil {
				t.Fatal(err)
			}
			alive, finish, observations := true, false, 0
			bed.manager.Prober = unitStopProber{&alive}
			bed.manager.Sleep = func(time.Duration) {
				observations++
				current, err := goal.ReadStopBatch(bed.root(), batch.StopID)
				if err != nil || current.State != goal.StopBatchOpen || len(current.Pending) != 1 || current.Pending[0] != id {
					t.Fatalf("stop completed before its execution ended: %+v %v", current, err)
				}
				currentReservation, err := dispatchcore.ReadRecordObject(path)
				if err != nil || currentReservation["status"] != prior {
					t.Fatalf("reservation marked before launch cancellation: %+v %v", currentReservation, err)
				}
				if finish {
					alive = false
				}
			}
			ports := fake.NewSet()
			ports.Clock.Current = bed.manager.Now()
			ports.Lease.ClassifyFunc = func(delegation.Invocation) (lease.ClassifyResult, error) {
				return lease.ClassifyResult{Class: lease.ClassSteward}, nil
			}
			ports.Goal.BreachStopFunc = func(string, uint64, time.Time, string) (goal.StopBatch, error) {
				return goal.ReadStopBatch(bed.root(), batch.StopID)
			}
			ports.Records.CASFunc = func(job, expect, target, patch string) (string, error) {
				execution, err := bed.manager.Store.Read(job)
				if err != nil || !execution.State.Terminal() {
					t.Fatalf("reservation marked while launch still runs: %+v %v", execution, err)
				}
				return dispatchcore.RecordCAS(bed.root(), job, expect, target, patch)
			}
			owners := ports.Ports()
			owners.Host = engineHost{launches: bed.manager}
			life, err := delegation.New(delegation.Config{Root: bed.root(), RepoScope: bed.root()}, owners)
			if err != nil {
				t.Fatal(err)
			}
			invoke := func() delegation.Result {
				var stderr bytes.Buffer
				result := life.Run(context.Background(), delegation.Request{Env: delegation.Env{DelegateInternal: true}, Stderr: &stderr},
					[]string{"__breach-stop-goal", "--goal", bed.id, "--revision", fmt.Sprint(file.Claimed.Revision)})
				if result.ExitCode != 0 && !strings.Contains(stderr.String(), "could not prove every recorded process dead") {
					t.Fatalf("unexpected stop failure: %d %s", result.ExitCode, &stderr)
				}
				return result
			}
			if result := invoke(); result.ExitCode != 1 || observations == 0 {
				t.Fatalf("a live launch was treated as stopped: %+v observations=%d", result, observations)
			}
			finish = true
			if result := invoke(); result.ExitCode != 0 || !strings.Contains(string(result.Stdout), "state=COMPLETE") {
				t.Fatalf("stop did not complete after cancellation: %+v", result)
			}
			execution, err := bed.manager.Store.Read(id)
			if err != nil || execution.State != launch.Cancelled || execution.FinishedAt == "" {
				t.Fatalf("execution not cancelled: %+v %v", execution, err)
			}
			batch, err = goal.ReadStopBatch(bed.root(), batch.StopID)
			if err != nil || batch.State != goal.StopBatchComplete || len(batch.Pending) != 0 || len(batch.Terminal) != 1 || batch.Terminal[0] != id ||
				len(batch.Observed) != 1 || batch.Observed[0].Status != "cancelled" || batch.Observed[0].Disposition != "LOCAL_TERMINAL" ||
				len(batch.CancelOutcomes) != 1 || batch.CancelOutcomes[0].Outcome != "CANCELLED" {
				t.Fatalf("batch did not complete: %+v %v", batch, err)
			}
		})
	}
}

func TestIntentUnitLaunchStopCancelsNeverStartedReservation(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"breach", "work-stop"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			bed.starter.hold = "build"
			brief := bed.brief("stop.md", "Build the unit.\n")
			code, result, _ := bed.work(append([]string{"work", "build", bed.id, "stop", "--brief", brief, "--lines", "5"}, workCheck...)...)
			if code != 3 {
				t.Fatalf("held build: %d %+v", code, result)
			}
			run := resultData(t, result)["run"].(string)
			unit, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil {
				t.Fatal(err)
			}
			id := unit.Rounds[0].Steps[0].LaunchID
			path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", id+".json")
			reservation, err := dispatchcore.ReadRecordObject(path)
			if err != nil || reservation["status"] != "pending" {
				t.Fatalf("build has no pending reservation: %+v %v", reservation, err)
			}
			stateDir, err := bed.manager.Store.StateDir(id)
			if err != nil {
				t.Fatal(err)
			}
			// A reservation survives even when no execution was started.
			if err := os.RemoveAll(stateDir); err != nil {
				t.Fatal(err)
			}
			if _, err := bed.manager.Store.Read(id); !os.IsNotExist(err) {
				t.Fatalf("launch record still exists: %v", err)
			}
			file := bed.goalFile(bed.id)
			capability := *file.StopCapability
			stopID := fmt.Sprintf("stop-%s-r%d-f%d", bed.id, file.Claimed.Revision, capability.FenceEpoch+1)
			endpoint, err := bed.dependencies().endpoint(bed.root())
			if err != nil {
				t.Fatal(err)
			}
			closed, err := goal.CloseStop(goal.CloseStopRequest{
				VerbRequest: goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: "goal-stop-custodian", Human: "Wido"},
					Ulid: "01ARZ3NDEKTSV4RRFFQ69G7S03", Now: bed.manager.Now(), ClaimEpoch: capability.ClaimEpoch},
				GoalID: bed.id, StopID: stopID, Reason: goal.StopReasonElapsedLimit, Capability: capability,
			})
			if err != nil || closed.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("stop fence did not close: %+v %v", closed, err)
			}
			fenced := bed.goalFile(bed.id)
			stamp := bed.manager.Now().UTC().Format(time.RFC3339)
			batch := goal.StopBatch{StopID: stopID, GoalID: bed.id, GoalRevision: file.Claimed.Revision,
				FenceEpoch: fenced.StopFence.Epoch, CapabilityGeneration: capability.Generation, Machine: capability.Machine,
				ClaimEpoch: capability.ClaimEpoch, Reason: fenced.StopFence.Reason,
				State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp}
			if err := goal.WriteStopBatch(bed.root(), batch); err != nil {
				t.Fatal(err)
			}
			ports := fake.NewSet()
			ports.Clock.Current = bed.manager.Now()
			ports.Lease.ClassifyFunc = func(delegation.Invocation) (lease.ClassifyResult, error) {
				return lease.ClassifyResult{Class: lease.ClassSteward}, nil
			}
			ports.Goal.BreachStopFunc = func(string, uint64, time.Time, string) (goal.StopBatch, error) {
				return goal.ReadStopBatch(bed.root(), stopID)
			}
			ports.Records.CASFunc = func(job, expect, target, patch string) (string, error) {
				return dispatchcore.RecordCAS(bed.root(), job, expect, target, patch)
			}
			owners := ports.Ports()
			owners.Host = engineHost{launches: bed.manager}
			life, err := delegation.New(delegation.Config{Root: bed.root(), RepoScope: bed.root()}, owners)
			if err != nil {
				t.Fatal(err)
			}
			invoke := func(args ...string) delegation.Result {
				var stderr bytes.Buffer
				result := life.Run(context.Background(), delegation.Request{Env: delegation.Env{DelegateInternal: true}, Stderr: &stderr}, args)
				if result.ExitCode != 0 {
					t.Fatalf("%v failed: exit=%d stderr=%s", args, result.ExitCode, &stderr)
				}
				return result
			}
			if stop == "breach" {
				result := invoke("__breach-stop-goal", "--goal", bed.id, "--revision", fmt.Sprint(file.Claimed.Revision))
				if !strings.Contains(string(result.Stdout), "state=COMPLETE") {
					t.Fatalf("breach stop did not report completion: %+v", result)
				}
			} else {
				person := bed.workOwners()
				person.processes.launches = func() *launch.Manager { return bed.manager }
				person.commandNow = func(string) (time.Time, error) { return bed.manager.Now(), nil }
				person.processes.cancelDispatch = func(_, job string) (map[string]any, int, error) {
					invoke("__cancel-owned", "--job", job)
					return map[string]any{"outcome": "CANCELLED", "jobId": job}, 0, nil
				}
				code, result := bed.runJSON(person, "work", "stop", bed.id)
				if code != 0 || resultData(t, result)["stopState"] != string(goal.StopBatchComplete) {
					t.Fatalf("person's stop did not complete: %d %+v", code, result)
				}
			}
			reservation, err = dispatchcore.ReadRecordObject(path)
			if err != nil || reservation["status"] != "cancelled" {
				t.Fatalf("reservation not cancelled: %+v %v", reservation, err)
			}
			batch, err = goal.ReadStopBatch(bed.root(), stopID)
			if err != nil || batch.State != goal.StopBatchComplete || batch.Failure != "" || batch.CompletedAt == "" || len(batch.Pending) != 0 ||
				len(batch.Terminal) != 1 || batch.Terminal[0] != id || len(batch.Observed) != 1 ||
				batch.Observed[0].Status != "cancelled" || batch.Observed[0].Disposition != "LOCAL_TERMINAL" {
				t.Fatalf("batch did not settle the reservation: %+v %v", batch, err)
			}
			if _, err := bed.manager.Store.Read(id); !os.IsNotExist(err) {
				t.Fatalf("stop invented a launch record: %v", err)
			}
		})
	}
}

func TestIntentUnitLaunchCustodyRefusalNamesRecovery(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.StopCapability = nil
	})
	brief := bed.brief("custody.md", "Build the unit.\n")
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "custody", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 1 || !strings.Contains(resultWords(result), "no proven claim custody") ||
		!strings.Contains(resultWords(result), "resume or re-claim it before dispatch") || len(bed.starter.launched()) != 0 {
		t.Fatalf("custody refusal has no dispatch recovery or starts work: %d %+v", code, result)
	}
}
