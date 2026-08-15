package run

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fakeProber struct {
	verdicts map[int64]identity.Liveness
	starts   map[int64]int64
}

func (f fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	verdict, ok := f.verdicts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	switch verdict {
	case identity.Alive:
		return identity.Exact{Pid: pid, StartedAt: time.Unix(f.starts[pid], 0)}, identity.Alive, nil
	case identity.Unknown:
		return identity.Exact{}, identity.Unknown, errors.New("procfs unreadable")
	default:
		return identity.Exact{}, identity.Dead, nil
	}
}

var mainCaller = Caller{Class: "MAIN", MainId: "main-test-1", OwnerLineage: "main-test-1", SessionId: "s1"}

func testStore(t *testing.T) *Store {
	t.Helper()
	base := time.Unix(1786900000, 0)
	return &Store{Root: t.TempDir(), Now: func() time.Time { return base }}
}

func launchOne(t *testing.T, s *Store, id string) string {
	t.Helper()
	nonce, err := s.Launch(mainCaller, LaunchParams{
		Id: id, Kind: "suite", Display: "the " + id + " suite",
		Log: "artifacts/" + id + ".log", Expect: Expect{Green: "ship it", Red: "read the log", Hung: "probe it", Unknown: "investigate"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return nonce
}

// MON-01: pending-before-process; a live id refuses reuse; the fence
// concludes a stale pending as launch-failed with a note, never deletion.
func TestLaunchReservationAndFence(t *testing.T) {
	s := testStore(t)
	nonce := launchOne(t, s, "suite-a")
	if !nonceRe.MatchString(nonce) {
		t.Fatalf("nonce not 32 hex: %q", nonce)
	}
	record, _ := s.Read("suite-a")
	if record.Status != StatusLaunching || record.Pid != nil {
		t.Fatalf("pending record wrong: %+v", record)
	}
	if _, err := s.Launch(mainCaller, LaunchParams{Id: "suite-a", Kind: "suite", Log: "x.log"}); err == nil {
		t.Fatal("live id reused")
	}

	// Before the fence: no transition.
	if result, err := s.Assess("suite-a"); err != nil || result.Transitioned {
		t.Fatalf("fence fired early: %+v %v", result, err)
	}
	// Past the fence: launch-failed, record retained.
	s.Now = func() time.Time { return time.Unix(1786900000, 0).Add(3 * time.Minute) }
	result, err := s.Assess("suite-a")
	if err != nil || !result.Transitioned || result.To != StatusLaunchFailed {
		t.Fatalf("fence did not conclude: %+v %v", result, err)
	}
	record, _ = s.Read("suite-a")
	if record == nil || record.Error == nil || record.TerminalSeq == nil {
		t.Fatalf("launch-failed record wrong: %+v", record)
	}
	// A terminal id is reusable — with a fresh lifecycle.
	if _, err := s.Launch(mainCaller, LaunchParams{Id: "suite-a", Kind: "suite", Log: "y.log"}); err != nil {
		t.Fatalf("terminal id not reusable: %v", err)
	}
}

// MON-02 + MON-06: the evidence table — sidecar green/red, forged and
// stale-generation sidecars ignored, dead-no-evidence=ended-unknown,
// Unknown concludes nothing; conclusion CAS fences on generation.
func TestConcludeEvidenceTable(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober

	bind := func(id string, pid int64) *Record {
		nonce := launchOne(t, s, id)
		prober.verdicts[pid] = identity.Alive
		prober.starts[pid] = 5000
		if err := s.Bind(id, nonce, pid, pid); err != nil {
			t.Fatal(err)
		}
		record, _ := s.Read(id)
		return record
	}

	// Green: matching sidecar, exit 0. The fake group is empty (no
	// process shares the pgid), so terminalization is direct.
	record := bind("green-run", 101)
	if err := s.WriteSidecar("green-run", record.Generation, record.LaunchNonce, 0); err != nil {
		t.Fatal(err)
	}
	prober.verdicts[101] = identity.Dead
	result, err := s.Assess("green-run")
	if err != nil || result.To != StatusGreen {
		t.Fatalf("green conclusion failed: %+v %v", result, err)
	}

	// Red: nonzero exit (a signal death shape).
	record = bind("red-run", 102)
	s.WriteSidecar("red-run", record.Generation, record.LaunchNonce, 137)
	prober.verdicts[102] = identity.Dead
	result, _ = s.Assess("red-run")
	record, _ = s.Read("red-run")
	if result.To != StatusRed || record.ExitCode == nil || *record.ExitCode != 137 {
		t.Fatalf("red conclusion wrong: %+v %+v", result, record)
	}

	// Forged sidecar (wrong nonce): ended-unknown, never believed.
	record = bind("forged-run", 103)
	s.WriteSidecar("forged-run", record.Generation, strings.Repeat("f", 32), 0)
	prober.verdicts[103] = identity.Dead
	result, _ = s.Assess("forged-run")
	if result.To != StatusEndedUnknown {
		t.Fatalf("forged sidecar believed: %+v", result)
	}

	// Unknown identity: concludes NOTHING, surfaces.
	record = bind("unknown-run", 104)
	prober.verdicts[104] = identity.Unknown
	result, _ = s.Assess("unknown-run")
	if result.Transitioned || len(result.Unreadable) == 0 {
		t.Fatalf("unknown concluded or stayed silent: %+v", result)
	}

	// Generation fencing: a stale-generation sidecar cannot conclude an
	// adopted successor. Adopt requires old dead+empty first.
	record = bind("adopted-run", 105)
	oldNonce := record.LaunchNonce
	prober.verdicts[105] = identity.Dead
	prober.verdicts[106] = identity.Alive
	prober.starts[106] = 6000
	// Adopt binds to OUR process group? Adopt probes pid 106 via the fake
	// prober but reads the real pgid — use our own pid's group so
	// Getpgid succeeds.
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 7000
	// The old group must be provably empty: pgid 105 has no live member
	// in the real process table (guaranteed — 105 is not a live pgid we
	// created), so adoption may proceed.
	if err := s.Adopt(mainCaller, "adopted-run", self); err != nil {
		t.Fatalf("adopt refused: %v", err)
	}
	record, _ = s.Read("adopted-run")
	if record.Generation != 2 || record.LaunchNonce == oldNonce {
		t.Fatalf("adoption did not mint a generation: %+v", record)
	}
	// The generation-1 sidecar with the OLD nonce lands: it must be
	// ignored for the g2 record.
	s.WriteSidecar("adopted-run", 1, oldNonce, 0)
	prober.verdicts[self] = identity.Dead
	result, _ = s.Assess("adopted-run")
	if result.To == StatusGreen {
		t.Fatalf("a stale-generation sidecar concluded the successor: %+v", result)
	}
}

// Draining: a dead leader with survivors drains with a frozen provisional
// verdict; the wind-down clock runs from endedAt; finalization keeps the
// frozen verdict (MON-02's freeze + MON-03's window).
func TestDrainingFreezesVerdict(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober

	// A real process group with a survivor: sleep in its own group via
	// our own subprocess (its pgid is our pid group... simpler: use a
	// real setsid sleep whose group has a live member — the sleep
	// itself. The leader identity we RECORD is a fake dead pid, but the
	// pgid we record is the sleep's live group).
	sleeper := exec.Command("/bin/sleep", "30")
	sleeper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	defer sleeper.Process.Kill()
	nonce := launchOne(t, s, "drain-run")
	prober.verdicts[201] = identity.Alive
	prober.starts[201] = 5000
	if err := s.Bind("drain-run", nonce, 201, 201); err != nil {
		t.Fatal(err)
	}
	// Rebind the recorded pgid to the sleeper's REAL group so the group
	// is non-empty (test seam: direct record surgery under the store's
	// own writer would violate CAS; use Adopt? Adopt changes generation.
	// Simplest: kill the leader and point the group at the sleeper's).
	record, _ := s.Read("drain-run")
	pg := int64(sleeper.Process.Pid)
	record.Pgid = &pg
	if err := s.write(record); err != nil {
		t.Fatal(err)
	}
	s.WriteSidecar("drain-run", record.Generation, record.LaunchNonce, 0)
	prober.verdicts[201] = identity.Dead

	result, err := s.Assess("drain-run")
	if err != nil || result.To != StatusDraining {
		t.Fatalf("dead leader with survivors did not drain: %+v %v", result, err)
	}
	record, _ = s.Read("drain-run")
	if record.ProvisionalVerdict == nil || *record.ProvisionalVerdict != StatusGreen || record.EndedAt == nil {
		t.Fatalf("provisional verdict not frozen at entry: %+v", record)
	}

	// Before the wind-down with a live survivor: stays draining.
	if result, _ := s.Assess("drain-run"); result.Transitioned {
		t.Fatalf("draining finalized early: %+v", result)
	}
	// Past the wind-down: finalizes to the FROZEN verdict.
	s.Now = func() time.Time { return time.Unix(1786900000, 0).Add(11 * time.Minute) }
	result, _ = s.Assess("drain-run")
	if result.To != StatusGreen {
		t.Fatalf("draining did not finalize the frozen verdict: %+v", result)
	}
}

// MON-10: prune drops only acked terminal records past the age, with
// their sidecars, reporting drops.
func TestPruneReportsDrops(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{301: identity.Alive}, starts: map[int64]int64{301: 5000}}
	s.Prober = prober
	nonce := launchOne(t, s, "old-run")
	s.Bind("old-run", nonce, 301, 301)
	record, _ := s.Read("old-run")
	s.WriteSidecar("old-run", record.Generation, record.LaunchNonce, 0)
	prober.verdicts[301] = identity.Dead
	s.Assess("old-run")
	if err := s.Ack(mainCaller, "old-run"); err != nil {
		t.Fatal(err)
	}

	// Too young: kept.
	dropped, err := s.Prune(mainCaller)
	if err != nil || len(dropped) != 0 {
		t.Fatalf("young record pruned: %v %v", dropped, err)
	}
	// Past the age: dropped with its sidecar.
	s.Now = func() time.Time { return time.Unix(1786900000, 0).AddDate(0, 0, 15) }
	dropped, err = s.Prune(mainCaller)
	if err != nil || len(dropped) != 1 || !strings.Contains(dropped[0], "old-run") {
		t.Fatalf("prune wrong: %v %v", dropped, err)
	}
	if _, err := os.Stat(SidecarPath(s.Root, "old-run", 1)); !os.IsNotExist(err) {
		t.Fatal("sidecar survived the prune")
	}
}

// MON-11: the bounds refuse at the source.
func TestBounds(t *testing.T) {
	s := testStore(t)
	cases := []LaunchParams{
		{Id: "Bad_Id", Kind: "suite", Log: "x.log"},
		{Id: "ok", Kind: "party", Log: "x.log"},
		{Id: "ok", Kind: "suite", Log: "/etc/x.log"},
		{Id: "ok", Kind: "suite", Log: "x.log", Display: strings.Repeat("d", 201)},
		{Id: "ok", Kind: "suite", Log: "x.log", StaleAfterMin: 2000},
		{Id: "ok", Kind: "suite", Log: "x.log", WindDownMin: 300},
		{Id: "ok", Kind: "suite", Log: "x.log", Expect: Expect{Green: strings.Repeat("g", 241)}},
	}
	for i, p := range cases {
		if _, err := s.Launch(mainCaller, p); err == nil {
			t.Errorf("case %d passed: %+v", i, p)
		}
	}
}

// MON-08: HUMAN callers carry nullable coordinates.
func TestHumanCoordinatesNullable(t *testing.T) {
	s := testStore(t)
	human := Caller{Class: "HUMAN", SessionId: "h1"}
	if _, err := s.Launch(human, LaunchParams{Id: "human-run", Kind: "custom", Log: "h.log"}); err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("human-run")
	if record.MainId != nil || record.ClaimEpoch != nil {
		t.Fatalf("human coordinates not nullable: %+v", record)
	}
}
