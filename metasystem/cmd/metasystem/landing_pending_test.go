package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type interruptedLandingSupervisor struct{}

func (interruptedLandingSupervisor) StartSupervisor(string, string) (identity.Ref, error) {
	panic("fixture interruption after reservation")
}

func TestLandingUnsetCancelsUnclaimedReservation(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	a := landingAgent{manager: func() *launch.Manager { return b.manager }}
	report, err := lane.Unset(b.home, "fixture-person", b.manager.Now(), false,
		lane.UnsetSeams{Settle: settleLandingAgent(a.running, a.cancel)})
	if err != nil || report.Unregistered || report.Settlement.Settled(false) {
		t.Fatalf("unset reported an unclaimed reservation settled: %+v %v", report, err)
	}
	r, err := b.manager.Store.Read(firstReservationID)
	if err != nil || r.Reason != "cancel-requested" || r.State.Terminal() {
		t.Fatalf("unset did not request cancellation: %+v %v", r, err)
	}
	if _, present, err := lane.Read(b.home); err != nil || !present {
		t.Fatalf("unset removed the lane before settlement: present=%t %v", present, err)
	}
	b.manager.Supervisor = recordingSupervisor{b.manager.Store}
	if _, err := b.manager.ResumePending(firstReservationID); err == nil {
		t.Fatal("cancelled reservation resumed")
	}
}

func TestLandingPendingReservationPreventsBatchIdle(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	a := landingAgent{manager: func() *launch.Manager { return b.manager }}
	registration, present, err := lane.Read(b.home)
	if err != nil || !present {
		t.Fatalf("fixture lane: %+v %v", registration, err)
	}
	if _, _, err := plain.HandIn(registration.Install, plain.Line{Goal: "fixture-goal", SHA: "fixture-sha"}); err != nil {
		t.Fatal(err)
	}
	seams := plain.ProveSeams{
		AgentRunning: func() (bool, error) { _, running, err := a.running(); return running, err },
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch":
				return "", nil
			case "rev-parse":
				return "fixture-main", nil
			default:
				t.Fatalf("unexpected Git call: %v", args)
				return "", nil
			}
		},
	}
	selected, err := plain.PersonSelection(registration.Install, b.root, registration,
		[]string{"fixture-goal"}, plain.ActProvenance{CheckedAt: b.manager.Now()}, seams)
	if err == nil || !strings.Contains(err.Error(), "another operation owns the selection") || selected != nil {
		t.Fatalf("pending reservation was treated as idle: %+v %v", selected, err)
	}
	if batch, err := plain.ReadBatch(registration.Install); err != nil || batch != nil {
		t.Fatalf("selection changed during a pending launch: %+v %v", batch, err)
	}
}

func TestLandingKeeperResumesUnclaimedReservation(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	records, err := b.manager.Store.List()
	if err != nil || len(records) != 1 || records[0].State != launch.Starting {
		t.Fatalf("pending reservation: %+v, %v", records, err)
	}
	var brief string
	if err := json.Unmarshal(records[0].AdapterData["brief"], &brief); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(brief); err != nil {
		t.Fatalf("the pending reservation lost its brief: %v", err)
	}
	b.manager.Supervisor = recordingSupervisor{b.manager.Store}
	second := b.keeper.Run()
	records, err = b.manager.Store.List()
	if err != nil || len(records) != 1 || records[0].Child == nil || second.Outcome != lane.AgentStarted || second.Launch != firstReservationID {
		t.Fatalf("recovery did not resume the same operation: %+v, %+v, %v", second, records, err)
	}
	if third := b.keeper.Run(); third.Outcome != lane.AgentRunning {
		t.Fatalf("repeated wake: %+v", third)
	}
}

