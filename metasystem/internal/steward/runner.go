package steward

// The runner: the metasystem's own ticker, zero host footprint. One
// detached process per repository holds the runner lock and ticks
// until disarmed or the host stops. Arming mints the identity
// record, verifies the operator is reachable, and spawns the loop;
// any session's start ensures it, so the watchdog revives with the
// first metasystem activity after a boot.

import (
	"context"
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

	channelphase "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
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
	Pid             int64  `json:"pid"`
	StartTicks      int64  `json:"startTicks"`
	BootID          string `json:"bootId"`
	PidStartedAt    int64  `json:"pidStartedAt"` // seconds identity: darwin has no ticks pair
	StartedAt       string `json:"startedAt"`
	FenceGeneration int64  `json:"fenceGeneration"`
}

// StoppedError is the process-creation refusal returned while the checkout's
// fence is closed. Callers render its two lines without adding a family prefix.
type StoppedError struct {
	Checkout string
	Record   stopfence.Record
	Thing    string
	Raced    bool
}

func (e *StoppedError) Error() string {
	first, err := stopfence.ClosedDescription(e.Record, e.Checkout)
	if err != nil {
		return "cannot render stopped steward-runner refusal: " + err.Error()
	}
	if e.Raced {
		first += fmt.Sprintf("; while %s started, it has been ended", e.Thing)
	}
	command, err := stopfence.ClosedCommand(e.Record, e.Checkout)
	if err != nil {
		return "cannot render stopped steward-runner refusal: " + err.Error()
	}
	return first + "\nat an agent-free terminal, run: " + command
}

func stoppedError(checkout, thing string, record stopfence.Record, raced bool) error {
	return &StoppedError{Checkout: checkout, Record: record, Thing: thing, Raced: raced}
}

func readOpenFence(checkout, thing string) (stopfence.Record, error) {
	closed, record, err := stopfence.Closed(checkout)
	if err != nil {
		return stopfence.Record{}, fmt.Errorf("read process-creation fence: %w", err)
	}
	if closed {
		return record, stoppedError(checkout, thing, record, false)
	}
	return record, nil
}

func runnerRecordPath(repoRoot string) string {
	return filepath.Join(runnerDir(repoRoot), "runner.json")
}

