package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// seatLaunchLineage is the owner lineage a seat launch's child environment
// names, read from the launch's own environment so the bed runs with D-seat's
// actual value.
func seatLaunchLineage(t *testing.T) string {
	t.Helper()
	for _, entry := range launch.SeatEnvironment() {
		if name, value, found := strings.Cut(entry, "="); found && name == "METASYSTEM_OWNER_LINEAGE" && value != "" {
			return value
		}
	}
	t.Fatal("the seat launch names no owner lineage")
	return ""
}

// seatGoalBed is a goal CLI bed with one approved goal and no other claim on
// the machine, the state a steward starts a seat in.
func seatGoalBed(t *testing.T) *goalCLIBed {
	t.Helper()
	bed := newGoalCLIBed(t, goalCLISeed{amend: func(goals map[string]*goal.GoalFile) {
		goals["ship-widget"] = nil
		goals["fix-docs"].Tier = 1
	}})
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "fix-docs"}, gcliLedgerHuman...)...)
	return bed
}

// seatStandIn starts a live process that stands for a seat main and
// announces it under lineage in root; the caller is the seat's own process.
func seatStandIn(t *testing.T, root, session, lineage string) (*testutil.HeldProcess, ownercall.Process) {
	t.Helper()
	held := testutil.StartHeldProcess(t, exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :"))
	t.Cleanup(func() { _ = held.Kill() })
	exact, state, err := (identity.KernelProber{}).Probe(int64(held.Command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the seat stand-in: %v %s", err, state)
	}
	if _, err := lease.AnnounceWithPair(root, session, exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"metasystem-main-claude-"+session, "claude", lineage); err != nil {
		t.Fatal(err)
	}
	return held, ownercall.Process{Pid: exact.Pid, StartedAt: exact.StartedAt.Unix()}
}

func seatLeaseEpoch(t *testing.T, root string) (int64, string) {
	t.Helper()
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}
	return holder.ClaimEpoch, holder.OwnerLineage
}

func seatParsedGoal(t *testing.T, bed *goalCLIBed, id string) *goal.GoalFile {
	t.Helper()
	file, problems := goal.ParseFile([]byte(bed.goalRecord(id)))
	if file == nil || len(problems) != 0 {
		t.Fatalf("goal %s does not parse: %v", id, problems)
	}
	return file
}

// TestStewardStartedMainClaimsAndBuildsAsHolder (authority, SW-13; fixture
// obligation): from the announced main's process, with the lineage the seat
// launch's environment names, the public goal claim claims under that
// lineage with the claim epoch equal to the lease epoch, and the goal-branch
// holder check passes for this process; the same command with the variable
// unset falls to the person's proof and, with no terminal, is refused.
func TestStewardStartedMainClaimsAndBuildsAsHolder(t *testing.T) {
	t.Parallel()
	bed := seatGoalBed(t)
	bed.lineage = seatLaunchLineage(t)
	bed.announceHolder()
	epoch, holderLineage := seatLeaseEpoch(t, bed.root)
	if holderLineage != "steward-seat" || epoch != 1 {
		t.Fatalf("the announced main does not hold the lease under the seat lineage: epoch=%d lineage=%q", epoch, holderLineage)
	}

	// The same command with the variable unset: a person's act, which needs
	// the proof of a terminal a person typed at; a headless seat has none.
	proved := false
	bed.lineage, bed.prove = "", func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		proved = true
		return humanauthority.Proof{Outcome: humanauthority.OutcomeTerminalMissing}, errors.New("no terminal a person typed at")
	}
	before := bed.tip()
	if code, out, errOut := bed.public("goal", "claim", "fix-docs"); code == 0 || !proved {
		t.Fatalf("goal claim without the seat lineage = code %d proved=%t out=%q err=%q", code, proved, out, errOut)
	}
	if bed.tip() != before {
		t.Fatal("the refused claim moved the ledger")
	}
	bed.lineage, bed.prove = seatLaunchLineage(t), fixedFixtureGoalAuthority

	gcliLedgerMust(t, bed, "goal", "claim", "fix-docs")
	file := seatParsedGoal(t, bed, "fix-docs")
	if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != "fixture-machine" || file.Claimed.Lineage != "steward-seat" ||
		file.StopCapability == nil || file.StopCapability.ClaimEpoch != epoch {
		t.Fatalf("the seat's claim = %+v capability=%+v, want lineage steward-seat at epoch %d", file.Claimed, file.StopCapability, epoch)
	}
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	check := goalBranchClaimCheckWith(bed.root, "fix-docs", endpoint, func(string, string) (string, error) { return "fixture-machine", nil },
		func(root string) string { return root })
	if err := check(); err != nil {
		t.Fatalf("the goal-branch holder check refused the seat main: %v", err)
	}
}

