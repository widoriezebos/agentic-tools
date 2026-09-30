package delegation

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

func jsonObject(pairs []string) (string, error) { return jsonedit.Object(pairs) }

func (s *session) brainFence(act string) string {
	return brain.Fence(s.root, act, s.l.ports.Goal.LedgerIdentity())
}

// emit is emit_event with the dispatch component: best effort, never fails.
// fields carries the reserved and payload keys; summary is its own argument.
func (s *session) emit(event string, fields map[string]string) {
	summary := fields["summary"]
	payload := make(map[string]string, len(fields))
	for key, value := range fields {
		if key != "summary" {
			payload[key] = value
		}
	}
	s.l.ports.Events.Emit(event, summary, payload)
}

// engineSkewPreflight refuses a dispatch whose engine is older than the
// checkout when engine or agent sources changed since the engine's commit.
// The stamp is the running engine's build stamp unless a focused fixture
// supplies one.
func (s *session) engineSkewPreflight(stamp string) error {
	if stamp == "" {
		stamp = supervise.BuildStamp
	}
	if stamp == "" || stamp == "dev" {
		return nil
	}
	out, _, err := s.l.ports.Git.Run(s.ctx, s.repoScope, "log", "--format=commit %H", "--name-only", "--ancestry-path", stamp+"..HEAD")
	if err != nil {
		return nil
	}
	text := string(out)
	checkoutCommit := text
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		checkoutCommit = text[:index]
	}
	checkoutCommit = strings.TrimPrefix(checkoutCommit, "commit ")
	prefix := ""
	if s.root != s.repoScope {
		prefix = strings.TrimPrefix(s.root, s.repoScope+"/") + "/"
	}
	relevant := false
	for _, line := range strings.Split(text, "\n") {
		for _, tree := range steward.EngineSkewPathspecs() {
			if strings.HasPrefix(line, prefix+tree+"/") {
				relevant = true
			}
		}
	}
	if checkoutCommit != "" && relevant {
		return s.die(1, fmt.Sprintf("dispatch refused: the engine (%s) is older than this checkout (%s), and engine scripts changed\nrebuild with go run ./cmd/devgate build, then arm the steward again", stamp, checkoutCommit))
	}
	return nil
}

// reportPlanDrift is report_plan_drift: STALE-PLAN lines on stderr, never a
// refusal.
func (s *session) reportPlanDrift() {
	for _, line := range report.OpenWork(s.root) {
		if strings.HasPrefix(line, "STALE-PLAN") {
			s.eprintln(line)
		}
	}
}

// requireFreshCensus is require_fresh_census: the engine's freshness and
// fingerprint verdict, or its exit code.
func (s *session) requireFreshCensus() error {
	verdict := filepath.Join(s.agents, "supervision", "last-census.json")
	state := filepath.Join(s.agents, "supervision", "state.json")
	// The re-arm remedy named in census refusals: the public form that arms
	// a checkout (the `up` owner).
	arm := "metasystem system start"
	if !exists(verdict) {
		return s.die(1, fmt.Sprintf("dispatch refused: census verdict is absent; run %s --repo %s", arm, s.repoScope))
	}
	return s.censusFresh(verdict, state, arm)
}

// requireOpenDispatchFence is require_open_dispatch_fence.
func (s *session) requireOpenDispatchFence() error {
	result, err := dispatch.FenceBeforeLaunch(s.root)
	if err != nil {
		s.eprintln(err.Error())
		return s.die(1, "dispatch refused: the process-creation fence could not be read")
	}
	encoded := encodeJSON(result)
	if result.Outcome == dispatch.LaunchFenceRefusedStopped {
		s.recordOutcomeRaw(encoded)
		s.println(encoded)
		return exitWith(1)
	}
	if result.Outcome != dispatch.LaunchFenceOpen {
		return s.die(1, "dispatch refused: the process-creation fence could not be read")
	}
	return nil
}

// closeCreationClaim is close_creation_claim.
func (s *session) closeCreationClaim(claim string) error {
	if claim == "" {
		return nil
	}
	if err := closeStopFenceClaim(claim); err != nil {
		s.eprintln("stopfence creating-close: " + err.Error())
		return exitWith(1)
	}
	if s.cleanupCreationClaim == claim {
		s.cleanupCreationClaim = ""
	}
	return nil
}

func (s *session) cleanupCreationClaimQuietly() {
	claim := s.cleanupCreationClaim
	if claim == "" {
		return
	}
	if closeStopFenceClaim(claim) == nil {
		s.cleanupCreationClaim = ""
	}
}

