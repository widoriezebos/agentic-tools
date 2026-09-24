package proofrun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	// TestWorkerProtocolVersion is the request/result negotiation protocol
	// implemented by the worker-capabilities command.
	TestWorkerProtocolVersion = 1
	// TestWorkerPolicyVersion identifies the worker-allocation inputs bound
	// into prepared execution identities and retained results.
	TestWorkerPolicyVersion = 1
	// TestWorkersEnvironment is the reserved adapter allowance exported to
	// native commands. Contract environment declarations cannot replace it.
	TestWorkersEnvironment        = testpolicy.TestWorkersEnvironment
	proofExecutionRootEnvironment = "METASYSTEM_PROOF_EXECUTION_ROOT"
)

type TestRunRequest struct {
	ProjectRoot        string
	openCandidate      func(projectRoot, candidateTree string) (candidateWorkspace, error)
	EngineRearm        *EngineRearm
	InstallationPrefix string
	CandidateTree      string
	BaseCommit         string
	PolicyBaseCommit   string
	Contract           testpolicy.Contract
	Plan               testpolicy.Plan
	AttemptID          string
	Environment        []string
	LogRoot            string
	ProgressPath       string
	Reused             map[string]GroupResult
	ContractDigest     string
	BaseContractDigest string
	PolicyEngineDigest string
	// JudgeKey names the judge that reads the candidate (ComputeJudgeKey); it
	// binds group identities where the policy engine's file digest used to.
	JudgeKey                     string
	PolicyEngine                 string
	CandidateEngine              string
	CandidateEngineDigest        string
	CandidateEngineBuildIdentity string
	ControlRoot                  string
	BehaviorPolicyDigest         string
	ComponentIdentities          map[string]string
	FreshnessEpisode             string          `json:",omitempty"`
	FreshnessBinding             string          `json:",omitempty"`
	FreshnessExpiresAt           string          `json:",omitempty"`
	FreshGroups                  map[string]bool `json:",omitempty"`
	FreshnessCandidateProjection string          `json:",omitempty"`
	SyntheticProbe               bool            `json:"-"`
	PreparedGroups               map[string]PreparedGroupExecution
	CommandStartedAt             string
	PreparationLaunches          int
	PreparationDurationMS        int64
	QueueDurationMS              int64
	EvidenceTimeoutMS            int64
	EvidenceMaxBytes             int64
	// Workers is the positive adapter-worker allowance for this attempt.
	// A zero value is accepted only for legacy direct callers and resolves to
	// one; production preparation always writes a positive value.
	Workers int `json:",omitempty"`
	// AdmissionMaximum reports the separately resolved top-level proof cap.
	// Zero retains its existing meaning: host admission is unlimited.
	AdmissionMaximum int `json:",omitempty"`
	// Concurrency bounds how many groups of one stage run at once; zero or
	// one runs them in plan order, one after another.
	Concurrency int
	// AllGroups keeps a delivery run collecting after a failed group. Other
	// purposes already collect every selected group.
	AllGroups   bool `json:",omitempty"`
	loadOptions []loadSampleOption
	now         func() time.Time
}

// CandidateWorkspace is the detached candidate bed used by a test request.
type CandidateWorkspace interface {
	Workspace() gittree.Workspace
	Close() error
}

type candidateWorkspace = CandidateWorkspace

// WithCandidateOpener sets the detached candidate bed for this request.
// A nil opener keeps the native worktree behavior.
func (request *TestRunRequest) WithCandidateOpener(open func(projectRoot, candidateTree string) (CandidateWorkspace, error)) {
	request.openCandidate = open
}

func (request TestRunRequest) candidateWorkspace() (candidateWorkspace, error) {
	if request.openCandidate != nil {
		return request.openCandidate(request.ProjectRoot, request.CandidateTree)
	}
	return (gittree.Workspace{Dir: request.ProjectRoot}).NewDetachedWorktree(request.CandidateTree)
}

// PreparedGroupExecution is immutable metadata collected once before
// admission and carried into the authenticated worker. Executable bytes are
// re-hashed there without repeating version helpers.
type PreparedGroupExecution struct {
	WorkerPolicyVersion int                  `json:"workerPolicyVersion,omitempty"`
	Workers             int                  `json:"workers,omitempty"`
	InputDigest         string               `json:"inputDigest"`
	EnvironmentDigest   string               `json:"environmentDigest"`
	ToolIdentities      map[string]string    `json:"toolIdentities"`
	ExecutableDigests   map[string]string    `json:"executableDigests"`
	Argv                []string             `json:"argv"`
	ImplicitInputs      []string             `json:"implicitInputs,omitempty"`
	CoverageInventory   []string             `json:"coverageInventory,omitempty"`
	CoverageModule      string               `json:"coverageModule,omitempty"`
	Expected            []NativeTestIdentity `json:"expected"`
	Unavailable         string               `json:"unavailable,omitempty"`
}

// EffectiveTestWorkers resolves the conservative legacy request value.
func EffectiveTestWorkers(request TestRunRequest) int {
	if request.Workers > 0 {
		return request.Workers
	}
	return 1
}

// TestWorkerPolicyActive distinguishes a negotiated request from the
// schema-less request sent by an older frontend. Zero remains the conservative
// direct-caller allowance, but it does not claim the negotiated protocol.
func TestWorkerPolicyActive(request TestRunRequest) bool {
	return request.Workers > 0
}

func groupExecutionIdentityVersion(request TestRunRequest) int {
	if TestWorkerPolicyActive(request) {
		return GroupExecutionIdentityVersion
	}
	return PreviousGroupExecutionIdentityVersion
}

// EffectiveGroupWorkers resolves a command or section declaration against
// the authenticated attempt allowance. Go partitions each consume one worker.
func EffectiveGroupWorkers(request TestRunRequest, group testpolicy.Group) (int, error) {
	attemptWorkers := EffectiveTestWorkers(request)
	workers := 1
	if group.Kind == "performance" {
		workers = attemptWorkers
	} else if group.Adapter != "go" && group.Resources.Workers != nil {
		workers = *group.Resources.Workers
		if workers == 0 {
			workers = attemptWorkers
		}
	}
	if workers < 1 || workers > attemptWorkers {
		return 0, fmt.Errorf("testing group %s requests %d workers from attempt allowance %d", group.ID, workers, attemptWorkers)
	}
	return workers, nil
}

// ValidateTestWorkerRequest refuses malformed authenticated policy before any
// selected native command can start.
func ValidateTestWorkerRequest(request TestRunRequest) error {
	if request.Workers < 0 {
		return fmt.Errorf("testing attempt workers must be positive")
	}
	if request.AdmissionMaximum < 0 {
		return fmt.Errorf("testing admission maximum cannot be negative")
	}
	if !TestWorkerPolicyActive(request) && request.AdmissionMaximum != 0 {
		return fmt.Errorf("testing admission maximum requires negotiated worker policy")
	}
	selected := make(map[string]bool, len(request.Plan.SelectedGroups))
	for _, id := range request.Plan.SelectedGroups {
		selected[id] = true
	}
	for _, group := range request.Contract.Groups {
		if len(selected) != 0 && !selected[group.ID] {
			continue
		}
		if _, reserved := group.Env[TestWorkersEnvironment]; reserved {
			return fmt.Errorf("testing group %s cannot set reserved environment %s", group.ID, TestWorkersEnvironment)
		}
		if !TestWorkerPolicyActive(request) && group.Resources.Workers != nil {
			return fmt.Errorf("testing group %s worker declaration requires negotiated worker policy", group.ID)
		}
		workers, err := EffectiveGroupWorkers(request, group)
		if err != nil {
			return err
		}
		if prepared, ok := request.PreparedGroups[group.ID]; ok {
			if TestWorkerPolicyActive(request) &&
				(prepared.WorkerPolicyVersion != TestWorkerPolicyVersion || prepared.Workers != workers) {
				return fmt.Errorf("testing group %s prepared worker policy differs from request", group.ID)
			}
			if !TestWorkerPolicyActive(request) && (prepared.WorkerPolicyVersion != 0 || prepared.Workers != 0) {
				return fmt.Errorf("testing group %s prepared metadata claims unnegotiated worker policy", group.ID)
			}
		}
	}
	return nil
}

func testRequestNow(request TestRunRequest) time.Time {
	if request.now != nil {
		return request.now().UTC()
	}
	return time.Now().UTC()
}

func testRequestClock(request TestRunRequest) (func() time.Time, bool, error) {
	if request.ControlRoot == "" {
		return func() time.Time { return time.Now().UTC() }, false, nil
	}
	authorization, err := fixtureauth.New(request.ControlRoot)
	if err != nil {
		return nil, false, err
	}
	if fixtureNow, ok, err := authorization.Clock().GoalNow(); err != nil {
		return nil, false, err
	} else if ok {
		return func() time.Time { return fixtureNow }, true, nil
	}
	return func() time.Time { return time.Now().UTC() }, false, nil
}

func RunTestPlan(ctx context.Context, request TestRunRequest) (TestResult, int, error) {
	ctx = withTestWorkerPool(ctx, EffectiveTestWorkers(request))
	clock, _, err := testRequestClock(request)
	if err != nil {
		return TestResult{}, 1, fmt.Errorf("testing semantic clock: %w", err)
	}
	request.now = clock
	workerStarted := time.Now().UTC()
	started := workerStarted
	if parsed, parseErr := time.Parse(time.RFC3339Nano, request.CommandStartedAt); parseErr == nil && !parsed.After(workerStarted) {
		started = parsed
	}
	result := NewTestResult(request)
	if err := ValidateTestWorkerRequest(request); err != nil {
		return result, 1, err
	}
	var admitted Attempt
	if request.ControlRoot != "" && request.AttemptID != "" {
		stored, err := ReadAttempt(request.ControlRoot, request.AttemptID)
		if err != nil && !os.IsNotExist(err) {
			return result, 1, err
		}
		if err == nil && len(stored.TestInventory) != 0 && !request.SyntheticProbe {
			if stored.CandidateTree != request.CandidateTree || stored.ExecutionRoot != request.ProjectRoot {
				return result, 1, fmt.Errorf("testing worker tree or root differs from admitted attempt")
			}
			if stored.FreshnessEpisode != request.FreshnessEpisode || stored.FreshnessBinding != request.FreshnessBinding ||
				stored.FreshnessExpiresAt != request.FreshnessExpiresAt {
				return result, 1, fmt.Errorf("testing worker freshness differs from admitted attempt")
			}
			if len(stored.TestFreshGroups) != len(request.FreshGroups) {
				return result, 1, fmt.Errorf("testing worker fresh groups differ from admitted attempt")
			}
			for id := range stored.TestFreshGroups {
				if !request.FreshGroups[id] {
					return result, 1, fmt.Errorf("testing worker fresh group %s differs from admitted attempt", id)
				}
			}
			admitted = stored
		}
	}
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	if request.Contract.SchemaVersion == testpolicy.ExecutionContractSchemaVersion {
		return runExecutionContractPlan(ctx, request, result, groups, admitted, started, workerStarted)
	}
	firstStatus := 0
	finalizeOperationalError := func(runErr error) (TestResult, int, error) {
		if len(result.Groups) == 0 {
			return result, 1, runErr
		}
		return finalizeTestPlanResult(request, result, groups, started, workerStarted, runErr), 1, runErr
	}
	progress := &progressWriter{path: request.ProgressPath}
	// Delivery normally stops launching at the first failure and records the
	// rest as not run (R-96-m1e). A caller collecting a complete failure set
	// opts into all groups; cadence and diagnostic already continue because
	// they exist to see every failure (R-16).
	stopAtFirstFailure := request.Plan.Purpose == testpolicy.PurposeDelivery && !request.AllGroups
	haltedBy := ""
	for _, stage := range request.Plan.Stages {
		var runnable []string
		for _, id := range stage.Groups {
			if haltedBy != "" {
				result.Groups = append(result.Groups, unlaunchedGroupResult(request, groups[id], haltReason(haltedBy)))
				continue
			}
			if len(admitted.TestInventory) != 0 {
				if admitted.TestInventory[id] != request.ComponentIdentities[id] {
					return finalizeOperationalError(fmt.Errorf("testing worker group %s differs from admitted inventory", id))
				}
				if admitted.TestWaits[id] != "" {
					borrowed, waitErr := WaitForTestProducer(ctx, request.ControlRoot, admitted, id)
					if waitErr != nil {
						return finalizeOperationalError(waitErr)
					}
					result.Groups = append(result.Groups, borrowed)
					if borrowed.Status != "reused" {
						if firstStatus == 0 {
							firstStatus = 1
						}
						if stopAtFirstFailure && haltedBy == "" {
							haltedBy = id
						}
					} else if groups[id].Kind == "build" {
						result.LaunchCounts.ReusedBuild++
					} else {
						result.LaunchCounts.ReusedTest++
					}
					continue
				}
				if admitted.TestOwned[id] != "" {
					runnable = append(runnable, id)
					continue
				}
				if source := admitted.TestSources[id]; source != "" {
					original, sourceErr := nativeSourceGroup(admitted, GroupResult{ID: id, ExecutionIdentity: admitted.TestInventory[id], ReuseAttempt: source}, testRequestNow(request))
					if sourceErr != nil || original.Status != "passed" || !original.CollectionComplete {
						return finalizeOperationalError(fmt.Errorf("admitted source for %s is incomplete: %v", id, sourceErr))
					}
					original.Status, original.NativeLaunched, original.ReuseAttempt = "reused", false, source
					original.CoveredByGroups, original.CoveredTests = nil, nil
					result.Groups = append(result.Groups, original)
					if groups[id].Kind == "build" {
						result.LaunchCounts.ReusedBuild++
					} else {
						result.LaunchCounts.ReusedTest++
					}
					continue
				}
			}
			if reused, ok := request.Reused[id]; ok {
				if err := validateRetainedGroupReuse(request.ControlRoot, id, reused); err != nil {
					reused.Status, reused.CollectionComplete, reused.NativeLaunched = "invalid", false, false
					reused.IdentityVersion = groupExecutionIdentityVersion(request)
					reused.NotRunReason, reused.ReuseAttempt = "forged or stale component reuse: "+err.Error(), ""
					result.Groups = append(result.Groups, reused)
					if firstStatus == 0 {
						firstStatus = 1
					}
					if stopAtFirstFailure && haltedBy == "" {
						haltedBy = id
					}
					continue
				}
				reused.Status = "reused"
				result.Groups = append(result.Groups, reused)
				switch groups[id].Kind {
				case "build":
					result.LaunchCounts.ReusedBuild++
				default:
					result.LaunchCounts.ReusedTest++
				}
				continue
			}
			if len(admitted.TestInventory) != 0 {
				return finalizeOperationalError(fmt.Errorf("testing group %s has no retained source for its admitted inventory", id))
			}
			runnable = append(runnable, id)
		}
		// A halt closes the gate for the whole stage, whether it came from an
		// earlier stage or from a stale reuse judged a moment ago in this one.
		if haltedBy != "" {
			for _, id := range runnable {
				result.Groups = append(result.Groups, unlaunchedGroupResult(request, groups[id], haltReason(haltedBy)))
			}
			runnable = nil
		}
		// Independent groups share the stage's bounded pool, while performance
		// groups run alone after them; results stay in plan order and stages stay
		// canary, standard, deep.
		stageResults, stageHaltedBy, progressErr := runStageGroups(ctx, request, groups, runnable, progress, stopAtFirstFailure)
		if haltedBy == "" {
			haltedBy = stageHaltedBy
		}
		for _, groupResult := range stageResults {
			result.Groups = append(result.Groups, groupResult)
			result.ChildDurationMS += groupResult.DurationMS
			result.LaunchCounts.Other += groupResult.OtherLaunches
			if groupResult.NativeLaunched {
				switch groups[groupResult.ID].Kind {
				case "build":
					result.LaunchCounts.Build++
				default:
					result.LaunchCounts.Test++
				}
			}
			if groupResult.Status != "passed" && groupResult.Status != "reused" {
				if firstStatus == 0 {
					if groupResult.NativeExitStatus != nil && *groupResult.NativeExitStatus != 0 {
						firstStatus = *groupResult.NativeExitStatus
					} else {
						firstStatus = 1
					}
				}
			}
		}
		if progressErr != nil {
			return finalizeTestPlanResult(request, result, groups, started, workerStarted, progressErr), 1, progressErr
		}
	}
	result = finalizeTestPlanResult(request, result, groups, started, workerStarted, nil)
	if request.Plan.Purpose == testpolicy.PurposeDelivery && !result.Delivery.Sufficient && firstStatus == 0 {
		firstStatus = 1
	}
	return result, firstStatus, nil
}

