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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type TestRunRequest struct {
	ProjectRoot                  string
	InstallationPrefix           string
	CandidateTree                string
	BaseCommit                   string
	PolicyBaseCommit             string
	Contract                     testpolicy.Contract
	Plan                         testpolicy.Plan
	AttemptID                    string
	Environment                  []string
	LogRoot                      string
	ProgressPath                 string
	Reused                       map[string]GroupResult
	ContractDigest               string
	BaseContractDigest           string
	PolicyEngineDigest           string
	PolicyEngine                 string
	CandidateEngine              string
	CandidateEngineDigest        string
	CandidateEngineBuildIdentity string
	ControlRoot                  string
	BehaviorPolicyDigest         string
	ComponentIdentities          map[string]string
	PreparedGroups               map[string]PreparedGroupExecution
	CommandStartedAt             string
	PreparationLaunches          int
	PreparationDurationMS        int64
	EvidenceTimeoutMS            int64
	EvidenceMaxBytes             int64
}

// PreparedGroupExecution is immutable metadata collected once before
// admission and carried into the authenticated worker. Executable bytes are
// re-hashed there without repeating version helpers.
type PreparedGroupExecution struct {
	InputDigest       string               `json:"inputDigest"`
	EnvironmentDigest string               `json:"environmentDigest"`
	ToolIdentities    map[string]string    `json:"toolIdentities"`
	ExecutableDigests map[string]string    `json:"executableDigests"`
	Argv              []string             `json:"argv"`
	ImplicitInputs    []string             `json:"implicitInputs,omitempty"`
	CoverageInventory []string             `json:"coverageInventory,omitempty"`
	CoverageModule    string               `json:"coverageModule,omitempty"`
	Expected          []NativeTestIdentity `json:"expected"`
	Unavailable       string               `json:"unavailable,omitempty"`
}

