package steward

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
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
	RoleRetroDebt         HealthRole = "retro-debt"
	RoleSessionMain       HealthRole = "session-main"
	RoleHookFreshness     HealthRole = "hook-freshness"
	RoleStopHookDuration  HealthRole = "stop-hook-duration"
	RoleLedgerAttention   HealthRole = "ledger-attention"
	RoleSeatPresence      HealthRole = "seat-presence"
	// Keep the published role name stable for existing health consumers.
	RoleClaimedGoalBudget   HealthRole = "claimed-goal-appetite"
	RoleStopCapabilityEpoch HealthRole = "stop-capability-epoch"
	RoleClaimedGoalDelivery HealthRole = "claimed-goal-delivery"
	RoleTrunkRed            HealthRole = "trunk-red"
	RoleSpendFence          HealthRole = "spend-fence"
	RoleNonterminalJobs     HealthRole = "nonterminal-jobs"
	RoleCapabilitySnapshots HealthRole = "capability-snapshots"
	RoleGovernedObligations HealthRole = "governed-obligations"
	RoleProofAttempts       HealthRole = "proof-attempts"
	RoleProofAdmission      HealthRole = "proof-admission"
)

var healthRoleOrder = []HealthRole{
	RoleStewardRunner,
	RoleSupervisionOwner,
	RoleRepoWatcher,
	RoleCensusFreshness,
	RoleNarratorFreshness,
	RoleSessionMain,
	RoleHookFreshness,
	RoleStopHookDuration,
	RoleContext,
	RoleLedgerAttention,
	RoleSeatPresence,
	RoleClaimedGoalBudget,
	RoleStopCapabilityEpoch,
	RoleClaimedGoalDelivery,
	RoleTrunkRed,
	RoleSpendFence,
	RoleGovernedObligations,
	RoleNonterminalJobs,
	RoleProofAttempts,
	RoleProofAdmission,
	RoleCapabilitySnapshots,
	RoleDisk,
}

// KnownHealthRole reports whether role belongs to the closed health schema.
func KnownHealthRole(role HealthRole) bool {
	for _, known := range healthRoleOrder {
		if role == known {
			return true
		}
	}
	return false
}

// RoleVerdict is one total role judgment and the exact command that repairs a
// non-alive result with the surface available today.
type RoleVerdict struct {
	Role                HealthRole   `json:"role"`
	Status              HealthStatus `json:"status"`
	Reason              string       `json:"reason"`
	Cause               string       `json:"cause,omitempty"`
	Standing            bool         `json:"standing,omitempty"`
	Remedy              string       `json:"remedy,omitempty"`
	DurationMillis      int64        `json:"durationMillis,omitempty"`
	ConsecutiveUnknown  int          `json:"consecutiveUnknown,omitempty"`
	ConsecutiveFailures int          `json:"consecutiveFailures,omitempty"`
	FailureEscalation   string       `json:"failureEscalation,omitempty"`
	NoAutomaticRemedy   bool         `json:"noAutomaticRemedy,omitempty"`
	// RemedyFacts are the typed causes behind an unhealthy verdict, one per
	// affected goal, so a caller can name the public act without reading
	// the reason text.
	RemedyFacts []RemedyFact `json:"remedyFacts,omitempty"`
}

// RemedyFact is one typed cause of an unhealthy role.
type RemedyFact struct {
	Cause    RemedyCause `json:"cause"`
	Goal     string      `json:"goal,omitempty"`
	Record   string      `json:"record,omitempty"`
	Stop     string      `json:"stop,omitempty"`
	Job      string      `json:"job,omitempty"`
	Incident string      `json:"incident,omitempty"`
	Command  string      `json:"command,omitempty"`
}

// RemedyCause names one actionable cause a health role reports.
type RemedyCause string

const (
	CauseUnavailable           RemedyCause = "unavailable"
	CauseUnreadable            RemedyCause = "unreadable"
	CauseSettingsInvalid       RemedyCause = "settings-invalid"
	CauseClockRegressed        RemedyCause = "clock-regressed"
	CauseObservationPending    RemedyCause = "observation-pending"
	CauseStopFenceClosed       RemedyCause = "stop-fence-closed"
	CauseJobProcessDead        RemedyCause = "job-process-dead"
	CauseTrunkRedUnowned       RemedyCause = "trunk-red-unowned"
	CauseBudgetMissing         RemedyCause = "budget-missing"
	CauseBudgetMalformed       RemedyCause = "budget-malformed"
	CauseBudgetUnknown         RemedyCause = "budget-unknown"
	CauseBudgetBreach          RemedyCause = "budget-breach"
	CauseBreachStopOpen        RemedyCause = "breach-stop-open"
	CauseBreachStopUnresolved  RemedyCause = "breach-stop-indeterminate"
	CauseEpochMismatch         RemedyCause = "epoch-mismatch"
	CauseForeignLineage        RemedyCause = "foreign-lineage"
	CauseStopCapabilityMissing RemedyCause = "stop-capability-missing"
)

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
	FailureCauses   map[HealthRole]string      `json:"failureCauses,omitempty"`
	FailureEpisodes map[HealthRole][]time.Time `json:"failureEpisodes,omitempty"`
}

// HealthVerdict is the typed result of one completed health observation.
type HealthVerdict struct {
	Schema         int                    `json:"schema"`
	ObservedAt     time.Time              `json:"observedAt"`
	Observation    int64                  `json:"observation"`
	Aggregate      string                 `json:"aggregate"`
	Roles          []RoleVerdict          `json:"roles"`
	ShouldAlert    bool                   `json:"shouldAlert"`
	Stopped        bool                   `json:"stopped,omitempty"`
	StopPhase      string                 `json:"stopPhase,omitempty"`
	StopUnresolved int                    `json:"stopUnresolved,omitempty"`
	FindingDigest  string                 `json:"findingDigest"`
	State          HealthObservationState `json:"-"`
	Spend          SpendObservation       `json:"-"`
}

// HookHealthPreview is the versioned, read-only health projection consumed by
// Stop presentation. Intervention classifications are separate from the
// ordinary verdict so display metadata cannot change health policy.
type HookHealthPreview struct {
	SchemaVersion int                  `json:"schemaVersion"`
	ExitCode      int                  `json:"exitCode"`
	Line          string               `json:"line"`
	Interventions []HealthIntervention `json:"interventions"`
	Verdict       HealthVerdict        `json:"verdict"`
}

type HealthIntervention struct {
	Role              HealthRole `json:"role"`
	HumanRequired     bool       `json:"humanRequired"`
	SupervisionRepair bool       `json:"supervisionRepair"`
	Owner             string     `json:"owner"`
	Restriction       string     `json:"restriction"`
}

// NewHookHealthPreview derives both representations from one health
// evaluation. Each role is classified explicitly; a remedy, dead status, or
// no-automatic-remedy marker alone never turns into an intervention.
func NewHookHealthPreview(verdict HealthVerdict) HookHealthPreview {
	interventions := make([]HealthIntervention, 0, len(verdict.Roles))
	for _, role := range verdict.Roles {
		item := HealthIntervention{Role: role.Role}
		switch role.Role {
		case RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher,
			RoleCensusFreshness, RoleNarratorFreshness, RoleSessionMain,
			RoleHookFreshness, RoleStopHookDuration, RoleCapabilitySnapshots:
			// Unknown means the check could not establish either health or
			// failure. Only an established machinery failure requests repair.
			if role.Status == HealthDead {
				item.SupervisionRepair = true
				item.Owner = "steward"
			}
		}
		interventions = append(interventions, item)
	}
	return HookHealthPreview{
		SchemaVersion: 1, ExitCode: verdict.ExitCode(), Line: verdict.Line("agent"),
		Interventions: interventions, Verdict: verdict,
	}
}

// SpendCrossing is one independently alertable ceiling multiple.
type SpendCrossing struct {
	ScopeID  string  `json:"scopeId"`
	Scope    string  `json:"scope"`
	Ceiling  string  `json:"ceiling"`
	Multiple int     `json:"multiple"`
	Machine  string  `json:"machine"`
	Spend    float64 `json:"spend"`
	Limit    float64 `json:"limit"`
	Day      string  `json:"day"`
}

// SpendObservation distinguishes a valid empty crossing set from an unknown
// measurement, which must not clear existing spend episodes.
type SpendObservation struct {
	Valid     bool            `json:"valid"`
	Crossings []SpendCrossing `json:"crossings"`
}

type healthRecord struct {
	State   HealthObservationState `json:"state"`
	Verdict HealthVerdict          `json:"verdict"`
}

// HealthRecordPath is the single durable health observation record.
func HealthRecordPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "health.json")
}

