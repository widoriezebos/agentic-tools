package steward

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// runCurrentEngine makes the recorded runner a process of the enrolled engine.
func (b *executionBed) runCurrentEngine(t *testing.T) identity.Exact {
	t.Helper()
	exact := b.old
	exact.Exe = b.replacement.Exe
	b.processes[exact.Pid] = exact
	b.record(t, exact)
	return exact
}

// publishProgress writes the recorded runner's own progress as of the bed's clock.
func (b *executionBed) publishProgress(t *testing.T, change func(*RunnerProgress)) {
	t.Helper()
	record := RunnerProgress{
		Pid: b.old.Pid, PidStartedAt: b.old.StartedAt.Unix(), Generation: 148, Capable: 1,
		Operation: "tick", OperationSince: b.now, State: "working", ProgressAt: b.now, At: b.now,
	}
	change(&record)
	if err := writeJSONAtomic(runnerProgressPath(b.root), record); err != nil {
		t.Fatal(err)
	}
}

func (b *executionBed) endAutoHealing(t *testing.T) {
	t.Helper()
	if err := saveHealthRecord(b.root, HealthRecordPath(b.root), healthRecord{
		State:   HealthObservationState{Sequence: 5, ObservedAt: b.now, UnknownCounts: map[HealthRole]int{}, FailureCounts: map[HealthRole]int{RoleStewardRunner: healthFailureLimit}},
		Verdict: HealthVerdict{Schema: 1, ObservedAt: b.now, Observation: 5, Aggregate: "unhealthy"},
	}); err != nil {
		t.Fatal(err)
	}
}

func (b *executionBed) verdict() RoleVerdict {
	return checkStewardRunnerObserved(b.root, b.now, b.clock, b.processes)
}

// assertLeftAlone runs all three entries that could replace the runner.
func (b *replacementBed) assertLeftAlone(t *testing.T) {
	t.Helper()
	noSleep := func(d time.Duration) { t.Fatalf("the entry waited %s", d) }
	if got, err := b.step(t); err != nil || got.Status != "CURRENT" || got.ReplacementPid != b.old.Pid {
		t.Fatalf("step: %+v %v", got, err)
	}
	if got, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, noSleep, b.deps); err != nil || got.Status != "CURRENT" {
		t.Fatalf("blocking entry: %+v %v", got, err)
	}
	if got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, noSleep); err != nil || got.Action != "verified" || got.Pid != b.old.Pid {
		t.Fatalf("ensure: %+v %v", got, err)
	}
	b.assertUntouched(t)
}

func (b *replacementBed) assertUntouched(t *testing.T) {
	t.Helper()
	if len(b.signals) != 0 || b.launches != 0 {
		t.Fatalf("signals=%v launches=%d", b.signals, b.launches)
	}
	noReplacement(t, b.root)
}

func (b *replacementBed) assertReplacing(t *testing.T, got RunnerRepairOutcome, err error, cause string) {
	t.Helper()
	if err != nil || got.Status != RunnerRepairStopping || got.PreviousPid != b.old.Pid {
		t.Fatalf("step: %+v %v", got, err)
	}
	pending, readErr := readRunnerReplacement(b.root)
	if readErr != nil || pending == nil || pending.Cause != cause {
		t.Fatalf("transition: %+v %v, want cause %s", pending, readErr, cause)
	}
	stops := 0
	for _, signal := range b.signals {
		if signal == syscall.SIGTERM {
			stops++
		}
	}
	if stops != 1 || b.launches != 0 {
		t.Fatalf("signals=%v launches=%d", b.signals, b.launches)
	}
}

