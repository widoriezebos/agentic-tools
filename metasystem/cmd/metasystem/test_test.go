package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestTestPlanReArmsOnALandedEngine(t *testing.T) {
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	called, rearm := 0, false
	prepareTestingForCommand = func(request testingSelectionRequest) (testingPreparation, error) {
		called++
		rearm = request.LandedRearm
		return testingPreparation{}, errors.New("stop after observing preparation")
	}
	status, _, _ := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", t.TempDir(), "--purpose", "diagnostic"})
	})
	if status != 1 || called != 1 || !rearm {
		t.Fatalf("outer test plan did not enter landed re-arm: status=%d calls=%d rearm=%t", status, called, rearm)
	}
}

func TestTestRunRearmsOnALandedEngine(t *testing.T) {
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	called, rearm := 0, false
	prepareTestingForCommand = func(request testingSelectionRequest) (testingPreparation, error) {
		called++
		rearm = request.LandedRearm
		return testingPreparation{}, errors.New("stop after observing preparation")
	}
	status, _, _ := captureCommandOutput(t, true, true, func() int {
		return runTestRun([]string{"--root", t.TempDir(), "--purpose", "diagnostic"})
	})
	if status != 1 || called != 1 || !rearm {
		t.Fatalf("outer test run did not enter landed re-arm: status=%d calls=%d rearm=%t", status, called, rearm)
	}
}

func TestTestingCommandAdmissionSamplesAfterPreparationAndAtForcedFallback(t *testing.T) {
	t.Parallel()
	semanticCommandStarted := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	afterPreparation := semanticCommandStarted.Add(2 * time.Minute)
	afterFallbackDecision := afterPreparation.Add(time.Minute)

	t.Run("advancing production clock", func(t *testing.T) {
		clockSamples := make(chan time.Time, 2)
		observed := make(chan proofLaunchAdmission, 2)
		preparationStarted := make(chan struct{})
		preparationReleased := make(chan struct{})
		fallbackReleased := make(chan struct{})
		done := make(chan struct{})
		owner := testingCommandAdmission{
			now: func() time.Time { return <-clockSamples },
			admitRun: func(_ testingSelectionRequest, admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
				observed <- admission
				return proofrun.Attempt{}, proofrun.LaunchResult{Disposition: proofrun.DispositionReusableSuccess}, false, nil
			},
			admitProof: func(admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
				observed <- admission
				return proofrun.Attempt{}, proofrun.LaunchResult{Disposition: proofrun.DispositionExecuted}, false, nil
			},
		}
		go func() {
			defer close(done)
			close(preparationStarted)
			<-preparationReleased
			_, _, _, _ = owner.initial(testingSelectionRequest{}, proofLaunchAdmission{Now: semanticCommandStarted})
			<-fallbackReleased
			_, _, _, _ = owner.forced(proofLaunchAdmission{Now: semanticCommandStarted})
		}()

		<-preparationStarted
		clockSamples <- afterPreparation
		close(preparationReleased)
		initial := <-observed
		clockSamples <- afterFallbackDecision
		close(fallbackReleased)
		fallback := <-observed
		<-done
		if initial.Now != afterPreparation || fallback.Now != afterFallbackDecision {
			t.Fatalf("admission instants initial=%s fallback=%s, want %s then %s", initial.Now, fallback.Now, afterPreparation, afterFallbackDecision)
		}
		if !fallback.ForceAttempt || !fallback.ForceGroups {
			t.Fatalf("forced fallback flags = attempt:%t groups:%t", fallback.ForceAttempt, fallback.ForceGroups)
		}
	})

	t.Run("fixed authorized fixture clock", func(t *testing.T) {
		observed := make(chan proofLaunchAdmission, 2)
		owner := testingCommandAdmission{
			now: func() time.Time { return semanticCommandStarted },
			admitRun: func(_ testingSelectionRequest, admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
				observed <- admission
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, nil
			},
			admitProof: func(admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
				observed <- admission
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, nil
			},
		}
		_, _, _, _ = owner.initial(testingSelectionRequest{}, proofLaunchAdmission{})
		_, _, _, _ = owner.forced(proofLaunchAdmission{})
		if initial, fallback := (<-observed).Now, (<-observed).Now; initial != semanticCommandStarted || fallback != semanticCommandStarted {
			t.Fatalf("fixed fixture admission instants initial=%s fallback=%s, want both %s", initial, fallback, semanticCommandStarted)
		}
	})
}

func TestVerifySamplesFreshnessAfterRetainedProofRevalidation(t *testing.T) {
	fixture := newPortableProofFixture(t)
	fixture.writeEpisodeContract()
	tree := fixture.commit("declare expiring command observation")
	before := time.Now().UTC().Truncate(time.Second)
	expires := before.Add(time.Minute)
	t.Setenv("METASYSTEM_GOAL_NOW", before.Format(time.RFC3339Nano))
	request := testingSelectionRequest{Root: fixture.root, GoalID: "portable", Tree: tree, Mode: testpolicy.ModeAuto,
		Purpose: testpolicy.PurposeDelivery, FreshEpisode: strings.Repeat("1", 64), FreshExpiresAt: expires.Format(time.RFC3339Nano)}
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTestingPreparationWorkerPolicy(&prepared); err != nil {
		t.Fatal(err)
	}
	buildIdentity, err := candidateEngineBuildIdentity(context.Background(), gittree.Workspace{Dir: prepared.ProjectRoot},
		prepared.Prefix, prepared.CandidateTree, prepared.Environment)
	if err != nil {
		t.Fatal(err)
	}
	runRequest := testingRunRequest(prepared, "", "", "", strings.Repeat("a", 64), buildIdentity)
	runRequest.FreshnessEpisode, runRequest.FreshnessExpiresAt = request.FreshEpisode, request.FreshExpiresAt
	runRequest.FreshGroups, _ = testingFreshGroups(prepared, request)
	if err := bindTestingFreshnessProjection(&runRequest, prepared.Installation); err != nil {
		t.Fatal(err)
	}
	identities, metadata, _, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), runRequest)
	if err != nil {
		t.Fatal(err)
	}
	runRequest.FreshnessBinding = testingFreshnessBinding(runRequest, identities, request.FreshEpisode)
	result := proofrun.NewTestResultAt(runRequest, before)
	zero := 0
	for _, id := range prepared.Plan.SelectedGroups {
		var definition testpolicy.Group
		for _, group := range prepared.EffectiveContract.Groups {
			if group.ID == id {
				definition = group
			}
		}
		item := metadata[id]
		result.Groups = append(result.Groups, proofrun.GroupResult{ID: id, Kind: definition.Kind, Obligations: definition.Obligations,
			InputDigest: item.InputDigest, InputManifest: append(append([]string(nil), definition.Inputs...), item.ImplicitInputs...),
			ExecutionIdentity: identities[id], IdentityVersion: proofrun.GroupExecutionIdentityVersion, Argv: item.Argv, CWD: definition.CWD,
			EnvironmentDigest: item.EnvironmentDigest, ToolIdentities: item.ToolIdentities, ExecutableDigests: item.ExecutableDigests,
			Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, Expected: item.Expected, Observed: item.Expected,
			CollectionComplete: true, ReportDigests: map[string]string{}, StartedAt: before.Format(time.RFC3339Nano), EndedAt: before.Format(time.RFC3339Nano)})
		result.LaunchCounts.Test++
	}
	result.RecomputeDelivery()
	proofIdentity, err := proofrun.BuildProofIdentity(prepared.ProjectRoot, prepared.ConfPath, "selected", "testing", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: prepared.proofControlRoot(), ExecutionRoot: prepared.ProjectRoot, CandidateTree: prepared.CandidateTree,
		GoalID: prepared.GoalID, GoalRevision: prepared.AccountingRevision, AccountingRevision: prepared.AccountingRevision,
		CandidateGoalID: prepared.GoalID, CandidateRevision: prepared.AccountingRevision, ReservedMinutes: 2,
		Identity: proofIdentity, Launcher: launcher, Now: before, ComponentIdentities: identities, SharedComponents: true,
		FreshnessEpisode: request.FreshEpisode, FreshnessBinding: runRequest.FreshnessBinding,
		FreshnessExpiresAt: request.FreshExpiresAt, FreshGroups: runRequest.FreshGroups,
	}, "0")))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("reserve retained proof: decision=%+v err=%v", decision, err)
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(prepared.proofControlRoot(), attempt.AttemptID,
		proofrun.TerminalSuccess, 0, "controlled retained proof", nil, &result, before.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	fixed, err := verifyRetainedTesting(request)
	if err != nil || !fixed.Delivery.Sufficient {
		t.Fatalf("authorized fixed fixture clock lost reusable proof: sufficient=%t err=%v groups=%+v", fixed.Delivery.Sufficient, err, fixed.Groups)
	}
	for _, boundary := range []struct {
		name       string
		at         time.Time
		sufficient bool
	}{
		{name: "before expiry", at: expires.Add(-time.Nanosecond), sufficient: true},
		{name: "at expiry", at: expires},
		{name: "after expiry", at: expires.Add(time.Nanosecond)},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			revalidated := false
			result, err := verifyRetainedTestingPrepared(request, prepared, retainedTestingVerification{
				clock: func() time.Time {
					if revalidated {
						return boundary.at
					}
					return before
				},
				revalidate: func(ctx context.Context, request proofrun.TestRunRequest, attempts []proofrun.Attempt) (map[string]string, error) {
					identities, err := proofrun.RevalidateRetainedGroupExecutionIdentities(ctx, request, attempts)
					revalidated = err == nil
					return identities, err
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !revalidated {
				t.Fatal("retained group identities were not revalidated")
			}
			if result.Delivery.Sufficient != boundary.sufficient {
				t.Fatalf("proof sufficient at %s = %t, want %t; groups=%+v", boundary.at, result.Delivery.Sufficient, boundary.sufficient, result.Groups)
			}
		})
	}
}

func TestTestRunKeepsProofRecordsUnderTheControlRoot(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	source, err := os.ReadFile("test.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(fset, "test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "runTestRun" {
			body = function.Body
			break
		}
	}
	if body == nil {
		t.Fatal("runTestRun was not found")
	}
	expressionText := func(expression ast.Expr) string {
		start, end := fset.Position(expression.Pos()).Offset, fset.Position(expression.End()).Offset
		if start < 0 || end < start || end > len(source) {
			return fmt.Sprintf("%T", expression)
		}
		return string(source[start:end])
	}
	isIdentifier := func(expression ast.Expr, name string) bool {
		identifier, ok := expression.(*ast.Ident)
		return ok && identifier.Name == name
	}
	callName := func(call *ast.CallExpr) string {
		switch function := call.Fun.(type) {
		case *ast.Ident:
			return function.Name
		case *ast.SelectorExpr:
			if qualifier, ok := function.X.(*ast.Ident); ok {
				return qualifier.Name + "." + function.Sel.Name
			}
		}
		return ""
	}
	found := map[string]int{}
	wantFirstArgument := map[string]bool{
		"proofrun.ReadAttempts":        true,
		"publishTestingResult":         true,
		"retainIncompleteProofAttempt": true,
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				identifier, ok := left.(*ast.Ident)
				if !ok || index >= len(typed.Rhs) {
					continue
				}
				right := typed.Rhs[index]
				switch identifier.Name {
				case "controlRoot":
					found["controlRoot assignment"]++
					call, ok := right.(*ast.CallExpr)
					var selector *ast.SelectorExpr
					if ok {
						selector, ok = call.Fun.(*ast.SelectorExpr)
					}
					if !ok || selector.Sel.Name != "proofControlRoot" || !isIdentifier(selector.X, "prepared") {
						t.Errorf("controlRoot assignment takes %s instead of prepared.proofControlRoot()", expressionText(right))
					}
				case "pathsRoot":
					found["pathsRoot filepath.Join"]++
					call, ok := right.(*ast.CallExpr)
					if !ok || callName(call) != "filepath.Join" || len(call.Args) == 0 || !isIdentifier(call.Args[0], "controlRoot") {
						t.Errorf("pathsRoot filepath.Join takes %s first instead of controlRoot", expressionText(right))
					}
				}
			}
		case *ast.CompositeLit:
			site := ""
			switch literalType := typed.Type.(type) {
			case *ast.Ident:
				if literalType.Name == "proofLaunchAdmission" {
					site = "proofLaunchAdmission.ControlRoot"
				}
			case *ast.SelectorExpr:
				if isIdentifier(literalType.X, "proofrun") && literalType.Sel.Name == "LaunchOptions" {
					site = "proofrun.LaunchOptions.ControlRoot"
				}
			}
			if site != "" {
				for _, element := range typed.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if site == "proofrun.LaunchOptions.ControlRoot" && isIdentifier(field.Key, "CommitTerminal") {
						found["proofrun.LaunchOptions.CommitTerminal"]++
						call, ok := field.Value.(*ast.CallExpr)
						if !ok || callName(call) != "testingTerminalCommit" {
							t.Errorf("proofrun.LaunchOptions.CommitTerminal takes %s instead of testingTerminalCommit(...)", expressionText(field.Value))
						}
						continue
					}
					if !isIdentifier(field.Key, "ControlRoot") {
						continue
					}
					found[site]++
					if !isIdentifier(field.Value, "controlRoot") {
						t.Errorf("%s takes %s instead of controlRoot", site, expressionText(field.Value))
					}
				}
			}
		case *ast.CallExpr:
			name := callName(typed)
			if !wantFirstArgument[name] {
				break
			}
			found[name]++
			if len(typed.Args) == 0 || !isIdentifier(typed.Args[0], "controlRoot") {
				argument := "no argument"
				if len(typed.Args) != 0 {
					argument = expressionText(typed.Args[0])
				}
				t.Errorf("%s takes %s first instead of controlRoot", name, argument)
			}
		}
		return true
	})
	for _, site := range []string{
		"controlRoot assignment",
		"proofLaunchAdmission.ControlRoot",
		"proofrun.ReadAttempts",
		"publishTestingResult",
		"retainIncompleteProofAttempt",
		"pathsRoot filepath.Join",
		"proofrun.LaunchOptions.ControlRoot",
		"proofrun.LaunchOptions.CommitTerminal",
	} {
		if found[site] == 0 {
			t.Errorf("runTestRun has no %s site", site)
		}
	}
	proofSource, err := os.ReadFile("proof_run.go")
	if err != nil {
		t.Fatal(err)
	}
	launcherSource, err := os.ReadFile(filepath.Join("..", "..", "internal", "proofrun", "launcher.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range testingTerminalControlRootProblems(source, proofSource, launcherSource) {
		t.Error(problem)
	}
	t.Run("wrong root mutation is rejected", func(t *testing.T) {
		wrongRoot := []byte(strings.Replace(string(proofSource),
			"FinalizeAttemptWithTestResultLocked(completion.ControlRoot,",
			"FinalizeAttemptWithTestResultLocked(completion.ExecutionRoot,", 1))
		if string(wrongRoot) == string(proofSource) {
			t.Fatal("wrong-root mutation did not find the terminal recording site")
		}
		if problems := testingTerminalControlRootProblems(source, wrongRoot, launcherSource); len(problems) == 0 {
			t.Fatal("structural witness accepted terminal recording under the execution root")
		}
	})
}