func runExecutionContractPlan(ctx context.Context, request TestRunRequest, result TestResult, groups map[string]testpolicy.Group, admitted Attempt, started, workerStarted time.Time) (TestResult, int, error) {
	ctx = withTestWorkerPool(ctx, EffectiveTestWorkers(request))
	progress := &progressWriter{path: request.ProgressPath}
	completed := map[string]GroupResult{}
	firstStatus := 0
	add := func(groupResult GroupResult) {
		result.Groups = append(result.Groups, groupResult)
		completed[groupResult.ID] = groupResult
		result.ChildDurationMS += groupResult.DurationMS
		result.LaunchCounts.Other += groupResult.OtherLaunches
		if groupResult.NativeLaunched {
			if groups[groupResult.ID].Kind == "build" {
				result.LaunchCounts.Build++
			} else {
				result.LaunchCounts.Test++
			}
		} else if groupResult.Status == "reused" {
			if groups[groupResult.ID].Kind == "build" {
				result.LaunchCounts.ReusedBuild++
			} else {
				result.LaunchCounts.ReusedTest++
			}
		}
		if groupResult.Status != "passed" && groupResult.Status != "reused" && firstStatus == 0 {
			if groupResult.NativeExitStatus != nil && *groupResult.NativeExitStatus != 0 {
				firstStatus = *groupResult.NativeExitStatus
			} else {
				firstStatus = 1
			}
		}
	}
	resolve := func(ctx context.Context, id string) (GroupResult, bool, error) {
		if len(admitted.TestInventory) != 0 {
			if admitted.TestInventory[id] != request.ComponentIdentities[id] {
				return GroupResult{}, false, fmt.Errorf("testing worker group %s differs from admitted inventory", id)
			}
			if admitted.TestWaits[id] != "" {
				borrowed, err := WaitForTestProducer(ctx, request.ControlRoot, admitted, id)
				return borrowed, err == nil, err
			}
			if source := admitted.TestSources[id]; source != "" {
				original, err := nativeSourceGroup(admitted, GroupResult{ID: id,
					ExecutionIdentity: admitted.TestInventory[id], ReuseAttempt: source}, testRequestNow(request))
				if err != nil || original.Status != "passed" || !original.CollectionComplete {
					return GroupResult{}, false, fmt.Errorf("admitted source for %s is incomplete: %v", id, err)
				}
				original.Status, original.NativeLaunched, original.ReuseAttempt = "reused", false, source
				original.CoveredByGroups, original.CoveredTests = nil, nil
				return original, true, nil
			}
			if admitted.TestOwned[id] == "" {
				return GroupResult{}, false, fmt.Errorf("testing group %s has no admitted execution owner", id)
			}
			return GroupResult{}, false, nil
		}
		if reused, ok := request.Reused[id]; ok {
			if err := validateRetainedGroupReuse(request.ControlRoot, id, reused); err != nil {
				reused.Status, reused.CollectionComplete, reused.NativeLaunched = "invalid", false, false
				reused.NotRunReason, reused.ReuseAttempt = "forged or stale component reuse: "+err.Error(), ""
			} else {
				reused.Status = "reused"
			}
			return reused, true, nil
		}
		return GroupResult{}, false, nil
	}
	for _, stage := range request.Plan.Stages {
		stageResults, _, err := runStageGroupsWithPrerequisites(ctx, request, groups, stage.Groups, progress, completed, resolve)
		for _, groupResult := range stageResults {
			if groupResult.ID != "" {
				add(groupResult)
			}
		}
		if err != nil {
			return finalizeTestPlanResult(request, result, groups, started, workerStarted, err), 1, err
		}
	}
	result = finalizeTestPlanResult(request, result, groups, started, workerStarted, nil)
	if request.Plan.Purpose == testpolicy.PurposeDelivery && !result.Delivery.Sufficient && firstStatus == 0 {
		firstStatus = 1
	}
	return result, firstStatus, nil
}

// finalizeTestPlanResult is the one projection from a completed or
// operationally interrupted plan into a result that can cross the worker
// boundary. An interruption records every later selected group as not run;
// it never invents a launch or replaces the outcomes already drained from
// the scheduler.
func finalizeTestPlanResult(request TestRunRequest, result TestResult, groups map[string]testpolicy.Group, started, workerStarted time.Time, runErr error) TestResult {
	if runErr != nil {
		seen := make(map[string]bool, len(result.Groups))
		for _, groupResult := range result.Groups {
			seen[groupResult.ID] = true
		}
		reason := "testing plan stopped after operational error: " + runErr.Error()
		result.Uncertainty = append(result.Uncertainty, reason)
		for _, id := range request.Plan.SelectedGroups {
			group, exists := groups[id]
			if seen[id] || !exists {
				continue
			}
			result.Groups = append(result.Groups, unlaunchedGroupResult(request, group, reason))
			seen[id] = true
		}
	}
	result.EndedAt, result.DurationMS = resultDuration(request, started)
	result.Cost.ActualDurationMS, result.Cost.ChildDurationMS = result.DurationMS, result.ChildDurationMS
	result.Cost.ExecutionDurationMS = time.Since(workerStarted).Milliseconds()
	result.Cost.ReusedLaunches = result.LaunchCounts.ReusedTest + result.LaunchCounts.ReusedBuild + result.LaunchCounts.ReusedOther
	result.StoppedAtFirstFailure = stoppedAtFirstFailure(result.Groups)
	result.RecomputeDelivery()
	return result
}

func stoppedAtFirstFailure(groups []GroupResult) bool {
	for _, group := range groups {
		if group.Status == "not-run" && strings.HasPrefix(group.NotRunReason, "delivery attempt stopped at the first failed group ") {
			return true
		}
	}
	return false
}

// haltReason is the not-run reason of every group a delivery attempt left
// unlaunched after its first failed group.
func haltReason(haltedBy string) string {
	return "delivery attempt stopped at the first failed group " + haltedBy
}

