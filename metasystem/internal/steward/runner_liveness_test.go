package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

const livenessPid, livenessGeneration = int64(44300), 4

// livenessBed is an enrolled checkout with a recorded, publishing runner and a
// process table the test controls.
type livenessBed struct {
	t      *testing.T
	root   string
	now    time.Time
	runner RunnerRecord
	probe  healthProbe
}

func newLivenessBed(t *testing.T) *livenessBed {
	t.Helper()
	b := &livenessBed{t: t, root: canonicalPath(t.TempDir()), now: claimQuotaClock, probe: healthProbe{}}
	b.runner = RunnerRecord{Pid: livenessPid, PidStartedAt: 100, StartTicks: 900, BootID: "boot-a", Capable: 1}
	b.setProcess(b.exact(), identity.Alive)
	b.conf("")
	enrollForLiveness(t, b.root)
	b.writeRunner()
	return b
}

func (b *livenessBed) exact() identity.Exact {
	return identity.Exact{Pid: b.runner.Pid, StartedAt: time.Unix(b.runner.PidStartedAt, 0), StartTicks: b.runner.StartTicks, BootID: b.runner.BootID}
}

func (b *livenessBed) setProcess(exact identity.Exact, state identity.Liveness) {
	b.probe[livenessPid] = struct {
		exact identity.Exact
		state identity.Liveness
		err   error
	}{exact: exact, state: state}
}

func (b *livenessBed) conf(contents string) {
	b.t.Helper()
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), []byte(contents), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *livenessBed) writeRunner() {
	b.t.Helper()
	if err := writeJSONAtomic(runnerRecordPath(b.root), b.runner); err != nil {
		b.t.Fatal(err)
	}
}

func (b *livenessBed) publish(record RunnerProgress) {
	b.t.Helper()
	if err := writeJSONAtomic(runnerProgressPath(b.root), record); err != nil {
		b.t.Fatal(err)
	}
}

// working is this runner's record: two minutes into a tick, three items done,
// the last of them a minute ago.
func (b *livenessBed) working() RunnerProgress {
	return RunnerProgress{
		Pid: b.runner.Pid, PidStartedAt: b.runner.PidStartedAt, StartTicks: b.runner.StartTicks, BootID: b.runner.BootID,
		Generation: livenessGeneration, Capable: 1, Operation: "tick", OperationSince: b.now.Add(-2 * time.Minute),
		State: "working", Done: 3, ProgressAt: b.now.Add(-time.Minute), At: b.now.Add(-time.Minute),
	}
}

func (b *livenessBed) waiting(reason string) RunnerProgress {
	record := b.working()
	record.Operation, record.State, record.Reason, record.Done = "wait", "waiting", reason, 0
	return record
}

func (b *livenessBed) read(now time.Time) RunnerLivenessReport {
	return runnerLiveness(b.root, now, func() time.Time { return now }, b.probe)
}

