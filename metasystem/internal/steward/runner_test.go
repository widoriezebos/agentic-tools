package steward

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func reapStewardRunnerFixture(t *testing.T, root string) {
	t.Helper()
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{
		Verb: "steward run",
		Resolve: func() (int, bool, error) {
			runner, present := LiveRunner(root)
			return int(runner.Pid), present, nil
		},
	}}, testenv.FixtureCleanup{
		Verb: "steward disarm",
		Run: func(ctx context.Context) error {
			result := make(chan error, 1)
			go func() {
				_, err := Disarm(root)
				result <- err
			}()
			select {
			case err := <-result:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

type runnerTickCensus struct {
	calls   chan<- struct{}
	stop    <-chan struct{}
	count   *int
	workers Workers
}

func (c runnerTickCensus) Workers(string) (Workers, error) {
	(*c.count)++
	select {
	case c.calls <- struct{}{}:
	case <-c.stop:
	}
	if *c.count == 4 {
		<-c.stop
	}
	return c.workers, nil
}

func TestArmTemporaryRefusesContentFreeRemoteWord(t *testing.T) {
	for _, test := range []struct {
		name     string
		word     string
		reviewBy string
		want     string
	}{
		{name: "whitespace word", word: " \t ", reviewBy: "2026-09-06", want: "non-whitespace"},
		{name: "non-date review", word: "Wido authorizes this enrollment", reviewBy: "whenever", want: "real date"},
		{name: "missing pair", want: "requires the verbatim word"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ArmTemporary(t.TempDir(), "/bin/true", test.word, test.reviewBy); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("temporary arm validation did not refuse by the expected cause: %v", err)
			}
		})
	}
}

func TestRunnerLaunchArgumentsRemainCompatible(t *testing.T) {
	if got := strings.Join(runnerLaunchArguments("/fixture/repo"), " "); got != "steward run --repo /fixture/repo" {
		t.Fatalf("runner launch argv = %q; want the pre-change contract", got)
	}
}

func TestRunLoopTicksUntilTheStopFile(t *testing.T) {
	t.Parallel()
	repository := newDecisionTickRepository(t)
	root := repository.root
	calls := make(chan struct{})
	stopCalls := make(chan struct{})
	var releaseCalls sync.Once
	censusCount := 0
	census := runnerTickCensus{calls: calls, stop: stopCalls, count: &censusCount, workers: Workers{Live: 1, CensusComplete: true}}
	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	sleeps := 0
	published := 0
	deps := runnerLoopDependencies{
		Tick: repository.runnerTick(), DeliverPending: runnerPendingDelivery(root, "true"),
		Now: func() time.Time { return now },
		Sleep: func(interval time.Duration) {
			if interval != 200*time.Millisecond {
				t.Errorf("runner loop sleep = %s; want 200ms", interval)
			}
			now = now.Add(interval)
			sleeps++
		},
		AfterRecordPublished: func() { published++ },
	}
	done := make(chan error, 1)
	go func() {
		done <- runLoopWithDependencies(root, census, nil, 50*time.Millisecond, TickConfig{Now: now}, deps)
		close(done)
	}()
	t.Cleanup(func() {
		// The loop must be stopped and drained before its checkout is torn down.
		if err := stopRunnerLoop(root); err != nil && !os.IsExist(err) {
			t.Errorf("stop RunLoop during cleanup: %v", err)
		}
		releaseCalls.Do(func() { close(stopCalls) })
		<-done
	})
	var ev Evidence
	for range 4 {
		select {
		case <-calls:
		case err := <-done:
			t.Fatalf("runner ended before four census calls: %v", err)
		}
		ev, _ = LoadEvidence(EvidencePath(root))
	}
	if ev.TicksSinceAdvance < 2 {
		t.Fatalf("the loop must tick repeatedly: %+v", ev)
	}
	if err := stopRunnerLoop(root); err != nil {
		t.Fatal(err)
	}
	releaseCalls.Do(func() { close(stopCalls) })
	if err := <-done; err != nil {
		t.Fatalf("a stopped loop exits clean: %v", err)
	}
	if sleeps != 3 {
		t.Fatalf("runner loop took %d artificial sleeps; want 3", sleeps)
	}
	if censusCount != 4 {
		t.Fatalf("runner sampled workers %d times; want 4", censusCount)
	}
	if published != 1 {
		t.Fatalf("runner published %d records; want 1", published)
	}
	if _, err := os.Stat(runnerRecordPath(root)); !os.IsNotExist(err) {
		t.Fatal("a stopped runner removes its record")
	}
}

func TestRunLoopAttemptsRevivalBeforeNotifyingItsFailure(t *testing.T) {
	t.Parallel()
	repository := newDecisionTickRepository(t)
	root := repository.root
	sink := filepath.Join(t.TempDir(), "alerts.log")
	command := `printf '%s\n' "$STEWARD_MESSAGE" >> ` + sink
	alertedBeforeRepair := false
	sleeps, published := 0, 0
	revive := func() error {
		if data, err := os.ReadFile(sink); err == nil && strings.Contains(string(data), "stalled-dead") {
			alertedBeforeRepair = true
		}
		if err := stopRunnerLoop(root); err != nil {
			return err
		}
		return os.ErrInvalid
	}
	deps := runnerLoopDependencies{
		Tick: repository.runnerTick(), DeliverPending: runnerPendingDelivery(root, command),
		Now: time.Now, Sleep: func(interval time.Duration) { sleeps++; _ = stopRunnerLoop(root) },
		AfterRecordPublished: func() { published++ },
	}
	if err := runLoopWithDependencies(root, deadCensus(), revive, time.Hour, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
	if alertedBeforeRepair {
		t.Fatal("the revive verdict reached the notifier before healing was attempted")
	}
	data, err := os.ReadFile(sink)
	if err != nil || !strings.Contains(string(data), "revival failed") || strings.Contains(string(data), "stalled-dead") {
		t.Fatalf("only the failed recovery should reach the queued notifier: %q %v", data, err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("delivered recovery must leave no pending notices: %v %v", pending, err)
	}
	if sleeps != 0 || published != 1 {
		t.Fatalf("recovery loop slept %d times and published %d records", sleeps, published)
	}
}

func TestSecondRunnerRefusesBesideALiveOne(t *testing.T) {
	t.Parallel()
	repository := newDecisionTickRepository(t)
	root := repository.root
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	done := make(chan error, 1)
	recordPublished := make(chan struct{})
	sleepGate := make(chan struct{})
	sleeps, published := 0, 0
	var releaseSleep sync.Once
	stopAndRelease := func() {
		if err := stopRunnerLoop(root); err != nil {
			t.Errorf("stop RunLoop: %v", err)
		}
		releaseSleep.Do(func() { close(sleepGate) })
	}
	deps := runnerLoopDependencies{
		Tick: repository.runnerTick(), DeliverPending: runnerPendingDelivery(root, "true"),
		Now: time.Now, Sleep: func(interval time.Duration) { sleeps++; <-sleepGate },
		AfterRecordPublished: func() { published++; close(recordPublished) },
	}
	go func() {
		done <- runLoopWithDependencies(root, census, nil, time.Hour, TickConfig{}, deps)
		close(done)
	}()
	t.Cleanup(func() { stopAndRelease(); <-done })
	select {
	case <-recordPublished:
	case err := <-done:
		t.Fatalf("first runner ended before publishing its record: %v", err)
	}
	secondDeps := runnerLoopDependencies{
		Tick: repository.runnerTick(), DeliverPending: runnerPendingDelivery(root, "true"),
		Now: time.Now, Sleep: time.Sleep,
		AfterRecordPublished: func() { t.Error("second runner published a record") },
	}
	if err := runLoopWithDependencies(root, census, nil, time.Hour, TickConfig{}, secondDeps); err == nil {
		t.Fatal("one repository, one runner")
	}
	stopAndRelease()
	if err := <-done; err != nil {
		t.Fatalf("first runner did not stop cleanly: %v", err)
	}
	if _, err := os.Stat(runnerRecordPath(root)); !os.IsNotExist(err) {
		t.Fatal("a stopped runner removes its record")
	}
	if published != 1 {
		t.Fatalf("first runner published %d records; want 1", published)
	}
	if sleeps > 1 {
		t.Fatalf("first runner slept %d times after stop", sleeps)
	}
}

func TestArmRefusesWithoutANotifier(t *testing.T) {
	root := canonicalPath(t.TempDir())
	deps := runnerPolicyDeps(t, root, &runnerPolicyNotify{err: os.ErrNotExist})
	outcome, err := armWithRearmDeps(root, "/usr/bin/true", false, false, false,
		humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal), deps)
	if err == nil || !strings.Contains(err.Error(), "no notification channel is configured") || outcome.Stage != StageBeforeMint {
		t.Fatalf("an unreachable watchdog must refuse before minting: %+v %v", outcome, err)
	}
	if _, err := os.Stat(RepoIdentityPath(root)); !os.IsNotExist(err) {
		t.Fatalf("refusal minted an identity: %v", err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("refusal launched a runner")
	}
}

func TestArmConfirmsTheGuardAndDisarmEndsIt(t *testing.T) {
	root := reviveRepo(t) // notify-command configured
	reapStewardRunnerFixture(t, root)
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Skipf("engine binary not built at %s", bin)
	}
	msg, err := Arm(root, bin)
	if err != nil || !strings.Contains(msg, "armed") {
		t.Fatalf("arm returns only once the repository is guarded: %q %v", msg, err)
	}
	if _, alive := liveRunner(root); !alive {
		t.Fatal("the confirmed runner is provably live")
	}
	// Idempotency: session-start ensure waits for a generation-bound pass and
	// verifies the same runner without spawning a duplicate.
	beforeEnsure, _ := liveRunner(root)
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	ensured, err := EnsureRunner(root, pinned, 1000)
	if err != nil || ensured.Action != "verified" || ensured.Pid != beforeEnsure.Pid {
		t.Fatalf("a second ensure must verify the live runner: %+v %v", ensured, err)
	}
	before, _ := liveRunner(root)
	restarted, err := Restart(root, bin)
	if err != nil || !strings.Contains(restarted, "armed") {
		t.Fatalf("restart replaces an alive runner: %q %v", restarted, err)
	}
	after, alive := liveRunner(root)
	if !alive || after.Pid == before.Pid {
		t.Fatalf("restart must record a different live runner: before=%+v after=%+v", before, after)
	}
	if out, err := Disarm(root); err != nil || !strings.Contains(out.LongForm(), "disarmed") {
		t.Fatalf("disarm ends it: %+v %v", out, err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("a disarmed repository has no runner")
	}
	id, err := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if err != nil || id.Generation < 1 || id.Enrollment != EnrollmentHumanTerminal {
		t.Fatalf("arm mints the identity: %+v %v", id, err)
	}
}

func TestKilledStewardIsRestoredByOneWatcherRepairPass(t *testing.T) {
	root := reviveRepo(t)
	reapStewardRunnerFixture(t, root)
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Skipf("engine binary not built at %s", bin)
	}
	if _, err := Arm(root, bin); err != nil {
		t.Fatal(err)
	}
	installedBefore, err := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if err != nil {
		t.Fatal(err)
	}
	initialWait, err := runnerStopWait(root, int((10*runnerConfirmationWait)/time.Second))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(initialWait)
	becameHealthy := false
	for time.Now().Before(deadline) {
		if checkStewardRunner(root, time.Now(), identity.KernelProber{}).Status == HealthAlive {
			becameHealthy = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	before, alive := liveRunner(root)
	if !becameHealthy || !alive || checkStewardRunner(root, time.Now(), identity.KernelProber{}).Status != HealthAlive {
		t.Fatal("the fixture runner never completed its first generation-bound pass")
	}
	if err := syscall.Kill(int(before.Pid), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	replacementWait, err := runnerStopWait(root, int((10*runnerReplacementKillWait)/time.Second))
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(replacementWait)
	died := false
	for time.Now().Before(deadline) {
		if _, stillAlive := liveRunner(root); !stillAlive {
			died = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !died {
		t.Fatalf("killed fixture runner pid %d remained alive after %s", before.Pid, replacementWait)
	}
	outcome, err := RepairEnrolledRunner(root)
	if err != nil {
		t.Fatal(err)
	}
	after, alive := liveRunner(root)
	installedAfter, idErr := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if outcome.Status != "RESTORED" || !alive || after.Pid == before.Pid || idErr != nil || installedAfter.Generation != installedBefore.Generation {
		t.Fatalf("one watcher pass must restore only the enrolled steward generation: outcome=%+v before=%+v after=%+v generation=%d->%d idErr=%v",
			outcome, before, after, installedBefore.Generation, installedAfter.Generation, idErr)
	}
}

func TestSlowFirstAttemptSurvivesSecondEnsureAndWatcherRepair(t *testing.T) {
	root := canonicalPath(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: mustAbs(t, root), Generation: 1, InstallPath: bin, InstallDigest: digest, MintedAt: now.UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	self, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read fixture process: %v %s", err, state)
	}
	if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{
		Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), StartTicks: self.StartTicks, BootID: self.BootID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := beginComponentAttempt(root, "steward-tick", 1, self.Ref(), now.Add(-11*time.Second)); err != nil {
		t.Fatal(err)
	}
	attempt, err := loadComponentEvidence(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || attempt.AttemptSeq != 1 || attempt.Outcome != "ATTEMPTING" || attempt.Generation != 1 || attempt.Pid != self.Pid || !attempt.LastAttempt.Equal(now.Add(-11*time.Second)) {
		t.Fatalf("the first attempt must be recorded before ensure: %+v %v", attempt, err)
	}
	before, alive := liveRunner(root)
	if !alive || before.Pid != self.Pid {
		t.Fatalf("the current process must own the runner record: %+v alive=%t", before, alive)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	clock := func() time.Time { return now }
	noSleep := func(d time.Duration) { t.Fatalf("slow attempt waited for %s", d) }
	deps := runnerPolicyDeps(t, root, &runnerPolicyNotify{output: "true\n"})
	ensured, err := ensureRunnerWithDependencies(root, pinned, 1000, deps, clock, noSleep)
	if err != nil || ensured.Action != "verified" || ensured.Pid != self.Pid || ensured.Generation != 1 {
		t.Fatalf("a second up must verify the slow first attempt without replacement: %+v %v", ensured, err)
	}
	repaired, err := repairEnrolledRunnerWithClock(root, nil, clock, noSleep)
	if err != nil || repaired.Status != "CURRENT" || repaired.ReplacementPid != self.Pid || repaired.Generation != 1 {
		t.Fatalf("a watcher cycle must preserve the slow first attempt: %+v %v", repaired, err)
	}
	after, alive := liveRunner(root)
	if !alive || after != before {
		t.Fatalf("ensure and repair must keep the same runner identity: before=%+v after=%+v alive=%t", before, after, alive)
	}
	persisted, err := loadComponentEvidence(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || !reflect.DeepEqual(persisted, attempt) {
		t.Fatalf("ensure and repair changed the attempting evidence: before=%+v after=%+v err=%v", attempt, persisted, err)
	}
	installed, err := VerifyIdentity(RepoIdentityPath(root), root)
	if err != nil || installed.Generation != 1 {
		t.Fatalf("ensure and repair changed enrollment: %+v %v", installed, err)
	}
}

func TestWatcherReplacesAliveRunnerWithOverdueAttempt(t *testing.T) {
	root := reviveRepo(t)
	reapStewardRunnerFixture(t, root)
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Skipf("engine binary not built at %s", bin)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("steward.tick-patience-sec=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: mustAbs(t, root), Generation: 1, InstallPath: bin, InstallDigest: digest, MintedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	stuck := exec.Command("/bin/sh", "-c", "printf 'ready\\n' >&3; IFS= read -r _ <&4")
	stuck.ExtraFiles = []*os.File{readyWrite, releaseRead}
	if err := stuck.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	stuckDone := make(chan struct{})
	go func() {
		_, _ = stuck.Process.Wait()
		close(stuckDone)
	}()
	stuckJoined := false
	t.Cleanup(func() {
		_ = releaseWrite.Close()
		if !stuckJoined {
			_ = stuck.Process.Kill()
			<-stuckDone
		}
	})
	ready, readyErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	if readyErr != nil || ready != "ready\n" {
		t.Fatalf("overdue runner readiness=%q err=%v", ready, readyErr)
	}
	exact, state, err := identity.KernelProber{}.Probe(int64(stuck.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read stuck fixture process: %v %s", err, state)
	}
	if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{
		Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), StartTicks: exact.StartTicks, BootID: exact.BootID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := beginComponentAttempt(root, "steward-tick", 1, exact.Ref(), time.Now().Add(-2*time.Second)); err != nil {
		t.Fatal(err)
	}
	repaired, err := RepairEnrolledRunner(root)
	if err != nil || repaired.Status != "RESTORED" || repaired.PreviousPid != exact.Pid || repaired.ReplacementPid == exact.Pid {
		t.Fatalf("the watcher must replace an alive runner past its configured patience: %+v %v", repaired, err)
	}
	<-stuckDone
	stuckJoined = true
	_ = releaseWrite.Close()
}

func TestWatcherRepairStopsWhenTheStewardBreakerEndsHealing(t *testing.T) {
	root := t.TempDir()
	top := mustAbs(t, root)
	trueBinary, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	trueDigest, err := installDigest(trueBinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: top, Generation: 3, InstallPath: trueBinary, InstallDigest: trueDigest, MintedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	state := HealthObservationState{
		Sequence: 5, ObservedAt: now, UnknownCounts: map[HealthRole]int{},
		FailureCounts: map[HealthRole]int{RoleStewardRunner: healthFailureLimit},
	}
	verdict := HealthVerdict{Schema: 1, ObservedAt: now, Observation: 5, Aggregate: "unhealthy"}
	if err := saveHealthRecord(root, HealthRecordPath(root), healthRecord{State: state, Verdict: verdict}); err != nil {
		t.Fatal(err)
	}
	outcome, err := RepairEnrolledRunner(root)
	if err != nil || outcome.Status != AutoHealEnded || outcome.Generation != 3 {
		t.Fatalf("the watcher must not repair after failure five: %+v %v", outcome, err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("the ended breaker still launched a runner")
	}
}

func TestWatcherRepairAbortsWhenEnrollmentChangesBeforeItsLock(t *testing.T) {
	root := t.TempDir()
	top := mustAbs(t, root)
	trueBinary, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	falseBinary, err := exec.LookPath("false")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIdentity := func(generation int, binary string) {
		t.Helper()
		digest, err := installDigest(binary)
		if err != nil {
			t.Fatal(err)
		}
		if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
			RepoIdentity: top, Generation: generation, InstallPath: binary, InstallDigest: digest, MintedAt: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatal(err)
		}
	}
	writeIdentity(1, trueBinary)
	validated := make(chan struct{})
	reenrolled := make(chan struct{})
	type repairResult struct {
		outcome RunnerRepairOutcome
		err     error
	}
	result := make(chan repairResult, 1)
	go func() {
		outcome, err := repairEnrolledRunner(root, func() {
			close(validated)
			<-reenrolled
		})
		result <- repairResult{outcome: outcome, err: err}
	}()
	<-validated
	writeIdentity(2, falseBinary)
	close(reenrolled)
	got := <-result
	if got.err != nil || got.outcome.Status != "ENROLLMENT_CHANGED" || got.outcome.Generation != 1 {
		t.Fatalf("repair must abort instead of launching bytes from the superseded generation: %+v %v", got.outcome, got.err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("the generation-crossing repair launched a runner")
	}
}

func TestArmReportsARunnerThatDiedTrying(t *testing.T) {
	root := canonicalPath(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := armWithRearmDeps(root, "/bin/sleep", false, false, false,
		humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal),
		runnerPolicyDeps(t, root, &runnerPolicyNotify{output: "true\n"})); err == nil || !strings.Contains(err.Error(), "died before guarding") {
		t.Fatalf("a runner that cannot run is a named failure, not a claimed guard: %v", err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("the failed runner must not guard the repository")
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(a); resolveErr == nil {
		a = resolved
	}
	return a
}

func TestArmStaysOutOfFixtureWorlds(t *testing.T) {
	root := canonicalPath(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := runnerPolicyDeps(t, root, nil)
	outcome, err := armWithRearmDeps(root, "/bin/sleep", false, false, false,
		humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal), deps)
	if err != nil || !strings.Contains(outcome.Message, "not armed: fake-runtimes repository") || outcome.Stage != StageBeforeMint {
		t.Fatalf("ambient arming must stay out of fake-runtimes repositories: %+v %v", outcome, err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("no runner may leak into a fixture world")
	}
	if _, err := os.Stat(RepoIdentityPath(root)); !os.IsNotExist(err) {
		t.Fatalf("fixture exclusion minted an identity: %v", err)
	}
}

func TestArmAcceptsASubdirectoryCheckout(t *testing.T) {
	top := t.TempDir()
	sub := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// The fake-runtimes refusal PROVES the worktree fence passed: git
	// answers --git-common-dir relative and --git-dir absolute from a
	// subdirectory, and a raw comparison would call this a linked
	// worktree and refuse to guard the primary checkout.
	if err := os.WriteFile(filepath.Join(sub, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := Arm(sub, "/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "fake-runtimes") {
		t.Fatalf("a subdirectory of the primary checkout must pass the worktree fence, got %q", msg)
	}
}

func TestArmRefusesALinkedWorktree(t *testing.T) {
	top := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	linked := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "-C", top, "worktree", "add", "-q", linked)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	msg, err := Arm(linked, "/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "linked worktree") {
		t.Fatalf("a linked worktree must refuse to arm, got %q", msg)
	}
}
