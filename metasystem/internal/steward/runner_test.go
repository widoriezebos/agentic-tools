package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

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

func TestRunLoopTicksUntilTheStopFile(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(root, census, nil, 50*time.Millisecond)
	}()
	overallDeadline := time.Now().Add(120 * time.Second)
	if deadline, ok := t.Deadline(); ok {
		overallDeadline = deadline.Add(-5 * time.Second)
	}
	lastEvidence, _ := LoadEvidence(EvidencePath(root))
	lastEvidenceChange := time.Now()
	for {
		ev, _ := LoadEvidence(EvidencePath(root))
		now := time.Now()
		if ev != lastEvidence {
			lastEvidence = ev
			lastEvidenceChange = now
		}
		if ev.TicksSinceAdvance >= 2 {
			break
		}
		if now.Sub(lastEvidenceChange) >= 10*time.Second || !now.Before(overallDeadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	ev, _ := LoadEvidence(EvidencePath(root))
	if ev.TicksSinceAdvance < 2 {
		t.Fatalf("the loop must tick repeatedly: %+v", ev)
	}
	if err := os.WriteFile(runnerStopPath(root), []byte("stop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a stopped loop exits clean: %v", err)
		}
	case <-time.After(time.Until(overallDeadline)):
		t.Fatal("the stop file must end the loop")
	}
	if _, err := os.Stat(runnerRecordPath(root)); !os.IsNotExist(err) {
		t.Fatal("a stopped runner removes its record")
	}
}

func TestRunLoopAttemptsRevivalBeforeNotifyingItsFailure(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	sink := filepath.Join(t.TempDir(), "alerts.log")
	command := `printf '%s\n' "$STEWARD_MESSAGE" >> ` + sink
	if out, err := gitConfig(root, "metasystem.steward.notify-command", command); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	alertedBeforeRepair := false
	revive := func() error {
		if data, err := os.ReadFile(sink); err == nil && strings.Contains(string(data), "stalled-dead") {
			alertedBeforeRepair = true
		}
		if err := os.WriteFile(runnerStopPath(root), []byte("stop\n"), 0o644); err != nil {
			return err
		}
		return os.ErrInvalid
	}
	if err := RunLoop(root, deadCensus(), revive, time.Hour); err != nil {
		t.Fatal(err)
	}
	if alertedBeforeRepair {
		t.Fatal("the revive verdict reached the notifier before healing was attempted")
	}
	data, err := os.ReadFile(sink)
	if err != nil || !strings.Contains(string(data), "revival failed") || strings.Contains(string(data), "stalled-dead") {
		t.Fatalf("only the failed recovery should reach the queued notifier: %q %v", data, err)
	}
}

func TestSecondRunnerRefusesBesideALiveOne(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	done := make(chan error, 1)
	go func() { done <- RunLoop(root, census, nil, time.Hour) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(runnerRecordPath(root)); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := RunLoop(root, census, nil, time.Hour); err == nil {
		t.Fatal("one repository, one runner")
	}
	os.WriteFile(runnerStopPath(root), []byte("stop\n"), 0o644)
	<-done
}

func TestArmRefusesWithoutANotifier(t *testing.T) {
	if runtime.GOOS == "darwin" {
		// darwin always has the platform notifier; the refusal is
		// the no-default platforms' behavior, proven on the Debian
		// guest in the battery.
		t.Skip("darwin's platform notifier makes an unconfigured channel reachable")
	}
	root := gitRepoWithCurrentGoal(t) // no notify-command configured
	if _, err := Arm(root, "/usr/bin/true"); err == nil {
		t.Fatal("an unreachable watchdog guards nothing; arm must refuse")
	}
}

func TestArmConfirmsTheGuardAndDisarmEndsIt(t *testing.T) {
	root := reviveRepo(t) // notify-command configured
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Skipf("engine binary not built at %s", bin)
	}
	t.Cleanup(func() { _, _ = Disarm(root) })
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
	if out, err := Disarm(root); err != nil || !strings.Contains(out, "disarmed") {
		t.Fatalf("disarm ends it: %q %v", out, err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("a disarmed repository has no runner")
	}
	id, err := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if err != nil || id.Generation < 1 {
		t.Fatalf("arm mints the identity: %+v %v", id, err)
	}
}

func TestKilledStewardIsRestoredByOneWatcherRepairPass(t *testing.T) {
	root := reviveRepo(t)
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Skipf("engine binary not built at %s", bin)
	}
	t.Cleanup(func() { _, _ = Disarm(root) })
	if _, err := Arm(root, bin); err != nil {
		t.Fatal(err)
	}
	installedBefore, err := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if checkStewardRunner(root, time.Now(), identity.KernelProber{}).Status == HealthAlive {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	before, alive := liveRunner(root)
	if !alive || checkStewardRunner(root, time.Now(), identity.KernelProber{}).Status != HealthAlive {
		t.Fatal("the fixture runner never completed its first generation-bound pass")
	}
	if err := syscall.Kill(int(before.Pid), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, stillAlive := liveRunner(root); !stillAlive {
			break
		}
		time.Sleep(20 * time.Millisecond)
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
	root := reviveRepo(t)
	if _, err := Arm(root, "/bin/sleep"); err == nil || !strings.Contains(err.Error(), "died before guarding") {
		t.Fatalf("a runner that cannot run is a named failure, not a claimed guard: %v", err)
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
	root := reviveRepo(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := Arm(root, "/bin/sleep")
	if err != nil || !strings.Contains(msg, "not armed") {
		t.Fatalf("ambient arming must stay out of fake-runtimes repositories: %q %v", msg, err)
	}
	if _, alive := liveRunner(root); alive {
		t.Fatal("no runner may leak into a fixture world")
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
