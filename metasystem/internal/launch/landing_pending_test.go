package launch

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestLandingPendingRecoveryChecksCurrentLane(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"same-reservation", "changed-lane", "second-reservation", "cancel", "fenced", "cancel-after-claim", "fenced-after-claim", "claimed", "ended"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			checkout, module := nestedLane(t)
			m, processes := laneManager(t, checkout, module)
			spec := StartSpec{ID: "landing-pending", Kind: LandingKind, WorkingDirectory: checkout,
				FenceRoot: module, Brief: landingBrief(t), Tag: "pending", AdapterData: landingGate(t)}
			m.Supervisor = interruptedLandingStarter{}
			interruptLandingStart(t, func() { _, _ = m.Start(spec) })
			m.Supervisor = supervisingStarter(t, m)
			retained, err := m.Store.Read(spec.ID)
			if err != nil {
				t.Fatal(err)
			}
			children := 0
			processes.onStart = func(Command) { children++ }
			m.SettingsError = errors.New("current installation settings are unavailable")
			m.Settings.LandingModel = "replacement-model"
			switch change {
			case "cancel":
				_, err = m.Store.Update(spec.ID, func(r *Record) error { r.Reason = "cancel-requested"; return nil })
			case "claimed":
				_, err = m.Store.Update(spec.ID, func(r *Record) error { r.Supervisor = &identity.Ref{Pid: 10, StartedAtSec: 10}; return nil })
			case "ended":
				_, err = m.Store.Update(spec.ID, func(r *Record) error { r.State = Cancelled; return nil })
			case "fenced":
				writeFence(t, module, stopfence.StateClosed, stopfence.PhaseStopped, 1)
			case "cancel-after-claim":
				m.supervisorClaimed = func(r Record) {
					_, err = m.Store.Update(r.ID, func(r *Record) error { r.Reason = "cancel-requested"; return nil })
				}
			case "fenced-after-claim":
				m.supervisorClaimed = func(Record) { writeFence(t, module, stopfence.StateClosed, stopfence.PhaseStopped, 1) }
			}
			if err != nil {
				t.Fatal(err)
			}
			if change == "changed-lane" {
				m.Lane = func() (LaneCheckout, error) { return LaneCheckout{}, nil }
			}
			if change == "second-reservation" {
				other, err := m.Store.Read(spec.ID)
				if err != nil {
					t.Fatal(err)
				}
				other.ID = "other-landing"
				if err := m.Store.Create(other); err != nil {
					t.Fatal(err)
				}
			}
			r, err := m.ResumePending(spec.ID)
			if change != "same-reservation" {
				want := map[string]string{"changed-lane": "none is registered", "second-reservation": "has not ended", "cancel": "cancellation", "fenced": "stopped", "cancel-after-claim": "cancel", "fenced-after-claim": "stopped", "claimed": "not an unclaimed", "ended": "not an unclaimed"}[change]
				if err == nil || !strings.Contains(err.Error(), want) || processes.command.Program != "" {
					t.Fatalf("recovery bypassed current lane: %+v %v command=%+v", r, err, processes.command)
				}
				return
			}
			if err != nil || r.ID != spec.ID || r.Child == nil || children != 1 {
				t.Fatalf("same reservation could not recover: %+v children=%d %v", r, children, err)
			}
			if !reflect.DeepEqual(r.Inputs, retained.Inputs) || r.Adapter != retained.Adapter ||
				readString(r.AdapterData, "model") != readString(retained.AdapterData, "model") ||
				readString(r.AdapterData, "effort") != readString(retained.AdapterData, "effort") ||
				!slices.Contains(processes.command.Args, readString(retained.AdapterData, "model")) ||
				readString(r.AdapterData, "settings") != readString(retained.AdapterData, "settings") ||
				!slices.Contains(processes.command.Args, readString(retained.AdapterData, "effort")) {
				t.Fatalf("recovery replaced retained inputs or settings: %+v command=%+v", r, processes.command)
			}
			records, err := m.Store.List()
			if err != nil || len(records) != 1 {
				t.Fatalf("recovery created a second reservation: %+v %v", records, err)
			}
			if _, err := m.ResumePending(spec.ID); err == nil || children != 1 {
				t.Fatalf("same reservation could not recover: %+v %v", r, err)
			}
		})
	}
}

func TestLandingPendingFencePreventsSupervisorStart(t *testing.T) {
	t.Parallel()
	checkout, module := nestedLane(t)
	m, _ := laneManager(t, checkout, module)
	spec := StartSpec{ID: "landing-pending-fenced", Kind: LandingKind, WorkingDirectory: checkout,
		FenceRoot: module, Brief: landingBrief(t), Tag: "pending", AdapterData: landingGate(t)}
	m.Supervisor = interruptedLandingStarter{}
	interruptLandingStart(t, func() { _, _ = m.Start(spec) })
	writeFence(t, module, stopfence.StateClosed, stopfence.PhaseStopped, 1)
	starts := 0
	m.Supervisor = fakeStarter{start: func(string) { starts++ }}
	_, err := m.ResumePending(spec.ID)
	if err == nil || !strings.Contains(err.Error(), "stopped") || starts != 0 {
		t.Fatalf("closed fence reached the supervisor: starts=%d %v", starts, err)
	}
	r, err := m.Store.Read(spec.ID)
	if err != nil || r.State != Starting || r.Supervisor != nil || r.Child != nil {
		t.Fatalf("closed fence changed the reservation: %+v %v", r, err)
	}
}

type interruptedLandingStarter struct{}

func (interruptedLandingStarter) StartSupervisor(string, string) (identity.Ref, error) {
	panic("fixture interruption after reservation")
}

func interruptLandingStart(t *testing.T, start func()) {
	t.Helper()
	defer func() {
		if got := recover(); got != "fixture interruption after reservation" {
			t.Fatalf("interruption = %v", got)
		}
	}()
	start()
	t.Fatal("fixture did not interrupt")
}