func testingTerminalControlRootProblems(testSource, proofSource, launcherSource []byte) []string {
	type parsedSource struct {
		bodies map[string]*ast.BlockStmt
	}
	parse := func(name string, source []byte) (parsedSource, error) {
		file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if err != nil {
			return parsedSource{}, err
		}
		parsed := parsedSource{bodies: map[string]*ast.BlockStmt{}}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				parsed.bodies[function.Name.Name] = function.Body
			}
		}
		return parsed, nil
	}
	parsed := map[string]parsedSource{}
	for _, input := range []struct {
		name   string
		source []byte
	}{{"test.go", testSource}, {"proof_run.go", proofSource}, {"launcher.go", launcherSource}} {
		value, err := parse(input.name, input.source)
		if err != nil {
			return []string{fmt.Sprintf("parse %s: %v", input.name, err)}
		}
		parsed[input.name] = value
	}
	var expressionPath func(ast.Expr) string
	expressionPath = func(expression ast.Expr) string {
		switch typed := expression.(type) {
		case *ast.Ident:
			return typed.Name
		case *ast.SelectorExpr:
			prefix := expressionPath(typed.X)
			if prefix != "" {
				return prefix + "." + typed.Sel.Name
			}
		}
		return ""
	}
	hasCall := func(body *ast.BlockStmt, name, firstArgument string) bool {
		found := false
		ast.Inspect(body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && expressionPath(call.Fun) == name && len(call.Args) > 0 && expressionPath(call.Args[0]) == firstArgument {
				found = true
			}
			return !found
		})
		return found
	}
	hasCompletionRoots := func(body *ast.BlockStmt) bool {
		found := false
		ast.Inspect(body, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || expressionPath(literal.Type) != "CompletionContext" {
				return true
			}
			fields := map[string]string{}
			for _, element := range literal.Elts {
				if field, ok := element.(*ast.KeyValueExpr); ok {
					fields[expressionPath(field.Key)] = expressionPath(field.Value)
				}
			}
			found = fields["ControlRoot"] == "controlRoot" && fields["ExecutionRoot"] == "options.Root"
			return !found
		})
		return found
	}
	var problems []string
	checks := []struct {
		file, function, call, argument string
	}{
		{"test.go", "runTestWorker", "canonicalProofRoot", "controlRoot"},
		{"test.go", "runTestWorker", "proofrun.AuthenticateWorker", "canonicalControl"},
		{"test.go", "runTestWorker", "proofrun.ReadAttempt", "canonicalControl"},
		{"test.go", "testingTerminalCommit", "readTestingWorkerResult", "workerResultPath"},
		{"test.go", "testingTerminalCommit", "commitProofTerminalWithTestResult", "completion"},
		{"proof_run.go", "commitProofTerminalWithTestResult", "commitProofTerminalWithReason", "completion"},
		{"proof_run.go", "commitProofTerminalWithReason", "proofrun.ReadProcessRecord", "completion.ControlRoot"},
		{"proof_run.go", "commitProofTerminalWithReason", "proofrun.FinalizeAttemptWithTestResultLocked", "completion.ControlRoot"},
		{"launcher.go", "LaunchSuite", "options.CommitTerminal", "completion"},
	}
	for _, check := range checks {
		body := parsed[check.file].bodies[check.function]
		if body == nil || !hasCall(body, check.call, check.argument) {
			problems = append(problems, fmt.Sprintf("%s does not link %s to %s(%s)", check.function, check.file, check.call, check.argument))
		}
	}
	if body := parsed["launcher.go"].bodies["LaunchSuite"]; body == nil || !hasCompletionRoots(body) {
		problems = append(problems, "LaunchSuite does not keep the terminal control root distinct from its execution root")
	}
	return problems
}

func TestPolicyChildNeverFetchesOrReArms(t *testing.T) {
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	preparations, rearmCalls := 0, 0
	prepareTestingForCommand = func(request testingSelectionRequest) (testingPreparation, error) {
		preparations++
		if request.LandedRearm {
			rearmCalls++
		}
		return testingPreparation{CandidateTree: "candidate", PolicyBaseCommit: "base"}, nil
	}
	status, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", t.TempDir(), "--purpose", "diagnostic", "--json", "--policy-child"})
	})
	var output testingPlanOutput
	if status != 0 || preparations != 1 || rearmCalls != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &output) != nil {
		t.Fatalf("policy child did not stay fetch/re-arm free with JSON-only stdout: status=%d preparations=%d rearms=%d stdout=%q stderr=%q", status, preparations, rearmCalls, stdout, stderr)
	}

	engine := filepath.Join(t.TempDir(), "policy-engine")
	script := "#!/bin/sh\nseen=\nfor arg in \"$@\"; do [ \"$arg\" = --policy-child ] && seen=1; done\n[ \"$seen\" = 1 ] || exit 9\nprintf '%s\\n' '{\"schemaVersion\":1}'\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := planWithTrustedPolicyEngine(engine, testingSelectionRequest{Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic}, t.TempDir(), "candidate"); err != nil {
		t.Fatalf("parent did not flag its policy child: %v", err)
	}
	verify, _, code := parseTestingSelection("test verify", []string{"--root", t.TempDir()}, false)
	if code != 0 || verify.LandedRearm || verify.PolicyChild {
		t.Fatalf("test verify changed its re-arm behavior: code=%d request=%+v", code, verify)
	}
}

func TestBaseMovedUnderTheRunRestartsPreparationOnce(t *testing.T) {
	t.Setenv(preparationRestartedEnv, "")
	calls := 0
	var prepared testingPreparation
	var prepareErr error
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		prepared, prepareErr = prepareTestingWith(testingSelectionRequest{LandedRearm: true}, func(request testingSelectionRequest) (testingPreparation, error) {
			calls++
			if !request.LandedRearm {
				return testingPreparation{}, errors.New("restart skipped landed re-arm entry")
			}
			if calls == 1 {
				return testingPreparation{}, &preparationBaseMove{ours: "old-base", engine: "new-base"}
			}
			return testingPreparation{PolicyBaseCommit: "new-base"}, nil
		})
		if prepareErr != nil {
			return 1
		}
		return 0
	})
	if status != 0 || prepareErr != nil || calls != 2 || prepared.PolicyBaseCommit != "new-base" ||
		os.Getenv(preparationRestartedEnv) != "1" || !strings.Contains(stderr, "restarting preparation once") {
		t.Fatalf("authenticated move did not restart once from the landed re-arm entry: status=%d calls=%d prepared=%+v err=%v guard=%q stderr=%q",
			status, calls, prepared, prepareErr, os.Getenv(preparationRestartedEnv), stderr)
	}
}

func newPolicyBaseMoveFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "engine.go"), []byte("package main\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), []byte("base contract\n"), 0o644)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "old base")
	old := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", old)
	return root, old
}

func TestBaseMovedToAnEngineChangeReArmsOnce(t *testing.T) {
	for _, test := range []struct {
		name           string
		contractChange bool
	}{
		{name: "engine path only"},
		{name: "engine path and testing contract", contractChange: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(preparationRestartedEnv, "")
			root, old := newPolicyBaseMoveFixture(t)
			writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "engine.go"), []byte("package main\nvar moved = true\n"), 0o644)
			if test.contractChange {
				writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), []byte("changed base contract\n"), 0o644)
			}
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "move base")
			moved := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", moved)
			decision := testingPlanOutput{CandidateTree: "candidate", PolicyBaseCommit: moved, BaseContractDigest: "old-digest"}
			if test.contractChange {
				decision.BaseContractDigest = "new-digest"
			}
			calls, rearms := 0, 0
			_, err := prepareTestingWith(testingSelectionRequest{LandedRearm: true}, func(request testingSelectionRequest) (testingPreparation, error) {
				calls++
				if calls == 1 {
					return testingPreparation{}, compareTrustedPolicyDecision(root, root, "candidate", old, "old-digest", decision)
				}
				if request.LandedRearm {
					rearms++
				}
				return testingPreparation{PolicyBaseCommit: moved}, nil
			})
			if err != nil || calls != 2 || rearms != 1 {
				t.Fatalf("descendant move did not succeed after exactly one restart and re-arm: calls=%d rearms=%d err=%v", calls, rearms, err)
			}
		})
	}
}

func TestSecondBaseMoveRefusesBaseMoved(t *testing.T) {
	t.Setenv(preparationRestartedEnv, "")
	calls := 0
	_, err := prepareTestingWith(testingSelectionRequest{}, func(testingSelectionRequest) (testingPreparation, error) {
		calls++
		if calls == 1 {
			return testingPreparation{}, &preparationBaseMove{ours: "base-a", engine: "base-b"}
		}
		return testingPreparation{}, &preparationBaseMove{ours: "base-b", engine: "base-c"}
	})
	if err == nil || calls != 2 || !strings.Contains(err.Error(), "cause=base-moved ours=base-b engine=base-c restarts=1") {
		t.Fatalf("second move did not refuse with the guarded base-moved cause: calls=%d err=%v", calls, err)
	}
}

func TestUnexplainedPolicyFieldRefusesDecisionMismatch(t *testing.T) {
	root, old := newPolicyBaseMoveFixture(t)
	writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "engine.go"), []byte("package main\nvar moved = true\n"), 0o644)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "move base")
	moved := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", moved)
	err := compareTrustedPolicyDecision(root, root, "candidate", old, "digest", testingPlanOutput{
		CandidateTree: "other-candidate", PolicyBaseCommit: moved, BaseContractDigest: "other-digest",
	})
	if err == nil || !strings.Contains(err.Error(), "cause=decision-mismatch field=candidate-tree") {
		t.Fatalf("candidate mismatch was incorrectly explained by the base move: %v", err)
	}
}

func TestNonDescendantPolicyBaseRefusesDecisionMismatch(t *testing.T) {
	root, common := newPolicyBaseMoveFixture(t)
	writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "engine.go"), []byte("package main\nvar ours = true\n"), 0o644)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "ours")
	ours := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "checkout", "-q", "-b", "sibling", common)
	writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "engine.go"), []byte("package main\nvar sibling = true\n"), 0o644)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "sibling")
	sibling := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", sibling)
	err := compareTrustedPolicyDecision(root, root, "candidate", ours, "digest", testingPlanOutput{
		CandidateTree: "candidate", PolicyBaseCommit: sibling, BaseContractDigest: "digest",
	})
	if err == nil || !strings.Contains(err.Error(), "cause=decision-mismatch field=policy-base-commit") {
		t.Fatalf("non-descendant base was incorrectly restarted: %v", err)
	}
}

