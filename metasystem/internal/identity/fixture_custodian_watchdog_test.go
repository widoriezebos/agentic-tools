package identity

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stuckCall selects an ordinal call of one method for one process.
type stuckCall struct {
	method  string
	pid     int64
	ordinal int
}

// stuckProber advances the manual clock inside selected process reads.
type stuckProber struct {
	table              fixtureTable
	clock              *manualCustodianClock
	stall              time.Duration
	stuckCalls         map[stuckCall]bool
	goneAt             map[int64]int
	calls              map[stuckCall]int
	seen               map[int64]int
	sleeps             func() int
	unknownBeforeSleep map[int64]bool
	stuckAfterSleep    map[int64]bool
	stalledAfterSleep  bool
}

func (prober *stuckProber) Probe(pid int64) (Exact, Liveness, error) {
	return prober.answer("Probe", pid)
}

func (prober *stuckProber) answer(method string, pid int64) (Exact, Liveness, error) {
	if prober.calls == nil {
		prober.calls = make(map[stuckCall]int)
		prober.seen = make(map[int64]int)
	}
	key := stuckCall{method, pid, 0}
	prober.calls[key]++
	prober.seen[pid]++
	stuck := prober.stuckCalls[stuckCall{method, pid, prober.calls[key]}]
	if prober.stuckAfterSleep[pid] && prober.sleeps() >= 1 && !prober.stalledAfterSleep {
		prober.stalledAfterSleep = true
		stuck = true
	}
	if stuck {
		prober.clock.Advance(prober.stall)
	}
	if at := prober.goneAt[pid]; at != 0 && prober.seen[pid] >= at {
		delete(prober.table, pid)
	}
	if prober.unknownBeforeSleep[pid] {
		if prober.sleeps() == 0 {
			return Exact{}, Unknown, errors.New("unreadable")
		}
		return Exact{}, Dead, nil
	}
	return prober.table.Probe(pid)
}

type stuckStartProber struct{ *stuckProber }

func (prober stuckStartProber) ReadStart(pid int64) (Exact, Liveness, error) {
	return prober.answer("ReadStart", pid)
}

// sleepCountingClock places slow non-probe work at the first sleep.
type sleepCountingClock struct {
	*manualCustodianClock
	slept     int
	slowFirst time.Duration
}

func (clock *sleepCountingClock) Sleep(duration time.Duration) {
	clock.slept++
	if clock.slept == 1 {
		clock.Advance(clock.slowFirst)
	}
	clock.Advance(duration)
}

func watchdogRecords(t *testing.T, ref Ref) string {
	t.Helper()
	value, err := EncodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(path, []byte("+"+value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCustodianHaltTimesOneProbe(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"chain", "owner", "descendants", "pruning", "scan", "signalling", "leash", "unreadable-owner", "lost-launcher"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []string{"slow-work", "stuck-probe"} {
				t.Run(mode, func(t *testing.T) {
					t.Parallel()
					clock := newManualCustodianClock()
					processes := fixtureTable{}
					prober := &stuckProber{table: processes, clock: clock, stall: 7 * time.Second, goneAt: map[int64]int{}, stuckCalls: map[stuckCall]bool{}}
					var halted []int
					slowDone := false
					slow := func() {
						if mode == "slow-work" && !slowDone {
							slowDone = true
							clock.Advance(7 * time.Second)
						}
					}
					runtime := custodianRuntime{
						prober: prober, bound: 5 * time.Second, haltMargin: custodianHaltMargin, poll: 250 * time.Millisecond, clock: clock,
						halt:   func(code int) { halted = append(halted, code) },
						scan:   func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil },
						sender: func(pid int, _ syscall.Signal) error { delete(processes, int64(pid)); return nil },
					}
					var stuck stuckCall
					switch path {
					case "chain", "owner", "descendants":
						processes[700] = fixtureExact(700, 70)
						prober.goneAt[700] = 2
						runtime.descendants = func(p Prober, _ Ref) ([]Ref, error) {
							slow()
							if path == "descendants" {
								for pid := int64(801); pid <= 805; pid++ {
									_, _, _ = p.Probe(pid)
								}
							}
							return nil, nil
						}
						switch path {
						case "chain":
							processes[600] = fixtureExact(600, 60)
							runtime.chain = []Ref{processes[600].Ref()}
							stuck = stuckCall{"Probe", 600, 1}
						case "owner":
							runtime.prober = stuckStartProber{prober}
							stuck = stuckCall{"ReadStart", 700, 1}
						case "descendants":
							stuck = stuckCall{"Probe", 803, 1}
						}
					case "pruning":
						processes[801] = fixtureExact(801, 81)
						runtime.prober = stuckStartProber{prober}
						runtime.records = watchdogRecords(t, processes[801].Ref())
						runtime.scan = func(Prober, Ref) ([]FixtureSurvivor, error) { slow(); return nil, nil }
						stuck = stuckCall{"Probe", 801, 1}
					case "scan":
						runtime.scan = func(p Prober, _ Ref) ([]FixtureSurvivor, error) {
							slow()
							for pid := int64(811); pid <= 815; pid++ {
								_, _, _ = p.Probe(pid)
							}
							return nil, nil
						}
						stuck = stuckCall{"Probe", 813, 1}
					case "signalling":
						processes[821] = fixtureExact(821, 82)
						runtime.prober = stuckStartProber{prober}
						scans := 0
						runtime.scan = func(Prober, Ref) ([]FixtureSurvivor, error) {
							scans++
							if scans == 1 {
								return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: processes[821].Ref(), Carrier: FixtureCarrierRecord}}, nil
							}
							return nil, nil
						}
						runtime.sender = func(pid int, _ syscall.Signal) error { slow(); delete(processes, int64(pid)); return nil }
						stuck = stuckCall{"ReadStart", 821, 1}
					case "leash":
						processes[831] = fixtureExact(831, 83)
						runtime.records = watchdogRecords(t, processes[831].Ref())
						calls := 0
						runtime.leash = func() (bool, error) { calls++; slow(); return calls == 1, nil }
						stuck = stuckCall{"Probe", 831, 1}
					case "unreadable-owner":
						counting := &sleepCountingClock{manualCustodianClock: clock}
						if mode == "slow-work" {
							counting.slowFirst = 7 * time.Second
						}
						runtime.clock = counting
						prober.sleeps = func() int { return counting.slept }
						prober.unknownBeforeSleep = map[int64]bool{700: true}
						prober.stuckAfterSleep = map[int64]bool{700: mode == "stuck-probe"}
					case "lost-launcher":
						processes[700] = fixtureExact(700, 70)
						runtime.chain = []Ref{fixtureExact(600, 60).Ref()}
						runtime.sender = func(pid int, _ syscall.Signal) error { slow(); delete(processes, int64(pid)); return nil }
						stuck = stuckCall{"Probe", 700, 2}
					}
					if mode == "stuck-probe" && stuck.pid != 0 {
						prober.stuckCalls[stuck] = true
					}
					owner := fixtureExact(700, 70).Ref()
					var log strings.Builder
					if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil {
						t.Fatal(err)
					}
					if !strings.HasSuffix(log.String(), FixtureCustodianCompletionLine(owner)) {
						t.Fatalf("missing completion: %s", &log)
					}
					if path == "lost-launcher" && !strings.Contains(log.String(), "action=kill-owner") {
						t.Fatalf("missing owner kill: %s", &log)
					}
					if runtime.records != "" {
						if _, err := os.Stat(runtime.records); !os.IsNotExist(err) {
							t.Fatalf("records remain: %v", err)
						}
					}
					if mode == "slow-work" {
						if len(halted) != 0 || strings.Contains(log.String(), "action=halt") {
							t.Fatalf("slow work halted: exits=%v log=%q", halted, &log)
						}
					} else if !slices.Equal(halted, []int{2}) || strings.Count(log.String(), "fixture-custodian action=halt after=6s\n") != 1 {
						t.Fatalf("stuck probe halt: exits=%v, want [2] and one halt line; log=%q", halted, &log)
					}
				})
			}
		})
	}
}