func TestTheIncidentRunnerIsLeftAlone(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	b.runCurrentEngine(t)
	attempt, err := loadComponentEvidence(ComponentEvidencePath(b.root, "steward-tick"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeComponentAttempt(b.root, "steward-tick", 148, attempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "fixture", nil, b.now); err != nil {
		t.Fatal(err)
	}
	// The old window closed at twenty minutes. Items finish far apart, so the
	// runner is also quiet at the later looks, and quiet replaces nothing.
	start, progressed := b.now, b.now
	for done, elapsed := range []time.Duration{21 * time.Minute, 58 * time.Minute, 94 * time.Minute} {
		b.now = start.Add(elapsed)
		b.publishProgress(t, func(r *RunnerProgress) { r.OperationSince, r.ProgressAt, r.Done = start, progressed, done })
		b.assertLeftAlone(t)
		progressed = b.now
	}
}

func TestAQuietRunnerIsNotReplaced(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*RunnerProgress)
	}{
		{"working", func(*RunnerProgress) {}},
		{"waiting for arbitration", func(r *RunnerProgress) { r.State, r.Reason = "waiting", "arbitration" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			b.runCurrentEngine(t)
			b.publishProgress(t, func(r *RunnerProgress) {
				r.OperationSince, r.ProgressAt, r.At = b.now.Add(-2*time.Hour), b.now.Add(-2*time.Hour), b.now.Add(-time.Minute)
				test.change(r)
			})
			b.assertLeftAlone(t)
			if got := b.verdict(); got.Status != HealthAlive || !strings.Contains(got.Reason, "quiet for 2h0m0s") {
				t.Fatalf("verdict: %+v", got)
			}
		})
	}
}

func TestAFailingRunnerIsReplacedThroughTheTransition(t *testing.T) {
	t.Parallel()
	t.Run("three failed ticks", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		b.runCurrentEngine(t)
		b.publishProgress(t, func(r *RunnerProgress) { r.FailedTicks = 3 })
		if got := b.verdict(); got.Status != HealthDead || !strings.Contains(got.Reason, "failed its last 3 ticks") {
			t.Fatalf("verdict: %+v", got)
		}
		got, err := b.step(t)
		b.assertReplacing(t, got, err, "failing")
		delete(b.processes, b.old.Pid)
		if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 {
			t.Fatalf("launch: %+v %v launches=%d", got, err, b.launches)
		}
		if got, err := b.step(t); err != nil || got.Status != "RESTORED" || got.ReplacementPid != b.replacement.Pid {
			t.Fatalf("announcement: %+v %v", got, err)
		}
		// The old process's failures are still on disk and are not the new one's.
		if got := runnerLiveness(b.root, b.now, b.clock, b.processes); got.Reading != RunnerStarting {
			t.Fatalf("new process: %+v", got)
		}
		if got, err := b.step(t); err != nil || got.Status != "CURRENT" || got.ReplacementPid != b.replacement.Pid || b.launches != 1 {
			t.Fatalf("after the replacement: %+v %v launches=%d", got, err, b.launches)
		}
	})
	t.Run("ensure replaces a failing runner", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		b.runCurrentEngine(t)
		b.publishProgress(t, func(r *RunnerProgress) { r.FailedTicks = 3 })
		got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, func(d time.Duration) {
			b.sleep(d)
			delete(b.processes, b.old.Pid)
			b.record(t, b.replacement)
		})
		stops := 0
		for _, signal := range b.signals {
			if signal == syscall.SIGTERM {
				stops++
			}
		}
		if err != nil || got.Action != "replaced" || got.Pid != b.replacement.Pid || stops != 1 {
			t.Fatalf("ensure: %+v %v signals=%v", got, err, b.signals)
		}
	})
	t.Run("two failed ticks", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		b.runCurrentEngine(t)
		b.publishProgress(t, func(r *RunnerProgress) { r.FailedTicks = 2 })
		if got := b.verdict(); got.Status != HealthAlive {
			t.Fatalf("verdict: %+v", got)
		}
		b.assertLeftAlone(t)
	})
}

