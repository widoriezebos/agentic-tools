package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// NamedGroupResult describes a native run without granting proof authority.
type NamedGroupResult struct {
	ID         string
	Status     string
	DurationMS int64
	Reasons    []string
	// Output is empty for green groups; red groups keep the last 200 diagnostic lines.
	Output string
}

// RunNamedGroups runs concrete groups and their prerequisites in a plain
// installation. Logs are temporary diagnostics, never retained proof records.
// Selection is validated in full before discovery or any native command runs.
func RunNamedGroups(ctx context.Context, installation string, contract testpolicy.Contract, ids, environment []string, progress ...func(string, int, []PackageExecution)) ([]NamedGroupResult, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no testing group ids supplied")
	}
	stages := make([]testpolicy.Stage, len(ids))
	for i, id := range ids {
		stages[i] = testpolicy.Stage{ID: id, Groups: []string{id}}
	}
	plan, err := testpolicy.WithPrerequisiteClosure(contract, testpolicy.Plan{SelectedGroups: ids, Stages: stages})
	if err != nil {
		return nil, err
	}
	groups := make(map[string]testpolicy.Group, len(contract.Groups))
	for _, group := range contract.Groups {
		groups[group.ID] = group
	}
	for _, id := range plan.SelectedGroups {
		if groups[id].PackageSelection != "" {
			return nil, fmt.Errorf("testing group %s is a package-selection template", id)
		}
	}
	root, err := filepath.Abs(installation)
	if err != nil {
		return nil, err
	}
	// Template contracts use paths relative to the directory containing the
	// installation. Adopted contracts use the installation itself.
	if filepath.Base(root) == "metasystem" && config.TemplateMode(root) {
		root = filepath.Dir(root)
	}
	if len(environment) == 0 {
		environment = os.Environ()
	}
	workers := runtime.GOMAXPROCS(0)
	for _, name := range []string{"METASYSTEM_TESTING_WORKERS", TestWorkersEnvironment} {
		for _, entry := range environment {
			if value, found := strings.CutPrefix(entry, name+"="); found && value != "" {
				allowance, err := strconv.Atoi(value)
				if err != nil || allowance < 1 {
					return nil, fmt.Errorf("%s must be a positive integer", name)
				}
				if name == "METASYSTEM_TESTING_WORKERS" || allowance < workers {
					workers = allowance
				}
			}
		}
	}
	logRoot, release, err := diskstore.ScratchDir("metasystem-named-groups-")
	if err != nil {
		return nil, err
	}
	defer release()
	request := TestRunRequest{Workers: workers, Environment: environment, LogRoot: logRoot}
	ctx = withTestWorkerPool(ctx, workers)
	cache := &goDiscoveryCache{}
	var results []NamedGroupResult
	known := map[string]string{}
	var run func(string) error
	run = func(id string) error {
		if _, done := known[id]; done {
			return nil
		}
		group := groups[id]
		groupRequest := request
		groupRequest.nativeProgress = nil
		if len(progress) > 0 && progress[0] != nil {
			groupRequest.nativeProgress = func(planned int, completed []PackageExecution) { progress[0](id, planned, completed) }
			if group.Adapter != "go" {
				groupRequest.nativeProgress(1, nil)
			}
		}
		result := NamedGroupResult{ID: id, Status: "red"}
		for _, dependency := range group.Requires {
			if err := run(dependency); err != nil {
				return err
			}
			if known[dependency] != "green" {
				result.Reasons = append(result.Reasons, "prerequisite "+dependency+" is red; not run")
			}
		}
		if len(result.Reasons) == 0 {
			var err error
			result, err = runNamedGroup(ctx, groupRequest, root, group, contract.SchemaVersion, cache)
			if err != nil {
				return fmt.Errorf("testing group %s: %w", id, err)
			}
		}
		if groupRequest.nativeProgress != nil && (group.Adapter != "go" || len(result.Reasons) > 0 && strings.Contains(result.Reasons[0], "not run")) {
			ms, status := result.DurationMS, "ok"
			if result.Status != "green" {
				status = "failed"
			}
			if group.Adapter == "go" {
				groupRequest.nativeProgress(1, nil)
			}
			groupRequest.nativeProgress(0, []PackageExecution{{Package: id, Shard: 0, Status: status, ElapsedMS: &ms}})
		}
		known[id] = result.Status
		results = append(results, result)
		return nil
	}
	for _, id := range ids {
		if err := run(id); err != nil {
			return results, err
		}
	}
	return results, nil
}