// FreshHookHealthPreviewAt checks the installation's fence before reading the
// tick's verdict. Only the Stop's own roles are evaluated; the tick's record
// and observation clock stay unchanged. An unusable record requires a preview.
func FreshHookHealthPreviewAt(repoRoot, metasystemRoot string, now time.Time, git func(...string) (string, error)) (HookHealthPreview, bool) {
	if stopped, err := healthStopped(metasystemRoot, repoRoot, now.UTC(), nil, SpendObservation{}, HealthObservationState{}); err != nil {
		return HookHealthPreview{}, false
	} else if stopped != nil {
		return NewHookHealthPreview(*stopped), true
	}
	record, err := loadHealthRecord(HealthRecordPath(repoRoot))
	if err != nil || record.Verdict.ObservedAt.After(now) {
		return HookHealthPreview{}, false
	}
	age := now.Sub(record.Verdict.ObservedAt).Seconds()
	if age >= 2*float64(tickSecondsWithGit(repoRoot, git)) {
		return HookHealthPreview{}, false
	}
	switch record.Verdict.Aggregate {
	case "healthy", "unhealthy", "unknown":
		for index, role := range record.Verdict.Roles {
			switch role.Role {
			case RoleHookFreshness:
				record.Verdict.Roles[index] = checkHookFreshnessAt(repoRoot, now.UTC(), true)
			case RoleStopHookDuration:
				record.Verdict.Roles[index] = checkStopHookDuration(repoRoot)
			}
		}
		record.Verdict.Roles = standingRoles(record.State, record.Verdict.Roles)
		record.Verdict.Aggregate, record.Verdict.ShouldAlert = healthSummary(record.Verdict.Roles)
		record.Verdict.FindingDigest = healthFindingDigest(record.Verdict.Roles)
		return NewHookHealthPreview(record.Verdict), true
	default:
		return HookHealthPreview{}, false
	}
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
func (v HealthVerdict) Line(audience ...string) string { return v.line(true, audience...) }

// LineWithoutRemedies is Line for a surface that lists its public remedies
// on their own: each role with its reason, no owner remedy.
func (v HealthVerdict) LineWithoutRemedies() string { return v.line(false) }

func (v HealthVerdict) line(remedies bool, audience ...string) string {
	items := make([]string, 0, len(v.Roles))
	for _, role := range v.Roles {
		if !remedies {
			role.Remedy = ""
		} else if len(audience) > 0 && role.Status != HealthAlive {
			act, plain := role.PublicRemedy(audience[0], nil, v.Stopped)
			role.Remedy = strings.TrimSpace(strings.Join(act, " ") + " " + plain)
		}
		items = append(items, role.Line())
	}
	prefix := "HEALTH "
	if v.Stopped {
		record := stopfence.Record{State: stopfence.StateClosed, Phase: v.StopPhase}
		if v.StopUnresolved > 0 {
			record.NotStopped = make([]stopfence.Survivor, v.StopUnresolved)
		}
		var err error
		prefix, err = stopfence.HealthPrefix(record)
		if err != nil {
			prefix = "HEALTH STOP STATE INVALID "
		}
	}
	return prefix + v.Aggregate + " — " + strings.Join(items, "; ")
}

// Line is the shared one-role rendering used by health and focused status
// commands.
func (v RoleVerdict) Line() string {
	item := fmt.Sprintf("%s=%s", v.Role, v.Status)
	if v.Reason != "" {
		item += " (" + v.Reason
		if v.Remedy != "" {
			item += "; remedy: " + v.Remedy
		}
		item += ")"
	}
	if v.ConsecutiveFailures > 0 {
		item += fmt.Sprintf(" [failure %d/%d; %s]", v.ConsecutiveFailures, healthFailureLimit, strings.ToLower(strings.ReplaceAll(v.FailureEscalation, "_", " ")))
	}
	if v.Standing {
		item += " [standing defect]"
	}
	return item
}

// ObserveHealth evaluates every role and durably advances exactly one
// observation. The state and its rendered verdict share one atomic record so
// a restart cannot observe a counter without the verdict that advanced it.
func ObserveHealth(repoRoot string, now time.Time, prober identity.Prober) (HealthVerdict, error) {
	return observeHealthWithEvaluation(repoRoot, now, prober, evaluateHealthRoles)
}

type healthRoleEvaluator func(string, string, time.Time, identity.Prober, bool) ([]RoleVerdict, SpendObservation)

func observeHealthWithEvaluation(repoRoot string, now time.Time, prober identity.Prober, evaluate healthRoleEvaluator) (HealthVerdict, error) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	if err := os.MkdirAll(filepath.Dir(healthLockPath(repoRoot)), 0o755); err != nil {
		return HealthVerdict{}, err
	}
	lockFile, err := lock.File(healthLockPath(repoRoot), 0o644, lock.Exclusive)
	if err != nil {
		return HealthVerdict{}, err
	}
	defer lockFile.Release()

	previous, err := loadHealthRecord(HealthRecordPath(repoRoot))
	stateUnreadable := err != nil && !os.IsNotExist(err)
	if stateUnreadable {
		previous = healthRecord{}
	}
	roles, spendObservation := evaluate(repoRoot, repoRoot, now.UTC(), prober, false)
	if stopped, err := healthStopped(repoRoot, repoRoot, now.UTC(), roles, spendObservation, previous.State); err != nil {
		return HealthVerdict{}, err
	} else if stopped != nil {
		return *stopped, nil
	}
	if stateUnreadable {
		for index := range roles {
			if roles[index].Role != RoleSpendFence && roles[index].Status == HealthAlive {
				durationMillis := roles[index].DurationMillis
				roles[index] = roleUnknown(roles[index].Role, "the prior health observation state was unreadable", "", RemedyFact{Cause: CauseUnreadable})
				roles[index].DurationMillis = durationMillis
			}
		}
	}
	verdict := applyHealthObservation(repoRoot, previous.State, roles, now.UTC())
	verdict.Spend = spendObservation
	if err := saveHealthRecord(repoRoot, HealthRecordPath(repoRoot), healthRecord{State: verdict.State, Verdict: verdict}); err != nil {
		return HealthVerdict{}, err
	}
	return verdict, nil
}

// PreviewHealth renders the hook's current-turn facts without advancing the
// periodic observation breaker. The hook records its own attempt and
// completion; only the steward tick owns durable alert escalation.
func PreviewHealth(repoRoot string, now time.Time, prober identity.Prober) HealthVerdict {
	return PreviewHealthAt(repoRoot, repoRoot, now, prober)
}

// PreviewHealthAt reads durable health state from repoRoot and installed
// configuration, adapters and the stop fence from metasystemRoot.
func PreviewHealthAt(repoRoot, metasystemRoot string, now time.Time, prober identity.Prober) HealthVerdict {
	return previewHealthAtWithMeasure(repoRoot, repoRoot, metasystemRoot, now, prober, measureSpend)
}

// PreviewInstalledHealth reads the run-state roles (steward, supervision,
// sessions, jobs, proofs) and the stop fence from installation, where the
// process verbs arm and stop them, and the other roles from stateRoot.
func PreviewInstalledHealth(stateRoot, installation string, now time.Time, prober identity.Prober) HealthVerdict {
	return previewHealthAtWithMeasure(stateRoot, installation, installation, now, prober, measureSpend)
}

type spendMeasureFunc func(string, string, time.Time) (spend.Ledger, error)

func previewHealthAtWithMeasure(repoRoot, runRoot, metasystemRoot string, now time.Time, prober identity.Prober, measure spendMeasureFunc) HealthVerdict {
	return previewHealthAtWithEvaluation(repoRoot, metasystemRoot, now, prober,
		func(repo, installation string, at time.Time, probe identity.Prober, currentHook bool) ([]RoleVerdict, SpendObservation) {
			return evaluateHealthRolesWithMeasure(repo, runRoot, installation, at, probe, currentHook, measure)
		})
}

func previewHealthAtWithEvaluation(repoRoot, metasystemRoot string, now time.Time, prober identity.Prober, evaluate healthRoleEvaluator) HealthVerdict {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	roles, spendObservation := evaluate(repoRoot, metasystemRoot, now.UTC(), prober, true)
	if stopped, err := healthStopped(metasystemRoot, repoRoot, now.UTC(), roles, spendObservation, HealthObservationState{}); err == nil && stopped != nil {
		return *stopped
	}
	record, _ := loadHealthRecord(HealthRecordPath(repoRoot))
	roles = standingRoles(record.State, roles)
	aggregate, alert := healthSummary(roles)
	return HealthVerdict{
		Schema: 1, ObservedAt: now.UTC(), Observation: record.State.Sequence,
		Aggregate: aggregate, ShouldAlert: alert, State: record.State,
		Roles: roles, FindingDigest: healthFindingDigest(roles), Spend: spendObservation,
	}
}

// healthStopped reads the stop fence under fenceRoot, the installation whose
// process records it guards, and words its remedy for the repository.
func healthStopped(fenceRoot, repoRoot string, now time.Time, roles []RoleVerdict, spend SpendObservation, state HealthObservationState) (*HealthVerdict, error) {
	closed, record, err := stopfence.Closed(fenceRoot)
	if err != nil {
		return nil, fmt.Errorf("read process-creation fence for health: %w", err)
	}
	if !closed {
		return nil, nil
	}
	remedy, err := stopfence.ClosedCommand(record, repoRoot)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		if roles[index].Status != HealthAlive && processHealthRole(roles[index].Role) {
			roles[index].RemedyFacts = []RemedyFact{{Cause: CauseStopFenceClosed, Command: remedy}}
			roles[index].Remedy = remedyFor(roles[index].Role, roles[index].RemedyFacts[0]).Plain
		}
		roles[index].ConsecutiveUnknown = 0
		roles[index].ConsecutiveFailures = 0
		roles[index].FailureEscalation = ""
	}
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
	verdict := HealthVerdict{
		Schema: 1, ObservedAt: now.UTC(), Observation: state.Sequence,
		Aggregate: aggregate, Roles: roles, ShouldAlert: false, Stopped: true, StopPhase: record.Phase,
		StopUnresolved: len(record.NotStopped),
		State:          state, Spend: spend,
	}
	verdict.FindingDigest = healthFindingDigest(roles)
	return &verdict, nil
}

func evaluateHealthRoles(repoRoot, metasystemRoot string, now time.Time, prober identity.Prober, currentHookAttempt bool) ([]RoleVerdict, SpendObservation) {
	return evaluateHealthRolesWithMeasure(repoRoot, repoRoot, metasystemRoot, now, prober, currentHookAttempt, measureSpend)
}

func evaluateHealthRolesWithMeasure(repoRoot, runRoot, metasystemRoot string, now time.Time, prober identity.Prober, currentHookAttempt bool, measure spendMeasureFunc) ([]RoleVerdict, SpendObservation) {
	return evaluateHealthRolesWithLedger(repoRoot, runRoot, metasystemRoot, now, prober, currentHookAttempt, measure, newHealthLedger(repoRoot, now))
}

// roleApplies keeps session checks only where the ladder seats a session.
// Steward and ledger duties belong to every checkout with a tick.
func roleApplies(role HealthRole, checkout string) bool {
	switch role {
	case RoleSessionMain, RoleContext, RoleHookFreshness, RoleStopHookDuration:
		home, err := board.Home()
		if err == nil {
			record, registered, readErr := lane.Read(home)
			if readErr == nil && registered && lane.OwnsLane(checkout, record) {
				return false
			}
		}
		if role == RoleHookFreshness || role == RoleStopHookDuration {
			// Template stewards keep state in their nested installation;
			// its checkout owns the hook registration files.
			if filepath.Base(checkout) == "metasystem" && config.TemplateMode(checkout) {
				checkout = filepath.Dir(checkout)
			}
			return len(runtimereg.RegisteredRuntimes(checkout)) > 0
		}
	}
	return true
}