func TestAuthorityDecidesBeforeTheReading(t *testing.T) {
	t.Parallel()
	working := func(*RunnerProgress) {}
	failing := func(r *RunnerProgress) { r.FailedTicks = 3 }
	obsolete := func(t *testing.T, b *replacementBed) { b.publishProgress(t, working) }
	current := func(change func(*RunnerProgress)) func(*testing.T, *replacementBed) {
		return func(t *testing.T, b *replacementBed) { b.runCurrentEngine(t); b.publishProgress(t, change) }
	}
	dead := func(t *testing.T, b *replacementBed) {
		current(working)(t, b)
		delete(b.processes, b.old.Pid)
	}
	for _, test := range []struct {
		name    string
		arrange func(*testing.T, *replacementBed)
		ended   bool
		status  string // empty when the step must fail
		failure string
	}{
		{name: "an obsolete engine, progressing", arrange: obsolete, status: RunnerRepairStopping},
		{name: "an obsolete engine, breaker ended", arrange: obsolete, ended: true, status: RunnerRepairStopping},
		{name: "current and progressing, breaker ended", arrange: current(working), ended: true, status: AutoHealEnded},
		{name: "dead, breaker ended", arrange: dead, ended: true, status: AutoHealEnded},
		{name: "failing, breaker ended", arrange: current(failing), ended: true, status: AutoHealEnded},
		{name: "unknown execution", arrange: func(t *testing.T, b *replacementBed) {
			exact := b.runCurrentEngine(t)
			exact.ExeKnown = false
			b.processes[exact.Pid] = exact
			b.publishProgress(t, failing)
		}, failure: "execution is unknown"},
		{name: "a process that cannot be inspected", arrange: func(t *testing.T, b *replacementBed) {
			current(failing)(t, b)
			b.deps.prober = censusSeamProbe(func(int64) (identity.Exact, identity.Liveness, error) {
				return identity.Exact{}, identity.Unknown, errors.New("unreadable")
			})
		}, failure: "steward freshness is unknown: recorded runner pid 41 cannot be inspected"},
		{name: "a changed enrollment", arrange: func(t *testing.T, b *replacementBed) {
			current(failing)(t, b)
			installed := b.pinned.Install
			installed.Generation++
			if err := MintIdentity(RepoIdentityPath(b.root), installed); err != nil {
				t.Fatal(err)
			}
		}, status: "ENROLLMENT_CHANGED"},
		{name: "a closed fence", arrange: func(t *testing.T, b *replacementBed) {
			current(failing)(t, b)
			closeProcessFence(t, b.root, 148)
		}, status: "FENCED"},
		{name: "excluded, breaker ended", arrange: func(t *testing.T, b *replacementBed) {
			current(failing)(t, b)
			b.deps.runnerExcluded = func(string, bool) (string, bool) { return "fixture", true }
		}, ended: true, status: "EXCLUDED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			test.arrange(t, b)
			if test.ended {
				b.endAutoHealing(t)
			}
			got, err := b.step(t)
			switch {
			case test.status == RunnerRepairStopping:
				b.assertReplacing(t, got, err, "obsolete-engine")
				return
			case test.failure != "":
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("step: %+v %v", got, err)
				}
			case err != nil || got.Status != test.status:
				t.Fatalf("step: %+v %v, want %s", got, err, test.status)
			}
			b.assertUntouched(t)
		})
	}
}

func TestARunnerThatDoesNotPublishIsNotJudged(t *testing.T) {
	t.Parallel()
	silent := func(t *testing.T, b *replacementBed, exact identity.Exact) {
		t.Helper()
		if err := writeJSONAtomic(runnerRecordPath(b.root), RunnerRecord{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix()}); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("the enrolled engine", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		silent(t, b, b.runCurrentEngine(t))
		if got, err := b.step(t); err == nil || !strings.Contains(err.Error(), "does not publish its progress") {
			t.Fatalf("step: %+v %v", got, err)
		}
		noSleep := func(d time.Duration) { t.Fatalf("ensure waited %s", d) }
		if got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, noSleep); err == nil || !strings.Contains(err.Error(), "cannot be verified") {
			t.Fatalf("ensure: %+v %v", got, err)
		}
		b.assertUntouched(t)
	})
	t.Run("an obsolete engine", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		silent(t, b, b.old)
		got, err := b.step(t)
		b.assertReplacing(t, got, err, "obsolete-engine")
	})
}

func TestARunnerReadsItselfAliveInItsOwnHealthPass(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, nil)
	testprovider.Register(t, root)
	loop := &helmLoop{root: canonicalPath(root)}
	// The tick's time stays behind every record the runner publishes.
	tickTime := claimQuotaClock
	now := tickTime.Add(2 * time.Hour)
	_, prober := progressProbe(tickTime)
	deps := idleRunnerDependencies(&now)
	deps.Self = prober
	var result TickResult
	deps.Tick = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		loop.stop(t)
		var err error
		result, err = RunTick(root, cfg, census)
		return result, err
	}
	cfg := TickConfig{Now: tickTime, Prober: prober, ProviderHome: testprovider.Home(root),
		Patterns: func(string, time.Time) error { return nil }}
	if err := runLoopWithDependencies(root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	for _, role := range result.Health.Roles {
		if role.Role == RoleStewardRunner {
			if role.Status != HealthAlive || !strings.Contains(role.Reason, "working") {
				t.Fatalf("role: %+v", role)
			}
			return
		}
	}
	t.Fatalf("no steward runner role in %+v", result.Health.Roles)
}

