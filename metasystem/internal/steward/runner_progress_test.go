package steward

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func recordProgress(t *testing.T, writes *[]RunnerProgress) func(string, string) error {
	t.Helper()
	return func(path, contents string) error {
		var record RunnerProgress
		if err := json.Unmarshal([]byte(contents), &record); err != nil {
			t.Fatal(err)
		}
		*writes = append(*writes, record)
		return atomicfile.WriteVolatile(path, contents)
	}
}

func readProgress(t *testing.T, root string) RunnerProgress {
	t.Helper()
	var record RunnerProgress
	if err := readJSON(runnerProgressPath(root), &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func progressProbe(at time.Time) (identity.Exact, identity.Prober) {
	self := identity.Exact{Pid: int64(os.Getpid()), StartedAt: at.Add(-time.Hour), StartTicks: 1234, BootID: "fixture-boot"}
	return self, censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid == self.Pid {
			return self, identity.Alive, nil
		}
		return identity.Exact{}, identity.Dead, nil
	})
}

func TestTheRunnerPublishesItsTickAndItsWait(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	start := time.Date(2026, 11, 16, 19, 0, 0, 0, time.UTC)
	now := start
	self, prober := progressProbe(start)
	if err := MintIdentity(RepoIdentityPath(loop.root), InstallIdentity{RepoIdentity: loop.root, Generation: 4}); err != nil {
		t.Fatal(err)
	}
	var writes []RunnerProgress
	deps := idleRunnerDependencies(&now)
	deps.Self, deps.progressWrite = prober, recordProgress(t, &writes)
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if !now.Before(start.Add(time.Second)) {
			loop.stop(t)
		}
	}
	deps.Tick = func(root string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
		var runner RunnerRecord
		if err := readJSON(runnerRecordPath(root), &runner); err != nil || runner.Capable != 1 || cfg.Progress == nil {
			t.Fatalf("runner=%+v progress=%p err=%v", runner, cfg.Progress, err)
		}
		now = now.Add(400 * time.Millisecond)
		return TickResult{}, nil
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, TickConfig{Now: start, ProviderHome: testprovider.Home(loop.root)}, deps); err != nil {
		t.Fatal(err)
	}
	var tickAndWait []RunnerProgress
	for _, record := range writes {
		if record.Operation == "tick" || record.Operation == "wait" {
			tickAndWait = append(tickAndWait, record)
		}
	}
	writes = tickAndWait
	if len(writes) != 5 {
		t.Fatalf("writes=%+v", writes)
	}
	begin, end, wait, cycle := writes[0], writes[1], writes[3], writes[4]
	if begin.Operation != "tick" || begin.State != "working" || begin.Done != 0 || !begin.ProgressAt.Equal(start) ||
		begin.Pid != self.Pid || begin.PidStartedAt != self.StartedAt.Unix() || begin.StartTicks != self.StartTicks ||
		begin.BootID != self.BootID || begin.Generation != 4 || begin.Capable != 1 || !begin.OperationSince.Equal(start) {
		t.Fatalf("tick begin=%+v", begin)
	}
	if end.Operation != "tick" || end.Done != 1 || end.OKTicks != 1 || end.FailedTicks != 0 {
		t.Fatalf("tick end=%+v", end)
	}
	if writes[2].Operation != "wait" || writes[2].State != "working" || writes[2].Done != 0 ||
		wait.Operation != "wait" || wait.State != "waiting" || wait.Reason != "idle" || wait.Deadline == nil || !wait.Deadline.Equal(start.Add(time.Second)) {
		t.Fatalf("wait=%+v, %+v", writes[2], wait)
	}
	if cycle.Cycle != 1 || !cycle.At.Equal(start.Add(time.Second)) || !cycle.ProgressAt.Equal(wait.ProgressAt) {
		t.Fatalf("cycle=%+v", cycle)
	}
	if _, err := os.Stat(runnerRecordPath(loop.root)); !os.IsNotExist(err) {
		t.Fatalf("runner record survived exit: %v", err)
	}
	if got := readProgress(t, loop.root); got.Cycle != 1 {
		t.Fatalf("progress did not survive exit: %+v", got)
	}
}