func TestPublishTestingResultSpillsOverTheBound(t *testing.T) {
	root := t.TempDir()
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for len(encoded) <= output.MaxInlineBytes {
		result.Groups = append(result.Groups, proofrun.GroupResult{ID: fmt.Sprintf("group-%d-%s", len(result.Groups), strings.Repeat("x", 512))})
		encoded, err = json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
	}
	var publishErr error
	stdout, _ := captureStdout(t, func() int {
		publishErr = publishTestingResult(root, "", result)
		return 0
	})
	if publishErr != nil {
		t.Fatal(publishErr)
	}
	if strings.Count(stdout, "\n") != 1 || !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("spilled result stdout is not exactly one line: %q", stdout)
	}
	reference, ok := output.Detect([]byte(strings.TrimSuffix(stdout, "\n")))
	if !ok {
		t.Fatalf("spilled result did not print a reference: %q", stdout)
	}
	var decoded proofrun.TestResult
	if err := readStrictJSON(reference.Path, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, result) {
		t.Fatalf("spilled result changed during publication")
	}
	wantFile := append(append([]byte(nil), encoded...), '\n')
	if got, err := os.ReadFile(reference.Path); err != nil || !reflect.DeepEqual(got, wantFile) {
		t.Fatalf("spilled file bytes differ: bytes=%d want=%d err=%v", len(got), len(wantFile), err)
	}
}

func TestPublishTestingResultUnderTheBoundIsUnchanged(t *testing.T) {
	root := t.TempDir()
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: "small"}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var publishErr error
	stdout, _ := captureStdout(t, func() int {
		publishErr = publishTestingResult(root, "", result)
		return 0
	})
	if publishErr != nil {
		t.Fatal(publishErr)
	}
	if want := string(encoded) + "\n"; stdout != want {
		t.Fatalf("inline result stdout = %q, want %q", stdout, want)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(output.Dir))); !os.IsNotExist(err) {
		t.Fatalf("inline result created an output directory: %v", err)
	}
}

func TestTestingSelectionCarriesDeliveryAllGroupsOnlyForExecution(t *testing.T) {
	root := t.TempDir()
	request, _, code := parseTestingSelection("test run", []string{"--root", root, "--purpose", "delivery", "--all-groups"}, true)
	if code != 0 || !request.AllGroups {
		t.Fatalf("test run did not carry --all-groups: code=%d request=%+v", code, request)
	}
	_, code = captureStderr(t, func() int {
		_, _, parsed := parseTestingSelection("test run", []string{"--root", root, "--purpose", "diagnostic", "--all-groups"}, true)
		return parsed
	})
	if code != 2 {
		t.Fatalf("diagnostic --all-groups status=%d, want usage refusal", code)
	}
	_, code = captureStderr(t, func() int {
		_, _, parsed := parseTestingSelection("test plan", []string{"--root", root, "--all-groups"}, false)
		return parsed
	})
	if code != 2 {
		t.Fatalf("read-only test selection accepted execution flag: status=%d", code)
	}
}

func TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := trustedPolicyEngine(root, strings.Repeat("a", 40), false); err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("missing retained policy engine was accepted: %v", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("read-only policy selection created build inputs: entries=%v err=%v", entries, err)
	}
}

func TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	ctx := context.Background()
	built, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine: %v", err)
	}
	t.Cleanup(func() { _ = built.Close() })
	repeated, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("repeat candidate proof engine build: %v", err)
	}
	t.Cleanup(func() { _ = repeated.Close() })
	if repeated.Commit != built.Commit || repeated.Digest != built.Digest {
		t.Fatalf("same candidate tree produced unstable proof engine identity: first=%+v repeated=%+v", built, repeated)
	}
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "records", "counselor", "peer.md"), []byte("ledger-only move\n"), 0o644)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/records/counselor/peer.md")
	recordsTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	recordsBuild, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", recordsTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine after records-only move: %v", err)
	}
	t.Cleanup(func() { _ = recordsBuild.Close() })
	if recordsBuild.Commit != built.Commit || recordsBuild.Digest != built.Digest {
		t.Fatalf("records-only move changed engine build identity or bytes: first=%+v records=%+v", built, recordsBuild)
	}
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "go.sum"), []byte("fixture.example/module v1.0.0 h1:changed\n"), 0o644)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/go.sum")
	moduleTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	moduleBuild, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", moduleTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine after go.sum move: %v", err)
	}
	t.Cleanup(func() { _ = moduleBuild.Close() })
	if moduleBuild.Commit == built.Commit {
		t.Fatalf("go.sum move did not change engine build identity: first=%s changed=%s", built.Commit, moduleBuild.Commit)
	}
	testingFixtureGit(t, fixture.projectRoot, "config", "i18n.commitEncoding", "ISO-8859-1")
	testingFixtureGit(t, fixture.projectRoot, "config", "author.name", "repository author")
	t.Run("foreign Git identity and encoding", func(t *testing.T) {
		for name, value := range map[string]string{
			"GIT_AUTHOR_NAME": "foreign author", "GIT_AUTHOR_EMAIL": "foreign-author@example.invalid",
			"GIT_COMMITTER_NAME": "foreign committer", "GIT_COMMITTER_EMAIL": "foreign-committer@example.invalid",
			"GIT_AUTHOR_DATE": "2010-01-02T03:04:05Z", "GIT_COMMITTER_DATE": "2011-02-03T04:05:06Z",
		} {
			t.Setenv(name, value)
		}
		foreignEnvironment, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
		if err != nil {
			t.Fatalf("build identical tree with foreign Git identity and encoding: %v", err)
		}
		t.Cleanup(func() { _ = foreignEnvironment.Close() })
		if foreignEnvironment.Commit != built.Commit || foreignEnvironment.Digest != built.Digest {
			t.Fatalf("same tree depended on ambient Git identity or encoding: first=%+v foreign=%+v", built, foreignEnvironment)
		}
	})
	testingFixtureGit(t, fixture.projectRoot, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "land candidate tree")
	afterLanding, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build identical tree after its checkout history moved: %v", err)
	}
	t.Cleanup(func() { _ = afterLanding.Close() })
	if afterLanding.Commit != built.Commit || afterLanding.Digest != built.Digest {
		t.Fatalf("same tree depended on its checkout history: first=%+v after-landing=%+v", built, afterLanding)
	}
	actualDigest, err := fileSHA256(built.Path)
	if err != nil || actualDigest != built.Digest || actualDigest == fixture.policyDigest {
		t.Fatalf("candidate engine digest=%s policy=%s actual=%s err=%v", built.Digest, fixture.policyDigest, actualDigest, err)
	}
	data, err := os.ReadFile(built.Path)
	if err != nil || !strings.Contains(string(data), "candidate engine source") || !strings.Contains(string(data), built.Commit) {
		t.Fatalf("candidate engine does not carry candidate source and commit: commit=%s data=%q err=%v", built.Commit, data, err)
	}

	group := testpolicy.Group{ID: "candidate-bed", Kind: "integration", Adapter: "section", CWD: "metasystem",
		Inputs: []string{"metasystem/cmd/metasystem/engine.txt"}, Obligations: []string{"candidate-engine"},
		Platforms: []string{"any"}, TargetMS: 1, Section: "candidate-bed"}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID}, Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{group.ID}}}}
	prepared := testingPreparation{ProjectRoot: fixture.projectRoot, Prefix: "metasystem", CandidateTree: fixture.candidateTree,
		BaseCommit: fixture.baseCommit, PolicyBaseCommit: fixture.baseCommit, EffectiveContract: contract, Plan: plan,
		ContractDigest: strings.Repeat("1", 64), BaseContractDigest: strings.Repeat("2", 64),
		PolicyEngineDigest: fixture.policyDigest, BehaviorPolicyDigest: strings.Repeat("3", 64),
		JudgeKey: proofrun.ComputeJudgeKey(ctx, fixture.projectRoot, fixture.baseCommit, "metasystem")}
	if prepared.JudgeKey == proofrun.DefaultJudgeKey() || strings.Contains(prepared.JudgeKey, ":unreadable:") {
		t.Fatalf("the fixture's engine sources did not yield a judge key: %s", prepared.JudgeKey)
	}
	request := testingRunRequest(prepared, "", "", built.Path, built.Digest, built.Commit)
	result := proofrun.NewTestResult(request)
	if request.JudgeKey != prepared.JudgeKey || result.JudgeKey != prepared.JudgeKey {
		t.Fatalf("the judge key was not carried into the request and the result: request=%s result=%s", request.JudgeKey, result.JudgeKey)
	}
	if result.PolicyEngineDigest != fixture.policyDigest || result.CandidateEngineDigest != built.Digest ||
		result.CandidateEngineBuildIdentity != built.Commit || result.CandidateTree != fixture.candidateTree {
		t.Fatalf("retained execution identity lost policy, candidate engine, or tree: %+v", result)
	}
	identities, err := proofrun.GroupExecutionIdentities(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	changedPolicy := request
	changedPolicy.PolicyEngineDigest = strings.Repeat("4", 64)
	policyIdentities, err := proofrun.GroupExecutionIdentities(ctx, changedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if identities[group.ID] != policyIdentities[group.ID] {
		t.Fatalf("a rebuilt judge (engine digest alone changed) changed the group execution identity: current=%s rebuilt=%s", identities[group.ID], policyIdentities[group.ID])
	}
	changedJudge := request
	changedJudge.JudgeKey = prepared.JudgeKey + ":changed"
	policyIdentities, err = proofrun.GroupExecutionIdentities(ctx, changedJudge)
	if err != nil {
		t.Fatal(err)
	}
	changedCandidate := request
	changedCandidate.CandidateEngineDigest = strings.Repeat("5", 64)
	candidateIdentities, err := proofrun.GroupExecutionIdentities(ctx, changedCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if identities[group.ID] == policyIdentities[group.ID] || identities[group.ID] == candidateIdentities[group.ID] {
		t.Fatalf("group execution identity omitted the judge key or the candidate engine: current=%s judge-change=%s candidate-change=%s", identities[group.ID], policyIdentities[group.ID], candidateIdentities[group.ID])
	}
}

func candidateResourceCustodyContext(t *testing.T) context.Context {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", executable, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build resource custodian fixture: %v\n%s", err, output)
	}
	return proofrun.WithResourceCustodyExecutable(context.Background(), executable)
}

func TestCandidateEngineArtifactReuseValidatesBytesAndBuildInputs(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	counter := filepath.Join(t.TempDir(), "builds")
	custodySeen := filepath.Join(t.TempDir(), "custody-seen")
	scriptPath := filepath.Join(fixture.installationRoot, "scripts", "agents", "go-build.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script = []byte(strings.Replace(string(script), "set -euo pipefail\n", "set -euo pipefail\nprintf 'build\\n' >> '"+counter+"'\nprintf '%s' \"${METASYSTEM_PROOF_ATTEMPT:-}\" > '"+custodySeen+"'\n", 1))
	writeTestingFixtureFile(t, scriptPath, script, 0o755)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/scripts/agents/go-build.sh")
	workspace := gittree.Workspace{Dir: fixture.projectRoot}
	tree, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	controlRoot := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(controlRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=64\n"), 0o600)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(controlRoot, "admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", controlRoot)
	custodyContext := candidateResourceCustodyContext(t)
	environment := inheritedTestingEnvironment(testingEnvironment(os.Environ()), []string{"METASYSTEM_PROOF_ATTEMPT=legacy-parent"})
	prepare := func(tree string) *candidateEngineBuild {
		t.Helper()
		artifact, err := prepareCandidateEngine(custodyContext, controlRoot, workspace, "metasystem", tree, environment)
		if err != nil {
			t.Fatal(err)
		}
		return artifact
	}
	buildCount := func() int {
		t.Helper()
		data, err := os.ReadFile(counter)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(data), "build\n")
	}
	first := prepare(tree)
	if buildCount() != 1 {
		t.Fatal("cold candidate engine preparation did not build once")
	}
	if seen, err := os.ReadFile(custodySeen); err != nil || string(seen) != "legacy-parent" {
		t.Fatalf("candidate build lost legacy proof custody: seen=%q err=%v", seen, err)
	}
	second := prepare(tree)
	if buildCount() != 1 || first.Path != second.Path || first.Digest != second.Digest {
		t.Fatalf("warm candidate engine preparation rebuilt: first=%+v second=%+v count=%d", first, second, buildCount())
	}
	environment = inheritedTestingEnvironment(testingEnvironment(os.Environ()), []string{"METASYSTEM_PROOF_ATTEMPT=another-parent"})
	custodyVariant := prepare(tree)
	if buildCount() != 1 || custodyVariant.Commit != first.Commit {
		t.Fatalf("custody-only change fragmented candidate engine cache: first=%s variant=%s count=%d", first.Commit, custodyVariant.Commit, buildCount())
	}
	if err := testexec.Locked(func() error { return os.Chmod(second.Path, 0o700) }); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(second.Path, []byte("corrupt"), 0o500); err != nil {
		t.Fatal(err)
	}
	third := prepare(tree)
	if buildCount() != 2 || third.Digest != first.Digest {
		t.Fatalf("corrupt candidate engine was not rebuilt: third=%+v count=%d", third, buildCount())
	}
	if err := os.Remove(third.Path); err != nil {
		t.Fatal(err)
	}
	prepare(tree)
	if buildCount() != 3 {
		t.Fatalf("missing candidate engine was not rebuilt: %d", buildCount())
	}
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "go.sum"), []byte("changed sum\n"), 0o644)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/go.sum")
	changedTree, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	changed := prepare(changedTree)
	if changed.Commit == first.Commit || buildCount() != 4 {
		t.Fatalf("changed build input reused previous artifact: first=%s changed=%s count=%d", first.Commit, changed.Commit, buildCount())
	}
	environment = append(append([]string(nil), environment...), "METASYSTEM_FIXTURE_BUILD_MODE=changed")
	changedEnvironment := prepare(changedTree)
	if changedEnvironment.Commit == changed.Commit || buildCount() != 5 {
		t.Fatalf("changed build environment reused previous artifact: previous=%s current=%s count=%d", changed.Commit, changedEnvironment.Commit, buildCount())
	}
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	toolDir := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(toolDir, "go"), []byte("#!/bin/sh\nexec '"+realGo+"' \"$@\"\n"), 0o755)
	toolEnvironment := append([]string(nil), environment...)
	for index, entry := range toolEnvironment {
		if strings.HasPrefix(entry, "PATH=") {
			toolEnvironment[index] = "PATH=" + toolDir + string(os.PathListSeparator) + strings.TrimPrefix(entry, "PATH=")
		}
	}
	environment = toolEnvironment
	changedTool := prepare(changedTree)
	if changedTool.Commit == changedEnvironment.Commit || buildCount() != 6 {
		t.Fatalf("changed Go executable reused previous artifact: previous=%s current=%s count=%d", changedEnvironment.Commit, changedTool.Commit, buildCount())
	}
	t.Run("physical command origin includes event-held cold build", testRunTestPlanIncludesEventHeldColdBuildFromPhysicalCommandOrigin)
}

