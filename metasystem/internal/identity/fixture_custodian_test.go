package identity

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type custodianRecordProber struct {
	fixtureTable
	diesOnLaterProbe int64
	probes           int
}

type manualCustodianClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*manualCustodianTimer]struct{}
}

type manualCustodianTimer struct {
	clock    *manualCustodianClock
	deadline time.Time
	action   func()
	stopped  bool
	fired    bool
}

func newManualCustodianClock() *manualCustodianClock {
	return &manualCustodianClock{now: time.Unix(1, 0), timers: make(map[*manualCustodianTimer]struct{})}
}

func (clock *manualCustodianClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *manualCustodianClock) After(duration time.Duration) <-chan time.Time {
	clock.Advance(duration)
	ready := make(chan time.Time, 1)
	ready <- clock.Now()
	return ready
}

func (clock *manualCustodianClock) AfterFunc(duration time.Duration, action func()) custodianTimer {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := &manualCustodianTimer{clock: clock, deadline: clock.now.Add(duration), action: action}
	clock.timers[timer] = struct{}{}
	return timer
}

func (clock *manualCustodianClock) Sleep(duration time.Duration) { clock.Advance(duration) }

func (clock *manualCustodianClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	var actions []func()
	for timer := range clock.timers {
		if timer.stopped || timer.fired || timer.deadline.After(clock.now) {
			continue
		}
		timer.fired = true
		delete(clock.timers, timer)
		actions = append(actions, timer.action)
	}
	clock.mu.Unlock()
	for _, action := range actions {
		action()
	}
}

func (timer *manualCustodianTimer) Stop() bool {
	timer.clock.mu.Lock()
	defer timer.clock.mu.Unlock()
	if timer.stopped || timer.fired {
		return false
	}
	timer.stopped = true
	delete(timer.clock.timers, timer)
	return true
}

func (prober *custodianRecordProber) Probe(pid int64) (Exact, Liveness, error) {
	if pid == prober.diesOnLaterProbe {
		prober.probes++
		if prober.probes > 1 {
			delete(prober.fixtureTable, pid)
		}
	}
	return prober.fixtureTable.Probe(pid)
}

func TestCustodianUsesConfiguredTiming(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		FixtureCustodianPollEnv:  "17ms",
		FixtureCustodianBoundEnv: "23s",
	}
	poll, bound, err := custodianTiming(func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	})
	if err != nil || poll != 17*time.Millisecond || bound != 23*time.Second {
		t.Fatalf("configured timing poll=%s bound=%s err=%v", poll, bound, err)
	}
	for _, name := range []string{FixtureCustodianPollEnv, FixtureCustodianBoundEnv} {
		values[name] = "0s"
		if _, _, err := custodianTiming(func(key string) (string, bool) {
			value, present := values[key]
			return value, present
		}); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("invalid %s was accepted: %v", name, err)
		}
		delete(values, name)
	}
}