func TestRunnerLivenessReadsEachCase(t *testing.T) {
	t.Parallel()
	pid := fmt.Sprintf("pid %d", livenessPid)
	starting := "runner " + pid + " has started and has not published yet"
	unreadableProgress := "the runner progress record is unreadable"
	publish := func(change func(*RunnerProgress)) func(*livenessBed) {
		return func(b *livenessBed) {
			record := b.working()
			change(&record)
			b.publish(record)
		}
	}
	// foreign is a failing record that differs from the runner in one field.
	foreign := func(change func(*RunnerProgress)) func(*livenessBed) {
		return publish(func(r *RunnerProgress) { r.FailedTicks = 3; change(r) })
	}
	torn := func(path func(string) string) func(*livenessBed) {
		return func(b *livenessBed) {
			if err := os.WriteFile(path(b.root), []byte("{torn"), 0o644); err != nil {
				b.t.Fatal(err)
			}
		}
	}
	withConf := func(conf string, change func(*RunnerProgress)) func(*livenessBed) {
		return func(b *livenessBed) { b.conf(conf); publish(change)(b) }
	}
	asProcess := func(state identity.Liveness, change func(*identity.Exact)) func(*livenessBed) {
		return func(b *livenessBed) {
			exact := b.exact()
			change(&exact)
			b.setProcess(exact, state)
			publish(func(*RunnerProgress) {})(b)
		}
	}
	same := func(*RunnerProgress) {}
	for _, test := range []struct {
		name     string
		arrange  func(*livenessBed)
		observed time.Duration // after the bed's now
		reading  RunnerReading
		cause    RemedyCause
		detail   string // the exact sentence, or its prefix when prefix is set
		prefix   bool
		noPid    bool
		own      bool
		quiet    bool
	}{
		{name: "no runner recorded", arrange: func(b *livenessBed) { os.Remove(runnerRecordPath(b.root)) }, reading: RunnerDead, cause: CauseUnavailable, detail: "no steward runner is recorded", noPid: true},
		{name: "runner record unreadable", arrange: torn(runnerRecordPath), reading: RunnerUnknown, cause: CauseUnreadable, detail: "the steward runner record is unreadable", noPid: true},
		{name: "process gone", arrange: asProcess(identity.Dead, func(*identity.Exact) {}), reading: RunnerDead, cause: CauseUnavailable, detail: "recorded runner " + pid + " is dead"},
		{name: "pid reused", arrange: asProcess(identity.Alive, func(e *identity.Exact) { e.StartTicks++ }), reading: RunnerDead, cause: CauseUnavailable, detail: "recorded runner " + pid + " is dead"},
		{name: "zombie with a working record", arrange: asProcess(identity.Alive, func(e *identity.Exact) { e.Zombie = true }), reading: RunnerDead, cause: CauseUnavailable, detail: "recorded runner " + pid + " is dead"},
		{name: "process cannot be inspected", arrange: asProcess(identity.Unknown, func(*identity.Exact) {}), reading: RunnerUnknown, cause: CauseUnreadable, detail: "recorded runner " + pid + " cannot be inspected"},
		{name: "enrollment unreadable", arrange: func(b *livenessBed) { publish(same)(b); os.Remove(RepoIdentityPath(b.root)) }, reading: RunnerUnknown, cause: CauseUnreadable, detail: "the steward installation generation is unreadable"},
		{name: "runner does not publish", arrange: func(b *livenessBed) { b.runner.Capable = 0; b.writeRunner(); publish(same)(b) }, reading: RunnerUnknown, cause: CauseObservationPending, detail: "runner " + pid + " does not publish its progress"},
		{name: "failed ticks invalid", arrange: withConf("steward.runner-failed-ticks=0\n", same), reading: RunnerUnknown, cause: CauseSettingsInvalid, detail: "the steward runner settings are invalid: steward.runner-failed-ticks", prefix: true},
		{name: "quiet minutes invalid", arrange: withConf("steward.runner-quiet-min=soon\n", same), reading: RunnerUnknown, cause: CauseSettingsInvalid, detail: "the steward runner settings are invalid: steward.runner-quiet-min", prefix: true},
		{name: "quiet minutes past the bound", arrange: withConf(fmt.Sprintf("steward.runner-quiet-min=%d\n", maxRunnerQuietMinutes+1), same), reading: RunnerUnknown, cause: CauseSettingsInvalid, detail: "the steward runner settings are invalid: steward.runner-quiet-min must be no greater", prefix: true},
		{name: "progress unreadable", arrange: torn(runnerProgressPath), reading: RunnerUnknown, cause: CauseUnreadable, detail: unreadableProgress},
		{name: "no progress record", arrange: func(*livenessBed) {}, reading: RunnerStarting, detail: starting},
		{name: "another pid", arrange: foreign(func(r *RunnerProgress) { r.Pid++ }), reading: RunnerStarting, detail: starting},
		{name: "another start time", arrange: foreign(func(r *RunnerProgress) { r.PidStartedAt++ }), reading: RunnerStarting, detail: starting},
		{name: "other start ticks", arrange: foreign(func(r *RunnerProgress) { r.StartTicks++ }), reading: RunnerStarting, detail: starting},
		{name: "another boot", arrange: foreign(func(r *RunnerProgress) { r.BootID = "boot-b" }), reading: RunnerStarting, detail: starting},
		{name: "another generation", arrange: foreign(func(r *RunnerProgress) { r.Generation++ }), reading: RunnerStarting, detail: starting},
		{name: "record later than the observation", arrange: publish(func(r *RunnerProgress) { r.At = r.At.Add(2 * time.Minute) }), reading: RunnerUnknown, cause: CauseClockRegressed, detail: "CLOCK_REGRESSED: runner progress is later than current UTC", own: true},
		{name: "record later than a stale now only", arrange: publish(func(r *RunnerProgress) { r.At = r.At.Add(time.Hour) }), observed: 2 * time.Hour, reading: RunnerProgressing, detail: "tick (working, 3 done) for 2m0s", own: true},
		{name: "unknown state", arrange: publish(func(r *RunnerProgress) { r.State = "resting" }), reading: RunnerUnknown, cause: CauseUnreadable, detail: unreadableProgress, own: true},
		{name: "three failed ticks", arrange: publish(func(r *RunnerProgress) { r.FailedTicks = 3 }), reading: RunnerFailing, cause: CauseUnavailable, detail: "runner " + pid + " failed its last 3 ticks", own: true},
		{name: "failing outranks waiting", arrange: publish(func(r *RunnerProgress) { r.State, r.Reason, r.FailedTicks = "waiting", "idle", 3 }), reading: RunnerFailing, cause: CauseUnavailable, detail: "runner " + pid + " failed its last 3 ticks", own: true},
		{name: "two failed ticks", arrange: publish(func(r *RunnerProgress) { r.FailedTicks, r.OKTicks = 2, 1 }), reading: RunnerProgressing, detail: "tick (working, 3 done) for 2m0s", own: true},
		{name: "one failure at a limit of one", arrange: withConf("steward.runner-failed-ticks=1\n", func(r *RunnerProgress) { r.FailedTicks = 1 }), reading: RunnerFailing, cause: CauseUnavailable, detail: "runner " + pid + " failed its last 1 ticks", own: true},
		{name: "waiting", arrange: publish(func(r *RunnerProgress) { r.State, r.Reason = "waiting", "arbitration" }), reading: RunnerWaiting, detail: "tick (waiting arbitration, 3 done) for 2m0s", own: true},
		{name: "working", arrange: publish(same), reading: RunnerProgressing, detail: "tick (working, 3 done) for 2m0s", own: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newLivenessBed(t)
			test.arrange(b)
			got := runnerLiveness(b.root, b.now, func() time.Time { return b.now.Add(test.observed) }, b.probe)
			detailMatches := got.Detail == test.detail || test.prefix && strings.HasPrefix(got.Detail, test.detail)
			wantPid := livenessPid
			if test.noPid {
				wantPid = 0
			}
			if got.Reading != test.reading || got.Cause != test.cause || !detailMatches || got.Pid != wantPid ||
				(got.Progress != nil) != test.own || got.Quiet != test.quiet {
				t.Fatalf("report=%+v, want %+v", got, test)
			}
		})
	}
	t.Run("a record published while observing is not judged", func(t *testing.T) {
		t.Parallel()
		b := newLivenessBed(t)
		b.publish(b.working())
		newer := b.working()
		newer.At = b.now.Add(time.Minute)
		got := runnerLiveness(b.root, b.now, func() time.Time { b.publish(newer); return b.now }, b.probe)
		if got.Reading != RunnerProgressing {
			t.Fatalf("report=%+v", got)
		}
	})
}

