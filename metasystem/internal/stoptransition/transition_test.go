package stoptransition

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

type fakeProber map[int64]identity.Liveness

func (p fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(20, 0)}, p[pid], nil
}

type scriptedFamily struct {
	name        string
	inventories [][]Item
	stops       int
	inventory   int
	complete    bool
	completions []bool
	auxiliary   []AuxiliaryOutcome
	stopFunc    func(Item) (Outcome, error)
}

type unreadableInventoryFamily struct {
	name     string
	path     string
	readable bool
}

func (f *unreadableInventoryFamily) Name() string { return f.name }
func (f *unreadableInventoryFamily) Inventory() ([]Item, error) {
	if f.readable {
		return nil, nil
	}
	err := errors.New("fixture inventory unreadable")
	if f.path != "" {
		return nil, &InventoryReadError{Path: f.path, Err: err}
	}
	return nil, err
}
func (f *unreadableInventoryFamily) Stop(Item) (Outcome, error) {
	return Outcome{}, errors.New("unreachable")
}

func (f *scriptedFamily) Name() string { return f.name }
func (f *scriptedFamily) Inventory() ([]Item, error) {
	index := f.inventory
	f.inventory++
	if index >= len(f.inventories) {
		return nil, nil
	}
	return f.inventories[index], nil
}
func (f *scriptedFamily) Stop(item Item) (Outcome, error) {
	f.stops++
	if f.stopFunc != nil {
		return f.stopFunc(item)
	}
	complete := f.complete
	if f.stops <= len(f.completions) {
		complete = f.completions[f.stops-1]
	}
	line := item.StatusLine + ": stopped (TERM)"
	if !complete {
		line = "NOT STOPPED " + item.StatusLine + ": fixture survivor; did: left it listed in the fence record"
	}
	return Outcome{Line: line, Complete: complete, Survivor: item.Survivor, Auxiliary: f.auxiliary}, nil
}

func testTransition(t *testing.T, family Family) *Transition {
	now := time.Unix(100, 0)
	return &Transition{
		Root: t.TempDir(), Checkout: "/checkout", Families: []Family{family},
		Prober: fakeProber{10: identity.Alive}, Now: func() time.Time { return now },
		Self: func() (identity.Ref, error) { return identity.Ref{Pid: 10, StartedAtSec: 20}, nil },
	}
}

func TestCreatorRaceStopsLateArrival(t *testing.T) {
	generation := int64(0)
	item := Item{Key: "run:late", StatusLine: "run late running", FenceGeneration: &generation, Survivor: stopfence.Survivor{Component: "run", ID: "late"}}
	family := &scriptedFamily{name: "run", inventories: [][]Item{nil, {item}, nil}, complete: true}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	if err != nil || report.ExitCode != 0 || family.stops != 1 || !strings.Contains(strings.Join(report.Lines, "\n"), "arrived during stop") {
		t.Fatalf("late-arrival report=%#v stops=%d err=%v", report, family.stops, err)
	}
}

func TestSurvivorDoesNotStopLaterFamilies(t *testing.T) {
	item := Item{Key: "run:stuck", StatusLine: "run stuck running", Survivor: stopfence.Survivor{Component: "run", ID: "stuck"}}
	family := &scriptedFamily{name: "run", inventories: [][]Item{{item}, nil}, complete: false}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	wantClosing := "stop incomplete for /checkout; 1 not stopped, listed above; run: metasystem stop --repo /checkout"
	if err != nil || report.ExitCode != 1 || report.Lines[len(report.Lines)-1] != wantClosing {
		t.Fatalf("survivor report=%#v err=%v", report, err)
	}
	record, err := stopfence.Read(transition.Root)
	if err != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 {
		t.Fatalf("fence=%#v err=%v", record, err)
	}
}