func TestCustodianReapsRecordedRefsWithoutTheTable(t *testing.T) {
	a, b := fixtureExact(500, 50).Ref(), fixtureExact(501, 51).Ref()
	aValue, _ := EncodeRef(a)
	bValue, _ := EncodeRef(b)
	recycled := fixtureExact(500, 50)
	if recycled.StartTicks != 0 {
		recycled.StartTicks++
	} else {
		recycled.StartedAt = recycled.StartedAt.Add(time.Microsecond)
	}
	for _, test := range []struct {
		name, contents   string
		current          Exact
		keepAlive        bool
		wantSignals      []int64
		wantLog          string
		unwantLog        string
		wantError        string
		wantRecordFile   bool
		wantCompleteLog  bool
		diesOnLaterProbe bool
		hasLeash         bool
		leashStates      []bool
		leashAlwaysHeld  bool
	}{
		{name: "live record", contents: "+" + aValue + "\n", current: fixtureExact(500, 50), wantSignals: []int64{500}, wantLog: "carrier=record", wantCompleteLog: true},
		{name: "leashed record exits during grace", contents: "+" + aValue + "\n", current: fixtureExact(500, 50), wantLog: "action=release pid=500 carrier=record", unwantLog: "action=kill pid=", wantCompleteLog: true, diesOnLaterProbe: true, hasLeash: true, leashStates: []bool{true, true, false}},
		{name: "leashed record never exits", contents: "+" + aValue + "\n", current: fixtureExact(500, 50), wantSignals: []int64{500}, wantLog: "action=kill pid=500 carrier=record", wantCompleteLog: true, hasLeash: true, leashAlwaysHeld: true},
		{name: "no leash signals at once", contents: "+" + aValue + "\n", current: fixtureExact(500, 50), wantSignals: []int64{500}, wantLog: "action=kill pid=500 carrier=record", wantCompleteLog: true, diesOnLaterProbe: true},
		{name: "dead record without leash keeps its line", contents: "+" + aValue + "\n", wantLog: "action=kill pid=500 carrier=record result=recorded process is gone", wantCompleteLog: true},
		{name: "live record remains alive", contents: "+" + aValue + "\n", current: fixtureExact(500, 50), keepAlive: true, wantSignals: []int64{500}, wantLog: "carrier=record", wantError: "fixture custodian cleanup exceeded", wantRecordFile: true},
		{name: "released record", contents: "+" + aValue + "\n-" + aValue + "\n", current: fixtureExact(500, 50), wantCompleteLog: true},
		{name: "torn final record", contents: "+" + aValue + "\n+" + bValue, current: fixtureExact(500, 50), wantSignals: []int64{500}, unwantLog: "pid=501", wantCompleteLog: true},
		{name: "recycled pid", contents: "+" + aValue + "\n", current: recycled, wantLog: "recorded process is gone", wantCompleteLog: true},
		{name: "malformed record", contents: "not-a-record\n", wantLog: "error=malformed-record line=1", wantCompleteLog: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			clock := newManualCustodianClock()
			records := filepath.Join(t.TempDir(), "records")
			if err := os.WriteFile(records, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			processes := fixtureTable{}
			if test.current.Pid != 0 {
				processes[test.current.Pid] = test.current
			}
			var signaled []int64
			var log strings.Builder
			prober := &custodianRecordProber{fixtureTable: processes}
			if test.diesOnLaterProbe {
				prober.diesOnLaterProbe = test.current.Pid
			}
			leashCalls := 0
			watchCloses := 0
			var leash func() (bool, error)
			if test.hasLeash {
				leash = func() (bool, error) {
					leashCalls++
					if test.leashAlwaysHeld {
						return true, nil
					}
					index := leashCalls - 1
					if index >= len(test.leashStates) {
						index = len(test.leashStates) - 1
					}
					return test.leashStates[index], nil
				}
			}
			runtime := custodianRuntime{
				prober: prober, records: records, poll: time.Millisecond, bound: 20 * time.Millisecond,
				clock: clock,
				leash: leash,
				scan:  func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, fmt.Errorf("denied kern.proc.all") },
				sender: func(pid int, _ syscall.Signal) error {
					signaled = append(signaled, int64(pid))
					if !test.keepAlive {
						delete(processes, int64(pid))
					}
					return nil
				},
			}
			if test.hasLeash {
				runtime.closeWatch = func() error {
					watchCloses++
					return nil
				}
			}
			err := reapDeadOwner(fixtureExact(700, 70).Ref(), &log, runtime)
			if test.wantError == "" && err != nil || test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("reapDeadOwner error = %v, want containing %q", err, test.wantError)
			}
			if !slices.Equal(signaled, test.wantSignals) {
				t.Fatalf("signals = %v, want %v", signaled, test.wantSignals)
			}
			if test.hasLeash && leashCalls < 2 {
				t.Fatalf("leash seam checks = %d, want at least 2", leashCalls)
			}
			if test.hasLeash && watchCloses != 1 {
				t.Fatalf("watch closes = %d, want 1", watchCloses)
			}
			if test.wantLog != "" && !strings.Contains(log.String(), test.wantLog) {
				t.Fatalf("log %q does not contain %q", log.String(), test.wantLog)
			}
			if test.unwantLog != "" && strings.Contains(log.String(), test.unwantLog) {
				t.Fatalf("log %q contains forbidden text %q", log.String(), test.unwantLog)
			}
			if strings.Count(log.String(), "scan=unavailable error=denied kern.proc.all") != 1 || strings.Contains(log.String(), "action=complete") != test.wantCompleteLog {
				t.Fatalf("custodian log = %q", log.String())
			}
			_, statErr := os.Lstat(records)
			if test.wantRecordFile && statErr != nil || !test.wantRecordFile && !os.IsNotExist(statErr) {
				t.Fatalf("record file state after cleanup: %v; want retained=%t", statErr, test.wantRecordFile)
			}
		})
	}
}
func TestControlledLaunchersExportTheirOwnRef(t *testing.T) {
	backing := make([]string, 1)
	got, err := ExportRunOwner(backing[:0])
	if err != nil || len(got) != 1 || backing[0] != "" {
		t.Fatalf("ExportRunOwner(spare capacity) = %v, %v; backing=%q", got, err, backing[0])
	}
	ref, err := ParseRef(strings.TrimPrefix(got[0], RunOwnerEnv+"="))
	if err != nil || AliveRef(KernelProber{}, ref) != Alive || ref.Pid != int64(os.Getpid()) {
		t.Fatalf("exported run owner = %v, %v", ref, err)
	}
	existing := []string{"A=b", RunOwnerEnv + "=outer"}
	if preserved, err := ExportRunOwner(existing); err != nil || len(preserved) != 2 || preserved[1] != existing[1] {
		t.Fatalf("existing run owner changed: %v, %v", preserved, err)
	}

	root := filepath.Clean(filepath.Join("..", ".."))
	sourceRoot := os.Getenv("FIXTURE_SOURCE_ROOT")
	if sourceRoot == "" {
		sourceRoot = root
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, "./cmd/metasystem")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build proc ref witness: %v\n%s", err, output)
	}
	command := exec.Command("sh", "-c", "metasystem proc ref --pid $$")
	command.Env = append(os.Environ(), "PATH="+filepath.Dir(engine)+":"+os.Getenv("PATH"))
	var output strings.Builder
	command.Stdout = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	shellPID := int64(command.Process.Pid)
	if err := command.Wait(); err != nil {
		t.Fatalf("shell proc ref: %v, output=%q", err, output.String())
	}
	shellRef, err := ParseRef(strings.TrimSpace(output.String()))
	if err != nil || shellRef.Pid != shellPID || !shellRef.NativeExact() {
		t.Fatalf("shell proc ref = %+v, %v; shell pid=%d output=%q", shellRef, err, shellPID, output.String())
	}

	gate := readLauncherSource(t, filepath.Join(sourceRoot, "scripts", "agents", "go-gate.sh"))
	exportAt, firstTestAt := strings.Index(gate, "export METASYSTEM_RUN_OWNER"), strings.Index(gate, "go test -count=1")
	if exportAt < 0 || firstTestAt < 0 || exportAt > firstTestAt {
		t.Fatalf("go-gate run owner export position=%d, first go test position=%d", exportAt, firstTestAt)
	}
	bed := readLauncherSource(t, filepath.Join(sourceRoot, "scripts", "agents", "fixture-bed-scenarios.sh"))
	ownerAt, childAt := strings.Index(bed, "harness_fixture_owner \"$fixture_bed_harness_root\""), strings.Index(bed, "\"$script\" --fixture-bed-child")
	if ownerAt < 0 || childAt < 0 || ownerAt > childAt {
		t.Fatalf("fixture bed owner position=%d, first scenario child position=%d", ownerAt, childAt)
	}
	budget := readLauncherSource(t, filepath.Join(sourceRoot, "scripts", "agents", "fixture-budget.sh"))
	ownerStart, ownerEnd := strings.Index(budget, "harness_fixture_owner()"), strings.Index(budget, "harness_fixture_record_pid()")
	if ownerStart < 0 || ownerEnd <= ownerStart {
		t.Fatal("fixture owner helper boundaries were not found")
	}
	ownerSource := budget[ownerStart:ownerEnd]
	ownerRefAt, ownerExportAt, custodianAt := strings.Index(ownerSource, "METASYSTEM_RUN_OWNER=$("), strings.Index(ownerSource, "export METASYSTEM_RUN_OWNER"), strings.Index(ownerSource, "proc custodian")
	if ownerRefAt < 0 || ownerExportAt < ownerRefAt || custodianAt < ownerExportAt {
		t.Fatalf("fixture owner ref=%d export=%d first child=%d", ownerRefAt, ownerExportAt, custodianAt)
	}
}

