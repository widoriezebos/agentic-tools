package identity

import (
	"fmt"
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

type custodianRecordProber struct {
	fixtureTable
	diesOnLaterProbe int64
	probes           int
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
	ownerUntil time.Time
	ownerState Liveness
}

func (table *custodianTable) Probe(pid int64) (Exact, Liveness, error) {
	if pid == 700 {
		if time.Now().Before(table.ownerUntil) {
			if table.ownerState == Alive {
				return table.fixtureTable.Probe(pid)
			}
			return Exact{}, table.ownerState, nil
		}
		return Exact{}, Dead, nil
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
	table := &custodianTable{fixtureTable: processes, ownerUntil: time.Now().Add(40 * time.Millisecond), ownerState: Alive}
	runtime.prober, runtime.bound = table, 30*time.Millisecond
	if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil || len(signaled) != 2 || signaled[0] != 702 || signaled[1] != 703 {
		t.Fatalf("custodian signaled=%v err=%v; want children 702 and 703 after owner death, never self 701", signaled, err)
	}
	if !strings.Contains(log.String(), "pid=702 carrier=environment") || !strings.Contains(log.String(), "pid=703 carrier=argv-word") {
		t.Fatalf("custodian log %q does not name each child's carrier", log.String())
	}
}

func TestCustodianHardHaltBoundsBlockedScan(t *testing.T) {
	release, halted, done := make(chan struct{}), make(chan int, 1), make(chan error, 1)
	oldPids := survivorPids
	survivorPids = func() ([]int64, error) { <-release; return nil, nil }
	runtime := custodianRuntime{prober: fixtureTable{}, poll: time.Millisecond, bound: 5 * time.Millisecond,
		halt: func(code int) { halted <- code }}
	var log strings.Builder
	go func() { done <- reapDeadOwner(fixtureExact(700, 70).Ref(), &log, runtime) }()
	t.Cleanup(func() { close(release); <-done; survivorPids = oldPids })
	select {
	case code := <-halted:
		if code != 2 {
			t.Fatalf("hard halt exit=%d, want 2", code)
		}
		want := fmt.Sprintf("fixture-custodian action=halt after=%s\n", runtime.bound+custodianHaltMargin)
		if !strings.Contains(log.String(), want) {
			t.Fatalf("hard halt log=%q, want %q", log.String(), want)
		}
	case <-time.After(runtime.bound + custodianHaltMargin + 250*time.Millisecond):
		t.Fatal("blocked fixture scan was not hard-halted")
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
	halted := make(chan int, 1)
	runtime := custodianRuntime{
		prober: processes, records: records, poll: time.Millisecond, bound: 5 * time.Millisecond,
		halt: func(code int) { halted <- code },
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			time.Sleep(600 * time.Millisecond)
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
	select {
	case code := <-halted:
		t.Fatalf("slow fixture scans hard-halted with code %d: log=%q", code, log.String())
	default:
	}
}

func TestCustodianCleanupBoundRequiresProgress(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	processes := fixtureTable{}
	nextPid := int64(800)
	runtime := custodianRuntime{
		prober: processes, poll: time.Millisecond, bound: 5 * time.Millisecond,
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
	started := time.Now()
	err := reapDeadOwner(owner, io.Discard, runtime)
	elapsed := time.Since(started)
	if err == nil || !strings.Contains(err.Error(), "fixture custodian cleanup exceeded") {
		t.Fatalf("respawning fixture cleanup error=%v after %s", err, elapsed)
	}
	if elapsed < runtime.bound || elapsed >= custodianHaltMargin {
		t.Fatalf("respawning fixture cleanup took %s, want at least %s and less than %s", elapsed, runtime.bound, custodianHaltMargin)
	}
}

func TestCustodianCleanupBoundStopsARespawningSet(t *testing.T) {
	owner, survivor := fixtureExact(700, 70).Ref(), fixtureExact(801, 81)
	processes := fixtureTable{}
	records := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(records, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	forceReturn := make(chan struct{})
	defer close(forceReturn)
	pass := 0
	runtime := custodianRuntime{
		prober: processes, records: records, poll: time.Millisecond, bound: 5 * time.Millisecond,
		scan: func(Prober, Ref) ([]FixtureSurvivor, error) {
			pass++
			select {
			case <-forceReturn:
			default:
				if pass%2 == 0 {
					return nil, nil
				}
			}
			processes[survivor.Pid] = survivor
			return []FixtureSurvivor{{Class: FixtureSurvivorCertain, Ref: survivor.Ref(), Carrier: FixtureCarrierEnvironment}}, nil
		},
		sender: func(pid int, _ syscall.Signal) error {
			delete(processes, int64(pid))
			return nil
		},
	}
	started := time.Now()
	done := make(chan error, 1)
	go func() { done <- reapDeadOwner(owner, io.Discard, runtime) }()
	select {
	case err := <-done:
		elapsed := time.Since(started)
		if err == nil || !strings.Contains(err.Error(), "fixture custodian cleanup exceeded") || elapsed < runtime.bound {
			t.Fatalf("respawning fixture cleanup error=%v after %s", err, elapsed)
		}
		if _, err := os.Stat(records); err != nil {
			t.Fatalf("respawning fixture cleanup removed records: %v", err)
		}
	case <-time.After(20 * runtime.bound):
		t.Fatalf("respawning fixture cleanup did not stop within %s", 20*runtime.bound)
	}
}

func TestCustodianCleanupBoundAllowsSlowProgress(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	processes := fixtureTable{}
	for pid := int64(800); pid < 810; pid++ {
		processes[pid] = fixtureExact(pid, pid)
	}
	removedThisPass := false
	runtime := custodianRuntime{
		prober: processes, poll: time.Millisecond, bound: 5 * time.Millisecond,
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
	started := time.Now()
	if err := reapDeadOwner(owner, io.Discard, runtime); err != nil {
		t.Fatalf("shrinking fixture cleanup returned %v with %d survivors", err, len(processes))
	}
	if elapsed := time.Since(started); elapsed <= runtime.bound {
		t.Fatalf("shrinking fixture cleanup took %s, want longer than bound %s", elapsed, runtime.bound)
	}
}

func TestCustodianKeepsSeparateProofAndCleanupBudgets(t *testing.T) {
	owner, child := fixtureExact(700, 70).Ref(), fixtureExact(702, 72)
	key := FixtureKey{Owner: owner, Test: t.Name(), Nonce: "00000001"}
	child.Environ, child.EnvironKnown = []string{fixtureWord(t, key)}, true
	processes := fixtureTable{702: child}
	installFixtureScanTable(t, processes)
	prober := &custodianTable{fixtureTable: processes, ownerUntil: time.Now().Add(45 * time.Millisecond), ownerState: Unknown}
	var signaled []int64
	runtime := custodianRuntime{prober: prober, poll: time.Millisecond, bound: 60 * time.Millisecond,
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