type custodianReadyBuffer struct{ strings.Builder }

func (*custodianReadyBuffer) Close() error { return nil }

func TestCustodianStartTimesOneProbe(t *testing.T) {
	t.Parallel()
	const helper = "FIXTURE_CUSTODIAN_START_TEST"
	if os.Getenv(helper) != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestCustodianStartTimesOneProbe$", "-test.timeout=30m", "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != RunOwnerEnv && name != FixtureCustodianChainEnv && name != FixtureCustodianEnv && name != helper {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, helper+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("startup probe fixture: %v\n%s", err, output)
		} else {
			t.Logf("%s", output)
		}
		return
	}
	for _, mode := range []string{"self-stuck", "chain-stuck", "slow-start"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{RunOwnerEnv, FixtureCustodianChainEnv, FixtureCustodianEnv} {
				if _, set := os.LookupEnv(name); set {
					t.Fatalf("fixture inherited %s", name)
				}
			}
			clock := newManualCustodianClock()
			self := int64(os.Getpid())
			prober := &stuckProber{table: fixtureTable{self: fixtureExact(self, 90), 600: fixtureExact(600, 60)}, clock: clock, stall: 7 * time.Second, stuckCalls: map[stuckCall]bool{}}
			if mode == "self-stuck" {
				prober.stuckCalls[stuckCall{"Probe", self, 1}] = true
			}
			if mode == "chain-stuck" {
				prober.stuckCalls[stuckCall{"Probe", 600, 1}] = true
			}
			first := true
			parent := func(pid int64) (int64, bool) {
				if first && mode == "slow-start" {
					clock.Advance(7 * time.Second)
				}
				first = false
				return 600, pid == 700
			}
			var halted []int
			runtime := custodianRuntime{prober: prober, bound: 5 * time.Second, poll: 250 * time.Millisecond, clock: clock,
				halt: func(code int) { halted = append(halted, code) }, scan: func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil }}
			var log strings.Builder
			ready := &custodianReadyBuffer{}
			owner := fixtureExact(700, 70).Ref()
			if err := startCustodian(owner, strings.NewReader(""), ready, &log, parent, runtime); err != nil {
				t.Fatal(err)
			}
			if ready.String() != "ready\n" || !strings.HasSuffix(log.String(), FixtureCustodianCompletionLine(owner)) {
				t.Fatalf("startup did not finish: ready=%q log=%q", ready.String(), &log)
			}
			if mode == "slow-start" {
				if len(halted) != 0 || strings.Contains(log.String(), "action=halt") {
					t.Fatalf("slow start halted: exits=%v log=%q", halted, &log)
				}
			} else if !slices.Equal(halted, []int{2}) || strings.Count(log.String(), "fixture-custodian action=halt after=6s\n") != 1 {
				t.Fatalf("startup stuck probe halt: exits=%v, want [2] and one halt line; log=%q", halted, &log)
			}
		})
	}
}

var _ io.WriteCloser = (*custodianReadyBuffer)(nil)