func readLauncherSource(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func TestRunOwnerResolution(t *testing.T) {
	owner, goTest, goTool, launcher := fixtureExact(700, 70), fixtureExact(701, 71), fixtureExact(702, 72), fixtureExact(703, 73)
	goTest.Argv, goTest.ArgvKnown = []string{"/usr/bin/go", "test"}, true
	goTool.Argv, goTool.ArgvKnown = []string{"go", "tool"}, true
	launcher.Argv, launcher.ArgvKnown = []string{"sh", "run"}, true
	table := fixtureTable{700: owner, 701: goTest, 702: goTool, 703: launcher}
	parents := map[int64]int64{700: 701, 701: 702, 702: 703, 703: 1}
	parent := func(pid int64) (int64, bool) { next, ok := parents[pid]; return next, ok }
	for index, unreadable := range []bool{false, true} {
		t.Run([]string{"go tooling", "unreadable argv ends walk"}[index], func(t *testing.T) {
			goTool.ArgvKnown = !unreadable
			table[702] = goTool
			chain, err := resolveRunOwner(table, parent, owner.Ref(), "", false)
			wantLen, wantLast := 3-index, int64(703-index)
			if err != nil || len(chain) != wantLen || chain[len(chain)-1].Pid != wantLast {
				t.Fatalf("chain=%v err=%v; want length %d ending at %d", chain, err, wantLen, wantLast)
			}
		})
	}
}

type custodianTable struct {
	fixtureTable
	ownerStates []Liveness
	ownerProbes int
}

func (table *custodianTable) Probe(pid int64) (Exact, Liveness, error) {
	if pid == 700 {
		state := Dead
		if table.ownerProbes < len(table.ownerStates) {
			state = table.ownerStates[table.ownerProbes]
			table.ownerProbes++
		}
		if state == Alive {
			return table.fixtureTable.Probe(pid)
		}
		return Exact{}, state, nil
	}
	return table.fixtureTable.Probe(pid)
}

func TestCustodianWaitsForDeadOwnerAndExcludesItself(t *testing.T) {
	owner, self := fixtureExact(700, 70).Ref(), fixtureExact(701, 71)
	environmentChild, argvChild := fixtureExact(702, 72), fixtureExact(703, 73)
	key := FixtureKey{Owner: owner, Test: t.Name(), Nonce: "00000001"}
	self.Argv, self.ArgvKnown = []string{fixtureWord(t, key)}, true
	environmentChild.Environ, environmentChild.EnvironKnown = []string{fixtureWord(t, key)}, true
	argvChild.Argv, argvChild.ArgvKnown = []string{fixtureWord(t, key)}, true
	processes := fixtureTable{700: fixtureExact(700, 70), 701: self, 702: environmentChild, 703: argvChild}
	installFixtureScanTable(t, processes)
	var signaled []int64
	var log strings.Builder
	runtime := custodianRuntime{prober: processes, self: self.Ref(), poll: time.Millisecond, bound: 2 * time.Millisecond,
		sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		}}
	if err := reapDeadOwner(owner, &log, runtime); err == nil || len(signaled) != 0 {
		t.Fatalf("live owner allowed reap: signaled=%v err=%v", signaled, err)
	}
	delete(processes, owner.Pid)
	table := &custodianTable{fixtureTable: processes, ownerStates: []Liveness{Alive, Alive, Dead}}
	runtime.prober, runtime.bound = table, 30*time.Millisecond
	runtime.clock = newManualCustodianClock()
	if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil || len(signaled) != 2 || signaled[0] != 702 || signaled[1] != 703 {
		t.Fatalf("custodian signaled=%v err=%v; want children 702 and 703 after owner death, never self 701", signaled, err)
	}
	if !strings.Contains(log.String(), "pid=702 carrier=environment") || !strings.Contains(log.String(), "pid=703 carrier=argv-word") {
		t.Fatalf("custodian log %q does not name each child's carrier", log.String())
	}
}

