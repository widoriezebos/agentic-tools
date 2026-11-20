package launch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPublicationEndBookingCollectsStampedGateLaunches(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"legacy", "stamped", "missing"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f, run, _, need, _ := publicationFixture(t, "base")
			f.runner.Installation = t.TempDir()
			subject := provePublication(t, f.runner, run, f.worktree, need.Base)
			if len(subject.GateLaunches) != 1 {
				t.Fatal("publication did not launch a check")
			}
			id := subject.GateLaunches[0]
			execution, err := f.manager.Store.Read(id)
			if err != nil || readString(execution.AdapterData, "unitRun") != run || readString(execution.AdapterData, "unitInstallation") != f.runner.Installation {
				t.Fatalf("publication stamp: %+v %v", execution, err)
			}
			if mode == "legacy" {
				if _, err := f.manager.Store.Update(id, func(r *Record) error {
					delete(r.AdapterData, "unitRun")
					delete(r.AdapterData, "unitInstallation")
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "missing" {
				dir, err := f.manager.Store.StateDir(id)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(dir, "record.json")); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			f.runner.CollectLaunch = func(_ UnitRunRecord, r Record, _ string) error {
				if r.ID == id {
					calls++
				}
				return nil
			}
			before, err := os.ReadFile(filepath.Join(f.runner.runDir(run), "run.json"))
			if err != nil {
				t.Fatal(err)
			}
			launches := len(f.starter.ids)
			if err := f.runner.BookEndedRead(run); err != nil {
				t.Fatal(err)
			}
			if (calls == 1) != (mode == "stamped") {
				t.Fatalf("gate collections=%d mode=%s", calls, mode)
			}
			after, err := os.ReadFile(filepath.Join(f.runner.runDir(run), "run.json"))
			if err != nil || !bytes.Equal(before, after) || len(f.starter.ids) != launches {
				t.Fatal("booking changed the subject or started work", err)
			}
		})
	}
}

func TestBookedStepKeepsTheWorktreeUntilStopped(t *testing.T) {
	t.Parallel()
	f := newUnitFixture(t, "")
	result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil {
		t.Fatal(err)
	}
	record := result.Record
	record.State = "running"
	record.Rounds[0].Steps = record.Rounds[0].Steps[:1]
	step := &record.Rounds[0].Steps[0]
	step.State, step.FinishedAt = StepRunning, ""
	if _, err := f.manager.Store.Update(step.LaunchID, func(r *Record) error { r.FinishedAt = r.StartedAt; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := f.runner.save(record); err != nil {
		t.Fatal(err)
	}
	probe := f.manager.Prober.(*fakeProber)
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	f.manager.Sleep(6 * time.Minute)
	if _, err := f.runner.BookEndedSteps(record.ID, f.manager.Now().Add(-5*time.Minute), func(UnitRunRecord, Record) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if released, _ := f.runner.treeQuiescent(treeReservation{Run: record.ID}); released {
		t.Fatal("booking released the worktree")
	}
	if err := f.runner.GateTree(record.Worktree, "another-run", nil); err == nil {
		t.Fatal("another builder stole the worktree")
	}
	if _, err := f.runner.CancelRun(record.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.runner.GateTree(record.Worktree, "another-run", nil); err != nil {
		t.Fatal("person's stop did not release the worktree", err)
	}
}

func TestBookingAndResumingAnOverlappingRound(t *testing.T) {
	for _, both := range []bool{false, true} {
		t.Run(map[bool]string{false: "proof-ended", true: "both-ended"}[both], func(t *testing.T) {
			f := newUnitFixture(t, "", "branch", "round", "branch")
			f.starter.onStart = func(r Record) error {
				if r.Kind == "proof" || r.Kind == "read" {
					f.starter.holdKind = r.Kind
				}
				return nil
			}
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped {
				t.Fatalf("first wait: %v", err)
			}
			round := first.Record.Rounds[0]
			ids := len(f.starter.ids)
			for _, step := range round.Steps[1:] {
				if !both && step.Kind == "read" {
					continue
				}
				_, err := f.manager.Store.Update(step.LaunchID, func(r *Record) error {
					r.State = Failed
					code := 1
					r.ExitCode = &code
					r.Supervisor = nil
					r.Child = nil
					r.FinishedAt = f.manager.Now().Format(time.RFC3339Nano)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			f.manager.Sleep(time.Minute)
			booked, err := f.runner.BookEndedSteps(first.Record.ID, f.manager.Now(), func(UnitRunRecord, Record) bool { return true })
			want := 1
			state := "running"
			if both {
				want = 2
				state = UnitStepEnded
			}
			if err != nil || len(booked) != want {
				t.Fatalf("bookings %v: %v", booked, err)
			}
			current, err := f.runner.Status(first.Record.ID)
			if err != nil || current.State != state {
				t.Fatalf("during booking: %s, %v", current.State, err)
			}
			f.manager.Prober.(*fakeProber).states[10] = identity.Dead
			f.manager.Prober.(*fakeProber).states[20] = identity.Dead
			last, err := f.runner.Advance(UnitRequest{Resume: first.Record.ID})
			if err != nil {
				t.Fatal(err)
			}
			done := last.Record.Rounds[0]
			if done.Outcome != "proof-red" || len(f.starter.ids) != ids || done.Steps[1].EndBooking.CollectedAt == "" || len(done.Reads) > 0 || !done.CheckoutRemoved {
				t.Fatalf("join after booking: %+v", done)
			}
		})
	}
}