func TestTheRunnerCountsWhatTheTickReturns(t *testing.T) {
	t.Parallel()
	t.Run("consecutive errors", func(t *testing.T) {
		t.Parallel()
		loop := newHelmLoop(t)
		now := claimQuotaClock
		_, prober := progressProbe(now)
		var writes []RunnerProgress
		deps := idleRunnerDependencies(&now)
		deps.Self, deps.progressWrite = prober, recordProgress(t, &writes)
		tickErr := errors.New("tick failed")
		errors := []error{tickErr, tickErr, nil, tickErr}
		ticks := 0
		deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
			ticks++
			now = now.Add(time.Second)
			if ticks == len(errors) {
				loop.stop(t)
			}
			return TickResult{}, errors[ticks-1]
		}
		if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 2*time.Second, TickConfig{Now: now, ProviderHome: testprovider.Home(loop.root)}, deps); err != nil {
			t.Fatal(err)
		}
		var completions []RunnerProgress
		for _, record := range writes {
			if record.Operation == "tick" && record.Done == 1 {
				completions = append(completions, record)
			}
		}
		if len(completions) != 4 {
			t.Fatalf("tick completions=%+v", completions)
		}
		for i, want := range [][2]int{{1, 0}, {2, 0}, {0, 1}, {1, 1}} {
			got := completions[i]
			if got.FailedTicks != want[0] || got.OKTicks != want[1] || !got.ProgressAt.Equal(claimQuotaClock.Add(time.Duration(2*i+1)*time.Second)) {
				t.Fatalf("tick %d=%+v, want failed/ok=%v", i+1, got, want)
			}
		}
	})
	t.Run("report fails after complete stamp", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil)
		testprovider.Register(t, root)
		loop := &helmLoop{root: canonicalPath(root)}
		now := claimQuotaClock
		_, prober := progressProbe(now)
		deps := idleRunnerDependencies(&now)
		deps.Self, deps.Tick = prober, RunTick
		reports := 0
		cfg := TickConfig{Now: now, Prober: prober, ProviderHome: testprovider.Home(root), Patterns: func(root string, _ time.Time) error {
			reports++
			held, err := TryAcquireArbitration(root)
			if held != nil {
				held.Release()
			}
			if !errors.Is(err, ErrArbitrationHeld) {
				t.Fatalf("tick report does not hold arbitration: %v", err)
			}
			if got := readProgress(t, root); got.Operation != "tick:reports" || got.State != "working" || got.Done != 0 {
				t.Fatalf("progress under arbitration=%+v", got)
			}
			loop.stop(t)
			return errors.New("report failed")
		}}
		if err := runLoopWithDependencies(root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
			t.Fatal(err)
		}
		evidence, err := os.ReadFile(ComponentEvidencePath(root, "steward-tick"))
		if err != nil || !strings.Contains(strings.ReplaceAll(string(evidence), " ", ""), `"outcome":"PASS_COMPLETE"`) {
			t.Fatalf("tick evidence=%s err=%v", evidence, err)
		}
		if got := readProgress(t, root); reports != 1 || got.FailedTicks != 1 || got.OKTicks != 0 {
			t.Fatalf("reports=%d progress=%+v", reports, got)
		}
	})
}

func TestRewritingTheRecordWithoutChangeKeepsProgressAt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	start := claimQuotaClock
	now := start
	self, _ := progressProbe(start)
	var writes []RunnerProgress
	p := newProgress(root, self, 4, func() time.Time { return now }, recordProgress(t, &writes))
	calls := []func(){func() { p.Begin("a") }, func() { p.ticked(nil) },
		func() { p.Wait("idle", time.Minute) }, func() { p.Wait("idle", time.Minute) }, p.cycled, p.Item, func() { p.Begin("b") }}
	for i, call := range calls {
		now = start.Add(time.Duration(i) * time.Second)
		call()
		got := writes[i]
		wantProgress := start.Add(time.Duration([]int{0, 1, 2, 2, 2, 5, 6}[i]) * time.Second)
		if !got.At.Equal(now) || !got.ProgressAt.Equal(wantProgress) {
			t.Fatalf("write %d=%+v, want progress at %s", i, got, wantProgress)
		}
	}
	if got := writes[5]; got.State != "waiting" || got.Reason != "idle" || got.Done != 2 {
		t.Fatalf("item changed the wait: %+v", got)
	}
	if got := writes[6]; got.Done != 0 || got.State != "working" || got.Reason != "" || got.Deadline != nil || !got.OperationSince.Equal(now) {
		t.Fatalf("begin did not reset the operation: %+v", got)
	}
	for _, reason := range []string{"arbitration", "external"} {
		p.Wait(reason, 0)
		if got := readProgress(t, root); got.Reason != reason || got.Deadline != nil {
			t.Fatalf("unbounded wait=%+v", got)
		}
		p.Working()
		if got := readProgress(t, root); got.State != "working" || got.Reason != "" || got.Deadline != nil {
			t.Fatalf("working=%+v", got)
		}
	}
	p.Wait("external", time.Minute)
	if got := readProgress(t, root); got.Deadline == nil || !got.Deadline.Equal(now.Add(time.Minute)) {
		t.Fatalf("bounded external wait=%+v", got)
	}
	written := len(writes)
	var silent *Progress
	silent.Begin("ignored")
	silent.Item()
	silent.Wait("external", time.Second)
	silent.Working()
	silent.ticked(errors.New("ignored"))
	silent.cycled()
	if len(writes) != written {
		t.Fatal("nil publisher wrote progress")
	}
}

func TestProgressWriteFailuresRecoverAndReportOnce(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_PROGRESS_WRITE_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProgressWriteFailuresRecoverAndReportOnce$")
		cmd.Env = append(os.Environ(), "METASYSTEM_PROGRESS_WRITE_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil || strings.Count(string(out), "publish steward runner progress:") != 2 {
			t.Fatalf("write failure reports: %v\n%s", err, out)
		}
		return
	}
	root, calls := t.TempDir(), 0
	self, _ := progressProbe(claimQuotaClock)
	p := newProgress(root, self, 4, func() time.Time { return claimQuotaClock }, func(path, contents string) error {
		calls++
		if calls%3 != 0 {
			return errors.New("writer unavailable")
		}
		return atomicfile.WriteVolatile(path, contents)
	})
	p.Begin("a")
	for range 5 {
		p.Item()
	}
	if got := readProgress(t, root); calls != 6 || got.Done != 5 {
		t.Fatalf("write failure lost progress: calls=%d record=%+v", calls, got)
	}
}