func TestCustodianReapsObservedDescendantsAfterReparenting(t *testing.T) {
	t.Parallel()

	owner, self := fixtureExact(700, 70), fixtureExact(701, 71)
	child, grandchild, unrelated := fixtureExact(702, 72), fixtureExact(703, 73), fixtureExact(704, 74)
	reused, unstable := fixtureExact(705, 75), fixtureExact(706, 76)
	for _, exact := range []*Exact{&self, &child, &grandchild, &unrelated, &reused, &unstable} {
		exact.EnvironKnown = true
	}
	processes := fixtureTable{
		owner.Pid: owner, self.Pid: self, child.Pid: child, grandchild.Pid: grandchild, unrelated.Pid: unrelated,
		reused.Pid: reused, unstable.Pid: unstable,
	}
	parents := map[int64]int64{
		self.Pid: owner.Pid, child.Pid: owner.Pid, grandchild.Pid: child.Pid, unrelated.Pid: 1,
		reused.Pid: owner.Pid, unstable.Pid: owner.Pid,
	}
	unstableParentReads := 0
	table := &custodianTable{fixtureTable: processes, ownerStates: []Liveness{Alive, Alive, Alive, Dead}}
	var signaled []int64
	runtime := custodianRuntime{
		prober: table, self: self.Ref(), poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: newManualCustodianClock(),
		descendants: func(prober Prober, ownerRef Ref) ([]Ref, error) {
			refs, err := fixtureDescendants(prober, ownerRef,
				func() ([]int64, error) {
					return []int64{owner.Pid, self.Pid, child.Pid, grandchild.Pid, unrelated.Pid, reused.Pid, unstable.Pid}, nil
				},
				func(pid int64) (int64, bool) {
					if pid == unstable.Pid {
						unstableParentReads++
						if unstableParentReads > 1 {
							return 1, true
						}
					}
					parent, known := parents[pid]
					return parent, known
				},
			)
			parents[child.Pid], parents[grandchild.Pid] = 1, 1
			processes[reused.Pid] = fixtureExact(reused.Pid, 7_500)
			return refs, err
		},
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil },
		sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		},
	}
	var log strings.Builder
	if err := runCustodian(owner.Ref(), strings.NewReader(""), &log, runtime); err != nil {
		t.Fatal(err)
	}
	slices.Sort(signaled)
	if !slices.Equal(signaled, []int64{child.Pid, grandchild.Pid}) {
		t.Fatalf("signaled=%v, want reparented child and grandchild but not reused pid %d", signaled, reused.Pid)
	}
	for _, want := range []string{
		"action=watch", "action=observe pid=702 carrier=descendant", "action=observe pid=703 carrier=descendant",
		"action=kill pid=702 carrier=descendant", "action=kill pid=703 carrier=descendant", "action=complete",
	} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("custodian log %q omits %q", log.String(), want)
		}
	}
	if strings.Contains(log.String(), "pid=701 carrier=descendant") || strings.Contains(log.String(), "pid=704 carrier=descendant") ||
		strings.Contains(log.String(), "pid=706 carrier=descendant") {
		t.Fatalf("custodian log %q observed itself, an unrelated process, or an unstable parent edge", log.String())
	}
}

