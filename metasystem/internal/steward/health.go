package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// HealthStatus is the complete role vocabulary. Unknown is evidence that
// could not support either a live or dead judgment; it never becomes alive by
// default.
type HealthStatus string

const (
	HealthAlive   HealthStatus = "alive"
	HealthDead    HealthStatus = "dead"
	HealthUnknown HealthStatus = "unknown"
)

// HealthRole names the stable checks printed on every health line.
type HealthRole string

const (
	RoleStewardRunner     HealthRole = "steward-runner"
	RoleSupervisionOwner  HealthRole = "supervision-owner"
	RoleRepoWatcher       HealthRole = "repo-watcher"
	RoleCensusFreshness   HealthRole = "census-freshness"
	RoleNarratorFreshness HealthRole = "narrator-freshness"
	RoleSessionMain       HealthRole = "session-main"
	RoleHookFreshness     HealthRole = "hook-freshness"
	// Keep the published role name stable for existing health consumers.
	RoleClaimedGoalBudget   HealthRole = "claimed-goal-appetite"
	RoleNonterminalJobs     HealthRole = "nonterminal-jobs"
	RoleCapabilitySnapshots HealthRole = "capability-snapshots"
)

var healthRoleOrder = []HealthRole{
	RoleStewardRunner,
	RoleSupervisionOwner,
	RoleRepoWatcher,
	RoleCensusFreshness,
	RoleNarratorFreshness,
	RoleSessionMain,
	RoleHookFreshness,
	RoleClaimedGoalBudget,
	RoleNonterminalJobs,
	RoleCapabilitySnapshots,
}

// RoleVerdict is one total role judgment and the exact command that repairs a
// non-alive result with the surface available today.
type RoleVerdict struct {
	Role                HealthRole   `json:"role"`
	Status              HealthStatus `json:"status"`
	Reason              string       `json:"reason"`
	Remedy              string       `json:"remedy,omitempty"`
	ConsecutiveUnknown  int          `json:"consecutiveUnknown,omitempty"`
	ConsecutiveFailures int          `json:"consecutiveFailures,omitempty"`
	FailureEscalation   string       `json:"failureEscalation,omitempty"`
	NoAutomaticRemedy   bool         `json:"noAutomaticRemedy,omitempty"`
}

const (
	AutoHealEligible = "AUTO_HEAL_ELIGIBLE"
	AutoHealEnded    = "AUTO_HEAL_ENDED"
	NoLawfulRemedy   = "NO_LAWFUL_REMEDY"
	HealingFlapping  = "HEALING_FLAPPING"

	healthFailureLimit = 5
	healthFlapLimit    = 3
	healthFlapWindow   = time.Hour
)

// HealthObservationState is the durable observation clock, the one-pass
// grace for each unknown role, and the failure episodes that survive healthy
// resets long enough to expose repeated heal-and-fail cycles.
type HealthObservationState struct {
	Sequence        int64                      `json:"sequence"`
	ObservedAt      time.Time                  `json:"observedAt"`
	UnknownCounts   map[HealthRole]int         `json:"unknownCounts"`
	FailureCounts   map[HealthRole]int         `json:"failureCounts"`
	FailureEpisodes map[HealthRole][]time.Time `json:"failureEpisodes,omitempty"`
}

// HealthVerdict is the typed result of one completed health observation.
type HealthVerdict struct {
	Schema        int                    `json:"schema"`
	ObservedAt    time.Time              `json:"observedAt"`
	Observation   int64                  `json:"observation"`
	Aggregate     string                 `json:"aggregate"`
	Roles         []RoleVerdict          `json:"roles"`
	ShouldAlert   bool                   `json:"shouldAlert"`
	FindingDigest string                 `json:"findingDigest"`
	State         HealthObservationState `json:"-"`
}

type healthRecord struct {
	State   HealthObservationState `json:"state"`
	Verdict HealthVerdict          `json:"verdict"`
}

// HealthRecordPath is the single durable health observation record.
func HealthRecordPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "health.json")
}

func healthLockPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "health.flock")
}

// ExitCode applies the aggregate boundary: dead outranks unknown.
func (v HealthVerdict) ExitCode() int {
	switch v.Aggregate {
	case "healthy":
		return 0
	case "unhealthy":
		return 1
	default:
		return 2
	}
}

// Line is the one-line operator view. Every role remains on the line so an
// all-clear is self-evident rather than implied by silence.
func (v HealthVerdict) Line() string {
	items := make([]string, 0, len(v.Roles))
	for _, role := range v.Roles {
		item := fmt.Sprintf("%s=%s", role.Role, role.Status)
		if role.Reason != "" {
			item += " (" + role.Reason
			if role.Remedy != "" {
				item += "; remedy: " + role.Remedy
			}
			item += ")"
		}
		if role.ConsecutiveFailures > 0 {
			item += fmt.Sprintf(" [failure %d/%d; %s]", role.ConsecutiveFailures, healthFailureLimit, strings.ToLower(strings.ReplaceAll(role.FailureEscalation, "_", " ")))
		}
		items = append(items, item)
	}
	return "HEALTH " + v.Aggregate + " — " + strings.Join(items, "; ")
}