func (s *session) cleanupCompositionTemporaries() {
	if s.cleanupPrompt != "" {
		_ = os.Remove(s.cleanupPrompt)
	}
	if s.cleanupComposition != "" {
		_ = os.Remove(s.cleanupComposition)
	}
	if s.cleanupStage != "" {
		_ = os.RemoveAll(s.cleanupStage)
	}
	s.cleanupPrompt, s.cleanupComposition, s.cleanupStage = "", "", ""
}

func (s *session) cleanupSubjectTemp() {
	removeQuietly(s.cleanupSubject)
	s.cleanupSubject = ""
	removeQuietly(s.cleanupAdmission)
	s.cleanupAdmission = ""
}

func (s *session) cleanupFollowUpMessage() {
	removeQuietly(s.cleanupMessage)
	s.cleanupMessage = ""
	removeQuietly(s.cleanupContinuation)
	s.cleanupContinuation = ""
}

// --- owner locks ---------------------------------------------------------

// ownerLock is owner_lock: 0 done, 3 busy, 4 not-owner, 1 mechanical.
func (s *session) ownerLockClaim(directory string) int {
	switch err := dispatch.OwnerLockClaim(directory, s.inv.CallerPid, s.tag); {
	case err == nil:
		return 0
	case errors.Is(err, dispatch.ErrOwnerLockBusy):
		return 3
	default:
		s.eprintln(err.Error())
		return 1
	}
}

func (s *session) ownerLockRelease(directory string) int {
	switch err := dispatch.OwnerLockRelease(directory, s.inv.CallerPid, s.tag); {
	case err == nil:
		return 0
	case errors.Is(err, dispatch.ErrOwnerLockNotOwner):
		return 4
	default:
		s.eprintln(err.Error())
		return 1
	}
}

// lockHolder names a lock's recorded holder for a LOCK_BUSY line.
func lockHolder(directory string) string {
	owner := filepath.Join(directory, "owner.json")
	pid := fieldOr(owner, "pid")
	tag := fieldOr(owner, "instanceTag")
	if pid == "" && tag == "" {
		return "unreadable"
	}
	if pid == "" {
		pid = "unknown"
	}
	if tag == "" {
		tag = "unknown"
	}
	return "pid=" + pid + ",tag=" + tag
}

// holderWords names a lock's recorded holder for a person: its process
// and instance tag, or that the record cannot be read.
func holderWords(directory string) string {
	owner := filepath.Join(directory, "owner.json")
	pid := fieldOr(owner, "pid")
	tag := fieldOr(owner, "instanceTag")
	switch {
	case pid == "" && tag == "":
		return "a holder whose record cannot be read"
	case tag == "":
		return "process " + pid
	case pid == "":
		return "instance " + tag
	}
	return "process " + pid + " (" + tag + ")"
}

// lockBusyMessage is the two lines a busy lock shows a person: what is
// held and by whom (and how long this waited, when it did), then what to
// do. The ranked LOCK_BUSY detail stays in the outcome record.
func lockBusyMessage(what, holder string, waited int64) string {
	line := what + " is busy: " + holder + " holds it"
	if waited >= 0 {
		line += fmt.Sprintf(" (waited %ds)", waited)
	}
	return line + "\nnothing was done; run the same command again once it is released"
}

// staleClaimMessage refuses a goal revision bound under another claim than
// the goal's current one.
func staleClaimMessage(goalID string, revision uint64, bound, current string) string {
	return fmt.Sprintf("goal %s revision %d was bound under claim %s, but the goal's current claim is %s\nnothing was dispatched; dispatch again under the current claim", goalID, revision, bound, current)
}

// acquireChainLock is acquire_chain_lock: one attempt; a live holder refuses.
func (s *session) acquireChainLock(chain string) error {
	dir := filepath.Join(s.locks, chain+".d")
	if err := os.MkdirAll(s.locks, 0o755); err != nil {
		return s.die(1, err.Error())
	}
	if s.ownerLockClaim(dir) == 0 {
		return nil
	}
	owner := filepath.Join(dir, "owner.json")
	pid := fieldOr(owner, "pid")
	tag := fieldOr(owner, "instanceTag")
	if pid == "" {
		return s.die(1, "chain lock has no owner lease: "+dir)
	}
	parsed, _ := strconv.ParseInt(pid, 10, 64)
	if s.tagState(parsed, pid, tag) == "unknown" {
		return s.die(1, "chain lock owner liveness cannot be verified: "+chain)
	}
	return s.die(1, "chain is busy: "+chain)
}

// tagState is tag_state over a raw pid string.
func (s *session) tagState(pid int64, raw, tag string) string {
	if !positiveInteger(raw) {
		return "dead"
	}
	return s.l.ports.Process.TagState(pid, tag)
}