func TestRunnerLivenessEntryAndObservationTime(t *testing.T) {
	t.Parallel()
	b := newLivenessBed(t)
	if err := os.Remove(runnerRecordPath(b.root)); err != nil {
		t.Fatal(err)
	}
	if got := RunnerLiveness(b.root, b.now); got.Reading != RunnerDead || got.Cause != CauseUnavailable {
		t.Fatalf("report=%+v", got)
	}
	later := time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := runnerObservedAt(later)(); !got.Equal(later) {
		t.Fatalf("observation %s fell behind the caller's time", got)
	}
}

func TestARunnerGoesQuietOnlyPastItsThreshold(t *testing.T) {
	t.Parallel()
	const threshold, hour = 30 * time.Minute, time.Hour
	deadlined := func(reason string, within time.Duration) func(*livenessBed, *RunnerProgress) {
		return func(b *livenessBed, r *RunnerProgress) {
			*r = b.waiting(reason)
			r.ProgressAt, r.At = b.now, b.now
			deadline := b.now.Add(within)
			r.Deadline = &deadline
		}
	}
	unbounded := func(reason string) func(*livenessBed, *RunnerProgress) {
		return func(b *livenessBed, r *RunnerProgress) { *r = b.waiting(reason); r.ProgressAt, r.At = b.now, b.now }
	}
	working := func(b *livenessBed, r *RunnerProgress) { r.ProgressAt, r.At = b.now, b.now }
	idle, external := deadlined("idle", hour), deadlined("external", time.Minute)
	for _, test := range []struct {
		name    string
		conf    string
		arrange func(*livenessBed, *RunnerProgress)
		after   time.Duration
		reading RunnerReading
		quiet   bool
	}{
		{"working at the threshold", "", working, threshold, RunnerProgressing, false},
		{"working just past it stays progressing", "", working, threshold + 1, RunnerProgressing, true},
		{"working long past it stays progressing", "", working, 5 * hour, RunnerProgressing, true},
		{"idle wait past the threshold, before its deadline", "", idle, threshold + 1, RunnerWaiting, false},
		{"idle wait at its deadline", "", idle, hour, RunnerWaiting, false},
		{"idle wait at the threshold past its deadline", "", idle, hour + threshold, RunnerWaiting, false},
		{"idle wait just past that", "", idle, hour + threshold + 1, RunnerWaiting, true},
		{"arbitration wait of two hours", "", unbounded("arbitration"), 2 * hour, RunnerWaiting, true},
		{"external wait at the threshold past its deadline", "", external, time.Minute + threshold, RunnerWaiting, false},
		{"external wait long past its deadline", "", external, time.Minute + threshold + time.Second, RunnerWaiting, true},
		{"external wait without a deadline at the threshold", "", unbounded("external"), threshold, RunnerWaiting, false},
		{"external wait without a deadline past it", "", unbounded("external"), threshold + 1, RunnerWaiting, true},
		{"a shorter setting, at it", "steward.runner-quiet-min=5\n", working, 5 * time.Minute, RunnerProgressing, false},
		{"a shorter setting, past it", "steward.runner-quiet-min=5\n", working, 5*time.Minute + 1, RunnerProgressing, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newLivenessBed(t)
			b.conf(test.conf)
			record := b.working()
			test.arrange(b, &record)
			b.publish(record)
			if got := b.read(b.now.Add(test.after)); got.Reading != test.reading || got.Quiet != test.quiet {
				t.Fatalf("report=%+v, want %s quiet=%t", got, test.reading, test.quiet)
			}
		})
	}
}