func TestAnExitedUnreapedRunnerIsReplaced(t *testing.T) {
	t.Parallel()
	exited := func(t *testing.T) *replacementBed {
		t.Helper()
		b := newReplacementBed(t)
		exact := b.runCurrentEngine(t)
		b.publishProgress(t, func(*RunnerProgress) {})
		exact.Zombie = true
		b.processes[exact.Pid] = exact
		b.startWithoutAnnouncement(t)
		return b
	}
	t.Run("the step launches without a signal", func(t *testing.T) {
		t.Parallel()
		b := exited(t)
		if got := b.verdict(); got.Status != HealthDead || !strings.Contains(got.Reason, "is dead") {
			t.Fatalf("verdict: %+v", got)
		}
		if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 || len(b.signals) != 0 {
			t.Fatalf("step: %+v %v launches=%d signals=%v", got, err, b.launches, b.signals)
		}
		pending, err := readRunnerReplacement(b.root)
		if err != nil || pending == nil || pending.Cause != "dead" {
			t.Fatalf("transition: %+v %v", pending, err)
		}
	})
	t.Run("ensure starts a new runner", func(t *testing.T) {
		t.Parallel()
		b := exited(t)
		got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, func(d time.Duration) {
			b.sleep(d)
			b.record(t, b.replacement)
		})
		if err != nil || got.Action != "started" || got.Pid != b.replacement.Pid || len(b.signals) != 0 {
			t.Fatalf("ensure: %+v %v signals=%v", got, err, b.signals)
		}
	})
	t.Run("a live process with an unreadable executable is refused", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		exact := b.runCurrentEngine(t)
		exact.ExeKnown = false
		b.processes[exact.Pid] = exact
		if got, err := b.step(t); err == nil || !strings.Contains(err.Error(), "execution is unknown") {
			t.Fatalf("step: %+v %v", got, err)
		}
		noSleep := func(d time.Duration) { t.Fatalf("ensure waited %s", d) }
		if got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, noSleep); err == nil || !strings.Contains(err.Error(), "execution is unknown") {
			t.Fatalf("ensure: %+v %v", got, err)
		}
		b.assertUntouched(t)
	})
}

func TestASlowObservationDoesNotHideAFailingRunner(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	b.runCurrentEngine(t)
	start := b.now
	// The runner published an hour after the step read its time.
	b.publishProgress(t, func(r *RunnerProgress) { r.FailedTicks, r.At = 3, start.Add(time.Hour) })
	reads := 0
	clock := func() time.Time {
		if reads++; reads == 1 {
			return start
		}
		return start.Add(2 * time.Hour)
	}
	got, err := repairPinnedStep(b.root, b.pinned, clock, b.deps)
	b.assertReplacing(t, got, err, "failing")
	if pending, readErr := readRunnerReplacement(b.root); readErr != nil || !pending.PhaseSince.Equal(start) {
		t.Fatalf("transition: %+v %v", pending, readErr)
	}
}

func TestAClockThatMovedBackReadsUnknown(t *testing.T) {
	t.Parallel()
	noon := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	clock := func(hour, minute int) time.Time { return time.Date(2026, 8, 24, hour, minute, 0, 0, time.UTC) }
	for _, test := range []struct {
		name     string
		at       time.Time
		failed   int
		observed time.Time
		reading  RunnerReading
	}{
		{"behind a failing record", clock(11, 59), 3, clock(11, 0), RunnerUnknown},
		{"behind the caller, after the record", clock(11, 1), 0, clock(11, 2), RunnerUnknown},
		{"a slow reader on a clock that only moved forward", clock(11, 59), 0, clock(14, 0), RunnerProgressing},
		{"a slow reader of a failing runner", clock(11, 59), 3, clock(14, 0), RunnerFailing},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newLivenessBed(t)
			b.now = noon
			record := b.working()
			record.At, record.ProgressAt, record.FailedTicks = test.at, test.at, test.failed
			b.publish(record)
			reads := 0
			got := runnerLiveness(b.root, noon, func() time.Time { reads++; return test.observed }, b.probe)
			if got.Reading != test.reading || reads != 1 {
				t.Fatalf("report=%+v reads=%d, want %s", got, reads, test.reading)
			}
			if test.reading == RunnerUnknown && (got.Cause != CauseClockRegressed || got.Quiet) {
				t.Fatalf("report=%+v", got)
			}
		})
	}
}