func testRunTestPlanIncludesEventHeldColdBuildFromPhysicalCommandOrigin(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	controlRoot := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(controlRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=64\n"), 0o600)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(controlRoot, "admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", controlRoot)
	semanticNow := time.Date(2026, 9, 19, 10, 30, 0, 0, time.UTC)
	t.Setenv("METASYSTEM_GOAL_NOW", semanticNow.Format(time.RFC3339Nano))
	custodyContext := candidateResourceCustodyContext(t)

	physicalEntry := time.Now().UTC()
	buildEntered, releaseBuild := make(chan struct{}), make(chan struct{})
	type buildOutcome struct {
		artifact *candidateEngineBuild
		err      error
	}
	built := make(chan buildOutcome, 1)
	go func() {
		artifact, err := prepareCandidateEngineWithColdPreflight(custodyContext, controlRoot,
			gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()), func() error {
				close(buildEntered)
				<-releaseBuild
				return nil
			})
		built <- buildOutcome{artifact: artifact, err: err}
	}()
	select {
	case <-buildEntered:
	case outcome := <-built:
		t.Fatalf("candidate build returned before its cold-build event: %v", outcome.err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	releasedAt := time.Now().UTC()
	close(releaseBuild)
	var outcome buildOutcome
	select {
	case outcome = <-built:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	t.Cleanup(func() { _ = outcome.artifact.Close() })
	entryReleaseMS := releasedAt.Sub(physicalEntry).Milliseconds()

	request := proofrun.TestRunRequest{ProjectRoot: fixture.projectRoot, CandidateTree: fixture.candidateTree,
		Contract: testpolicy.Contract{SchemaVersion: 1}, Plan: testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic},
		CommandStartedAt: physicalEntry.Format(time.RFC3339Nano)}
	for _, mode := range []struct {
		name        string
		controlRoot string
		fixedStamps bool
	}{{name: "production"}, {name: "authorized fixture", controlRoot: controlRoot, fixedStamps: true}} {
		t.Run(mode.name, func(t *testing.T) {
			candidate := request
			candidate.ControlRoot = mode.controlRoot
			result, status, err := proofrun.RunTestPlan(context.Background(), candidate)
			if err != nil || status != 0 {
				t.Fatalf("run result status=%d err=%v", status, err)
			}
			if result.Cost.ActualDurationMS+2 < entryReleaseMS {
				t.Fatalf("actual duration %dms omits observed command-entry-to-build-release interval %dms", result.Cost.ActualDurationMS, entryReleaseMS)
			}
			if mode.fixedStamps {
				want := semanticNow.Format(time.RFC3339Nano)
				if result.StartedAt != want || result.EndedAt != want {
					t.Fatalf("semantic stamps started=%s ended=%s, want %s", result.StartedAt, result.EndedAt, want)
				}
			}
		})
	}
}

func TestFailedCandidateEngineBuildCannotFillArtifactCache(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	counter := filepath.Join(t.TempDir(), "builds")
	scriptPath := filepath.Join(fixture.installationRoot, "scripts", "agents", "go-build.sh")
	script := []byte("#!/usr/bin/env bash\nprintf 'build\\n' >> '" + counter + "'\nexit 23\n")
	writeTestingFixtureFile(t, scriptPath, script, 0o755)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/scripts/agents/go-build.sh")
	workspace := gittree.Workspace{Dir: fixture.projectRoot}
	tree, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	controlRoot := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(controlRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=64\n"), 0o600)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(controlRoot, "admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", controlRoot)
	custodyContext := candidateResourceCustodyContext(t)
	environment := testingEnvironment(os.Environ())
	for run := 0; run < 2; run++ {
		if artifact, err := prepareCandidateEngine(custodyContext, controlRoot, workspace, "metasystem", tree, environment); artifact != nil || err == nil {
			t.Fatalf("failed build %d entered cache: artifact=%+v err=%v", run, artifact, err)
		}
	}
	data, err := os.ReadFile(counter)
	if err != nil || strings.Count(string(data), "build\n") != 2 {
		t.Fatalf("failed builds were cached: count=%q err=%v", data, err)
	}
	buildIdentity, err := candidateEngineBuildIdentity(context.Background(), workspace, "metasystem", tree, environment)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines", buildIdentity)
	if _, err := os.Lstat(entry); !os.IsNotExist(err) {
		t.Fatalf("failed build published an artifact entry: %v", err)
	}
}

func TestBatchPrefixTestingControlRootRetainsAttemptOutsideExecution(t *testing.T) {
	t.Parallel()
	controlRoot, executionRoot := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(controlRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared := testingPreparation{Installation: executionRoot, ControlRoot: controlRoot, ProjectRoot: executionRoot}
	runRequest := testingRunRequest(prepared, "", "", "", strings.Repeat("1", 64), strings.Repeat("2", 40))
	if runRequest.ControlRoot != controlRoot || runRequest.ProjectRoot != executionRoot {
		t.Fatalf("batch prefix request control=%s execution=%s, want %s and %s", runRequest.ControlRoot, runRequest.ProjectRoot, controlRoot, executionRoot)
	}
	proofIdentity, err := proofrun.BuildProofIdentity(executionRoot, "", "selected", "testing", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		CandidateGoalID: "goal-a", CandidateRevision: 2, CandidateTree: strings.Repeat("b", 40), ReservedMinutes: 2,
		Identity: proofIdentity, Launcher: launcher, Now: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
	}, "0")))
	if err != nil || decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("reserve split-root batch prefix attempt: decision=%+v error=%v", decision, err)
	}
	retained, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
	if err != nil || retained.ControlRoot != controlRoot || retained.ExecutionRoot != executionRoot {
		t.Fatalf("durable split-root attempt=%+v error=%v", retained, err)
	}
	if _, err := proofrun.ReadAttempt(executionRoot, attempt.AttemptID); !os.IsNotExist(err) {
		t.Fatalf("batch prefix attempt leaked into disposable execution root: %v", err)
	}
}

func TestCandidateEngineTrimpathIsReproducibleAcrossMaterializationDirectories(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	stamp := strings.Repeat("a", 40)
	build := func(name string) string {
		root := filepath.Join(t.TempDir(), name)
		writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "go-build.sh"), script, 0o755)
		writeTestingFixtureFile(t, filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.26\n"), 0o644)
		writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
		output := filepath.Join(t.TempDir(), "metasystem")
		command := exec.Command("bash", "scripts/agents/go-build.sh", "--trimpath", "--out", output)
		command.Dir = root
		command.Env = append(candidateEngineBuildEnvironment(testingEnvironment(os.Environ()), stamp),
			"GOCACHE="+filepath.Join(cacheRoot, "build"), "GOMODCACHE="+filepath.Join(cacheRoot, "modules"))
		if combined, buildErr := command.CombinedOutput(); buildErr != nil {
			t.Fatalf("build identical tree in %s: %v\n%s", root, buildErr, combined)
		}
		digest, digestErr := fileSHA256(output)
		if digestErr != nil {
			t.Fatal(digestErr)
		}
		return digest
	}
	first, second := build("first-materialization"), build("second-materialization")
	if first != second {
		t.Fatalf("one source tree built in two directories had different candidate engine digests: first=%s second=%s", first, second)
	}
}

func TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody(t *testing.T) {
	stamp := strings.Repeat("a", 40)
	environment := candidateEngineBuildEnvironment([]string{
		"PATH=/fixture/bin", "GOFLAGS=-mod=vendor", "GOWORK=/foreign/workspace", "GOTOOLCHAIN=auto",
		"GOEXPERIMENT=fieldtrack", "GOENV=/foreign/goenv", "CGO_ENABLED=1", "GOAMD64=v4", "GOARM64=v9.5", "GOARM=5",
		"METASYSTEM_PROOF_CONTROL_ROOT=/proof", "METASYSTEM_PROOF_ATTEMPT=proof-attempt",
	}, stamp)
	values := map[string]string{}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		values[name] = value
	}
	want := map[string]string{"CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0", "GOARM": "7",
		"GOENV": "off", "GOEXPERIMENT": "", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "GOWORK": "off", "METASYSTEM_BUILD_STAMP": stamp,
		"METASYSTEM_PROOF_CONTROL_ROOT": "/proof", "METASYSTEM_PROOF_ATTEMPT": "proof-attempt"}
	for name, value := range want {
		if values[name] != value {
			t.Fatalf("candidate build environment %s=%q, want %q: %v", name, values[name], value, environment)
		}
	}
}

func TestRunOwnerSurvivesTestingWorkerFilters(t *testing.T) {
	const owner = "outer-exact-ref"
	prepared := testingEnvironment([]string{"PATH=/fixture/bin", identity.RunOwnerEnv + "=" + owner, "UNRELATED=drop"})
	if joined := strings.Join(prepared, "\n"); !strings.Contains(joined, identity.RunOwnerEnv+"="+owner) || strings.Contains(joined, "UNRELATED=") {
		t.Fatalf("testing environment filtered the run owner incorrectly: %v", prepared)
	}
	worker := inheritedTestingEnvironment([]string{"PATH=/fixture/bin"}, []string{
		"METASYSTEM_PROOF_CONTROL_ROOT=/proof", identity.RunOwnerEnv + "=" + owner,
		identity.FixtureAttemptEnv + "=attempt-a", "UNRELATED=drop",
	})
	if joined := strings.Join(worker, "\n"); !strings.Contains(joined, identity.RunOwnerEnv+"="+owner) ||
		!strings.Contains(joined, identity.FixtureAttemptEnv+"=attempt-a") || strings.Contains(joined, "UNRELATED=") {
		t.Fatalf("worker inheritance filtered the run owner incorrectly: %v", worker)
	}
}

func TestTestWorkerBuildIdentityCompatibilityDoorIsPolicyProbeOnly(t *testing.T) {
	root, err := os.MkdirTemp("", "metasystem-policy-probe.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	candidateEngine := filepath.Join(root, "candidate-engine")
	writeTestingFixtureFile(t, candidateEngine, []byte("candidate engine\n"), 0o755)
	candidateDigest, err := fileSHA256(candidateEngine)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.TestRunRequest{
		CandidateEngine: candidateEngine, CandidateEngineDigest: candidateDigest,
		ProjectRoot: root, Contract: testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{{ID: "literal"}}},
		Plan: testpolicy.Plan{SelectedGroups: []string{"literal"}},
	}
	packet, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(root, "request.json")
	writeTestingFixtureFile(t, packetPath, packet, 0o600)
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--packet", packetPath, "--packet-sha256", packetDigest, "--result", filepath.Join(root, "result.json")}

	t.Setenv(policyProbeWorkerEnvironment, "")
	stderr, code := captureStderr(t, func() int { return runTestWorker(args) })
	if code != 3 || !strings.Contains(stderr, "input-bound candidate engine is absent") {
		t.Fatalf("ordinary worker accepted a request without a candidate build identity: code=%d stderr=%q", code, stderr)
	}

	t.Setenv(policyProbeWorkerEnvironment, "1")
	stderr, code = captureStderr(t, func() int { return runTestWorker(args) })
	if code != 3 || strings.Contains(stderr, "input-bound candidate engine is absent") ||
		!strings.Contains(stderr, "input-bound policy engine changed") {
		t.Fatalf("legacy policy probe did not pass the build-identity request check: code=%d stderr=%q", code, stderr)
	}
}