// evaluateHealthRolesWithLedger reads the roles kept wholly in run state under
// runRoot; the roles that also read the goal ledger or registers use repoRoot.
func evaluateHealthRolesWithLedger(repoRoot, runRoot, metasystemRoot string, now time.Time, prober identity.Prober, currentHookAttempt bool, measure spendMeasureFunc, ledger *healthLedger) ([]RoleVerdict, SpendObservation) {
	state, stateErr := readHealthObject(filepath.Join(runRoot, "artifacts", "agents", "supervision", "state.json"))
	spendStarted := time.Now()
	spendRole, spendObservation := checkSpendFenceWithMeasure(repoRoot, now, measure)
	spendRole.DurationMillis = elapsedRoleMillis(spendStarted)
	timed := func(name HealthRole, check func() RoleVerdict) RoleVerdict {
		if !roleApplies(name, runRoot) {
			return RoleVerdict{}
		}
		started := time.Now()
		role := check()
		role.DurationMillis = elapsedRoleMillis(started)
		return role
	}
	roles := []RoleVerdict{
		timed(RoleStewardRunner, func() RoleVerdict { return checkStewardRunner(runRoot, now, prober) }),
		timed(RoleSupervisionOwner, func() RoleVerdict { return checkSupervisionOwner(runRoot, prober) }),
		timed(RoleRepoWatcher, func() RoleVerdict { return checkRepoWatcher(runRoot, now, state, stateErr, prober) }),
		timed(RoleCensusFreshness, func() RoleVerdict { return checkCensusFreshness(runRoot, now, state, stateErr) }),
		timed(RoleNarratorFreshness, func() RoleVerdict { return checkNarratorFreshness(runRoot, now) }),
		timed(RoleSessionMain, func() RoleVerdict {
			return checkSessionMainWithLedger(repoRoot, runRoot, now, prober, ledger, defaultSeatDependencies(HealthSeatLauncher))
		}),
		timed(RoleHookFreshness, func() RoleVerdict { return checkHookFreshnessAt(runRoot, now, currentHookAttempt) }),
		timed(RoleStopHookDuration, func() RoleVerdict { return checkStopHookDuration(runRoot) }),
		timed(RoleContext, func() RoleVerdict { return checkInstalledContextBudget(metasystemRoot, now, prober) }),
		timed(RoleLedgerAttention, func() RoleVerdict { return checkLedgerAttention(repoRoot, now) }),
		timed(RoleSeatPresence, func() RoleVerdict { return checkSeatPresence(runRoot, now) }),
		timed(RoleClaimedGoalBudget, func() RoleVerdict { return checkClaimedGoalBudgetsWith(repoRoot, now, ledger) }),
		timed(RoleStopCapabilityEpoch, func() RoleVerdict { return checkStopCapabilityEpochWith(repoRoot, now, ledger) }),
		timed(RoleClaimedGoalDelivery, func() RoleVerdict { return checkClaimedGoalDeliveryWith(repoRoot, now, ledger) }),
		timed(RoleTrunkRed, func() RoleVerdict { return checkTrunkRedWith(repoRoot, now, ledger) }),
		spendRole,
		timed(RoleGovernedObligations, func() RoleVerdict { return checkGovernedObligations(repoRoot) }),
		timed(RoleNonterminalJobs, func() RoleVerdict { return checkNonterminalJobs(runRoot, prober) }),
		timed(RoleProofAttempts, func() RoleVerdict { return checkProofAttempts(runRoot, prober) }),
		timed(RoleProofAdmission, func() RoleVerdict { return checkProofAdmission(runRoot, now, inspectHostLeases) }),
		timed(RoleCapabilitySnapshots, func() RoleVerdict { return checkCapabilitySnapshots(runRoot, metasystemRoot, now) }),
		timed(RoleDisk, func() RoleVerdict { return checkDisk(runRoot) }),
	}
	return slices.DeleteFunc(roles, func(role RoleVerdict) bool { return role.Role == "" }), spendObservation
}

func elapsedRoleMillis(started time.Time) int64 {
	elapsed := time.Since(started).Milliseconds()
	if elapsed < 1 {
		return 1
	}
	return elapsed
}

var measureSpend = spend.Measure

// HealthSeatLauncher is the command layer's seat launcher, also read by health.
var HealthSeatLauncher SeatLauncher

func checkSpendFence(repoRoot string, now time.Time) (RoleVerdict, SpendObservation) {
	return checkSpendFenceWithMeasure(repoRoot, now, measureSpend)
}

func checkSpendFenceWithMeasure(repoRoot string, now time.Time, measure spendMeasureFunc) (RoleVerdict, SpendObservation) {
	return checkSpendFenceWithMeasureAndMachine(repoRoot, now, measure, goal.ResolveMachine)
}

func checkSpendFenceWithMeasureAndMachine(repoRoot string, now time.Time, measure spendMeasureFunc, resolveMachine func(string) (string, error)) (RoleVerdict, SpendObservation) {
	machine := "this machine"
	if enrolled, err := resolveMachine(repoRoot); err == nil {
		machine = enrolled
	}
	ledger, err := measure(repoRoot, machine, now)
	remedy := remedyFor(RoleSpendFence, RemedyFact{}).Plain
	if err != nil {
		return roleUnknown(RoleSpendFence, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), SpendObservation{Valid: false}
	}
	settings := ledger.Settings
	crossings := make([]SpendCrossing, 0, 4+len(ledger.ClaimedGoals)*2)
	addCrossing := func(scope spend.ScopeSummary, scopeName, ceiling string, value, limit float64) {
		if limit <= 0 || value < limit {
			return
		}
		crossings = append(crossings, SpendCrossing{
			ScopeID: scope.ID, Scope: scopeName, Ceiling: ceiling, Multiple: int(math.Floor(value / limit)),
			Machine: ledger.Machine, Spend: value, Limit: limit, Day: ledger.Day,
		})
	}
	addCrossing(ledger.DayScope, "day", "tokens", ledger.DayScope.Tokens, float64(settings.DayTokenCeiling))
	addCrossing(ledger.DayScope, "day", "money", ledger.DayScope.Money, settings.DayMoneyCeiling)
	goals := append([]string(nil), ledger.ClaimedGoals...)
	if len(goals) == 0 {
		goals = []string{"none"}
	}
	for _, goalID := range goals {
		scope := ledger.GoalScopes[goalID]
		if scope.ID == "" {
			scope = spend.ScopeSummary{ID: "goal-" + goalID, Goal: goalID, Machine: ledger.Machine}
		}
		addCrossing(scope, "goal", "tokens", scope.Tokens, float64(settings.GoalTokenCeiling))
		addCrossing(scope, "goal", "money", scope.Money, settings.GoalMoneyCeiling)
	}
	sort.Slice(crossings, func(i, j int) bool {
		left := crossings[i].ScopeID + "." + crossings[i].Ceiling
		right := crossings[j].ScopeID + "." + crossings[j].Ceiling
		return left < right
	})
	prefix := ""
	if len(crossings) > 0 {
		labels := make([]string, len(crossings))
		for index, crossing := range crossings {
			labels[index] = fmt.Sprintf("%s.%sx%d", crossing.ScopeID, crossing.Ceiling, crossing.Multiple)
		}
		prefix = "CROSSED " + strings.Join(labels, ",") + " "
	}
	reason := fmt.Sprintf("%smode=%s day=%s tokens=%.0f/%d money=%s%.2f/%.2f unpriced=%d unmeasured=%d unreadable=%d inflight=%d; seat tokens=%.0f lifetime=%.0f files=%d aged=%d unreadable=%d unmeasured requests=%d",
		prefix, settings.Mode, ledger.Day, ledger.DayScope.Tokens, settings.DayTokenCeiling,
		settings.Currency, ledger.DayScope.Money, settings.DayMoneyCeiling, ledger.DayScope.Unpriced,
		ledger.DayScope.Unmeasured, ledger.DayScope.Unreadable, ledger.DayScope.Inflight,
		ledger.Seat.DayTokens, ledger.Seat.LifetimeTokens, ledger.Seat.Files, ledger.Seat.AgedFiles,
		ledger.Seat.UnreadableFiles, ledger.Seat.UnmeasuredRequests)
	for _, goalID := range goals {
		scope := ledger.GoalScopes[goalID]
		reason += fmt.Sprintf("; goal=%s tokens=%.0f/%d money=%s%.2f/%.2f unpriced=%d unmeasured=%d unreadable=%d",
			goalID, scope.Tokens, settings.GoalTokenCeiling, settings.Currency, scope.Money,
			settings.GoalMoneyCeiling, scope.Unpriced, scope.Unmeasured, scope.Unreadable)
	}
	role := roleAlive(RoleSpendFence, reason)
	if len(crossings) > 0 {
		role.Remedy = remedy
	}
	return role, SpendObservation{Valid: true, Crossings: crossings}
}

func checkHookFreshness(repoRoot string, now time.Time) RoleVerdict {
	return checkHookFreshnessAt(repoRoot, now, false)
}

