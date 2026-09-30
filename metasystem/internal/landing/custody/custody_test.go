package custody

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
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
	if record.Child == "" || len(record.Groups) != 1 || record.Groups[0] != pid {
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
		{"child alive", Record{Issuer: self, Child: self, Groups: []int64{9}}, Probes{Group: groupProbe(nil, nil)}, Live},
		{"child ended, group empty", Record{Issuer: self, Child: gone, Groups: []int64{9}}, Probes{Group: groupProbe(nil, nil)}, Dead},
		{"child ended, group still has members", Record{Issuer: self, Child: gone, Groups: []int64{9}}, Probes{Group: groupProbe(map[int64]bool{9: true}, nil)}, Live},
		{"group membership unreadable", Record{Issuer: self, Child: gone, Groups: []int64{9}}, Probes{Group: groupProbe(nil, map[int64]bool{9: true})}, Unknown},
		{"child identity garbled", Record{Issuer: self, Child: "not-a-ref"}, Probes{}, Unknown},
		{"in-process work running", Record{Issuer: self, Child: self, InProcess: true}, Probes{}, Live},
		{"in-process work ended", Record{Issuer: self, Child: self, InProcess: true, Ended: true, Groups: []int64{9}}, Probes{Group: groupProbe(nil, nil)}, Dead},
		{"in-process work ended, its child group runs", Record{Issuer: self, Child: self, InProcess: true, Ended: true, Groups: []int64{9}}, Probes{Group: groupProbe(map[int64]bool{9: true}, nil)}, Live},
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

// The lane's leases are those taken for its installation, plus any whose
// record can't be read; another installation's leases and an older
// engine's are not the lane's.
func TestLaneLeasesAreTheInstallationsAndTheUnreadable(t *testing.T) {
	t.Parallel()
	installation := filepath.Join(t.TempDir(), "lane", "metasystem")
	reports := []proofrun.HostLeaseReport{
		{Lease: "lease-heavy-lane", Owner: proofrun.ProcessIdentity{Pid: 5}, Conf: filepath.Join(installation, "metasystem.conf"), State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-seat", Owner: proofrun.ProcessIdentity{Pid: 6}, Conf: "/seat/metasystem/metasystem.conf", State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-older", Owner: proofrun.ProcessIdentity{Pid: 7}, State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-torn", State: proofrun.HostLeaseUnknown, Reason: "unreadable"},
	}
	leases, err := laneLeases(installation, func(string) ([]proofrun.HostLeaseReport, error) { return reports, nil })
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, lease := range leases {
		names = append(names, lease.Name)
	}
	if strings.Join(names, ",") != "lease-heavy-lane,lease-heavy-torn" {
		t.Fatalf("lane leases = %q", names)
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
