package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestBatchLedgerOwnerDefaultRefusesUnbound(t *testing.T) {
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	owner, err := batchLedgerOwner(root)
	if err == nil || owner != nil || !strings.Contains(err.Error(), "no machine nickname is enrolled") {
		t.Fatalf("unbound batch ledger owner=(%T, %v), want missing machine enrollment refusal", owner, err)
	}
}

func TestBatchLedgerOwnerUsesLandingIdentity(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	owner, err := batchLedgerOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	ledgerOwner, ok := owner.(*ledgerTrunkRedOwner)
	if !ok {
		t.Fatalf("batch ledger owner type %T, want ledger adapter", owner)
	}
	if ledgerOwner.actor.Machine != "mac-cli" || ledgerOwner.actor.Lineage != landingOwnerLineage {
		t.Fatalf("batch ledger owner actor=%+v, want mac-cli+%s", ledgerOwner.actor, landingOwnerLineage)
	}
}

func TestBatchOwnerWiringBound(t *testing.T) {
	if os.Getenv("GO_WANT_BATCH_OWNER_SIGNAL_HELPER") != "" {
		root := os.Getenv("BATCH_OWNER_TEST_ROOT")
		batchOwnerAcquire = func(string) (batchOwnerLease, error) {
			return batchOwnerLease{root: root, pid: int64(os.Getpid()), epoch: 1}, nil
		}
		batchOwnerRequire = func(batchOwnerLease) error { return nil }
		batchOwnerConstruct = func(settings config.BatchLanding, held batchOwnerLease, _ productionBatchOwnerInputs, now func() time.Time) (*batch.Owner, error) {
			calls := 0
			return batch.NewOwner(batch.OwnerOptions{
				Store: batch.NewStore(root, nil), Settings: settings, Actor: "fixture", PID: held.pid, Now: now,
				FetchTree: func() (string, error) { return "tree", nil },
				ReadClaim: func(string, string, string, string) (batch.Claim, error) { return batch.Claim{}, nil },
				Rebind:    func(string, string) error { return nil }, Mint: func() (string, error) { return "opid", nil },
				LogRed: func(string, batch.TrunkRedRecordOutcome) {}, BaseCommit: func(string) (string, error) { return "commit", nil },
				RunDiagnostic: func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
					return batch.DiagnosticResult{}, nil
				},
				DescendsFrom: func(string, string) (bool, error) { return false, nil }, Sample: func() proofrun.LoadSample { return proofrun.LoadSample{} },
				Admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{} },
				Launch:    func(string, proofrun.LoadSample, string) error { return nil },
				After:     func(time.Duration) <-chan time.Time { return make(chan time.Time) },
				Report:    func(string, error) {}, Glob: func(string) ([]string, error) {
					calls++
					if calls == 1 {
						fmt.Println("READY")
					} else {
						fmt.Println("TICK")
					}
					return nil, nil
				},
			})
		}
		seat := os.Getenv("BATCH_OWNER_TEST_SEAT")
		os.Exit(runBatchOwner([]string{"--root", seat, "--landing-root", root, "--max-wait", "1m", "--interval", "1m"}))
	}
	root := syncedClaimedGoalFixture(t)
	seat := t.TempDir()
	goalSyncMutationGit(t, seat, "init", "-q")
	command := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerWiringBound$")
	command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_SIGNAL_HELPER=1", "BATCH_OWNER_TEST_ROOT="+root,
		"BATCH_OWNER_TEST_SEAT="+seat, "METASYSTEM_GOAL_NOW=2026-09-17T10:00:00Z")
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("signal helper not ready: %q (%v)", scanner.Text(), scanner.Err())
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	bound := time.NewTimer(wiringBound)
	defer bound.Stop()
	for observed := ""; observed != "TICK"; {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("SIGUSR1 wiring ended before the owner loop ticked: %v", scanner.Err())
			}
			observed = line
		case <-bound.C:
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("SIGUSR1 did not reach the owner loop within %s", wiringBound)
		}
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestBatchOwnerManualAcquireCleansFailedAnnouncement(t *testing.T) {
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q")
	original := batchOwnerAnnounce
	t.Cleanup(func() { batchOwnerAnnounce = original })
	batchOwnerAnnounce = func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, lineage string) (string, error) {
		if _, err := original(root, session, pid, start, startTicks, bootID, tag, runtime, lineage); err != nil {
			return "", err
		}
		return "", errors.New("injected failure after announcement")
	}
	if _, err := acquireBatchOwner(root); err == nil || !strings.Contains(err.Error(), "injected failure after announcement") {
		t.Fatalf("manual acquisition error=%v, want injected post-announcement failure", err)
	}
	if announcements := lease.AnnouncementsFor(root, int64(os.Getpid())); len(announcements) != 0 {
		t.Fatalf("manual acquisition failure leaked %d announcements, want none", len(announcements))
	}
}