func TestCustodianOwnerDeathDuringObservationIsQuiet(t *testing.T) {
	t.Parallel()

	owner := fixtureExact(700, 70)
	for _, states := range [][]Liveness{{Alive, Dead}, {Alive, Alive, Dead}} {
		table := &custodianTable{fixtureTable: fixtureTable{owner.Pid: owner}, ownerStates: states}
		var log strings.Builder
		runtime := custodianRuntime{
			prober: table, poll: time.Millisecond, bound: 5 * time.Millisecond, clock: newManualCustodianClock(),
			descendants: func(prober Prober, ownerRef Ref) ([]Ref, error) {
				return fixtureDescendants(prober, ownerRef,
					func() ([]int64, error) { return []int64{owner.Pid}, nil },
					func(int64) (int64, bool) { return 0, true },
				)
			},
			scan: func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil },
		}
		if err := runCustodian(owner.Ref(), strings.NewReader(""), &log, runtime); err != nil {
			t.Fatal(err)
		}
		want := FixtureCustodianWatchLine(owner.Ref()) + FixtureCustodianCompletionLine(owner.Ref())
		if log.String() != want {
			t.Fatalf("owner states %v log=%q, want quiet %q", states, log.String(), want)
		}
	}
}

func TestNestedCustodiansDoNotSignalEachOther(t *testing.T) {
	t.Parallel()

	launcher, launcherCustodian := fixtureExact(700, 70), fixtureExact(701, 71)
	shell, owner, ownerCustodian := fixtureExact(702, 72), fixtureExact(703, 73), fixtureExact(704, 74)
	child, grandchild, unreadable := fixtureExact(705, 75), fixtureExact(706, 76), fixtureExact(707, 77)
	for _, exact := range []*Exact{&shell, &owner, &child, &grandchild} {
		exact.EnvironKnown = true
	}
	marker := FixtureCustodianEnv + "=1"
	launcherCustodian.Environ, launcherCustodian.EnvironKnown = []string{marker}, true
	key := FixtureKey{Owner: launcher.Ref(), Test: t.Name(), Nonce: "00000001"}
	ownerCustodian.Environ, ownerCustodian.EnvironKnown = []string{marker, fixtureWord(t, key)}, true
	processes := fixtureTable{700: launcher, 701: launcherCustodian, 702: shell, 703: owner, 704: ownerCustodian, 705: child, 706: grandchild, 707: unreadable}
	parents := map[int64]int64{701: 700, 702: 700, 703: 702, 704: 703, 705: 703, 706: 705, 707: 703}
	survivors, err := ScanFixtureSurvivors([]int64{704}, processes, func(int64) FixtureProcessScope { return FixtureProcessScope{} }, FixtureSurvivorSelection{Key: &key})
	if err != nil || len(survivors) != 0 {
		t.Fatalf("nested custodian survivor selection=%v err=%v, want none", survivors, err)
	}
	table := &custodianTable{fixtureTable: processes, ownerStates: []Liveness{Alive, Alive, Alive, Dead}}
	var signaled []int64
	runtime := custodianRuntime{prober: table, self: launcherCustodian.Ref(), poll: time.Millisecond, bound: 5 * time.Millisecond, clock: newManualCustodianClock(),
		descendants: func(prober Prober, root Ref) ([]Ref, error) {
			refs, scanErr := fixtureDescendants(prober, root, func() ([]int64, error) { return []int64{700, 701, 702, 703, 704, 705, 706, 707}, nil }, func(pid int64) (int64, bool) { parent, ok := parents[pid]; return parent, ok })
			delete(processes, launcher.Pid)
			return refs, scanErr
		},
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil }, sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		}}
	var launcherLog strings.Builder
	if err := runCustodian(launcher.Ref(), strings.NewReader(""), &launcherLog, runtime); err != nil {
		t.Fatal(err)
	}
	slices.Sort(signaled)
	if !slices.Equal(signaled, []int64{702, 703, 705, 706}) || AliveRef(processes, ownerCustodian.Ref()) != Alive || AliveRef(processes, unreadable.Ref()) != Alive {
		t.Fatalf("launcher custodian signaled=%v nested-state=%s unreadable-state=%s log=%q", signaled, AliveRef(processes, ownerCustodian.Ref()), AliveRef(processes, unreadable.Ref()), launcherLog.String())
	}
	var ownerLog strings.Builder
	ownerRuntime := custodianRuntime{prober: processes, self: ownerCustodian.Ref(), poll: time.Millisecond, bound: 5 * time.Millisecond, clock: newManualCustodianClock(), scan: func(Prober, Ref) ([]FixtureSurvivor, error) { return nil, nil }}
	if err := reapLostLauncher(owner.Ref(), launcher.Ref(), &ownerLog, ownerRuntime); err != nil || !strings.Contains(ownerLog.String(), "dead-launcher=") {
		t.Fatalf("nested custodian did not finish its launcher-loss reap: err=%v log=%q", err, ownerLog.String())
	}
}