func TestFrozenWorkerProbePathsCanonicalizeRootOnce(t *testing.T) {
	t.Parallel()
	physical := filepath.Join(t.TempDir(), "physical")
	if err := os.Mkdir(physical, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	root, packet, result, err := frozenWorkerProbePaths(alias)
	if err != nil {
		t.Fatal(err)
	}
	wantParent, err := canonicalPath(physical)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(root) != wantParent || !strings.HasPrefix(filepath.Base(root), "metasystem-policy-probe.") {
		t.Fatalf("probe root=%q, want canonical parent=%q and preserved prefix", root, wantParent)
	}
	if filepath.Dir(packet) != root || filepath.Dir(result) != root || packet == result {
		t.Fatalf("probe paths root=%q packet=%q result=%q", root, packet, result)
	}
}

func TestFrozenPolicyProbeRefusalNamesResultPathPredicate(t *testing.T) {
	t.Parallel()
	request := proofrun.TestRunRequest{
		ProjectRoot: "/private/var/folders/metasystem-policy-probe.fixture",
		Contract:    testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{{ID: "literal"}}},
		Plan:        testpolicy.Plan{SelectedGroups: []string{"literal"}},
	}
	refusal := frozenPolicyProbeRefusal(request, "/var/folders/metasystem-policy-probe.fixture/result.json")
	if !strings.Contains(refusal, `result directory="/var/folders/metasystem-policy-probe.fixture"`) ||
		!strings.Contains(refusal, `project root="/private/var/folders/metasystem-policy-probe.fixture"`) {
		t.Fatalf("result path predicate diagnostic=%q", refusal)
	}
}

func TestCandidateBuiltCommitPassesDispatchSkewPreflight(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	ctx := context.Background()
	built, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = built.Close() })
	detached, err := (gittree.Workspace{Dir: fixture.projectRoot}).NewDetachedWorktree(fixture.candidateTree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = detached.Close() })
	candidateRoot := filepath.Join(detached.Workspace().Dir, "metasystem")
	installedEngine := filepath.Join(candidateRoot, "bin", "metasystem")
	data, err := os.ReadFile(built.Path)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, installedEngine, data, 0o755)
	preflight := func(stamp string) ([]byte, error) {
		command := exec.Command("bash", "scripts/agents/dispatch.sh", "__engine-skew-preflight", stamp)
		command.Dir = candidateRoot
		command.Env = append(testingEnvironment(os.Environ()), "METASYSTEM_BIN="+installedEngine)
		return command.CombinedOutput()
	}
	oldOutput, oldErr := preflight(fixture.baseCommit)
	if exit, ok := oldErr.(*exec.ExitError); !ok || exit.ExitCode() != 1 || !strings.Contains(string(oldOutput), "is older than checkout commit") {
		t.Fatalf("untouched refusal was not reproduced with the enrolled engine stamp: err=%v output=%s", oldErr, oldOutput)
	}
	if output, err := preflight(""); err != nil {
		t.Fatalf("installed candidate engine's reported stamp did not pass dispatch skew preflight: %v\n%s", err, output)
	}
	ancestry := exec.Command("git", "-C", detached.Workspace().Dir, "log", "--ancestry-path", built.Commit+"..HEAD")
	ancestry.Env = gittree.ScrubbedEnviron()
	if output, err := ancestry.CombinedOutput(); err != nil || len(strings.TrimSpace(string(output))) != 0 {
		t.Fatalf("candidate stamp unexpectedly has an ancestry path to its materialized checkout: err=%v output=%s", err, output)
	}
	for _, stamp := range []string{"witness-0123456789ab", "dev-0123456789ab-dirty", "dev", "adopted-target"} {
		if output, err := preflight(stamp); err != nil {
			t.Fatalf("non-commit engine stamp %q was refused: %v\n%s", stamp, err, output)
		}
	}
	freshProject := t.TempDir()
	freshCandidateRoot := filepath.Join(freshProject, "metasystem")
	for _, relative := range []string{"dispatch.sh", "checkout-execution-guard.sh"} {
		script, err := os.ReadFile(filepath.Join(candidateRoot, "scripts", "agents", relative))
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(freshCandidateRoot, "scripts", "agents", relative), script, 0o755)
	}
	freshEngine := filepath.Join(freshCandidateRoot, "bin", "metasystem")
	writeTestingFixtureFile(t, freshEngine, data, 0o755)
	testingFixtureGit(t, freshProject, "init", "-q", "-b", "main")
	testingFixtureGit(t, freshProject, "add", ".")
	testingFixtureGit(t, freshProject, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fresh fixture repository")
	missingCandidate := exec.Command("git", "-C", freshProject, "cat-file", "-e", built.Commit+"^{commit}")
	missingCandidate.Env = gittree.ScrubbedEnviron()
	if err := missingCandidate.Run(); err == nil {
		t.Fatalf("fresh fixture repository unexpectedly contains candidate stamp %s", built.Commit)
	}
	freshPreflight := exec.Command("bash", "scripts/agents/dispatch.sh", "__engine-skew-preflight")
	freshPreflight.Dir = freshCandidateRoot
	freshPreflight.Env = append(testingEnvironment(os.Environ()), "METASYSTEM_BIN="+freshEngine)
	if output, err := freshPreflight.CombinedOutput(); err != nil {
		t.Fatalf("fresh repository refused its installed candidate engine's reported stamp: %v\n%s", err, output)
	}
}

func TestCandidateEngineBuildFailureCannotFallBackToPolicyEngine(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	broken := []byte("#!/usr/bin/env bash\nset -euo pipefail\necho 'fixture candidate compile failed' >&2\nexit 23\n")
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "scripts", "agents", "go-build.sh"), broken, 0o755)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/scripts/agents/go-build.sh")
	brokenTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	built, err := buildCandidateEngine(context.Background(), gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", brokenTree, testingEnvironment(os.Environ()))
	if built != nil || err == nil || !strings.Contains(err.Error(), "candidate engine build failed") || !strings.Contains(err.Error(), "fixture candidate compile failed") {
		t.Fatalf("candidate build failure did not remain an explicit insufficient outcome: build=%+v err=%v", built, err)
	}
	if digest, digestErr := fileSHA256(fixture.policyEngine); digestErr != nil || digest != fixture.policyDigest {
		t.Fatalf("candidate failure changed or substituted the policy engine: digest=%s err=%v", digest, digestErr)
	}
}

func TestVerifyRecoversCandidateDigestFromNewestSufficientAttempt(t *testing.T) {
	const groupID = "candidate-bed"
	digest := strings.Repeat("a", 64)
	executionIdentity := strings.Repeat("b", 64)
	candidateDigest := strings.Repeat("c", 64)
	buildIdentity := strings.Repeat("f", 40)
	candidateTree := strings.Repeat("e", 40)
	group := testpolicy.Group{ID: groupID, Kind: "unit", CWD: ".", Inputs: []string{"source.go"},
		Obligations: []string{"candidate-engine"}, Platforms: []string{"any"}, TargetMS: 1}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{groupID}, SelectedGroups: []string{groupID}}
	prepared := testingPreparation{CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := testingRunRequest(prepared, "successful-attempt", "", "", candidateDigest, buildIdentity)
	request.ProjectRoot, request.BaseCommit = "/project", "base"
	successful := proofrun.NewTestResult(request)
	zero := 0
	successful.Groups = []proofrun.GroupResult{{ID: groupID, Kind: group.Kind, Obligations: group.Obligations,
		InputManifest: group.Inputs, ExecutionIdentity: executionIdentity, Status: "passed", NativeLaunched: true,
		CollectionComplete: true, NativeExitStatus: &zero, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	// A sufficient attempt from an earlier plan remains a valid source for
	// the deterministic candidate engine; its groups are reused only while no
	// newer observation at the same identity contradicts them.
	successful.CandidateTree = strings.Repeat("9", 40)
	successful.PlanDigest = strings.Repeat("1", 64)
	successful.RecomputeDelivery()
	failed := successful
	failed.AttemptID = "later-failed-attempt"
	// A red battery is still a completed measurement of the deterministic
	// candidate engine. Carried landing needs its structured insufficiency;
	// sufficiency remains the later delivery decision, not an engine-identity
	// precondition.
	failed.CandidateEngineDigest = strings.Repeat("d", 64)
	exit := 23
	failed.Groups = append([]proofrun.GroupResult(nil), successful.Groups...)
	failed.Groups[0].Status, failed.Groups[0].NativeExitStatus = "failed", &exit
	failed.RecomputeDelivery()
	now := time.Now().UTC()
	attempts := []proofrun.Attempt{
		{AttemptID: successful.AttemptID, GoalID: prepared.GoalID, AccountingRevision: prepared.AccountingRevision,
			StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess},
			PendingTestGroups: map[string]string{groupID: executionIdentity}, TestResult: &successful},
		{AttemptID: failed.AttemptID, GoalID: prepared.GoalID, AccountingRevision: prepared.AccountingRevision,
			StartedAt: now.Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalFailed},
			PendingTestGroups: map[string]string{groupID: executionIdentity}, TestResult: &failed},
	}
	recovered, err := retainedCandidateEngineDigest(prepared, attempts, buildIdentity, false)
	if err != nil || recovered != candidateDigest {
		t.Fatalf("later failed attempt hid the sufficient candidate engine: digest=%s err=%v", recovered, err)
	}
	carriedDigest, err := retainedCandidateEngineDigest(prepared, attempts, buildIdentity, true)
	if err != nil || carriedDigest != failed.CandidateEngineDigest {
		t.Fatalf("carried verification did not retain the newest completed red measurement: digest=%s err=%v", carriedDigest, err)
	}
	templateRequest := testingRunRequest(prepared, "", "", "", recovered, buildIdentity)
	templateRequest.ProjectRoot, templateRequest.BaseCommit = "/project", "base"
	projection := proofrun.ReusedTestResult(proofrun.NewTestResult(templateRequest), attempts,
		map[string]string{groupID: executionIdentity}, contract)
	// The later attempt failed the same group at the same identity: that is
	// the newest observation, so the earlier pass is not reused (green then
	// red yields no reuse) while the candidate digest above is still recovered.
	if projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].Status != "not-run" || projection.Groups[0].NotRunReason != "newest-observation-failed" {
		t.Fatalf("verification composed the earlier pass although a newer attempt failed the group at the same identity: %+v", projection)
	}
}

func TestDiagnosticsReadersFollowTheCandidatePair(t *testing.T) {
	candidate := proofrun.Attempt{SchemaVersion: proofrun.CandidateAttemptSchemaVersion,
		GoalID: "authority-c", AccountingRevision: 3, CandidateGoalID: "candidate-x", CandidateRevision: 7}
	if !attemptAccountsForCandidate(candidate, "candidate-x", 7) {
		t.Fatal("candidate-owned diagnostic attempt was not selected")
	}
	if attemptAccountsForCandidate(candidate, "authority-c", 3) {
		t.Fatal("diagnostic reader selected the authority pair instead of the candidate pair")
	}
	legacy := proofrun.Attempt{SchemaVersion: proofrun.AttemptSchemaVersion, GoalID: "legacy", AccountingRevision: 5}
	if !attemptAccountsForCandidate(legacy, "legacy", 5) {
		t.Fatal("diagnostic reader stopped selecting schema-2 authority accounting")
	}
}