// ObserveHealth evaluates every role and durably advances exactly one
// observation. The state and its rendered verdict share one atomic record so
// a restart cannot observe a counter without the verdict that advanced it.
func ObserveHealth(repoRoot string, now time.Time, prober identity.Prober) (HealthVerdict, error) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	if err := os.MkdirAll(filepath.Dir(healthLockPath(repoRoot)), 0o755); err != nil {
		return HealthVerdict{}, err
	}
	lockFile, err := os.OpenFile(healthLockPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return HealthVerdict{}, err
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return HealthVerdict{}, err
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

	previous, err := loadHealthRecord(HealthRecordPath(repoRoot))
	stateUnreadable := err != nil && !os.IsNotExist(err)
	if stateUnreadable {
		previous = healthRecord{}
	}
	roles := evaluateHealthRoles(repoRoot, now.UTC(), prober, false)
	if stateUnreadable {
		remedy := fmt.Sprintf("metasystem health --repo %q", repoRoot)
		for index := range roles {
			if roles[index].Status == HealthAlive {
				roles[index] = roleUnknown(roles[index].Role, "the prior health observation state was unreadable", remedy)
			}
		}
	}
	verdict := applyHealthObservation(repoRoot, previous.State, roles, now.UTC())
	if err := saveHealthRecord(repoRoot, HealthRecordPath(repoRoot), healthRecord{State: verdict.State, Verdict: verdict}); err != nil {
		return HealthVerdict{}, err
	}
	return verdict, nil
}

// PreviewHealth renders the hook's current-turn facts without advancing the
// periodic observation breaker. The hook records its own attempt and
// completion; only the steward tick owns durable alert escalation.
func PreviewHealth(repoRoot string, now time.Time, prober identity.Prober) HealthVerdict {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	roles := evaluateHealthRoles(repoRoot, now.UTC(), prober, true)
	aggregate := "healthy"
	for _, role := range roles {
		if role.Status == HealthDead {
			aggregate = "unhealthy"
			break
		}
		if role.Status == HealthUnknown {
			aggregate = "unknown"
		}
	}
	return HealthVerdict{
		Schema: 1, ObservedAt: now.UTC(), Aggregate: aggregate,
		Roles: roles, FindingDigest: healthFindingDigest(roles),
	}
}

func evaluateHealthRoles(repoRoot string, now time.Time, prober identity.Prober, currentHookAttempt bool) []RoleVerdict {
	state, stateErr := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "state.json"))
	return []RoleVerdict{
		checkStewardRunner(repoRoot, now, prober),
		checkSupervisionOwner(repoRoot, prober),
		checkRepoWatcher(repoRoot, now, state, stateErr, prober),
		checkCensusFreshness(repoRoot, now, state, stateErr),
		checkNarratorFreshness(repoRoot, now),
		checkSessionMain(repoRoot, prober),
		checkHookFreshnessAt(repoRoot, now, currentHookAttempt),
		checkClaimedGoalBudgets(repoRoot, now),
		checkNonterminalJobs(repoRoot, prober),
		checkCapabilitySnapshots(repoRoot, now),
	}
}

func checkHookFreshness(repoRoot string, now time.Time) RoleVerdict {
	return checkHookFreshnessAt(repoRoot, now, false)
}

