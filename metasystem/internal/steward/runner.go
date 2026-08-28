package steward

// The runner: the metasystem's own ticker, zero host footprint. One
// detached process per repository holds the runner lock and ticks
// until disarmed or the host stops. Arming mints the identity
// record, verifies the operator is reachable, and spawns the loop;
// any session's start ensures it, so the watchdog revives with the
// first metasystem activity after a boot.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func runnerDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward")
}
func runnerLockPath(repoRoot string) string {
	return filepath.Join(runnerDir(repoRoot), "runner.flock")
}
func runnerStopPath(repoRoot string) string { return filepath.Join(runnerDir(repoRoot), "stop") }
func runnerLogPath(repoRoot string) string  { return filepath.Join(runnerDir(repoRoot), "runner.log") }

// RunnerRecord names the live runner for status and disarm.
type RunnerRecord struct {
	Pid          int64  `json:"pid"`
	StartTicks   int64  `json:"startTicks"`
	BootID       string `json:"bootId"`
	PidStartedAt int64  `json:"pidStartedAt"` // seconds identity: darwin has no ticks pair
	StartedAt    string `json:"startedAt"`
}

func runnerRecordPath(repoRoot string) string {
	return filepath.Join(runnerDir(repoRoot), "runner.json")
}

// TickSeconds reads the cadence; the default is ten minutes.
func TickSeconds(repoRoot string) int {
	out, err := exec.Command("git", "-C", repoRoot, "config", "--get", "metasystem.steward.tick-seconds").Output()
	if err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
			return n
		}
	}
	return 600
}