func TestVerifyDoesNotKeyLegacyCandidateDigestByWholeTreeReceipt(t *testing.T) {
	const groupID = "candidate-bed"
	digest := strings.Repeat("a", 64)
	candidateTree := strings.Repeat("b", 40)
	group := testpolicy.Group{ID: groupID, Kind: "unit", CWD: ".", Inputs: []string{"source.go"},
		Obligations: []string{"candidate-engine"}, Platforms: []string{"any"}, TargetMS: 1}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{groupID}, SelectedGroups: []string{groupID}}
	prepared := testingPreparation{Installation: t.TempDir(), CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := testingRunRequest(prepared, "legacy-success", "", "", digest, strings.Repeat("c", 40))
	request.ProjectRoot, request.BaseCommit = "/project", "base"
	legacy := proofrun.NewTestResult(request)
	legacy.CandidateEngineIdentityVersion = 0
	legacy.CandidateEngineDigest = ""
	zero := 0
	legacy.Groups = []proofrun.GroupResult{{ID: groupID, Kind: group.Kind, Obligations: group.Obligations,
		InputManifest: group.Inputs, ExecutionIdentity: digest, Status: "passed", NativeLaunched: true,
		CollectionComplete: true, NativeExitStatus: &zero, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	legacy.RecomputeDelivery()
	receipt := landing.TestReceipt{SchemaVersion: 2, Tree: candidateTree, ProvedTree: candidateTree, Testing: &legacy}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := landing.TestReceiptPath(prepared.Installation, candidateTree)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := retainedCandidateEngineDigest(prepared, nil, strings.Repeat("d", 40), false)
	if err == nil || recovered != "" || !strings.Contains(err.Error(), "candidate engine digest is absent") {
		t.Fatalf("legacy whole-tree receipt unexpectedly supplied a cross-tip engine identity: digest=%s err=%v", recovered, err)
	}
}

type candidateEngineFixture struct {
	projectRoot, installationRoot, baseCommit, candidateTree, policyEngine, policyDigest string
}

func newCandidateEngineFixture(t *testing.T) candidateEngineFixture {
	t.Helper()
	projectRoot := t.TempDir()
	installationRoot := filepath.Join(projectRoot, "metasystem")
	buildScript := `#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == --trimpath && "$2" == --out && -n "${3:-}" ]]
[[ "${CGO_ENABLED+x}:$CGO_ENABLED" == x:0 ]]
[[ "${GOENV+x}:$GOENV" == x:off ]]
[[ "${GOEXPERIMENT+x}:$GOEXPERIMENT" == x: ]]
[[ "${GOFLAGS+x}:$GOFLAGS" == x:-mod=readonly ]]
[[ "${GOTOOLCHAIN+x}:$GOTOOLCHAIN" == x:local ]]
[[ "${GOWORK+x}:$GOWORK" == x:off ]]
stamp=$(git rev-parse HEAD)
[[ "$METASYSTEM_BUILD_STAMP" == "$stamp" ]]
source=$(cat cmd/metasystem/engine.txt)
{
  printf '#!/usr/bin/env bash\nstamp=%q\n' "$stamp"
  cat <<'ENGINE'
if [[ "${1:-}" == supervise && "${2:-}" == status ]]; then
  printf '{"engineBuild":"%s"}\n' "$stamp"
  exit 0
fi
if [[ "${1:-}" == json && "${2:-}" == get ]]; then
  printf '%s\n' "$stamp"
  exit 0
fi
exit 0
ENGINE
  printf '# %s\n' "$source"
} >"$3"
chmod +x "$3"
`
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", "go-build.sh"), []byte(buildScript), 0o755)
	for _, relative := range []string{"dispatch.sh", "checkout-execution-guard.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", relative))
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", relative), data, 0o755)
	}
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", "validate-section-selector.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755)
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "cmd", "metasystem", "engine.txt"), []byte("enrolled engine source\n"), 0o644)
	testingFixtureGit(t, projectRoot, "init", "-q", "-b", "main")
	testingFixtureGit(t, projectRoot, "add", ".")
	testingFixtureGit(t, projectRoot, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "enrolled engine")
	baseCommit := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	policyEngine := filepath.Join(t.TempDir(), "metasystem")
	writeTestingFixtureFile(t, policyEngine, []byte("#!/usr/bin/env bash\n# enrolled engine\nexit 0\n"), 0o755)
	policyDigest, err := fileSHA256(policyEngine)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "cmd", "metasystem", "engine.txt"), []byte("candidate engine source\n"), 0o644)
	testingFixtureGit(t, projectRoot, "add", "metasystem/cmd/metasystem/engine.txt")
	candidateTree, err := (gittree.Workspace{Dir: projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	return candidateEngineFixture{projectRoot: projectRoot, installationRoot: installationRoot, baseCommit: baseCommit,
		candidateTree: candidateTree, policyEngine: policyEngine, policyDigest: policyDigest}
}

func TestTestingPlanAdoptsCandidateFallbackOnlyWhenBaseHasNone(t *testing.T) {
	candidate := testFallbackContract()
	for _, test := range []struct {
		name             string
		baseFallback     bool
		expectedFallback string
	}{
		{name: "candidate-fallback-fills-empty-base", expectedFallback: "residual"},
		{name: "base-fallback-remains-protected", baseFallback: true, expectedFallback: "trusted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			base := candidate
			base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces...)
			base.Groups = append([]testpolicy.Group(nil), candidate.Groups...)
			if test.baseFallback {
				base.Fallback = "trusted"
				base.Surfaces[1].Paths = []string{"candidate-owned/**"}
				base.Surfaces = append(base.Surfaces, testpolicy.Surface{ID: "trusted", Paths: []string{}, Standard: []string{"trusted"}})
				base.Groups = append(base.Groups, testFallbackGroup("trusted", "unowned.txt"))
			} else {
				base.Fallback = ""
				base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces[:1]...)
				base.Groups = append([]testpolicy.Group(nil), candidate.Groups[:1]...)
			}
			baseBytes, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			candidateBytes, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), baseBytes, 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "owned", "source.go"), []byte("package owned\n"), 0o644)
			testingFixtureGit(t, root, "init", "-q", "-b", "main")
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
			baseCommit := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			const landingRef = "refs/remotes/origin/main"
			testingFixtureGit(t, root, "update-ref", landingRef, baseCommit)
			testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)

			engine := filepath.Join(t.TempDir(), "metasystem")
			build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
				"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+baseCommit, "-o", engine, ".")
			if output, buildErr := build.CombinedOutput(); buildErr != nil {
				t.Fatalf("build fallback policy engine: %v\n%s", buildErr, output)
			}
			canonicalRoot, err := canonicalProofRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			canonicalEngine, err := canonicalPath(engine)
			if err != nil {
				t.Fatal(err)
			}
			digest, err := fileSHA256(canonicalEngine)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
				InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-10T00:00:00Z", Enrollment: steward.EnrollmentFixture,
				EngineBuild: baseCommit, LandedCommit: baseCommit, LandingRef: landingRef}); err != nil {
				t.Fatal(err)
			}

			writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), candidateBytes, 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "unowned.txt"), []byte("changed\n"), 0o644)
			testingFixtureGit(t, root, "add", "testing.json", "unowned.txt")
			candidateTree, err := (gittree.Workspace{Dir: root}).StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(root, ".git", "objects"))
			t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", "")
			t.Setenv("GIT_CONFIG_COUNT", "0")
			prepared, err := prepareTesting(testingSelectionRequest{Root: root, Tree: candidateTree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic})
			if err != nil {
				t.Fatal(err)
			}
			if prepared.EffectiveContract.Fallback != test.expectedFallback {
				t.Fatalf("effective fallback = %q, want %q", prepared.EffectiveContract.Fallback, test.expectedFallback)
			}
			if len(prepared.Plan.Uncertainty) != 0 || !containsString(prepared.Plan.AffectedSurfaces, test.expectedFallback) || !containsString(prepared.Plan.SelectedGroups, test.expectedFallback) {
				t.Fatalf("unowned path did not select the fallback without uncertainty: %+v", prepared.Plan)
			}
			if test.baseFallback && containsString(prepared.Plan.AffectedSurfaces, candidate.Fallback) {
				t.Fatalf("candidate fallback replaced the protected base fallback: %+v", prepared.Plan)
			}
		})
	}
}

func testFallbackContract() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"owned/**"}, Standard: []string{"app"}},
			{ID: "residual", Paths: []string{}, Standard: []string{"residual"}},
		},
		Groups:  []testpolicy.Group{testFallbackGroup("app", "owned/**"), testFallbackGroup("residual", "unowned.txt")},
		Always:  testpolicy.Always{Canary: []string{"app"}},
		Unknown: []string{"app"},
		Cadence: []string{"app"},
	}
}

func testFallbackGroup(id, input string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{input}, Outputs: []string{}, Tools: []testpolicy.Tool{},
		Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`"all"`)}
}