func TestAuxiliaryBookkeepingFailureHasItsOwnLineSurvivorCountAndExit(t *testing.T) {
	item := Item{
		Key: "supervision:owner", StatusLine: "supervision-owner pid 41",
		Survivor: stopfence.Survivor{Component: "supervision-owner", Pid: 41, PidStartedAt: 100},
	}
	bookkeeping := stopfence.Survivor{
		Component: "supervision-lock", Path: "/checkout/artifacts/agents/supervision/lock.d", Reason: "directory not empty",
	}
	family := &scriptedFamily{
		name: "supervision", inventories: [][]Item{{item}, nil}, complete: true,
		auxiliary: []AuxiliaryOutcome{{
			Key:      "supervision-lock:" + bookkeeping.Path,
			Line:     "NOT STOPPED supervision-lock " + bookkeeping.Path + ": directory not empty; did: left the lock",
			Survivor: bookkeeping,
		}},
	}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	if err != nil || report.ExitCode != 1 {
		t.Fatalf("bookkeeping failure report=%#v err=%v", report, err)
	}
	want := "supervision-owner pid 41: stopped (TERM)\nNOT STOPPED supervision-lock " + bookkeeping.Path + ": directory not empty; did: left the lock"
	if !strings.Contains(strings.Join(report.Lines, "\n"), want) || report.Lines[len(report.Lines)-1] != "stop incomplete for /checkout; 1 not stopped, listed above; run: metasystem stop --repo /checkout" {
		t.Fatalf("bookkeeping failure lines=%#v", report.Lines)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || !sameSurvivorIdentity(record.NotStopped[0], bookkeeping) {
		t.Fatalf("bookkeeping survivor=%+v err=%v", record, readErr)
	}
}

func TestSuiteHostedRunConcludesFromSidecarBeforeAnyRunSignal(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1786900000, 0)
	pid := int64(61)
	states := fakeProber{pid: identity.Alive}
	store := &runpkg.Store{
		Root: root, Prober: states, Now: func() time.Time { return now },
		Getpgid:      func(int64) (int64, error) { return pid, nil },
		AllPids:      func() ([]int64, error) { return nil, nil },
		GroupPresent: func(int64) (bool, bool) { return states[pid] == identity.Alive, true },
	}
	nonce, err := store.Launch(runpkg.Caller{Class: "HUMAN"}, runpkg.LaunchParams{Id: "suite-host", Kind: "suite", Log: "suite.log"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("suite-host", nonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	record, err := store.Read("suite-host")
	if err != nil || record == nil {
		t.Fatalf("read suite host: record=%+v err=%v", record, err)
	}
	// The proof-run family has already stopped the wrapper group before the
	// run family begins. An ordinary assessment here would prematurely freeze
	// ended-unknown while the wrapper's atomic sidecar is still arriving.
	states[pid] = identity.Dead
	stoppedSuites := &suiteStopGroups{groups: map[int64]bool{pid: true}}
	family := newRunFamily(root, 1, stoppedSuites)
	family.store = store
	family.items["run:suite-host"] = *record
	family.now = func() time.Time { return now }
	var sidecarErr error
	family.sleep = func(duration time.Duration) {
		now = now.Add(duration)
		sidecarErr = store.WriteSidecar(record.RunId, record.Generation, record.LaunchNonce, 7)
	}
	outcome, err := family.Stop(Item{Key: "run:suite-host", Survivor: stopfence.Survivor{Component: "run", ID: "suite-host", Pid: pid, PidStartedAt: 20}})
	want := "run suite-host running pid 61 pgid 61 wrapped: concluded red (suite stopped)"
	if err != nil || sidecarErr != nil || !outcome.Complete || outcome.Line != want {
		t.Fatalf("suite-hosted run outcome=%+v err=%v sidecarErr=%v", outcome, err, sidecarErr)
	}
	concluded, err := store.Read("suite-host")
	if err != nil || concluded == nil || concluded.Status != runpkg.StatusRed || concluded.ExitCode == nil || *concluded.ExitCode != 7 {
		t.Fatalf("suite-hosted record=%+v err=%v", concluded, err)
	}
}

func TestStopTransitionRunFamilyCarriesGovernedSpendProjection(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	pid := int64(64)
	states := fakeProber{pid: identity.Alive}
	family := newRunFamily(root, 1, &suiteStopGroups{groups: map[int64]bool{pid: true}})
	if family.store.ProjectSpend == nil {
		t.Fatal("production stop-transition run family has no governed spend projection")
	}
	family.store.Now = func() time.Time { return now }
	family.store.Prober = states
	family.store.Getpgid = func(int64) (int64, error) { return pid, nil }
	family.store.AllPids = func() ([]int64, error) { return nil, nil }
	weightGeneration := uint64(0)
	family.store.AdmitGoverned = func(runpkg.GovernedAdmissionRequest) (runpkg.GovernedAdmissionResult, error) {
		return runpkg.GovernedAdmissionResult{Attempt: runpkg.GovernedAttempt{
			GoalRevision: 3, ObligationRevision: 6, Recurrence: governance.StandingSharedProcess,
			WeightGeneration: &weightGeneration, ExecutionCostMinutes: 30, AttemptOrdinal: 1,
			Budget:          goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1},
			BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
				Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
				SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
			}, AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: runpkg.BreakerClosed,
		}}, nil
	}
	nonce, err := family.store.Launch(runpkg.Caller{Class: "HUMAN"}, runpkg.LaunchParams{Id: "governed-stop", Kind: "suite",
		Display: "governed stop", Log: "artifacts/governed-stop.log", GoalId: "bounded", ObligationRevision: 6, StandingShared: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := family.store.Bind("governed-stop", nonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	record, err := family.store.Read("governed-stop")
	if err != nil || record == nil {
		t.Fatalf("read governed stop run: %+v %v", record, err)
	}
	if err := family.store.WriteSidecar(record.RunId, record.Generation, record.LaunchNonce, 1); err != nil {
		t.Fatal(err)
	}
	states[pid] = identity.Dead
	family.items["run:governed-stop"] = *record
	outcome, err := family.Stop(Item{Key: "run:governed-stop", Survivor: stopfence.Survivor{Component: "run", ID: "governed-stop", Pid: pid}})
	if err != nil || !outcome.Complete {
		t.Fatalf("production run-family stop did not complete: %+v %v", outcome, err)
	}
	concluded, err := family.store.Read("governed-stop")
	state, found, stateErr := obligationstate.Load(root, "bounded", 3, 6)
	if err != nil || stateErr != nil || concluded == nil || concluded.Status != runpkg.StatusRed || !found || len(state.Attempts) != 1 ||
		!strings.Contains(concluded.Governed.ExhaustionReason, "BUDGET_UNKNOWN at conclusion: record=governed-stop reason=") {
		t.Fatalf("stop did not durably carry the projection reason: run=%+v state=%+v found=%t err=%v stateErr=%v", concluded, state, found, err, stateErr)
	}
}

func TestTerminalRunWithoutSuiteJoinHasOneVerdictAndNoSuitePhrase(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1786900000, 0)
	pid := int64(62)
	states := fakeProber{pid: identity.Alive}
	store := &runpkg.Store{
		Root: root, Prober: states, Now: func() time.Time { return now },
		Getpgid:      func(int64) (int64, error) { return pid, nil },
		AllPids:      func() ([]int64, error) { return nil, nil },
		GroupPresent: func(int64) (bool, bool) { return states[pid] == identity.Alive, true },
	}
	nonce, err := store.Launch(runpkg.Caller{Class: "HUMAN"}, runpkg.LaunchParams{Id: "ordinary-host", Kind: "custom", Log: "ordinary.log"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("ordinary-host", nonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	inventoried, err := store.Read("ordinary-host")
	if err != nil || inventoried == nil {
		t.Fatalf("read inventoried run: record=%+v err=%v", inventoried, err)
	}
	if err := store.WriteSidecar(inventoried.RunId, inventoried.Generation, inventoried.LaunchNonce, 8); err != nil {
		t.Fatal(err)
	}
	states[pid] = identity.Dead
	if _, err := store.Assess(inventoried.RunId); err != nil {
		t.Fatal(err)
	}

	family := newRunFamily(root, 1, &suiteStopGroups{groups: map[int64]bool{}})
	family.store = store
	family.items["run:ordinary-host"] = *inventoried
	outcome, err := family.Stop(Item{Key: "run:ordinary-host", Survivor: stopfence.Survivor{Component: "run", ID: "ordinary-host", Pid: pid, PidStartedAt: 20}})
	want := "run ordinary-host red pid 62 pgid 62 wrapped: already gone"
	if err != nil || !outcome.Complete || outcome.Line != want || strings.Contains(outcome.Line, "suite stopped") || strings.Count(outcome.Line, "already gone") != 1 {
		t.Fatalf("ordinary terminal run outcome=%+v err=%v", outcome, err)
	}
}

func TestRunOutcomeLineGrammar(t *testing.T) {
	base := runpkg.StopOutcome{
		RunID: "grammar", InitialStatus: runpkg.StatusRunning, Status: runpkg.StatusRed,
		Custody: runpkg.CustodyWrapped, PID: 63, PGID: 63, Signal: runpkg.StopSignalTerm,
		Result: runpkg.StopResultStopped, Reason: "TERM",
	}
	tests := []struct {
		name         string
		outcome      runpkg.StopOutcome
		suiteStopped bool
		want         string
		complete     bool
		wantError    string
	}{
		{
			name: "suite host", outcome: base, suiteStopped: true,
			want: "run grammar running pid 63 pgid 63 wrapped: concluded red (suite stopped)", complete: true,
		},
		{
			name: "adopted custody",
			outcome: runpkg.StopOutcome{
				RunID: "grammar", InitialStatus: runpkg.StatusRunning, Status: runpkg.StatusRunning,
				Custody: runpkg.CustodyAdoptedVerified, PID: 64, Signal: runpkg.StopSignalNone,
				Result: runpkg.StopResultNotStopped, Reason: "not the metasystem's process, not signalled",
			},
			want: "run grammar running pid 64 adopted-verified: not the metasystem's process, not signalled", complete: true,
		},
		{
			name: "wrapped survivor",
			outcome: runpkg.StopOutcome{
				RunID: "grammar", InitialStatus: runpkg.StatusRunning, Status: runpkg.StatusRunning,
				Custody: runpkg.CustodyWrapped, PID: 65, PGID: 65, Signal: runpkg.StopSignalKill,
				Result: runpkg.StopResultNotStopped, Reason: "group survived KILL",
			},
			want: "NOT STOPPED run grammar running pid 65 pgid 65 wrapped: group survived KILL; did: left the run record", complete: false,
		},
		{
			name: "already terminal", outcome: runpkg.StopOutcome{
				RunID: "grammar", InitialStatus: runpkg.StatusGreen, Status: runpkg.StatusGreen,
				Custody: runpkg.CustodyWrapped, PID: 66, PGID: 66, Signal: runpkg.StopSignalNone,
				Result: runpkg.StopResultAlreadyGone, Reason: "record is terminal",
			},
			want: "run grammar green pid 66 pgid 66 wrapped: already gone", complete: true,
		},
		{
			name: "launch failed by stop", outcome: runpkg.StopOutcome{
				RunID: "grammar", InitialStatus: runpkg.StatusLaunching, Status: runpkg.StatusLaunchFailed,
				Custody: runpkg.CustodyWrapped, Signal: runpkg.StopSignalNone,
				Result: runpkg.StopResultStopped, Reason: "launch failed",
			},
			want: "run grammar launching wrapped: launch failed (stopped)", complete: true,
		},
		{
			name: "concluded after TERM", outcome: base,
			want: "run grammar running pid 63 pgid 63 wrapped: concluded red (TERM)", complete: true,
		},
		{
			name: "concluded after KILL", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.Signal = runpkg.StopSignalKill
				return outcome
			}(),
			want: "run grammar running pid 63 pgid 63 wrapped: concluded red (KILL after TERM was ignored)", complete: true,
		},
		{
			name: "missing initial status", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.InitialStatus = ""
				return outcome
			}(),
			wantError: "carries no initial record status",
		},
		{
			name: "suite host missing terminal verdict", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.Status = runpkg.StatusRunning
				return outcome
			}(), suiteStopped: true,
			wantError: "suite stop completed without a terminal record status",
		},
		{
			name: "signal stop missing terminal verdict", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.Status = runpkg.StatusRunning
				return outcome
			}(),
			wantError: "stop completed without a terminal record status",
		},
		{
			name: "terminal conclusion missing signal", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.Signal = runpkg.StopSignalNone
				return outcome
			}(),
			wantError: "stopped without a known signal outcome",
		},
		{
			name: "unknown result", outcome: func() runpkg.StopOutcome {
				outcome := base
				outcome.Result = "unclassified"
				return outcome
			}(),
			wantError: "stop returned unknown result",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			line, complete, err := runOutcomeLine(test.outcome, test.suiteStopped)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || line != "" || complete {
					t.Fatalf("run line = %q complete=%v err=%v, want error containing %q", line, complete, err, test.wantError)
				}
				return
			}
			if err != nil || line != test.want || complete != test.complete {
				t.Fatalf("run line = %q complete=%v err=%v, want %q complete=%v", line, complete, err, test.want, test.complete)
			}
		})
	}
}