func TestBatchOwnerReleaseDoesNotRecreateRemovedRoot(t *testing.T) {
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q")
	held, err := acquireBatchOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	held.retire()
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release recreated a removed checkout root: %v", err)
	}
}

func TestBatchOwnerNextProcessRecoversReleasedAndKilledHolder(t *testing.T) {
	if mode := os.Getenv("GO_WANT_BATCH_OWNER_RELEASE_HELPER"); mode != "" {
		root := os.Getenv("BATCH_OWNER_TEST_ROOT")
		held, err := acquireBatchOwner(root)
		if err != nil {
			t.Fatal(err)
		}
		if mode != "hold-dead" {
			defer held.retire()
		}
		fmt.Println("READY")
		if mode != "once" {
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
		return
	}

	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q")
	startHolder := func(mode string) (*exec.Cmd, io.WriteCloser) {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerNextProcessRecoversReleasedAndKilledHolder$")
		command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_RELEASE_HELPER="+mode, "BATCH_OWNER_TEST_ROOT="+root)
		stdin, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(stdout)
		if !scanner.Scan() || scanner.Text() != "READY" {
			t.Fatalf("%s holder did not acquire the fixture lease: %q (%v)", mode, scanner.Text(), scanner.Err())
		}
		return command, stdin
	}
	runOnce := func(label string) {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerNextProcessRecoversReleasedAndKilledHolder$")
		command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_RELEASE_HELPER=once", "BATCH_OWNER_TEST_ROOT="+root)
		if output, err := command.CombinedOutput(); err != nil || !strings.Contains(string(output), "READY") {
			t.Fatalf("next process did not recover %s holder: %v output=%q", label, err, output)
		}
	}

	released, releasedInput := startHolder("hold-release")
	if err := releasedInput.Close(); err != nil {
		t.Fatal(err)
	}
	if err := released.Wait(); err != nil {
		t.Fatal(err)
	}
	runOnce("released")

	dead, deadInput := startHolder("hold-dead")
	defer deadInput.Close()
	if err := dead.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := dead.Wait(); err == nil {
		t.Fatal("killed owner exited successfully")
	}
	runOnce("unreleased dead")
}

func TestBatchTickStopsBeforePassAfterLeaseLoss(t *testing.T) {
	landingRoot := syncedClaimedGoalFixture(t)
	seatRoot := t.TempDir()
	goalSyncMutationGit(t, seatRoot, "init", "-q")
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-17T10:00:00Z")
	originalAcquire, originalRequire, originalTick := batchOwnerAcquire, batchOwnerRequire, batchOwnerTick
	t.Cleanup(func() {
		batchOwnerAcquire, batchOwnerRequire, batchOwnerTick = originalAcquire, originalRequire, originalTick
	})
	batchOwnerAcquire = func(string) (batchOwnerLease, error) {
		return batchOwnerLease{root: landingRoot, pid: int64(os.Getpid()), epoch: 1}, nil
	}
	batchOwnerRequire = func(batchOwnerLease) error { return errors.New("injected lost lease") }
	acted := false
	batchOwnerTick = func(*batch.Owner, string) error {
		acted = true
		return nil
	}
	code := runBatchTick([]string{"--root", seatRoot, "--landing-root", landingRoot, "--max-wait", "1m", "--batch", "01j5x00000000000000000ba99"})
	if code != 1 {
		t.Fatalf("tick after lease loss returned code %d, want refusal code 1", code)
	}
	if acted {
		t.Fatal("tick kept acting as holder after the lease was lost")
	}
}

func TestBatchCommandsResolveInputsBeforeAnnouncement(t *testing.T) {
	commands := map[string]func(string, string) int{
		"owner": func(seat, landing string) int {
			return runBatchOwner([]string{"--root", seat, "--landing-root", landing, "--max-wait", "1m", "--interval", "1m"})
		},
		"tick": func(seat, landing string) int {
			return runBatchTick([]string{"--root", seat, "--landing-root", landing, "--max-wait", "1m", "--batch", "01j5x00000000000000000ba98"})
		},
	}
	for name, run := range commands {
		t.Run(name, func(t *testing.T) {
			landingRoot := syncedClaimedGoalFixture(t)
			goalSyncMutationGit(t, landingRoot, "config", "--unset", "metasystem.goal.machine")
			seatRoot := t.TempDir()
			goalSyncMutationGit(t, seatRoot, "init", "-q")
			t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-17T10:00:00Z")
			if code := run(seatRoot, landingRoot); code != 1 {
				t.Fatalf("%s setup failure returned code %d, want 1", name, code)
			}
			if announcements := lease.AnnouncementsFor(landingRoot, int64(os.Getpid())); len(announcements) != 0 {
				t.Fatalf("%s setup failure announced %d times before inputs resolved", name, len(announcements))
			}
			if _, err := lease.CurrentHolder(landingRoot); !errors.Is(err, lease.ErrLeaseAbsent) {
				t.Fatalf("%s setup failure left a checkout lease: %v", name, err)
			}
		})
	}
}

func TestBatchOwnerManualProofAndEnvironmentFailuresReleaseAnnouncement(t *testing.T) {
	t.Run("holder proof", func(t *testing.T) {
		root := t.TempDir()
		goalSyncMutationGit(t, root, "init", "-q")
		mains := filepath.Join(root, "artifacts", "agents", "mains")
		if err := os.MkdirAll(mains, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mains, "zz-invalid-announcement.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := acquireBatchOwner(root); err == nil || !strings.Contains(err.Error(), "holder proof failed") {
			t.Fatalf("manual acquisition error=%v, want holder proof failure", err)
		}
		if announcements := lease.AnnouncementsFor(root, int64(os.Getpid())); len(announcements) != 0 {
			t.Fatalf("holder proof failure leaked %d manual announcements", len(announcements))
		}
	})

	t.Run("environment", func(t *testing.T) {
		root := t.TempDir()
		goalSyncMutationGit(t, root, "init", "-q")
		original := batchOwnerSetenv
		t.Cleanup(func() { batchOwnerSetenv = original })
		batchOwnerSetenv = func(string, string) error { return errors.New("injected environment failure") }
		if _, err := acquireBatchOwner(root); err == nil || !strings.Contains(err.Error(), "injected environment failure") {
			t.Fatalf("manual acquisition error=%v, want environment failure", err)
		}
		if announcements := lease.AnnouncementsFor(root, int64(os.Getpid())); len(announcements) != 0 {
			t.Fatalf("environment failure leaked %d manual announcements", len(announcements))
		}
	})
}

func TestBatchCommandsReleaseAfterConstructionFailure(t *testing.T) {
	original := batchOwnerConstruct
	t.Cleanup(func() { batchOwnerConstruct = original })
	constructions := 0
	batchOwnerConstruct = func(config.BatchLanding, batchOwnerLease, productionBatchOwnerInputs, func() time.Time) (*batch.Owner, error) {
		constructions++
		return nil, errors.New("injected construction failure")
	}
	commands := map[string]func(string, string) int{
		"owner": func(seat, landing string) int {
			return runBatchOwner([]string{"--root", seat, "--landing-root", landing, "--max-wait", "1m", "--interval", "1m"})
		},
		"tick": func(seat, landing string) int {
			return runBatchTick([]string{"--root", seat, "--landing-root", landing, "--max-wait", "1m", "--batch", "01j5x00000000000000000ba97"})
		},
	}
	for name, run := range commands {
		t.Run(name, func(t *testing.T) {
			landingRoot := syncedClaimedGoalFixture(t)
			seatRoot := t.TempDir()
			goalSyncMutationGit(t, seatRoot, "init", "-q")
			t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-17T10:00:00Z")
			before := constructions
			if code := run(seatRoot, landingRoot); code != 1 {
				t.Fatalf("%s construction failure returned code %d, want 1", name, code)
			}
			if constructions != before+1 {
				t.Fatalf("%s made %d construction attempts, want one injected failure", name, constructions-before)
			}
			if announcements := lease.AnnouncementsFor(landingRoot, int64(os.Getpid())); len(announcements) != 0 {
				t.Fatalf("%s construction failure leaked %d announcements", name, len(announcements))
			}
		})
	}
}

func TestBatchOwnerLoopStopsBeforePassAfterLeaseLoss(t *testing.T) {
	originalRequire, originalResume := batchOwnerRequire, batchOwnerResume
	t.Cleanup(func() { batchOwnerRequire, batchOwnerResume = originalRequire, originalResume })
	batchOwnerRequire = func(batchOwnerLease) error { return errors.New("injected lost lease") }
	acted := false
	batchOwnerResume = func(*batch.Owner) { acted = true }
	err := loopBatchOwner(nil, batchOwnerLease{}, time.Minute, make(chan struct{}), make(chan struct{}))
	if err == nil || !strings.Contains(err.Error(), "injected lost lease") {
		t.Fatalf("owner loop error=%v, want lost lease refusal", err)
	}
	if acted {
		t.Fatal("owner loop kept acting as holder after the lease was lost")
	}
}