func TestProtectedCoverageFloorCannotFallOrDisappear(t *testing.T) {
	root := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":80.0}}`), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet-linux.json"), []byte(`{"floors":{"internal/app":79.0}}`), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":79.9}}`), 0o644)
	testingFixtureGit(t, root, "add", ".")
	candidate, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	if err := protectCoverageRatchets(workspace, base, candidate, ""); err == nil || !strings.Contains(err.Error(), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
		t.Fatalf("lowered base floor was accepted: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":80.1,"internal/new":50.0}}`), 0o644)
	testingFixtureGit(t, root, "add", ".")
	candidate, err = workspace.StagedTree()
	if err != nil || protectCoverageRatchets(workspace, base, candidate, "") != nil {
		t.Fatalf("raised protected floor was refused: %v", err)
	}
}

func TestFrozenPublicVersionOneProtectionCorpusIsComplete(t *testing.T) {
	cases := testpolicy.FrozenProtectionProbeCases()
	want := []string{"remove-required-provider", "shrink-dependency-graph", "lower-coverage-floor", "remove-required-test", "emit-zero-tests", "forge-component-reuse"}
	if len(cases) != len(want) {
		t.Fatalf("frozen corpus has %d cases, want %d", len(cases), len(want))
	}
	for index, id := range want {
		if cases[index].ID != id || len(cases[index].PublicArgv) != 2 || cases[index].PublicArgv[0] != "test" || cases[index].ResultField == "" {
			t.Fatalf("frozen corpus case %d = %+v", index, cases[index])
		}
	}
}

func TestFrozenWorkerProbesTraverseActiveEmptyLegacyAndForgedReuse(t *testing.T) {
	t.Parallel()
	request := proofrun.TestRunRequest{Workers: 3, AdmissionMaximum: 5,
		Plan: testpolicy.Plan{SelectedGroups: []string{"policy-protection"}}}
	type call struct {
		id      string
		workers int
	}
	var calls []call
	err := runFrozenPolicyProtectionCorpusWith(context.Background(), request,
		func(context.Context, proofrun.TestRunRequest, testpolicy.ProtectionProbeCase) error { return nil },
		func(_ context.Context, observed proofrun.TestRunRequest, probe testpolicy.ProtectionProbeCase) error {
			calls = append(calls, call{id: probe.ID, workers: observed.Workers})
			return nil
		})
	want := []call{{id: "emit-zero-tests", workers: 3}, {id: "emit-zero-tests", workers: 0}, {id: "forge-component-reuse", workers: 3}}
	if err != nil || !reflect.DeepEqual(calls, want) {
		t.Fatalf("worker probe calls=%v want=%v err=%v", calls, want, err)
	}
}

func TestFrozenWorkerProbeReaderAcceptsCandidateGroupFields(t *testing.T) {
	group := testpolicy.Group{ID: "literal", Kind: "unit", Inputs: []string{"source.txt"}, TargetMS: 1}
	request := proofrun.TestRunRequest{
		ProjectRoot:           t.TempDir(),
		CandidateTree:         strings.Repeat("a", 40),
		BaseCommit:            strings.Repeat("b", 40),
		Contract:              testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}},
		Plan:                  testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID}},
		CandidateEngineDigest: strings.Repeat("c", 64),
	}
	result := proofrun.NewTestResult(request)
	result.Groups = []proofrun.GroupResult{{ID: group.ID, Kind: group.Kind, InputManifest: group.Inputs, Status: "invalid"}}
	result.RecomputeDelivery()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var candidate map[string]any
	if err := json.Unmarshal(data, &candidate); err != nil {
		t.Fatal(err)
	}
	candidateGroups := candidate["groups"].([]any)
	// A field only a newer candidate engine writes; it must stay unknown to
	// this engine's strict reader, so it is not any field the shape has since
	// adopted (progressRule joined the shape on 2026-09-11).
	candidateGroups[0].(map[string]any)["candidateOnlyField"] = "from-a-newer-engine"
	data, err = json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	probeResult, err := readFrozenWorkerProbeResult(path)
	if err != nil || len(probeResult.Groups) != 1 || probeResult.Groups[0].Status != "invalid" || probeResult.Groups[0].CollectionComplete {
		t.Fatalf("probe reader lost the negative worker judgment: result=%+v err=%v", probeResult, err)
	}
	if _, err := readTestingWorkerResult(path); err == nil || !strings.Contains(err.Error(), `unknown field "candidateOnlyField"`) {
		t.Fatalf("strict destination worker reader accepted the candidate field: %v", err)
	}
}

func TestFrozenNegativeProbeResponseIsLegacyOnlyForActualInvalidResult(t *testing.T) {
	t.Parallel()
	group := testpolicy.Group{ID: "literal", Kind: "unit", Inputs: []string{"source.txt"}, TargetMS: 1}
	request := proofrun.TestRunRequest{SyntheticProbe: true, ProjectRoot: t.TempDir(), CandidateTree: strings.Repeat("a", 40),
		BaseCommit: strings.Repeat("b", 40), Contract: testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}},
		Plan: testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
			ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"literal"}, SelectedGroups: []string{"literal"}},
		CandidateEngineDigest: strings.Repeat("c", 64), Workers: 1}
	result := proofrun.NewTestResult(request)
	result.Groups = []proofrun.GroupResult{{ID: "literal", Kind: "unit", InputManifest: []string{"source.txt"}, Status: "invalid",
		IdentityVersion: proofrun.GroupExecutionIdentityVersion, ExecutableDigests: map[string]string{"__argv0__": strings.Repeat("d", 64)}}}
	result.RecomputeDelivery()
	legacy, err := frozenNegativeProbeResponse(request, result)
	if err != nil || legacy.SchemaVersion != proofrun.LegacyTestResultSchemaVersion || legacy.Delivery.Sufficient ||
		legacy.Groups[0].Status != "invalid" || legacy.Groups[0].CollectionComplete || legacy.Groups[0].IdentityVersion != 0 ||
		len(legacy.Groups[0].ExecutableDigests) != 0 || result.SchemaVersion != proofrun.TestResultSchemaVersion {
		t.Fatalf("legacy projection changed negative verdict or normal result: legacy=%+v source=%+v err=%v", legacy, result, err)
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "identityVersion") || strings.Contains(string(data), "freshnessEpisode") || strings.Contains(string(data), "freshGroups") ||
		strings.Contains(string(data), "queueDurationMs") {
		t.Fatalf("probe-only v1 wire contains new schema fields: %s", data)
	}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if decoded, err := readFrozenWorkerProbeResult(path); err != nil || decoded.SchemaVersion != 1 || decoded.Delivery.Sufficient {
		t.Fatalf("frozen reader rejected negative v1 response: %+v %v", decoded, err)
	}
	legacyRequest := request
	legacyRequest.Workers, legacyRequest.AdmissionMaximum = 0, 0
	legacyResult := proofrun.NewTestResult(legacyRequest)
	legacyResult.Groups = []proofrun.GroupResult{{ID: "literal", Kind: "unit", InputManifest: []string{"source.txt"}, Status: "invalid",
		IdentityVersion: proofrun.PreviousGroupExecutionIdentityVersion}}
	legacyResult.RecomputeDelivery()
	legacyProjection, err := frozenNegativeProbeResponse(legacyRequest, legacyResult)
	if err != nil || legacyResult.SchemaVersion != proofrun.PreviousTestResultSchemaVersion ||
		legacyResult.Groups[0].IdentityVersion != proofrun.PreviousGroupExecutionIdentityVersion ||
		legacyProjection.SchemaVersion != proofrun.LegacyTestResultSchemaVersion || legacyProjection.Groups[0].IdentityVersion != 0 {
		t.Fatalf("zero-worker frozen probe projection=%+v source=%+v err=%v", legacyProjection, legacyResult, err)
	}
	for name, mutate := range map[string]func(*proofrun.TestRunRequest, *proofrun.TestResult){
		"normal worker": func(r *proofrun.TestRunRequest, _ *proofrun.TestResult) { r.SyntheticProbe = false },
		"green":         func(_ *proofrun.TestRunRequest, r *proofrun.TestResult) { r.Delivery.Sufficient = true },
		"multiple":      func(_ *proofrun.TestRunRequest, r *proofrun.TestResult) { r.Groups = append(r.Groups, r.Groups[0]) },
		"fresh":         func(r *proofrun.TestRunRequest, _ *proofrun.TestResult) { r.FreshnessEpisode = strings.Repeat("f", 64) },
		"reused":        func(_ *proofrun.TestRunRequest, r *proofrun.TestResult) { r.Groups[0].ReuseAttempt = "forged" },
	} {
		t.Run(name, func(t *testing.T) {
			r, candidate := request, result
			candidate.Groups = append([]proofrun.GroupResult(nil), result.Groups...)
			mutate(&r, &candidate)
			if _, err := frozenNegativeProbeResponse(r, candidate); err == nil {
				t.Fatalf("%s projected into legacy probe response", name)
			}
		})
	}
}

func TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable(t *testing.T) {
	root := t.TempDir()
	group := testpolicy.Group{ID: "policy-protection", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"source.go"},
		Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{"testing-policy-protected"}, Platforms: []string{"any"},
		TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`["TestGuard"]`)}
	smoke := testpolicy.Group{ID: "candidate-smoke", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"source.go"},
		Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{"candidate-smoke", "testing-policy-protected"}, Platforms: []string{"any"},
		TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`["TestSmoke"]`)}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "testing-policy", Paths: []string{"testing.json", "scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
			{ID: "proof-and-landing", Paths: []string{"source.go"}, DependsOn: []string{"testing-policy"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
		}, Groups: []testpolicy.Group{group, smoke}, Always: testpolicy.Always{Canary: []string{"policy-protection", "candidate-smoke"}, Standard: []string{"policy-protection", "candidate-smoke"}},
		Unknown: []string{"policy-protection", "candidate-smoke"}, Cadence: []string{"policy-protection", "candidate-smoke"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\nmetasystem.runtimes=fake\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "source.go"), []byte("package fixture\n"), 0o644)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/app":80.0}}`), 0o644)
	}
	writeTestingFixtureFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\nbin/\n"), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	head, unborn, err := (gittree.Workspace{Dir: root}).HeadCommit()
	if err != nil || unborn {
		t.Fatal(err)
	}
	const landingRef = "refs/remotes/origin/main"
	testingFixtureGit(t, root, "update-ref", landingRef, head)
	testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)
	refsBefore := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)")
	configBefore := testingFixtureGit(t, root, "config", "--local", "--list")
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command(goPath, "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+head, "-o", engine, ".")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build public-v1 probe candidate: %v\n%s", buildErr, output)
	}
	engine, err = canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.TestRunRequest{ControlRoot: root, ProjectRoot: root, PolicyBaseCommit: head, PolicyEngine: engine, PolicyEngineDigest: digest,
		CandidateEngine: engine, CandidateEngineDigest: digest, Environment: testingEnvironment(os.Environ())}
	for _, probe := range testpolicy.FrozenProtectionProbeCases()[:4] {
		t.Run(probe.ID, func(t *testing.T) {
			if err := runFrozenSelectionProbe(context.Background(), request, probe); err != nil {
				t.Fatal(err)
			}
			if refsAfter := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)"); refsAfter != refsBefore {
				t.Fatalf("successful protected probe changed caller refs:\nbefore:\n%safter:\n%s", refsBefore, refsAfter)
			}
			if configAfter := testingFixtureGit(t, root, "config", "--local", "--list"); configAfter != configBefore {
				t.Fatalf("successful protected probe changed caller configuration:\nbefore:\n%safter:\n%s", configBefore, configAfter)
			}
		})
	}
	failedRequest := request
	failedRequest.PolicyBaseCommit = strings.Repeat("b", 40)
	if err := runFrozenSelectionProbe(context.Background(), failedRequest, testpolicy.FrozenProtectionProbeCases()[0]); err == nil {
		t.Fatal("protected probe accepted a destination that differed from the real caller ref")
	}
	if refsAfter := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)"); refsAfter != refsBefore {
		t.Fatalf("failed protected probe changed caller refs:\nbefore:\n%safter:\n%s", refsBefore, refsAfter)
	}
	if configAfter := testingFixtureGit(t, root, "config", "--local", "--list"); configAfter != configBefore {
		t.Fatalf("failed protected probe changed caller configuration:\nbefore:\n%safter:\n%s", configBefore, configAfter)
	}
}

func TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker(t *testing.T) {
	// The module root is the metasystem directory wherever the package sits:
	// under the repository (<repo>/metasystem) or at the root of the gate's
	// extracted snapshot, which has no parent repository around it.
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source, err := proofrun.Freeze(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	t.Setenv("GIT_AUTHOR_DATE", "2026-09-21T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-09-21T00:00:00Z")
	engine := filepath.Join(t.TempDir(), "metasystem")
	builtForCommit := ""
	now := time.Now().UTC()
	for _, layout := range frozenCorpusSourceLayouts() {
		t.Run(layout.name, func(t *testing.T) {
			layoutRoot := materializeFrozenCorpusLayout(t, source.Root, layout)
			runFrozenPublicVersionOneCorpus(t, layoutRoot, engine, &builtForCommit, now)
		})
	}
}

type frozenCorpusSourceLayout struct {
	name, prefix string
	git          bool
}

func frozenCorpusSourceLayouts() []frozenCorpusSourceLayout {
	return []frozenCorpusSourceLayout{
		{name: "standalone-no-git"},
		{name: "project-root", git: true},
		{name: "arbitrary-nested-prefix", prefix: "tools/custom-installation", git: true},
	}
}

func materializeFrozenCorpusLayout(t *testing.T, sourceRoot string, layout frozenCorpusSourceLayout) string {
	t.Helper()
	project := t.TempDir()
	root := project
	if layout.prefix != "" {
		root = filepath.Join(project, filepath.FromSlash(layout.prefix))
	}
	copyFrozenCorpusTree(t, sourceRoot, root)
	if layout.git {
		testingFixtureGit(t, project, "init", "-q", "-b", "main")
		testingFixtureGit(t, project, "add", ".")
		testingFixtureGit(t, project, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "source layout")
	}
	return root
}

func runFrozenPublicVersionOneCorpus(t *testing.T, sourceRoot, engine string, builtForCommit *string, now time.Time) {
	t.Helper()
	frozen, err := proofrun.Freeze(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = frozen.Close() })
	projectRoot, root := normalizeFrozenCorpusSource(t, frozen.Root)
	testingFixtureGit(t, projectRoot, "config", "user.name", "Test")
	testingFixtureGit(t, projectRoot, "config", "user.email", "test@example.invalid")
	if err := os.Remove(filepath.Join(root, "testing.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	testingFixtureGit(t, projectRoot, "add", "-A")
	testingFixtureGit(t, projectRoot, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "source without testing contract")
	base := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	if _, present, err := (gittree.Workspace{Dir: projectRoot}).FileAt(base, "metasystem/testing.json"); err != nil || present {
		t.Fatalf("first-transition base contains a testing contract: present=%v err=%v", present, err)
	}

	command := `mkdir -p reports; printf '%s\n' '<testsuite><testcase classname="protection" name="required"/></testsuite>' > reports/result.xml`
	group := testpolicy.Group{ID: "policy-protection", Kind: "unit", Adapter: "command", CWD: "metasystem",
		Inputs: []string{"metasystem/testing.json", "metasystem/internal/testpolicy/**"}, Outputs: []string{"metasystem/reports"}, Obligations: []string{"testing-policy-protected"},
		Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"sh", "-c", command}, Reports: []string{"metasystem/reports"}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: "metasystem/reports/result.xml", Classname: "protection", Name: "required"}}}
	smoke := testpolicy.Group{ID: "candidate-smoke", Kind: "unit", Adapter: "command", CWD: "metasystem",
		Inputs: []string{"metasystem/testing.json"}, Outputs: []string{"metasystem/reports-smoke"}, Obligations: []string{"candidate-smoke", "testing-policy-protected"}, Platforms: []string{"any"}, TargetMS: 1000,
		Argv:    []string{"sh", "-c", `mkdir -p reports-smoke; printf '%s\n' '<testsuite><testcase classname="candidate" name="smoke"/></testsuite>' > reports-smoke/result.xml`},
		Reports: []string{"metasystem/reports-smoke"}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "metasystem/reports-smoke/result.xml", Classname: "candidate", Name: "smoke"}}}
	groups := []testpolicy.Group{group, smoke}
	for index, id := range []string{"fast-static-build", "section/dispatcher-adapter-and-mission-runner-fixtures", "section/goal-cli-fixtures", "section/land-fixtures", "section/adoption-fixtures"} {
		reportDir := fmt.Sprintf("reports-transition-%d", index)
		projectReportDir := "metasystem/" + reportDir
		fixtureCommand := fmt.Sprintf(`test -s testing.json && mkdir -p %s && printf '%%s\n' '<testsuite><testcase classname="transition" name="case-%d"/></testsuite>' > %s/result.xml`, reportDir, index, reportDir)
		groups = append(groups, testpolicy.Group{ID: id, Kind: "integration", Adapter: "command", CWD: "metasystem",
			Inputs: []string{"metasystem/testing.json"}, Outputs: []string{projectReportDir}, Obligations: []string{"first-transition-" + id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"sh", "-c", fixtureCommand}, Reports: []string{projectReportDir}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: projectReportDir + "/result.xml", Classname: "transition", Name: fmt.Sprintf("case-%d", index)}}})
	}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "testing-policy", Paths: []string{"metasystem/testing.json", "metasystem/metasystem.conf", "metasystem/.gitignore", "metasystem/plans/goals/**", "metasystem/internal/testpolicy/**", "metasystem/scripts/agents/coverage-ratchet.json", "metasystem/scripts/agents/coverage-ratchet-linux.json"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
			{ID: "proof-and-landing", Paths: []string{"metasystem/cmd/metasystem/test.go"}, DependsOn: []string{"testing-policy"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
		}, Groups: groups, Always: testpolicy.Always{Canary: []string{"policy-protection", "candidate-smoke"}, Standard: []string{"policy-protection", "candidate-smoke"}},
		Unknown: []string{"policy-protection", "candidate-smoke"}, Cadence: []string{"policy-protection", "candidate-smoke"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	confPath := filepath.Join(root, "metasystem.conf")
	writeTestingFixtureFile(t, confPath, []byte("metasystem.version=1\nmetasystem.runtimes=fake\nrole.code-critic.runtime=fake\ntesting.contract=testing.json\ndispatch.cap-min=1\ndispatch.cap-max=120\n"), 0o644)
	proofFixture := pinProofBinaryFixture(t, root)
	ignorePath := filepath.Join(root, ".gitignore")
	ignore, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, ignorePath, append(ignore, []byte("reports*/\n")...), 0o644)
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture runs one bounded policy corpus."}
	// Six frozen cases through a real worker take about two and a half
	// minutes under the race detector on a loaded box; a one-minute cap made
	// this a wall-clock test that failed only inside the race gate.
	budget := &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 6, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	intent := "Run the frozen policy corpus through the first testing transition."
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FB0", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	goalFile := &goal.GoalFile{Id: "policy-corpus", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
		NextStep: "Run the authenticated worker.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3, Budget: budget,
		Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "policy-corpus", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB3", "fixture-machine", "policy-corpus"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1},
		History: []goal.HistoryLine{
			{At: now.Add(-2 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB1", "fixture-machine", "policy-corpus"), Verb: "open", Actor: "fixture-machine+policy-corpus", Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB2", "fixture-machine", "policy-corpus"), Verb: "claim", Actor: "fixture-machine+policy-corpus", Keep: -1},
			{At: now.Add(-30 * time.Second).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB3", "fixture-machine", "policy-corpus"), Verb: "approve", Actor: "human:fixture", Keep: -1},
		}}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "policy-corpus.md"), goal.RenderFile(goalFile), 0o644)
	testingFixtureGit(t, projectRoot, "config", "goal.sync-remote", "local")
	testingFixtureGit(t, projectRoot, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	testingFixtureGit(t, projectRoot, "add", "-A", "metasystem/testing.json", "metasystem/metasystem.conf", "metasystem/.gitignore", "metasystem/internal/testpolicy", "metasystem/plans/goals")
	testingFixtureGit(t, projectRoot, "commit", "-qm", "reviewed testing contract")
	candidateCommit := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	candidate := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD^{tree}"))
	testingFixtureGit(t, projectRoot, "update-ref", "refs/remotes/origin/main", base)
	testingFixtureGit(t, projectRoot, "update-ref", goal.LocalLedgerBranch, candidateCommit)
	testingFixtureGit(t, projectRoot, "update-ref", goal.AcceptedRef, candidateCommit)
	testingFixtureGit(t, projectRoot, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/origin/main")

	if *builtForCommit == "" {
		build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
			"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+candidateCommit, "-o", engine, ".")
		build.Dir = filepath.Join(root, "cmd", "metasystem")
		build.Env = gittree.ScrubbedEnviron()
		if output, buildErr := build.CombinedOutput(); buildErr != nil {
			t.Fatalf("build first-transition worker: %v\n%s", buildErr, output)
		}
		*builtForCommit = candidateCommit
	} else if candidateCommit != *builtForCommit {
		t.Fatalf("normalized source layout candidate commit=%s, shared engine commit=%s", candidateCommit, *builtForCommit)
	}
	identityTable := filepath.Join(t.TempDir(), "process-identities.json")
	identities := map[string]map[string]any{
		fmt.Sprint(os.Getpid()): {"terminal": true},
	}
	seen := map[int64]bool{int64(os.Getpid()): true}
	current, ok := identity.ParentPid(int64(os.Getpid()))
	for ok && !seen[current] {
		seen[current] = true
		identities[fmt.Sprint(current)] = map[string]any{"pidStartedAt": 1, "command": "fixture-neutral-ancestor"}
		current, ok = identity.ParentPid(current)
	}
	identityData, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityTable, identityData, 0o600); err != nil {
		t.Fatal(err)
	}
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	if !fixtureauth.FixtureModeRoot(root) {
		t.Fatalf("frozen first-transition root %s does not authorize its synthetic fake-runtime admission", root)
	}
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	if _, selected, err := proofrun.FixtureHostAdmissionDirectory(root); err != nil || !selected {
		t.Fatalf("frozen admission namespace %s under temp %s is not usable: selected=%v err=%v", admissionDir, os.TempDir(), selected, err)
	}
	processTable := filepath.Join(t.TempDir(), "processes.json")
	writeTestingFixtureFile(t, processTable, []byte("[]\n"), 0o600)
	fixtureEnvironment := append(receiptCanaryEnvironment(),
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable,
		"METASYSTEM_CENSUS_PROCESS_FILE="+processTable,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root)
	// Public admission owns the attempt, authenticated worker, input parity,
	// and atomic terminal receipt. A manually reserved parent would bind a
	// different source/configuration context from the actual testing command.
	public := proofFixture.command(fixtureEnvironment, engine, "test", "run", "--root", root, "--tree", candidate,
		"--mode", "auto", "--purpose", "delivery", "--goal", "policy-corpus", "--cap-min", "5")
	public.Dir = projectRoot
	output, runErr := public.CombinedOutput()
	if runErr != nil {
		t.Fatalf("authenticated first-transition worker did not complete all six frozen cases: %v\n%s", runErr, output)
	}
	if _, err := os.Stat(filepath.Join(admissionDir, "admission.lock")); err != nil {
		t.Fatalf("public worker did not use its authenticated temporary admission directory: %v", err)
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("first-transition reservation count=%d err=%v", len(attempts), err)
	}
	stored := attempts[0]
	if stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalSuccess ||
		stored.TestResult == nil || !stored.TestResult.Delivery.Sufficient || len(stored.TestResult.Groups) != 7 ||
		len(stored.DeliveryReceiptBytes) == 0 {
		t.Fatalf("first-transition worker result is incomplete: attempt=%+v", stored)
	}
}