func RunTestPlan(ctx context.Context, request TestRunRequest) (TestResult, int, error) {
	workerStarted := time.Now().UTC()
	started := workerStarted
	if parsed, err := time.Parse(time.RFC3339Nano, request.CommandStartedAt); err == nil && !parsed.After(started) {
		started = parsed
	}
	result := NewTestResult(request)
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	firstStatus := 0
	for _, stage := range request.Plan.Stages {
		for _, id := range stage.Groups {
			if reused, ok := request.Reused[id]; ok {
				if err := validateRetainedGroupReuse(request.ControlRoot, id, reused); err != nil {
					reused.Status, reused.CollectionComplete, reused.NativeLaunched = "invalid", false, false
					reused.NotRunReason, reused.ReuseAttempt = "forged or stale component reuse: "+err.Error(), ""
					result.Groups = append(result.Groups, reused)
					if firstStatus == 0 {
						firstStatus = 1
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
			if err := testGroupProgress(request.ProgressPath, id, "start", "", ""); err != nil {
				return result, 1, err
			}
			groupResult := runTestGroup(ctx, request, groups[id])
			result.Groups = append(result.Groups, groupResult)
			if err := testGroupProgress(request.ProgressPath, id, "end", groupResult.Status, groupResult.NotRunReason); err != nil {
				return result, 1, err
			}
			result.ChildDurationMS += groupResult.DurationMS
			result.LaunchCounts.Other += groupResult.OtherLaunches
			if groupResult.NativeLaunched {
				switch groups[id].Kind {
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
	}
	result.EndedAt, result.DurationMS = resultDuration(started)
	result.Cost.ActualDurationMS, result.Cost.ChildDurationMS = result.DurationMS, result.ChildDurationMS
	result.Cost.ExecutionDurationMS = time.Since(workerStarted).Milliseconds()
	result.Cost.ReusedLaunches = result.LaunchCounts.ReusedTest + result.LaunchCounts.ReusedBuild + result.LaunchCounts.ReusedOther
	result.RecomputeDelivery()
	if request.Plan.Purpose == testpolicy.PurposeDelivery && !result.Delivery.Sufficient && firstStatus == 0 {
		firstStatus = 1
	}
	return result, firstStatus, nil
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

func validateRetainedGroupReuse(root, id string, reused GroupResult) error {
	if root == "" || reused.ReuseAttempt == "" || reused.ID != id || reused.ExecutionIdentity == "" {
		return fmt.Errorf("component has no exact retained outer owner")
	}
	attempt, err := ReadAttempt(root, reused.ReuseAttempt)
	if err != nil || attempt.Terminal == nil || attempt.Terminal.Result != TerminalSuccess || attempt.TestResult == nil {
		return fmt.Errorf("outer attempt is not retained terminal success")
	}
	for _, recorded := range attempt.TestResult.Groups {
		if recorded.ID == id && recorded.ExecutionIdentity == reused.ExecutionIdentity && recorded.CollectionComplete &&
			(recorded.Status == "passed" || recorded.Status == "reused") {
			return nil
		}
	}
	return fmt.Errorf("outer attempt does not own matching complete group evidence")
}

func NewTestResult(request TestRunRequest) TestResult {
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
	return TestResult{SchemaVersion: TestResultSchemaVersion, AttemptID: request.AttemptID,
		Purpose: request.Plan.Purpose, RequestedMode: request.Plan.RequestedMode, RequiredMode: request.Plan.RequiredMode,
		ExecutedMode: request.Plan.ExecutedMode, ProjectRoot: request.ProjectRoot, InstallationPrefix: request.InstallationPrefix,
		BaseCommit: request.BaseCommit, CandidateTree: request.CandidateTree, PolicyBaseCommit: request.PolicyBaseCommit,
		ContractDigest: contractDigest, BaseContractDigest: baseContractDigest,
		PolicyEngineDigest: policyEngineDigest, CandidateEngineIdentityVersion: candidateEngineIdentityVersion,
		CandidateEngineDigest: request.CandidateEngineDigest, CandidateEngineBuildIdentity: request.CandidateEngineBuildIdentity,
		BehaviorPolicyDigest: behaviorPolicyDigest,
		PlanDigest:           TestPlanDigest(request.Contract, request.Plan, request.CandidateTree), Risk: request.Plan.Risk,
		RequiredGroups: append([]string(nil), request.Plan.RequiredGroups...), SelectedGroups: append([]string(nil), request.Plan.SelectedGroups...),
		Omissions: append([]testpolicy.Omission(nil), request.Plan.Omissions...), Uncertainty: append([]string(nil), request.Plan.Uncertainty...),
		LaunchCounts: LaunchCounts{Other: request.PreparationLaunches, CountsComplete: true}, StartedAt: request.CommandStartedAt,
		Cost: TestCost{DeclaredTargetMS: targetMS, PreparationDurationMS: request.PreparationDurationMS}}
}

func runTestGroup(ctx context.Context, request TestRunRequest, group testpolicy.Group) (result GroupResult) {
	started := time.Now().UTC()
	limits, sampleInterval := groupSupervisorSettings(group.CPUBudgetSeconds)
	result = GroupResult{ID: group.ID, Kind: group.Kind, Obligations: append([]string(nil), group.Obligations...),
		InputDigest: request.CandidateTree, InputManifest: append([]string(nil), group.Inputs...), CWD: group.CWD,
		ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}, StartedAt: started.Format(time.RFC3339Nano),
		ProgressRule: progressRule(limits)}
	detached, err := (gittree.Workspace{Dir: request.ProjectRoot}).NewDetachedWorktree(request.CandidateTree)
	if err != nil {
		result.Status = "invalid"
		result.NotRunReason = err.Error()
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	defer func() {
		timeout, maxBytes := time.Duration(request.EvidenceTimeoutMS)*time.Millisecond, request.EvidenceMaxBytes
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
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
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	result.InputDigest = inputDigest
	result.EnvironmentDigest = digestEnvironment(environment)
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
		plannedTools, _, argv, expected, implicitInputs, coverageInventory, coverageModule, launches, availabilityErr = plannedToolIdentities(ctx, cwd, environment, group, root, nil)
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
	result.ExecutionIdentity = groupExecutionIdentity(request, group, cwd, inputDigest, result.EnvironmentDigest, result.ToolIdentities, expected)
	if availabilityErr != nil {
		result.Status = "unavailable"
		result.NotRunReason = availabilityErr.Error()
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	if err := prepareGroupOutputs(root, group); err != nil {
		result.Status = "invalid"
		result.NotRunReason = err.Error()
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	sectionReport := ""
	switch group.Adapter {
	case "go":
		if metaSystemStewardConsumesCandidateEngine(request, group, cwd) {
			engine, err := prepareSectionEngine(cwd, request.CandidateEngine, request.CandidateEngineDigest)
			if err != nil {
				result.Status, result.NotRunReason = "invalid", err.Error()
				result.EndedAt, result.DurationMS = resultDuration(started)
				return result
			}
			environment = mergeTestEnvironment(environment, map[string]string{"METASYSTEM_BIN": engine})
		}
	case "section":
		sectionReport = filepath.Join(request.LogRoot, group.ID+".stage-results.tsv")
		environment = mergeTestEnvironment(environment, map[string]string{"METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT": sectionReport})
		if request.CandidateEngine != "" || request.CandidateEngineDigest != "" {
			engine, err := prepareSectionEngine(cwd, request.CandidateEngine, request.CandidateEngineDigest)
			if err != nil {
				result.Status, result.NotRunReason = "invalid", err.Error()
				result.EndedAt, result.DurationMS = resultDuration(started)
				return result
			}
			// Readiness follows the verified immutable input-bound artifact.
			// The shell still authenticates this worker's actual parent custody.
			environment = mergeTestEnvironment(environment, map[string]string{
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
			result.EndedAt, result.DurationMS = resultDuration(started)
			return result
		}
	}
	command, commandErr := explicitEnvironmentCommand(ctx, cwd, environment, argv)
	if commandErr != nil {
		result.Status = "unavailable"
		result.NotRunReason = commandErr.Error()
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	if err := os.MkdirAll(request.LogRoot, 0o700); err != nil {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log directory: %v", err)
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	result.LogPath = filepath.Join(request.LogRoot, group.ID+".log")
	if err := os.MkdirAll(filepath.Dir(result.LogPath), 0o700); err != nil {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log parent: %v", err)
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	logFile, err := os.OpenFile(result.LogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		result.Status, result.NotRunReason = "invalid", fmt.Sprintf("create group log: %v", err)
		result.EndedAt, result.DurationMS = resultDuration(started)
		return result
	}
	var output synchronizedBuffer
	activity := newOutputActivity(time.Now())
	tee := &activityWriter{activity: activity, writer: io.MultiWriter(&output, logFile)}
	command.Stdout, command.Stderr = tee, tee
	supervised := superviseCommand(command, supervisorOptions{Context: ctx, Limits: limits, SampleInterval: sampleInterval,
		Activity: activity, StageResultPath: sectionReport})
	closeErr := logFile.Close()
	if supervised.RuleSuffix != "" {
		result.ProgressRule += "+" + supervised.RuleSuffix
	}
	result.CPUSeconds = supervised.CPUSeconds
	result.LongestSilentSeconds = supervised.LongestSilentSeconds
	result.LongestZeroCPUSeconds = supervised.LongestZeroCPUSeconds
	result.NativeLaunched = supervised.Started
	err = supervised.WaitErr
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exit = exitErr.ExitCode()
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				value := status.Signal().String()
				result.Signal = &value
			}
		} else {
			result.Status = "unavailable"
			result.NotRunReason = err.Error()
			result.NativeLaunched = false
			result.NativeExitStatus = nil
		}
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
	if result.Status != "unavailable" {
		result.NativeExitStatus = &exit
	}
	// A native nonzero status is retained above. It is not an adapter parse
	// error and must not overwrite the structured section/JUnit diagnostics.
	err = nil
	result.LogDigest = digestBytes(output.Bytes())
	if result.Status == "" {
		switch group.Adapter {
		case "go":
			result.Observed, result.Missing, result.Unexpected, result.CollectionComplete = parseGoJSON(output.Bytes(), expected)
			if group.Coverage {
				allTests, _, _ := testpolicy.GoTests(group)
				if !allTests {
					result.Status = "invalid"
					result.CollectionComplete = false
					result.NotRunReason = "an explicit Go test-name subset cannot claim whole-package coverage floors"
				} else if violations, coverageErr := checkGroupCoverage(root, coverageModule, coverageInventory, output.String()); coverageErr != nil {
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
			if exit != 0 || !result.CollectionComplete || nativeEvidenceFailed(result.Observed) {
				result.Status = "failed"
				if exit == 0 && !result.CollectionComplete {
					result.Status = "invalid"
				}
			} else {
				result.Status = "passed"
			}
		}
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
	result.EndedAt, result.DurationMS = resultDuration(started)
	return result
}

func mergeInputManifest(declared, implicit []string) []string {
	seen := map[string]bool{}
	for _, path := range append(append([]string(nil), declared...), implicit...) {
		seen[path] = true
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
// inputs and executable bytes but never invokes version helpers, go list,
// tests, builds or dependency downloads.
func RevalidateRetainedGroupExecutionIdentities(ctx context.Context, request TestRunRequest, attempts []Attempt) (map[string]string, error) {
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	detached, err := (gittree.Workspace{Dir: request.ProjectRoot}).NewDetachedWorktree(request.CandidateTree)
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
		var source *GroupResult
		var newest *Attempt
		for index := range attempts {
			attempt := attempts[index]
			if attempt.TestResult == nil || attempt.Terminal == nil || attempt.Terminal.Result != TerminalSuccess {
				continue
			}
			result := attempt.TestResult
			if result.ContractDigest != request.ContractDigest || result.BaseContractDigest != request.BaseContractDigest ||
				result.PolicyEngineDigest != request.PolicyEngineDigest || result.BehaviorPolicyDigest != request.BehaviorPolicyDigest {
				continue
			}
			for groupIndex := range result.Groups {
				candidate := &result.Groups[groupIndex]
				if candidate.ID == id && candidate.CollectionComplete && (candidate.Status == "passed" || candidate.Status == "reused") &&
					(newest == nil || newerAttempt(newest, attempt).AttemptID == attempt.AttemptID) {
					copyGroup := *candidate
					source, newest = &copyGroup, newerAttempt(newest, attempt)
				}
			}
		}
		if source == nil || len(source.ExecutableDigests) == 0 || len(source.InputManifest) == 0 {
			identities[id] = digestBytes([]byte("missing-retained-metadata\x00" + id + "\x00" + request.CandidateTree))
			continue
		}
		declared := map[string]bool{}
		for _, path := range group.Inputs {
			declared[path] = true
		}
		var implicit []string
		for _, path := range source.InputManifest {
			if !declared[path] {
				implicit = append(implicit, path)
			}
		}
		environment := groupTestEnvironment(request, group)
		inputDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicit)
		cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
		if digestErr != nil || inputDigest != source.InputDigest || digestEnvironment(environment) != source.EnvironmentDigest ||
			verifyPreparedExecutables(ctx, cwd, environment, group, source.Argv, source.ExecutableDigests) != nil {
			identities[id] = digestBytes([]byte("changed-retained-metadata\x00" + id + "\x00" + request.CandidateTree))
			continue
		}
		identities[id] = groupExecutionIdentity(request, group, cwd, inputDigest, source.EnvironmentDigest, source.ToolIdentities, source.Expected)
	}
	return identities, ctx.Err()
}

// PrepareGroupExecutionIdentities collects bounded metadata once for the
// complete selection. It reports only helpers that actually started and uses
// one immutable candidate bed for all groups.
func PrepareGroupExecutionIdentities(ctx context.Context, request TestRunRequest) (map[string]string, map[string]PreparedGroupExecution, int, error) {
	groups := make(map[string]testpolicy.Group, len(request.Contract.Groups))
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	identities := make(map[string]string, len(request.Plan.SelectedGroups))
	prepared := make(map[string]PreparedGroupExecution, len(request.Plan.SelectedGroups))
	detached, err := (gittree.Workspace{Dir: request.ProjectRoot}).NewDetachedWorktree(request.CandidateTree)
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
		toolIdentities, executables, argv, expected, implicitInputs, coverageInventory, coverageModule, count, availabilityErr := plannedToolIdentities(ctx, cwd, environment, group, root, discoveryCache)
		launches += count
		inputDigest, digestErr := digestGroupInputsWithImplicit(root, group, environment, implicitInputs)
		if digestErr != nil {
			return nil, nil, launches, digestErr
		}
		environmentDigest := digestEnvironment(environment)
		identities[id] = groupExecutionIdentity(request, group, cwd, inputDigest, environmentDigest, toolIdentities, expected)
		item := PreparedGroupExecution{InputDigest: inputDigest, EnvironmentDigest: environmentDigest,
			ToolIdentities: toolIdentities, ExecutableDigests: executables, Argv: argv, Expected: expected,
			ImplicitInputs: implicitInputs, CoverageInventory: coverageInventory, CoverageModule: coverageModule}
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

func plannedToolIdentities(ctx context.Context, cwd string, environment []string, group testpolicy.Group, root string, discoveryCache *goDiscoveryCache) (map[string]string, map[string]string, []string, []NativeTestIdentity, []string, []string, string, int, error) {
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
	argv, expected, discovery, discoveryStarted, err := groupArguments(ctx, group, root, cwd, environment, discoveryCache)
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
	sectionEngine := ""
	if group.Adapter == "section" {
		sectionEngine = request.CandidateEngineDigest
	}
	stewardEngine := ""
	if metaSystemStewardConsumesCandidateEngine(request, group, cwd) {
		stewardEngine = request.CandidateEngineDigest
	}
	identityBytes, _ := json.Marshal(struct {
		Group                testpolicy.Group
		Inputs               string
		Env                  string
		Tools                map[string]string
		Discovery            []NativeTestIdentity
		Platform             string
		ContractDigest       string
		BaseContractDigest   string
		PolicyEngineDigest   string
		BehaviorPolicyDigest string
		SectionEngineDigest  string
		StewardEngineDigest  string `json:",omitempty"`
	}{group, inputDigest, environmentDigest, toolIdentities, discovery, runtime.GOOS + "/" + runtime.GOARCH,
		request.ContractDigest, request.BaseContractDigest, request.PolicyEngineDigest, request.BehaviorPolicyDigest, sectionEngine, stewardEngine})
	return digestBytes(identityBytes)
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
	switch group.Adapter {
	case "go":
		argv, expected, discovery, started, err := goArgumentsCached(ctx, group, cwd, environment, discoveryCache)
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

func nativeEvidenceFailed(observed []NativeTestIdentity) bool {
	for _, test := range observed {
		if test.Status != "passed" {
			return true
		}
	}
	return false
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
		hash.Write([]byte("declaration\x00" + declaration + "\x00"))
		plain := strings.TrimSuffix(declaration, "/**")
		path := filepath.Join(root, filepath.FromSlash(plain))
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("group input %s escapes candidate root", declaration)
		}
		var entries []entry
		walkErr := filepath.WalkDir(path, func(current string, directory os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) && current == path {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			relative, relErr := filepath.Rel(root, current)
			if relErr != nil {
				return relErr
			}
			relative = filepath.ToSlash(relative)
			if relative == ".git" || strings.HasPrefix(relative, ".git/") {
				if directory.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if directory.IsDir() {
				return nil
			}
			item, inspectErr := inspectEntry(current, relative, directory)
			if inspectErr != nil {
				return inspectErr
			}
			entries = append(entries, item)
			return nil
		})
		if walkErr != nil {
			return "", fmt.Errorf("read group input %s: %w", declaration, walkErr)
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
	environment := mergeTestEnvironment(request.Environment, group.Env)
	if group.Adapter == "section" {
		environment = dropTestEnvironmentName(environment, "METASYSTEM_BIN")
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
		case "METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RECORD_KEY", "METASYSTEM_PROOF_CREATION_CLAIM", "METASYSTEM_PROOF_AUTH_BIN":
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.Strings(filtered)
	return digestBytes([]byte(strings.Join(filtered, "\x00")))
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
	if err := command.Start(); err != nil {
		return "", "", false, fmt.Errorf("tool %s unavailable: %w", tool.ID, err)
	}
	err = command.Wait()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", "", true, fmt.Errorf("tool %s version command: %w", tool.ID, ctxErr)
		}
		return "", "", true, fmt.Errorf("tool %s unavailable: %w", tool.ID, err)
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
	return command, nil
}