// waitForLock is the bounded wait every ranked lock shares: claim until the
// holder releases or the scaled cap passes. It returns the claim status (0,
// 3 on the cap, other on a mechanical failure), the elapsed seconds and the
// scaled cap.
func (s *session) waitForLock(directory string, base int64) (int, int64, int64, error) {
	maximum, err := s.fixtureWaitCap(base)
	if err != nil {
		return 1, 0, 0, err
	}
	clock := s.l.ports.Clock
	started := clock.Now()
	deadline := started.Add(time.Duration(maximum) * time.Second)
	for {
		status := s.ownerLockClaim(directory)
		if status != 3 {
			return status, int64(clock.Now().Sub(started) / time.Second), maximum, nil
		}
		if !clock.Now().Before(deadline) {
			return 3, int64(clock.Now().Sub(started) / time.Second), maximum, nil
		}
		clock.Sleep(50 * time.Millisecond)
	}
}

// acquireLaunchChainLock is acquire_launch_chain_lock.
func (s *session) acquireLaunchChainLock(chain string) error {
	dir := filepath.Join(s.locks, chain+".d")
	if err := os.MkdirAll(s.locks, 0o755); err != nil {
		return s.die(1, err.Error())
	}
	status, _, _, err := s.waitForLock(dir, 10)
	if err != nil {
		return err
	}
	switch status {
	case 0:
		return nil
	case 3:
		s.recordOutcome("LOCK_BUSY", "refused", fmt.Sprintf("rank=chain key=%s holder=%s retry=retry-after-the-named-holder-releases", chain, lockHolder(dir)), s.outcomeJob())
		return s.die(1, lockBusyMessage("launch chain "+chain, holderWords(dir), -1))
	}
	return s.die(1, "cannot acquire launch chain lock: "+chain)
}

// outcomeJob is ${job:-${child:-}}.
func (s *session) outcomeJob() string {
	if s.dieJob != "" {
		return s.dieJob
	}
	return s.dieChild
}

// releaseChainLock is release_chain_lock.
func (s *session) releaseChainLock(chain string) error {
	if chain == "" {
		return nil
	}
	if s.ownerLockRelease(filepath.Join(s.locks, chain+".d")) == 4 {
		return s.die(1, "refusing to release another owner's chain lock")
	}
	return nil
}

// acquireGoalRevisionLock is acquire_goal_revision_lock.
func (s *session) acquireGoalRevisionLock(goalID string, revision uint64) error {
	dir, err := dispatch.GoalRevisionLockDir(s.root, goalID, revision)
	if err != nil {
		s.eprintln(err.Error())
		return s.die(1, fmt.Sprintf("cannot resolve goal-revision lock for %s revision %d", goalID, revision))
	}
	s.goalLockDir = dir
	status, elapsed, maximum, err := s.waitForLock(dir, 10)
	if err != nil {
		return err
	}
	if status != 0 {
		detail := fmt.Sprintf("rank=goal-revision key=%s/r%d holder=%s retry=retry-after-the-named-holder-releases elapsed=%ds cap=%ds", goalID, revision, lockHolder(dir), elapsed, maximum)
		s.recordOutcome("LOCK_BUSY", "refused", detail, s.outcomeJob())
		return s.die(1, lockBusyMessage(fmt.Sprintf("goal %s revision %d", goalID, revision), holderWords(dir), elapsed))
	}
	s.goalLockHeld = true
	return nil
}

// releaseGoalRevisionLock is release_goal_revision_lock.
func (s *session) releaseGoalRevisionLock() error {
	if !s.goalLockHeld {
		return nil
	}
	if s.ownerLockRelease(s.goalLockDir) == 4 {
		return s.die(1, "refusing to release another owner's goal-revision lock")
	}
	s.goalLockHeld = false
	s.goalLockDir = ""
	return nil
}

func (s *session) lifecycleLockDir(job string) string {
	return filepath.Join(s.recordLocks, job+".lifecycle.d")
}

// acquireLifecycleLockUntil is acquire_lifecycle_lock_until; false with the
// busy-lock message on stderr when the holder outlasts the scaled cap.
func (s *session) acquireLifecycleLockUntil(job string, base int64) (bool, error) {
	if err := os.MkdirAll(s.recordLocks, 0o755); err != nil {
		s.eprintln(err.Error())
		return false, nil
	}
	dir := s.lifecycleLockDir(job)
	status, elapsed, _, err := s.waitForLock(dir, base)
	if err != nil {
		return false, err
	}
	if status == 0 {
		return true, nil
	}
	if status == 3 {
		s.eprintln(lockBusyMessage("job "+job, holderWords(dir), elapsed))
	}
	return false, nil
}