func TestCustodianHardHaltBoundsBlockedScan(t *testing.T) {
	t.Parallel()

	clock := newManualCustodianClock()
	var halted []int
	runtime := custodianRuntime{prober: fixtureTable{}, poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: clock, halt: func(code int) { halted = append(halted, code) },
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			clock.Advance(5*time.Millisecond + custodianHaltMargin)
			return nil, nil
		}}
	var log strings.Builder
	if err := reapDeadOwner(fixtureExact(700, 70).Ref(), &log, runtime); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(halted, []int{2, 2}) {
		t.Fatalf("hard halt exits=%v, want one per blocked scan", halted)
	}
	want := fmt.Sprintf("fixture-custodian action=halt after=%s\n", runtime.bound+custodianHaltMargin)
	if strings.Count(log.String(), want) != 2 {
		t.Fatalf("hard halt log=%q, want two %q lines", log.String(), want)
	}
}

func TestCustodianSlowScansComplete(t *testing.T) {
	owner, recorded := fixtureExact(700, 70).Ref(), fixtureExact(701, 71).Ref()
	recordedValue, err := EncodeRef(recorded)
	if err != nil {
		t.Fatal(err)
	}
	records := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(records, []byte("+"+recordedValue+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	processes := fixtureTable{recorded.Pid: fixtureExact(recorded.Pid, 71)}
	clock := newManualCustodianClock()
	var halted []int
	runtime := custodianRuntime{
		prober: processes, records: records, poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: clock, halt: func(code int) { halted = append(halted, code) },
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			clock.Advance(600 * time.Millisecond)
			return nil, nil
		},
		sender: func(pid int, _ syscall.Signal) error {
			delete(processes, int64(pid))
			return nil
		},
	}
	var log strings.Builder
	if err := reapDeadOwner(owner, &log, runtime); err != nil {
		t.Fatalf("slow fixture scans returned %v: log=%q", err, log.String())
	}
	if want := FixtureCustodianCompletionLine(owner); !strings.HasSuffix(log.String(), want) {
		t.Fatalf("slow fixture scan log=%q, want suffix %q", log.String(), want)
	}
	if len(halted) != 0 {
		t.Fatalf("slow fixture scans hard-halted with codes %v: log=%q", halted, log.String())
	}
}

func TestCustodianConvergesAfterTwoScansWithoutNewDescendants(t *testing.T) {
	t.Parallel()

	owner := fixtureExact(700, 70).Ref()
	first, late := fixtureExact(801, 81), fixtureExact(802, 82)
	processes := fixtureTable{}
	clock := newManualCustodianClock()
	pass := 0
	runtime := custodianRuntime{
		prober: processes, poll: 250 * time.Millisecond, bound: 5 * time.Second, clock: clock,
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			pass++
			switch pass {
			case 1:
				processes[first.Pid] = first
				return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: first.Ref(), Carrier: FixtureCarrierEnvironment}}, nil
			case 3:
				processes[late.Pid] = late
				return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: late.Ref(), Carrier: FixtureCarrierEnvironment}}, nil
			default:
				return nil, nil
			}
		},
		sender: func(pid int, _ syscall.Signal) error {
			delete(processes, int64(pid))
			return nil
		},
	}
	if err := reapDeadOwner(owner, io.Discard, runtime); err != nil {
		t.Fatal(err)
	}
	if pass != 5 {
		t.Fatalf("cleanup scans=%d, want 5: a late new identity resets exactly two settling scans", pass)
	}
}