func checkHookFreshnessAt(repoRoot string, now time.Time, currentAttempt bool) RoleVerdict {
	remedy := fmt.Sprintf("metasystem health --repo %q", repoRoot)
	record, durabilityPending, err := loadComponentEvidenceForHealth(repoRoot, "supervision-hook")
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleHookFreshness, "no hook turn generation is recorded", remedy)
		}
		return roleUnknown(RoleHookFreshness, "the hook completion evidence is unreadable", remedy)
	}
	if record.Generation < 1 || record.TurnKeyDigest == "" {
		return roleUnknown(RoleHookFreshness, "the hook turn generation is incomplete", remedy)
	}
	if record.LastAttempt.After(now) || record.LastCompletion.After(now) || record.LastSuccess.After(now) {
		return roleUnknown(RoleHookFreshness, "CLOCK_REGRESSED: hook evidence is later than current UTC", remedy)
	}
	if durabilityPending || record.Outcome == "DURABILITY_PENDING" {
		return roleUnknown(RoleHookFreshness, "the hook completion is waiting for durability proof", remedy)
	}
	if currentAttempt && (record.Outcome == "ATTEMPTING" || record.LastCompletion.Before(record.LastAttempt)) {
		if len(record.AttemptHistory) == 0 {
			return roleUnknown(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending with no prior completed turn", record.Generation), remedy)
		}
		prior := record.AttemptHistory[len(record.AttemptHistory)-1]
		if prior.Result == ComponentOK && prior.Outcome == "EMITTED" {
			return roleAlive(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending; prior generation %d completed as OK/EMITTED", record.Generation, prior.Generation))
		}
		return roleDead(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending after prior generation %d ended as %s/%s", record.Generation, prior.Generation, prior.Result, prior.Outcome), remedy)
	}
	if record.Outcome == "ATTEMPTING" || record.LastCompletion.Before(record.LastAttempt) {
		return roleDead(RoleHookFreshness, fmt.Sprintf("turn generation %d has an attempt without completion", record.Generation), remedy)
	}
	if record.Result != ComponentOK || record.Outcome != "EMITTED" ||
		record.SuccessAttemptSeq != record.AttemptSeq || !record.LastSuccess.Equal(record.LastCompletion) {
		return roleDead(RoleHookFreshness, fmt.Sprintf("turn generation %d did not complete as OK/EMITTED", record.Generation), remedy)
	}
	return roleAlive(RoleHookFreshness, fmt.Sprintf("turn generation %d completed as OK/EMITTED", record.Generation))
}

func applyHealthObservation(repoRoot string, previous HealthObservationState, roles []RoleVerdict, now time.Time) HealthVerdict {
	unknownCounts := make(map[HealthRole]int, len(healthRoleOrder))
	for key, value := range previous.UnknownCounts {
		unknownCounts[key] = value
	}
	failureCounts := make(map[HealthRole]int, len(healthRoleOrder))
	for key, value := range previous.FailureCounts {
		failureCounts[key] = value
	}
	failureEpisodes := make(map[HealthRole][]time.Time, len(healthRoleOrder))
	for key, values := range previous.FailureEpisodes {
		failureEpisodes[key] = append([]time.Time(nil), values...)
	}
	ordered := append([]RoleVerdict(nil), roles...)
	order := make(map[HealthRole]int, len(healthRoleOrder))
	for index, role := range healthRoleOrder {
		order[role] = index
	}
	sort.SliceStable(ordered, func(i, j int) bool { return order[ordered[i].Role] < order[ordered[j].Role] })
	observedAt := now.UTC()
	if previous.ObservedAt.After(observedAt) {
		observedAt = previous.ObservedAt.UTC()
		remedy := fmt.Sprintf("metasystem health --repo %q", repoRoot)
		for index := range ordered {
			if ordered[index].Status == HealthAlive {
				ordered[index].Status = HealthUnknown
				ordered[index].Reason = "CLOCK_REGRESSED: the prior health observation is later than current UTC"
				ordered[index].Remedy = remedy
			}
		}
	}
	aggregate := "healthy"
	alert := false
	for index := range ordered {
		role := &ordered[index]
		roleEpisodes := failureEpisodes[role.Role][:0]
		for _, openedAt := range failureEpisodes[role.Role] {
			if !openedAt.Before(observedAt.Add(-healthFlapWindow)) {
				roleEpisodes = append(roleEpisodes, openedAt)
			}
		}
		failureEpisodes[role.Role] = roleEpisodes
		if role.Status == HealthUnknown {
			unknownCounts[role.Role]++
			role.ConsecutiveUnknown = unknownCounts[role.Role]
			if unknownCounts[role.Role] >= 2 {
				alert = true
			}
			if aggregate == "healthy" {
				aggregate = "unknown"
			}
			continue
		}
		unknownCounts[role.Role] = 0
		if role.Status == HealthDead {
			aggregate = "unhealthy"
			if failureCounts[role.Role] == 0 {
				failureEpisodes[role.Role] = append(failureEpisodes[role.Role], observedAt)
			}
			failureCounts[role.Role]++
			if failureCounts[role.Role] >= healthFailureLimit {
				failureCounts[role.Role] = healthFailureLimit
			}
			switch {
			case !hasLawfulAutomaticRemedy(*role, ordered):
				role.FailureEscalation = NoLawfulRemedy
				alert = true
			case failureCounts[role.Role] >= healthFailureLimit:
				role.FailureEscalation = AutoHealEnded
				alert = true
			case len(failureEpisodes[role.Role]) >= healthFlapLimit:
				role.FailureEscalation = HealingFlapping
				alert = true
			default:
				role.FailureEscalation = AutoHealEligible
			}
			role.ConsecutiveFailures = failureCounts[role.Role]
			continue
		}
		failureCounts[role.Role] = 0
	}
	state := HealthObservationState{
		Sequence: previous.Sequence + 1, ObservedAt: observedAt,
		UnknownCounts: unknownCounts, FailureCounts: failureCounts, FailureEpisodes: failureEpisodes,
	}
	verdict := HealthVerdict{
		Schema: 1, ObservedAt: observedAt, Observation: state.Sequence,
		Aggregate: aggregate, Roles: ordered, ShouldAlert: alert, State: state,
	}
	verdict.FindingDigest = healthFindingDigest(ordered)
	return verdict
}

func hasLawfulAutomaticRemedy(role RoleVerdict, roles []RoleVerdict) bool {
	roleIsDead := func(want HealthRole) bool {
		for _, candidate := range roles {
			if candidate.Role == want {
				return candidate.Status == HealthDead
			}
		}
		return false
	}
	switch role.Role {
	case RoleStewardRunner:
		return true
	case RoleRepoWatcher:
		return strings.Contains(role.Reason, "recorded pid") ||
			strings.Contains(role.Reason, "lastSuccess is stale") ||
			strings.Contains(role.Reason, "latest attempt passed its deadline")
	case RoleCensusFreshness:
		// A failed census prevents the watcher pass from completing, so the
		// watcher's owner replaces the producer that owns this evidence.
		return roleIsDead(RoleRepoWatcher)
	case RoleNarratorFreshness:
		// The watcher repairs the steward process whose failed tick also stops
		// narration. An isolated narrator failure has no separate automatic act.
		return roleIsDead(RoleStewardRunner)
	case RoleClaimedGoalBudget:
		return !role.NoAutomaticRemedy
	default:
		return false
	}
}

func checkStewardRunner(repoRoot string, now time.Time, prober identity.Prober) RoleVerdict {
	remedy := fmt.Sprintf("metasystem steward restart --repo %q", repoRoot)
	path := runnerRecordPath(repoRoot)
	var runner RunnerRecord
	if err := readJSON(path, &runner); err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleStewardRunner, "no steward runner is recorded", remedy)
		}
		return roleUnknown(RoleStewardRunner, "the steward runner record is unreadable", remedy)
	}
	process := identity.Ref{Pid: runner.Pid, StartedAtSec: runner.PidStartedAt, StartTicks: runner.StartTicks, BootID: runner.BootID}
	switch identity.AliveRef(prober, process) {
	case identity.Dead:
		return roleDead(RoleStewardRunner, fmt.Sprintf("recorded runner pid %d is dead", runner.Pid), remedy)
	case identity.Unknown:
		return roleUnknown(RoleStewardRunner, fmt.Sprintf("recorded runner pid %d cannot be inspected", runner.Pid), remedy)
	}
	generation, err := installedGeneration(repoRoot)
	if err != nil {
		return roleUnknown(RoleStewardRunner, "the steward installation generation is unreadable", remedy)
	}
	return componentFreshness(repoRoot, "steward-tick", RoleStewardRunner, generation, time.Duration(2*TickSeconds(repoRoot))*time.Second, now, remedy, &process,
		fmt.Sprintf("runner pid %d and generation %d success are current", runner.Pid, generation))
}

