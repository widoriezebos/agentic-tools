package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunLoopTicksUntilTheStopFile(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(root, census, nil, 50*time.Millisecond)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ev, _ := LoadEvidence(EvidencePath(root))
		if ev.TicksSinceAdvance >= 2 {
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
	case <-time.After(3 * time.Second):
		t.Fatal("the stop file must end the loop")
	}
	if _, err := os.Stat(runnerRecordPath(root)); !os.IsNotExist(err) {
		t.Fatal("a stopped runner removes its record")
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
	// Idempotency: a second arm finds the guard, never a duplicate.
	again, err := Arm(root, bin)
	if err != nil || !strings.Contains(again, "already armed") {
		t.Fatalf("a second arm collapses onto the live runner: %q %v", again, err)
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