// unlaunchedGroupResult records a selected group that never launched, with
// the reason; the shape is the one ValidateTestResult accepts for not-run.
func unlaunchedGroupResult(request TestRunRequest, group testpolicy.Group, reason string) GroupResult {
	return GroupResult{ID: group.ID, Kind: group.Kind, Obligations: append([]string(nil), group.Obligations...),
		IdentityVersion: groupExecutionIdentityVersion(request),
		InputDigest:     request.CandidateTree, InputManifest: append([]string(nil), group.Inputs...), CWD: group.CWD,
		ExecutionIdentity: request.ComponentIdentities[group.ID],
		Status:            "not-run", NotRunReason: reason,
		ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
}

// progressWriter serializes the progress file and the TEST-GROUP lines on
// stdout, which several groups now write at once.
type progressWriter struct {
	mu   sync.Mutex
	path string
}

func (w *progressWriter) record(group, event, status, reason string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return testGroupProgress(w.path, group, event, status, reason)
}

// verdict records the supervisor's judgement of a group in the progress
// file, where the suite watchdog reads it.
func (w *progressWriter) verdict(group, verdict string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.path == "" {
		return nil
	}
	return AppendSectionEvent(w.path, SectionEvent{Suite: "testing", Section: group, Event: "verdict",
		At: time.Now().UTC().Format(time.RFC3339Nano), Depth: 0, Verdict: verdict})
}

var runStageTestGroup = runTestGroup

type stageGroupResolver func(context.Context, string) (GroupResult, bool, error)

type stageGroupDependenciesContextKey struct{}

type stageGroupDependencies struct {
	runGroup          func(context.Context, TestRunRequest, testpolicy.Group) GroupResult
	deliverCompletion func(chan<- stageGroupCompletion, stageGroupCompletion)
	beforeDispatch    func(string)
	afterAdmission    func(map[string]GroupResult)
}

func withStageGroupDependencies(ctx context.Context, dependencies stageGroupDependencies) context.Context {
	return context.WithValue(ctx, stageGroupDependenciesContextKey{}, dependencies)
}

func stageGroupDependenciesFromContext(ctx context.Context) stageGroupDependencies {
	dependencies := stageGroupDependencies{
		runGroup: runStageTestGroup,
		deliverCompletion: func(completion chan<- stageGroupCompletion, outcome stageGroupCompletion) {
			completion <- outcome
		},
		beforeDispatch: func(string) {},
		afterAdmission: func(map[string]GroupResult) {},
	}
	if injected, ok := ctx.Value(stageGroupDependenciesContextKey{}).(stageGroupDependencies); ok {
		if injected.runGroup != nil {
			dependencies.runGroup = injected.runGroup
		}
		if injected.deliverCompletion != nil {
			dependencies.deliverCompletion = injected.deliverCompletion
		}
		if injected.beforeDispatch != nil {
			dependencies.beforeDispatch = injected.beforeDispatch
		}
		if injected.afterAdmission != nil {
			dependencies.afterAdmission = injected.afterAdmission
		}
	}
	return dependencies
}

type stageGroupCompletion struct {
	index               int
	result              GroupResult
	err                 error
	stoppedBeforeLaunch bool
}

// runStageGroups runs an independent stage through the same completion-driven
// owner used by execution-contract prerequisite stages.
func runStageGroups(ctx context.Context, request TestRunRequest, groups map[string]testpolicy.Group, ids []string, progress *progressWriter, stopAtFirstFailure bool) ([]GroupResult, string, error) {
	return runStageGroupsScheduled(ctx, request, groups, ids, progress, stopAtFirstFailure, nil, nil)
}

// runStageGroupsWithPrerequisites releases same-stage dependents whenever an
// owned or retained prerequisite reaches a terminal result. Results remain in
// plan order even though completion order drives subsequent launches.
func runStageGroupsWithPrerequisites(ctx context.Context, request TestRunRequest, groups map[string]testpolicy.Group, ids []string, progress *progressWriter,
	completed map[string]GroupResult, resolve stageGroupResolver) ([]GroupResult, string, error) {
	return runStageGroupsScheduled(ctx, request, groups, ids, progress, false, completed, resolve)
}

// runStageGroupsScheduled keeps one owner for group concurrency, prerequisite
// readiness, retained evidence, performance isolation and cancellation drain.
func runStageGroupsScheduled(ctx context.Context, request TestRunRequest, groups map[string]testpolicy.Group, ids []string, progress *progressWriter,
	stopAtFirstFailure bool, prior map[string]GroupResult, resolve stageGroupResolver) ([]GroupResult, string, error) {
	ctx = withTestWorkerPool(ctx, EffectiveTestWorkers(request))
	dependencies := stageGroupDependenciesFromContext(ctx)
	results := make([]GroupResult, len(ids))
	if len(ids) == 0 {
		return results, "", nil
	}
	groupCap := request.Concurrency
	if groupCap < 1 {
		groupCap = 1
	}
	if groupCap > len(ids) {
		groupCap = len(ids)
	}
	// Longest first: each group's last measured duration in the retained
	// attempts (its declared targetMs when nothing was measured) orders the
	// launches, so the long groups start while the short ones fill the other
	// slots and the stage's wall time approaches its longest group instead of
	// the last long group's start plus its length. Results still land in plan
	// order.
	expected := retainedGroupDurations(request.ControlRoot)
	weight := func(id string) int64 {
		if measured, ok := expected[id]; ok && measured > groups[id].TargetMS {
			return measured
		}
		return groups[id].TargetMS
	}
	order := make([]int, len(ids))
	for index := range ids {
		if _, exists := groups[ids[index]]; !exists {
			return nil, "", fmt.Errorf("selected testing group %s is absent", ids[index])
		}
		order[index] = index
	}
	sort.SliceStable(order, func(a, b int) bool {
		aPerformance := groups[ids[order[a]]].Kind == "performance"
		bPerformance := groups[ids[order[b]]].Kind == "performance"
		if aPerformance != bPerformance {
			return !aPerformance
		}
		return weight(ids[order[a]]) > weight(ids[order[b]])
	})

	const (
		stagePending = iota
		stageActive
		stageDone
	)
	state := make([]int, len(ids))
	owned := make([]bool, len(ids))
	position := make(map[string]int, len(ids))
	for index, id := range ids {
		if _, duplicate := position[id]; duplicate {
			return nil, "", fmt.Errorf("testing stage contains duplicate group %s", id)
		}
		position[id] = index
	}
	known := make(map[string]GroupResult, len(prior)+len(ids))
	for id, groupResult := range prior {
		known[id] = groupResult
	}
	// A Go group waits for broader same-context groups that select its tests,
	// then launches only what their passing native terminals left uncovered.
	coverageSources := planStageCoverage(request, groups, ids)
	coverageParticipant := make([]bool, len(ids))
	for consumer, sources := range coverageSources {
		for _, source := range sources {
			coverageParticipant[consumer], coverageParticipant[source] = true, true
		}
	}

	remaining, active := len(ids), 0
	performanceActive := false
	stopping := false
	haltedBy := ""
	var firstErr error
	completion := make(chan stageGroupCompletion, len(ids))
	var launched sync.WaitGroup
	acquireCtx, cancelAcquires := context.WithCancel(ctx)
	defer cancelAcquires()
	type stopCause struct {
		haltedBy string
		err      error
	}
	var stopMu sync.Mutex
	var stopOnce sync.Once
	var stopped bool
	var cause stopCause
	signalStop := func(next stopCause) {
		stopOnce.Do(func() {
			stopMu.Lock()
			stopped, cause = true, next
			stopMu.Unlock()
			cancelAcquires()
		})
	}
	readStop := func() (stopCause, bool) {
		stopMu.Lock()
		defer stopMu.Unlock()
		return cause, stopped
	}
	adoptStop := func() {
		next, ok := readStop()
		if !ok {
			return
		}
		stopping = true
		if haltedBy == "" && next.haltedBy != "" {
			haltedBy = next.haltedBy
		}
		if firstErr == nil && next.err != nil {
			firstErr = next.err
		}
	}
	var stoppedBeforeLaunch []int

	finish := func(index int, groupResult GroupResult) {
		if state[index] == stageDone {
			return
		}
		state[index] = stageDone
		results[index] = groupResult
		known[ids[index]] = groupResult
		remaining--
	}
	unlaunched := func(index int) GroupResult {
		reason := "progress record failed before this group launched"
		if firstErr == nil && haltedBy != "" {
			reason = haltReason(haltedBy)
		}
		return unlaunchedGroupResult(request, groups[ids[index]], reason)
	}
	prerequisites := func(index int) (ready bool, blockers []string, err error) {
		ready = true
		for _, dependency := range groups[ids[index]].Requires {
			if result, done := known[dependency]; done {
				if result.Status != "passed" && result.Status != "reused" {
					blockers = append(blockers, dependency)
				}
				continue
			}
			if _, sameStage := position[dependency]; sameStage {
				ready = false
				continue
			}
			return false, nil, fmt.Errorf("testing plan omits prerequisite %s for %s", dependency, ids[index])
		}
		for _, source := range coverageSources[index] {
			if state[source] != stageDone {
				ready = false
			}
		}
		return ready, blockers, nil
	}
	launch := func(index int, release func()) {
		id, group := ids[index], groups[ids[index]]
		state[index] = stageActive
		active++
		if group.Kind == "performance" {
			performanceActive = true
		}
		var sources []GroupResult
		for _, source := range coverageSources[index] {
			sources = append(sources, results[source])
		}
		launched.Add(1)
		go func() {
			defer launched.Done()
			groupCtx := withTestWorkerPool(ctx, EffectiveTestWorkers(request))
			if coverageParticipant[index] {
				groupCtx = withCoverageSources(groupCtx, sources)
			}
			if release != nil {
				defer release()
			}
			dependencies.beforeDispatch(id)
			_, stageStopped := readStop()
			ctxErr := ctx.Err()
			if ctxErr != nil {
				signalStop(stopCause{err: ctxErr})
				stageStopped = true
			}
			if stageStopped {
				dependencies.deliverCompletion(completion, stageGroupCompletion{index: index,
					result: unlaunchedGroupResult(request, group, "testing stage stopped before this group launched"),
					err:    ctxErr, stoppedBeforeLaunch: ctxErr == nil})
				return
			}
			if err := progress.record(id, "start", "", ""); err != nil {
				signalStop(stopCause{err: err})
				dependencies.deliverCompletion(completion, stageGroupCompletion{index: index,
					result: unlaunchedGroupResult(request, group, "record testing group start: "+err.Error()), err: err})
				return
			}
			groupResult := dependencies.runGroup(groupCtx, request, group)
			var progressErr error
			if groupResult.Status == "dead" || groupResult.Status == "runaway" {
				progressErr = progress.verdict(id, groupResult.Status)
			}
			if err := progress.record(id, "end", groupResult.Status, groupResult.NotRunReason); progressErr == nil {
				progressErr = err
			}
			if progressErr != nil {
				signalStop(stopCause{err: progressErr})
			} else if stopAtFirstFailure && groupResult.Status != "passed" && groupResult.Status != "reused" {
				signalStop(stopCause{haltedBy: id})
			}
			dependencies.deliverCompletion(completion, stageGroupCompletion{index: index, result: groupResult, err: progressErr})
		}()
	}
	waiting := make([]func(), len(ids))
	stopWaiting := func(index int) {
		if waiting[index] != nil {
			waiting[index]()
			waiting[index] = nil
		}
	}
	stopAllWaiting := func() {
		for index := range waiting {
			stopWaiting(index)
		}
	}
	defer stopAllWaiting()
	tryLaunch := func(index int) (bool, <-chan struct{}, error) {
		group := groups[ids[index]]
		if group.Adapter == "go" {
			launch(index, nil)
			return true, nil, nil
		}
		workers, err := EffectiveGroupWorkers(request, group)
		if group.Kind == "performance" {
			workers = EffectiveTestWorkers(request)
		}
		if err != nil {
			return false, nil, err
		}
		stopWaiting(index)
		release, changed, stop, acquired, err := tryAcquireTestWorkers(acquireCtx, workers)
		if err != nil {
			return false, nil, err
		}
		if !acquired {
			waiting[index] = stop
			return false, changed, nil
		}
		launch(index, release)
		return true, nil, nil
	}
	consume := func(outcome stageGroupCompletion) {
		active--
		if groups[ids[outcome.index]].Kind == "performance" {
			performanceActive = false
		}
		finish(outcome.index, outcome.result)
		if outcome.stoppedBeforeLaunch {
			stoppedBeforeLaunch = append(stoppedBeforeLaunch, outcome.index)
		}
		if outcome.err != nil {
			if firstErr == nil {
				firstErr = outcome.err
			}
			signalStop(stopCause{err: outcome.err})
		}
		if stopAtFirstFailure && outcome.err == nil && !outcome.stoppedBeforeLaunch && outcome.result.Status != "passed" && outcome.result.Status != "reused" {
			signalStop(stopCause{haltedBy: ids[outcome.index]})
		}
		adoptStop()
	}

	drainFor := -1
	for remaining > 0 {
		madeProgress := false
		var capacityChanged <-chan struct{}
		adoptStop()
		if !stopping {
			if err := ctx.Err(); err != nil {
				firstErr = err
				stopping = true
				signalStop(stopCause{err: err})
			}
		}
		if stopping {
			stopAllWaiting()
			for _, index := range order {
				if state[index] == stagePending {
					finish(index, unlaunched(index))
					madeProgress = true
				}
			}
		} else {
			// Terminal retained results and prerequisite failures do not consume a
			// group slot. Re-scan until each result has released all dependents it
			// can make ready in this scheduling turn.
			for changed := true; changed; {
				changed = false
				for _, index := range order {
					if state[index] != stagePending {
						continue
					}
					ready, blockers, err := prerequisites(index)
					if err != nil {
						if firstErr == nil {
							firstErr = err
						}
						stopping = true
						signalStop(stopCause{err: err})
						break
					}
					if len(blockers) != 0 {
						finish(index, blockedGroupResult(unlaunchedGroupResult(request, groups[ids[index]], "prerequisite blocked"), blockers))
						changed, madeProgress = true, true
						continue
					}
					if !ready || owned[index] || resolve == nil {
						continue
					}
					groupResult, resolved, err := resolve(ctx, ids[index])
					if err != nil {
						if firstErr == nil {
							firstErr = err
						}
						stopping = true
						signalStop(stopCause{err: err})
						break
					}
					if resolved {
						finish(index, groupResult)
						changed, madeProgress = true, true
					} else {
						owned[index] = true
					}
				}
				if stopping {
					break
				}
			}
		}
		if stopping {
			if remaining == 0 {
				break
			}
			if active == 0 {
				continue
			}
			consume(<-completion)
			continue
		}

		admitError := func(index int, err error) {
			stopWaiting(index)
			if errors.Is(err, context.Canceled) {
				if _, stageStopped := readStop(); stageStopped {
					adoptStop()
					finish(index, unlaunched(index))
					return
				}
			}
			if firstErr == nil {
				firstErr = err
			}
			finish(index, unlaunchedGroupResult(request, groups[ids[index]], "acquire testing workers: "+err.Error()))
			stopping = true
			signalStop(stopCause{err: err})
		}
		if drainFor >= 0 && (state[drainFor] != stagePending || active >= groupCap || performanceActive) {
			if state[drainFor] != stagePending {
				drainFor = -1
			}
		}
		if drainFor >= 0 && active < groupCap && !performanceActive {
			ready, blockers, err := prerequisites(drainFor)
			if err != nil {
				admitError(drainFor, err)
			} else if ready && len(blockers) == 0 && (groups[ids[drainFor]].Kind != "performance" || active == 0) {
				admitted, changed, err := tryLaunch(drainFor)
				if err != nil {
					admitError(drainFor, err)
				} else if admitted {
					madeProgress = true
					drainFor = -1
				} else {
					capacityChanged = changed
				}
			}
		}
		if !stopping && drainFor < 0 && active < groupCap && !performanceActive {
			oldestBlocked := -1
			bypassed := false
			for _, index := range order {
				if active >= groupCap || performanceActive {
					break
				}
				if state[index] != stagePending {
					continue
				}
				ready, blockers, err := prerequisites(index)
				if err != nil {
					admitError(index, err)
					break
				}
				if !ready || len(blockers) != 0 {
					continue
				}
				if groups[ids[index]].Kind == "performance" && active != 0 {
					continue
				}
				admitted, changed, err := tryLaunch(index)
				if err != nil {
					admitError(index, err)
					break
				}
				if !admitted {
					if oldestBlocked < 0 {
						oldestBlocked = index
						capacityChanged = changed
					}
					continue
				}
				madeProgress = true
				if oldestBlocked >= 0 {
					bypassed = true
				}
			}
			if oldestBlocked >= 0 && bypassed {
				drainFor = oldestBlocked
			}
		}
		if stopping {
			continue
		}
		if remaining == 0 {
			break
		}
		dependencies.afterAdmission(known)
		if active == 0 && capacityChanged == nil {
			if madeProgress {
				continue
			}
			firstErr = fmt.Errorf("testing plan has a prerequisite cycle or later-stage prerequisite")
			stopping = true
			signalStop(stopCause{err: firstErr})
			continue
		}

		select {
		case outcome := <-completion:
			consume(outcome)
		case <-capacityChanged:
		case <-ctx.Done():
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			stopping = true
			signalStop(stopCause{err: firstErr})
		}
	}
	launched.Wait()
	adoptStop()
	if firstErr == nil {
		firstErr = ctx.Err()
	}
	for _, index := range stoppedBeforeLaunch {
		results[index] = unlaunched(index)
	}
	return results, haltedBy, firstErr
}

// retainedGroupDurations reads the newest measured duration of every group
// from the retained attempts under the control root. It is advisory
// scheduling input only: a missing or unreadable inventory means no
// measurements, never an error.
func retainedGroupDurations(controlRoot string) map[string]int64 {
	durations := map[string]int64{}
	if controlRoot == "" {
		return durations
	}
	attempts, err := ReadAttempts(controlRoot)
	if err != nil {
		return durations
	}
	newest := map[string]string{}
	for _, attempt := range attempts {
		if attempt.TestResult == nil {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			if group.DurationMS <= 0 || (group.Status != "passed" && group.Status != "failed") {
				continue
			}
			if at, seen := newest[group.ID]; seen && at >= attempt.StartedAt {
				continue
			}
			newest[group.ID] = attempt.StartedAt
			durations[group.ID] = group.DurationMS
		}
	}
	return durations
}

// runShardedGoGroup runs one Go group as isolated go test partitions. An
// explicit shard count is a ceiling; otherwise the attempt worker allowance
// is the ceiling. Every partition writes its own log beside the group's;
// the group log is their concatenation in shard order, which is also what
// the group's parsers read. The outcome is the merged supervision: any
// verdict, the first wait error, the summed CPU, the longest silences, and
// the first nonzero exit. With coverage on, each shard writes coverage data
// under the group's log directory and the merged per-package percentages
// come back as go-test-shaped lines for the coverage floors.
type goShardLifecycleHooks struct {
	prepareCoverage func(index int, directory, groupID string) error
	openLog         func(path string) (*os.File, error)
	wrapOutput      func(index int, writer io.Writer) io.Writer
}

type goShardLifecycleHooksKey struct{}

func prepareGoShardCoverage(ctx context.Context, index int, directory, groupID string) error {
	if hooks, ok := ctx.Value(goShardLifecycleHooksKey{}).(goShardLifecycleHooks); ok && hooks.prepareCoverage != nil {
		return hooks.prepareCoverage(index, directory, groupID)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create shard coverage directory: %w", err)
	}
	// The test binary writes its counters here only at exit, so for the
	// shard's whole run the directory would stand empty, and the evidence
	// collector could sweep it. The marker says a writer is coming; covdata
	// ignores it.
	if err := os.WriteFile(filepath.Join(directory, ".pending"), []byte(groupID+"\n"), 0o600); err != nil {
		return fmt.Errorf("mark shard coverage directory: %w", err)
	}
	return nil
}

func openGoShardLog(ctx context.Context, path string) (*os.File, error) {
	if hooks, ok := ctx.Value(goShardLifecycleHooksKey{}).(goShardLifecycleHooks); ok && hooks.openLog != nil {
		return hooks.openLog(path)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
}

type cancelOnWriteError struct {
	mu     sync.Mutex
	writer io.Writer
	cancel context.CancelFunc
	err    error
}

func (writer *cancelOnWriteError) Write(data []byte) (int, error) {
	n, err := writer.writer.Write(data)
	if err != nil {
		writer.mu.Lock()
		if writer.err == nil {
			writer.err = err
		}
		writer.mu.Unlock()
		writer.cancel()
	}
	return n, err
}

func (writer *cancelOnWriteError) Err() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.err
}

func runShardedGoGroup(ctx context.Context, request TestRunRequest, group testpolicy.Group, cwd string, environment []string, expected []NativeTestIdentity,
	inventory []string, modulePrefix string, limits supervisorLimits, sampleInterval time.Duration, logPath string,
	output *synchronizedBuffer) (supervisorOutcome, error, string, error) {
	ctx = withTestWorkerPool(ctx, EffectiveTestWorkers(request))
	ceiling := EffectiveTestWorkers(request)
	if group.Shards > 0 && group.Shards < ceiling {
		ceiling = group.Shards
	}
	partitions := partitionGoTests(expected, inventory, modulePrefix, ceiling)
	coverageRoot := strings.TrimSuffix(logPath, ".log") + ".coverage"
	if group.Coverage {
		if err := os.RemoveAll(coverageRoot); err != nil {
			return supervisorOutcome{}, nil, "", fmt.Errorf("reset shard coverage directory: %w", err)
		}
	}
	type shardRun struct {
		outcome   supervisorOutcome
		output    synchronizedBuffer
		logPath   string
		closeErr  error
		launchErr error
	}
	runs := make([]shardRun, len(partitions))
	// An error while launching a later shard must not leave the earlier
	// shards running inside a worktree the caller is about to remove. The
	// shards share one cancelable context, and launch-loop failures cancel
	// it and wait before returning.
	shardCtx, cancelShards := context.WithCancel(ctx)
	defer cancelShards()
	var wg sync.WaitGroup
	var setupErr error
	nativeEnvironment := overlayTestEnvironment(environment, map[string]string{
		"GOMAXPROCS":           "1",
		TestWorkersEnvironment: "1",
	})
	for index := range partitions {
		args := goNativeTestArguments(group, group.Coverage)
		patterns := make([]string, len(partitions[index].Names))
		for at, name := range partitions[index].Names {
			patterns[at] = regexp.QuoteMeta(name)
		}
		pattern := "^$"
		if len(patterns) > 0 {
			pattern = "^(" + strings.Join(patterns, "|") + ")$"
		}
		args = append(args, "-run", pattern)
		args = append(args, partitions[index].Packages...)
		shardDir := filepath.Join(coverageRoot, fmt.Sprintf("shard-%d", index+1))
		if group.Coverage {
			if err := prepareGoShardCoverage(shardCtx, index, shardDir, group.ID); err != nil {
				setupErr = err
				cancelShards()
				break
			}
			args = append(args, "-args", "-test.gocoverdir="+shardDir)
		}
		runs[index].logPath = fmt.Sprintf("%s.shard-%d.log", strings.TrimSuffix(logPath, ".log"), index+1)
		wg.Add(1)
		go func(index int, args []string) {
			defer wg.Done()
			release, err := acquireTestWorkers(shardCtx, 1)
			if err != nil {
				runs[index].launchErr = fmt.Errorf("acquire Go test worker: %w", err)
				cancelShards()
				return
			}
			defer release()
			command, err := explicitEnvironmentCommand(context.WithoutCancel(shardCtx), cwd, nativeEnvironment, args)
			if err != nil {
				runs[index].launchErr = err
				cancelShards()
				return
			}
			logFile, err := openGoShardLog(shardCtx, runs[index].logPath)
			if err != nil {
				runs[index].launchErr = fmt.Errorf("create shard log: %w", err)
				cancelShards()
				return
			}
			activity := newOutputActivity(time.Now())
			var outputWriter io.Writer = io.MultiWriter(&runs[index].output, logFile)
			if hooks, ok := shardCtx.Value(goShardLifecycleHooksKey{}).(goShardLifecycleHooks); ok && hooks.wrapOutput != nil {
				outputWriter = hooks.wrapOutput(index, outputWriter)
			}
			guardedOutput := &cancelOnWriteError{writer: outputWriter, cancel: cancelShards}
			tee := &activityWriter{activity: activity, writer: guardedOutput}
			command.Stdout, command.Stderr = tee, tee
			closeInherited := func() {}
			if HostResourceLeaseFromContext(shardCtx) == nil {
				var inheritErr error
				closeInherited, inheritErr = InheritHostResourceLease(command)
				if inheritErr != nil {
					runs[index].launchErr = inheritErr
					runs[index].closeErr = logFile.Close()
					cancelShards()
					return
				}
			}
			runs[index].outcome = superviseCommand(command, supervisorOptions{Context: shardCtx, Limits: limits, SampleInterval: sampleInterval, Activity: activity})
			closeInherited()
			runs[index].closeErr = logFile.Close()
			if err := guardedOutput.Err(); err != nil {
				runs[index].launchErr = fmt.Errorf("write Go test shard output: %w", err)
				cancelShards()
				return
			}
			if !runs[index].outcome.Started && runs[index].outcome.WaitErr != nil {
				runs[index].launchErr = fmt.Errorf("start Go test shard: %w", runs[index].outcome.WaitErr)
				cancelShards()
			}
		}(index, append([]string(nil), args...))
	}
	wg.Wait()
	merged := supervisorOutcome{}
	var closeErr error
	launchErr := setupErr
	for index := range runs {
		run := &runs[index]
		merged.Started = merged.Started || run.outcome.Started
		merged.CPUSeconds += run.outcome.CPUSeconds
		if run.outcome.LongestSilentSeconds > merged.LongestSilentSeconds {
			merged.LongestSilentSeconds = run.outcome.LongestSilentSeconds
		}
		if run.outcome.LongestZeroCPUSeconds > merged.LongestZeroCPUSeconds {
			merged.LongestZeroCPUSeconds = run.outcome.LongestZeroCPUSeconds
		}
		if merged.Verdict == "" && run.outcome.Verdict != "" {
			merged.Verdict, merged.Reason, merged.Dump = run.outcome.Verdict, fmt.Sprintf("shard %d: %s", index+1, run.outcome.Reason), run.outcome.Dump
		}
		if merged.RuleSuffix == "" {
			merged.RuleSuffix = run.outcome.RuleSuffix
		}
		if merged.WaitErr == nil && run.outcome.WaitErr != nil {
			merged.WaitErr = run.outcome.WaitErr
		}
		if closeErr == nil && run.closeErr != nil {
			closeErr = run.closeErr
		}
		if run.launchErr != nil && (launchErr == nil || errors.Is(launchErr, context.Canceled) && !errors.Is(run.launchErr, context.Canceled)) {
			launchErr = run.launchErr
		}
		output.Write(run.output.Bytes())
	}
	// The merged percentages are appended to the group's diagnostic output,
	// while the separate return remains the only coverage-authority channel.
	writeLog := func() error {
		if err := os.WriteFile(logPath, output.Bytes(), 0o600); err != nil {
			return fmt.Errorf("write group log: %w", err)
		}
		return nil
	}
	coverageMerge := ""
	if group.Coverage && merged.Started && launchErr == nil {
		dirs := make([]string, len(partitions))
		for index := range dirs {
			dirs[index] = filepath.Join(coverageRoot, fmt.Sprintf("shard-%d", index+1))
		}
		percent, err := explicitEnvironmentCommand(ctx, cwd, environment, []string{"go", "tool", "covdata", "percent", "-i=" + strings.Join(dirs, ",")})
		if err != nil {
			_ = writeLog()
			return merged, closeErr, "", err
		}
		var percentOutput bytes.Buffer
		percent.Stdout, percent.Stderr = &percentOutput, &percentOutput
		err = RunResourceCommand(ctx, percent, HostResourceLeaseFromContext(ctx))
		data := percentOutput.Bytes()
		if err != nil {
			_ = writeLog()
			return merged, closeErr, "", fmt.Errorf("merge shard coverage: %v: %s", err, strings.TrimSpace(string(data)))
		}
		// The merged percentages join the group's go test -json stream as
		// output events in go test's own summary shape, so the coverage
		// parser reads them last and they win over the shards' partial lines.
		var lines strings.Builder
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			// covdata writes the package prefix before checking whether the
			// package has statements. An empty package therefore contributes no
			// newline, so the measured package is the field immediately before
			// the exact coverage token sequence, not necessarily fields[0].
			for index := 1; index+3 < len(fields); index++ {
				if fields[index] != "coverage:" || fields[index+2] != "of" || fields[index+3] != "statements" ||
					!strings.HasSuffix(fields[index+1], "%") {
					continue
				}
				percentage := strings.TrimSuffix(fields[index+1], "%")
				value, parseErr := strconv.ParseFloat(percentage, 64)
				if parseErr != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
					continue
				}
				pkg := fields[index-1]
				event, err := json.Marshal(goEvent{Action: "output", Package: pkg,
					Output: fmt.Sprintf("ok  \t%s\t0.000s\tcoverage: %s of statements\n", pkg, fields[index+1])})
				if err != nil {
					_ = writeLog()
					return merged, closeErr, "", err
				}
				lines.Write(event)
				lines.WriteString("\n")
				break
			}
		}
		coverageMerge = lines.String()
		output.Write([]byte(coverageMerge))
	}
	if err := writeLog(); err != nil {
		return merged, closeErr, "", err
	}
	return merged, closeErr, coverageMerge, launchErr
}