// TestSuccessorSeatInheritsADeadPredecessorsClaim (SW-14; the goal CLI leg
// of section 3's test 11, the up leg is internal/up's): seat one claims the
// goal and dies; the successor under the same lineage succeeds the lease
// with the epoch preserved and no takeover, its goal claim answers "already
// claimed by this session", and the goal-branch holder check passes. A
// person's session under another lineage takes the lease over at the epoch
// plus one, and the claim stays foreign until the person's goal claim
// --take-over.
func TestSuccessorSeatInheritsADeadPredecessorsClaim(t *testing.T) {
	t.Parallel()
	t.Run("the successor seat continues", func(t *testing.T) {
		t.Parallel()
		bed := seatGoalBed(t)
		bed.lineage = seatLaunchLineage(t)
		one, caller := seatStandIn(t, bed.root, "seat-one", bed.lineage)
		bed.caller = caller
		gcliLedgerMust(t, bed, "goal", "claim", "fix-docs")
		epoch, _ := seatLeaseEpoch(t, bed.root)
		_ = one.Kill()

		bed.announceHolder()
		next, lineage := seatLeaseEpoch(t, bed.root)
		current := seatLease(t, bed.root)
		if next != epoch || lineage != "steward-seat" || len(current.Takeovers) != 0 {
			t.Fatalf("the successor did not succeed the lease: epoch %d -> %d lineage=%q takeovers=%+v", epoch, next, lineage, current.Takeovers)
		}
		out := gcliLedgerMust(t, bed, "goal", "claim", "fix-docs")
		if !strings.Contains(out, "already claimed by this session") {
			t.Fatalf("the successor's claim did not continue the held goal: %q", out)
		}
		file := seatParsedGoal(t, bed, "fix-docs")
		if file.Claimed == nil || file.Claimed.Lineage != "steward-seat" || file.StopCapability == nil || file.StopCapability.ClaimEpoch != next {
			t.Fatalf("the held claim after succession = %+v capability=%+v", file.Claimed, file.StopCapability)
		}
		endpoint, err := bed.endpoint(bed.root)
		if err != nil {
			t.Fatal(err)
		}
		if err := goalBranchClaimCheckWith(bed.root, "fix-docs", endpoint, func(string, string) (string, error) { return "fixture-machine", nil },
			func(root string) string { return root })(); err != nil {
			t.Fatalf("the goal-branch holder check refused the successor: %v", err)
		}
	})
	t.Run("a person takes the seat back", func(t *testing.T) {
		t.Parallel()
		bed := seatGoalBed(t)
		bed.lineage = seatLaunchLineage(t)
		one, caller := seatStandIn(t, bed.root, "seat-one", bed.lineage)
		bed.caller = caller
		gcliLedgerMust(t, bed, "goal", "claim", "fix-docs")
		epoch, _ := seatLeaseEpoch(t, bed.root)
		_ = one.Kill()

		bed.lineage = "person-terminal"
		bed.announceHolder()
		next, lineage := seatLeaseEpoch(t, bed.root)
		if next != epoch+1 || lineage != "person-terminal" || len(seatLease(t, bed.root).Takeovers) != 1 {
			t.Fatalf("the person's session did not take the lease over: epoch %d -> %d lineage=%q", epoch, next, lineage)
		}
		gcliLedgerRefused(t, bed, "already claimed by another session on this machine (steward-seat)", "goal", "claim", "fix-docs")
		gcliLedgerMust(t, bed, "goal", "claim", "fix-docs", "--take-over", "--reason", "the seat was stopped", "--by", "Wido")
		file := seatParsedGoal(t, bed, "fix-docs")
		if file.Claimed == nil || file.Claimed.Lineage == "steward-seat" {
			t.Fatalf("the take-over left the seat's claim: %+v", file.Claimed)
		}
	})
}

