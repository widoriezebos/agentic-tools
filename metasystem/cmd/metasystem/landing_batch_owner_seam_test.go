package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	isolateGlobalGitConfig(t)
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	owner, err := productionBatchLedgerOwner(root)
	if err == nil || owner != nil || !strings.Contains(err.Error(), "no machine nickname is enrolled") {
		t.Fatalf("unbound batch ledger owner=(%T, %v), want missing machine enrollment refusal", owner, err)
	}
}

func TestBatchLedgerOwnerUsesLandingIdentity(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	owner, err := productionBatchLedgerOwner(root)
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

func isolateGlobalGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestResolveExplicitBatchLandingRejectsInvalidRoots(t *testing.T) {
	seat := t.TempDir()
	now := func() time.Time { return time.Unix(1, 0) }
	for _, test := range []struct {
		name string
		root string
		want string
	}{
		{name: "relative", root: "relative", want: "absolute"},
		{name: "missing checkout", root: filepath.Join(t.TempDir(), "missing"), want: "existing checkout"},
		{name: "seat checkout", root: seat, want: "non-seat checkout"},
		{name: "ordinary directory", root: t.TempDir(), want: "existing checkout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := config.ResolveExplicitBatchLanding(test.root, seat, time.Minute, now); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("explicit root %q error=%v, want %q", test.root, err, test.want)
			}
		})
	}
}