func testGroupProgress(path, group, event, status, reason string) error {
	if path == "" {
		return nil
	}
	if err := AppendSectionEvent(path, SectionEvent{Suite: "testing", Section: group,
		Event: event, At: time.Now().UTC().Format(time.RFC3339Nano), Depth: 0}); err != nil {
		return fmt.Errorf("record testing group %s %s: %w", group, event, err)
	}
	// The launcher watches output growth separately from section boundaries.
	// Emit actual group transitions into its pipe so completed work resets silence.
	_, err := fmt.Fprintf(os.Stdout, "TEST-GROUP %s %s status=%s reason=%s\n", event, group, status, reason)
	return err
}

// validateRetainedGroupReuse checks a supplied reuse against the attempt it
// names in the control root: the attempt must have reached a
// terminal (never live, whatever its result) and must own matching, complete,
// passed evidence for the group at the same identity.
func validateRetainedGroupReuse(root, id string, reused GroupResult) error {
	if root == "" || reused.ReuseAttempt == "" || reused.ID != id || reused.ExecutionIdentity == "" {
		return fmt.Errorf("component has no exact retained outer owner")
	}
	attempt, err := ReadAttempt(root, reused.ReuseAttempt)
	if err != nil || attempt.Terminal == nil || attempt.TestResult == nil {
		return fmt.Errorf("outer attempt is not a retained terminal")
	}
	for _, recorded := range attempt.TestResult.Groups {
		if recorded.ID == id && recorded.ExecutionIdentity == reused.ExecutionIdentity && recorded.CollectionComplete &&
			(recorded.Status == "passed" || recorded.Status == "reused") {
			return nil
		}
	}
	return fmt.Errorf("outer attempt does not own matching complete group evidence")
}