func seatLease(t *testing.T, root string) lease.Lease {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value lease.Lease
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// TestWorkStopEndsTheSeatAndTheTickSeesNoMain (coexistence): a running seat
// launch whose child is the announced seat main in the checkout; work stop
// cancels the launch and ends its process group, and the census the next
// tick reads counts no live seat main.
func TestWorkStopEndsTheSeatAndTheTickSeesNoMain(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	checkout := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(checkout); err == nil {
		checkout = resolved
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(checkout, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(checkout, "bin", "metasystem"), []byte("engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The seat main: argv0 claude, in the checkout, leading its own group,
	// as the launch supervisor starts the runtime.
	command := exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :")
	command.Args[0] = "claude"
	command.Dir = checkout
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	held := testutil.StartHeldProcess(t, command)
	exact, state, err := (identity.KernelProber{}).Probe(int64(held.Command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the seat main: %v %s", err, state)
	}
	if _, err := lease.AnnounceWithPair(checkout, "seat-main", exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"metasystem-main-claude-seat-main", "claude", seatLaunchLineage(t)); err != nil {
		t.Fatal(err)
	}
	child := exact.Ref()
	store := launch.Store{Root: b.launchDir}
	if err := store.Create(launch.Record{ID: "seat-1", Kind: "seat", Tag: "n0nce", WorkingDirectory: checkout, State: launch.Running,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Child: &child, ProcessGroup: &child}); err != nil {
		t.Fatal(err)
	}
	// The seat main is this test's child, joined as soon as it exits; a
	// grace after a signal waits for that join, so the group is provably
	// gone exactly when a signal ended it.
	joined := make(chan struct{})
	go func() { _ = held.Command.Wait(); close(joined) }()
	t.Cleanup(func() {
		select {
		case <-joined:
		default:
			_ = held.Command.Process.Kill()
			<-joined
		}
	})
	processes := &seatSignalRecorder{OSProcesses: launch.OSProcesses{Prober: identity.KernelProber{}}}
	manager := &launch.Manager{Store: store, Processes: processes, Prober: identity.KernelProber{}, Now: time.Now,
		Sleep: func(time.Duration) {
			if processes.signalled {
				<-joined
			}
		}}
	owners := b.owners()
	owners.processes.launches = func() *launch.Manager { return manager }

	census := steward.RuntimeWorkerCensus{MetasystemRoot: checkout}
	before, err := census.Workers(checkout)
	if err != nil || before.LiveSeatMains == 0 {
		t.Fatalf("the census before the stop = %+v err=%v, want the live seat main", before, err)
	}
	code, result := b.runJSON(owners, "work", "stop", "seat-1")
	if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "launch seat-1 cancelled: cancelled") {
		t.Fatalf("work stop seat-1 = code %d %+v", code, result)
	}
	if alive, err := processes.GroupAlive(exact.Pid); err != nil || alive || !processes.signalled {
		t.Fatalf("the seat's process group survived the stop: alive=%t err=%v", alive, err)
	}
	after, err := census.Workers(checkout)
	if err != nil || after.LiveSeatMains != 0 {
		t.Fatalf("the census after the stop = %+v err=%v, want no live seat main", after, err)
	}
}

// seatSignalRecorder is the launch's process system, noting that the
// cancellation signalled the seat's group.
type seatSignalRecorder struct {
	launch.OSProcesses
	signalled bool
}

func (p *seatSignalRecorder) SignalGroup(pgid int64, signal syscall.Signal) error {
	p.signalled = true
	return p.OSProcesses.SignalGroup(pgid, signal)
}