// enrollForLiveness gives a checkout the enrollment the reading requires.
func enrollForLiveness(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: livenessGeneration}); err != nil {
		t.Fatal(err)
	}
}

func TestTicksThroughTheLoopDecideFailing(t *testing.T) {
	t.Parallel()
	failed := errors.New("tick failed")
	for _, test := range []struct {
		name    string
		results []error
		want    RunnerReading
	}{
		{"three errors", []error{failed, failed, failed}, RunnerFailing},
		{"error, error, success", []error{failed, failed, nil}, RunnerProgressing},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			enrollForLiveness(t, loop.root)
			now := claimQuotaClock
			_, prober := progressProbe(now)
			deps := idleRunnerDependencies(&now)
			deps.Self = prober
			ticks := 0
			var got RunnerLivenessReport
			deps.Tick = func(root string, _ TickConfig, _ WorkerCensus) (TickResult, error) {
				if ticks == len(test.results) {
					got = runnerLiveness(root, now, func() time.Time { return now }, prober)
					loop.stop(t)
					return TickResult{}, nil
				}
				ticks++
				now = now.Add(time.Second)
				return TickResult{}, test.results[ticks-1]
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 2*time.Second, TickConfig{Now: now, ProviderHome: testprovider.Home(loop.root)}, deps); err != nil {
				t.Fatal(err)
			}
			if got.Reading != test.want {
				t.Fatalf("report=%+v, want %s", got, test.want)
			}
		})
	}
	t.Run("reports fail after each complete tick", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil)
		testprovider.Register(t, root)
		loop := &helmLoop{root: canonicalPath(root)}
		now := claimQuotaClock
		_, prober := progressProbe(now)
		deps := idleRunnerDependencies(&now)
		deps.Self = prober
		ticks := 0
		var got RunnerLivenessReport
		deps.Tick = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			if ticks++; ticks > 3 {
				got = runnerLiveness(root, now, func() time.Time { return now }, prober)
				loop.stop(t)
				return TickResult{}, nil
			}
			cfg.Now = now
			return RunTick(root, cfg, census)
		}
		cfg := TickConfig{Now: now, Prober: prober, ProviderHome: testprovider.Home(root),
			Patterns: func(string, time.Time) error { return errors.New("report failed") }}
		if err := runLoopWithDependencies(root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
			t.Fatal(err)
		}
		if got.Reading != RunnerFailing || got.Progress == nil || got.Progress.FailedTicks != 3 {
			t.Fatalf("report=%+v progress=%+v", got, got.Progress)
		}
		evidence, err := os.ReadFile(ComponentEvidencePath(root, "steward-tick"))
		if err != nil || !strings.Contains(strings.ReplaceAll(string(evidence), " ", ""), `"outcome":"PASS_COMPLETE"`) {
			t.Fatalf("tick evidence=%s err=%v", evidence, err)
		}
	})
}