// assignSupervisorOutcome is the single projection from native supervision
// into retained group accounting. A process that started remains a launch
// even when a later setup, output, or wait operation fails. An exit status is
// retained only when waiting observed either successful exit or an ExitError.
func assignSupervisorOutcome(result *GroupResult, outcome supervisorOutcome) {
	if outcome.RuleSuffix != "" {
		result.ProgressRule += "+" + outcome.RuleSuffix
	}
	result.CPUSeconds = outcome.CPUSeconds
	result.LongestSilentSeconds = outcome.LongestSilentSeconds
	result.LongestZeroCPUSeconds = outcome.LongestZeroCPUSeconds
	result.NativeLaunched = outcome.Started
	result.NativeExitStatus = nil
	result.Signal = nil
	if !outcome.Started {
		if outcome.WaitErr != nil {
			result.Status = "unavailable"
			result.NotRunReason = outcome.WaitErr.Error()
		}
		return
	}
	if outcome.WaitErr == nil {
		exit := 0
		result.NativeExitStatus = &exit
		return
	}
	var exitErr *exec.ExitError
	if !errors.As(outcome.WaitErr, &exitErr) {
		result.Status = "unavailable"
		result.NotRunReason = outcome.WaitErr.Error()
		return
	}
	exit := exitErr.ExitCode()
	result.NativeExitStatus = &exit
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		value := status.Signal().String()
		result.Signal = &value
	}
}

func NewTestResult(request TestRunRequest) TestResult {
	return newTestResult(request, testRequestNow(request))
}

// NewTestResultAt constructs a reusable-result template at the caller's
// admitted semantic time. It changes no serialized request schema.
func NewTestResultAt(request TestRunRequest, now time.Time) TestResult {
	return newTestResult(request, now.UTC())
}

func newTestResult(request TestRunRequest, semanticNow time.Time) TestResult {
	contractBytes, _ := json.Marshal(request.Contract)
	contractDigest := request.ContractDigest
	if contractDigest == "" {
		contractDigest = digestBytes(contractBytes)
	}
	baseContractDigest := request.BaseContractDigest
	if baseContractDigest == "" {
		baseContractDigest = contractDigest
	}
	policyEngineDigest := request.PolicyEngineDigest
	if policyEngineDigest == "" {
		policyEngineDigest = digestBytes([]byte(fmt.Sprintf("testpolicy-schema-%d", request.Contract.SchemaVersion)))
	}
	judgeKey := judgeKeyOf(request)
	behaviorPolicyDigest := request.BehaviorPolicyDigest
	if behaviorPolicyDigest == "" {
		behaviorPolicyDigest = digestBytes([]byte(fmt.Sprintf("behavior-policy-version-%d", request.Contract.SchemaVersion)))
	}
	targetMS := int64(0)
	selected := map[string]bool{}
	for _, id := range request.Plan.SelectedGroups {
		selected[id] = true
	}
	for _, group := range request.Contract.Groups {
		if selected[group.ID] {
			targetMS += group.TargetMS
		}
	}
	candidateEngineIdentityVersion := CandidateEngineIdentitySchemaVersion
	if request.CandidateEngineBuildIdentity == "" {
		candidateEngineIdentityVersion = candidateEngineDigestIdentityVersion
	}
	schemaVersion := PreviousTestResultSchemaVersion
	workerPolicyVersion, workers := 0, 0
	var admissionMaximum *int
	if TestWorkerPolicyActive(request) {
		schemaVersion = TestResultSchemaVersion
		workerPolicyVersion, workers = TestWorkerPolicyVersion, EffectiveTestWorkers(request)
		resolvedAdmissionMaximum := request.AdmissionMaximum
		admissionMaximum = &resolvedAdmissionMaximum
	}
	return TestResult{SchemaVersion: schemaVersion, AttemptID: request.AttemptID, EngineRearm: request.EngineRearm,
		WorkerPolicyVersion: workerPolicyVersion, Workers: workers, AdmissionMaximum: admissionMaximum,
		Purpose: request.Plan.Purpose, RequestedMode: request.Plan.RequestedMode, RequiredMode: request.Plan.RequiredMode,
		ExecutedMode: request.Plan.ExecutedMode, ProjectRoot: request.ProjectRoot, InstallationPrefix: request.InstallationPrefix,
		BaseCommit: request.BaseCommit, CandidateTree: request.CandidateTree, PolicyBaseCommit: request.PolicyBaseCommit,
		ContractDigest: contractDigest, BaseContractDigest: baseContractDigest,
		PolicyEngineDigest: policyEngineDigest, JudgeKey: judgeKey, CandidateEngineIdentityVersion: candidateEngineIdentityVersion,
		CandidateEngineDigest: request.CandidateEngineDigest, CandidateEngineBuildIdentity: request.CandidateEngineBuildIdentity,
		BehaviorPolicyDigest: behaviorPolicyDigest,
		PlanDigest:           TestPlanDigest(request.Contract, request.Plan, request.CandidateTree), Risk: request.Plan.Risk,
		FreshnessEpisode: request.FreshnessEpisode, FreshnessBinding: request.FreshnessBinding,
		FreshnessExpiresAt: request.FreshnessExpiresAt, FreshGroups: cloneFreshGroups(request.FreshGroups),
		RequiredGroups: append([]string(nil), request.Plan.RequiredGroups...), SelectedGroups: append([]string(nil), request.Plan.SelectedGroups...),
		Omissions: append([]testpolicy.Omission(nil), request.Plan.Omissions...), Uncertainty: append([]string(nil), request.Plan.Uncertainty...),
		LaunchCounts: LaunchCounts{Other: request.PreparationLaunches, CountsComplete: true}, StartedAt: semanticNow.Format(time.RFC3339Nano),
		Cost: TestCost{DeclaredTargetMS: targetMS, PreparationDurationMS: request.PreparationDurationMS,
			QueueDurationMS: request.QueueDurationMS}, semanticNow: semanticNow}
}