func TestCustodianCleanupBoundRequiresProgress(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	processes := fixtureTable{}
	nextPid := int64(800)
	clock := newManualCustodianClock()
	runtime := custodianRuntime{
		prober: processes, poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: clock,
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			nextPid++
			exact := fixtureExact(nextPid, nextPid)
			processes[nextPid] = exact
			return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: exact.Ref(), Carrier: FixtureCarrierEnvironment}}, nil
		},
		sender: func(pid int, _ syscall.Signal) error {
			delete(processes, int64(pid))
			return nil
		},
	}
	started := clock.Now()
	err := reapDeadOwner(owner, io.Discard, runtime)
	elapsed := clock.Now().Sub(started)
	if err == nil || !strings.Contains(err.Error(), "fixture custodian cleanup exceeded") {
		t.Fatalf("respawning fixture cleanup error=%v after %s", err, elapsed)
	}
	if elapsed < runtime.bound || elapsed >= custodianHaltMargin {
		t.Fatalf("respawning fixture cleanup took %s, want at least %s and less than %s", elapsed, runtime.bound, custodianHaltMargin)
	}
}

func TestCustodianCleanupBoundStopsARespawningSet(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	processes := fixtureTable{}
	records := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(records, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	pass := 0
	nextPID := int64(800)
	clock := newManualCustodianClock()
	runtime := custodianRuntime{
		prober: processes, records: records, poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: clock,
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			pass++
			if pass%2 == 0 {
				return nil, nil
			}
			nextPID++
			survivor := fixtureExact(nextPID, nextPID)
			processes[survivor.Pid] = survivor
			return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: survivor.Ref(), Carrier: FixtureCarrierEnvironment}}, nil
		},
		sender: func(pid int, _ syscall.Signal) error {
			delete(processes, int64(pid))
			return nil
		},
	}
	started := clock.Now()
	err := reapDeadOwner(owner, io.Discard, runtime)
	elapsed := clock.Now().Sub(started)
	if err == nil || !strings.Contains(err.Error(), "fixture custodian cleanup exceeded") || elapsed < runtime.bound {
		t.Fatalf("respawning fixture cleanup error=%v after %s", err, elapsed)
	}
	if _, err := os.Stat(records); err != nil {
		t.Fatalf("respawning fixture cleanup removed records: %v", err)
	}
}

