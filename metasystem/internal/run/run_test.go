package run

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
	return &Store{
		Root: t.TempDir(),
		Now:  func() time.Time { return base },
		// Synthetic pids are their own group leaders unless a test
		// overrides with the real kernel.
		Getpgid: func(pid int64) (int64, error) { return pid, nil },
		AllPids: func() ([]int64, error) { return []int64{}, nil },
	}
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

// Pending-before-process; a live id refuses reuse; the fence
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

// The evidence table — sidecar green/red, forged and
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
	bind("unknown-run", 104)
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
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 7000
	// The old synthetic group is empty in the synthetic process table,
	// so adoption may proceed.
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

// TestAssessUsesSyntheticProcessTable proves a synthetic member drains the run while an empty table concludes it.
func TestAssessUsesSyntheticProcessTable(t *testing.T) {
	assess := func(t *testing.T, members []int64) AssessResult {
		t.Helper()
		s := testStore(t)
		s.AllPids = func() ([]int64, error) { return members, nil }
		s.GroupPresent = func(int64) (bool, bool) { return false, false }
		prober := fakeProber{
			verdicts: map[int64]identity.Liveness{701: identity.Alive},
			starts:   map[int64]int64{701: 5000},
		}
		s.Prober = prober
		nonce := launchOne(t, s, "table-run")
		if err := s.Bind("table-run", nonce, 701, 701); err != nil {
			t.Fatal(err)
		}
		prober.verdicts[701] = identity.Dead
		result, err := s.Assess("table-run")
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	if result := assess(t, []int64{701}); result.To != StatusDraining {
		t.Fatalf("synthetic group member did not drain: %+v", result)
	}
	if result := assess(t, []int64{}); result.To != StatusEndedUnknown {
		t.Fatalf("empty synthetic table did not conclude: %+v", result)
	}
}

// Draining: a dead leader with survivors drains with a frozen provisional
// verdict; the wind-down clock runs from endedAt; finalization keeps the
// frozen verdict.
func TestDrainingFreezesVerdict(t *testing.T) {
	s := testStore(t)
	// This test creates a real process group and must scan the real table.
	// The group reader stays synthetic through Bind because Bind checks the
	// synthetic pid's group; no group-emptiness question is asked before rebind.
	s.AllPids = nil
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
	s.Getpgid = nil

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

// Prune drops only acked terminal records past the age, with
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

// The bounds refuse at the source.
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

// HUMAN callers carry nullable coordinates.
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

// The remaining verb refusals and helpers: ack rules, sidecar filename
// exclusion from the record glob, owner digests, lifecycle tags.
func TestVerbRefusalsAndHelpers(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{401: identity.Alive}, starts: map[int64]int64{401: 5000}}
	s.Prober = prober
	nonce := launchOne(t, s, "helper-run")
	if err := s.Ack(mainCaller, "helper-run"); err == nil {
		t.Fatal("ack of a non-terminal record passed")
	}
	if err := s.Bind("helper-run", strings.Repeat("0", 32), 401, 401); err == nil {
		t.Fatal("a wrong nonce bound")
	}
	if err := s.Bind("helper-run", nonce, 401, 401); err != nil {
		t.Fatal(err)
	}
	if err := s.Adopt(mainCaller, "helper-run", 401); err == nil {
		t.Fatal("adopt passed with a live old leader")
	}
	record, _ := s.Read("helper-run")
	s.WriteSidecar("helper-run", record.Generation, record.LaunchNonce, 3)
	prober.verdicts[401] = identity.Dead
	s.Assess("helper-run")
	if err := s.Ack(mainCaller, "helper-run"); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(mainCaller, "helper-run"); err == nil {
		t.Fatal("double ack passed")
	}

	// The record glob never ingests a sidecar.
	files := RecordFiles(s.Root)
	for _, f := range files {
		if strings.Contains(f, ".exit.json") {
			t.Fatalf("sidecar in the record glob: %s", f)
		}
	}
	if len(files) != 1 {
		t.Fatalf("record glob wrong: %v", files)
	}

	if OwnerDigest("main-1") == OwnerDigest("") {
		t.Fatal("human and main owner digests collide")
	}
	if got := record.LifecycleTag(); !strings.HasPrefix(got, "run:helper-run.g1.") {
		t.Fatalf("lifecycle tag wrong: %s", got)
	}

	// Assess on a missing record errors; on a terminal record it is a
	// no-op.
	if _, err := s.Assess("ghost"); err == nil {
		t.Fatal("assess of a missing record passed")
	}
	if result, err := s.Assess("helper-run"); err != nil || result.Transitioned {
		t.Fatalf("terminal assess moved: %+v %v", result, err)
	}
}

// The hung flag rides log mtime; Register adopts an already-running
// process with pattern evidence; the pattern path concludes green on
// match and ended-unknown otherwise.
func TestHungFlagAndRegisterPattern(t *testing.T) {
	s := testStore(t)
	// This test registers real processes and must scan their real groups.
	s.AllPids, s.Getpgid = nil, nil
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober

	// Register our own live process (kinship with ourselves holds).
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 8000
	logPath := filepath.Join(s.Root, "reg.log")
	os.WriteFile(logPath, []byte("working\n"), 0o644)
	// Pin the log's mtime to the test's fake epoch: the hung check
	// compares REAL mtime against the injected clock, so leaving
	// the real creation time in place is a time bomb that detonates
	// the moment the wall clock passes the hardcoded epoch — which
	// it did, hours after this test was written.
	base := time.Unix(1786900000, 0)
	if err := os.Chtimes(logPath, base, base); err != nil {
		t.Fatalf("the clock pin must hold or the time bomb is back: %v", err)
	}
	err := s.Register(mainCaller, LaunchParams{
		Id: "reg-run", Kind: "custom", Display: "registered work",
		Log: logPath, StaleAfterMin: 1,
	}, self, "(?m)^ALL GREEN$")
	if err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("reg-run")
	if record.Custody != CustodyAdoptedVerified || record.Evidence.Mode != EvidencePattern {
		t.Fatalf("register custody/evidence wrong: %+v", record)
	}

	// Quiet log past staleAfterMin: hung flag sets; activity clears it.
	s.Now = func() time.Time { return time.Unix(1786900000, 0).Add(2 * time.Minute) }
	if _, err := s.Assess("reg-run"); err != nil {
		t.Fatal(err)
	}
	record, _ = s.Read("reg-run")
	if record.HungSince == nil {
		t.Fatal("quiet log did not set the hung flag")
	}
	now := time.Unix(1786900000, 0).Add(3 * time.Minute)
	os.Chtimes(logPath, now, now)
	s.Now = func() time.Time { return now }
	s.Assess("reg-run")
	record, _ = s.Read("reg-run")
	if record.HungSince != nil {
		t.Fatal("log activity did not clear the hung flag")
	}

	// Death with a matching pattern: green. (Group is our own — alive —
	// so this drains first; write the match, kill the identity, and walk
	// the drain to expiry.)
	os.WriteFile(logPath, []byte("done\nALL GREEN\n"), 0o644)
	prober.verdicts[self] = identity.Dead
	result, err := s.Assess("reg-run")
	if err != nil {
		t.Fatal(err)
	}
	if result.To == StatusDraining {
		s.Now = func() time.Time { return now.Add(time.Duration(record.WindDownMin+1) * time.Minute) }
		result, _ = s.Assess("reg-run")
	}
	if result.To != StatusGreen {
		t.Fatalf("pattern match did not conclude green: %+v", result)
	}

	// A second registered run with a NON-matching pattern: ended-unknown,
	// never a red guess. The leader must be a REAL process (Register
	// reads the kernel pgid), so spawn a sleeper in its own group — the
	// Mac's coincidental pid 999 hid this until the Linux gate refused.
	sleeper := exec.Command("/bin/sleep", "30")
	sleeper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	sleeperPid := int64(sleeper.Process.Pid)
	prober.verdicts[sleeperPid] = identity.Alive
	prober.starts[sleeperPid] = 9000
	os.WriteFile(filepath.Join(s.Root, "nm.log"), []byte("no verdict here\n"), 0o644)
	if err := s.Register(mainCaller, LaunchParams{Id: "nomatch-run", Kind: "custom", Log: filepath.Join(s.Root, "nm.log")}, sleeperPid, "NEVER$"); err != nil {
		t.Fatal(err)
	}
	sleeper.Process.Kill()
	sleeper.Wait()
	prober.verdicts[sleeperPid] = identity.Dead
	result, _ = s.Assess("nomatch-run")
	if result.To == StatusDraining {
		s.Now = func() time.Time { return now.Add(200 * time.Minute) }
		result, _ = s.Assess("nomatch-run")
	}
	if result.To != StatusEndedUnknown {
		t.Fatalf("pattern no-match guessed: %+v", result)
	}
}

func TestGroupEmptyAcceptsTheKernelsAbsentGroupProofBeforeEnumeration(t *testing.T) {
	s := testStore(t)
	s.GroupPresent = func(int64) (bool, bool) { return false, true }
	s.AllPids = func() ([]int64, error) { return nil, errors.New("process table denied") }
	if !s.groupEmpty(424242) {
		t.Fatal("an absent process group still depended on full process enumeration")
	}
	s.GroupPresent = func(int64) (bool, bool) { return true, true }
	if s.groupEmpty(424242) {
		t.Fatal("a present group was called empty when membership was unreadable")
	}
	s.GroupPresent = func(int64) (bool, bool) { return false, false }
	if s.groupEmpty(424242) {
		t.Fatal("an unknown group was called empty when membership was unreadable")
	}
}

// The watch round-trip: owner-keyed exclusive
// waiters, stale-lifecycle replacement, human keys, pinned exit codes.
func TestWaiterContract(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 8000

	target := WaiterTarget{Generation: 1, LaunchNonce: strings.Repeat("a", 32)}
	if err := s.RegisterWaiter("run", "w-run", mainCaller, target); err != nil {
		t.Fatal(err)
	}
	// Same owner, same lifecycle, still alive: busy (64).
	err := s.RegisterWaiter("run", "w-run", mainCaller, target)
	if WaiterExitCode(err) != ExitWaiterBusy {
		t.Fatalf("live same-lifecycle waiter not busy: %v", err)
	}
	// Same owner, NEW lifecycle: the stale live waiter is replaceable.
	fresh := WaiterTarget{Generation: 2, LaunchNonce: strings.Repeat("b", 32)}
	if err := s.RegisterWaiter("run", "w-run", mainCaller, fresh); err != nil {
		t.Fatalf("stale-lifecycle waiter blocked replacement: %v", err)
	}
	// A FOREIGN owner registers beside, not against, ours — and its
	// record never satisfies OUR LiveWaiter fact.
	foreign := Caller{Class: "MAIN", MainId: "main-other", SessionId: "s2"}
	if err := s.RegisterWaiter("run", "w-run", foreign, fresh); err != nil {
		t.Fatalf("foreign owner blocked: %v", err)
	}
	if !LiveWaiter(s.Root, prober, "run", "w-run", mainCaller.MainId, fresh) {
		t.Fatal("own live waiter not seen")
	}
	if LiveWaiter(s.Root, prober, "run", "w-run", "main-third", fresh) {
		t.Fatal("a third party saw someone else's waiter as its own")
	}
	// Target mismatch never satisfies.
	if LiveWaiter(s.Root, prober, "run", "w-run", mainCaller.MainId, target) {
		t.Fatal("a stale target satisfied LiveWaiter")
	}
	// A HUMAN caller keys on human:<uid> — distinct from
	// every main and stable without session plumbing.
	human := Caller{Class: "HUMAN", SessionId: "h1"}
	if err := s.RegisterWaiter("run", "w-run", human, fresh); err != nil {
		t.Fatalf("human waiter refused: %v", err)
	}
	if !LiveWaiter(s.Root, prober, "run", "w-run", "", fresh) {
		t.Fatal("the human's waiter not seen under the uid key")
	}
	// Compare-and-delete: removal only deletes our own record.
	s.RemoveWaiter("run", "w-run", human)
	if LiveWaiter(s.Root, prober, "run", "w-run", "", fresh) {
		t.Fatal("removal missed the human record")
	}
	if !LiveWaiter(s.Root, prober, "run", "w-run", mainCaller.MainId, fresh) {
		t.Fatal("removal deleted someone else's record")
	}

	// The watch round-trip: a launched run concluded green exits 0 with
	// the waiter record gone.
	nonce := launchOne(t, s, "watch-run")
	prober.verdicts[501] = identity.Alive
	prober.starts[501] = 5000
	if err := s.Bind("watch-run", nonce, 501, 501); err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("watch-run")
	s.WriteSidecar("watch-run", record.Generation, record.LaunchNonce, 0)
	prober.verdicts[501] = identity.Dead
	done := make(chan int, 1)
	go func() { done <- s.Watch("watch-run", mainCaller, 30*time.Millisecond) }()
	waiterTarget := WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce}
	deadline := time.Now().Add(5 * time.Second)
	for !LiveWaiter(s.Root, prober, "run", "watch-run", mainCaller.MainId, waiterTarget) {
		if time.Now().After(deadline) {
			t.Fatal("watch did not publish its waiter record")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := s.Assess("watch-run"); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != ExitGreen {
			t.Fatalf("watch exit %d, want green", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not return after conclusion")
	}
	if LiveWaiter(s.Root, prober, "run", "watch-run", mainCaller.MainId, waiterTarget) {
		t.Fatal("the waiter record survived the watch exit")
	}
	// Watch on a missing record: 4.
	if code := s.Watch("ghost-run", mainCaller, time.Millisecond); code != ExitNoRecord {
		t.Fatalf("missing record watch exit %d, want 4", code)
	}
}

// A mutation carrying lease coordinates must prove
// its epoch is STILL current inside the runs lock — a stale child, an
// unverifiable lease, and a current holder each get their exact answer.
// Humans (nil epoch) are never gated.
func TestEpochSeamRefusesStale(t *testing.T) {
	s := testStore(t)
	current := int64(5)
	verifiable := true
	s.CurrentEpoch = func() (*int64, bool) {
		if !verifiable {
			return nil, false
		}
		return &current, true
	}
	epoch := int64(5)
	holder := mainCaller
	holder.ClaimEpoch = &epoch

	if _, err := s.Launch(holder, LaunchParams{Id: "epoch-ok", Kind: "suite", Log: "a.log"}); err != nil {
		t.Fatalf("a current-epoch launch refused: %v", err)
	}
	current = 6
	_, err := s.Launch(holder, LaunchParams{Id: "epoch-stale", Kind: "suite", Log: "b.log"})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a stale-epoch launch was allowed: %v", err)
	}
	verifiable = false
	_, err = s.Launch(holder, LaunchParams{Id: "epoch-blind", Kind: "suite", Log: "c.log"})
	if err == nil || !strings.Contains(err.Error(), "cannot verify") {
		t.Fatalf("an unverifiable epoch was allowed: %v", err)
	}
	// A human carries no epoch and is never gated on it.
	if _, err := s.Launch(Caller{Class: "HUMAN", SessionId: "h"}, LaunchParams{Id: "epoch-human", Kind: "suite", Log: "d.log"}); err != nil {
		t.Fatalf("a human launch was epoch-gated: %v", err)
	}
}

// Pattern evidence trusts the BOUND file, not the
// path — a log swapped for a symlink between bind and conclusion is
// refused as ended-unknown, never read through to a green.
func TestPatternEvidenceSymlinkSwap(t *testing.T) {
	s := testStore(t)
	// This test registers a real subprocess and must scan its real group.
	s.AllPids, s.Getpgid = nil, nil
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober

	sleeper := exec.Command("/bin/sleep", "30")
	sleeper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { sleeper.Process.Kill(); sleeper.Wait() }()
	pid := int64(sleeper.Process.Pid)
	prober.verdicts[pid] = identity.Alive
	prober.starts[pid] = 9000

	logPath := filepath.Join(s.Root, "swap.log")
	os.WriteFile(logPath, []byte("working\n"), 0o644)
	if err := s.Register(mainCaller, LaunchParams{Id: "swap-run", Kind: "custom", Log: logPath, WindDownMin: 1}, pid, "(?m)^ALL GREEN$"); err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("swap-run")

	// The swap: the bound path becomes a symlink to a file that WOULD
	// match the verdict pattern.
	decoy := filepath.Join(s.Root, "decoy.log")
	os.WriteFile(decoy, []byte("ALL GREEN\n"), 0o644)
	os.Remove(record.Log)
	if err := os.Symlink(decoy, record.Log); err != nil {
		t.Fatal(err)
	}

	sleeper.Process.Kill()
	sleeper.Wait()
	prober.verdicts[pid] = identity.Dead
	result, err := s.Assess("swap-run")
	if err != nil {
		t.Fatal(err)
	}
	if result.To == StatusDraining {
		s.Now = func() time.Time { return time.Unix(1786900000, 0).Add(200 * time.Minute) }
		result, err = s.Assess("swap-run")
		if err != nil {
			t.Fatal(err)
		}
	}
	if result.To != StatusEndedUnknown {
		t.Fatalf("a swapped log concluded %s, not ended-unknown", result.To)
	}
	record, _ = s.Read("swap-run")
	if record.Error == nil || !strings.Contains(*record.Error, "moved since bind") {
		t.Fatalf("the swap was not surfaced: %+v", record.Error)
	}
}