func checkSupervisionOwner(repoRoot string, prober identity.Prober) RoleVerdict {
	remedy := supervisionRemedy(repoRoot)
	owner, err := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "lock.d", "owner.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleSupervisionOwner, "no supervision owner holds the repository lock", remedy)
		}
		return roleUnknown(RoleSupervisionOwner, "the supervision owner lock is unreadable", remedy)
	}
	ref, ok := processRef(owner)
	if !ok {
		return roleUnknown(RoleSupervisionOwner, "the lock owner's process identity is incomplete", remedy)
	}
	switch identity.AliveRef(prober, ref) {
	case identity.Alive:
		return roleAlive(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d is alive", ref.Pid))
	case identity.Dead:
		return roleDead(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d is dead", ref.Pid), remedy)
	default:
		return roleUnknown(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d cannot be inspected", ref.Pid), remedy)
	}
}

func checkRepoWatcher(repoRoot string, now time.Time, state map[string]any, stateErr error, prober identity.Prober) RoleVerdict {
	remedy := supervisionRemedy(repoRoot)
	if stateErr != nil {
		if os.IsNotExist(stateErr) {
			return roleDead(RoleRepoWatcher, "no supervision state is recorded", remedy)
		}
		return roleUnknown(RoleRepoWatcher, "the supervision state is unreadable", remedy)
	}
	var entry map[string]any
	if components, ok := state["components"].(map[string]any); ok {
		entry, _ = components["watcher"].(map[string]any)
	}
	if entry == nil {
		return roleDead(RoleRepoWatcher, "the role has no recorded process", remedy)
	}
	ref, ok := processRef(entry)
	if !ok {
		return roleUnknown(RoleRepoWatcher, "the recorded process identity is incomplete", remedy)
	}
	switch identity.AliveRef(prober, ref) {
	case identity.Dead:
		return roleDead(RoleRepoWatcher, fmt.Sprintf("recorded pid %d is dead", ref.Pid), remedy)
	case identity.Unknown:
		return roleUnknown(RoleRepoWatcher, fmt.Sprintf("recorded pid %d cannot be inspected", ref.Pid), remedy)
	}
	generation, generationOK := healthInt(state["generation"])
	interval, intervalOK := healthInt(state["intervalSec"])
	if !generationOK || generation < 1 || !intervalOK || interval < 1 {
		return roleUnknown(RoleRepoWatcher, "the watcher generation or producer interval is unreadable", remedy)
	}
	return componentFreshness(repoRoot, "repo-watcher", RoleRepoWatcher, int(generation), time.Duration(2*interval)*time.Second,
		now, remedy, &ref, fmt.Sprintf("watcher pid %d and generation %d success are current", ref.Pid, generation))
}

func checkCensusFreshness(repoRoot string, now time.Time, state map[string]any, stateErr error) RoleVerdict {
	remedy := supervisionRemedy(repoRoot)
	census, err := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "last-census.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleCensusFreshness, "no census success is recorded", remedy)
		}
		return roleUnknown(RoleCensusFreshness, "the census evidence is unreadable", remedy)
	}
	if result, _ := census["verdict"].(string); result != "SUCCESS" {
		return roleDead(RoleCensusFreshness, "the latest census did not succeed", remedy)
	}
	if stateErr != nil {
		return roleUnknown(RoleCensusFreshness, "the census generation cannot be compared with supervision", remedy)
	}
	wantGeneration, wantOK := healthInt(state["generation"])
	gotGeneration, gotOK := healthInt(census["generation"])
	if !wantOK || !gotOK || wantGeneration < 1 || gotGeneration < 1 {
		return roleUnknown(RoleCensusFreshness, "the census generation evidence is incomplete", remedy)
	}
	if wantGeneration != gotGeneration {
		return roleDead(RoleCensusFreshness, fmt.Sprintf("census generation %d does not match supervision generation %d", gotGeneration, wantGeneration), remedy)
	}
	lastSuccess, ok := evidenceSuccessTime(census)
	if !ok {
		return roleUnknown(RoleCensusFreshness, "the census has no readable lastSuccess", remedy)
	}
	age := now.Sub(lastSuccess)
	if age < 0 {
		return roleUnknown(RoleCensusFreshness, "CLOCK_REGRESSED: census lastSuccess is later than current UTC", remedy)
	}
	intervalSeconds, ok := healthInt(census["intervalSec"])
	if !ok || intervalSeconds < 1 {
		return roleUnknown(RoleCensusFreshness, "the census producer interval is unreadable", remedy)
	}
	window := time.Duration(2*intervalSeconds) * time.Second
	if age >= window {
		return roleDead(RoleCensusFreshness, fmt.Sprintf("census lastSuccess is stale at %s", age.Round(time.Second)), remedy)
	}
	return roleAlive(RoleCensusFreshness, fmt.Sprintf("census generation %d succeeded %s ago", gotGeneration, age.Round(time.Second)))
}