func runTestGroup(ctx context.Context, request TestRunRequest, group testpolicy.Group) (result GroupResult) {
	ctx = withTestWorkerPool(ctx, EffectiveTestWorkers(request))
	started := time.Now()
	startedAt := testRequestNow(request)
	limits, sampleInterval := groupSupervisorSettings(group.CPUBudgetSeconds)
	result = GroupResult{ID: group.ID, Kind: group.Kind, Obligations: append([]string(nil), group.Obligations...),
		IdentityVersion: groupExecutionIdentityVersion(request),
		InputDigest:     request.CandidateTree, InputManifest: append([]string(nil), group.Inputs...), CWD: group.CWD,
		ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}, StartedAt: startedAt.Format(time.RFC3339Nano),
		ProgressRule: progressRule(limits)}
	defer func() {
		if group.Adapter == "section" && result.NotRunReason == "" && result.NativeExitStatus != nil &&
			(*result.NativeExitStatus != 0 || result.Status != "passed") {
			result.NotRunReason = scriptFailureReason(result.LogPath, *result.NativeExitStatus)
		}
		if result.Status == "passed" || result.Status == "reused" || result.LogPath == "" {
			return
		}
		recordAppendFailure := func(err error) {
			if result.NotRunReason != "" {
				result.NotRunReason += "; "
			}
			result.NotRunReason += "verdict line not written: " + err.Error()
		}
		logFile, err := os.OpenFile(result.LogPath, os.O_RDWR|os.O_APPEND, 0)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err != nil {
			recordAppendFailure(err)
			return
		}
		exitStatus := "none"
		if result.NativeExitStatus != nil {
			exitStatus = fmt.Sprintf("%d", *result.NativeExitStatus)
		}
		reason := strings.Join(strings.Fields(result.NotRunReason), " ")
		verdict := fmt.Sprintf("TEST-VERDICT %s status=%s exit=%s reason=%s\n", group.ID, result.Status, exitStatus, reason)
		existing, err := io.ReadAll(logFile)
		if err != nil {
			recordAppendFailure(errors.Join(err, logFile.Close()))
			return
		}
		separator := ""
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			separator = "\n"
		}
		var reruns strings.Builder
		for _, finding := range result.Reruns {
			fmt.Fprintf(&reruns, "TEST-RERUN %s %s.%s first=%s second=%s failed-load=%s rerun-load=%s\n",
				group.ID, finding.Package, finding.Test, finding.First, finding.Second,
				finding.FailedLoad.Describe(), finding.RerunLoad.Describe())
		}
		_, writeErr := io.WriteString(logFile, separator+reruns.String()+verdict)
		_, seekErr := logFile.Seek(0, io.SeekStart)
		logBytes, readErr := io.ReadAll(logFile)
		closeErr := logFile.Close()
		if err := errors.Join(writeErr, seekErr, readErr, closeErr); err != nil {
			recordAppendFailure(err)
			return
		}
		result.LogDigest = digestBytes(logBytes)
	}()
	detached, err := request.candidateWorkspace()
	if err != nil {
		result.Status = "invalid"
		result.NotRunReason = err.Error()
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	defer func() {
		// The detached evidence copy is bounded by bytes, never by the clock:
		// a sixty-second bound turned a group invalid on a slow disk under
		// load (proof-groups-detect-hangs-by-progress-not-the-clock, slice 2).
		timeout, maxBytes := time.Duration(0), request.EvidenceMaxBytes
		if maxBytes < 1 {
			maxBytes = 512 * 1024 * 1024
		}
		controlRoot := request.ControlRoot
		if controlRoot == "" {
			controlRoot = request.ProjectRoot
		}
		_, _, preserveErr := PreserveDetachedSuiteFailures(controlRoot, detached.Workspace().Dir, group.ID, timeout, maxBytes)
		if preserveErr == nil {
			preserveErr = detached.Close()
		}
		if preserveErr != nil {
			result.Status = "invalid"
			result.CollectionComplete = false
			if result.NotRunReason != "" {
				result.NotRunReason += "; "
			}
			result.NotRunReason += "preserve detached suite-failure evidence before cleanup: " + preserveErr.Error()
		}
	}()
	root := detached.Workspace().Dir
	cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
	environment := groupTestEnvironment(request, group)
	prepared, hasPrepared := request.PreparedGroups[group.ID]
	implicitInputs := prepared.ImplicitInputs
	coverageInventory := prepared.CoverageInventory
	coverageModule := prepared.CoverageModule
	inputDigest, err := digestGroupInputsWithImplicit(root, group, environment, implicitInputs)
	if err != nil {
		result.Status = "invalid"
		result.NotRunReason = err.Error()
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	result.InputDigest = inputDigest
	result.EnvironmentDigest = digestGroupEnvironment(group, environment)
	plannedTools, argv, expected, availabilityErr := map[string]string{}, []string(nil), []NativeTestIdentity(nil), error(nil)
	if hasPrepared {
		if prepared.InputDigest != inputDigest || prepared.EnvironmentDigest != result.EnvironmentDigest {
			availabilityErr = fmt.Errorf("prepared group inputs or environment changed before execution")
		} else {
			plannedTools = cloneStringMap(prepared.ToolIdentities)
			expected = append([]NativeTestIdentity(nil), prepared.Expected...)
			argv = append([]string(nil), prepared.Argv...)
			if prepared.Unavailable != "" {
				availabilityErr = errors.New(prepared.Unavailable)
			} else {
				availabilityErr = verifyPreparedExecutables(ctx, cwd, environment, group, argv, prepared.ExecutableDigests)
			}
		}
	} else {
		var launches int
		plannedTools, _, argv, expected, implicitInputs, coverageInventory, coverageModule, launches, availabilityErr = plannedToolIdentities(ctx, cwd, environment, group, root, nil, request.Contract.SchemaVersion)
		result.OtherLaunches += launches
		if discoveredDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicitInputs); digestErr != nil {
			availabilityErr = errors.Join(availabilityErr, digestErr)
		} else {
			inputDigest = discoveredDigest
			result.InputDigest = discoveredDigest
		}
	}
	result.ToolIdentities = plannedTools
	if hasPrepared {
		result.ExecutableDigests = cloneStringMap(prepared.ExecutableDigests)
	}
	result.InputManifest = mergeInputManifest(group.Inputs, implicitInputs)
	result.Argv, result.Expected = argv, expected
	// One composition per attempt: the launcher planned this group's identity
	// on the same candidate tree, and the record carries that plan, so a
	// launcher and a worker of different builds cannot disagree inside one
	// attempt. The inputs digest is still read before and after the run.
	result.ExecutionIdentity = groupExecutionIdentity(request, group, cwd, inputDigest, result.EnvironmentDigest, result.ToolIdentities, expected)
	if planned := request.ComponentIdentities[group.ID]; planned != "" {
		result.ExecutionIdentity = planned
	}
	if availabilityErr != nil {
		result.Status = "unavailable"
		result.NotRunReason = availabilityErr.Error()
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	var credit testCoverageCredit
	if sources, participant := coverageSourcesFromContext(ctx); participant && group.Adapter == "go" {
		result.NativeContext = goNativeContext(request, group)
		if result.NativeContext != "" && len(sources) != 0 {
			credit = creditCoveredTests(result, sources)
		}
	}
	nativeExpected := expected
	if len(credit.tests) != 0 {
		nativeExpected = credit.residual
	}
	if err := prepareGroupOutputs(root, group); err != nil {
		result.Status = "invalid"
		result.NotRunReason = err.Error()
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	sectionReport := ""
	switch group.Adapter {
	case "go":
		if metaSystemStewardConsumesCandidateEngine(request, group, cwd) {
			engine, err := prepareSectionEngine(cwd, request.CandidateEngine, request.CandidateEngineDigest)
			if err != nil {
				result.Status, result.NotRunReason = "invalid", err.Error()
				result.EndedAt, result.DurationMS = resultDuration(request, started)
				return result
			}
			environment = overlayTestEnvironment(environment, map[string]string{
				"METASYSTEM_BIN": engine,
				"METASYSTEM_STEWARD_TEST_CANDIDATE_ENGINE": "1",
			})
		}
	case "section":
		sectionReport = filepath.Join(request.LogRoot, group.ID+".stage-results.tsv")
		environment = overlayTestEnvironment(environment, map[string]string{"METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT": sectionReport})
		if request.CandidateEngine != "" || request.CandidateEngineDigest != "" {
			engine, err := prepareSectionEngine(cwd, request.CandidateEngine, request.CandidateEngineDigest)
			if err != nil {
				result.Status, result.NotRunReason = "invalid", err.Error()
				result.EndedAt, result.DurationMS = resultDuration(request, started)
				return result
			}
			// Readiness follows the verified immutable input-bound artifact.
			// The shell still authenticates this worker's actual parent custody.
			environment = overlayTestEnvironment(environment, map[string]string{
				"METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY": "ready",
				"METASYSTEM_PROOF_AUTH_BIN":                engine,
				"METASYSTEM_SUITE_PROGRESS_ACTIVE":         "1",
				"METASYSTEM_SUITE_PROGRESS_LOG":            filepath.Join(request.LogRoot, group.ID+".log"),
				"METASYSTEM_SUITE_PROGRESS_ROOT":           cwd,
			})
		}
	}
	if group.Adapter == "section" {
		if err := os.MkdirAll(filepath.Dir(sectionReport), 0o700); err != nil {
			result.Status = "invalid"
			result.NotRunReason = fmt.Sprintf("create section result parent: %v", err)
			result.EndedAt, result.DurationMS = resultDuration(request, started)
			return result
		}
	}
	if err := os.MkdirAll(request.LogRoot, 0o700); err != nil {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log directory: %v", err)
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	result.LogPath = filepath.Join(request.LogRoot, group.ID+".log")
	if err := os.MkdirAll(filepath.Dir(result.LogPath), 0o700); err != nil {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log parent: %v", err)
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	var output synchronizedBuffer
	var supervised supervisorOutcome
	var closeErr error
	var coverageMerge string
	var launchErr error
	if group.Adapter == "go" && len(credit.tests) != 0 && len(credit.residual) == 0 {
		// Every expected test already passed natively in this result, so the
		// group launches nothing and its log names the covering groups.
		output.Write([]byte(fmt.Sprintf("TEST-COVERED %s by %s: %d expected tests passed natively in this result\n",
			group.ID, strings.Join(credit.sources, ","), len(credit.tests))))
		closeErr = os.WriteFile(result.LogPath, output.Bytes(), 0o600)
	} else if group.Adapter == "go" {
		// The group's discovered tests run as concurrent go test launches
		// inside this one group; their outputs join in shard order, and
		// whole-package coverage is merged from the shards' coverage data.
		supervised, closeErr, coverageMerge, launchErr = runShardedGoGroup(ctx, request, group, cwd, environment, nativeExpected,
			credit.residualInventory(coverageInventory, coverageModule), coverageModule, limits, sampleInterval, result.LogPath, &output)
	} else {
		// The supervisor owns cancellation so it can census and terminate the
		// complete process tree. exec.CommandContext would kill the root first,
		// orphaning descendants before that census.
		command, commandErr := explicitEnvironmentCommand(context.WithoutCancel(ctx), cwd, environment, argv)
		if commandErr != nil {
			result.Status = "unavailable"
			result.NotRunReason = commandErr.Error()
			result.EndedAt, result.DurationMS = resultDuration(request, started)
			return result
		}
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		logFile, err := os.OpenFile(result.LogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log: %v", err)
			result.EndedAt, result.DurationMS = resultDuration(request, started)
			return result
		}
		activity := newOutputActivity(time.Now())
		tee := &activityWriter{activity: activity, writer: io.MultiWriter(&output, logFile)}
		command.Stdout, command.Stderr = tee, tee
		closeInherited, inheritErr := InheritHostResourceLease(command)
		if inheritErr != nil {
			result.Status, result.NotRunReason = "invalid", inheritErr.Error()
			result.EndedAt, result.DurationMS = resultDuration(request, started)
			_ = logFile.Close()
			return result
		}
		supervised = superviseCommand(command, supervisorOptions{Context: ctx, Limits: limits, SampleInterval: sampleInterval,
			Activity: activity, StageResultPath: sectionReport})
		closeInherited()
		closeErr = logFile.Close()
	}
	assignSupervisorOutcome(&result, supervised)
	if launchErr != nil {
		result.Status = "unavailable"
		result.NotRunReason = launchErr.Error()
		result.EndedAt, result.DurationMS = resultDuration(request, started)
		return result
	}
	exit := 0
	if result.NativeExitStatus != nil {
		exit = *result.NativeExitStatus
	}
	if closeErr != nil && result.Status == "" {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("close group log: %v", closeErr)
	}
	if supervised.Verdict != "" && supervised.Verdict != "cancelled" {
		result.Status = supervised.Verdict
		result.NotRunReason = supervised.Reason
		if supervised.Dump != "" {
			result.NotRunReason += "; " + supervised.Dump
		}
	} else if supervised.Verdict == "cancelled" {
		result.Status, result.NotRunReason = "cancelled", supervised.Reason
	}
	// A native nonzero status is retained above. It is not an adapter parse
	// error and must not overwrite the structured section/JUnit diagnostics.
	err = nil
	result.LogDigest = digestBytes(output.Bytes())
	rerunEligible := false
	if result.Status == "" {
		switch group.Adapter {
		case "go":
			result.Observed, result.Missing, result.Unexpected, result.CollectionComplete = parseGoJSON(output.Bytes(), nativeExpected)
			if len(credit.tests) != 0 {
				if len(credit.residual) == 0 {
					result.Observed, result.Missing, result.Unexpected, result.CollectionComplete = nil, nil, nil, true
				}
				result.Observed = append(result.Observed, credit.observed...)
				sortNative(result.Observed)
				result.CoveredByGroups, result.CoveredTests = credit.sources, credit.tests
			}
			if group.Coverage {
				allTests, _, _ := testpolicy.GoTests(group)
				if !allTests {
					result.Status = "invalid"
					result.CollectionComplete = false
					result.NotRunReason = "an explicit Go test-name subset cannot claim whole-package coverage floors"
				} else if violations, coverageErr := checkGroupCoverage(root, coverageModule, coverageInventory, coverageMerge); coverageErr != nil {
					result.Status = "invalid"
					result.CollectionComplete = false
					result.NotRunReason = coverageErr.Error()
				} else if len(violations) > 0 {
					result.Status = "failed"
					result.CollectionComplete = false
					result.NotRunReason = strings.Join(violations, "; ")
				}
			}
		case "command":
			if group.Format == "junit-xml" {
				result.Observed, result.Missing, result.Unexpected, result.CollectionComplete, result.ReportDigests, err = parseJUnit(root, group)
			} else {
				result.CollectionComplete = exit == 0
			}
		case "section":
			sectionStatus, reportedExit, sectionBlocked, sectionComplete := parseSectionResult(sectionReport, group.Section)
			result.Blocked, result.CollectionComplete = sectionBlocked, sectionComplete
			if reportedExit != exit {
				result.Status = "invalid"
				result.CollectionComplete = false
				result.NotRunReason = fmt.Sprintf("section result exit %d disagrees with native process exit %d", reportedExit, exit)
			} else {
				result.Status = sectionStatus
			}
		}
		if err != nil {
			result.Status = "invalid"
			result.NotRunReason = err.Error()
		}
		if result.Status == "" {
			evidenceFailed, evidenceSummary := nativeEvidenceSummaryForGroup(request, group, result.Observed)
			if exit != 0 || !result.CollectionComplete || evidenceFailed {
				result.Status = "failed"
				rerunEligible = group.Adapter == "go" && evidenceFailed
				if exit == 0 && !result.CollectionComplete {
					result.Status = "invalid"
				}
				if result.NotRunReason == "" {
					collection := "complete"
					if !result.CollectionComplete {
						collection = "incomplete"
					}
					switch {
					case evidenceSummary != "":
						result.NotRunReason = fmt.Sprintf("%s (process exit %d, collection %s)", evidenceSummary, exit, collection)
					case exit != 0:
						result.NotRunReason = fmt.Sprintf("process exit %d with no failing test in the evidence (collection %s)", exit, collection)
					default:
						result.NotRunReason = fmt.Sprintf("collection incomplete with no failing test in the evidence (process exit %d)", exit)
					}
				}
			} else {
				result.Status = "passed"
			}
		}
	}
	if rerunEligible {
		rerunFailedTests(ctx, request, group, cwd, environment, limits, sampleInterval, &result)
	}
	if afterDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicitInputs); digestErr != nil || afterDigest != inputDigest {
		result.Status = "invalid"
		result.CollectionComplete = false
		if digestErr != nil {
			result.NotRunReason = fmt.Sprintf("re-read group inputs: %v", digestErr)
		} else {
			result.NotRunReason = fmt.Sprintf("group inputs changed during execution: before %s, after %s", inputDigest, afterDigest)
		}
	}
	result.EndedAt, result.DurationMS = resultDuration(request, started)
	return result
}

func mergeInputManifest(declared, implicit []string) []string {
	seen := map[string]bool{}
	for _, path := range declared {
		seen[path] = true
	}
	for _, path := range implicit {
		if !seen[path] {
			seen[pathpattern.EncodeLiteral(path)] = true
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

// GroupExecutionIdentities performs bounded metadata/tool identity reads for
// the selected groups. It does not launch a declared test or build command.
func GroupExecutionIdentities(ctx context.Context, request TestRunRequest) (map[string]string, error) {
	identities, _, _, err := PrepareGroupExecutionIdentities(ctx, request)
	return identities, err
}

// RevalidateRetainedGroupExecutionIdentities rebuilds current group identity
// from metadata already committed by prior attempts. It re-hashes declared
// inputs and executable bytes, retaining version-command observations only
// while their declared closure is unchanged. It launches no tool, group
// command, or discovery process.
func RevalidateRetainedGroupExecutionIdentities(ctx context.Context, request TestRunRequest, attempts []Attempt) (map[string]string, error) {
	if err := ValidateTestWorkerRequest(request); err != nil {
		return nil, err
	}
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	expectedResultSchema := PreviousTestResultSchemaVersion
	if TestWorkerPolicyActive(request) {
		expectedResultSchema = TestResultSchemaVersion
	}
	detached, err := request.candidateWorkspace()
	if err != nil {
		return nil, err
	}
	defer detached.Close()
	root := detached.Workspace().Dir
	identities := make(map[string]string, len(request.Plan.SelectedGroups))
	for _, id := range request.Plan.SelectedGroups {
		group, ok := groups[id]
		if !ok {
			return nil, fmt.Errorf("selected testing group %s is absent", id)
		}
		environment := groupTestEnvironment(request, group)
		cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
		var newest *Attempt
		ambiguous := false
		for index := range attempts {
			attempt := attempts[index]
			// A later result for a changed definition cannot hide compatible
			// metadata retained by an earlier successful observation.
			if attempt.TestResult == nil || attempt.Terminal == nil {
				continue
			}
			result := attempt.TestResult
			if result.SchemaVersion != expectedResultSchema || result.JudgeKey != judgeKeyOf(request) || result.BehaviorPolicyDigest != request.BehaviorPolicyDigest {
				continue
			}
			for groupIndex := range result.Groups {
				source := &result.Groups[groupIndex]
				if source.ID != id || source.IdentityVersion != groupExecutionIdentityVersion(request) ||
					!source.CollectionComplete || (source.Status != "passed" && source.Status != "reused") ||
					len(source.ExecutableDigests) == 0 || len(source.InputManifest) == 0 {
					continue
				}
				toolMetadataComplete := len(source.ToolIdentities) >= len(group.Tools)
				for _, tool := range group.Tools {
					if source.ToolIdentities[tool.ID] == "" || source.ExecutableDigests[tool.ID] == "" {
						toolMetadataComplete = false
						break
					}
				}
				if !toolMetadataComplete {
					continue
				}
				declared := map[string]bool{}
				for _, path := range group.Inputs {
					declared[path] = true
				}
				var implicit []string
				manifestValid := true
				for _, entry := range source.InputManifest {
					path, literal, err := pathpattern.ManifestEntry(entry)
					if err != nil {
						manifestValid = false
						break
					}
					if literal || !declared[path] {
						implicit = append(implicit, path)
					}
				}
				if !manifestValid {
					continue
				}
				inputDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicit)
				if digestErr != nil || inputDigest != source.InputDigest || digestGroupEnvironment(group, environment) != source.EnvironmentDigest ||
					verifyPreparedExecutables(ctx, cwd, environment, group, source.Argv, source.ExecutableDigests) != nil {
					continue
				}
				identity := groupExecutionIdentity(request, group, cwd, inputDigest, source.EnvironmentDigest, source.ToolIdentities, source.Expected)
				if identity == source.ExecutionIdentity {
					selected, tied := newerAttempt(newest, attempt)
					if tied {
						ambiguous = true
						identities[id] = ""
					} else if newest == nil || selected.AttemptID == attempt.AttemptID {
						ambiguous = false
						identities[id] = identity
					}
					newest = selected
				}
			}
		}
		if ambiguous || identities[id] == "" {
			identities[id] = digestBytes([]byte("missing-retained-metadata\x00" + id + "\x00" + request.CandidateTree))
		}
	}
	return identities, ctx.Err()
}

// PrepareGroupExecutionIdentities collects bounded metadata once for the
// complete selection. It reports only helpers that actually started and uses
// one immutable candidate bed for all groups.
func PrepareGroupExecutionIdentities(ctx context.Context, request TestRunRequest) (map[string]string, map[string]PreparedGroupExecution, int, error) {
	if err := ValidateTestWorkerRequest(request); err != nil {
		return nil, nil, 0, err
	}
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	identities := make(map[string]string, len(request.Plan.SelectedGroups))
	prepared := make(map[string]PreparedGroupExecution, len(request.Plan.SelectedGroups))
	detached, err := request.candidateWorkspace()
	if err != nil {
		return nil, nil, 0, err
	}
	defer detached.Close()
	root := detached.Workspace().Dir
	launches := 0
	discoveryCache := &goDiscoveryCache{catalogs: map[string]goPackageCatalog{}}
	for _, id := range request.Plan.SelectedGroups {
		group, ok := groups[id]
		if !ok {
			return nil, nil, launches, fmt.Errorf("selected testing group %s is absent", id)
		}
		cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
		environment := groupTestEnvironment(request, group)
		toolIdentities, executables, argv, expected, implicitInputs, coverageInventory, coverageModule, count, availabilityErr := plannedToolIdentities(ctx, cwd, environment, group, root, discoveryCache, request.Contract.SchemaVersion)
		launches += count
		inputDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicitInputs)
		if digestErr != nil {
			return nil, nil, launches, digestErr
		}
		environmentDigest := digestGroupEnvironment(group, environment)
		identities[id] = groupExecutionIdentity(request, group, cwd, inputDigest, environmentDigest, toolIdentities, expected)
		item := PreparedGroupExecution{
			InputDigest: inputDigest, EnvironmentDigest: environmentDigest,
			ToolIdentities: toolIdentities, ExecutableDigests: executables, Argv: argv, Expected: expected,
			ImplicitInputs: implicitInputs, CoverageInventory: coverageInventory, CoverageModule: coverageModule}
		if TestWorkerPolicyActive(request) {
			item.WorkerPolicyVersion = TestWorkerPolicyVersion
			item.Workers, _ = EffectiveGroupWorkers(request, group)
		}
		if availabilityErr != nil {
			item.Unavailable = availabilityErr.Error()
		}
		prepared[id] = item
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, launches, err
	}
	return identities, prepared, launches, nil
}

func plannedToolIdentities(ctx context.Context, cwd string, environment []string, group testpolicy.Group, root string, discoveryCache *goDiscoveryCache, schemaVersion int) (map[string]string, map[string]string, []string, []NativeTestIdentity, []string, []string, string, int, error) {
	identities := map[string]string{}
	executables := map[string]string{}
	launches := 0
	var unavailable []error
	for _, tool := range group.Tools {
		identity, executableDigest, started, err := identifyTool(ctx, cwd, environment, tool)
		if started {
			launches++
		}
		if err != nil {
			identities[tool.ID] = unavailableToolIdentity("tool", tool.ID, tool.Executable)
			unavailable = append(unavailable, fmt.Errorf("tool %s: %w", tool.ID, err))
			continue
		}
		identities[tool.ID] = identity
		executables[tool.ID] = executableDigest
	}
	argv, expected, discovery, discoveryStarted, err := groupArgumentsForSchema(ctx, group, root, cwd, environment, discoveryCache, schemaVersion)
	if discoveryStarted {
		launches++
	}
	if err != nil || len(argv) == 0 {
		identities["__argv0__"] = unavailableToolIdentity("command", group.ID, "")
		if err == nil {
			err = fmt.Errorf("native command is empty")
		}
		return identities, executables, argv, expected, discovery.Inputs, discovery.Inventory, discovery.ModulePrefix, launches, errors.Join(append(unavailable, err)...)
	}
	command, err := explicitEnvironmentCommand(ctx, cwd, environment, argv)
	if err != nil {
		identities["__argv0__"] = unavailableToolIdentity("command", group.ID, argv[0])
		unavailable = append(unavailable, err)
	} else if identity, identityErr := executableIdentity(command.Path); identityErr != nil {
		identities["__argv0__"] = unavailableToolIdentity("command", group.ID, argv[0])
		unavailable = append(unavailable, identityErr)
	} else {
		identities["__argv0__"] = identity
		executables["__argv0__"] = identity
	}
	return identities, executables, argv, expected, discovery.Inputs, discovery.Inventory, discovery.ModulePrefix, launches, errors.Join(unavailable...)
}

func unavailableToolIdentity(kind, id, executable string) string {
	return digestBytes([]byte("unavailable\x00" + kind + "\x00" + id + "\x00" + executable))
}

func groupExecutionIdentity(request TestRunRequest, group testpolicy.Group, cwd, inputDigest, environmentDigest string, toolIdentities map[string]string, discovery []NativeTestIdentity) string {
	// The protected effective definition is already in request.Contract. The
	// obligations and supported platforms decide policy, not what is launched.
	definition := group
	definition.Obligations = nil
	definition.Platforms = nil
	definition.Requires = nil
	definition.Phase = ""
	sectionEngine := ""
	if group.Adapter == "section" {
		sectionEngine = request.CandidateEngineDigest
	}
	stewardEngine := ""
	if metaSystemStewardConsumesCandidateEngine(request, group, cwd) {
		stewardEngine = request.CandidateEngineDigest
	}
	if !TestWorkerPolicyActive(request) {
		identityBytes, _ := json.Marshal(struct {
			Version              int
			Group                testpolicy.Group
			Inputs               string
			Env                  string
			Tools                map[string]string
			Discovery            []NativeTestIdentity
			Platform             string
			JudgeKey             string
			BehaviorPolicyDigest string
			SectionEngineDigest  string
			StewardEngineDigest  string `json:",omitempty"`
			GoSkipVerdictPolicy  string `json:",omitempty"`
		}{PreviousGroupExecutionIdentityVersion, definition, inputDigest, environmentDigest, toolIdentities, discovery,
			runtime.GOOS + "/" + runtime.GOARCH, judgeKeyOf(request), request.BehaviorPolicyDigest, sectionEngine,
			stewardEngine, goSkipVerdictPolicy(request, group)})
		return digestBytes(identityBytes)
	}
	identityBytes, _ := json.Marshal(struct {
		Version              int
		WorkerPolicyVersion  int
		AttemptWorkers       int
		GroupWorkers         int
		Group                testpolicy.Group
		Inputs               string
		Env                  string
		Tools                map[string]string
		Discovery            []NativeTestIdentity
		Platform             string
		JudgeKey             string
		BehaviorPolicyDigest string
		SectionEngineDigest  string
		StewardEngineDigest  string `json:",omitempty"`
		GoSkipVerdictPolicy  string `json:",omitempty"`
	}{GroupExecutionIdentityVersion, TestWorkerPolicyVersion, EffectiveTestWorkers(request), mustEffectiveGroupWorkers(request, group),
		definition, inputDigest, environmentDigest, toolIdentities, discovery, runtime.GOOS + "/" + runtime.GOARCH,
		judgeKeyOf(request), request.BehaviorPolicyDigest, sectionEngine, stewardEngine, goSkipVerdictPolicy(request, group)})
	return digestBytes(identityBytes)
}

func mustEffectiveGroupWorkers(request TestRunRequest, group testpolicy.Group) int {
	workers, err := EffectiveGroupWorkers(request, group)
	if err != nil {
		return 0
	}
	return workers
}

func metaSystemStewardConsumesCandidateEngine(request TestRunRequest, group testpolicy.Group, cwd string) bool {
	if group.Adapter != "go" || filepath.Clean(group.CWD) != filepath.Clean(request.InstallationPrefix) {
		return false
	}
	moduleRoot, module, err := nearestGoModule(cwd)
	if err != nil || module != "github.com/widoriezebos/agentic-tools/metasystem" || canonicalGoPath(moduleRoot) != canonicalGoPath(cwd) {
		return false
	}
	for _, pkg := range group.Packages {
		if strings.TrimPrefix(filepath.ToSlash(filepath.Clean(pkg)), "./") == "internal/steward" {
			return true
		}
	}
	return false
}

func groupArguments(ctx context.Context, group testpolicy.Group, root, cwd string, environment []string, discoveryCache *goDiscoveryCache) ([]string, []NativeTestIdentity, goDiscovery, bool, error) {
	return groupArgumentsForSchema(ctx, group, root, cwd, environment, discoveryCache, testpolicy.SchemaVersion)
}

func groupArgumentsForSchema(ctx context.Context, group testpolicy.Group, root, cwd string, environment []string, discoveryCache *goDiscoveryCache, schemaVersion int) ([]string, []NativeTestIdentity, goDiscovery, bool, error) {
	switch group.Adapter {
	case "go":
		argv, expected, discovery, started, err := goArgumentsCachedForSchema(ctx, group, cwd, environment, discoveryCache, schemaVersion)
		if err == nil {
			moduleRoot, _, moduleErr := nearestGoModule(cwd)
			prefix, relErr := filepath.Rel(root, moduleRoot)
			if moduleErr != nil || relErr != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) {
				return argv, expected, discovery, started, fmt.Errorf("go module escapes candidate root")
			}
			for index, path := range discovery.Inputs {
				discovery.Inputs[index] = filepath.ToSlash(filepath.Join(prefix, filepath.FromSlash(path)))
			}
			if group.Coverage {
				discovery.Inputs = append(discovery.Inputs, coverageBaselineInputs()...)
				sort.Strings(discovery.Inputs)
			}
		}
		return argv, expected, discovery, started, err
	case "section":
		// Prepared argv survives the metadata worktree. Resolve the selector
		// under the actual group's cwd, never a removed preparation pathname.
		script := filepath.Join("scripts", "agents", "validate-section-selector.sh")
		return []string{"bash", script, "run", group.Section}, nil, goDiscovery{}, false, nil
	case "command":
		return append([]string(nil), group.Argv...), nil, goDiscovery{}, false, nil
	default:
		return nil, nil, goDiscovery{}, false, fmt.Errorf("unsupported test adapter %q", group.Adapter)
	}
}

func prepareSectionEngine(cwd, source, expected string) (string, error) {
	info, err := os.Lstat(source)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("section engine is not a regular executable: %v", err)
	}
	data, err := os.ReadFile(source)
	if err != nil || !validSHA256(expected) || digestBytes(data) != expected {
		return "", fmt.Errorf("section engine differs from its admitted executable bytes")
	}
	bin := filepath.Join(cwd, "bin")
	if info, err := os.Lstat(bin); err == nil && !info.IsDir() {
		return "", fmt.Errorf("section engine destination is not a real directory")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(cwd, "artifacts", "agents", "supervision"), 0o700); err != nil {
		return "", err
	}
	destination := filepath.Join(bin, "metasystem")
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
	if err != nil {
		return "", fmt.Errorf("section engine must not replace candidate-owned bytes: %w", err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	actual, err := os.ReadFile(destination)
	if err != nil || digestBytes(actual) != expected {
		return "", fmt.Errorf("section engine copy changed before launch")
	}
	return destination, nil
}

func nativeEvidenceSummaryForGroup(request TestRunRequest, group testpolicy.Group, observed []NativeTestIdentity) (failed bool, summary string) {
	return nativeEvidenceSummaryWithSkips(observed, goSkipVerdictPolicy(request, group) != "")
}

func goSkipVerdictPolicy(request TestRunRequest, group testpolicy.Group) string {
	if request.Contract.SchemaVersion != testpolicy.ExecutionContractSchemaVersion || group.Adapter != "go" {
		return ""
	}
	all, _, err := testpolicy.GoTests(group)
	if err == nil && all {
		return "go-full-package-native-skips-v1"
	}
	return ""
}

func nativeEvidenceSummaryWithSkips(observed []NativeTestIdentity, allowSkipped bool) (failed bool, summary string) {
	nonPassing := make([]string, 0, 5)
	nonPassingCount := 0
	failedCount := 0
	for _, test := range observed {
		if test.Status != "passed" && !(allowSkipped && test.Status == "skipped") {
			nonPassingCount++
			if test.Status == "failed" {
				failedCount++
			}
			if len(nonPassing) < 5 {
				nonPassing = append(nonPassing, fmt.Sprintf("%s.%s %s", test.Classname, test.Name, test.Status))
			}
		}
	}
	if nonPassingCount == 0 {
		return false, ""
	}
	prefix := ""
	if failedCount == 0 {
		prefix = "no test failed; "
	}
	return true, fmt.Sprintf("%s%d of %d observed tests did not pass: %s", prefix, nonPassingCount, len(observed), strings.Join(nonPassing, ", "))
}

func executableIdentity(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read command executable identity: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read command executable mode: %w", err)
	}
	return digestBytes(append([]byte(fmt.Sprintf("mode=%#o\x00", info.Mode().Perm())), data...)), nil
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func verifyPreparedExecutables(ctx context.Context, cwd string, environment []string, group testpolicy.Group, argv []string, expected map[string]string) error {
	for _, tool := range group.Tools {
		path, err := ResolveTestingExecutable(ctx, cwd, environment, []string{tool.Executable})
		if err != nil {
			return fmt.Errorf("tool %s unavailable after preparation: %w", tool.ID, err)
		}
		digest, err := executableIdentity(path)
		if err != nil || digest != expected[tool.ID] {
			return fmt.Errorf("tool %s executable changed after preparation", tool.ID)
		}
	}
	path, err := ResolveTestingExecutable(ctx, cwd, environment, argv)
	if err != nil {
		return err
	}
	digest, err := executableIdentity(path)
	if err != nil || digest != expected["__argv0__"] {
		return fmt.Errorf("native command executable changed after preparation")
	}
	return nil
}

func digestGroupInputs(root string, group testpolicy.Group, environment []string) (string, error) {
	return digestGroupInputsWithImplicit(root, group, environment, nil)
}

func digestGroupInputsWithImplicit(root string, group testpolicy.Group, environment, implicit []string) (string, error) {
	hash := sha256.New()
	// Retained manifests merge declared and discovered paths. Hash that same
	// input set during execution and read-only verification.
	declared := make(map[string]bool, len(group.Inputs))
	for _, path := range group.Inputs {
		declared[path] = true
	}
	unique := make(map[string]bool, len(implicit))
	for _, path := range implicit {
		if !declared[path] {
			unique[path] = true
		}
	}
	implicit = make([]string, 0, len(unique))
	for path := range unique {
		implicit = append(implicit, path)
	}
	sort.Strings(implicit)
	for _, relative := range implicit {
		if !filepath.IsLocal(relative) || relative == "." || filepath.ToSlash(filepath.Clean(relative)) != relative ||
			strings.ContainsRune(relative, '\x00') || strings.Contains(relative, `\`) {
			return "", fmt.Errorf("implicit group input %q is not an exact relative path", relative)
		}
		parent := root
		components := strings.Split(relative, "/")
		for _, component := range components[:len(components)-1] {
			parent = filepath.Join(parent, component)
			info, err := os.Lstat(parent)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return "", fmt.Errorf("read implicit group input %s: %w", relative, err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("implicit group input %s traverses symlink %s", relative, parent)
			}
		}
		hash.Write([]byte("implicit\x00" + relative + "\x00"))
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, statErr := os.Lstat(path)
		if os.IsNotExist(statErr) {
			hash.Write([]byte("absent\x00"))
			continue
		}
		if statErr != nil {
			return "", fmt.Errorf("read implicit group input %s: %w", relative, statErr)
		}
		item, err := inspectEntry(path, filepath.ToSlash(relative), fs.FileInfoToDirEntry(info))
		if err != nil {
			return "", fmt.Errorf("read implicit group input %s: %w", relative, err)
		}
		body := recordBody(item)
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(body)))
		hash.Write(length[:])
		hash.Write(body)
	}
	for _, declaration := range group.Inputs {
		pattern, err := pathpattern.Parse(declaration)
		if err != nil {
			return "", fmt.Errorf("group input %s: %w", declaration, err)
		}
		hash.Write([]byte("declaration\x00" + pattern.String() + "\x00"))
		matches, err := pattern.Expand(root)
		if err != nil {
			return "", fmt.Errorf("read group input %s: %w", declaration, err)
		}
		for _, relative := range matches {
			current := filepath.Join(root, filepath.FromSlash(relative))
			info, err := os.Lstat(current)
			if err != nil {
				return "", fmt.Errorf("read group input %s: %w", relative, err)
			}
			item, err := inspectEntry(current, relative, fs.FileInfoToDirEntry(info))
			if err != nil {
				return "", fmt.Errorf("read group input %s: %w", relative, err)
			}
			body := recordBody(item)
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(body)))
			hash.Write(length[:])
			hash.Write(body)
		}
		if len(matches) == 0 {
			hash.Write([]byte("absent\x00"))
		}
	}
	for _, external := range group.ExternalInputs {
		path, err := resolveExternalInput(external.Path, environment)
		if err != nil {
			return "", fmt.Errorf("external input %s: %w", external.ID, err)
		}
		hash.Write([]byte("external\x00" + external.ID + "\x00" + external.Path + "\x00"))
		var entries []entry
		walkErr := filepath.WalkDir(path, func(current string, directory os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) && current == path {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if directory.IsDir() {
				return nil
			}
			relative, relErr := filepath.Rel(path, current)
			if relErr != nil {
				return relErr
			}
			if relative == "." {
				relative = filepath.Base(path)
			}
			item, inspectErr := inspectEntry(current, "external/"+external.ID+"/"+filepath.ToSlash(relative), directory)
			if inspectErr != nil {
				return inspectErr
			}
			entries = append(entries, item)
			return nil
		})
		if walkErr != nil {
			return "", fmt.Errorf("read external input %s: %w", external.ID, walkErr)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
		for _, item := range entries {
			body := recordBody(item)
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(body)))
			hash.Write(length[:])
			hash.Write(body)
		}
		if len(entries) == 0 {
			hash.Write([]byte("absent\x00"))
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func resolveExternalInput(locator string, environment []string) (string, error) {
	if filepath.IsAbs(locator) {
		return locator, nil
	}
	if !strings.HasPrefix(locator, "${") {
		return "", fmt.Errorf("locator is neither absolute nor environment based")
	}
	end := strings.Index(locator, "}")
	if end < 3 {
		return "", fmt.Errorf("environment locator is malformed")
	}
	name := locator[2:end]
	value, found := environmentLookup(environment)(name)
	if !found || value == "" {
		return "", fmt.Errorf("environment variable %s is unset", name)
	}
	path := value
	if suffix := strings.TrimPrefix(locator[end+1:], "/"); suffix != "" {
		path = filepath.Join(value, filepath.FromSlash(suffix))
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("environment variable %s does not resolve to an absolute path", name)
	}
	return filepath.Clean(path), nil
}

func prepareGroupOutputs(root string, group testpolicy.Group) error {
	for _, declared := range group.Outputs {
		path := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(declared, "/**")))
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("output %s escapes private candidate", declared)
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("output %s is not empty at group start", declared)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func mergeTestEnvironment(base []string, additions map[string]string) []string {
	if len(base) == 0 {
		base = os.Environ()
	}
	return overlayTestEnvironment(base, additions)
}

func overlayTestEnvironment(base []string, additions map[string]string) []string {
	values := map[string]string{}
	order := []string{}
	for _, entry := range base {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			if _, seen := values[name]; !seen {
				order = append(order, name)
			}
			values[name] = value
		}
	}
	for name, value := range additions {
		if _, seen := values[name]; !seen {
			order = append(order, name)
		}
		values[name] = value
	}
	sort.Strings(order)
	result := make([]string, 0, len(order))
	for _, name := range order {
		result = append(result, name+"="+values[name])
	}
	return result
}

func groupTestEnvironment(request TestRunRequest, group testpolicy.Group) []string {
	var environment []string
	if group.EnvironmentMode == "explicit" {
		environment = []string{}
		names := make([]string, 0, len(group.Env))
		for name := range group.Env {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			environment = append(environment, name+"="+group.Env[name])
		}
	} else {
		environment = mergeTestEnvironment(request.Environment, group.Env)
	}
	if group.Adapter == "section" {
		environment = dropTestEnvironmentName(environment, "METASYSTEM_BIN")
	}
	if TestWorkerPolicyActive(request) {
		workers, err := EffectiveGroupWorkers(request, group)
		if err != nil {
			workers = 1
		}
		reserved := map[string]string{TestWorkersEnvironment: strconv.Itoa(workers)}
		if group.Adapter == "go" {
			reserved["GOMAXPROCS"] = "1"
		}
		environment = overlayTestEnvironment(environment, reserved)
	}
	if request.ControlRoot != "" && request.ProjectRoot != "" {
		environment = overlayTestEnvironment(environment, map[string]string{
			proofExecutionRootEnvironment: request.ProjectRoot,
		})
	}
	return environment
}

func dropTestEnvironmentName(environment []string, drop string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name != drop {
			result = append(result, entry)
		}
	}
	return result
}

// TestingEnvironment applies the same deterministic environment overlay used
// by execution. Readiness and verification must resolve executables from this
// exact environment instead of using the caller's ambient PATH differently.
func TestingEnvironment(base []string, additions map[string]string) []string {
	return mergeTestEnvironment(base, additions)
}

// GroupTestingEnvironment is the tool-readiness view of a group's actual
// execution environment, including its explicit or inherited mode.
func GroupTestingEnvironment(base []string, group testpolicy.Group) []string {
	return groupTestEnvironment(TestRunRequest{Environment: base}, group)
}

// ResolveTestingExecutable performs the execution owner's non-launching argv
// resolution. Absolute executables remain absolute; repository-relative argv
// is resolved beneath cwd and bare names use only the explicit PATH.
func ResolveTestingExecutable(ctx context.Context, cwd string, environment, argv []string) (string, error) {
	command, err := explicitEnvironmentCommand(ctx, cwd, environment, argv)
	if err != nil {
		return "", err
	}
	return command.Path, nil
}

func digestEnvironment(environment []string) string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		// Fixture custody values locate the run and attempt without changing a
		// test outcome, so binding them to the proof key would prevent reuse.
		case identity.RunOwnerEnv, identity.FixtureAttemptEnv:
			continue
		case "METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RECORD_KEY", "METASYSTEM_PROOF_CREATION_CLAIM", "METASYSTEM_PROOF_AUTH_BIN", proofExecutionRootEnvironment:
			continue
		// Where a build caches or writes its temporary files does not change
		// what a test does; binding the locations would key every identity to
		// one chain's cache or one round's scratch and defeat reuse across
		// rounds and seats' attempts (delegate-rounds-reuse-a-warm-gate).
		case "GOCACHE", "GOMODCACHE", "GOTMPDIR", "STATICCHECK_CACHE", "TMPDIR", "TMP", "TEMP":
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.Strings(filtered)
	return digestBytes([]byte(strings.Join(filtered, "\x00")))
}

func digestGroupEnvironment(group testpolicy.Group, environment []string) string {
	if group.EnvironmentMode != "explicit" {
		return digestEnvironment(environment)
	}
	ordered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != proofExecutionRootEnvironment {
			ordered = append(ordered, entry)
		}
	}
	sort.Strings(ordered)
	return digestBytes([]byte(strings.Join(ordered, "\x00")))
}
func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func identifyTool(ctx context.Context, cwd string, environment []string, tool testpolicy.Tool) (string, string, bool, error) {
	command, err := explicitEnvironmentCommand(ctx, cwd, environment, append([]string{tool.Executable}, tool.VersionArgs...))
	if err != nil {
		return "", "", false, fmt.Errorf("tool %s unavailable: %w", tool.ID, err)
	}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	err = RunResourceCommand(ctx, command, HostResourceLeaseFromContext(ctx))
	started := command.Process != nil
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", "", started, fmt.Errorf("tool %s version command: %w", tool.ID, ctxErr)
		}
		return "", "", started, fmt.Errorf("tool %s unavailable: %w", tool.ID, err)
	}
	executableDigest, err := executableIdentity(command.Path)
	if err != nil {
		return "", "", true, fmt.Errorf("tool %s executable identity: %w", tool.ID, err)
	}
	payload := append([]byte(executableDigest+"\x00"), output.Bytes()...)
	return digestBytes(payload), executableDigest, true, nil
}

func explicitEnvironmentCommand(ctx context.Context, cwd string, environment, argv []string) (*exec.Cmd, error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, fmt.Errorf("empty command")
	}
	executable := argv[0]
	if !filepath.IsAbs(executable) && strings.ContainsRune(executable, filepath.Separator) {
		executable = filepath.Join(cwd, executable)
	} else if !filepath.IsAbs(executable) {
		pathValue := ""
		for _, entry := range environment {
			name, value, ok := strings.Cut(entry, "=")
			if ok && name == "PATH" {
				pathValue = value
			}
		}
		resolved := ""
		for _, directory := range filepath.SplitList(pathValue) {
			if directory == "" {
				directory = cwd
			} else if !filepath.IsAbs(directory) {
				directory = filepath.Join(cwd, directory)
			}
			candidate := filepath.Join(directory, executable)
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				resolved = candidate
				break
			}
		}
		if resolved == "" {
			return nil, fmt.Errorf("executable %s is absent from the explicit PATH", argv[0])
		}
		executable = resolved
	}
	command := exec.CommandContext(ctx, executable, argv[1:]...)
	command.Args[0] = argv[0]
	command.Dir, command.Env = cwd, environment
	AttachHostResourceLease(ctx, command)
	return command, nil
}

// judgeKeyOf is the request's judge key, or the default when the caller
// computed none: a rebuild of an unchanged judge keeps every group identity,
// which the engine's file digest could not.
func judgeKeyOf(request TestRunRequest) string {
	if request.JudgeKey != "" {
		return request.JudgeKey
	}
	return DefaultJudgeKey()
}