var runnerAfterRecordPublished func()

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
	top := canonicalPath(repoRoot)
	fence, err := readOpenFence(top, "the steward runner")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(runnerDir(top), 0o755); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(runnerLockPath(top), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("a runner already guards this repository")
	}
	_ = os.Remove(runnerStopPath(top))

	self, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return fmt.Errorf("the runner cannot read its own identity")
	}
	claim, err := stopfence.Creating(top, "steward-run", fence.Generation, self.Ref())
	if err != nil {
		return fmt.Errorf("open steward runner creation claim: %w", err)
	}
	defer claim.Close()
	if err := writeJSONAtomic(runnerRecordPath(top), RunnerRecord{
		Pid: int64(os.Getpid()), StartTicks: self.StartTicks, BootID: self.BootID,
		PidStartedAt:    self.StartedAt.Unix(),
		StartedAt:       time.Now().UTC().Format(time.RFC3339),
		FenceGeneration: fence.Generation,
	}); err != nil {
		return err
	}
	defer os.Remove(runnerRecordPath(top))
	if runnerAfterRecordPublished != nil {
		runnerAfterRecordPublished()
	}
	closed, second, err := stopfence.Closed(top)
	if err != nil {
		return fmt.Errorf("re-read process-creation fence: %w", err)
	}
	if closed {
		return stoppedError(top, "the steward runner", second, true)
	}
	if second.Generation != fence.Generation {
		return fmt.Errorf("the checkout %s was stopped and armed again while the steward runner started; the steward runner has been ended; the caller may retry", top)
	}
	if err := claim.Close(); err != nil {
		return fmt.Errorf("close steward runner creation claim: %w", err)
	}

	for {
		if _, err := os.Stat(runnerStopPath(top)); err == nil {
			return nil
		}
		result, err := RunTick(top, TickConfig{}, census)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tick failed: %v\n", err)
		}
		resume := err == nil && result.Decision.Action == ActRevive
		if !resume {
			// A prepared intent whose launch never happened holds the
			// active-continuation guard and would otherwise never complete.
			// Resume it so CompleteRevival can re-arbitrate and either launch
			// or cancel on the current world.
			if _, ok, resumeErr := ResumableIntent(top); resumeErr == nil && ok {
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
				if qErr := QueueNotification(top, PendingNotification{
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
		if _, deliverErr := DeliverPending(top); deliverErr != nil {
			fmt.Fprintf(os.Stderr, "notifications pending: %v\n", deliverErr)
		}
		channelContext, cancelChannel := context.WithTimeout(context.Background(), 15*time.Second)
		if undelivered, channelErr := channelphase.Run(channelContext, top); channelErr != nil {
			fmt.Fprintf(os.Stderr, "channel pending: %d undelivered: %v\n", undelivered, channelErr)
		}
		cancelChannel()
		deadline := time.Now().Add(interval)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(runnerStopPath(top)); err == nil {
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
	outcome, err := arm(repoRoot, binaryPath, false, false, false, humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal))
	return outcome.Message, err
}

// ArmFixture is Arm when the caller's HUMAN classification came from the
// fixture-authority process table rather than a real terminal.
func ArmFixture(repoRoot, binaryPath string) (string, error) {
	outcome, err := arm(repoRoot, binaryPath, false, false, true, humanMintDecision("human-terminal", "", "", EnrollmentFixture))
	return outcome.Message, err
}

// ArmTemporary is Arm under a recorded remote human authorization: the
// human's verbatim word and their re-approval date ride the enrollment
// record itself, so the temporary state is visible to every reader until
// a terminal re-arm mints the next generation without them.
func ArmTemporary(repoRoot, binaryPath, humanWord, reviewBy string) (string, error) {
	if err := humanauthority.ValidateTemporaryWordPair(humanWord, reviewBy); err != nil {
		return "", err
	}
	if humanWord == "" {
		return "", fmt.Errorf("temporary steward arm requires the verbatim word and review-by date")
	}
	outcome, err := arm(repoRoot, binaryPath, true, false, false, humanMintDecision("human-word", humanWord, reviewBy, EnrollmentTemporaryWord))
	return outcome.Message, err
}

// ArmStage names the last enrollment transition that changed machine state.
type ArmStage int

const (
	StageBeforeMint ArmStage = iota
	StageStopAttempted
	StageStopped
	StageMinted
)

// ReArmOutcome records both the admitted provenance and the last transition,
// including a visible identity whose crash durability remains unconfirmed.
type ReArmOutcome struct {
	Status             string
	Stage              ArmStage
	Generation         int
	PreviousGeneration int
	RunnerPid          int64
	StoppedRunnerPid   int64
	EngineBuild        string
	LandedCommit       string
	LandingRef         string
	DurabilityPending  bool
	NoticeErr          error
}

type mintPlan struct {
	Skip         bool
	Message      string
	MintedBy     string
	Word         string
	ReviewBy     string
	Witnessed    int
	WitnessedAt  string
	EngineBuild  string
	LandedCommit string
	LandingRef   string
	Enrollment   string
}

type armOutcome struct {
	ReArmOutcome
	Message string
}

type mintDecision func(prior InstallIdentity, priorErr error, bytes enrolledBytes) (mintPlan, error)

func humanMintDecision(mintedBy, word, reviewBy, enrollment string) mintDecision {
	return func(_ InstallIdentity, _ error, bytes enrolledBytes) (mintPlan, error) {
		if bytes.Err != nil {
			return mintPlan{}, bytes.Err
		}
		return mintPlan{MintedBy: mintedBy, Word: word, ReviewBy: reviewBy, EngineBuild: bytes.Stamp, Enrollment: enrollment}, nil
	}
}

// ReArmRebuiltEngine replaces an enrolled engine only when the build stamp
// read from its changed bytes resolves to the installation's configured
// remote-tracking history. Caller identity is deliberately irrelevant.
func ReArmRebuiltEngine(repoRoot, installationRoot, invokingBinary string) (ReArmOutcome, error) {
	decision := func(prior InstallIdentity, priorErr error, bytes enrolledBytes) (mintPlan, error) {
		if priorErr != nil {
			return mintPlan{}, fmt.Errorf("%w: %v", ErrEnrollmentDrift, priorErr)
		}
		if canonicalPath(invokingBinary) != prior.InstallPath {
			return mintPlan{}, fmt.Errorf("%w: engine %q is not the enrolled engine %q", ErrEnrollmentDrift, canonicalPath(invokingBinary), prior.InstallPath)
		}
		if bytes.Err != nil {
			return mintPlan{}, bytes.Err
		}
		if bytes.Digest == prior.InstallDigest {
			return mintPlan{Skip: true, Message: fmt.Sprintf("already current (generation %d)", prior.Generation)}, nil
		}
		landingRef, err := readOwnedLandingRef(installationRoot)
		if err != nil {
			return mintPlan{}, fmt.Errorf("%w: %v", ErrEnrollmentDrift, err)
		}
		commit, err := resolveLandedBuild(repoRoot, installationRoot, landingRef, bytes.Stamp)
		if err != nil {
			return mintPlan{}, fmt.Errorf("%w: rebuilt engine at %s: %v", ErrEnrollmentDrift, prior.InstallPath, err)
		}
		witnessed, witnessedAt := prior.HumanWitnessedGeneration, prior.HumanWitnessedAt
		if prior.MintedBy == "" {
			witnessed, witnessedAt = 0, ""
		}
		return mintPlan{
			MintedBy: "machine-rebuild", Word: prior.TemporaryHumanWord, ReviewBy: prior.ReviewBy,
			Witnessed: witnessed, WitnessedAt: witnessedAt, EngineBuild: bytes.Stamp,
			LandedCommit: commit, LandingRef: landingRef, Enrollment: prior.Enrollment,
		}, nil
	}
	outcome, err := arm(repoRoot, invokingBinary, true, true, false, decision)
	return outcome.ReArmOutcome, err
}

// Restart replaces a live runner before arming the repository again. It is
// the repair path for a process that remains alive but no longer completes
// ticks.
func Restart(repoRoot, binaryPath string) (string, error) {
	outcome, err := arm(repoRoot, binaryPath, true, false, false, humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal))
	return outcome.Message, err
}

// RestartFixture is Restart under fixture-granted HUMAN classification.
func RestartFixture(repoRoot, binaryPath string) (string, error) {
	outcome, err := arm(repoRoot, binaryPath, true, false, true, humanMintDecision("human-terminal", "", "", EnrollmentFixture))
	return outcome.Message, err
}

// EnsureRunnerResult reports how session-start arming treated the steward.
// Excluded is reserved for fixture repositories and linked worktrees whose
// standing policy deliberately assigns the runner to another checkout.
type EnsureRunnerResult struct {
	Action     string
	Pid        int64
	Generation int
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}

func scaledRunnerWait(scaleMilli int) time.Duration {
	if scaleMilli < 1 {
		scaleMilli = 1000
	}
	seconds := (10*scaleMilli + 999) / 1000
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func waitForRunnerSuccess(repoRoot string, wait time.Duration) RoleVerdict {
	deadline := time.Now().Add(wait)
	verdict := checkStewardRunner(repoRoot, time.Now(), identity.KernelProber{})
	for verdict.Status != HealthAlive && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		verdict = checkStewardRunner(repoRoot, time.Now(), identity.KernelProber{})
	}
	return verdict
}

// EnsureRunner consults the standing enrolled engine, verifies a successful
// generation-bound tick, and restores the runner without minting a generation.
func EnsureRunner(repoRoot string, enrolled *EnrolledBinary, scaleMilli int) (EnsureRunnerResult, error) {
	top := canonicalPath(repoRoot)
	if fence, err := readOpenFence(top, "the steward runner"); err != nil {
		var stopped *StoppedError
		if errors.As(err, &stopped) {
			return EnsureRunnerResult{Action: "FENCED", Generation: int(fence.Generation)}, nil
		}
		return EnsureRunnerResult{}, err
	}
	if _, excluded := runnerExclusion(top, false); excluded {
		return EnsureRunnerResult{Action: "excluded"}, nil
	}
	if _, ok := NotifyCommand(top); !ok {
		return EnsureRunnerResult{}, fmt.Errorf("no notification channel is configured; an unreachable watchdog guards nothing — set metasystem.steward.notify-command")
	}
	if enrolled == nil || enrolled.file == nil {
		return EnsureRunnerResult{}, fmt.Errorf("the enrolled engine is not pinned")
	}
	wasAlive := false
	if _, alive := liveRunner(top); alive {
		wasAlive = true
	}
	if wasAlive {
		verdict := waitForRunnerSuccess(top, scaledRunnerWait(scaleMilli))
		if verdict.Status == HealthAlive {
			record, _ := liveRunner(top)
			return EnsureRunnerResult{Action: "verified", Pid: record.Pid, Generation: enrolled.Install.Generation}, nil
		}
		if verdict.Status == HealthUnknown {
			return EnsureRunnerResult{}, fmt.Errorf("steward runner cannot be verified: %s", verdict.Reason)
		}
	}
	repair, err := repairPinnedRunner(top, enrolled, nil, scaledRunnerWait(scaleMilli))
	if err != nil {
		return EnsureRunnerResult{}, err
	}
	if repair.Status != "RESTORED" && repair.Status != "CURRENT" {
		return EnsureRunnerResult{}, fmt.Errorf("steward runner repair stopped with %s", repair.Status)
	}
	action := "verified"
	if repair.Status == "RESTORED" {
		action = "started"
		if wasAlive {
			action = "replaced"
		}
	}
	record, alive := liveRunner(top)
	if !alive {
		return EnsureRunnerResult{}, fmt.Errorf("steward runner completed a pass but its process identity is no longer live")
	}
	return EnsureRunnerResult{Action: action, Pid: record.Pid, Generation: enrolled.Install.Generation}, nil
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
	if fence, err := readOpenFence(top, "the steward runner"); err != nil {
		var stopped *StoppedError
		if errors.As(err, &stopped) {
			return RunnerRepairOutcome{Status: "FENCED", Generation: int(fence.Generation)}, nil
		}
		return RunnerRepairOutcome{}, err
	}
	if _, err := os.Stat(RepoIdentityPath(top)); errors.Is(err, os.ErrNotExist) {
		return RunnerRepairOutcome{Status: "NOT_ENROLLED"}, nil
	}
	pinned, err := OpenEnrolledBinary(top)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		return RunnerRepairOutcome{}, err
	}
	return repairPinnedRunner(top, pinned, beforeLock, 10*time.Second)
}

func repairPinnedRunner(top string, pinned *EnrolledBinary, beforeLock func(), wait time.Duration) (RunnerRepairOutcome, error) {
	installed := pinned.Install
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
	if fence, err := readOpenFence(top, "the steward runner"); err != nil {
		var stopped *StoppedError
		if errors.As(err, &stopped) {
			return RunnerRepairOutcome{Status: "FENCED", Generation: int(fence.Generation)}, nil
		}
		return RunnerRepairOutcome{}, err
	}
	lockedInstalled, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	if lockedInstalled.Generation != installed.Generation || lockedInstalled.InstallPath != installed.InstallPath ||
		lockedInstalled.InstallDigest != installed.InstallDigest {
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
	replacement, err := launchRunner(top, pinned)
	if err != nil {
		return RunnerRepairOutcome{}, err
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if current := checkStewardRunner(top, time.Now(), identity.KernelProber{}); current.Status == HealthAlive {
			return RunnerRepairOutcome{
				Status: "RESTORED", Generation: installed.Generation,
				PreviousPid: previous.Pid, ReplacementPid: replacement.Pid,
			}, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return RunnerRepairOutcome{}, fmt.Errorf("replacement runner pid %d did not complete generation %d within %s", replacement.Pid, installed.Generation, wait)
}

func runnerExclusion(top string, allowFixture bool) (string, bool) {
	if commonDir, err := exec.Command("git", "-C", top, "rev-parse", "--git-common-dir").Output(); err == nil {
		if gitDir, dirErr := exec.Command("git", "-C", top, "rev-parse", "--git-dir").Output(); dirErr == nil &&
			canonicalGitPath(top, string(commonDir)) != canonicalGitPath(top, string(gitDir)) {
			return "linked worktree (the primary checkout owns the watchdog)", true
		}
	}
	if fixtureauth.FixtureModeRoot(top) && !allowFixture {
		return "fake-runtimes repository (fixtures arm deliberately)", true
	}
	return "", false
}

var beforeArmLock func()
var afterArmDecision func()

func arm(repoRoot, binaryPath string, replace, machine, allowFixture bool, decide mintDecision) (armOutcome, error) {
	outcome := armOutcome{ReArmOutcome: ReArmOutcome{Stage: StageBeforeMint}}
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return outcome, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(top); resolveErr == nil {
		top = resolved
	}
	if _, err := readOpenFence(top, "the steward runner"); err != nil {
		return outcome, err
	}
	if reason, excluded := runnerExclusion(top, allowFixture); excluded {
		outcome.Message = "not armed: " + reason
		return outcome, nil
	}
	if _, ok := NotifyCommand(top); !ok {
		return outcome, fmt.Errorf("no notification channel is configured; an unreachable watchdog guards nothing — set metasystem.steward.notify-command")
	}
	runnerPath := runnerDir(top)
	if err := os.MkdirAll(runnerPath, 0o755); err != nil {
		return outcome, fmt.Errorf("create runner directory %s: %w", runnerPath, err)
	}
	// One arm at a time. Every field and eligibility fact is read inside the
	// lock, and neither a refusal nor a no-op touches a live runner.
	armLockPath := filepath.Join(runnerPath, "arm.flock")
	armLock, err := os.OpenFile(armLockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return outcome, fmt.Errorf("open arm lock %s: %w", armLockPath, err)
	}
	defer armLock.Close()
	if beforeArmLock != nil {
		beforeArmLock()
	}
	if err := unix.Flock(int(armLock.Fd()), unix.LOCK_EX); err != nil {
		return outcome, fmt.Errorf("take arm lock %s: %w", armLockPath, err)
	}
	if _, err := readOpenFence(top, "the steward runner"); err != nil {
		return outcome, err
	}
	identityPath := RepoIdentityPath(top)
	prior, priorErr := VerifyIdentity(identityPath, top)
	if errors.Is(priorErr, os.ErrNotExist) {
		// Only the arm path owns publication recovery. A reader may observe the
		// marker before the first identity rename and must never clear it.
		_ = os.Remove(identityDurabilityPendingPath(identityPath))
	}
	if priorErr == nil {
		if _, markerErr := os.Stat(identityDurabilityPendingPath(identityPath)); markerErr == nil {
			durable, publishErr := publishIdentity(identityPath, prior)
			if publishErr != nil {
				return outcome, fmt.Errorf("re-publish identity with durability pending: %w", publishErr)
			}
			outcome.DurabilityPending = !durable
		}
	}
	bytesPath := canonicalPath(binaryPath)
	if machine && priorErr == nil {
		bytesPath = prior.InstallPath
	}
	bytes := readEnrolledBytes(InstallIdentity{InstallPath: bytesPath, InstallDigest: "candidate"})
	if bytes.File != nil {
		defer bytes.File.Close()
	}
	plan, err := decide(prior, priorErr, bytes)
	if err != nil {
		return outcome, err
	}
	if plan.Enrollment != EnrollmentTemporaryWord && fixtureauth.FixtureModeRoot(top) {
		plan.Enrollment = EnrollmentFixture
	}
	outcome.EngineBuild, outcome.LandedCommit, outcome.LandingRef = plan.EngineBuild, plan.LandedCommit, plan.LandingRef
	outcome.PreviousGeneration = prior.Generation
	if afterArmDecision != nil {
		afterArmDecision()
	}
	if plan.Skip {
		outcome.Status = "already-current"
		outcome.Generation = prior.Generation
		outcome.Message = plan.Message
		if rec, alive := liveRunner(top); alive {
			outcome.RunnerPid = rec.Pid
			outcome.Message += fmt.Sprintf(", runner pid %d", rec.Pid)
		}
		return outcome, nil
	}
	replaceForHumanTerminalWitness := false
	if rec, alive := liveRunner(top); alive {
		if !replace {
			humanTerminalBytesChanged := priorErr == nil && plan.MintedBy == "human-terminal" &&
				bytesPath == prior.InstallPath && bytes.Digest != prior.InstallDigest
			if !humanTerminalBytesChanged {
				outcome.RunnerPid = rec.Pid
				outcome.Message = fmt.Sprintf("already armed (runner pid %d); %s", rec.Pid, EnrollmentProvenance(prior))
				if prior.MintedBy == "" || prior.MintedBy == "machine-rebuild" {
					outcome.Message += " — run steward restart at the terminal to witness it"
				}
				return outcome, nil
			}
			replaceForHumanTerminalWitness = true
		}
		outcome.StoppedRunnerPid = rec.Pid
		outcome.Stage = StageStopAttempted
		if err := stopRunnerForReplacement(top, rec); err != nil {
			return outcome, err
		}
		outcome.Stage = StageStopped
	}
	generation := prior.Generation + 1
	mintedAt := time.Now().UTC().Format(time.RFC3339)
	witnessed, witnessedAt := plan.Witnessed, plan.WitnessedAt
	if plan.MintedBy != "machine-rebuild" {
		witnessed, witnessedAt = generation, mintedAt
	}
	durable, err := publishIdentity(identityPath, InstallIdentity{
		RepoIdentity: top, Generation: generation, InstallPath: bytesPath, InstallDigest: bytes.Digest, MintedAt: mintedAt,
		Enrollment:         plan.Enrollment,
		TemporaryHumanWord: plan.Word, ReviewBy: plan.ReviewBy, MintedBy: plan.MintedBy,
		HumanWitnessedGeneration: witnessed, HumanWitnessedAt: witnessedAt, EngineBuild: plan.EngineBuild,
		LandedCommit: plan.LandedCommit, LandingRef: plan.LandingRef,
	})
	if err != nil {
		return outcome, err
	}
	outcome.Stage = StageMinted
	outcome.Generation = generation
	outcome.DurabilityPending = !durable
	outcome.Status = "re-armed"
	if machine {
		outcome.NoticeErr = QueueNotification(top, PendingNotification{
			Nonce: fmt.Sprintf("engine-rearm-generation-%d", generation),
			Message: fmt.Sprintf("steward: re-armed the rebuilt engine: generation %d previous %d engine %s landed %s on %s",
				generation, prior.Generation, plan.EngineBuild, shortCommit(plan.LandedCommit), plan.LandingRef),
		})
	}
	pinned, err := OpenEnrolledBinary(top)
	if err != nil {
		return outcome, err
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		return outcome, err
	}
	record, err := launchRunner(top, pinned)
	if err != nil {
		return outcome, err
	}
	outcome.RunnerPid = record.Pid
	pending := ""
	if outcome.DurabilityPending {
		pending = " (durability pending)"
	}
	if machine {
		outcome.Message = fmt.Sprintf("armed (runner pid %d) (generation=%d previous=%d engine=%s landed=%s ref=%s)%s",
			record.Pid, generation, prior.Generation, plan.EngineBuild, shortCommit(plan.LandedCommit), plan.LandingRef, pending)
		return outcome, nil
	}
	if replaceForHumanTerminalWitness {
		outcome.Message = fmt.Sprintf("replaced live runner pid %d after the enrolled engine bytes changed; armed human-terminal generation %d with human witness %d (runner pid %d)%s",
			outcome.StoppedRunnerPid, generation, witnessed, record.Pid, pending)
		return outcome, nil
	}
	if plan.Word != "" {
		outcome.Message = fmt.Sprintf("armed TEMPORARILY under a recorded remote human word, review by %s (runner pid %d)%s", plan.ReviewBy, record.Pid, pending)
		return outcome, nil
	}
	outcome.Message = fmt.Sprintf("armed (runner pid %d)%s", record.Pid, pending)
	return outcome, nil
}

func launchRunner(repoRoot string, binary *EnrolledBinary) (RunnerRecord, error) {
	logFile, err := os.OpenFile(runnerLogPath(repoRoot), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return RunnerRecord{}, err
	}
	defer logFile.Close()
	cmd, err := binary.Command("steward", "run", "--repo", repoRoot)
	if err != nil {
		return RunnerRecord{}, err
	}
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

var runnerStopWriter = os.WriteFile
var runnerSignal = syscall.Kill

func stopRunnerForReplacement(repoRoot string, runner RunnerRecord) error {
	if err := runnerStopWriter(runnerStopPath(repoRoot), []byte("restart\n"), 0o644); err != nil {
		return fmt.Errorf("write restart marker before stopping runner pid %d: %w", runner.Pid, err)
	}
	if err := runnerSignal(int(runner.Pid), syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("stop runner pid %d for replacement: %w", runner.Pid, err)
	}
	// A stalled runner may itself be stopped, so let it receive the termination
	// signal before deciding whether a hard stop is necessary.
	_ = runnerSignal(int(runner.Pid), syscall.SIGCONT)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := liveRunner(repoRoot); !alive {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if current, alive := liveRunner(repoRoot); alive && current.Pid == runner.Pid {
		if err := runnerSignal(int(runner.Pid), syscall.SIGKILL); err != nil && err != syscall.ESRCH {
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

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// EnrollmentProvenance renders the one standing explanation of who minted an
// enrollment and, for a machine rebuild, which landed bytes it consumed.
func EnrollmentProvenance(id InstallIdentity) string {
	var result string
	switch id.MintedBy {
	case "human-terminal", "human-word":
		result = fmt.Sprintf("enrollment generation %d human-witnessed (engine %s)", id.Generation, id.EngineBuild)
	case "machine-rebuild":
		result = fmt.Sprintf("enrollment generation %d machine-minted (rebuild, engine %s, landed %s on %s)",
			id.Generation, id.EngineBuild, shortCommit(id.LandedCommit), id.LandingRef)
		if id.HumanWitnessedGeneration > 0 {
			result += fmt.Sprintf(" above human-witnessed generation %d of %s", id.HumanWitnessedGeneration, id.HumanWitnessedAt)
		} else {
			result += "; no human witness is recorded"
		}
	default:
		result = fmt.Sprintf("enrollment generation %d LEGACY (minted before provenance stamping; no human witness recorded)", id.Generation)
	}
	if id.TemporaryHumanWord != "" {
		result += fmt.Sprintf("; TEMPORARY under a recorded remote human word, review by %s", id.ReviewBy)
	}
	return result
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

// RunnerStopOutcome records the identity-safe ladder used to end a steward
// runner. Signal is none, term, or kill; Result is stopped, already-gone, or
// not-stopped.
type RunnerStopOutcome struct {
	Record RunnerRecord
	Signal string
	Result string
	Reason string
}

// LongForm preserves the steward disarm command's established output.
func (o RunnerStopOutcome) LongForm() string {
	switch {
	case o.Result == "already-gone":
		return "not armed"
	case o.Result == "stopped" && o.Signal == "none":
		return "disarmed"
	case o.Result == "stopped":
		return "disarmed (signalled)"
	default:
		return "not disarmed: " + o.Reason
	}
}

func runnerStopWait(root string, seconds int) (time.Duration, error) {
	scale := 1000
	if raw := os.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, fmt.Errorf("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
		}
		scale = parsed
	}
	wait := time.Duration(seconds) * time.Second * time.Duration(scale) / 1000
	if wait < 10*time.Millisecond {
		wait = 10 * time.Millisecond
	}
	return wait, nil
}

func sameRunner(left, right RunnerRecord) bool {
	if left.Pid != right.Pid || left.Pid < 1 {
		return false
	}
	if left.StartTicks > 0 && left.BootID != "" && right.StartTicks > 0 && right.BootID != "" {
		return left.StartTicks == right.StartTicks && left.BootID == right.BootID
	}
	return left.PidStartedAt > 0 && left.PidStartedAt == right.PidStartedAt
}

func waitForRunnerGone(root string, record RunnerRecord, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		current, alive := liveRunner(root)
		if !alive || !sameRunner(record, current) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	current, alive := liveRunner(root)
	return !alive || !sameRunner(record, current)
}

// Disarm stops the runner through its orderly marker, then TERM and KILL,
// re-proving the recorded identity beside every signal.
func Disarm(repoRoot string) (RunnerStopOutcome, error) {
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return RunnerStopOutcome{}, err
	}
	rec, alive := liveRunner(top)
	if !alive {
		return RunnerStopOutcome{Record: rec, Signal: "none", Result: "already-gone", Reason: "not armed"}, nil
	}
	outcome := RunnerStopOutcome{Record: rec, Signal: "none", Result: "not-stopped"}
	if err := runnerStopWriter(runnerStopPath(top), []byte("disarm\n"), 0o644); err != nil {
		outcome.Reason = "write orderly stop marker: " + err.Error()
		return outcome, err
	}
	orderlyWait, err := runnerStopWait(top, 5)
	if err != nil {
		outcome.Reason = err.Error()
		return outcome, err
	}
	if waitForRunnerGone(top, rec, orderlyWait) {
		outcome.Result = "stopped"
		outcome.Reason = "orderly stop marker"
		return outcome, nil
	}
	current, still := liveRunner(top)
	if !still || !sameRunner(rec, current) {
		outcome.Result = "already-gone"
		outcome.Reason = "identity changed before TERM"
		return outcome, nil
	}
	outcome.Signal = "term"
	if err := runnerSignal(int(rec.Pid), syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		outcome.Reason = "TERM failed: " + err.Error()
		return outcome, err
	}
	termWait, err := runnerStopWait(top, 2)
	if err != nil {
		outcome.Reason = err.Error()
		return outcome, err
	}
	if waitForRunnerGone(top, rec, termWait) {
		outcome.Result = "stopped"
		outcome.Reason = "TERM"
		return outcome, nil
	}
	current, still = liveRunner(top)
	if !still || !sameRunner(rec, current) {
		outcome.Result = "already-gone"
		outcome.Reason = "identity changed before KILL"
		return outcome, nil
	}
	outcome.Signal = "kill"
	if err := runnerSignal(int(rec.Pid), syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		outcome.Reason = "KILL failed: " + err.Error()
		return outcome, err
	}
	if waitForRunnerGone(top, rec, termWait) {
		outcome.Result = "stopped"
		outcome.Reason = "KILL after TERM was ignored"
		return outcome, nil
	}
	outcome.Reason = "runner remained alive after KILL"
	return outcome, nil
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

// LiveRunner returns the runner only when its recorded kernel identity is
// still the same live process. It is read-only and never adopts a pid from
// record shape alone.
func LiveRunner(repoRoot string) (RunnerRecord, bool) {
	return liveRunner(canonicalPath(repoRoot))
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