func TestBatchOwnerWiringBound(t *testing.T) {
	if os.Getenv("GO_WANT_BATCH_OWNER_SIGNAL_HELPER") != "" {
		root := os.Getenv("BATCH_OWNER_TEST_ROOT")
		batchOwnerCadenceTick = func(string, batchOwnerLease, func() time.Time) error { return nil }
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
	for observed := ""; observed != "TICK"; observed = scanner.Text() {
		if !scanner.Scan() {
			t.Fatalf("SIGUSR1 wiring ended before the owner loop ticked: %v", scanner.Err())
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
	if err := held.retire(); err != nil {
		t.Fatal(err)
	}
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
		fmt.Println("READY")
		switch mode {
		case "once":
			if err := held.retire(); err != nil {
				t.Fatal(err)
			}
		case "hold-release":
			_, _ = os.Stdin.Read(make([]byte, 1))
			if err := held.retire(); err != nil {
				t.Fatal(err)
			}
			fmt.Println("RETIRED")
			_, _ = os.Stdin.Read(make([]byte, 1))
		case "hold-dead":
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
		return
	}

	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q")
	startHolder := func(mode string) (*exec.Cmd, io.WriteCloser, *bufio.Scanner) {
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
		t.Cleanup(func() {
			if command.ProcessState == nil {
				_ = command.Process.Kill()
				_ = command.Wait()
			}
		})
		return command, stdin, scanner
	}
	runOnce := func(label string) {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerNextProcessRecoversReleasedAndKilledHolder$")
		command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_RELEASE_HELPER=once", "BATCH_OWNER_TEST_ROOT="+root)
		if output, err := command.CombinedOutput(); err != nil || !strings.Contains(string(output), "READY") {
			t.Fatalf("next process did not recover %s holder: %v output=%q", label, err, output)
		}
	}

	released, releasedInput, releasedOutput := startHolder("hold-release")
	announcements := lease.AnnouncementsFor(root, int64(released.Process.Pid))
	if len(announcements) != 1 {
		t.Fatalf("released holder announcements=%d, want one before retirement", len(announcements))
	}
	announcementPath := filepath.Join(root, "artifacts", "agents", "mains", fmt.Sprintf("landing-owner-%d-%d.json", released.Process.Pid, released.Process.Pid))
	cursorPath := filepath.Join(root, "artifacts", "agents", "mains", announcements[0].MainId+".protocol-cursor.json")
	if _, err := releasedInput.Write([]byte("r")); err != nil {
		t.Fatal(err)
	}
	if !releasedOutput.Scan() || releasedOutput.Text() != "RETIRED" {
		t.Fatalf("release helper did not retire while alive: %q (%v)", releasedOutput.Text(), releasedOutput.Err())
	}
	for _, path := range []string{announcementPath, cursorPath, cursorPath + ".lock"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("live released holder left %s: %v", filepath.Base(path), err)
		}
	}
	if err := releasedInput.Close(); err != nil {
		t.Fatal(err)
	}
	if err := released.Wait(); err != nil {
		t.Fatal(err)
	}
	runOnce("released")

	dead, deadInput, _ := startHolder("hold-dead")
	defer deadInput.Close()
	if err := dead.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := dead.Wait(); err == nil {
		t.Fatal("killed owner exited successfully")
	}
	runOnce("unreleased dead")
}

func TestBatchOwnerRetirementErrorsReachEveryCaller(t *testing.T) {
	retireErr := errors.New("injected retirement failure")
	tests := []struct {
		name string
		run  func(*testing.T) (int, string)
	}{
		{name: "manual owner", run: func(t *testing.T) (int, string) {
			root := syncedClaimedGoalFixture(t)
			seat := t.TempDir()
			goalSyncMutationGit(t, seat, "init", "-q")
			originalAcquire, originalConstruct, originalRequire, originalRetire := batchOwnerAcquire, batchOwnerConstruct, batchOwnerRequire, batchOwnerRetire
			t.Cleanup(func() {
				batchOwnerAcquire, batchOwnerConstruct, batchOwnerRequire, batchOwnerRetire = originalAcquire, originalConstruct, originalRequire, originalRetire
			})
			batchOwnerAcquire = func(string) (batchOwnerLease, error) {
				return batchOwnerLease{root: root, session: "fixture", pid: int64(os.Getpid()), started: 1, epoch: 1, announced: true}, nil
			}
			batchOwnerConstruct = func(config.BatchLanding, batchOwnerLease, productionBatchOwnerInputs, func() time.Time) (*batch.Owner, error) {
				return &batch.Owner{}, nil
			}
			batchOwnerRequire = func(batchOwnerLease) error { return errors.New("stop owner loop") }
			batchOwnerRetire = func(string, string, int64, int64) error { return retireErr }
			code, _, stderr := captureCommandOutput(t, false, true, func() int {
				return runBatchOwner([]string{"--root", seat, "--landing-root", root, "--max-wait", "1m", "--interval", "1m"})
			})
			return code, stderr
		}},
		{name: "manual tick", run: func(t *testing.T) (int, string) {
			root := syncedClaimedGoalFixture(t)
			seat := t.TempDir()
			goalSyncMutationGit(t, seat, "init", "-q")
			originalAcquire, originalConstruct, originalRequire, originalTick, originalRetire := batchOwnerAcquire, batchOwnerConstruct, batchOwnerRequire, batchOwnerTick, batchOwnerRetire
			t.Cleanup(func() {
				batchOwnerAcquire, batchOwnerConstruct, batchOwnerRequire, batchOwnerTick, batchOwnerRetire = originalAcquire, originalConstruct, originalRequire, originalTick, originalRetire
			})
			batchOwnerAcquire = func(string) (batchOwnerLease, error) {
				return batchOwnerLease{root: root, session: "fixture", pid: int64(os.Getpid()), started: 1, epoch: 1, announced: true}, nil
			}
			batchOwnerConstruct = func(config.BatchLanding, batchOwnerLease, productionBatchOwnerInputs, func() time.Time) (*batch.Owner, error) {
				return &batch.Owner{}, nil
			}
			batchOwnerRequire = func(batchOwnerLease) error { return nil }
			batchOwnerTick = func(*batch.Owner, string) error { return nil }
			batchOwnerRetire = func(string, string, int64, int64) error { return retireErr }
			code, _, stderr := captureCommandOutput(t, false, true, func() int {
				return runBatchTick([]string{"--root", seat, "--landing-root", root, "--max-wait", "1m", "--batch", "01j5x00000000000000000ba98"})
			})
			return code, stderr
		}},
		{name: "supervised release", run: func(t *testing.T) (int, string) {
			code, _, stderr := captureCommandOutput(t, false, true, func() int {
				return reportLandingOwnerRelease(func() error { return retireErr })
			})
			return code, stderr
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, report := test.run(t)
			if code != 1 || !strings.Contains(report, retireErr.Error()) {
				t.Fatalf("retirement result code=%d report=%q, want surfaced failure", code, report)
			}
		})
	}
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
	err := loopBatchOwner(nil, batchOwnerLease{}, "root", func() time.Time { return time.Unix(1, 0) }, time.Minute, make(chan struct{}), make(chan struct{}))
	if err == nil || !strings.Contains(err.Error(), "injected lost lease") {
		t.Fatalf("owner loop error=%v, want lost lease refusal", err)
	}
	if acted {
		t.Fatal("owner loop kept acting as holder after the lease was lost")
	}
}

func TestBatchOwnerCadenceWiringBound(t *testing.T) {
	originalRequire, originalResume := batchOwnerRequire, batchOwnerResume
	originalStart, originalTick, originalReport := batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport
	t.Cleanup(func() {
		batchOwnerRequire, batchOwnerResume = originalRequire, originalResume
		batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport = originalStart, originalTick, originalReport
	})
	var order []string
	batchOwnerRequire = func(batchOwnerLease) error { return nil }
	batchOwnerResume = func(*batch.Owner) { order = append(order, "landing") }
	var launched func()
	batchOwnerCadenceStart = func(tick func()) {
		launched = tick
		tick()
	}
	batchOwnerCadenceTick = func(string, batchOwnerLease, func() time.Time) error {
		order = append(order, "cadence")
		return errors.New("injected cadence failure")
	}
	reports := 0
	batchOwnerCadenceReport = func(err error) {
		if !strings.Contains(err.Error(), "injected cadence failure") {
			t.Fatalf("reported error=%v", err)
		}
		reports++
	}
	stop := make(chan struct{})
	close(stop)
	clock := func() time.Time { return time.Unix(7, 0) }
	if err := loopBatchOwner(nil, batchOwnerLease{epoch: 3}, "landing-root", clock, time.Minute, make(chan struct{}), stop); err != nil {
		t.Fatal(err)
	}
	if launched == nil {
		t.Fatalf("owner order=%v launched=%t", order, launched != nil)
	}
	if strings.Join(order, ",") != "landing,cadence" || reports != 1 {
		t.Fatalf("cadence order=%v reports=%d", order, reports)
	}
}

func TestBatchOwnerLoopStopJoinsCadenceTick(t *testing.T) {
	originalRequire, originalResume := batchOwnerRequire, batchOwnerResume
	originalStart, originalTick, originalReport := batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport
	t.Cleanup(func() {
		batchOwnerRequire, batchOwnerResume = originalRequire, originalResume
		batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport = originalStart, originalTick, originalReport
	})

	started, finish := make(chan struct{}), make(chan struct{})
	starts := 0
	batchOwnerRequire = func(batchOwnerLease) error { return nil }
	batchOwnerResume = func(*batch.Owner) {}
	batchOwnerCadenceStart = func(tick func()) {
		starts++
		go tick()
	}
	batchOwnerCadenceTick = func(string, batchOwnerLease, func() time.Time) error {
		close(started)
		<-finish
		return nil
	}
	batchOwnerCadenceReport = func(error) {}

	stop := make(chan struct{})
	close(stop)
	cadence := newBatchOwnerCadence()
	done := make(chan error, 1)
	go func() {
		done <- loopBatchOwnerWithCadence(nil, batchOwnerLease{}, "landing-root", time.Now, time.Minute, make(chan struct{}), stop, cadence)
	}()
	<-started
	<-cadence.stopping
	runtime.Gosched()

	returnedBeforeTick := false
	select {
	case err := <-done:
		returnedBeforeTick = true
		if err != nil {
			t.Errorf("owner loop stop error=%v", err)
		}
	default:
	}
	runBatchOwnerPass(nil, batchOwnerLease{}, "landing-root", time.Now, cadence)
	if starts != 1 {
		t.Errorf("cadence starts after stop=%d, want one admitted tick", starts)
	}
	close(finish)
	if !returnedBeforeTick {
		if err := <-done; err != nil {
			t.Errorf("owner loop stop error=%v", err)
		}
	}
	select {
	case <-cadence.done:
	default:
		t.Error("owner loop returned without completing its cadence stop")
	}
	if returnedBeforeTick {
		t.Fatal("owner loop stop returned before its in-flight cadence tick ended")
	}
}