func checkNarratorFreshness(repoRoot string, now time.Time) RoleVerdict {
	remedy := fmt.Sprintf("metasystem steward restart --repo %q", repoRoot)
	generation, err := installedGeneration(repoRoot)
	if err != nil {
		return roleUnknown(RoleNarratorFreshness, "the steward installation generation is unreadable", remedy)
	}
	return componentFreshness(repoRoot, "narrator", RoleNarratorFreshness, generation, time.Duration(2*TickSeconds(repoRoot))*time.Second, now, remedy, nil,
		fmt.Sprintf("narrator generation %d success is current", generation))
}

func componentFreshness(repoRoot, component string, role HealthRole, generation int, window time.Duration, now time.Time, remedy string, expectedSuccess *identity.Ref, aliveReason string) RoleVerdict {
	record, durabilityPending, err := loadComponentEvidenceForHealth(repoRoot, component)
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(role, "no successful component pass is recorded", remedy)
		}
		return roleUnknown(role, "the component success evidence is unreadable", remedy)
	}
	if record.Generation != generation {
		return roleDead(role, fmt.Sprintf("component generation %d does not match installation generation %d", record.Generation, generation), remedy)
	}
	if record.LastSuccess.IsZero() {
		return roleDead(role, "the current generation has no successful completion", remedy)
	}
	if durabilityPending || record.Outcome == "DURABILITY_PENDING" {
		return roleUnknown(role, "the latest completion is waiting for durability proof", remedy)
	}
	if expectedSuccess != nil {
		success := identity.Ref{Pid: record.SuccessPid, StartedAtSec: record.SuccessPidStartedAt, StartTicks: record.SuccessPidStartTicks, BootID: record.SuccessBootID}
		if !sameComponentProcess(success, *expectedSuccess) {
			return roleDead(role, fmt.Sprintf("lastSuccess belongs to pid %d, not resident runner pid %d", success.Pid, expectedSuccess.Pid), remedy)
		}
	}
	if record.LastSuccess.After(now) || record.LastCompletion.After(now) || record.LastAttempt.After(now) {
		return roleUnknown(role, "CLOCK_REGRESSED: component evidence is later than current UTC", remedy)
	}
	if record.Outcome == "ATTEMPTING" && now.Sub(record.LastAttempt) >= window {
		return roleDead(role, "the latest attempt passed its deadline without completion", remedy)
	}
	if now.Sub(record.LastSuccess) >= window {
		return roleDead(role, fmt.Sprintf("lastSuccess is stale at %s", now.Sub(record.LastSuccess).Round(time.Second)), remedy)
	}
	return roleAlive(role, aliveReason)
}

func checkSessionMain(repoRoot string, prober identity.Prober) RoleVerdict {
	remedy := supervisionRemedy(repoRoot)
	directory := filepath.Join(repoRoot, "artifacts", "agents", "mains")
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleSessionMain, "no session main is announced", remedy)
		}
		return roleUnknown(RoleSessionMain, "session announcements are unreadable", remedy)
	}
	unknown := false
	dead := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		value, readErr := readHealthObject(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			unknown = true
			continue
		}
		sessionID, _ := value["sessionId"].(string)
		if sessionID == "" {
			continue
		}
		mainID, _ := value["mainId"].(string)
		if mainID == "" {
			mainID = sessionID
		}
		ref, ok := processRef(value)
		if !ok {
			unknown = true
			continue
		}
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			return roleAlive(RoleSessionMain, fmt.Sprintf("announced main %s is alive", mainID))
		case identity.Dead:
			dead = true
		case identity.Unknown:
			unknown = true
		}
	}
	if unknown {
		return roleUnknown(RoleSessionMain, "no announced main has readable liveness", remedy)
	}
	if dead {
		return roleDead(RoleSessionMain, "every announced session main is dead", remedy)
	}
	return roleDead(RoleSessionMain, "no session main is announced", remedy)
}