func runNamedGroup(ctx context.Context, request TestRunRequest, root string, group testpolicy.Group, schema int, cache *goDiscoveryCache) (NamedGroupResult, error) {
	started := time.Now()
	result := NamedGroupResult{ID: group.ID, Status: "red"}
	// The proof command runs in a fresh snapshot, so no stale declared output
	// exists and prepareGroupOutputs is unnecessary here.
	group.Coverage = false
	cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
	environment := groupTestEnvironment(request, group)
	argv, expected, discovery, _, err := groupArgumentsForSchema(ctx, group, root, cwd, environment, cache, schema)
	if err != nil {
		return result, err
	}
	limits, interval := groupSupervisorSettings(group.CPUBudgetSeconds)
	var output synchronizedBuffer
	var outcome supervisorOutcome
	if group.Adapter == "go" {
		logPath := filepath.Join(request.LogRoot, group.ID+".log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
			return result, err
		}
		var closeErr, launchErr error
		outcome, closeErr, _, _, launchErr = runShardedGoGroup(ctx, request, group, cwd, environment, expected,
			discovery.Inventory, discovery.ModulePrefix, limits, interval, logPath, &output, goCacheFacts{})
		if launchErr != nil || closeErr != nil {
			return result, fmt.Errorf("native Go run: launch %v; close %v", launchErr, closeErr)
		}
	} else {
		command, err := explicitEnvironmentCommand(context.WithoutCancel(ctx), cwd, environment, argv)
		if err != nil {
			return result, err
		}
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		activity := newOutputActivity(time.Now())
		tee := &activityWriter{activity: activity, writer: &output}
		command.Stdout, command.Stderr = tee, tee
		closeInherited, err := InheritHostResourceLease(command)
		if err != nil {
			return result, err
		}
		outcome = superviseCommand(command, supervisorOptions{Context: ctx, Limits: limits, SampleInterval: interval, Activity: activity})
		closeInherited()
	}
	var native GroupResult
	assignSupervisorOutcome(&native, outcome)
	if outcome.Verdict != "" {
		result.Reasons = append(result.Reasons, outcome.Verdict+": "+outcome.Reason)
	}
	if native.NativeExitStatus != nil && *native.NativeExitStatus != 0 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("exit status %d", *native.NativeExitStatus))
	} else if outcome.WaitErr != nil {
		result.Reasons = append(result.Reasons, outcome.WaitErr.Error())
	}
	complete := true
	var observed, missing, unexpected []NativeTestIdentity
	switch {
	case group.Adapter == "go":
		observed, missing, unexpected, complete = parseGoJSON(output.Bytes(), expected)
	case group.Adapter == "command" && group.Format == "junit-xml":
		observed, missing, unexpected, complete, _, err = parseJUnit(root, group)
		if err != nil {
			return result, err
		}
	}
	for _, test := range missing {
		name := test.Name
		if test.Classname != "" {
			name = test.Classname + "." + name
		}
		result.Reasons = append(result.Reasons, "missing test "+name)
	}
	for _, test := range unexpected {
		result.Reasons = append(result.Reasons, "unexpected test "+test.Classname+"."+test.Name)
	}
	for _, test := range observed {
		if test.Status == "failed" {
			result.Reasons = append(result.Reasons, "failed test "+test.Classname+"."+test.Name)
		}
	}
	if group.Adapter == "command" {
		if failed, summary := nativeEvidenceSummaryForGroup(request, group, observed); failed {
			result.Reasons = append(result.Reasons, summary)
		}
	}
	if !complete && len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "native test collection incomplete")
	}
	if len(result.Reasons) == 0 {
		result.Status = "green"
	} else if group.Adapter == "go" {
		// The gate's diagnostic reruns never turn an initial failure green.
		diagnostic := GroupResult{ID: group.ID, Status: "failed", Observed: observed}
		rerunFailedTests(ctx, request, group, cwd, environment, limits, interval, &diagnostic)
		for _, rerun := range diagnostic.Reruns {
			result.Reasons = append(result.Reasons, "diagnostic rerun "+rerun.Package+"."+rerun.Test+": "+rerun.Second)
		}
	}
	if result.Status == "red" {
		result.Output = string(output.Bytes())
		if group.Adapter == "go" {
			result.Output = namedGroupGoOutput(output.Bytes())
		}
		result.Output = namedGroupOutputTail(result.Output)
	}
	result.DurationMS = time.Since(started).Milliseconds()
	return result, nil
}