// RunLoop is the runner's body: acquire the per-repository lock
// (refusing beside a live runner), then tick until the stop file
// appears. Each pass reaps, decides, drives any lawful revival, and then
// retries pending notifications.
// launcher. The loop never crashes out of a tick: a failed pass is
// reported and the next tick tries again.
func RunLoop(repoRoot string, census WorkerCensus, revive func() error, interval time.Duration) error {
	if err := os.MkdirAll(runnerDir(repoRoot), 0o755); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(runnerLockPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("a runner already guards this repository")
	}
	_ = os.Remove(runnerStopPath(repoRoot))

	self, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return fmt.Errorf("the runner cannot read its own identity")
	}
	if err := writeJSONAtomic(runnerRecordPath(repoRoot), RunnerRecord{
		Pid: int64(os.Getpid()), StartTicks: self.StartTicks, BootID: self.BootID,
		PidStartedAt: self.StartedAt.Unix(),
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	defer os.Remove(runnerRecordPath(repoRoot))

	for {
		if _, err := os.Stat(runnerStopPath(repoRoot)); err == nil {
			return nil
		}
		result, err := RunTick(repoRoot, TickConfig{}, census)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tick failed: %v\n", err)
		}
		resume := err == nil && result.Decision.Action == ActRevive
		if !resume {
			// A prepared intent whose launch never happened holds the
			// active-continuation guard and would otherwise never complete.
			// Resume it so CompleteRevival can re-arbitrate and either launch
			// or cancel on the current world.
			if _, ok, resumeErr := ResumableIntent(repoRoot); resumeErr == nil && ok {
				resume = true
			}
		}
		if resume && revive != nil {
			if reviveErr := revive(); reviveErr != nil {
				fmt.Fprintf(os.Stderr, "revival failed: %v\n", reviveErr)
				// The failure is an incident, not a log line: a revival
				// that dies before minting its intent would otherwise
				// retry silently every tick — armed, dead worker, open
				// goal, and both visibility channels quiet.
				if qErr := QueueNotification(repoRoot, PendingNotification{
					Nonce:   "revive-failure",
					Message: "steward: revival failed — " + reviveErr.Error(),
				}); qErr != nil {
					fmt.Fprintf(os.Stderr, "revive-failure incident could not queue: %v\n", qErr)
				}
			}
		}
		// Recovery runs before delivery. A failed recovery queues its incident
		// above and can reach the operator in this same pass; a successful one
		// leaves only silent history.
		if _, deliverErr := DeliverPending(repoRoot); deliverErr != nil {
			fmt.Fprintf(os.Stderr, "notifications pending: %v\n", deliverErr)
		}
		deadline := time.Now().Add(interval)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(runnerStopPath(repoRoot)); err == nil {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// Arm makes the repository guarded: mint the identity record for the
// repo's own binary, verify the operator is reachable, and spawn the
// detached runner unless one already lives. Idempotent.
func Arm(repoRoot, binaryPath string) (string, error) {
	return arm(repoRoot, binaryPath, false)
}

// Restart replaces a live runner before arming the repository again. It is
// the repair path for a process that remains alive but no longer completes
// ticks.
func Restart(repoRoot, binaryPath string) (string, error) {
	return arm(repoRoot, binaryPath, true)
}

// RunnerRepairOutcome says whether the watcher found the enrolled steward
// current or restored that same installation generation.
type RunnerRepairOutcome struct {
	Status         string
	Generation     int
	PreviousPid    int64
	ReplacementPid int64
}

// RepairEnrolledRunner is the watcher's narrow repair path. It never mints an
// installation identity: it may launch only the binary and generation already
// enrolled in this checkout.
func RepairEnrolledRunner(repoRoot string) (RunnerRepairOutcome, error) {
	return repairEnrolledRunner(repoRoot, nil)
}

func repairEnrolledRunner(repoRoot string, beforeLock func()) (RunnerRepairOutcome, error) {
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(top); resolveErr == nil {
		top = resolved
	}
	installed, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RunnerRepairOutcome{Status: "NOT_ENROLLED"}, nil
		}
		return RunnerRepairOutcome{}, err
	}
	if installed.InstallPath == "" {
		return RunnerRepairOutcome{}, fmt.Errorf("the enrolled steward has no installation path")
	}
	bin, err := filepath.Abs(installed.InstallPath)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(bin); resolveErr == nil {
		bin = resolved
	}
	if info, statErr := os.Stat(bin); statErr != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return RunnerRepairOutcome{}, fmt.Errorf("the enrolled steward binary %q is not executable", bin)
	}
	if beforeLock != nil {
		beforeLock()
	}
	if err := os.MkdirAll(runnerDir(top), 0o755); err != nil {
		return RunnerRepairOutcome{}, err
	}
	armLock, err := os.OpenFile(filepath.Join(runnerDir(top), "arm.flock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	defer armLock.Close()
	if err := unix.Flock(int(armLock.Fd()), unix.LOCK_EX); err != nil {
		return RunnerRepairOutcome{}, err
	}
	lockedInstalled, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	if lockedInstalled.Generation != installed.Generation || lockedInstalled.InstallPath != installed.InstallPath {
		return RunnerRepairOutcome{Status: "ENROLLMENT_CHANGED", Generation: installed.Generation}, nil
	}
	ended, err := AutoHealingEnded(top, RoleStewardRunner)
	if err != nil {
		return RunnerRepairOutcome{}, fmt.Errorf("read steward health breaker: %w", err)
	}
	if ended {
		return RunnerRepairOutcome{Status: AutoHealEnded, Generation: installed.Generation}, nil
	}

	verdict := checkStewardRunner(top, time.Now(), identity.KernelProber{})
	if verdict.Status == HealthAlive {
		record, _ := liveRunner(top)
		return RunnerRepairOutcome{Status: "CURRENT", Generation: installed.Generation, ReplacementPid: record.Pid}, nil
	}
	if verdict.Status == HealthUnknown {
		return RunnerRepairOutcome{}, fmt.Errorf("steward freshness is unknown: %s", verdict.Reason)
	}
	previous, alive := liveRunner(top)
	if alive {
		if err := stopRunnerForReplacement(top, previous); err != nil {
			return RunnerRepairOutcome{}, err
		}
	}
	replacement, err := launchRunner(top, bin)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if current := checkStewardRunner(top, time.Now(), identity.KernelProber{}); current.Status == HealthAlive {
			return RunnerRepairOutcome{
				Status: "RESTORED", Generation: installed.Generation,
				PreviousPid: previous.Pid, ReplacementPid: replacement.Pid,
			}, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return RunnerRepairOutcome{}, fmt.Errorf("replacement runner pid %d did not complete generation %d within ten seconds", replacement.Pid, installed.Generation)
}

func arm(repoRoot, binaryPath string, replace bool) (string, error) {
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(top); resolveErr == nil {
		top = resolved
	}
	if commonDir, err := exec.Command("git", "-C", top, "rev-parse", "--git-common-dir").Output(); err == nil {
		if gitDir, dirErr := exec.Command("git", "-C", top, "rev-parse", "--git-dir").Output(); dirErr == nil &&
			canonicalGitPath(top, string(commonDir)) != canonicalGitPath(top, string(gitDir)) {
			// A linked worktree is a delegate's disposable copy: a
			// watchdog armed there would outlive the job and guard a
			// directory built to be deleted. The comparison is on
			// CANONICAL paths: git answers relative from one flag and
			// absolute from the other inside a subdirectory checkout,
			// and a raw string mismatch would refuse every primary
			// checkout that is not the repository root.
			return "not armed: linked worktree (the primary checkout owns the watchdog)", nil
		}
	}
	if config.ConfValue(filepath.Join(top, "metasystem.conf"), "metasystem.runtimes", "") == "fake" {
		// A fake-runtimes repository is a fixture world: sessions come
		// and go by the thousand and their repositories are deleted
		// minutes later. A leaked runner ticking a dead directory is
		// exactly the leak class the suite hygiene rules name, so the
		// ambient arming path stays out; a fixture that wants a runner
		// arms it deliberately.
		return "not armed: fake-runtimes repository (fixtures arm deliberately)", nil
	}
	if _, ok := NotifyCommand(top); !ok {
		return "", fmt.Errorf("no notification channel is configured; an unreachable watchdog guards nothing — set metasystem.steward.notify-command")
	}
	if err := os.MkdirAll(runnerDir(top), 0o755); err != nil {
		return "", err
	}
	// One arm at a time, and EVERYTHING an arm changes happens inside
	// the lock: identity generations cannot be superseded by a racing
	// arm, and the second contender finds the first's live runner.
	armLock, err := os.OpenFile(filepath.Join(runnerDir(top), "arm.flock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", err
	}
	defer armLock.Close()
	if err := unix.Flock(int(armLock.Fd()), unix.LOCK_EX); err != nil {
		return "", err
	}
	if rec, alive := liveRunner(top); alive {
		if !replace {
			return fmt.Sprintf("already armed (runner pid %d)", rec.Pid), nil
		}
		if err := stopRunnerForReplacement(top, rec); err != nil {
			return "", err
		}
	}
	prior, _ := VerifyIdentity(RepoIdentityPath(top), top)
	bin, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", err
	}
	if err := MintIdentity(RepoIdentityPath(top), InstallIdentity{
		RepoIdentity: top, Generation: prior.Generation + 1,
		InstallPath: bin, MintedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	record, err := launchRunner(top, bin)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("armed (runner pid %d)", record.Pid), nil
}

func launchRunner(repoRoot, binaryPath string) (RunnerRecord, error) {
	logFile, err := os.OpenFile(runnerLogPath(repoRoot), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return RunnerRecord{}, err
	}
	defer logFile.Close()
	cmd := exec.Command(binaryPath, "steward", "run", "--repo", repoRoot)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return RunnerRecord{}, err
	}
	// The runner is detached on purpose: it must outlive this launch.
	go func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rec, alive := liveRunner(repoRoot); alive {
			return rec, nil
		}
		if _, probeState, _ := (identity.KernelProber{}).Probe(int64(cmd.Process.Pid)); probeState == identity.Dead {
			return RunnerRecord{}, fmt.Errorf("the runner died before guarding the repository; see %s", runnerLogPath(repoRoot))
		}
		time.Sleep(50 * time.Millisecond)
	}
	return RunnerRecord{}, fmt.Errorf("the runner did not confirm within ten seconds; see %s", runnerLogPath(repoRoot))
}

func stopRunnerForReplacement(repoRoot string, runner RunnerRecord) error {
	if err := os.WriteFile(runnerStopPath(repoRoot), []byte("restart\n"), 0o644); err != nil {
		return err
	}
	if err := syscall.Kill(int(runner.Pid), syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("stop runner pid %d for replacement: %w", runner.Pid, err)
	}
	// A stalled runner may itself be stopped, so let it receive the termination
	// signal before deciding whether a hard stop is necessary.
	_ = syscall.Kill(int(runner.Pid), syscall.SIGCONT)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := liveRunner(repoRoot); !alive {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if current, alive := liveRunner(repoRoot); alive && current.Pid == runner.Pid {
		if err := syscall.Kill(int(runner.Pid), syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("kill stalled runner pid %d for replacement: %w", runner.Pid, err)
		}
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := liveRunner(repoRoot); !alive {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("runner pid %d remained alive after replacement stop", runner.Pid)
}

// canonicalGitPath resolves one git rev-parse answer to a canonical
// absolute path: relative answers are relative to the queried
// directory, and symlinks (darwin's /var vs /private/var) resolve.
func canonicalGitPath(base, answer string) string {
	p := strings.TrimSpace(answer)
	if p == "" {
		return p
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}

// Disarm stops the runner: the stop file ends the loop at its next
// check; a runner that lingers past the grace gets the signal.
func Disarm(repoRoot string) (string, error) {
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	rec, alive := liveRunner(top)
	if !alive {
		return "not armed", nil
	}
	if err := os.WriteFile(runnerStopPath(top), []byte("disarm\n"), 0o644); err != nil {
		return "", err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, still := liveRunner(top); !still {
			return "disarmed", nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Revalidate beside the signal: five seconds passed since the
	// last look, and a freed pid may already belong to a stranger.
	if recNow, still := liveRunner(top); still && recNow.Pid == rec.Pid {
		_ = syscall.Kill(int(rec.Pid), syscall.SIGTERM)
		return "disarmed (signalled)", nil
	}
	return "disarmed", nil
}

// liveRunner reads the record and proves the process by the
// clock-step-immune pair; a dead or reused pid is not a runner.
func liveRunner(repoRoot string) (RunnerRecord, bool) {
	var rec RunnerRecord
	if err := readJSON(runnerRecordPath(repoRoot), &rec); err != nil {
		return rec, false
	}
	live, state, err := identity.KernelProber{}.Probe(rec.Pid)
	if err != nil || state != identity.Alive {
		return rec, false
	}
	if rec.StartTicks > 0 && rec.BootID != "" && live.StartTicks > 0 && live.BootID != "" {
		return rec, live.StartTicks == rec.StartTicks && live.BootID == rec.BootID
	}
	if rec.PidStartedAt > 0 {
		// The seconds identity decides where no ticks pair exists: a
		// pid reused by an unrelated process is not our runner, and
		// neither arm nor disarm may treat it as one.
		return rec, live.StartedAt.Unix() == rec.PidStartedAt
	}
	// A record with no identity at all proves nothing: treat the
	// runner as absent rather than adopt a stranger.
	return rec, false
}

// writeJSONAtomic and readJSON are the runner record's disk shape.
func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
