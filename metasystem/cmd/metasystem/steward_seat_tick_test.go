package main

// The dry end-to-end check of the steward's seat start (g1-s77 integration):
// one production tick over a real Git checkout whose installation holds one
// ready goal, the seat it selects started through the command layer's seat
// launcher into a launch manager whose supervisor runs in this process over
// a recording process system. No runtime, no session and no real seat run.

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// seatTickCensus proves no worker alive and the census complete.
type seatTickCensus struct{}

func (seatTickCensus) Workers(string) (steward.Workers, error) {
	return steward.Workers{CensusComplete: true}, nil
}

// seatTickLiveMain is the census once a main has announced itself.
type seatTickLiveMain struct{}

func (seatTickLiveMain) Workers(string) (steward.Workers, error) {
	return steward.Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true}, nil
}

// seatTickProber answers the supervisor (10) and the child (20) alive.
type seatTickProber struct{}

func (seatTickProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == 10 || pid == 20 {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
	}
	return identity.Exact{Pid: pid}, identity.Dead, nil
}

type seatTickChild struct{}

func (seatTickChild) Wait() (int, error) { return 0, nil }

// seatTickProcesses records the one command the supervisor starts.
type seatTickProcesses struct{ commands []launch.Command }

func (*seatTickProcesses) SelfRef() (identity.Ref, error) {
	return identity.Ref{Pid: 10, StartedAtSec: 10}, nil
}
func (p *seatTickProcesses) StartChild(command launch.Command) (launch.Child, identity.Ref, error) {
	p.commands = append(p.commands, command)
	return seatTickChild{}, identity.Ref{Pid: 20, StartedAtSec: 20}, nil
}
func (*seatTickProcesses) SignalGroup(int64, syscall.Signal) error { return nil }
func (*seatTickProcesses) GroupAlive(int64) (bool, error)          { return false, nil }

// seatTickSupervisor runs `metasystem launch supervise --id` in this process.
type seatTickSupervisor struct {
	t       *testing.T
	manager *launch.Manager
}

func (s seatTickSupervisor) StartSupervisor(id, _ string) (identity.Ref, error) {
	if _, err := s.manager.Supervise(id); err != nil {
		s.t.Logf("supervise %s: %v", id, err)
	}
	return identity.Ref{Pid: 10, StartedAtSec: 10}, nil
}

func seatTickGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// seatTickCheckout is a Git checkout with the installation at metasystem/,
// its accepted goal ledger in local mode holding one approved ready goal.
func seatTickCheckout(t *testing.T) (top, install string) {
	t.Helper()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	install = filepath.Join(top, "metasystem")
	intent := "Serve the seat"
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "m1", "approval")
	ready := &goal.GoalFile{Id: "fix-docs", State: goal.StateApproved, Tier: 3, Intent: intent, Origin: goal.OriginMain,
		NextStep: "Build it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2, Budget: &budget,
		History: []goal.HistoryLine{
			{At: "2026-08-23T00:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "m1", "coordinator"), Verb: "open",
				Actor: "m1+coordinator", Targets: []string{"fix-docs"}, Keep: -1},
			{At: "2026-08-23T00:01:00Z", Opid: opid, Verb: "approve", Actor: "human:Wido", Targets: []string{"fix-docs"}, Keep: -1},
		},
		Approved: &goal.ApprovalRecord{By: "human:Wido", At: "2026-08-23T00:01:00Z", Revision: 2, Opid: opid,
			Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 3, budget)}}
	root := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	for relative, data := range map[string][]byte{
		"metasystem.conf":         []byte("launch.seat.runtime=claude\n"),
		"plans/goals/backlog.md":  goal.RenderRoot(root),
		"plans/goals/fix-docs.md": goal.RenderFile(ready),
	} {
		path := filepath.Join(install, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seatTickGit(t, top, "init", "-q")
	seatTickGit(t, top, "config", "metasystem.goal.machine", "m1")
	seatTickGit(t, top, "config", "goal.sync-remote", "local")
	seatTickGit(t, top, "config", "metasystem.steward.notify-command", "true")
	seatTickGit(t, top, "add", "-A")
	seatTickGit(t, top, "commit", "-qm", "ledger")
	seatTickGit(t, top, "update-ref", goal.AcceptedRef, "HEAD")
	return top, install
}

// TestStewardTickStartsASeatLaunch: one tick over ready work with no seat
// selects the goal, and the runner's start makes a seat launch at the
// checkout's top, fenced by the installation, whose command carries the seat
// lineage; the steward's seat record names the launch and the goal.
func TestStewardTickStartsASeatLaunch(t *testing.T) {
	t.Parallel()
	top, install := seatTickCheckout(t)
	settings, err := launch.ResolveSettings(filepath.Join(install, "metasystem.conf"), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	processes := &seatTickProcesses{}
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()},
		Adapters:  map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude"}},
		Processes: processes, Prober: seatTickProber{}, Settings: settings,
		Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) }, Grace: time.Second, Poll: time.Second}
	manager.Supervisor = seatTickSupervisor{t: t, manager: manager}
	launcher := newStewardSeatLauncher()
	launcher.manager = func() *launch.Manager { return manager }
	// This host's landing lane is another checkout; the installation's own
	// metasystem.conf turned its seat on (Amendment 1).
	launcher.laneRoot = func() (string, bool, error) { return filepath.Join(t.TempDir(), "landing"), true, nil }
	workStateRoot := t.TempDir()
	config := steward.TickConfig{Now: now, Seat: launcher, WorkStateRoot: workStateRoot}

	// The same checkout registered as the host's landing lane starts no
	// seat: the tick keeps today's notification (Amendment 1).
	asLane := launcher
	asLane.laneRoot = func() (string, bool, error) { return top, true, nil }
	if held, err := steward.RunTick(install, steward.TickConfig{Now: now, Seat: asLane, WorkStateRoot: workStateRoot}, seatTickCensus{}); err != nil ||
		held.Seat != nil || held.Decision.Action != steward.ActNotify {
		t.Fatalf("the landing lane's tick starts no seat: %+v %+v %v", held.Decision, held.Seat, err)
	}

	result, err := steward.RunTick(install, config, seatTickCensus{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != steward.ActRevive || result.Seat == nil || result.Seat.Goal != "fix-docs" {
		t.Fatalf("ready work with no seat selects the goal: %+v %+v", result.Decision, result.Seat)
	}
	// The runner's pass starts the selected seat (runner.go, StartSeat),
	// reading the census again under the start's lock: a main that appeared
	// since the tick withdraws the start (SOL-A-03).
	if withdrawn, err := steward.StartSeat(install, config, seatTickLiveMain{}, *result.Seat); err == nil || withdrawn.LaunchID != "" {
		t.Fatalf("a seat start after a main appeared went ahead: %+v %v", withdrawn, err)
	}
	if records, _ := manager.Store.List(); len(records) != 0 || len(processes.commands) != 0 {
		t.Fatalf("a withdrawn seat start launched: %+v", records)
	}
	seat, err := steward.StartSeat(install, config, seatTickCensus{}, *result.Seat)
	if err != nil {
		t.Fatal(err)
	}
	record, err := manager.Store.Read(seat.LaunchID)
	if err != nil {
		t.Fatalf("no launch record for seat %s: %v", seat.LaunchID, err)
	}
	var fenceRoot string
	if raw, ok := record.AdapterData["fenceRoot"]; ok {
		fenceRoot = strings.Trim(string(raw), `"`)
	}
	if record.Kind != "seat" || record.Goal != "" || record.WorkingDirectory != top || fenceRoot != install {
		t.Fatalf("the seat launch is the seat kind at the checkout top %s fenced by %s: kind=%q goal=%q dir=%q fence=%q",
			top, install, record.Kind, record.Goal, record.WorkingDirectory, fenceRoot)
	}
	if len(processes.commands) != 1 {
		t.Fatalf("the supervisor started %d children", len(processes.commands))
	}
	command := processes.commands[0]
	if command.Directory != top || !slices.Contains(command.Environment, "METASYSTEM_OWNER_LINEAGE="+steward.SeatLineage) ||
		!slices.Contains(command.Environment, "METASYSTEM_DELEGATE_ROOT=") || !slices.Contains(command.Environment, "METASYSTEM_SESSION_ID=") {
		t.Fatalf("the seat runs at the checkout top under the seat lineage: dir=%q env=%q", command.Directory, command.Environment)
	}
	if seat.Goal != "fix-docs" || seat.Machine != "m1" || seat.ApprovalOpid == "" {
		t.Fatalf("the steward's seat record names the goal, the machine and the approval: %+v", seat)
	}
	if top, err := stateroot.RepositoryTop(install); err != nil || top != record.WorkingDirectory {
		t.Fatalf("the working directory is the Git top of the installation: %q %v", top, err)
	}
}