func TestSupervisionFamilyKeepsSuccessfulProcessSeparateFromBookkeepingFailures(t *testing.T) {
	current := supervise.InventoryItem{
		Component: "supervision-owner", Identity: identity.Ref{Pid: 41, StartedAtSec: 100},
		Tag: "owner-tag", Generation: 7,
	}
	key := "supervision:supervision-owner:7:41"
	family := newSupervisionFamily(LocalConfig{Root: "/state", Installation: "/engine", Checkout: "/checkout"}, &processSnapshot{})
	family.items[key] = current
	family.failureAnchor = key
	family.shutdown = func(string, string, string, string, int) (supervise.ShutdownReport, error) {
		return supervise.ShutdownReport{
			Outcomes: []supervise.ComponentOutcome{{
				Component: current.Component, Identity: current.Identity, Tag: current.Tag, Generation: current.Generation,
				Signal: supervise.ShutdownSignalKill, Result: supervise.ShutdownStopped,
			}},
			Failures: []supervise.ShutdownFailure{{
				Component: "supervision-registry", Path: "/registry.jsonl", Reason: "append failed",
				Did: "left the registry without the shutdown-escalated row",
			}},
		}, errors.New("append failed")
	}
	outcome, err := family.Stop(Item{Key: key, Survivor: stopfence.Survivor{Component: current.Component, Pid: 41, PidStartedAt: 100}})
	if err != nil || !outcome.Complete || strings.Contains(outcome.Line, "reaped reason") || outcome.Line != "supervision-owner pid 41 tag owner-tag generation 7: killed (TERM ignored)" {
		t.Fatalf("supervision process outcome=%+v err=%v", outcome, err)
	}
	if len(outcome.Auxiliary) != 1 || outcome.Auxiliary[0].Complete || outcome.Auxiliary[0].Survivor.Component != "supervision-registry" || outcome.Auxiliary[0].Survivor.Path != "/registry.jsonl" {
		t.Fatalf("supervision bookkeeping outcome=%+v", outcome.Auxiliary)
	}
}

func TestMissingAnchoredSupervisionOutcomeStillPublishesBookkeepingFailure(t *testing.T) {
	current := supervise.InventoryItem{
		Component: "supervision-owner", Identity: identity.Ref{Pid: 51, StartedAtSec: 110},
		Tag: "owner-tag", Generation: 8,
	}
	key := "supervision:supervision-owner:8:51"
	item := Item{
		Key: key, StatusLine: "supervision-owner pid 51 tag owner-tag generation 8: running",
		Survivor: stopfence.Survivor{Component: current.Component, Pid: 51, PidStartedAt: 110, Tag: current.Tag},
	}
	supervision := newSupervisionFamily(LocalConfig{Root: "/state", Installation: "/engine", Checkout: "/checkout"}, &processSnapshot{})
	supervision.items[key] = current
	supervision.failureAnchor = key
	supervision.shutdown = func(string, string, string, string, int) (supervise.ShutdownReport, error) {
		failure := supervise.ShutdownFailure{
			Component: "supervision-registry", Path: "/registry.jsonl", Reason: "append failed",
			Did: "left the registry without the shutdown-escalated row",
		}
		return supervise.ShutdownReport{Failures: []supervise.ShutdownFailure{failure}}, errors.New("append failed")
	}
	family := &scriptedFamily{name: "supervision", inventories: [][]Item{{item}, nil}, stopFunc: supervision.Stop}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 1 || !strings.Contains(joined, "NOT STOPPED supervision-registry /registry.jsonl: append failed; did: left the registry without the shutdown-escalated row") {
		t.Fatalf("missing anchored outcome report=%#v err=%v", report, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 2 {
		t.Fatalf("missing anchored outcome fence=%+v err=%v", record, readErr)
	}
	foundRegistry := false
	for _, survivor := range record.NotStopped {
		if survivor.Component == "supervision-registry" && survivor.Path == "/registry.jsonl" && survivor.Reason == "append failed" {
			foundRegistry = true
		}
	}
	if !foundRegistry {
		t.Fatalf("registry bookkeeping failure was not saved: %+v", record.NotStopped)
	}
}

func TestCompleteStopUsesSuccessClosingLine(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{nil}})
	report, err := transition.Stop()
	want := "checkout /checkout\nnothing is running\nstopped /checkout; start again: metasystem arm --repo /checkout"
	if err != nil || report.ExitCode != 0 || strings.Join(report.Lines, "\n") != want {
		t.Fatalf("complete stop = %#v err=%v, want %q", report, err, want)
	}
}

func TestIncompleteStopCountsFinalSurvivorsInClosingLine(t *testing.T) {
	item := func(id string) Item {
		return Item{Key: "run:" + id, StatusLine: "run " + id + " running", Survivor: stopfence.Survivor{Component: "run", ID: id}}
	}
	family := &scriptedFamily{name: "run", inventories: [][]Item{{item("one"), item("two")}, nil}, complete: false}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	want := "stop incomplete for /checkout; 2 not stopped, listed above; run: metasystem stop --repo /checkout"
	if err != nil || report.ExitCode != 1 || report.Lines[len(report.Lines)-1] != want {
		t.Fatalf("incomplete stop = %#v err=%v, want closing %q", report, err, want)
	}
}

func TestStatusNamesWhoClosedTheFenceAndWhen(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{nil}})
	if err := stopfence.Write(transition.Root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 2,
		ChangedAt: "2026-09-07T07:40:00Z", Checkout: transition.Checkout,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 77}},
	}); err != nil {
		t.Fatal(err)
	}
	report, err := transition.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := "checkout /checkout\nnothing is running\nstopped since 2026-09-07T07:40:00Z by stop pid 77; start again: metasystem arm --repo /checkout"
	if got := strings.Join(report.Lines, "\n"); got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestCreatorRaceBoundsContinuingArrivalsAtThreePasses(t *testing.T) {
	generation := int64(0)
	item := func(id string) Item {
		return Item{Key: "run:" + id, StatusLine: "run " + id + " running", FenceGeneration: &generation,
			Survivor: stopfence.Survivor{Component: "run", ID: id}}
	}
	family := &scriptedFamily{name: "run", inventories: [][]Item{
		nil, {item("one")}, {item("two")}, {item("three")},
	}, complete: true}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 0 || family.stops != 3 || !strings.Contains(joined, "run three running: stopped (TERM) (arrived during stop)") || strings.Contains(joined, "NOT STOPPED run three running") {
		t.Fatalf("continuing-arrival report=%#v stops=%d err=%v", report, family.stops, err)
	}
}

func TestFinalInventoryStopsAndRecordsAnOwnedSurvivor(t *testing.T) {
	item := Item{Key: "run:final", StatusLine: "run final running", Survivor: stopfence.Survivor{Component: "run", ID: "final"}}
	family := &scriptedFamily{name: "run", inventories: [][]Item{nil, nil, {item}}, complete: false}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 1 || family.stops != 1 || !strings.Contains(joined, "NOT STOPPED run final running: fixture survivor") {
		t.Fatalf("final-inventory stop = %#v stops=%d err=%v", report, family.stops, err)
	}
	if strings.Contains(joined, "nothing is running") || strings.Contains(joined, "stopped /checkout; start again") {
		t.Fatalf("live final-inventory item coexisted with a success report: %q", joined)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].ID != "final" {
		t.Fatalf("final-inventory fence = %#v err=%v", record, readErr)
	}
}

func TestFinalInventoryUsesObservationOnlyFamilyReport(t *testing.T) {
	item := Item{Key: "untracked:55:20", StatusLine: "untracked pid 55 running", ObserveOnly: true, Survivor: stopfence.Survivor{Component: "untracked", Pid: 55, PidStartedAt: 20}}
	family := &scriptedFamily{name: "untracked", inventories: [][]Item{nil, nil, {item}}, complete: true}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 0 || family.stops != 1 || !strings.Contains(joined, "untracked pid 55 running: stopped (TERM)") {
		t.Fatalf("final observation-only report = %#v stops=%d err=%v", report, family.stops, err)
	}
}

func TestNotStoppedOutcomeContinuesToTheNextFamily(t *testing.T) {
	firstItem := Item{Key: "mission:stuck", StatusLine: "mission stuck running", Survivor: stopfence.Survivor{Component: "mission", ID: "stuck"}}
	secondItem := Item{Key: "job:next", StatusLine: "job next running", Survivor: stopfence.Survivor{Component: "job", ID: "next"}}
	first := &scriptedFamily{name: "mission", inventories: [][]Item{{firstItem}, nil}, complete: false}
	second := &scriptedFamily{name: "job", inventories: [][]Item{{secondItem}, nil}, complete: true}
	transition := testTransition(t, first)
	transition.Families = []Family{first, second}
	report, err := transition.Stop()
	if err != nil || report.ExitCode != 1 || first.stops != 1 || second.stops != 1 {
		t.Fatalf("ordered continuation report=%#v first=%d second=%d err=%v", report, first.stops, second.stops, err)
	}
}