func TestFrozenPublicVersionOneCorpusNormalizesSupportedSourceLayouts(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(source, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(source, ".gitignore"), []byte("reports*/\n"), 0o644)
	for _, test := range frozenCorpusSourceLayouts() {
		t.Run(test.name, func(t *testing.T) {
			writeTestingFixtureFile(t, filepath.Join(source, "internal", "shape.txt"), []byte(test.name+"\n"), 0o644)
			root := materializeFrozenCorpusLayout(t, source, test)
			if got := strings.TrimSpace(string(mustReadTestingFixtureFile(t, filepath.Join(root, ".gitignore")))); got != "reports*/" {
				t.Fatalf("copied ignore file = %q", got)
			}
			if got := strings.TrimSpace(string(mustReadTestingFixtureFile(t, filepath.Join(root, "internal", "shape.txt")))); got != test.name {
				t.Fatalf("copied source marker = %q, want %q", got, test.name)
			}
			frozen, err := proofrun.Freeze(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = frozen.Close() })
			privateProject, privateRoot := normalizeFrozenCorpusSource(t, frozen.Root)
			if privateRoot != filepath.Join(privateProject, "metasystem") {
				t.Fatalf("normalized root = %s beneath %s", privateRoot, privateProject)
			}
			if got := strings.TrimSpace(string(mustReadTestingFixtureFile(t, filepath.Join(privateRoot, "internal", "shape.txt")))); got != test.name {
				t.Fatalf("normalized source marker = %q, want %q", got, test.name)
			}
			wantProject, err := canonicalPath(privateProject)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(testingFixtureGit(t, privateProject, "rev-parse", "--show-toplevel")); got != wantProject {
				t.Fatalf("normalized Git root = %q, want %q", got, wantProject)
			}
		})
	}
}

func copyFrozenCorpusTree(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cp", "-R", sourceRoot+string(filepath.Separator)+".", destinationRoot)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("copy frozen corpus source: %v\n%s", err, output)
	}
	if err := os.RemoveAll(filepath.Join(destinationRoot, ".git")); err != nil {
		t.Fatal(err)
	}
}

func normalizeFrozenCorpusSource(t *testing.T, sourceRoot string) (string, string) {
	t.Helper()
	projectRoot := t.TempDir()
	projectRoot, err := canonicalPath(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(projectRoot, "metasystem")
	copyFrozenCorpusTree(t, sourceRoot, root)
	testingFixtureGit(t, projectRoot, "init", "-q", "-b", "main")
	return projectRoot, root
}

func mustReadTestingFixtureFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine(t *testing.T) {
	root := t.TempDir()
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"source.txt"}, Standard: []string{"app"}, Critical: []string{"app"}}},
		Groups: []testpolicy.Group{{ID: "app", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"}, Outputs: []string{"reports"},
			Obligations: []string{"app"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"false"}, Reports: []string{"reports"}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "app", Name: "required"}}}},
		Always: testpolicy.Always{Canary: []string{"app"}}, Unknown: []string{"app"}, Cadence: []string{"app"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644)
	proofFixture := pinProofBinaryFixture(t, root)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "source.txt"), []byte("source\n"), 0o644)
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	head := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	const landingRef = "refs/remotes/origin/main"
	testingFixtureGit(t, root, "update-ref", landingRef, head)
	testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)
	t.Setenv("METASYSTEM_TRUSTED_POLICY_DECISION", "1")
	prepared, err := prepareTesting(testingSelectionRequest{Root: root, Tree: tree, Mode: testpolicy.ModeStandard, Purpose: testpolicy.PurposeDiagnostic})
	if err == nil && (!strings.HasPrefix(prepared.JudgeKey, proofrun.JudgeCompatibilityVersion+":") || strings.Contains(prepared.JudgeKey, ":unreadable:")) {
		t.Fatalf("preparation did not compute a readable judge key: %q", prepared.JudgeKey)
	}
	if err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("ordinary caller bypassed retained engine authentication with an ambient flag: %v", err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", engine, ".")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build public flag-negative engine: %v\n%s", buildErr, output)
	}
	public := proofFixture.command(nil, engine, "test", "plan", "--root", root, "--tree", tree, "--mode", "standard", "--purpose", "diagnostic")
	public.Env = append(public.Env, "METASYSTEM_TRUSTED_POLICY_DECISION=1")
	output, publicErr := public.CombinedOutput()
	if publicErr == nil || !strings.Contains(string(output), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("public caller bypassed retained engine authentication with an ambient flag: err=%v output=%s", publicErr, output)
	}
}

func TestTestListCheckPlanAndVerifyWithoutLaunching(t *testing.T) {
	// This package test is itself not a dispatching metasystem executable. The
	// separate retained-engine process boundary is covered by the public
	// binary fixture; this test covers zero application launches.
	root := t.TempDir()
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(root, ".git", "objects"))
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(root, ".git", "objects"))
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	pathOnly := filepath.Join(root, "path")
	if err := os.Mkdir(pathOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"git": gitPath, "go": goPath, "sh": shPath, "tar": tarPath} {
		if err := os.Symlink(target, filepath.Join(pathOnly, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", pathOnly)
	marker := filepath.Join(root, "test-command-launched")
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"src/**"}, Standard: []string{"app-command"}, Critical: []string{"app-output"}}},
		Groups: []testpolicy.Group{{ID: "app-command", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"src/**"}, Outputs: []string{"reports"},
			Tools: []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf shell-v1"}}}, Obligations: []string{"app-output"},
			Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"sh", "-c", "touch " + marker}, Reports: []string{"reports"},
			Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "app", Name: "smoke"}}}},
		Always: testpolicy.Always{Canary: []string{"app-command"}}, Unknown: []string{"app-command"}, Cadence: []string{"app-command"}}
	contractBytes, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\nmetasystem.runtimes=fake\n"), 0o644)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(root, "admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), contractBytes, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "src", "output.txt"), []byte("v1\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\n"), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	head, unborn, err := (gittree.Workspace{Dir: root}).HeadCommit()
	if err != nil || unborn {
		t.Fatal(err)
	}
	currentEngine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command(goPath, "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+head, "-o", currentEngine, ".")
	build.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build source-bound policy engine: %v\n%s", buildErr, output)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(currentEngine, filepath.Join(root, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(currentEngine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(currentEngine)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "peer.md"), []byte("ledger-only destination advancement\n"), 0o644)
	testingFixtureGit(t, root, "add", "plans/goals/peer.md")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "ledger-only destination advancement")
	recordOnlyDestination := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", recordOnlyDestination)
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
		InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-09T00:00:00Z", Enrollment: steward.EnrollmentFixture,
		MintedBy: "machine-rebuild", EngineBuild: head[:12], LandedCommit: recordOnlyDestination, LandingRef: "refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "--verify", head+"^{commit}")); got != head {
		t.Fatalf("fixture source commit moved: got=%s want=%s", got, head)
	}
	if _, _, _, err := trustedPolicyEngine(root, recordOnlyDestination, false); err != nil {
		t.Fatalf("record-only destination advancement did not reuse the genuinely source-bound engine built at %s: %v", head, err)
	}
	tree, err := (gittree.Workspace{Dir: root}).HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	if status := runTestList([]string{"--root", root, "--json"}); status != 0 {
		t.Fatalf("test list status = %d", status)
	}
	if status := runTestCheck([]string{"--root", root, "--json"}); status != 0 {
		t.Fatalf("test check status = %d", status)
	}
	if status := runTestPlan([]string{"--root", root, "--tree", tree, "--purpose", "diagnostic", "--json"}); status != 0 {
		t.Fatalf("test plan status = %d", status)
	}
	if status := runTestVerify([]string{"--root", root, "--tree", tree, "--purpose", "diagnostic", "--json"}); status != 1 {
		t.Fatalf("test verify without proof status = %d, want 1", status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("metadata-only commands launched the declared test command: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "internal", "policy.txt"), []byte("changed engine input\n"), 0o644)
	testingFixtureGit(t, root, "add", "internal/policy.txt")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "change engine projection")
	changedEngineDestination := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	if _, _, _, err := trustedPolicyEngine(root, changedEngineDestination, false); err == nil || !strings.Contains(err.Error(), "different ENGINE projections") {
		t.Fatalf("destination with changed engine inputs reused an older policy engine: %v", err)
	}
}

func writeTestingFixtureFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func testingFixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