func TestLandingKeeperHoldsChangedPendingIdentity(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"batch", "registration", "cancel", "unknown-process", "missing-binding", "unreadable-registration", "unreadable-brief", "changed-fence", "changed-directory"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			b := newPendingLandingBed(t)
			b.interrupt(t)
			switch change {
			case "batch":
				*b.scope = "replacement-batch"
			case "registration":
				r, present, err := lane.Read(b.home)
				if err != nil || !present {
					t.Fatalf("fixture lane: %+v %v", r, err)
				}
				r.CustodyEpoch++
				writeQuestionFixture(t, lane.RecordPath(b.home), r)
			case "unreadable-registration":
				start := b.keeper.Start
				b.keeper.Start = func(root string, wake lane.Wake) (string, error) {
					if err := os.WriteFile(lane.RecordPath(b.home), []byte("{torn"), 0600); err != nil {
						t.Fatal(err)
					}
					return start(root, wake)
				}
			case "cancel", "unknown-process", "missing-binding", "unreadable-brief", "changed-fence", "changed-directory":
				_, err := b.manager.Store.Update(firstReservationID, func(r *launch.Record) error {
					switch change {
					case "cancel":
						r.Reason = "cancel-requested"
					case "unknown-process":
						r.Supervisor = &identity.Ref{Pid: 101, StartedAtSec: 102}
					case "missing-binding":
						delete(r.AdapterData, "landingPendingBinding")
					case "unreadable-brief":
						delete(r.AdapterData, "brief")
					case "changed-fence":
						r.AdapterData["fenceRoot"] = json.RawMessage(`"/other-install"`)
					case "changed-directory":
						r.WorkingDirectory = "/other-checkout"
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			b.manager.Supervisor = recordingSupervisor{b.manager.Store}
			out := b.keeper.Run()
			records, err := b.manager.Store.List()
			if err != nil || len(records) != 1 || records[0].Child != nil || out.Outcome == lane.AgentStarted {
				t.Fatalf("changed pending identity was admitted: %+v %+v %v", out, records, err)
			}
			reason := map[string]string{
				"batch":                   "its lane registration or selected batch changed",
				"registration":            "its lane registration or selected batch changed",
				"cancel":                  "cancellation is requested",
				"missing-binding":         "its retained lane registration cannot be read",
				"unreadable-registration": "the current lane registration cannot be read",
				"unreadable-brief":        "its retained brief cannot be read",
				"changed-fence":           "its retained process fence does not name the registered installation",
				"changed-directory":       "its lane registration or selected batch changed",
			}[change]
			if change != "unknown-process" && !strings.Contains(out.Line, reason) {
				t.Fatalf("held line lost its reason %q: %+v", reason, out)
			}
			if change != "unknown-process" && (out.Outcome != lane.AgentHeld || !strings.HasPrefix(out.Line, "the landing agent's start is held: ") || !strings.Contains(out.Line, "(reservation "+firstReservationID+")") || !strings.HasSuffix(out.Line, "run metasystem work stop "+firstReservationID+" to request cancellation") || strings.Contains(out.Line, "retried") || strings.Contains(out.Line, "20 min") || strings.Contains(out.Line, "AgentHeld") || strings.Contains(out.Line, "AgentFailed")) {
				t.Fatalf("held reservation is not visible: %+v", out)
			}
		})
	}
}

func TestLandingKeeperProductionWiringResumesUnclaimedReservation(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	state, err := lane.ReadAgentState(b.home)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := time.Parse(time.RFC3339, state.StartingAt)
	if err != nil || b.clock.Sub(claimed) <= 10*time.Minute {
		t.Fatalf("claim is not expired: %+v %v", state, err)
	}
	b.manager.Supervisor = recordingSupervisor{b.manager.Store}
	a := newTestLandingAgent(func(a *landingAgent) {
		a.manager = func() *launch.Manager { return b.manager }
		a.now = b.manager.Now
	})
	k := newLandingAgentKeeper(b.root, b.home, a)
	// Isolate admission from recovery while retaining all production launch wiring.
	k.Refresh, k.Observe, k.Prepare, k.Clock = nil, nil, nil, nil
	k.Holds, k.ProviderHold, k.Helmed, k.AdmitWake = nil, nil, nil, nil
	k.Fingerprint, k.BarrenStop, k.Waiting = nil, nil, nil
	k.Continuation = b.keeper.Continuation
	k.Sources = b.keeper.Sources
	out := k.Run()
	records, err := b.manager.Store.List()
	if err != nil || out.Outcome != lane.AgentStarted || out.Launch != firstReservationID || len(records) != 1 {
		t.Fatalf("production recovery: %+v records=%+v err=%v", out, records, err)
	}
	if records[0].ID != firstReservationID || records[0].Child == nil || *records[0].Child != (identity.Ref{Pid: 20, StartedAtSec: 20}) {
		t.Fatalf("reservation did not record its child: %+v", records[0])
	}
}

func TestLandingKeeperSupervisorLaunchFailureStaysFailed(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	calls := 0
	b.manager.Supervisor = supervisorStart(func(string, string) (identity.Ref, error) {
		calls++
		return identity.Ref{}, errors.New("supervisor could not launch")
	})
	out := b.keeper.Run()
	if calls != 1 || out.Outcome != lane.AgentFailed || !strings.Contains(out.Line, "supervisor could not launch") {
		t.Fatalf("supervisor failure: calls=%d run=%+v", calls, out)
	}
}

func TestLandingKeeperPauseLeavesPendingReservationRecoverable(t *testing.T) {
	t.Parallel()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	// A paused lane without a selected continuation starts no recovery.
	*b.scope = ""
	if _, err := lane.SetPause(b.home, "fixture-person", b.manager.Now()); err != nil {
		t.Fatal(err)
	}
	b.manager.Supervisor = recordingSupervisor{b.manager.Store}
	if out := b.keeper.Run(); out.Outcome != lane.AgentPaused {
		t.Fatalf("paused lane recovered work: %+v", out)
	}
	r, err := b.manager.Store.Read(firstReservationID)
	if err != nil || r.State != launch.Starting || r.Child != nil {
		t.Fatalf("pause destroyed pending evidence: %+v %v", r, err)
	}
}

const firstReservationID = "landing-pending-fixture"

type pendingLandingBed struct {
	keeper     lane.AgentKeeper
	manager    *launch.Manager
	home, root string
	scope      *string
	clock      *time.Time
}

func newPendingLandingBed(t *testing.T) pendingLandingBed {
	t.Helper()
	base := t.TempDir()
	home, root := filepath.Join(base, "home"), filepath.Join(base, "lane")
	module := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(module, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	root, module = resolvedPath(root), resolvedPath(module)
	writeQuestionFixture(t, lane.RecordPath(home), lane.Record{Root: root, Install: module,
		CustodyEpoch: 1, RegisteredBy: "fixture-person", At: now.Format(time.RFC3339)})
	settings := launch.DefaultSettings()
	settings.LandingRuntime, settings.LandingModel = "claude", "fixture-model"
	m := &launch.Manager{Store: launch.Store{Root: filepath.Join(base, "launches")}, CapacityHome: home,
		Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/claude"}},
		Settings: settings, Supervisor: interruptedLandingSupervisor{}, Now: func() time.Time { return now },
		Sleep: func(time.Duration) {}, StartCap: time.Minute, Poll: time.Second,
		Lane: landingLaneCheckout(func() (string, error) { return home, nil })}
	a := newTestLandingAgent(func(a *landingAgent) {
		a.home, a.manager = home, func() *launch.Manager { return m }
		a.settings = func(string) (launch.Settings, error) { return settings, nil }
		a.nonce = func() (string, error) { return "pending-fixture", nil }
	})
	scope := "batch-fixture"
	k := lane.AgentKeeper{Home: home, Self: module, Now: m.Now, Running: a.running, Unclaimed: a.unclaimed, Start: a.start,
		Continuation: func(lane.Record) string { return scope },
		Sources:      lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{"queued"}, nil }}}
	return pendingLandingBed{clock: &now, keeper: k, manager: m, home: home, root: root, scope: &scope}
}

func (b pendingLandingBed) interrupt(t *testing.T) {
	t.Helper()
	func() {
		defer func() {
			if got := recover(); got != "fixture interruption after reservation" {
				t.Fatalf("interruption = %v", got)
			}
		}()
		b.keeper.Run()
		t.Fatal("fixture did not interrupt")
	}()
	*b.clock = b.clock.Add(11 * time.Minute)
}
