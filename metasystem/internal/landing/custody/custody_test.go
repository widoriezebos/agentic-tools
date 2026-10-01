package custody

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

var custodyNow = time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)

func testHome(t *testing.T) string { return filepath.Join(t.TempDir(), "home") }

// deadRef is the exact identity of a process that ran and has ended.
func deadRef(t *testing.T) string {
	t.Helper()
	command := exec.Command("sleep", "120")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	_ = command.Process.Kill()
	_ = command.Wait()
	if err != nil || state != identity.Alive {
		t.Fatalf("probe: %v %v", state, err)
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// groupProbe is a group read the test decides.
func groupProbe(members map[int64]bool, unknown map[int64]bool) func(int64) (bool, error) {
	return func(group int64) (bool, error) {
		if unknown[group] {
			return false, errors.New("operation not permitted")
		}
		return members[group], nil
	}
}

// A kernel launch through Start: the record is written before the child
// starts, the child leads its own group, and the record holds its exact
// identity and group. While it runs it is live; once it has ended and its
// group is empty it settles, and the settlement is kept.
func TestStartRecordsChildAndGroupAndSettlesAfterItEnds(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	command := exec.Command("sleep", "120")
	record, err := Start(home, KindProve, "batch b1 subject batch", custodyNow, command)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	pid := int64(command.Process.Pid)
	if record.Child == "" || len(record.Groups) != 1 || record.Groups[0].ID != pid || record.Groups[0].Leader != record.Child {
		t.Fatalf("record = %+v; want the child and its own group %d", record, pid)
	}
	if group, err := syscall.Getpgid(int(pid)); err != nil || int64(group) != pid {
		t.Fatalf("the child's group = %d %v; want it to lead its own", group, err)
	}
	state, err := Probe(home, record.ID, Probes{})
	if err != nil || state.State != Live {
		t.Fatalf("running child = %+v %v; want live", state, err)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	state, err = Probe(home, record.ID, Probes{})
	if err != nil || state.State != Dead || state.Record.Settled == "" {
		t.Fatalf("ended child = %+v %v; want dead and settled", state, err)
	}
	// A settled record is never probed again: a reused group id can't make
	// it live.
	reused := Probes{Group: groupProbe(map[int64]bool{pid: true}, nil)}
	if state, _ := Probe(home, record.ID, reused); state.State != Dead {
		t.Fatalf("settled record with its group id reused = %+v; want dead", state)
	}
}

// A start that fails cancels the record: nothing runs, nothing holds.
func TestFailedStartCancelsTheRecord(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	command := exec.Command(filepath.Join(t.TempDir(), "no-such-program"))
	record, err := Start(home, KindValidate, "validation cadence-run-1", custodyNow, command)
	if err == nil {
		t.Fatal("a missing program started")
	}
	stored, ok, readErr := Read(home, record.ID)
	if readErr != nil || !ok || !stored.Cancelled || stored.Child != "" {
		t.Fatalf("record after a failed start = %+v %v %v; want cancelled", stored, ok, readErr)
	}
	if settlement, err := Settle(home, Probes{}); err != nil || !settlement.Settled(false) {
		t.Fatalf("settlement = %+v %v; want settled", settlement, err)
	}
}

// Judge reads the run's own processes, never its run record: each case is
// what the kernel reports.
func TestJudgeReadsProcessesNotRunRecords(t *testing.T) {
	t.Parallel()
	selfExact, _, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	self, _ := identity.EncodeRef(selfExact.Ref())
	gone := deadRef(t)
	cases := []struct {
		name   string
		record Record
		probes Probes
		want   string
	}{
		{"starting", Record{Issuer: self}, Probes{}, Live},
		{"launcher died before binding", Record{Issuer: gone}, Probes{}, Unknown},
		{"child alive", Record{Issuer: self, Child: self, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(nil, nil)}, Live},
		{"child ended, group empty", Record{Issuer: self, Child: gone, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(nil, nil)}, Dead},
		{"child ended, group still has members", Record{Issuer: self, Child: gone, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(map[int64]bool{9: true}, nil)}, Live},
		{"group membership unreadable", Record{Issuer: self, Child: gone, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(nil, map[int64]bool{9: true})}, Unknown},
		{"child identity garbled", Record{Issuer: self, Child: "not-a-ref"}, Probes{}, Unknown},
		{"in-process work running", Record{Issuer: self, Child: self, InProcess: true}, Probes{}, Live},
		{"in-process work ended", Record{Issuer: self, Child: self, InProcess: true, Ended: true, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(nil, nil)}, Dead},
		{"in-process work ended, its child group runs", Record{Issuer: self, Child: self, InProcess: true, Ended: true, Groups: []Group{{ID: 9}}}, Probes{Group: groupProbe(map[int64]bool{9: true}, nil)}, Live},
		{"cancelled", Record{Issuer: gone, Cancelled: true}, Probes{}, Dead},
	}
	for _, tc := range cases {
		if got := Judge(tc.record, tc.probes); got.State != tc.want {
			t.Errorf("%s: %s (%s), want %s", tc.name, got.State, got.Why, tc.want)
		}
	}
}

// Settle sums every record, the lane's proof leases and the proving lock:
// live and unknown each listed, settled records persisted.
func TestSettleListsLiveAndUnknownAcrossTheLane(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	live, err := Open(home, KindProve, "batch b1", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindSelf(home, live.ID); err != nil {
		t.Fatal(err)
	}
	probes := Probes{
		Leases: func() ([]Lease, error) {
			return []Lease{{Name: "lease-heavy-a", State: LeaseLive, Reason: "a live owner holds the lease"},
				{Name: "lease-heavy-b", State: LeaseUnknown, Reason: "custodian pid 7 liveness is unknown"},
				{Name: "lease-heavy-c", State: "reclaimable-dead"}}, nil
		},
		Proving: func() (string, bool, error) { return "pid 42", true, nil },
	}
	settlement, err := Settle(home, probes)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(settlement.Live, "\n") + "\n--\n" + strings.Join(settlement.Unknown, "\n")
	for _, want := range []string{"prove batch b1", "lease-heavy-a", "lease-heavy-b", "proving lock (pid 42)"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("settlement %q does not name %q", joined, want)
		}
	}
	if strings.Contains(joined, "lease-heavy-c") || settlement.Settled(true) {
		t.Fatalf("settlement = %+v", settlement)
	}
	if err := End(home, live.ID); err != nil {
		t.Fatal(err)
	}
	settlement, err = Settle(home, Probes{})
	if err != nil || !settlement.Settled(false) {
		t.Fatalf("after the in-process work ended = %+v %v", settlement, err)
	}
	if stored, _, _ := Read(home, live.ID); stored.Settled == "" {
		t.Fatalf("a proven settlement was not kept: %+v", stored)
	}
}

// An unreadable record is unknown custody, never nothing.
func TestUnreadableRecordIsUnknown(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	if err := os.MkdirAll(Dir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(home), "prove-torn.json"), []byte("{\"schema\":1,"), 0o600); err != nil {
		t.Fatal(err)
	}
	settlement, err := Settle(home, Probes{})
	if err != nil || len(settlement.Unknown) != 1 || settlement.Settled(false) || !settlement.Settled(true) {
		t.Fatalf("settlement = %+v %v; want one unknown", settlement, err)
	}
}

// A person's override records unknown custody settled in their name and
// never touches live custody.
func TestOverrideSettlesOnlyUnknownInThePersonsName(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	live, err := Open(home, KindProve, "batch b1", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	gone := deadRef(t)
	orphan := Record{Schema: schema, ID: "validate-orphan", Kind: KindValidate, Subject: "validation cadence-run-1", OpenedAt: custodyNow.Format(time.RFC3339Nano), Issuer: gone}
	if err := withLock(home, func() error { return write(home, orphan) }); err != nil {
		t.Fatal(err)
	}
	if _, err := Override(home, "", Probes{}); err == nil {
		t.Fatal("an unnamed override was taken")
	}
	passed, err := Override(home, "Wido", Probes{})
	if err != nil || len(passed) != 1 || passed[0] != orphan.ID {
		t.Fatalf("override passed %q %v; want only the orphan", passed, err)
	}
	stored, _, _ := Read(home, orphan.ID)
	if stored.Forced != "Wido" || stored.Settled == "" {
		t.Fatalf("overridden record = %+v", stored)
	}
	settlement, err := Settle(home, Probes{})
	if err != nil || len(settlement.Unknown) != 0 || len(settlement.Live) != 1 || !strings.Contains(settlement.Live[0], live.ID) {
		t.Fatalf("settlement after the override = %+v %v; want only the live one", settlement, err)
	}
}

// The barrier a new execution passes: live custody holds it, force or not;
// unknown custody holds it unless a person forced it.
func TestClearHoldsNewWorkWhileCustodyIsLiveOrUnknown(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	if err := Clear(home, Probes{}, false); err != nil {
		t.Fatalf("an empty store: %v", err)
	}
	unknown := Probes{Leases: func() ([]Lease, error) { return []Lease{{Name: "lease-heavy-x", State: LeaseUnknown}}, nil }}
	var held *Held
	if err := Clear(home, unknown, false); !errors.As(err, &held) || len(held.Unknown) != 1 {
		t.Fatalf("unknown lease: %v", err)
	}
	if err := Clear(home, unknown, true); err != nil {
		t.Fatalf("unknown lease, forced: %v", err)
	}
	live := Probes{Leases: func() ([]Lease, error) { return []Lease{{Name: "lease-heavy-y", State: LeaseLive}}, nil }}
	for _, force := range []bool{false, true} {
		if err := Clear(home, live, force); !errors.As(err, &held) || len(held.Live) != 1 {
			t.Fatalf("live lease, force %v: %v", force, err)
		}
	}
}

// A group bound after the start (a proof launcher's suite, led by the
// suite) holds custody after the work's own process has ended, while its
// leader runs; once the leader has ended and the group is empty it settles.
func TestBoundGroupHoldsAfterTheChildEnds(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	record, err := Open(home, KindVerify, "publish b1", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindGroup(home, record.ID, identity.Ref{Pid: 1}); err == nil {
		t.Fatal("process group 1 was bound")
	}
	if err := BindSelf(home, record.ID); err != nil {
		t.Fatal(err)
	}
	suite := exec.Command("sleep", "120")
	suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := suite.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = suite.Process.Kill(); _ = suite.Wait() })
	leader, _, err := (identity.KernelProber{}).Probe(int64(suite.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	if err := BindGroup(home, record.ID, leader.Ref()); err != nil {
		t.Fatal(err)
	}
	if err := End(home, record.ID); err != nil {
		t.Fatal(err)
	}
	if state, _ := Probe(home, record.ID, Probes{}); state.State != Live {
		t.Fatalf("ended in-process work whose bound group runs = %+v; want live", state)
	}
	_ = suite.Process.Kill()
	_ = suite.Wait()
	if state, _ := Probe(home, record.ID, Probes{}); state.State != Dead {
		t.Fatalf("ended in-process work with its bound group ended = %+v; want dead", state)
	}
}

// F-1: a group whose leader has ended while its pid now belongs to an
// unrelated process (after a crash or a reboot) is gone, although the
// reused id's own group has members: the record settles instead of
// holding the lane forever. Real processes: the "reused" pid is a live
// process leading its own group; the record names an earlier process with
// that pid.
func TestReusedGroupPidSettles(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	unrelated := exec.Command("sleep", "120")
	unrelated.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() })
	exact, _, err := (identity.KernelProber{}).Probe(int64(unrelated.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	earlier := exact.Ref()
	earlier.StartedAtSec -= 3600
	if earlier.StartedAtUnixMicro != 0 {
		earlier.StartedAtUnixMicro -= 3600 * 1_000_000
	}
	if earlier.StartTicks != 0 {
		earlier.StartTicks -= 360_000
	}
	record, err := Open(home, KindValidate, "validation cadence-run-1", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindChild(home, record.ID, earlier); err != nil {
		t.Fatal(err)
	}
	if members, err := GroupMembers(earlier.Pid); err != nil || !members {
		t.Fatalf("the reused id's group = %v %v; want members (the hazard)", members, err)
	}
	state, err := Probe(home, record.ID, Probes{})
	if err != nil || state.State != Dead {
		t.Fatalf("a record whose group pid was reused = %+v %v; want dead", state, err)
	}
	if settlement, err := Settle(home, Probes{}); err != nil || !settlement.Settled(false) {
		t.Fatalf("settlement = %+v %v", settlement, err)
	}
}

// A group whose leader has exited, and whose pid no process holds, is still
// live while other members run: the leader's exit is not the group's end.
// Real processes: the leader starts a member in its own group and exits.
func TestGroupWithExitedLeaderAndRunningMembersIsLive(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	leaderCommand := exec.Command("sh", "-c", `sleep 120 </dev/null >/dev/null 2>&1 & echo "$!"; read -r _ || :`)
	leaderCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := leaderCommand.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := leaderCommand.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := leaderCommand.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	// The leader waits on its input, so its identity is read while it runs.
	leader, state, err := (identity.KernelProber{}).Probe(int64(leaderCommand.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("leader probe = %v %v", state, err)
	}
	record, err := Open(home, KindProve, "batch b1 attempt a1 member:goal-a", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindChild(home, record.ID, leader.Ref()); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	if err := leaderCommand.Wait(); err != nil {
		t.Fatal(err)
	}
	group := int64(leaderCommand.Process.Pid)
	if _, liveness, err := (identity.KernelProber{}).Probe(group); err != nil || liveness == identity.Alive {
		t.Fatalf("the leader's pid %d = %v %v; want no process holding it", group, liveness, err)
	}
	if members, err := GroupMembers(group); err != nil || !members {
		t.Fatalf("the group = %v %v; want its member %d running in it", members, err, member)
	}
	state2, err := Probe(home, record.ID, Probes{})
	if err != nil || state2.State != Live || !strings.Contains(state2.Why, "still has members") {
		t.Fatalf("a group whose leader exited while a member runs = %+v %v; want live", state2, err)
	}
	if settlement, err := Settle(home, Probes{}); err != nil || settlement.Settled(false) || len(settlement.Live) != 1 {
		t.Fatalf("settlement = %+v %v; want the group live", settlement, err)
	}
}

// The environment a kernel launch carries binds a group to its record; work
// no kernel verb launched binds nothing; a half-set environment is an
// error.
func TestBindFromEnvironment(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	record, err := Open(home, KindProve, "batch b1", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	environ := Environment([]string{"PATH=/bin", EnvID + "=stale"}, home, record.ID)
	lookup := func(env []string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			for _, entry := range env {
				if key, value, _ := strings.Cut(entry, "="); key == name {
					return value, true
				}
			}
			return "", false
		}
	}
	if strings.Count(strings.Join(environ, "\n"), EnvID+"=") != 1 {
		t.Fatalf("environment = %q; want the stale id replaced", environ)
	}
	self, _, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	leader := self.Ref()
	if bound, err := BindFromEnvironment(lookup(environ), leader); !bound || err != nil {
		t.Fatalf("bind = %v %v", bound, err)
	}
	stored, _, _ := Read(home, record.ID)
	if len(stored.Groups) != 1 || stored.Groups[0].ID != leader.Pid || stored.Groups[0].Leader == "" {
		t.Fatalf("groups = %+v", stored.Groups)
	}
	if bound, err := BindFromEnvironment(lookup([]string{"PATH=/bin"}), leader); bound || err != nil {
		t.Fatalf("no custody environment: bound %v %v", bound, err)
	}
	if bound, err := BindFromEnvironment(lookup([]string{EnvID + "=x"}), leader); !bound || err == nil {
		t.Fatalf("half-set environment: bound %v %v", bound, err)
	}
}