func (s *session) releaseLifecycleLock(job string) {
	s.ownerLockRelease(s.lifecycleLockDir(job))
}

func (s *session) releaseExitLifecycle() {
	if s.cleanupLifecycle == "" {
		return
	}
	s.releaseLifecycleLock(s.cleanupLifecycle)
	s.cleanupLifecycle = ""
}

func (s *session) capAuthorityLockDir() string {
	return filepath.Join(s.agents, "supervision", "cap-authority.lock.d")
}

// acquireCapAuthorityLock is acquire_cap_authority_lock.
func (s *session) acquireCapAuthorityLock() error {
	dir := s.capAuthorityLockDir()
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return s.die(1, err.Error())
	}
	status, elapsed, maximum, err := s.waitForLock(dir, 10)
	if err != nil {
		return err
	}
	if status != 0 {
		detail := fmt.Sprintf("rank=cap-authority key=repository holder=%s retry=retry-after-the-named-holder-releases elapsed=%ds cap=%ds", lockHolder(dir), elapsed, maximum)
		s.recordOutcome("LOCK_BUSY", "refused", detail, s.outcomeJob())
		return s.die(1, lockBusyMessage("the repository's cap authority", holderWords(dir), elapsed))
	}
	s.capAuthorityHeld = true
	return nil
}

// releaseCapAuthorityLock is release_cap_authority_lock.
func (s *session) releaseCapAuthorityLock() error {
	if !s.capAuthorityHeld {
		return nil
	}
	if s.ownerLockRelease(s.capAuthorityLockDir()) == 4 {
		return s.die(1, "refusing to release another owner's cap-authority lock")
	}
	s.capAuthorityHeld = false
	return nil
}

// --- checkout execution guard -----------------------------------------

// guardAcquire is checkout_execution_guard_acquire: wait for exclusive
// checkout execution (or join an owning chain) bounded by watch.cap-min.
func (s *session) guardAcquire(owner string) error {
	waitMin, err := s.configGet("watch.cap-min", config.MustDefault("watch.cap-min"))
	if err != nil {
		return err
	}
	progressSec, err := s.configGet("watch.interval-sec", config.MustDefault("watch.interval-sec"))
	if err != nil {
		return err
	}
	if !positiveInteger(waitMin) {
		s.eprintln("checkout execution guard: watch.cap-min must be a positive integer")
		return exitWith(1)
	}
	if !positiveInteger(progressSec) {
		s.eprintln("checkout execution guard: watch.interval-sec must be a positive integer")
		return exitWith(1)
	}
	if s.env.GuardFixture != "" {
		attempted, ok := field(s.env.GuardFixture, "attempted")
		if !ok || !filepath.IsAbs(attempted) {
			return exitWith(2)
		}
		if err := touch(attempted); err != nil {
			return exitWith(2)
		}
	}
	minutes, _ := strconv.ParseInt(waitMin, 10, 64)
	seconds, _ := strconv.ParseInt(progressSec, 10, 64)
	result, err := s.l.ports.Guard.Acquire(s.guardRoot, s.inv.CallerPid, owner,
		time.Duration(minutes)*time.Minute, time.Duration(seconds)*time.Second, s.stderr)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	switch result {
	case gaterun.GuardAcquired, gaterun.GuardJoined:
		s.guardHeld = true
	}
	return nil
}

// guardRelease is checkout_execution_guard_release.
func (s *session) guardRelease() error {
	if !s.guardHeld {
		return nil
	}
	s.guardHeld = false
	if err := s.l.ports.Guard.Release(s.guardRoot, s.inv.CallerPid); err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return nil
}

// censusFresh is `job census-fresh --root`: freshness and the fingerprint
// match are the engine's one verdict.
func (s *session) censusFresh(verdict, state, arm string) error {
	fingerprint, err := census.Fingerprint(s.root, s.repoScope)
	if err != nil {
		s.eprintf("dispatch refused: census fingerprint cannot be computed: %v\n", err)
		return exitWith(1)
	}
	now, err := s.goalNow()
	if err != nil {
		s.eprintf("dispatch refused: census clock cannot be resolved: %v\n", err)
		return exitWith(1)
	}
	if err := dispatch.CensusFresh(verdict, state, arm, s.repoScope, fingerprint, now); err != nil {
		s.eprintln(err.Error())
		var window dispatch.ArmingWindowError
		if errors.As(err, &window) {
			return exitWith(9)
		}
		return exitWith(1)
	}
	return nil
}

// closeStopFenceClaim is stopfence creating-close: an absent claim is closed.
func closeStopFenceClaim(claim string) error {
	if err := stopfence.CloseClaim(claim); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