func TestTheHealthRowShowsTheReading(t *testing.T) {
	t.Parallel()
	b := newLivenessBed(t)
	process := identity.Ref{Pid: b.runner.Pid, StartedAtSec: b.runner.PidStartedAt, StartTicks: b.runner.StartTicks, BootID: b.runner.BootID}
	attempt, err := beginComponentAttempt(b.root, "steward-tick", livenessGeneration, process, b.now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeComponentAttempt(b.root, "steward-tick", livenessGeneration, attempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "fixture", nil, b.now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	unpublished := checkStewardRunner(b.root, b.now, b.probe)
	if unpublished.Status != HealthAlive || !strings.Contains(unpublished.Reason, "has started and has not published yet") {
		t.Fatalf("no progress record: %+v", unpublished)
	}
	snapshot := func() (string, time.Time) {
		t.Helper()
		data, readErr := os.ReadFile(runnerProgressPath(b.root))
		info, statErr := os.Stat(runnerProgressPath(b.root))
		if readErr != nil || statErr != nil {
			t.Fatalf("progress record: read=%v stat=%v", readErr, statErr)
		}
		return string(data), info.ModTime()
	}
	record := b.working()
	b.publish(record)
	contents, modified := snapshot()
	fresh := checkStewardRunner(b.root, b.now, b.probe)
	if fresh.Status != unpublished.Status || !strings.Contains(fresh.Reason, "; tick (working, 3 done) for 2m0s") || strings.Contains(fresh.Reason, "quiet") {
		t.Fatalf("fresh runner: %+v", fresh)
	}
	if after, at := snapshot(); after != contents || !at.Equal(modified) {
		t.Fatal("reading the runner changed its progress record")
	}
	record.ProgressAt = b.now.Add(-31 * time.Minute)
	b.publish(record)
	quiet := checkStewardRunner(b.root, b.now, b.probe)
	if quiet.Status != unpublished.Status || !strings.Contains(quiet.Reason, "for 2m0s; quiet for 31m0s") {
		t.Fatalf("quiet runner: %+v", quiet)
	}
}

func TestTheIdleWaitStaysWaitingThroughItsCallbacks(t *testing.T) {
	t.Parallel()
	const interval, quietAfter = time.Hour, 5 * time.Minute
	loop := newHelmLoop(t)
	enrollForLiveness(t, loop.root)
	conf := filepath.Join(loop.root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("launch.seat.runtime=claude\nsteward.runner-quiet-min=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := claimQuotaClock
	if _, err := testprovider.Record(loop.root, outage.ProviderLimit, "limited", "fixture", start); err != nil {
		t.Fatal(err)
	}
	now := start
	_, prober := progressProbe(now)
	deps := idleRunnerDependencies(&now)
	deps.Self, deps.ProviderHome = prober, testprovider.Home(loop.root)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }
	pastThreshold := map[string]int{}
	check := func(source string) {
		got := runnerLiveness(loop.root, now, func() time.Time { return now }, prober)
		if got.Progress == nil || got.Progress.Operation != "wait" {
			return // the keeper's own step, before the wait begins
		}
		deadline := got.Progress.Deadline
		if got.Reading != RunnerWaiting || got.Progress.Reason != "idle" || got.Quiet || deadline == nil || !deadline.Equal(start.Add(interval)) {
			t.Fatalf("%s at %s: report=%+v progress=%+v", source, now.Sub(start), got, got.Progress)
		}
		if now.Sub(start) > quietAfter {
			pastThreshold[source]++
		}
		if pastThreshold["keeper"] > 0 && pastThreshold["probe"] > 0 {
			loop.stop(t)
		}
	}
	cfg := TickConfig{Now: start, ProviderHome: testprovider.Home(loop.root),
		KeepLandingLane: func() lane.AgentRun { check("keeper"); return lane.AgentRun{Outcome: lane.AgentIdle} },
		ProbeProvider:   func(string) (bool, error) { check("probe"); return false, nil }}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, interval, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if pastThreshold["keeper"] == 0 || pastThreshold["probe"] == 0 {
		t.Fatalf("callbacks past the threshold=%v", pastThreshold)
	}
}