func TestInventoryFailureAfterFenceCloseContinuesToTheNextFamily(t *testing.T) {
	first := &unreadableInventoryFamily{name: "mission"}
	item := Item{Key: "job:next", StatusLine: "job next running", Survivor: stopfence.Survivor{Component: "job", ID: "next"}}
	second := &scriptedFamily{name: "job", inventories: [][]Item{{item}, nil}, complete: true}
	transition := testTransition(t, first)
	transition.Families = []Family{first, second}
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 1 || second.stops != 1 || !strings.Contains(joined, "NOT STOPPED mission inventory: fixture inventory unreadable; did: continued to the next stop step") {
		t.Fatalf("inventory continuation report=%#v later-stops=%d err=%v", report, second.stops, err)
	}
	if strings.Count(joined, "NOT STOPPED mission inventory:") != 1 || !strings.Contains(joined, "stop incomplete for /checkout; 1 not stopped, listed above") {
		t.Fatalf("inventory failure report and footer disagree: %q", joined)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 {
		t.Fatalf("inventory failure fence = %#v err=%v", record, readErr)
	}
	survivor := record.NotStopped[0]
	if survivor.Component != "family-inventory" || survivor.ID != "mission" || survivor.Pid != 0 || survivor.Reason != "fixture inventory unreadable" {
		t.Fatalf("inventory failure survivor = %#v", survivor)
	}
	_, armErr := transition.OpenFence("arm")
	var unreadable *UnreadableFamilySurvivorError
	if !errors.As(armErr, &unreadable) || unreadable.Family != "mission" || !strings.Contains(armErr.Error(), "mission records") {
		t.Fatalf("arm over unreadable family = %T %v", armErr, armErr)
	}
}

func TestUnreadableFamilyNamesFileAndClearsAfterAReadableStop(t *testing.T) {
	recordPath := "/checkout/artifacts/agents/jobs/broken.json"
	family := &unreadableInventoryFamily{name: "job", path: recordPath}
	transition := testTransition(t, family)

	firstStop, err := transition.Stop()
	joined := strings.Join(firstStop.Lines, "\n")
	if err != nil || firstStop.ExitCode != 1 || !strings.Contains(joined, "NOT STOPPED job inventory "+recordPath+": fixture inventory unreadable") {
		t.Fatalf("first unreadable stop = %#v err=%v", firstStop, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || len(record.NotStopped) != 1 || record.NotStopped[0].Path != recordPath || record.NotStopped[0].Reason != "fixture inventory unreadable" {
		t.Fatalf("unreadable survivor = %#v err=%v", record, readErr)
	}
	_, armErr := transition.OpenFence("arm")
	var unreadable *UnreadableFamilySurvivorError
	if !errors.As(armErr, &unreadable) || unreadable.Path != recordPath || !strings.Contains(armErr.Error(), "cannot probe the job survivor") || !strings.Contains(armErr.Error(), recordPath) {
		t.Fatalf("first arm over unreadable survivor = %T %v", armErr, armErr)
	}

	family.readable = true
	secondStop, err := transition.Stop()
	if err != nil || secondStop.ExitCode != 0 {
		t.Fatalf("readable retry stop = %#v err=%v", secondStop, err)
	}
	record, readErr = stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopped || len(record.NotStopped) != 0 {
		t.Fatalf("readable retry fence = %#v err=%v", record, readErr)
	}
	if _, armErr = transition.OpenFence("arm"); armErr != nil {
		t.Fatalf("second arm remained blocked after readable stop: %v", armErr)
	}
}

func TestCrashedStopReinventoryExcludesObservationOnlyProcesses(t *testing.T) {
	wrapped := Item{Key: "run:wrapped", StatusLine: "run wrapped running", Survivor: stopfence.Survivor{Component: "run", ID: "wrapped", Pid: 33, PidStartedAt: 20}}
	adopted := Item{Key: "run:adopted", StatusLine: "run adopted running", Survivor: stopfence.Survivor{Component: "run", ID: "adopted", Pid: 44, PidStartedAt: 20}, ObserveOnly: true}
	untracked := Item{Key: "untracked:55:20", StatusLine: "untracked pid 55 running", Survivor: stopfence.Survivor{Component: "untracked", Pid: 55, PidStartedAt: 20}, ObserveOnly: true}
	transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{{wrapped, adopted}}})
	transition.Families = []Family{
		transition.Families[0],
		&scriptedFamily{name: "untracked", inventories: [][]Item{{untracked}}},
	}
	transition.Prober = fakeProber{10: identity.Alive, 22: identity.Dead, 33: identity.Alive, 44: identity.Alive, 55: identity.Alive}
	if err := stopfence.Write(transition.Root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 7,
		ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 22, PidStartedAt: 20}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := transition.OpenFence("arm")
	var local *LocalSurvivorError
	if !errors.As(err, &local) || local.Survivor.ID != "wrapped" {
		t.Fatalf("crashed-stop arm = %T %v", err, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].ID != "wrapped" {
		t.Fatalf("crashed-stop survivors = %#v err=%v", record, readErr)
	}
}

func TestCrashedStopReinventoryRecordsUnreadableFamily(t *testing.T) {
	transition := testTransition(t, &unreadableInventoryFamily{name: "mission"})
	transition.Families = []Family{
		transition.Families[0],
		&scriptedFamily{name: "run", inventories: [][]Item{nil}},
	}
	transition.Prober = fakeProber{10: identity.Alive, 22: identity.Dead}
	if err := stopfence.Write(transition.Root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 7,
		ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 22, PidStartedAt: 20}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := transition.OpenFence("arm")
	var unreadable *UnreadableFamilySurvivorError
	if !errors.As(err, &unreadable) || unreadable.Family != "mission" || unreadable.Reason != "fixture inventory unreadable" {
		t.Fatalf("crashed-stop unreadable family = %T %v", err, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 {
		t.Fatalf("crashed-stop unreadable record = %#v err=%v", record, readErr)
	}
	survivor := record.NotStopped[0]
	if survivor.Component != "family-inventory" || survivor.ID != "mission" || survivor.Reason != "fixture inventory unreadable" {
		t.Fatalf("crashed-stop unreadable survivor = %#v", survivor)
	}
}

func TestUnknownFamilyIsReportedAndRecordedAfterFenceClose(t *testing.T) {
	item := Item{Key: "future:one", StatusLine: "future one running", Survivor: stopfence.Survivor{Component: "future", ID: "one"}}
	family := &scriptedFamily{name: "future", inventories: [][]Item{{item}, nil}, complete: true}
	transition := testTransition(t, family)
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 1 || family.stops != 1 ||
		!strings.Contains(joined, "future one running: stopped (TERM)") ||
		!strings.Contains(joined, "NOT STOPPED future shutdown step: family future has no numbered shutdown step; did: continued to the next stop step") {
		t.Fatalf("unknown-family report = %#v err=%v", report, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 {
		t.Fatalf("unknown-family fence = %#v err=%v", record, readErr)
	}
	if survivor := record.NotStopped[0]; survivor.Component != "family" || survivor.ID != "future" || survivor.Reason != "family future has no numbered shutdown step" {
		t.Fatalf("unknown-family survivor = %#v", survivor)
	}
}

func TestLateRetryUpdatesReportAndFinalSurvivorTruth(t *testing.T) {
	for _, test := range []struct {
		name            string
		completions     []bool
		wantExit        int
		wantPhase       string
		wantNotStopped  int
		wantClosingPart string
	}{
		{name: "retry succeeds", completions: []bool{false, true}, wantExit: 0, wantPhase: stopfence.PhaseStopped, wantNotStopped: 0, wantClosingPart: "stopped /checkout; start again"},
		{name: "retry remains incomplete", completions: []bool{false, false}, wantExit: 1, wantPhase: stopfence.PhaseStopIncomplete, wantNotStopped: 1, wantClosingPart: "1 not stopped, listed above"},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := Item{Key: "run:retry", StatusLine: "run retry running", Survivor: stopfence.Survivor{Component: "run", ID: "retry", Pid: 61, PidStartedAt: 20}}
			family := &scriptedFamily{name: "run", inventories: [][]Item{{item}, {item}, nil}, completions: test.completions}
			transition := testTransition(t, family)
			report, err := transition.Stop()
			joined := strings.Join(report.Lines, "\n")
			if err != nil || report.ExitCode != test.wantExit || family.stops != 2 || !strings.Contains(joined, test.wantClosingPart) {
				t.Fatalf("retry report=%#v stops=%d err=%v", report, family.stops, err)
			}
			wantAttempts := 1
			if test.wantExit == 0 {
				wantAttempts = 0
			}
			if got := strings.Count(joined, "NOT STOPPED run retry running"); got != wantAttempts {
				t.Fatalf("retry NOT STOPPED attempts = %d, want %d: %q", got, wantAttempts, joined)
			}
			if test.wantExit == 0 && !strings.Contains(joined, "run retry running: stopped (TERM)") {
				t.Fatalf("successful retry outcome was not printed: %q", joined)
			}
			record, readErr := stopfence.Read(transition.Root)
			if readErr != nil || record.Phase != test.wantPhase || len(record.NotStopped) != test.wantNotStopped {
				t.Fatalf("retry fence=%#v err=%v", record, readErr)
			}
		})
	}
}

type orderedFamily struct {
	name        string
	events      *[]string
	inventories int
}

func (f *orderedFamily) Name() string { return f.name }
func (f *orderedFamily) Inventory() ([]Item, error) {
	*f.events = append(*f.events, "inventory:"+f.name)
	f.inventories++
	if f.inventories > 1 {
		return nil, nil
	}
	return []Item{{Key: f.name + ":one", StatusLine: f.name + " one running"}}, nil
}
func (f *orderedFamily) Stop(item Item) (Outcome, error) {
	*f.events = append(*f.events, "stop:"+f.name)
	return Outcome{Line: item.StatusLine + ": stopped", Complete: true}, nil
}

func TestStopObserverErrorIsReportedAndLaterFamiliesStillRun(t *testing.T) {
	var events []string
	first := &orderedFamily{name: "mission", events: &events}
	second := &orderedFamily{name: "job", events: &events}
	third := &orderedFamily{name: "proof-run", events: &events}
	transition := testTransition(t, first)
	transition.Families = []Family{first, second, third}
	observerErr := errors.New("injected observer error")
	transition.AfterStep = func(step int) error {
		events = append(events, "after:"+string(rune('0'+step)))
		if step == 2 {
			return observerErr
		}
		return nil
	}
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 1 || !strings.Contains(joined, "NOT STOPPED observer after shutdown step 2: injected observer error") {
		t.Fatalf("observer result = %#v err=%v", report, err)
	}
	want := "inventory:mission,inventory:job,inventory:proof-run,stop:mission,after:1,stop:job,after:2,stop:proof-run,after:3,inventory:mission,inventory:job,inventory:proof-run,after:8,inventory:mission,inventory:job,inventory:proof-run,after:9"
	if got := strings.Join(events, ","); got != want {
		t.Fatalf("event order = %s, want %s", got, want)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.State != stopfence.StateClosed || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].Component != "observer" {
		t.Fatalf("observer fence = %#v err=%v", record, readErr)
	}
}

func TestStopObserverErrorsUseSectionFourStepNumbersWithoutEndingTheTransaction(t *testing.T) {
	for _, test := range []struct {
		crashAt int
		want    string
	}{
		{crashAt: 7, want: "stop:mission,after:1,stop:job,after:2,stop:proof-run,after:3,stop:run,after:4,stop:steward,after:5,stop:supervision,after:6,after:7,after:8,after:9"},
		{crashAt: 8, want: "stop:mission,after:1,stop:job,after:2,stop:proof-run,after:3,stop:run,after:4,stop:steward,after:5,stop:supervision,after:6,after:7,after:8,after:9"},
		{crashAt: 9, want: "stop:mission,after:1,stop:job,after:2,stop:proof-run,after:3,stop:run,after:4,stop:steward,after:5,stop:supervision,after:6,after:7,after:8,after:9"},
	} {
		t.Run(string(rune('0'+test.crashAt)), func(t *testing.T) {
			var events []string
			families := []Family{}
			for _, name := range []string{"mission", "job", "proof-run", "run", "steward", "supervision", "untracked"} {
				families = append(families, &orderedFamily{name: name, events: &events})
			}
			transition := testTransition(t, families[0])
			transition.Families = families
			observerErr := errors.New("injected observer error")
			transition.AfterStep = func(step int) error {
				events = append(events, "after:"+string(rune('0'+step)))
				if step == test.crashAt {
					return observerErr
				}
				return nil
			}
			report, err := transition.Stop()
			if err != nil || report.ExitCode != 1 || !strings.Contains(strings.Join(report.Lines, "\n"), "observer after shutdown step "+string(rune('0'+test.crashAt))) {
				t.Fatalf("step %d observer result = %#v err=%v", test.crashAt, report, err)
			}
			var actions []string
			for _, event := range events {
				if strings.HasPrefix(event, "stop:") || strings.HasPrefix(event, "after:") {
					actions = append(actions, event)
				}
			}
			if got := strings.Join(actions, ","); got != test.want {
				t.Fatalf("step %d action order = %s, want %s", test.crashAt, got, test.want)
			}
		})
	}
}

func TestStewardFamilyReportsNarratorBesideRunner(t *testing.T) {
	family := newStewardFamily(t.TempDir())
	item := Item{Key: "steward:runner"}
	family.items[item.Key] = steward.RunnerRecord{Pid: 41, PidStartedAt: 42}
	outcome, err := family.Stop(item)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Line != "steward-runner pid 41 started 42: already gone" {
		t.Fatalf("steward line = %q", outcome.Line)
	}
	if got := strings.Join(outcome.AdditionalLines, "\n"); got != "narrator: stopped with the steward runner" {
		t.Fatalf("narrator lines = %q", got)
	}
	report := Report{}
	appendOutcomeLines(&report, outcome, "")
	want := "steward-runner pid 41 started 42: already gone\nnarrator: stopped with the steward runner"
	if got := strings.Join(report.Lines, "\n"); got != want {
		t.Fatalf("steward report = %q, want %q", got, want)
	}
}

func TestJobFamilyStatusLineKeepsRecordStatusAndAddsVerdict(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"jobId":"stop-fixture-job","status":"running","pid":33550,"pidStartedAt":33500,"pgid":33550,"role":"design-critic"}`
	if err := os.WriteFile(filepath.Join(jobs, "stop-fixture-job.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := newJobFamily(LocalConfig{Root: root}).Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("job inventory has %d items, want 1", len(items))
	}
	got := items[0].StatusLine
	want := "job stop-fixture-job running pid 33550 pgid 33550 role design-critic: running"
	if got != want {
		t.Fatalf("job status line = %q, want %q", got, want)
	}
}

func TestJobFamilyInventoryFailurePreservesTheRecordPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "jobs", "broken.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := newJobFamily(LocalConfig{Root: root}).Inventory()
	var readErr *InventoryReadError
	if !errors.As(err, &readErr) || readErr.Path != path || !strings.Contains(readErr.Error(), "cannot read the job record") || !strings.Contains(readErr.Error(), "EOF") {
		t.Fatalf("job inventory error = %T %v", err, err)
	}
}

func TestRemoteJobStopAndArmRefusalNameMachineAndCancelCommand(t *testing.T) {
	jobID := "remote-one"
	machine := "other-machine"
	job := newJobFamily(LocalConfig{})
	key := "job:" + jobID
	record := map[string]any{"jobId": jobID, "status": "running"}
	job.items[key] = jobItem{record: record, lens: dispatch.JobRecordOf(record), remote: true, machine: machine}
	outcome, err := job.Stop(Item{Key: key})
	wantStop := "NOT STOPPED job remote-one running machine other-machine: owned by another machine; did: nothing, cancel it from other-machine with metasystem delegate --cancel remote-one"
	if err != nil || outcome.Complete || outcome.Line != wantStop {
		t.Fatalf("remote stop = %#v err=%v", outcome, err)
	}

	transition := testTransition(t, &scriptedFamily{name: "run"})
	jobPath := filepath.Join(transition.Root, "artifacts", "agents", "jobs", jobID+".json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte(`{"jobId":"remote-one","status":"running","machineId":"other-machine"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := stopfence.Write(transition.Root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 3,
		ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
		NotStopped: []stopfence.Survivor{{Component: "job", ID: jobID, MachineID: machine}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = transition.OpenFence("arm")
	var remote *RemoteJobSurvivorError
	if !errors.As(err, &remote) || err.Error() != "job remote-one is owned by machine other-machine and is not terminal" || strings.Contains(err.Error(), "\n") {
		t.Fatalf("remote arm sentence = %v", err)
	}
	if got := remote.Remedy(); got != "cancel it from other-machine with metasystem delegate --cancel remote-one" {
		t.Fatalf("remote arm remedy = %q", got)
	}
}

func TestArmRefusalsPreserveTheirExactBranchEvidence(t *testing.T) {
	t.Run("stop in progress", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.Prober = fakeProber{10: identity.Alive, 22: identity.Alive}
		if err := stopfence.Write(transition.Root, stopfence.Record{
			State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 4,
			ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
			By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 22, PidStartedAt: 20}},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := transition.OpenFence("arm")
		var typed *StopInProgressError
		if !errors.As(err, &typed) || err.Error() != "a checkout transition by pid 22 holds the checkout" {
			t.Fatalf("stop-in-progress error = %T %v", err, err)
		}
	})

	t.Run("local survivor", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.Prober = fakeProber{10: identity.Alive, 33: identity.Alive}
		if err := stopfence.Write(transition.Root, stopfence.Record{
			State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 4,
			ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
			NotStopped: []stopfence.Survivor{{Component: "run", ID: "held", Pid: 33, PidStartedAt: 20}},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := transition.OpenFence("arm")
		var typed *LocalSurvivorError
		if !errors.As(err, &typed) || err.Error() != "run pid 33 started 20 survived the last stop" {
			t.Fatalf("local-survivor error = %T %v", err, err)
		}
	})

	t.Run("unknown creator", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.Prober = fakeProber{10: identity.Alive, 44: identity.Unknown}
		claimPath := filepath.Join(stopfence.CreatingDir(transition.Root), "run-launch-44.json")
		if err := stopfence.Write(transition.Root, stopfence.Record{
			State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 4,
			ChangedAt: "2026-09-07T00:00:00Z", Checkout: transition.Checkout,
			NotStopped: []stopfence.Survivor{{Component: "creator", ID: claimPath, Tag: "run-launch", Pid: 44, PidStartedAt: 20, Reason: "liveness unknown"}},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := transition.OpenFence("arm")
		var typed *CreatorClaimSurvivorError
		if !errors.As(err, &typed) || !strings.Contains(err.Error(), "creator run-launch claim "+claimPath) {
			t.Fatalf("creator-survivor error = %T %v", err, err)
		}
	})
}

func TestTransitionLockContentionClassifiesOnlyLiveHolderAsStopInProgress(t *testing.T) {
	t.Run("live holder", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.ScaleMilli = 1
		exact, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
		if err != nil || state != identity.Alive {
			t.Fatalf("current identity = %#v state=%v err=%v", exact, state, err)
		}
		transition.Self = func() (identity.Ref, error) { return exact.Ref(), nil }
		held, err := stopfence.Acquire(transition.Root, "stop", exact.Ref(), 1)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Release()
		_, err = transition.Arm()
		var inProgress *StopInProgressError
		if !errors.As(err, &inProgress) || inProgress.Pid != int64(os.Getpid()) {
			t.Fatalf("live-holder arm = %T %v", err, err)
		}
	})

	t.Run("unreadable holder", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.ScaleMilli = 1
		if err := os.MkdirAll(stopfence.LockPath(transition.Root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stopfence.LockPath(transition.Root), "owner.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := transition.Arm()
		var holder *lock.HolderError
		var inProgress *StopInProgressError
		if !errors.As(err, &holder) || holder.State != lock.Unknown || errors.As(err, &inProgress) {
			t.Fatalf("unreadable-holder arm = %T %v", err, err)
		}
	})

	t.Run("dead holder", func(t *testing.T) {
		transition := testTransition(t, &scriptedFamily{name: "run"})
		transition.ScaleMilli = 1
		if err := os.MkdirAll(stopfence.LockPath(transition.Root), 0o755); err != nil {
			t.Fatal(err)
		}
		owner := `{"pid":2147483647,"pidStartedAt":1,"label":"stop"}`
		if err := os.WriteFile(filepath.Join(stopfence.LockPath(transition.Root), "owner.json"), []byte(owner), 0o644); err != nil {
			t.Fatal(err)
		}
		report, err := transition.Arm()
		if err != nil || report.ExitCode != 0 {
			t.Fatalf("dead-holder arm = %#v err=%v", report, err)
		}
	})
}

func TestStopRemovesMalformedClaimAndFinishesTheTransaction(t *testing.T) {
	item := Item{Key: "run:after-claim", StatusLine: "run after-claim running", Survivor: stopfence.Survivor{Component: "run", ID: "after-claim"}}
	family := &scriptedFamily{name: "run", inventories: [][]Item{{item}, nil}, complete: true}
	transition := testTransition(t, family)
	claimPath := filepath.Join(stopfence.CreatingDir(transition.Root), "broken.json")
	if err := os.MkdirAll(filepath.Dir(claimPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claimPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := transition.Stop()
	if err != nil || report.ExitCode != 1 || family.stops != 1 {
		t.Fatalf("malformed-claim stop = %#v err=%v", report, err)
	}
	joined := strings.Join(report.Lines, "\n")
	if !strings.Contains(joined, "creator claim "+claimPath) || !strings.Contains(joined, "did: removed "+claimPath) || !strings.Contains(joined, "stop incomplete for /checkout; 1 not stopped") {
		t.Fatalf("malformed-claim report = %q", joined)
	}
	if _, err := os.Stat(claimPath); !os.IsNotExist(err) {
		t.Fatalf("malformed claim survived: %v", err)
	}
	record, err := stopfence.Read(transition.Root)
	if err != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].Component != "creator-claim" || record.NotStopped[0].ID != claimPath {
		t.Fatalf("final fence = %#v err=%v", record, err)
	}
}

func TestStopWaitsUnknownClaimOnceRemovesItAndDoesNotWedgeNextStop(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run"})
	transition.Prober = fakeProber{10: identity.Alive, 55: identity.Unknown}
	now := time.Unix(100, 0)
	transition.Now = func() time.Time { return now }
	transition.ScaleMilli = 1
	transition.Sleep = func(duration time.Duration) { now = now.Add(duration) }
	claimPath := filepath.Join(stopfence.CreatingDir(transition.Root), "run-launch-55.json")
	if err := os.MkdirAll(filepath.Dir(claimPath), 0o755); err != nil {
		t.Fatal(err)
	}
	claim := stopfence.CreationClaim{
		SchemaVersion: stopfence.SchemaVersion, Verb: "run-launch", Generation: 0,
		Creator:  stopfence.Process{Pid: 55, PidStartedAt: 20},
		OpenedAt: time.Unix(99, 0).UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(claim)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claimPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := transition.Stop()
	if err != nil || report.ExitCode != 1 {
		t.Fatalf("unknown-claim stop = %#v err=%v", report, err)
	}
	joined := strings.Join(report.Lines, "\n")
	if !strings.Contains(joined, "NOT STOPPED creator run-launch claim "+claimPath+": liveness unknown") || !strings.Contains(joined, "removed creation claim "+claimPath) {
		t.Fatalf("unknown-claim report = %q", joined)
	}
	if _, err := os.Stat(claimPath); !os.IsNotExist(err) {
		t.Fatalf("unknown claim survived: %v", err)
	}
	record, err := stopfence.Read(transition.Root)
	if err != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].ID != claimPath {
		t.Fatalf("unknown-claim fence = %#v err=%v", record, err)
	}
	transition.Prober = fakeProber{10: identity.Alive, 55: identity.Dead}
	second, err := transition.Stop()
	if err != nil || second.ExitCode != 0 {
		t.Fatalf("second stop remained wedged = %#v err=%v", second, err)
	}
}

func TestUnreadableFenceStatusArmAndStopBehavior(t *testing.T) {
	for _, fixture := range []struct {
		name           string
		body           string
		wantGeneration int64
	}{
		{name: "malformed JSON", body: "{", wantGeneration: 1},
		{name: "future schema", body: `{"schemaVersion":2,"state":"closed","phase":"stopped","generation":12}`, wantGeneration: 13},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			newTransition := func(t *testing.T) *Transition {
				transition := testTransition(t, &scriptedFamily{name: "run"})
				path := stopfence.TransitionPath(transition.Root)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(fixture.body), 0o644); err != nil {
					t.Fatal(err)
				}
				return transition
			}

			t.Run("status", func(t *testing.T) {
				transition := newTransition(t)
				transition.Families = []Family{
					&scriptedFamily{name: "run", inventories: [][]Item{
						{{Key: "run:retained", StatusLine: "run retained running"}},
					}},
				}
				report, err := transition.Status()
				joined := strings.Join(report.Lines, "\n")
				if err != nil || report.ExitCode != 1 || !strings.Contains(joined, "checkout /checkout\nrun retained running\nfence record unreadable:") || !strings.Contains(joined, "repair with metasystem arm --repo /checkout") || !strings.Contains(joined, "status incomplete for /checkout; 1 read failures") {
					t.Fatalf("unreadable status = %#v err=%v", report, err)
				}
			})

			t.Run("stop", func(t *testing.T) {
				transition := newTransition(t)
				_, err := transition.Stop()
				var unreadable *stopfence.RecordUnreadableError
				if !errors.As(err, &unreadable) {
					t.Fatalf("unreadable stop error = %T %v", err, err)
				}
			})

			t.Run("arm", func(t *testing.T) {
				transition := newTransition(t)
				report, err := transition.Arm()
				joined := strings.Join(report.Lines, "\n")
				if err != nil || report.ExitCode != 0 || !strings.Contains(joined, "fence record replaced (generation counter recovered; was unreadable:") || !strings.Contains(joined, "armed /checkout generation") {
					t.Fatalf("unreadable arm = %#v err=%v", report, err)
				}
				record, readErr := stopfence.Read(transition.Root)
				if readErr != nil || record.State != stopfence.StateOpen || record.Generation != fixture.wantGeneration {
					t.Fatalf("replacement fence = %#v err=%v", record, readErr)
				}
			})
		})
	}
}

func TestUnreadableFenceRepairUsesHighestDurableGeneration(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run"})
	transitionPath := stopfence.TransitionPath(transition.Root)
	if err := os.MkdirAll(filepath.Dir(transitionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transitionPath, []byte(`{"schemaVersion":1,"state":"open","phase":"armed","generation":7`), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(transition.Root, "artifacts", "agents", "supervision", "state.json")
	if err := os.WriteFile(statePath, []byte(`{"generation":8}`), 0o644); err != nil {
		t.Fatal(err)
	}
	registryHome := t.TempDir()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", registryHome)
	registryPath := filepath.Join(registryHome, ".metasystem", "armed-checkouts.jsonl")
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	registryRow := `{"schemaVersion":1,"event":"relaunched","checkoutPath":"/checkout","at":"2026-09-07T00:00:00Z","ownerTag":"owner","generation":11,"watcherTag":"watcher","reaperTag":"reaper","retiredThrough":0}` + "\n"
	if err := os.WriteFile(registryPath, []byte(registryRow), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := transition.Arm()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 0 || !strings.Contains(joined, "generation counter recovered") || !strings.Contains(joined, "armed /checkout generation 12") {
		t.Fatalf("recovered arm = %#v err=%v", report, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || record.Generation != 12 {
		t.Fatalf("recovered generation fence = %#v err=%v", record, readErr)
	}
}

func TestRemoteJobEvidenceCasesPreserveTheClosedFenceOnRefusal(t *testing.T) {
	const jobID = "remote-evidence"
	const machine = "machine-b"
	for _, test := range []struct {
		name        string
		body        string
		injectErr   error
		wantOpen    bool
		wantKind    string
		wantOpenJob bool
	}{
		{name: "terminal", body: `{"jobId":"remote-evidence","machineId":"machine-b","status":"completed"}`, wantOpen: true},
		{name: "open", body: `{"jobId":"remote-evidence","machineId":"machine-b","status":"running"}`, wantOpenJob: true},
		{name: "missing", wantKind: "missing"},
		{name: "malformed", body: `{`, wantKind: "unreadable"},
		{name: "injected read failure", body: `{}`, injectErr: errors.New("injected record read failure"), wantKind: "unreadable"},
		{name: "mismatched machine", body: `{"jobId":"remote-evidence","machineId":"machine-c","status":"completed"}`, wantKind: "mismatched"},
		{name: "mismatched job", body: `{"jobId":"another-job","machineId":"machine-b","status":"completed"}`, wantKind: "mismatched"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transition := testTransition(t, &scriptedFamily{name: "run"})
			jobPath := filepath.Join(transition.Root, "artifacts", "agents", "jobs", jobID+".json")
			if test.body != "" {
				if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(jobPath, []byte(test.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			record := stopfence.Record{
				State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 8,
				ChangedAt: "2026-09-08T00:00:00Z", Checkout: transition.Checkout,
				NotStopped: []stopfence.Survivor{{Component: "job", ID: jobID, MachineID: machine, Reason: "remote terminal state unproven"}},
			}
			if err := stopfence.Write(transition.Root, record); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(stopfence.TransitionPath(transition.Root))
			if err != nil {
				t.Fatal(err)
			}
			if test.injectErr != nil {
				transition.JobRecordRead = func(string) (map[string]any, error) { return nil, test.injectErr }
			}
			_, armErr := transition.OpenFence("arm")
			if test.wantOpen {
				if armErr != nil {
					t.Fatalf("terminal evidence did not clear the survivor: %v", armErr)
				}
				opened, readErr := stopfence.Read(transition.Root)
				if readErr != nil || opened.State != stopfence.StateOpen {
					t.Fatalf("opened fence = %#v err=%v", opened, readErr)
				}
				return
			}
			if test.wantOpenJob {
				var openJob *RemoteJobSurvivorError
				if !errors.As(armErr, &openJob) || !strings.Contains(openJob.Remedy(), "machine-b") {
					t.Fatalf("open remote job refusal = %T %v", armErr, armErr)
				}
			} else {
				var evidence *RemoteJobEvidenceError
				if !errors.As(armErr, &evidence) || evidence.Kind != test.wantKind || !strings.Contains(armErr.Error(), jobPath) {
					t.Fatalf("remote evidence refusal = %T %v", armErr, armErr)
				}
			}
			after, err := os.ReadFile(stopfence.TransitionPath(transition.Root))
			if err != nil || string(after) != string(before) {
				t.Fatalf("arm refusal changed the closed fence: err=%v before=%q after=%q", err, before, after)
			}
		})
	}
}

func TestMissingRemoteJobSurvivesRepeatedStopsUntilMatchingTerminalEvidence(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{nil, nil, nil, nil, nil, nil, nil, nil}})
	remote := stopfence.Survivor{Component: "job", ID: "remote-retained", MachineID: "machine-b", Reason: "remote terminal state unproven"}
	if err := stopfence.Write(transition.Root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 4,
		ChangedAt: "2026-09-08T00:00:00Z", Checkout: transition.Checkout, NotStopped: []stopfence.Survivor{remote},
	}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		report, err := transition.Stop()
		joined := strings.Join(report.Lines, "\n")
		if err != nil || report.ExitCode != 1 || strings.Count(joined, "NOT STOPPED") != 1 || !strings.Contains(joined, "record ") || !strings.Contains(joined, " is missing") {
			t.Fatalf("missing-evidence stop %d = %#v err=%v", attempt+1, report, err)
		}
		record, readErr := stopfence.Read(transition.Root)
		if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || record.NotStopped[0].MachineID != "machine-b" {
			t.Fatalf("retained remote stop %d = %#v err=%v", attempt+1, record, readErr)
		}
	}
	jobPath := filepath.Join(transition.Root, "artifacts", "agents", "jobs", "remote-retained.json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte(`{"jobId":"remote-retained","machineId":"machine-b","status":"running"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := transition.OpenFence("arm"); err == nil {
		t.Fatal("a readable non-terminal record discharged the remote survivor")
	}
	if err := os.WriteFile(jobPath, []byte(`{"jobId":"remote-retained","machineId":"machine-b","status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := transition.OpenFence("arm"); err != nil {
		t.Fatalf("matching terminal evidence did not allow arm: %v", err)
	}
}

func TestPersistentSurvivorHasOneFinalLineDespiteEveryStopAttempt(t *testing.T) {
	item := Item{Key: "run:persistent", StatusLine: "run persistent running", Survivor: stopfence.Survivor{Component: "run", ID: "persistent", Pid: 75, PidStartedAt: 20}}
	family := &scriptedFamily{name: "run", inventories: [][]Item{{item}, {item}, {item}, {item}, {item}}, complete: false}
	report, err := testTransition(t, family).Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || family.stops != 5 || strings.Count(joined, "NOT STOPPED run persistent running") != 1 || report.ExitCode != 1 {
		t.Fatalf("persistent survivor report=%#v stops=%d err=%v", report, family.stops, err)
	}
}

func TestFinalPublicationFailureLeavesStoppingBytesAndCountsRecordingFailure(t *testing.T) {
	for _, test := range []struct {
		name      string
		family    *scriptedFamily
		wantCount int
	}{
		{name: "all processes stopped", family: &scriptedFamily{name: "run", inventories: [][]Item{nil}}, wantCount: 1},
		{name: "one survivor", family: &scriptedFamily{name: "run", inventories: [][]Item{{{Key: "run:one", StatusLine: "run one running", Survivor: stopfence.Survivor{Component: "run", ID: "one"}}}, nil}, complete: false}, wantCount: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			transition := testTransition(t, test.family)
			writes := 0
			var first []byte
			transition.FenceWrite = func(record stopfence.Record) (bool, error) {
				writes++
				if writes == 2 {
					return false, errors.New("injected final publication failure")
				}
				durable, err := stopfence.Publish(transition.Root, record)
				if err == nil {
					first, err = os.ReadFile(stopfence.TransitionPath(transition.Root))
				}
				return durable, err
			}
			report, err := transition.Stop()
			joined := strings.Join(report.Lines, "\n")
			if err != nil || report.ExitCode != 1 || writes != 2 || !strings.Contains(joined, "final result could not be saved") || !strings.Contains(report.Lines[len(report.Lines)-1], fmt.Sprintf("%d not stopped", test.wantCount)) || !strings.Contains(report.Lines[len(report.Lines)-1], "final result not saved") {
				t.Fatalf("publication failure = %#v writes=%d err=%v", report, writes, err)
			}
			after, readErr := os.ReadFile(stopfence.TransitionPath(transition.Root))
			if readErr != nil || string(after) != string(first) {
				t.Fatalf("failed final publication changed first bytes: err=%v first=%q after=%q", readErr, first, after)
			}
			record, readErr := stopfence.Read(transition.Root)
			if readErr != nil || record.Phase != stopfence.PhaseStopping {
				t.Fatalf("published fence after failure = %#v err=%v", record, readErr)
			}
			status, statusErr := transition.Status()
			if statusErr != nil || status.ExitCode != 0 || !strings.Contains(strings.Join(status.Lines, "\n"), "stop unfinished since") {
				t.Fatalf("status over unpublished result = %#v err=%v", status, statusErr)
			}
		})
	}
}

func TestCommittedDurabilityDoubtKeepsPublishedFinalResult(t *testing.T) {
	transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{nil}})
	writes := 0
	transition.FenceWrite = func(record stopfence.Record) (bool, error) {
		writes++
		durable, err := stopfence.Publish(transition.Root, record)
		if writes == 2 && err == nil {
			return false, nil
		}
		return durable, err
	}
	report, err := transition.Stop()
	joined := strings.Join(report.Lines, "\n")
	if err != nil || report.ExitCode != 0 || writes != 2 || !strings.Contains(joined, "final result published; durability unconfirmed") || strings.Contains(joined, "NOT STOPPED fence record") {
		t.Fatalf("durability doubt report=%#v writes=%d err=%v", report, writes, err)
	}
	record, readErr := stopfence.Read(transition.Root)
	if readErr != nil || !stopfence.Completed(record) {
		t.Fatalf("doubted publication contents = %#v err=%v", record, readErr)
	}
}

func TestCrashedStopReinventoryRetainsAndRefreshesRemoteEvidenceBeforeArm(t *testing.T) {
	newCrashed := func(t *testing.T) *Transition {
		transition := testTransition(t, &scriptedFamily{name: "run", inventories: [][]Item{nil}})
		transition.Prober = fakeProber{10: identity.Alive, 22: identity.Dead}
		if err := stopfence.Write(transition.Root, stopfence.Record{
			State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 9,
			ChangedAt: "2026-09-08T03:00:00Z", Checkout: transition.Checkout,
			By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 22, PidStartedAt: 20}},
			NotStopped: []stopfence.Survivor{{Component: "job", ID: "remote-crash", MachineID: "machine-b", Reason: "carried before interruption"}},
		}); err != nil {
			t.Fatal(err)
		}
		return transition
	}

	t.Run("published re-inventory retains missing evidence", func(t *testing.T) {
		transition := newCrashed(t)
		_, err := transition.OpenFence("arm")
		var evidence *RemoteJobEvidenceError
		if !errors.As(err, &evidence) || evidence.Kind != "missing" {
			t.Fatalf("arm after interrupted stop = %T %v", err, err)
		}
		record, readErr := stopfence.Read(transition.Root)
		if readErr != nil || record.Phase != stopfence.PhaseStopIncomplete || len(record.NotStopped) != 1 || !strings.Contains(record.NotStopped[0].Reason, " is missing") {
			t.Fatalf("crashed-stop remote retention = %#v err=%v", record, readErr)
		}
	})

	t.Run("failed re-inventory publication refuses to open", func(t *testing.T) {
		transition := newCrashed(t)
		var attempted stopfence.Record
		transition.FenceWrite = func(record stopfence.Record) (bool, error) {
			attempted = record
			return false, errors.New("injected re-inventory write failure")
		}
		_, err := transition.OpenFence("arm")
		var publication *FencePublicationError
		if !errors.As(err, &publication) || len(attempted.NotStopped) != 1 || !strings.Contains(attempted.NotStopped[0].Reason, " is missing") {
			t.Fatalf("failed crashed-stop publication = %T %v attempted=%#v", err, err, attempted)
		}
		record, readErr := stopfence.Read(transition.Root)
		if readErr != nil || record.Phase != stopfence.PhaseStopping {
			t.Fatalf("failed re-inventory opened or rewrote fence = %#v err=%v", record, readErr)
		}
	})
}

type statusFixtureFamily struct {
	name   string
	items  []Item
	err    error
	events *[]string
}

func (f *statusFixtureFamily) Name() string { return f.name }
func (f *statusFixtureFamily) Inventory() ([]Item, error) {
	if f.events != nil {
		*f.events = append(*f.events, f.name)
	}
	return f.items, f.err
}
func (f *statusFixtureFamily) Stop(Item) (Outcome, error) {
	return Outcome{}, errors.New("unreachable")
}

func TestStatusShowsPhaseSurvivorsAndPartialInventoryTruth(t *testing.T) {
	t.Run("incomplete record with no live process", func(t *testing.T) {
		transition := testTransition(t, &statusFixtureFamily{name: "run"})
		record := stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 2,
			ChangedAt: "2026-09-08T01:00:00Z", Checkout: transition.Checkout, By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 81}},
			NotStopped: []stopfence.Survivor{{Component: "job", ID: "remote", MachineID: "machine-b", Reason: "terminal state remains unproven"}}}
		if err := stopfence.Write(transition.Root, record); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(stopfence.TransitionPath(transition.Root))
		report, err := transition.Status()
		joined := strings.Join(report.Lines, "\n")
		if err != nil || report.ExitCode != 0 || !strings.Contains(joined, "unresolved from last stop: job remote machine machine-b") || !strings.Contains(joined, "no live processes found; the last stop is still unresolved") || !strings.Contains(joined, "1 unresolved entries from the last stop") || strings.Contains(joined, "nothing is running") {
			t.Fatalf("incomplete status = %#v err=%v", report, err)
		}
		after, _ := os.ReadFile(stopfence.TransitionPath(transition.Root))
		if string(after) != string(before) {
			t.Fatal("status changed the durable fence")
		}
	})

	t.Run("phase and list disagreements remain incomplete", func(t *testing.T) {
		for _, record := range []stopfence.Record{
			{State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 2, ChangedAt: "now", By: stopfence.Actor{Verb: "stop"}},
			{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 2, ChangedAt: "now", By: stopfence.Actor{Verb: "stop"}, NotStopped: []stopfence.Survivor{{Component: "run", ID: "one", Reason: "unresolved"}}},
		} {
			transition := testTransition(t, &statusFixtureFamily{name: "run"})
			if err := stopfence.Write(transition.Root, record); err != nil {
				t.Fatal(err)
			}
			report, err := transition.Status()
			if err != nil || !strings.Contains(strings.Join(report.Lines, "\n"), "stop incomplete for /checkout") {
				t.Fatalf("phase disagreement status = %#v err=%v", report, err)
			}
		}
	})

	t.Run("family failures do not hide later families or fence", func(t *testing.T) {
		var events []string
		transition := testTransition(t, &statusFixtureFamily{name: "first", items: []Item{{Key: "first:one", StatusLine: "first one running"}}, events: &events})
		transition.Families = []Family{
			transition.Families[0],
			&statusFixtureFamily{name: "broken", err: &InventoryReadError{Path: "/records/broken.json", Err: errors.New("broken JSON")}, events: &events},
			&statusFixtureFamily{name: "later", items: []Item{{Key: "later:one", StatusLine: "later one running"}}, events: &events},
			&statusFixtureFamily{name: "also-broken", err: errors.New("source offline"), events: &events},
		}
		if err := stopfence.Write(transition.Root, stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 2, ChangedAt: "now", Checkout: transition.Checkout, By: stopfence.Actor{Verb: "stop"}}); err != nil {
			t.Fatal(err)
		}
		report, err := transition.Status()
		joined := strings.Join(report.Lines, "\n")
		if err != nil || report.ExitCode != 1 || strings.Join(events, ",") != "first,broken,later,also-broken" || !strings.Contains(joined, "first one running\ninventory unreadable: broken /records/broken.json: broken JSON\nlater one running\ninventory unreadable: also-broken records: source offline") || !strings.Contains(joined, "recorded fence: stopped since") || !strings.Contains(joined, "status incomplete for /checkout; 2 read failures") || strings.Contains(joined, "nothing is running") {
			t.Fatalf("partial status = %#v events=%v err=%v", report, events, err)
		}
	})
}