func checkHookFreshnessAt(repoRoot string, now time.Time, currentAttempt bool) RoleVerdict {
	// A hook turn is recorded by the next agent turn the registered hooks
	// see; registering them is the act a person can take.
	record, durabilityPending, err := loadComponentEvidenceForHealth(repoRoot, "supervision-hook")
	if err != nil {
		var busy *ComponentEvidenceBusyError
		if errors.As(err, &busy) {
			return roleUnknown(RoleHookFreshness, busy.Error(), "", RemedyFact{Cause: CauseObservationPending})
		}
		if os.IsNotExist(err) {
			return roleDead(RoleHookFreshness, "no hook turn generation is recorded", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleHookFreshness, "the hook completion evidence is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	if record.Generation < 1 || record.TurnKeyDigest == "" {
		return roleUnknown(RoleHookFreshness, "the hook turn generation is incomplete", "", RemedyFact{Cause: CauseUnreadable})
	}
	if record.LastAttempt.After(now) || record.LastCompletion.After(now) || record.LastSuccess.After(now) {
		return roleUnknown(RoleHookFreshness, "CLOCK_REGRESSED: hook evidence is later than current UTC", "", RemedyFact{Cause: CauseClockRegressed})
	}
	if durabilityPending || record.Outcome == "DURABILITY_PENDING" {
		return roleUnknown(RoleHookFreshness, "the hook completion is waiting for durability proof", "", RemedyFact{Cause: CauseObservationPending})
	}
	openAttempt := record.Outcome == "ATTEMPTING" || record.LastCompletion.Before(record.LastAttempt)
	if openAttempt && now.Sub(record.LastAttempt) >= stopHookBudgetSeconds*time.Second {
		return roleDead(RoleHookFreshness, fmt.Sprintf("turn generation %d has an attempt without completion past the %ds Stop budget", record.Generation, stopHookBudgetSeconds), "", RemedyFact{Cause: CauseUnavailable})
	}
	if currentAttempt && openAttempt {
		if len(record.AttemptHistory) == 0 {
			return roleUnknown(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending with no prior completed turn", record.Generation), "", RemedyFact{Cause: CauseObservationPending})
		}
		prior := record.AttemptHistory[len(record.AttemptHistory)-1]
		if prior.Result == ComponentOK && prior.Outcome == "EMITTED" {
			return roleAlive(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending; prior generation %d completed as OK/EMITTED", record.Generation, prior.Generation))
		}
		return roleUnknown(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending after prior generation %d ended as %s/%s", record.Generation, prior.Generation, prior.Result, prior.Outcome), "", RemedyFact{Cause: CauseObservationPending})
	}
	if openAttempt {
		return roleUnknown(RoleHookFreshness, fmt.Sprintf("turn generation %d is pending within the %ds Stop budget", record.Generation, stopHookBudgetSeconds), "", RemedyFact{Cause: CauseObservationPending})
	}
	if record.Result != ComponentOK || record.Outcome != "EMITTED" ||
		record.SuccessAttemptSeq != record.AttemptSeq || !record.LastSuccess.Equal(record.LastCompletion) {
		return roleDead(RoleHookFreshness, fmt.Sprintf("turn generation %d did not complete as OK/EMITTED", record.Generation), "", RemedyFact{Cause: CauseUnavailable})
	}
	return roleAlive(RoleHookFreshness, fmt.Sprintf("turn generation %d completed as OK/EMITTED", record.Generation))
}

// stopHookBudgetSeconds matches the Stop budget shipped by the registration
// templates under metasystem/internal/runtimes/enforcement.
const stopHookBudgetSeconds = 60

var defaultStopHookSlowSeconds = config.MustIntDefault("steward.stop-slow-sec")

func checkStopHookDuration(repoRoot string) RoleVerdict {
	return checkStopHookDurationWithMachine(repoRoot, goal.ResolveMachine)
}

func checkStopHookDurationWithMachine(repoRoot string, machineName func(string) (string, error)) RoleVerdict {
	record, _, err := loadComponentEvidenceForHealth(repoRoot, "supervision-hook")
	if err != nil {
		var busy *ComponentEvidenceBusyError
		if errors.As(err, &busy) {
			return roleUnknown(RoleStopHookDuration, busy.Error(), "", RemedyFact{Cause: CauseObservationPending})
		}
		if os.IsNotExist(err) {
			return roleAlive(RoleStopHookDuration, "no Stop has been measured yet")
		}
		return roleUnknown(RoleStopHookDuration, "the Stop duration evidence is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}

	outcome := record.Outcome
	elapsed := record.LastStopElapsedSec
	generation, attemptSeq := record.Generation, record.AttemptSeq
	if record.Outcome == "ATTEMPTING" {
		outcome = ""
		elapsed = nil
		if size := len(record.AttemptHistory); size > 0 {
			latest := record.AttemptHistory[size-1]
			outcome = latest.Outcome
			elapsed = latest.StopElapsedSec
			generation, attemptSeq = latest.Generation, latest.AttemptSeq
		}
	}
	if elapsed == nil {
		return roleAlive(RoleStopHookDuration, "the last Stop carried no measurement")
	}
	if stopRearmedEngine(repoRoot, generation, attemptSeq) {
		return roleAlive(RoleStopHookDuration, fmt.Sprintf("the last Stop took %ds and re-armed the rebuilt engine", *elapsed))
	}

	machine := "this machine"
	if enrolled, machineErr := machineName(repoRoot); machineErr == nil {
		machine = enrolled
	}
	if outcome == "DEADLINE_EXPIRED" {
		return roleDead(RoleStopHookDuration,
			fmt.Sprintf("the last Stop expired its deadline after %ds of the %ds budget on %s", *elapsed, stopHookBudgetSeconds, machine), "", RemedyFact{Cause: CauseUnavailable})
	}

	threshold, err := boundedConfig(repoRoot, "steward.stop-slow-sec", defaultStopHookSlowSeconds, 1)
	if err != nil {
		return roleUnknown(RoleStopHookDuration, "the Stop slow threshold is invalid: "+err.Error(), "", RemedyFact{Cause: CauseSettingsInvalid})
	}
	if *elapsed >= int64(threshold) {
		return roleDead(RoleStopHookDuration,
			fmt.Sprintf("the last Stop took %ds of the %ds budget on %s; the threshold is %ds", *elapsed, stopHookBudgetSeconds, machine, threshold), "", RemedyFact{Cause: CauseUnavailable})
	}
	return roleAlive(RoleStopHookDuration,
		fmt.Sprintf("the last Stop took %ds of the %ds budget", *elapsed, stopHookBudgetSeconds))
}

// A re-arm belongs only to the Stop named by its arming record, including
// when a later attempt is open and health reads the completed history.
func stopRearmedEngine(repoRoot string, generation int, attemptSeq int64) bool {
	file, err := os.Open(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "arming.log"))
	if err != nil {
		return false
	}
	defer file.Close()
	want := fmt.Sprintf("stop-re-armed %d %d", generation, attemptSeq)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		_, event, ok := strings.Cut(scanner.Text(), " ")
		if ok && event == want {
			return true
		}
	}
	return false
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
	failureCauses := make(map[HealthRole]string, len(healthRoleOrder))
	for key, value := range previous.FailureCauses {
		failureCauses[key] = value
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
		for index := range ordered {
			if ordered[index].Status == HealthAlive {
				ordered[index].Status = HealthUnknown
				ordered[index].Reason = "CLOCK_REGRESSED: the prior health observation is later than current UTC"
				ordered[index].RemedyFacts = []RemedyFact{{Cause: CauseClockRegressed}}
				ordered[index].Remedy = remedyFor(ordered[index].Role, ordered[index].RemedyFacts[0]).Plain
			}
		}
	}
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
			continue
		}
		unknownCounts[role.Role] = 0
		if role.Status == HealthDead {
			cause := healthCause(*role)
			if !hasLawfulAutomaticRemedy(*role, ordered) && failureCauses[role.Role] != cause {
				failureCounts[role.Role] = 0
			}
			failureCauses[role.Role] = cause
			if failureCounts[role.Role] == 0 {
				failureEpisodes[role.Role] = append(failureEpisodes[role.Role], observedAt)
			}
			failureCounts[role.Role]++
			if failureCounts[role.Role] >= healthFailureLimit {
				failureCounts[role.Role] = healthFailureLimit
			}
			continue
		}
		failureCounts[role.Role] = 0
		delete(failureCauses, role.Role)
	}
	state := HealthObservationState{
		Sequence: previous.Sequence + 1, ObservedAt: observedAt,
		UnknownCounts: unknownCounts, FailureCounts: failureCounts, FailureCauses: failureCauses, FailureEpisodes: failureEpisodes,
	}
	ordered = standingRoles(state, ordered)
	aggregate, alert := healthSummary(ordered)
	verdict := HealthVerdict{
		Schema: 1, ObservedAt: observedAt, Observation: state.Sequence,
		Aggregate: aggregate, Roles: ordered, ShouldAlert: alert, State: state,
	}
	verdict.FindingDigest = healthFindingDigest(ordered)
	return verdict
}

// healthCause keeps diagnostic ages, counts and identities out of a failure's
// identity. Checks can supply a cause when words alone cannot distinguish it.
func healthCause(role RoleVerdict) string {
	if role.Cause != "" {
		return role.Cause
	}
	words := strings.Fields(role.Reason)
	kept := words[:0]
	for _, word := range words {
		if strings.IndexFunc(word, unicode.IsDigit) < 0 {
			kept = append(kept, word)
		}
	}
	return strings.Join(kept, " ")
}

// standingRoles projects the tick's counters without advancing them. A role
// keeps its dead diagnostic when its failure belongs to a standing defect.
func standingRoles(state HealthObservationState, roles []RoleVerdict) []RoleVerdict {
	projected := append([]RoleVerdict(nil), roles...)
	for index := range projected {
		role := &projected[index]
		role.Standing, role.ConsecutiveFailures, role.ConsecutiveUnknown, role.FailureEscalation = false, 0, 0, ""
		if role.Status == HealthUnknown {
			role.ConsecutiveUnknown = state.UnknownCounts[role.Role]
		}
		if role.Status != HealthDead {
			continue
		}
		lawful := hasLawfulAutomaticRemedy(*role, roles)
		if lawful || state.FailureCauses[role.Role] == healthCause(*role) {
			role.ConsecutiveFailures = state.FailureCounts[role.Role]
		}
		switch {
		case !lawful:
			role.FailureEscalation = NoLawfulRemedy
			role.Standing = role.ConsecutiveFailures >= healthFailureLimit
		case role.ConsecutiveFailures >= healthFailureLimit:
			role.FailureEscalation = AutoHealEnded
			if role.Role == RoleCapabilitySnapshots || role.Role == RoleLedgerAttention {
				role.Remedy = healthClearRemedy(role.Role, "", role.Reason).Plain
			}
		case len(state.FailureEpisodes[role.Role]) >= healthFlapLimit:
			role.FailureEscalation = HealingFlapping
		default:
			role.FailureEscalation = AutoHealEligible
		}
	}
	return projected
}

func healthSummary(roles []RoleVerdict) (string, bool) {
	aggregate, alert := "healthy", false
	for _, role := range roles {
		if role.Standing {
			continue
		}
		switch role.Status {
		case HealthDead:
			aggregate = "unhealthy"
			alert = alert || role.FailureEscalation != AutoHealEligible
		case HealthUnknown:
			if aggregate == "healthy" {
				aggregate = "unknown"
			}
			alert = alert || role.ConsecutiveUnknown >= 2
		}
	}
	return aggregate, alert
}

func fileStandingDefects(repoRoot string, health HealthVerdict, now time.Time, deliver func(string, string) error) error {
	standing := make(map[HealthRole]RoleVerdict)
	for _, role := range health.Roles {
		if role.Standing {
			standing[role.Role] = role
		}
	}
	_, err := UpdatePatterns(repoRoot, PatternCycle{Now: now, ClearTicks: 1, Deliver: deliver,
		Step: func([]byte) ([]byte, []PatternRun, error) {
			observations := make([]PatternObservation, 0, len(healthRoleOrder))
			for _, name := range healthRoleOrder {
				observation := PatternObservation{Kind: ObsClear, Work: string(name)}
				if role, ok := standing[name]; ok {
					observation.Kind = ObsFinding
					observation.Since = now.UTC()
					observation.Message = fmt.Sprintf("The %s role has a standing health defect: %s; remedy: %s", name, role.Reason, role.Remedy)
					observation.Evidence = []AlertEvidence{{Record: HealthRecordPath(repoRoot), At: now.UTC().Format(time.RFC3339Nano),
						Fact: role.Reason + "; remedy: " + role.Remedy}}
				}
				observations = append(observations, observation)
			}
			return nil, []PatternRun{{Pattern: "health-standing-red", Observations: observations}}, nil
		}})
	return err
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
	case RoleStewardRunner, RoleSessionMain, RoleCapabilitySnapshots, RoleLedgerAttention:
		return true
	case RoleRepoWatcher:
		return watcherRepairable(role)
	case RoleCensusFreshness:
		// A failed census prevents the watcher pass from completing, so the
		// watcher's owner replaces the producer that owns this evidence.
		return roleIsDead(RoleRepoWatcher)
	case RoleNarratorFreshness:
		// The watcher repairs the steward process whose failed tick also stops
		// narration. An isolated narrator failure has no separate automatic act.
		return roleIsDead(RoleStewardRunner)
	case RoleSeatPresence:
		// The next tick republishes by itself, so a dead presence role must
		// follow the ordinary consecutive-failure path rather than escalate
		// on its first dead observation.
		return true
	case RoleClaimedGoalBudget, RoleStopCapabilityEpoch:
		return !role.NoAutomaticRemedy
	default:
		return false
	}
}

func checkStewardRunner(repoRoot string, now time.Time, prober identity.Prober) RoleVerdict {
	return checkStewardRunnerWithCadence(repoRoot, now, prober, TickSeconds)
}

func checkStewardRunnerWithCadence(repoRoot string, now time.Time, prober identity.Prober, tickSeconds func(string) int) RoleVerdict {
	installed, durabilityPending, installationErr := installedEnrollment(repoRoot)
	withEnrollment := func(verdict RoleVerdict) RoleVerdict {
		if installationErr != nil {
			return verdict
		}
		provenance := EnrollmentProvenance(installed)
		if line := RearmDeferredLine(repoRoot); line != "" {
			provenance += "; " + line
		}
		if durabilityPending {
			provenance += " (durability pending)"
		}
		if verdict.Reason == "" {
			verdict.Reason = provenance
		} else {
			verdict.Reason += "; " + provenance
		}
		return verdict
	}
	path := runnerRecordPath(repoRoot)
	var runner RunnerRecord
	if err := readJSON(path, &runner); err != nil {
		if os.IsNotExist(err) {
			return withEnrollment(roleDead(RoleStewardRunner, "no steward runner is recorded", "", RemedyFact{Cause: CauseUnavailable}))
		}
		return withEnrollment(roleUnknown(RoleStewardRunner, "the steward runner record is unreadable", "", RemedyFact{Cause: CauseUnreadable}))
	}
	process := identity.Ref{Pid: runner.Pid, StartedAtSec: runner.PidStartedAt, StartTicks: runner.StartTicks, BootID: runner.BootID}
	switch identity.AliveRef(prober, process) {
	case identity.Dead:
		return withEnrollment(roleDead(RoleStewardRunner, fmt.Sprintf("recorded runner pid %d is dead", runner.Pid), "", RemedyFact{Cause: CauseUnavailable}))
	case identity.Unknown:
		return withEnrollment(roleUnknown(RoleStewardRunner, fmt.Sprintf("recorded runner pid %d cannot be inspected", runner.Pid), "", RemedyFact{Cause: CauseUnreadable}))
	}
	if installationErr != nil {
		return roleUnknown(RoleStewardRunner, "the steward installation generation is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	generation := installed.Generation
	record, _, evidenceErr := loadComponentEvidenceForHealth(repoRoot, "steward-tick")
	var busy *ComponentEvidenceBusyError
	if errors.As(evidenceErr, &busy) {
		return withEnrollment(roleUnknown(RoleStewardRunner, busy.Error(), "", RemedyFact{Cause: CauseObservationPending}))
	}
	if evidenceErr == nil && record.Generation == generation && record.Outcome == "ATTEMPTING" {
		attemptProcess := identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt, StartTicks: record.PidStartTicks, BootID: record.BootID}
		if sameComponentProcess(attemptProcess, process) {
			if record.LastAttempt.After(now) {
				return withEnrollment(roleUnknown(RoleStewardRunner, "CLOCK_REGRESSED: tick attempt evidence is later than current UTC", "", RemedyFact{Cause: CauseClockRegressed}))
			}
			patience, patienceErr := runnerTickPatience(repoRoot, record.LastDurationMillis)
			if patienceErr != nil {
				return withEnrollment(roleUnknown(RoleStewardRunner, "the steward tick patience is invalid: "+patienceErr.Error(), "", RemedyFact{Cause: CauseSettingsInvalid}))
			}
			age := now.Sub(record.LastAttempt)
			if age < patience {
				return withEnrollment(roleAlive(RoleStewardRunner, fmt.Sprintf("runner pid %d is attempting generation %d (age %s, patience %s)", runner.Pid, generation, age.Round(time.Second), patience)))
			}
			return withEnrollment(roleDead(RoleStewardRunner, fmt.Sprintf("runner pid %d attempt is stuck at %s (patience %s)", runner.Pid, age.Round(time.Second), patience), "", RemedyFact{Cause: CauseUnavailable}))
		}
	}
	return withEnrollment(componentFreshness(repoRoot, "steward-tick", RoleStewardRunner, generation, time.Duration(2*tickSeconds(repoRoot))*time.Second, now, "", &process,
		fmt.Sprintf("runner pid %d and generation %d success are current", runner.Pid, generation)))
}

func runnerTickPatience(repoRoot string, lastDurationMillis int64) (time.Duration, error) {
	floorSeconds := config.MustIntDefault("steward.tick-patience-sec")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "metasystem.conf")); statErr == nil || !os.IsNotExist(statErr) {
		configured, err := boundedConfig(repoRoot, "steward.tick-patience-sec", floorSeconds, 1)
		if err != nil {
			return 0, err
		}
		floorSeconds = configured
	}
	const maxPatienceSeconds = int64(^uint64(0)>>1) / int64(time.Second)
	if int64(floorSeconds) > maxPatienceSeconds {
		return 0, fmt.Errorf("steward.tick-patience-sec must be no greater than %d", maxPatienceSeconds)
	}
	patience := time.Duration(floorSeconds) * time.Second
	if lastDurationMillis <= 0 {
		return patience, nil
	}
	const maxDurationMillis = int64(^uint64(0)>>1) / int64(time.Millisecond)
	if lastDurationMillis > maxDurationMillis/3 {
		return time.Duration(1<<63 - 1), nil
	}
	measured := 3 * time.Duration(lastDurationMillis) * time.Millisecond
	if measured > patience {
		patience = measured
	}
	return patience, nil
}

func checkSupervisionOwner(repoRoot string, prober identity.Prober) RoleVerdict {
	owner, err := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "lock.d", "owner.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleSupervisionOwner, "no supervision owner holds the repository lock", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleSupervisionOwner, "the supervision owner lock is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	ref, ok := processRef(owner)
	if !ok {
		return roleUnknown(RoleSupervisionOwner, "the lock owner's process identity is incomplete", "", RemedyFact{Cause: CauseUnreadable})
	}
	switch identity.AliveRef(prober, ref) {
	case identity.Alive:
		return roleAlive(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d is alive", ref.Pid))
	case identity.Dead:
		return roleDead(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d is dead", ref.Pid), "", RemedyFact{Cause: CauseUnavailable})
	default:
		return roleUnknown(RoleSupervisionOwner, fmt.Sprintf("lock owner pid %d cannot be inspected", ref.Pid), "", RemedyFact{Cause: CauseUnreadable})
	}
}

func checkRepoWatcher(repoRoot string, now time.Time, state map[string]any, stateErr error, prober identity.Prober) RoleVerdict {
	if stateErr != nil {
		if os.IsNotExist(stateErr) {
			return roleDead(RoleRepoWatcher, "no supervision state is recorded", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleRepoWatcher, "the supervision state is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	var entry map[string]any
	if components, ok := state["components"].(map[string]any); ok {
		entry, _ = components["watcher"].(map[string]any)
	}
	if entry == nil {
		return roleDead(RoleRepoWatcher, "the role has no recorded process", "", RemedyFact{Cause: CauseUnavailable})
	}
	ref, ok := processRef(entry)
	if !ok {
		return roleUnknown(RoleRepoWatcher, "the recorded process identity is incomplete", "", RemedyFact{Cause: CauseUnreadable})
	}
	switch identity.AliveRef(prober, ref) {
	case identity.Dead:
		return roleDead(RoleRepoWatcher, fmt.Sprintf("recorded pid %d is dead", ref.Pid), "", RemedyFact{Cause: CauseUnavailable})
	case identity.Unknown:
		return roleUnknown(RoleRepoWatcher, fmt.Sprintf("recorded pid %d cannot be inspected", ref.Pid), "", RemedyFact{Cause: CauseUnreadable})
	}
	generation, generationOK := healthInt(state["generation"])
	interval, intervalOK := healthInt(state["intervalSec"])
	if !generationOK || generation < 1 || !intervalOK || interval < 1 {
		return roleUnknown(RoleRepoWatcher, "the watcher generation or producer interval is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	return componentFreshness(repoRoot, "repo-watcher", RoleRepoWatcher, int(generation), time.Duration(2*interval)*time.Second,
		now, "", &ref, fmt.Sprintf("watcher pid %d and generation %d success are current", ref.Pid, generation))
}

func checkCensusFreshness(repoRoot string, now time.Time, state map[string]any, stateErr error) RoleVerdict {
	census, err := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "last-census.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleCensusFreshness, "no census success is recorded", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleCensusFreshness, "the census evidence is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	if result, _ := census["verdict"].(string); result != "SUCCESS" {
		return roleDead(RoleCensusFreshness, "the latest census did not succeed", "", RemedyFact{Cause: CauseUnavailable})
	}
	if stateErr != nil {
		return roleUnknown(RoleCensusFreshness, "the census generation cannot be compared with supervision", "", RemedyFact{Cause: CauseUnreadable})
	}
	wantGeneration, wantOK := healthInt(state["generation"])
	gotGeneration, gotOK := healthInt(census["generation"])
	if !wantOK || !gotOK || wantGeneration < 1 || gotGeneration < 1 {
		return roleUnknown(RoleCensusFreshness, "the census generation evidence is incomplete", "", RemedyFact{Cause: CauseUnreadable})
	}
	if wantGeneration != gotGeneration {
		return roleDead(RoleCensusFreshness, fmt.Sprintf("census generation %d does not match supervision generation %d", gotGeneration, wantGeneration), "", RemedyFact{Cause: CauseUnavailable})
	}
	lastSuccess, ok := evidenceSuccessTime(census)
	if !ok {
		return roleUnknown(RoleCensusFreshness, "the census has no readable lastSuccess", "", RemedyFact{Cause: CauseUnreadable})
	}
	age := now.Sub(lastSuccess)
	if age < 0 {
		return roleUnknown(RoleCensusFreshness, "CLOCK_REGRESSED: census lastSuccess is later than current UTC", "", RemedyFact{Cause: CauseClockRegressed})
	}
	intervalSeconds, ok := healthInt(census["intervalSec"])
	if !ok || intervalSeconds < 1 {
		return roleUnknown(RoleCensusFreshness, "the census producer interval is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	window := time.Duration(2*intervalSeconds) * time.Second
	if age >= window {
		return roleDead(RoleCensusFreshness, fmt.Sprintf("census lastSuccess is stale at %s", age.Round(time.Second)), "", RemedyFact{Cause: CauseUnavailable})
	}
	return roleAlive(RoleCensusFreshness, fmt.Sprintf("census generation %d succeeded %s ago", gotGeneration, age.Round(time.Second)))
}

func checkNarratorFreshness(repoRoot string, now time.Time) RoleVerdict {
	return checkNarratorFreshnessWithCadence(repoRoot, now, TickSeconds)
}

func checkNarratorFreshnessWithCadence(repoRoot string, now time.Time, tickSeconds func(string) int) RoleVerdict {
	generation, err := installedGeneration(repoRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return roleUnknown(RoleNarratorFreshness, "no steward installation generation is recorded", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleNarratorFreshness, "the steward installation generation is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	return componentFreshness(repoRoot, "narrator", RoleNarratorFreshness, generation, time.Duration(2*tickSeconds(repoRoot))*time.Second, now, "", nil,
		fmt.Sprintf("narrator generation %d success is current", generation))
}

func componentFreshness(repoRoot, component string, role HealthRole, generation int, window time.Duration, now time.Time, _ string, expectedSuccess *identity.Ref, aliveReason string) RoleVerdict {
	record, durabilityPending, err := loadComponentEvidenceForHealth(repoRoot, component)
	if err != nil {
		var busy *ComponentEvidenceBusyError
		if errors.As(err, &busy) {
			return roleUnknown(role, busy.Error(), "", RemedyFact{Cause: CauseObservationPending})
		}
		if os.IsNotExist(err) {
			return roleDead(role, "no successful component pass is recorded", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(role, "the component success evidence is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	if record.LastSuccess.After(now) || record.LastCompletion.After(now) || record.LastAttempt.After(now) {
		return roleUnknown(role, "CLOCK_REGRESSED: component evidence is later than current UTC", "", RemedyFact{Cause: CauseClockRegressed})
	}
	if durabilityPending || record.Outcome == "DURABILITY_PENDING" {
		return roleUnknown(role, "the latest completion is waiting for durability proof", "", RemedyFact{Cause: CauseObservationPending})
	}
	if role == RoleRepoWatcher && record.Outcome != "ATTEMPTING" && record.Result != ComponentOK {
		reason := "the latest watcher pass failed: " + record.Outcome
		if record.LastFailure != "" {
			reason += ": " + record.LastFailure
		}
		return roleDead(role, reason, "", RemedyFact{Cause: CauseUnavailable})
	}
	if record.Generation != generation && role != RoleNarratorFreshness {
		return roleDead(role, fmt.Sprintf("component generation %d does not match installation generation %d", record.Generation, generation), "", RemedyFact{Cause: CauseUnavailable})
	}
	if record.LastSuccess.IsZero() {
		return roleDead(role, "the current generation has no successful completion", "", RemedyFact{Cause: CauseUnavailable})
	}
	if expectedSuccess != nil {
		success := identity.Ref{Pid: record.SuccessPid, StartedAtSec: record.SuccessPidStartedAt, StartTicks: record.SuccessPidStartTicks, BootID: record.SuccessBootID}
		if !sameComponentProcess(success, *expectedSuccess) {
			return roleDead(role, fmt.Sprintf("lastSuccess belongs to pid %d, not resident runner pid %d", success.Pid, expectedSuccess.Pid), "", RemedyFact{Cause: CauseUnavailable})
		}
	}
	if record.Outcome == "ATTEMPTING" && now.Sub(record.LastAttempt) >= window {
		return roleDead(role, "the latest attempt passed its deadline without completion", "", RemedyFact{Cause: CauseUnavailable})
	}
	if now.Sub(record.LastSuccess) >= window {
		return roleDead(role, fmt.Sprintf("lastSuccess is stale at %s", now.Sub(record.LastSuccess).Round(time.Second)), "", RemedyFact{Cause: CauseUnavailable})
	}
	if record.Generation != generation && role == RoleNarratorFreshness {
		return roleAlive(role, fmt.Sprintf("narrator generation %d success is fresh; waiting for the first pass of installation generation %d", record.Generation, generation))
	}
	return roleAlive(role, aliveReason)
}

// watcherRepairable names the failures the supervision owner can repair by
// replacing its current watcher, including a replacement whose pass failed.
func watcherRepairable(role RoleVerdict) bool {
	return strings.Contains(role.Reason, "recorded pid") ||
		strings.Contains(role.Reason, "lastSuccess is stale") ||
		strings.Contains(role.Reason, "latest attempt passed its deadline") ||
		strings.HasPrefix(role.Reason, "the latest watcher pass failed:")
}

func checkSessionMainWithLedger(repoRoot, runRoot string, now time.Time, prober identity.Prober, ledger *healthLedger, dependencies seatDependencies) RoleVerdict {
	return checkSessionMainForSeat(runRoot, prober, func(records []SeatRecord) (Decision, *SeatSelection, error) {
		if !ledger.read().newWorld {
			return Decision{}, nil, nil
		}
		if ledger.endpointErr != nil {
			return Decision{}, nil, ledger.endpointErr
		}
		if ledger.projectionErr != nil {
			return Decision{}, nil, ledger.projectionErr
		}
		machine, err := dependencies.Machine(repoRoot)
		if err != nil {
			return Decision{}, nil, err
		}
		work, err := goal.ClaimableWorkFromProjection(ledger.projection, machine, prober)
		if err != nil {
			return Decision{}, nil, err
		}
		kind, _, _ := classifySharedBacklog(work)
		dependencies.Project = func(string, time.Time) (goal.Projection, error) { return ledger.projection, nil }
		_, providerOutage := standingProviderOutage(repoRoot, now, nil)
		decision, selection, _ := seatDecision(repoRoot, (TickConfig{Now: now}).withDefaults(), kind, work,
			Workers{CensusComplete: true}, providerOutage, dependencies, seatTickState{Records: records})
		if decision.Verdict == VerdictDegraded || decision.Verdict == VerdictUnknown {
			return decision, nil, errors.New(decision.Reason)
		}
		return decision, selection, nil
	})
}

// checkSessionMainForSeat judges the need for a successor through the seat
// ladder. The announcement reader also serves cleanup's process proof.
func checkSessionMainForSeat(runRoot string, prober identity.Prober, decide func([]SeatRecord) (Decision, *SeatSelection, error)) RoleVerdict {
	role := checkSessionMain(runRoot, prober)
	if role.Status != HealthDead {
		return role
	}
	records, err := readSeatRecords(runRoot)
	if err != nil {
		return roleUnknown(RoleSessionMain, "the seat ladder cannot read its launches: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	decision, selection, err := decide(records)
	if err != nil {
		return roleUnknown(RoleSessionMain, "the seat ladder cannot read its work: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	if selection == nil {
		return roleAlive(RoleSessionMain, "no seat step is due; "+decision.Reason)
	}
	return roleDead(RoleSessionMain, decision.Reason, "", RemedyFact{Cause: CauseUnavailable})
}

func checkSessionMain(repoRoot string, prober identity.Prober) RoleVerdict {
	directory := filepath.Join(repoRoot, "artifacts", "agents", "mains")
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return roleDead(RoleSessionMain, "no session main is announced", "", RemedyFact{Cause: CauseUnavailable})
		}
		return roleUnknown(RoleSessionMain, "session announcements are unreadable", "", RemedyFact{Cause: CauseUnreadable})
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
		return roleUnknown(RoleSessionMain, "no announced main has readable liveness", "", RemedyFact{Cause: CauseUnreadable})
	}
	if dead {
		return roleDead(RoleSessionMain, "every announced session main is dead", "", RemedyFact{Cause: CauseUnavailable})
	}
	return roleDead(RoleSessionMain, "no session main is announced", "", RemedyFact{Cause: CauseUnavailable})
}

func checkClaimedGoalBudgetsWith(repoRoot string, now time.Time, ledger *healthLedger) RoleVerdict {
	if !ledger.read().newWorld {
		return roleAlive(RoleClaimedGoalBudget, "the bootstrap ledger has no claimed-goal records")
	}
	if ledger.endpointErr != nil {
		return roleUnknown(RoleClaimedGoalBudget, "the claimed-goal ledger endpoint is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	return checkClaimedGoalBudgetsFromProjection(repoRoot, now, ledger.projection, ledger.endpoint.LocalMode(), ledger.projectionErr, nil)
}

func checkClaimedGoalBudgetsFromProjection(repoRoot string, now time.Time, projection goal.Projection, localMode bool, projectionErr error, carryRepository goal.Repository) RoleVerdict {
	if projectionErr != nil {
		if unknown, ok := dispatch.GoalRecordBudgetUnknown(projectionErr); ok {
			role := roleDead(RoleClaimedGoalBudget,
				fmt.Sprintf("BUDGET_UNKNOWN record=%s reason=%s", unknown.Record, unknown.Reason), "", RemedyFact{Cause: CauseBudgetUnknown, Record: unknown.Record})
			role.NoAutomaticRemedy = true
			return role
		}
		if id, malformed := malformedBudgetGoal(projectionErr); malformed {
			role := roleDead(RoleClaimedGoalBudget,
				fmt.Sprintf("claimed goal %s has a malformed structured budget tuple", id), "", RemedyFact{Cause: CauseBudgetMalformed, Goal: id})
			role.NoAutomaticRemedy = true
			return role
		}
		return roleUnknown(RoleClaimedGoalBudget, "the claimed-goal ledger is unreadable", "", RemedyFact{Cause: CauseUnreadable})
	}
	type budgetFailure struct {
		reason    string
		automatic bool
		fact      RemedyFact
	}
	var dead []budgetFailure
	var known []string
	riskUnanswered := 0
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
		if file.Risk == nil {
			riskUnanswered++
		}
		exceptionEvidence := fmt.Sprintf(" exceptions=%d carried=%d", file.BudgetExceptions, goal.LandedCarryCount(file))
		if file.BudgetExceptions >= 2 {
			exceptionEvidence += " repeated exception: defect signal"
		}
		if file.Budget == nil {
			dead = append(dead, budgetFailure{
				reason: fmt.Sprintf("%s BUDGET_MISSING record=plans/goals/%s.md: claimed goal has no structured budget", id, id),
				fact:   RemedyFact{Cause: CauseBudgetMissing, Goal: id, Record: "plans/goals/" + id + ".md"},
			})
			continue
		}
		if file.StopFence != nil {
			batch, batchErr := goal.ReadStopBatch(repoRoot, file.StopFence.StopID)
			if batchErr != nil {
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_INDETERMINATE stop=%s reason=%s",
						id, file.Claimed.Revision, file.StopFence.StopID, batchErr),
					fact: RemedyFact{Cause: CauseBreachStopUnresolved, Goal: id, Stop: file.StopFence.StopID},
				})
				continue
			}
			switch batch.State {
			case goal.StopBatchComplete:
				known = append(known, fmt.Sprintf("%s revision=%d BREACH_STOP_COMPLETE stop=%s terminalJobs=%d foreignJobs=%d%s",
					id, file.Claimed.Revision, batch.StopID, len(batch.Terminal), len(batch.Foreign), stopFiringEvidenceSummary(batch)))
			case goal.StopBatchIndeterminate:
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_INDETERMINATE stop=%s reason=%s%s",
						id, file.Claimed.Revision, batch.StopID, batch.Failure, stopFiringEvidenceSummary(batch)),
					fact: RemedyFact{Cause: CauseBreachStopUnresolved, Goal: id, Stop: batch.StopID},
				})
			default:
				dead = append(dead, budgetFailure{
					reason: fmt.Sprintf("%s revision=%d BREACH_STOP_OPEN stop=%s pendingJobs=%d%s",
						id, file.Claimed.Revision, batch.StopID, len(batch.Pending), stopFiringEvidenceSummary(batch)),

					automatic: true,
					fact:      RemedyFact{Cause: CauseBreachStopOpen, Goal: id, Stop: batch.StopID},
				})
			}
			continue
		}
		budget := dispatch.ProjectBudget(repoRoot, file, now)
		if budget.Status == dispatch.BudgetUnknown {
			dead = append(dead, budgetFailure{
				reason: fmt.Sprintf("%s BUDGET_UNKNOWN record=%s reason=%s",
					id, budget.Unknown.Record, budget.Unknown.Reason),
				fact: RemedyFact{Cause: CauseBudgetUnknown, Goal: id, Record: budget.Unknown.Record},
			})
			continue
		}
		if len(budget.Breaches) > 0 {
			var fields []string
			for _, breach := range budget.Breaches {
				state := ""
				if breach.State != "" {
					state = string(breach.State) + " "
				}
				fields = append(fields, fmt.Sprintf("%s%s used=%s limit=%s", state, breach.Field, breach.Used, breach.Limit))
			}
			dead = append(dead, budgetFailure{
				reason: fmt.Sprintf("%s revision=%d BREACH %s designCritiques=%d/%d codeCritiques=%d/%d", id, budget.GoalRevision, strings.Join(fields, ", "),
					budget.DesignCritiques, budget.Limits.ReviewRoundLimit, budget.CodeCritiques, budget.Limits.ReviewRoundLimit),

				automatic: true,
				fact:      RemedyFact{Cause: CauseBudgetBreach, Goal: id},
			})
			continue
		}
		if budget.ElapsedState == dispatch.AdmissionClosedElapsed {
			known = append(known, fmt.Sprintf("%s revision=%d ADMISSION_CLOSED_ELAPSED elapsed=%s admissionLimit=%s breachLimit=%s gracePercent=%d attempts=%d/%d reservedJobMinutes=%d/%d activeJobs=%d/%d designCritiques=%d/%d codeCritiques=%d/%d%s",
				id, budget.GoalRevision, budget.Elapsed.Round(time.Second), budget.Limits.ElapsedLimit,
				budget.ElapsedBreachLimit, budget.ElapsedGracePercent,
				budget.Attempts, budget.Limits.AttemptLimit,
				budget.ReservedJobMinutes, budget.Limits.ReservedJobMinutesLimit,
				budget.ActiveJobs, budget.Limits.ActiveJobLimit,
				budget.DesignCritiques, budget.Limits.ReviewRoundLimit,
				budget.CodeCritiques, budget.Limits.ReviewRoundLimit, exceptionEvidence))
			continue
		}
		known = append(known, fmt.Sprintf("%s revision=%d attempts=%d/%d reservedJobMinutes=%d/%d activeJobs=%d/%d designCritiques=%d/%d codeCritiques=%d/%d elapsed=%s/%s%s",
			id, budget.GoalRevision, budget.Attempts, budget.Limits.AttemptLimit,
			budget.ReservedJobMinutes, budget.Limits.ReservedJobMinutesLimit,
			budget.ActiveJobs, budget.Limits.ActiveJobLimit,
			budget.DesignCritiques, budget.Limits.ReviewRoundLimit,
			budget.CodeCritiques, budget.Limits.ReviewRoundLimit,
			budget.Elapsed.Round(time.Second), budget.Limits.ElapsedLimit, exceptionEvidence))
	}
	codeTip := "refs/remotes/origin/main"
	if localMode {
		codeTip = "refs/heads/main"
	}
	carryCounts, carryErr := goal.CountCarriesAtEndpoint(goal.Endpoint{Root: repoRoot, Repository: carryRepository}, projection.Tree, codeTip, now)
	if carryErr != nil {
		return roleUnknown(RoleClaimedGoalBudget, "the carried-landing counter is unreadable: "+carryErr.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	known = append([]string{fmt.Sprintf("CARRIED today=%d open=%d inflight=%d debt=%d", carryCounts.Today, carryCounts.Open, carryCounts.Inflight, carryCounts.Debt)}, known...)
	if len(dead) > 0 {
		reasons := make([]string, 0, len(dead))
		noAutomaticRemedy := false
		var facts []RemedyFact
		for _, failure := range dead {
			reasons = append(reasons, failure.reason)
			facts = append(facts, failure.fact)
			if !failure.automatic && !noAutomaticRemedy {
				noAutomaticRemedy = true
				facts[0], facts[len(facts)-1] = facts[len(facts)-1], facts[0]
			}
		}
		role := roleDead(RoleClaimedGoalBudget, fmt.Sprintf("riskUnanswered=%d; %s", riskUnanswered, strings.Join(reasons, "; ")), "", facts...)
		role.NoAutomaticRemedy = noAutomaticRemedy
		return role
	}
	if len(known) == 0 {
		return roleAlive(RoleClaimedGoalBudget, fmt.Sprintf("riskUnanswered=%d; there are no claimed goals", riskUnanswered))
	}
	return roleAlive(RoleClaimedGoalBudget, fmt.Sprintf("riskUnanswered=%d; %s", riskUnanswered, strings.Join(known, "; ")))
}

func checkStopCapabilityEpochWith(repoRoot string, now time.Time, ledger *healthLedger) RoleVerdict {
	if !ledger.read().newWorld {
		return roleAlive(RoleStopCapabilityEpoch, "the goal store has no claimed goals")
	}
	if err := ledger.endpointErr; err != nil {
		return roleUnknown(RoleStopCapabilityEpoch, "the goal store endpoint is unreadable: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	return checkStopCapabilityEpochFromProjection(repoRoot, now, ledger.projection, ledger.projectionErr, goal.ResolveMachine)
}

func checkStopCapabilityEpochFromProjection(repoRoot string, now time.Time, projection goal.Projection, projectionErr error, readMachine func(string) (string, error)) RoleVerdict {
	if projectionErr != nil {
		return roleUnknown(RoleStopCapabilityEpoch, "the goal store is unreadable: "+projectionErr.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	machine, err := readMachine(repoRoot)
	if err != nil {
		return roleUnknown(RoleStopCapabilityEpoch, "the enrolled machine is unreadable: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	var claimed []*goal.GoalFile
	for _, id := range goal.SortedGoalIds(projection.Tree.Live) {
		file := projection.Tree.Live[id]
		if file.State == goal.StateClaimed && file.Claimed != nil && file.Claimed.Machine == machine {
			claimed = append(claimed, file)
		}
	}
	if len(claimed) == 0 {
		return roleAlive(RoleStopCapabilityEpoch, "this machine has no claimed goal")
	}
	holder, found, err := readStopCapabilityHolder(repoRoot)
	if !found && err == nil {
		return roleAlive(RoleStopCapabilityEpoch, "there is no live lease holder to compare")
	}
	if err != nil {
		return roleUnknown(RoleStopCapabilityEpoch, "the checkout lease is unreadable: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	var current []string
	for _, file := range claimed {
		if file.StopCapability == nil {
			role := roleUnknown(RoleStopCapabilityEpoch,
				fmt.Sprintf("claimed goal %s has no stop capability to compare", file.Id), "", RemedyFact{Cause: CauseStopCapabilityMissing, Goal: file.Id, Record: "plans/goals/" + file.Id + ".md"})
			return role
		}
		capabilityEpoch := file.StopCapability.ClaimEpoch
		if capabilityEpoch == holder.ClaimEpoch {
			current = append(current, fmt.Sprintf("%s epoch=%d", file.Id, capabilityEpoch))
			continue
		}
		if file.Claimed.Lineage == holder.OwnerLineage {
			role := roleDead(RoleStopCapabilityEpoch,
				fmt.Sprintf("goal %s stop capability claim epoch %d differs from live lease claim epoch %d under owner lineage %s",
					file.Id, capabilityEpoch, holder.ClaimEpoch, holder.OwnerLineage), "", RemedyFact{Cause: CauseEpochMismatch, Goal: file.Id})
			return role
		}
		role := roleDead(RoleStopCapabilityEpoch,
			fmt.Sprintf("goal %s was claimed under owner lineage %s but the live lease belongs to owner lineage %s",
				file.Id, file.Claimed.Lineage, holder.OwnerLineage), "", RemedyFact{Cause: CauseForeignLineage, Goal: file.Id})
		role.NoAutomaticRemedy = true
		return role
	}
	return roleAlive(RoleStopCapabilityEpoch, "stop capability epochs match the live lease: "+strings.Join(current, ", "))
}

type stopCapabilityHolder struct {
	HolderMainID string `json:"holderMainId"`
	OwnerLineage string `json:"ownerLineage"`
	Pid          int64  `json:"pid"`
	Revision     int64  `json:"revision"`
	ClaimEpoch   int64  `json:"claimEpoch"`
}

func readStopCapabilityHolder(repoRoot string) (stopCapabilityHolder, bool, error) {
	path := filepath.Join(repoRoot, "artifacts", "agents", "mains", "worktree-lease.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return stopCapabilityHolder{}, false, nil
	}
	if err != nil {
		return stopCapabilityHolder{}, false, err
	}
	var holder stopCapabilityHolder
	if err := json.Unmarshal(data, &holder); err != nil || holder.HolderMainID == "" ||
		holder.Pid < 1 || holder.Revision < 1 || holder.ClaimEpoch < 1 {
		return stopCapabilityHolder{}, false, fmt.Errorf("checkout lease schema is invalid")
	}
	if holder.OwnerLineage == "" {
		holder.OwnerLineage = holder.HolderMainID
	}
	return holder, true, nil
}

func stopFiringEvidenceSummary(batch goal.StopBatch) string {
	if batch.Reason != goal.StopReasonElapsedLimit {
		return ""
	}
	if batch.FiringEvidence == nil {
		return " firingEvidence=legacy-unavailable"
	}
	evidence := batch.FiringEvidence
	return fmt.Sprintf(" ELAPSED_BREACH used=%s limit=%s grace=%d admissionLimit=%s",
		evidence.ElapsedUsed, evidence.BreachBoundary, evidence.GracePercent, evidence.AdmissionLimit)
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
	if len(dead) > 0 {
		verdict := roleDead(RoleNonterminalJobs, "non-terminal jobs with dead recorded processes: "+strings.Join(dead, ","), "", RemedyFact{Cause: CauseJobProcessDead, Job: dead[0]})
		return verdict
	}
	if len(unknown) > 0 {
		return roleUnknown(RoleNonterminalJobs, "non-terminal jobs with unreadable process evidence: "+strings.Join(unknown, ","), "", RemedyFact{Cause: CauseUnreadable})
	}
	return roleAlive(RoleNonterminalJobs, "no non-terminal job has a provably dead recorded process")
}

func checkCapabilitySnapshots(repoRoot, metasystemRoot string, now time.Time) RoleVerdict {
	role, _ := capabilitySnapshotStatus(repoRoot, metasystemRoot, now, exec.LookPath)
	return role
}

func capabilitySnapshotStatus(repoRoot, metasystemRoot string, now time.Time, lookPath func(string) (string, error)) (RoleVerdict, []string) {
	runtimeValue, _, err := config.Get(config.GetParams{
		Key: "metasystem.runtimes", ConfPath: filepath.Join(metasystemRoot, "metasystem.conf"),
	})
	if err != nil {
		return roleUnknown(RoleCapabilitySnapshots, "metasystem.runtimes is unreadable", "", RemedyFact{Cause: CauseSettingsInvalid}), nil
	}
	if runtimeValue == "none" {
		return roleAlive(RoleCapabilitySnapshots, "no runtime capability snapshots are configured"), nil
	}
	maxAgeDays, err := nonnegativeConfig(metasystemRoot, "capability.snapshot-max-age-days", config.MustIntDefault("capability.snapshot-max-age-days"))
	if err != nil {
		return roleUnknown(RoleCapabilitySnapshots, "capability.snapshot-max-age-days is unreadable", "", RemedyFact{Cause: CauseSettingsInvalid}), nil
	}
	runtimes := strings.Split(runtimeValue, ",")
	paths, _ := filepath.Glob(filepath.Join(repoRoot, "artifacts", "agents", "capabilities", "*.json"))
	var dead []string
	var unknown []string
	seen := make(map[string]bool)
	for _, runtimeName := range runtimes {
		runtimeName = strings.TrimSpace(runtimeName)
		if seen[runtimeName] {
			continue
		}
		seen[runtimeName] = true
		if runtimeName == "" {
			unknown = append(unknown, "empty-runtime")
			continue
		}
		declaration, supported := runtimereg.Lookup(runtimeName)
		if !supported || !declaration.HasAdapter {
			unknown = append(unknown, runtimeName+":NO_ADAPTER")
			continue
		}
		if declaration.Executable != "" {
			if _, err := lookPath(declaration.Executable); err != nil {
				continue
			}
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
		if probe, _, err := loadComponentEvidenceForHealth(repoRoot, "capability-probe-"+runtimeName); err == nil &&
			probe.Result == ComponentError && !newest.After(probe.LastCompletion) {
			dead = append(dead, runtimeName)
			continue
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
	if len(dead) > 0 {
		reason := "missing or stale capability snapshots: " + strings.Join(dead, ",")
		for _, runtime := range dead {
			reason = healthRemedyReason(repoRoot, "capability-probe-"+runtime, reason)
		}
		return roleDead(RoleCapabilitySnapshots, reason, "", RemedyFact{Cause: CauseUnavailable}), dead
	}
	if len(unknown) > 0 {
		fact := RemedyFact{Cause: CauseUnreadable}
		for _, name := range unknown {
			if strings.HasSuffix(name, ":CLOCK_REGRESSED") {
				fact.Cause = CauseClockRegressed
			}
			if name == "empty-runtime" || strings.HasSuffix(name, ":NO_ADAPTER") {
				fact.Cause = CauseSettingsInvalid
				break
			}
		}
		reason := "capability snapshot ages are unreadable: " + strings.Join(unknown, ",")
		if fact.Cause == CauseSettingsInvalid {
			reason += "; correct metasystem.runtimes"
		}
		return roleUnknown(RoleCapabilitySnapshots, reason, "", fact), nil
	}
	return roleAlive(RoleCapabilitySnapshots, "runtimes on PATH have fresh capability snapshots"), nil
}

func healthRemedyReason(root, component, reason string) string {
	if record, _, err := loadComponentEvidenceForHealth(root, component); err == nil && record.Result == ComponentError {
		return reason + "; " + component + ": " + record.LastFailure
	}
	return reason
}

func saveRemediedHealth(root string, verdict HealthVerdict) error {
	held, err := lock.File(healthLockPath(root), 0o644, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	record, err := loadHealthRecord(HealthRecordPath(root))
	if err != nil {
		return err
	}
	// A later observation already owns the cached verdict.
	if record.State.Sequence != verdict.Observation {
		return nil
	}
	record.Verdict = verdict
	return saveHealthRecord(root, HealthRecordPath(root), record)
}

func roleAlive(role HealthRole, reason string) RoleVerdict {
	return RoleVerdict{Role: role, Status: HealthAlive, Reason: reason}
}

func roleDead(role HealthRole, reason, diagnostic string, facts ...RemedyFact) RoleVerdict {
	verdict := roleUnknown(role, reason, diagnostic, facts...)
	verdict.Status = HealthDead
	return verdict
}

func roleUnknown(role HealthRole, reason, diagnostic string, facts ...RemedyFact) RoleVerdict {
	if len(facts) == 0 {
		facts = []RemedyFact{{Cause: CauseUnavailable}}
	}
	act, plain := remedyFor(role, facts[0]).Render("human", nil)
	return RoleVerdict{Role: role, Status: HealthUnknown, Reason: reason, RemedyFacts: facts, Remedy: strings.TrimSpace(strings.Join(act, " ") + " " + plain)}
}

func installedGeneration(repoRoot string) (int, error) {
	installed, _, err := installedEnrollment(repoRoot)
	return installed.Generation, err
}

func installedEnrollment(repoRoot string) (InstallIdentity, bool, error) {
	absolute, err := filepath.Abs(repoRoot)
	if err != nil {
		return InstallIdentity{}, false, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	installed, err := VerifyIdentity(RepoIdentityPath(repoRoot), absolute)
	if err != nil {
		return InstallIdentity{}, false, err
	}
	_, markerErr := os.Stat(identityDurabilityPendingPath(RepoIdentityPath(repoRoot)))
	if markerErr != nil && !os.IsNotExist(markerErr) {
		return InstallIdentity{}, false, markerErr
	}
	return installed, markerErr == nil, nil
}

// checkProofAttempts is the steward incident for a hung proof: a live
// attempt whose launcher is provably dead, or whose every recorded process
// has ended, holds a reservation nobody can release but the reaper's next
// reconciliation pass. The role reads what the reconciler reads.
func checkProofAttempts(repoRoot string, prober identity.Prober) RoleVerdict {
	root := filepath.Join(repoRoot, "metasystem")
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "proof-runs")); err != nil {
		root = repoRoot
	}
	paths, _ := readRunnerRecords(repoRoot, filepath.Join(root, "artifacts", "agents", "proof-runs", "attempts"), func(path string) (proofAttemptHead, error) {
		value, terminal, err := readProofAttemptHead(path)
		return proofAttemptHead{value, terminal}, err
	})
	var dead, unknown []string
	for _, path := range paths {
		if path.err != nil {
			unknown = append(unknown, strings.TrimSuffix(path.name, ".json"))
			continue
		}
		value := path.value.value
		if path.value.terminal {
			continue
		}
		attemptID, _ := value["attemptId"].(string)
		if attemptID == "" {
			attemptID = strings.TrimSuffix(path.name, ".json")
		}
		launcher, ok := value["launcher"].(map[string]any)
		if !ok {
			unknown = append(unknown, attemptID)
			continue
		}
		ref, ok := processRef(launcher)
		if !ok {
			unknown = append(unknown, attemptID)
			continue
		}
		switch identity.AliveRef(prober, ref) {
		case identity.Dead:
			dead = append(dead, fmt.Sprintf("%s (launcher pid %d)", attemptID, ref.Pid))
		case identity.Unknown:
			unknown = append(unknown, attemptID)
		}
	}
	if len(dead) > 0 {
		return roleDead(RoleProofAttempts, "live proof attempts whose launcher is dead: "+strings.Join(dead, ","), "", RemedyFact{Cause: CauseUnavailable})
	}
	if len(unknown) > 0 {
		return roleUnknown(RoleProofAttempts, "live proof attempts with unreadable launcher evidence: "+strings.Join(unknown, ","), "", RemedyFact{Cause: CauseUnreadable})
	}
	return roleAlive(RoleProofAttempts, "no live proof attempt has a dead launcher")
}

type proofAttemptHead struct {
	value    map[string]any
	terminal bool
}

// proofAdmissionRedKey bounds how long a heavy proof lease may stay dead or
// unknown before the role turns red; a dead one is normally reclaimed by the
// check itself or by the next admission pass well inside it.
const proofAdmissionRedKey = "steward.proof-admission-red-min"

const defaultProofAdmissionRedMinutes = 10

// inspectHostLeases is the host's heavy lease census. It takes a provably
// dead, settled lease through the admission reclaim path when admission.lock
// is free. Package tests replace it: the admission directory is host-wide.
var inspectHostLeases = proofrun.InspectHostLeases

// checkProofAdmission reports every dirty heavy proof-admission lease with
// its owner, age and state, and turns red when one has been dead or unknown
// for longer than steward.proof-admission-red-min.
func checkProofAdmission(repoRoot string, now time.Time, inspect func(string) ([]proofrun.HostLeaseReport, error)) RoleVerdict {
	minutes, err := boundedConfig(repoRoot, proofAdmissionRedKey, defaultProofAdmissionRedMinutes, 1)
	if err != nil {
		return roleUnknown(RoleProofAdmission, err.Error(), "", RemedyFact{Cause: CauseSettingsInvalid})
	}
	threshold := time.Duration(minutes) * time.Minute
	reports, err := inspect(repoRoot)
	if err != nil {
		return roleUnknown(RoleProofAdmission, "heavy proof leases are unreadable: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	var lines, red, remedies []string
	for _, report := range reports {
		age := now.Sub(report.Since).Round(time.Second)
		line := fmt.Sprintf("%s owner pid %d group %d age %s %s: %s", report.Lease, report.Owner.Pid, report.Owner.Pgid, age, report.State, report.Reason)
		lines = append(lines, line)
		if report.Remedy != "" {
			remedies = append(remedies, report.Lease+": "+report.Remedy)
		}
		if (report.State == proofrun.HostLeaseDead || report.State == proofrun.HostLeaseUnknown) && age > threshold {
			red = append(red, line)
		}
	}
	remedy := strings.Join(remedies, "; ")
	if len(red) > 0 {
		return roleDead(RoleProofAdmission, fmt.Sprintf("heavy proof leases dead or unknown for over %d min: %s", minutes, strings.Join(red, "; ")), "", RemedyFact{Cause: CauseUnavailable, Command: remedy})
	}
	if len(lines) == 0 {
		return roleAlive(RoleProofAdmission, "no dirty heavy proof lease")
	}
	verdict := roleAlive(RoleProofAdmission, "heavy proof leases: "+strings.Join(lines, "; "))
	if remedy != "" {
		verdict.RemedyFacts = []RemedyFact{{Cause: CauseUnavailable, Command: remedy}}
		_, verdict.Remedy = verdict.PublicRemedy("human", nil)
	}
	return verdict
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

// readProofAttemptHead reads a proof attempt record only as far as the
// proof-attempts role needs. A terminal attempt is decided at its "terminal"
// key: the payload after it (the delivery receipt and test results, up to
// 25 MB a record, 518 MB across seat m1e's 268 records on 2026-09-27) is
// never parsed, because a finished attempt has no launcher to judge. That
// full parse cost the Stop hook about three seconds of CPU per turn. A live
// attempt is read whole and must be exactly one JSON object, as before.
func readProofAttemptHead(path string) (map[string]any, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.UseNumber()
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		if err == nil {
			err = fmt.Errorf("not a JSON object")
		}
		return nil, false, err
	}
	value := map[string]any{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, false, err
		}
		key, _ := token.(string)
		var field any
		if err := decoder.Decode(&field); err != nil {
			return nil, false, err
		}
		if key == "terminal" && field != nil {
			return nil, true, nil
		}
		value[key] = field
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, false, err
	}
	return value, false, nil
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
		if role.Status != HealthAlive && !role.Standing {
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
	// A retired check cannot invalidate the observation clock or the other
	// roles' breaker counts kept by an installed steward.
	delete(record.State.UnknownCounts, RoleRetroDebt)
	delete(record.State.FailureCounts, RoleRetroDebt)
	delete(record.State.FailureCauses, RoleRetroDebt)
	delete(record.State.FailureEpisodes, RoleRetroDebt)
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
	for role := range record.State.FailureCauses {
		if !validRoles[role] {
			return healthRecord{}, fmt.Errorf("health observation record has an invalid failure cause role")
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