func checkClaimedGoalBudgets(repoRoot string, now time.Time) RoleVerdict {
	if !goal.NewWorld(repoRoot) {
		return roleAlive(RoleClaimedGoalBudget, "the bootstrap ledger has no claimed-goal records")
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return roleUnknown(RoleClaimedGoalBudget, "the claimed-goal ledger endpoint is unreadable", "metasystem goal list --root "+strconv.Quote(repoRoot))
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		if unknown, ok := dispatch.GoalRecordBudgetUnknown(err); ok {
			return roleUnknown(RoleClaimedGoalBudget,
				fmt.Sprintf("BUDGET_UNKNOWN record=%s reason=%s", unknown.Record, unknown.Reason),
				"repair the exact BUDGET_UNKNOWN record, then run metasystem health --repo "+strconv.Quote(repoRoot))
		}
		if id, malformed := malformedBudgetGoal(err); malformed {
			role := roleDead(RoleClaimedGoalBudget,
				fmt.Sprintf("claimed goal %s has a malformed structured budget tuple", id), goalBudgetRemedy(id))
			role.NoAutomaticRemedy = true
			return role
		}
		return roleUnknown(RoleClaimedGoalBudget, "the claimed-goal ledger is unreadable", "metasystem goal list --root "+strconv.Quote(repoRoot))
	}
	type budgetFailure struct {
		reason    string
		remedy    string
		automatic bool
	}
	var dead []budgetFailure
	var unknown []string
	var known []string
	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed {
			continue
		}
		if file.Budget == nil {
			dead = append(dead, budgetFailure{
				reason: fmt.Sprintf("%s BUDGET_MISSING record=plans/goals/%s.md: claimed goal has no structured budget", id, id),
				remedy: goalBudgetRemedy(id),
			})
			continue
		}
		if file.StopFence != nil {
			batch, batchErr := goal.ReadStopBatch(repoRoot, file.StopFence.StopID)
			if batchErr != nil {
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_INDETERMINATE stop=%s reason=%s",
						id, file.Claimed.Revision, file.StopFence.StopID, batchErr),
					remedy: "inspect the named stop batch and exact job record; keep the launch fence closed",
				})
				continue
			}
			switch batch.State {
			case goal.StopBatchComplete:
				known = append(known, fmt.Sprintf("%s revision=%d BREACH_STOP_COMPLETE stop=%s terminalJobs=%d foreignJobs=%d",
					id, file.Claimed.Revision, batch.StopID, len(batch.Terminal), len(batch.Foreign)))
			case goal.StopBatchIndeterminate:
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_INDETERMINATE stop=%s reason=%s",
						id, file.Claimed.Revision, batch.StopID, batch.Failure),
					remedy: "inspect the named stop batch and exact job record; keep the launch fence closed",
				})
			default:
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_OPEN stop=%s pendingJobs=%d",
						id, file.Claimed.Revision, batch.StopID, len(batch.Pending)),
					remedy: "metasystem steward tick --repo " + strconv.Quote(repoRoot), automatic: true,
				})
			}
			continue
		}
		budget := dispatch.ProjectBudget(repoRoot, file, now)
		if budget.Status == dispatch.BudgetUnknown {
			unknown = append(unknown, fmt.Sprintf("%s BUDGET_UNKNOWN record=%s reason=%s",
				id, budget.Unknown.Record, budget.Unknown.Reason))
			continue
		}
		if len(budget.Breaches) > 0 {
			var fields []string
			for _, breach := range budget.Breaches {
				fields = append(fields, fmt.Sprintf("%s used=%s limit=%s", breach.Field, breach.Used, breach.Limit))
			}
			dead = append(dead, budgetFailure{
				reason: fmt.Sprintf("%s revision=%d BREACH %s", id, budget.GoalRevision, strings.Join(fields, ", ")),
				remedy: "metasystem steward tick --repo " + strconv.Quote(repoRoot), automatic: true,
			})
			continue
		}
		known = append(known, fmt.Sprintf("%s revision=%d attempts=%d/%d reservedJobMinutes=%d/%d activeJobs=%d/%d elapsed=%s/%s",
			id, budget.GoalRevision, budget.Attempts, budget.Limits.AttemptLimit,
			budget.ReservedJobMinutes, budget.Limits.ReservedJobMinutesLimit,
			budget.ActiveJobs, budget.Limits.ActiveJobLimit,
			budget.Elapsed.Round(time.Second), budget.Limits.ElapsedLimit))
	}
	if len(dead) > 0 {
		reasons := make([]string, 0, len(dead)+len(unknown))
		remedy := dead[0].remedy
		noAutomaticRemedy := false
		for _, failure := range dead {
			reasons = append(reasons, failure.reason)
			if !failure.automatic && !noAutomaticRemedy {
				remedy = failure.remedy
				noAutomaticRemedy = true
			}
		}
		if len(unknown) > 0 {
			reasons = append(reasons, unknown...)
			remedy = "repair the exact BUDGET_UNKNOWN record, then run metasystem health --repo " + strconv.Quote(repoRoot)
			noAutomaticRemedy = true
		}
		role := roleDead(RoleClaimedGoalBudget, strings.Join(reasons, "; "), remedy)
		role.NoAutomaticRemedy = noAutomaticRemedy
		return role
	}
	if len(unknown) > 0 {
		return roleUnknown(RoleClaimedGoalBudget, strings.Join(unknown, "; "),
			"repair the exact BUDGET_UNKNOWN record, then run metasystem health --repo "+strconv.Quote(repoRoot))
	}
	if len(known) == 0 {
		return roleAlive(RoleClaimedGoalBudget, "there are no claimed goals")
	}
	return roleAlive(RoleClaimedGoalBudget, strings.Join(known, "; "))
}

func malformedBudgetGoal(err error) (string, bool) {
	var parseErr *goal.TreeReadError
	if !errors.As(err, &parseErr) {
		return "", false
	}
	const prefix = "plans/goals/"
	for _, problem := range parseErr.Problems {
		text := string(problem)
		if !strings.Contains(text, ": Budget:") || !strings.HasPrefix(text, prefix) {
			continue
		}
		path := strings.SplitN(text, ":", 2)[0]
		relative := strings.TrimPrefix(path, prefix)
		if strings.HasPrefix(relative, "done/") || !strings.HasSuffix(relative, ".md") {
			continue
		}
		file, _ := goal.ParseFile(parseErr.Files[path])
		if file == nil || file.State != goal.StateClaimed {
			continue
		}
		return strings.TrimSuffix(relative, ".md"), true
	}
	return "", false
}

func goalBudgetRemedy(id string) string {
	return fmt.Sprintf("metasystem goal set-budget --root . --id %s --elapsed-limit DURATION --attempt-limit POSITIVE_INTEGER --reserved-job-minutes-limit POSITIVE_INTEGER --active-job-limit POSITIVE_INTEGER", id)
}