func namedGroupGoOutput(encoded []byte) string {
	type diagnosticEvent struct {
		goEvent
		ImportPath  string
		FailedBuild string
	}
	lines := strings.SplitAfter(string(encoded), "\n")
	failed := map[string]bool{}
	builds := map[string]bool{}
	for _, line := range lines {
		var event diagnosticEvent
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Action == "fail" && event.Test != "" {
			failed[event.Package+"\x00"+event.Test] = true
		}
		if event.Action == "build-fail" {
			builds[event.ImportPath] = true
		}
		if event.FailedBuild != "" {
			builds[event.FailedBuild] = true
			builds[event.Package] = true
		}
		if event.Test == "" && strings.Contains(event.Output, "[build failed]") {
			builds[event.Package] = true
		}
	}
	var plain strings.Builder
	for _, line := range lines {
		var event diagnosticEvent
		if json.Unmarshal([]byte(line), &event) != nil {
			// Older Go toolchains write build diagnostics directly to stderr.
			plain.WriteString(line)
		} else if failed[event.Package+"\x00"+event.Test] || builds[event.ImportPath] || event.Test == "" && builds[event.Package] {
			plain.WriteString(event.Output)
		}
	}
	return plain.String()
}

func namedGroupOutputTail(output string) string {
	lines := strings.SplitAfter(output, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= 200 {
		return output
	}
	return fmt.Sprintf("[%d lines omitted]\n", len(lines)-200) + strings.Join(lines[len(lines)-200:], "")
}

// LandingEnvironment describes the toolchain and kernel without timestamps,
// host names or installation paths, for comparison by the proof command.
func LandingEnvironment(ctx context.Context, root string, environment []string) (string, error) {
	if len(environment) == 0 {
		environment = os.Environ()
	}
	read := func(argv ...string) ([]byte, error) {
		command, err := explicitEnvironmentCommand(ctx, root, environment, argv)
		if err != nil {
			return nil, err
		}
		return command.Output()
	}
	version, err := read("go", "version")
	if err != nil {
		return "", fmt.Errorf("read go version: %w", err)
	}
	names := []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT", "GOTOOLCHAIN"}
	values, err := read(append([]string{"go", "env", "-json"}, names...)...)
	if err != nil {
		return "", fmt.Errorf("read go environment: %w", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(values, &decoded); err != nil {
		return "", err
	}
	parts := []string{strings.Join(strings.Fields(string(version)), " ")}
	for _, name := range names {
		parts = append(parts, name+"="+strconv.Quote(decoded[name]))
	}
	if kernel, err := read("uname", "-sr"); err == nil {
		parts = append(parts, strings.Join(strings.Fields(string(kernel)), " "))
	}
	return strings.Join(parts, "; "), nil
}