func TestCustodianCleanupBoundAllowsSlowProgress(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	processes := fixtureTable{}
	for pid := int64(800); pid < 810; pid++ {
		processes[pid] = fixtureExact(pid, pid)
	}
	removedThisPass := false
	clock := newManualCustodianClock()
	runtime := custodianRuntime{
		prober: processes, poll: time.Millisecond, bound: 5 * time.Millisecond,
		clock: clock,
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			removedThisPass = false
			survivors := make([]FixtureSurvivor, 0, len(processes))
			for _, exact := range processes {
				survivors = append(survivors, FixtureSurvivor{Class: FixtureSurvivorCertain, Ref: exact.Ref(), Carrier: FixtureCarrierEnvironment})
			}
			return survivors, nil
		},
		sender: func(pid int, _ syscall.Signal) error {
			if !removedThisPass {
				delete(processes, int64(pid))
				removedThisPass = true
			}
			return nil
		},
	}
	started := clock.Now()
	if err := reapDeadOwner(owner, io.Discard, runtime); err != nil {
		t.Fatalf("shrinking fixture cleanup returned %v with %d survivors", err, len(processes))
	}
	if elapsed := clock.Now().Sub(started); elapsed <= runtime.bound {
		t.Fatalf("shrinking fixture cleanup took %s, want longer than bound %s", elapsed, runtime.bound)
	}
}

func TestCustodianKeepsSeparateProofAndCleanupBudgets(t *testing.T) {
	owner, child := fixtureExact(700, 70).Ref(), fixtureExact(702, 72)
	key := FixtureKey{Owner: owner, Test: t.Name(), Nonce: "00000001"}
	child.Environ, child.EnvironKnown = []string{fixtureWord(t, key)}, true
	processes := fixtureTable{702: child}
	installFixtureScanTable(t, processes)
	prober := &custodianTable{fixtureTable: processes, ownerStates: []Liveness{Unknown, Unknown, Dead}}
	var signaled []int64
	runtime := custodianRuntime{prober: prober, poll: time.Millisecond, bound: 60 * time.Millisecond,
		clock: newManualCustodianClock(),
		sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		}}
	var log strings.Builder
	if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil || len(signaled) != 1 || signaled[0] != 702 ||
		!strings.Contains(log.String(), "action=complete") {
		t.Fatalf("custodian signaled=%v log=%q err=%v; want a complete cleanup after proof", signaled, log.String(), err)
	}
}
