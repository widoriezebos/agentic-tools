package steward

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestArmSpawnsADetachedRunnerAndDisarmEndsIt(t *testing.T) {
	root := reviveRepo(t) // notify-command configured
	msg, err := Arm(root, "/bin/sleep")
	// /bin/sleep will not understand the steward arguments and die
	// instantly — which is exactly what this leg wants to observe:
	// arm's spawn machinery works, and disarm handles a dead runner.
	if err != nil {
		t.Fatalf("arm: %v (%s)", err, msg)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := liveRunner(root); !alive {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if out, err := Disarm(root); err != nil || out == "" {
		t.Fatalf("disarm reports its outcome: %q %v", out, err)
	}
	id, err := VerifyIdentity(RepoIdentityPath(root), mustAbs(t, root))
	if err != nil || id.Generation < 1 {
		t.Fatalf("arm mints the identity: %+v %v", id, err)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