func checkNonterminalJobs(repoRoot string, prober identity.Prober) RoleVerdict {
	paths, _ := filepath.Glob(filepath.Join(repoRoot, "artifacts", "agents", "jobs", "*.json"))
	sort.Strings(paths)
	var dead []string
	var unknown []string
	for _, path := range paths {
		value, err := readHealthObject(path)
		if err != nil {
			unknown = append(unknown, strings.TrimSuffix(filepath.Base(path), ".json"))
			continue
		}
		jobID, _ := value["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		status, statusOK := value["status"].(string)
		if !statusOK || status == "" {
			unknown = append(unknown, jobID)
			continue
		}
		if dispatch.TerminalStatus(status) {
			continue
		}
		if status != "pending-setup" && status != "pending" && status != "running" {
			unknown = append(unknown, jobID)
			continue
		}
		if value["pid"] == nil {
			continue
		}
		ref, ok := processRef(value)
		if !ok {
			unknown = append(unknown, jobID)
			continue
		}
		switch identity.AliveRef(prober, ref) {
		case identity.Dead:
			dead = append(dead, jobID)
		case identity.Unknown:
			unknown = append(unknown, jobID)
		}
	}
	remedy := fmt.Sprintf("%q reap", filepath.Join(repoRoot, "scripts", "agents", "dispatch.sh"))
	if len(dead) > 0 {
		return roleDead(RoleNonterminalJobs, "non-terminal jobs with dead recorded processes: "+strings.Join(dead, ","), remedy)
	}
	if len(unknown) > 0 {
		return roleUnknown(RoleNonterminalJobs, "non-terminal jobs with unreadable process evidence: "+strings.Join(unknown, ","), remedy)
	}
	return roleAlive(RoleNonterminalJobs, "no non-terminal job has a provably dead recorded process")
}

func checkCapabilitySnapshots(repoRoot string, now time.Time) RoleVerdict {
	runtimeValue, _, err := config.Get(config.GetParams{
		Key: "metasystem.runtimes", ConfPath: filepath.Join(repoRoot, "metasystem.conf"),
	})
	if err != nil {
		return roleUnknown(RoleCapabilitySnapshots, "metasystem.runtimes is unreadable", "metasystem config validate --conf "+strconv.Quote(filepath.Join(repoRoot, "metasystem.conf")))
	}
	if runtimeValue == "none" {
		return roleAlive(RoleCapabilitySnapshots, "no runtime capability snapshots are configured")
	}
	maxAgeDays, err := nonnegativeConfig(repoRoot, "capability.snapshot-max-age-days", 30)
	if err != nil {
		return roleUnknown(RoleCapabilitySnapshots, "capability.snapshot-max-age-days is unreadable", "metasystem config validate --conf "+strconv.Quote(filepath.Join(repoRoot, "metasystem.conf")))
	}
	runtimes := strings.Split(runtimeValue, ",")
	paths, _ := filepath.Glob(filepath.Join(repoRoot, "artifacts", "agents", "capabilities", "*.json"))
	var dead []string
	var unknown []string
	for _, runtimeName := range runtimes {
		runtimeName = strings.TrimSpace(runtimeName)
		if runtimeName == "" {
			unknown = append(unknown, "empty-runtime")
			continue
		}
		declaration, supported := runtimereg.Lookup(runtimeName)
		if !supported || !declaration.HasAdapter {
			unknown = append(unknown, runtimeName+":NO_ADAPTER")
			continue
		}
		var newest time.Time
		malformed := false
		for _, path := range paths {
			if !strings.HasPrefix(filepath.Base(path), runtimeName+"-") {
				continue
			}
			value, readErr := readHealthObject(path)
			if readErr != nil {
				malformed = true
				continue
			}
			if recorded, _ := value["runtime"].(string); recorded != runtimeName {
				malformed = true
				continue
			}
			capturedRaw, _ := value["capturedAt"].(string)
			captured, parseErr := time.Parse(time.RFC3339Nano, capturedRaw)
			if parseErr != nil {
				malformed = true
				continue
			}
			if captured.After(newest) {
				newest = captured
			}
		}
		if newest.IsZero() {
			if malformed {
				unknown = append(unknown, runtimeName)
			} else {
				dead = append(dead, runtimeName)
			}
			continue
		}
		age := now.Sub(newest)
		if age < 0 {
			unknown = append(unknown, runtimeName+":CLOCK_REGRESSED")
			continue
		}
		if age > time.Duration(maxAgeDays)*24*time.Hour {
			dead = append(dead, runtimeName)
		}
	}
	remedyFor := func(names []string) string {
		commands := make([]string, 0, len(names))
		for _, name := range names {
			name = strings.TrimSuffix(name, ":CLOCK_REGRESSED")
			if name == "empty-runtime" || strings.HasSuffix(name, ":NO_ADAPTER") {
				commands = append(commands, "metasystem config validate --conf "+strconv.Quote(filepath.Join(repoRoot, "metasystem.conf")))
				continue
			}
			commands = append(commands, fmt.Sprintf("%q probe", filepath.Join(repoRoot, "scripts", "agents", "adapters", name+".sh")))
		}
		return strings.Join(commands, " && ")
	}
	if len(dead) > 0 {
		return roleDead(RoleCapabilitySnapshots, "missing or stale capability snapshots: "+strings.Join(dead, ","), remedyFor(dead))
	}
	if len(unknown) > 0 {
		return roleUnknown(RoleCapabilitySnapshots, "capability snapshot ages are unreadable: "+strings.Join(unknown, ","), remedyFor(unknown))
	}
	return roleAlive(RoleCapabilitySnapshots, "the newest configured runtime snapshots are within their age limit")
}

func roleAlive(role HealthRole, reason string) RoleVerdict {
	return RoleVerdict{Role: role, Status: HealthAlive, Reason: reason}
}

func roleDead(role HealthRole, reason, remedy string) RoleVerdict {
	return RoleVerdict{Role: role, Status: HealthDead, Reason: reason, Remedy: remedy}
}

func roleUnknown(role HealthRole, reason, remedy string) RoleVerdict {
	return RoleVerdict{Role: role, Status: HealthUnknown, Reason: reason, Remedy: remedy}
}

func installedGeneration(repoRoot string) (int, error) {
	absolute, err := filepath.Abs(repoRoot)
	if err != nil {
		return 0, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	installed, err := VerifyIdentity(RepoIdentityPath(repoRoot), absolute)
	if err != nil {
		return 0, err
	}
	return installed.Generation, nil
}

func processRef(value map[string]any) (identity.Ref, bool) {
	pid, pidOK := healthInt(value["pid"])
	started, startedOK := healthInt(value["pidStartedAt"])
	if !pidOK || !startedOK || pid < 1 || started < 1 {
		return identity.Ref{}, false
	}
	ticks, _ := healthInt(value["pidStartTicks"])
	boot, _ := value["bootId"].(string)
	return identity.Ref{Pid: pid, StartedAtSec: started, StartTicks: ticks, BootID: boot}, true
}

func sameComponentProcess(left, right identity.Ref) bool {
	if left.Pid != right.Pid || left.Pid < 1 {
		return false
	}
	if left.StartTicks > 0 && left.BootID != "" && right.StartTicks > 0 && right.BootID != "" {
		return left.StartTicks == right.StartTicks && left.BootID == right.BootID
	}
	return left.StartedAtSec > 0 && left.StartedAtSec == right.StartedAtSec
}

func evidenceSuccessTime(value map[string]any) (time.Time, bool) {
	if raw, ok := value["lastSuccess"].(string); ok && raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		return parsed, err == nil
	}
	epoch, ok := healthInt(value["completedAtEpoch"])
	if !ok || epoch < 1 {
		return time.Time{}, false
	}
	return time.Unix(epoch, 0).UTC(), true
}

func nonnegativeConfig(repoRoot, key string, fallback int) (int, error) {
	return boundedConfig(repoRoot, key, fallback, 0)
}

func boundedConfig(repoRoot, key string, fallback, minimum int) (int, error) {
	value, _, err := config.Get(config.GetParams{
		Key: key, ConfPath: filepath.Join(repoRoot, "metasystem.conf"),
		Default: strconv.Itoa(fallback), DefaultSet: true,
	})
	if err != nil {
		return 0, err
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < minimum {
		return 0, fmt.Errorf("%s must be an integer of at least %d", key, minimum)
	}
	return number, nil
}

func supervisionRemedy(repoRoot string) string {
	return fmt.Sprintf("%q --repo %q", filepath.Join(repoRoot, "scripts", "agents", "arm-supervision.sh"), repoRoot)
}

func readHealthObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		if err == nil {
			err = fmt.Errorf("not a JSON object")
		}
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func healthInt(value any) (int64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Int64()
		return parsed, err == nil
	case float64:
		if number != math.Trunc(number) {
			return 0, false
		}
		return int64(number), true
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}

func healthFindingDigest(roles []RoleVerdict) string {
	var fields []string
	for _, role := range roles {
		if role.Status != HealthAlive {
			fields = append(fields, string(role.Role)+"="+string(role.Status))
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\n")))
	return hex.EncodeToString(sum[:])
}

func loadHealthRecord(path string) (healthRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return healthRecord{}, err
	}
	var record healthRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return healthRecord{}, fmt.Errorf("health observation record is malformed: %w", err)
	}
	if record.State.Sequence < 1 || record.State.ObservedAt.IsZero() || record.State.UnknownCounts == nil ||
		record.Verdict.Schema != 1 || record.Verdict.Observation != record.State.Sequence ||
		!record.Verdict.ObservedAt.Equal(record.State.ObservedAt) {
		return healthRecord{}, fmt.Errorf("health observation record is incomplete")
	}
	validRoles := make(map[HealthRole]bool, len(healthRoleOrder))
	for _, role := range healthRoleOrder {
		validRoles[role] = true
	}
	for role, count := range record.State.UnknownCounts {
		if !validRoles[role] || count < 0 {
			return healthRecord{}, fmt.Errorf("health observation record has an invalid unknown counter")
		}
	}
	if record.State.FailureCounts == nil {
		record.State.FailureCounts = make(map[HealthRole]int)
	}
	for role, count := range record.State.FailureCounts {
		if !validRoles[role] || count < 0 || count > healthFailureLimit {
			return healthRecord{}, fmt.Errorf("health observation record has an invalid failure counter")
		}
	}
	if record.State.FailureEpisodes == nil {
		record.State.FailureEpisodes = make(map[HealthRole][]time.Time)
	}
	for role, episodes := range record.State.FailureEpisodes {
		if !validRoles[role] {
			return healthRecord{}, fmt.Errorf("health observation record has an invalid failure episode role")
		}
		for _, openedAt := range episodes {
			if openedAt.IsZero() {
				return healthRecord{}, fmt.Errorf("health observation record has an invalid failure episode time")
			}
		}
	}
	return record, nil
}

// AutoHealingEnded reports the durable five-observation breaker for one role.
// A missing record means no observation has ended healing yet; unreadable
// state never authorizes a repair.
func AutoHealingEnded(repoRoot string, role HealthRole) (bool, error) {
	record, err := loadHealthRecord(HealthRecordPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return record.State.FailureCounts[role] >= healthFailureLimit, nil
}

func saveHealthRecord(repoRoot, path string, record healthRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("health observation was published with durability unknown")
	}
	return nil
}
