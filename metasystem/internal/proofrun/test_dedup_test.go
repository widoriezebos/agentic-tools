package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const dedupModule = "github.com/widoriezebos/agentic-tools/metasystem"

func dedupFixtureFiles(alphaTwo string) map[string]testSnapshotEntry {
	return map[string]testSnapshotEntry{
		"metasystem/go.mod": testSnapshotFile("module "+dedupModule+"\n\ngo 1.22\n", 0o644),
		"metasystem/internal/alpha/alpha_test.go": testSnapshotFile(`package alpha

import "testing"

func TestA1(t *testing.T) { t.Run("sub", func(t *testing.T) {}) }
func TestA2(t *testing.T) { `+alphaTwo+` }
func TestA3(t *testing.T) {}
`, 0o644),
		"metasystem/internal/beta/beta_test.go": testSnapshotFile(`package beta

import "testing"

func TestB1(t *testing.T) {}
func TestB2(t *testing.T) {}
`, 0o644),
	}
}

func dedupGroup(id string, packages []string, tests string, tags ...string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs:      []string{"metasystem/go.mod", "metasystem/internal/**"},
		Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 60000,
		Packages: packages, Tests: []byte(tests), BuildTags: tags}
}

// runDedupStage prepares and runs one stage through the real scheduler and
// group runner, returning results by group and native `run` events per test
// counted from the go test JSON shard logs.
func runDedupStage(t *testing.T, alphaTwo string, groups []testpolicy.Group) (map[string]GroupResult, map[string]int, TestRunRequest, time.Duration) {
	t.Helper()
	root := t.TempDir()
	snapshot := newTestSnapshotFactory(t, root, strings.Repeat("d", 40), dedupFixtureFiles(alphaTwo), 1+len(groups))
	byID := map[string]testpolicy.Group{}
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
		ids = append(ids, group.ID)
	}
	request := TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: snapshot.tree,
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-mod=readonly -buildvcs=false"), LogRoot: filepath.Join(root, "logs"),
		Workers: 2, Concurrency: len(groups), openCandidate: snapshot.open,
		Contract: testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, Groups: groups}, Plan: testpolicy.Plan{SelectedGroups: ids}}
	identities, prepared, _, err := PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreparedGroups, request.ComponentIdentities = prepared, identities
	started := time.Now()
	results, _, err := runStageGroups(context.Background(), request, byID, ids, &progressWriter{}, false)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]GroupResult{}
	for _, result := range results {
		out[result.ID] = result
	}
	runs := map[string]int{}
	logs, err := filepath.Glob(filepath.Join(request.LogRoot, "*.shard-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range logs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, `"Action":"run"`) {
				continue
			}
			for _, name := range []string{"TestA1", "TestA2", "TestA3", "TestB1", "TestB2"} {
				if strings.Contains(line, `"Test":"`+name+`"}`) {
					runs[name]++
				}
			}
		}
	}
	return out, runs, request, elapsed
}

func dedupTestResult(attemptID string, groups ...GroupResult) TestResult {
	result := componentAttemptResult(attemptID, groups[0].ID, groups[0].ExecutionIdentity, "passed")
	admissionMaximum := 0
	result.SchemaVersion, result.WorkerPolicyVersion, result.Workers, result.AdmissionMaximum = TestResultSchemaVersion, TestWorkerPolicyVersion, 2, &admissionMaximum
	result.SelectedGroups, result.RequiredGroups, result.Groups = nil, nil, nil
	result.LaunchCounts.Test = 0
	for _, group := range groups {
		result.SelectedGroups = append(result.SelectedGroups, group.ID)
		result.RequiredGroups = append(result.RequiredGroups, group.ID)
		result.Groups = append(result.Groups, group)
		if group.NativeLaunched {
			result.LaunchCounts.Test++
		}
	}
	result.RecomputeDelivery()
	return result
}

func cloneGroupResult(group GroupResult) GroupResult {
	group.Expected = append([]NativeTestIdentity(nil), group.Expected...)
	group.Observed = append([]NativeTestIdentity(nil), group.Observed...)
	group.CoveredByGroups = append([]string(nil), group.CoveredByGroups...)
	group.CoveredTests = append([]NativeTestIdentity(nil), group.CoveredTests...)
	group.ToolIdentities = cloneStringMap(group.ToolIdentities)
	group.ExecutableDigests = cloneStringMap(group.ExecutableDigests)
	return group
}

func TestGoDedupRunsEachRequiredTestOnceAndKeepsDistinctConfigurations(t *testing.T) {
	t.Parallel()
	results, runs, _, elapsed := runDedupStage(t, "", []testpolicy.Group{
		dedupGroup("alpha-all", []string{"internal/alpha"}, `"all"`),
		dedupGroup("alpha-named", []string{"internal/alpha"}, `["TestA1","TestA3"]`),
		dedupGroup("span-named", []string{"internal/alpha", "internal/beta"}, `["TestA2","TestB1"]`),
		dedupGroup("alpha-tagged", []string{"internal/alpha"}, `["TestA1"]`, "dedupfixture"),
		dedupGroup("beta-named", []string{"internal/beta"}, `["TestB1"]`),
	})
	t.Logf("stage wall time %s; native run events %v", elapsed, runs)
	for id, result := range results {
		if result.Status != "passed" || !result.CollectionComplete {
			t.Fatalf("%s did not pass: %+v", id, result)
		}
	}
	// Without deduplication TestA1 runs three times and TestA2, TestA3 twice.
	// The tagged group is a distinct native configuration and runs TestA1
	// again; TestB2 is selected by no group.
	if want := map[string]int{"TestA1": 2, "TestA2": 1, "TestA3": 1, "TestB1": 1}; !reflect.DeepEqual(runs, want) {
		t.Fatalf("native run events=%v, want %v", runs, want)
	}
	source := results["alpha-all"]
	if !source.NativeLaunched || len(source.CoveredByGroups) != 0 || source.NativeContext == "" {
		t.Fatalf("broad source lost native evidence: %+v", source)
	}
	named := results["alpha-named"]
	if named.NativeLaunched || named.NativeExitStatus != nil || !reflect.DeepEqual(named.CoveredByGroups, []string{"alpha-all"}) ||
		len(named.CoveredTests) != 2 || len(named.Expected) != 2 || named.NativeContext != source.NativeContext {
		t.Fatalf("covered group shape: %+v", named)
	}
	observed := map[string]string{}
	for _, identity := range named.Observed {
		observed[identity.Name] = identity.Status
	}
	if want := map[string]string{"TestA1": "passed", "TestA1/sub": "passed", "TestA3": "passed"}; !reflect.DeepEqual(observed, want) {
		t.Fatalf("covered group observed=%v, want source terminals %v", observed, want)
	}
	if data, err := os.ReadFile(named.LogPath); err != nil || !strings.Contains(string(data), "TEST-COVERED alpha-named by alpha-all") ||
		named.LogDigest != digestBytes(data) {
		t.Fatalf("covered group log: err=%v digest=%s\n%s", err, named.LogDigest, data)
	}
	span := results["span-named"]
	if !span.NativeLaunched || !reflect.DeepEqual(span.CoveredByGroups, []string{"alpha-all"}) || len(span.CoveredTests) != 1 ||
		span.CoveredTests[0].Name != "TestA2" || len(span.Observed) != 2 || len(span.Expected) != 2 {
		t.Fatalf("partial overlap did not run the residual only: %+v", span)
	}
	tagged := results["alpha-tagged"]
	if !tagged.NativeLaunched || len(tagged.CoveredByGroups) != 0 || tagged.NativeContext != "" {
		t.Fatalf("distinct build tags shared evidence: %+v", tagged)
	}
	// A partly covered group lends the terminals it ran natively.
	beta := results["beta-named"]
	if beta.NativeLaunched || !reflect.DeepEqual(beta.CoveredByGroups, []string{"span-named"}) || len(beta.CoveredTests) != 1 {
		t.Fatalf("native residual terminal was not lent: %+v", beta)
	}
	result := dedupTestResult("attempt-dedup", source, named, span, tagged, beta)
	if err := ValidateTestResult(result); err != nil || !result.Delivery.Sufficient {
		t.Fatalf("runner result is not valid covered evidence: err=%v delivery=%+v", err, result.Delivery)
	}

	mutations := []struct {
		name   string
		mutate func(groups map[string]*GroupResult)
		want   string
	}{
		{"failed source", func(g map[string]*GroupResult) {
			exit := 1
			g["alpha-all"].Status, g["alpha-all"].NativeExitStatus = "failed", &exit
		}, "not a complete native pass"},
		{"cancelled source", func(g map[string]*GroupResult) { g["alpha-all"].Status = "cancelled" }, "not a complete native pass"},
		{"missing source terminal", func(g map[string]*GroupResult) {
			var kept []NativeTestIdentity
			for _, identity := range g["alpha-all"].Observed {
				if identity.Name != "TestA3" {
					kept = append(kept, identity)
				}
			}
			g["alpha-all"].Observed = kept
		}, "without a passing source terminal"},
		{"skipped source terminal", func(g map[string]*GroupResult) {
			for index, identity := range g["alpha-all"].Observed {
				if identity.Name == "TestA2" {
					g["alpha-all"].Observed[index].Status = "skipped"
				}
			}
		}, "without a passing source terminal"},
		{"modified environment", func(g map[string]*GroupResult) { g["alpha-named"].EnvironmentDigest = strings.Repeat("e", 64) }, "different native context"},
		{"modified context", func(g map[string]*GroupResult) { g["span-named"].NativeContext = strings.Repeat("f", 64) }, "different native context"},
		{"modified tools", func(g map[string]*GroupResult) { g["alpha-all"].ToolIdentities["go"] = "other" }, "different native context"},
		{"coverage cycle", func(g map[string]*GroupResult) {
			g["alpha-all"].CoveredByGroups = []string{"span-named"}
			g["alpha-all"].CoveredTests = []NativeTestIdentity{g["span-named"].CoveredTests[0]}
		}, "coverage cycle"},
		{"credited terminal cannot lend", func(g map[string]*GroupResult) {
			a2 := g["span-named"].CoveredTests[0]
			named := g["alpha-named"]
			named.Expected = append(named.Expected, NativeTestIdentity{Classname: a2.Classname, Name: a2.Name, Status: "expected"})
			named.Observed = append(named.Observed, a2)
			named.CoveredTests = append(named.CoveredTests, a2)
			named.CoveredByGroups = []string{"alpha-all", "span-named"}
		}, "unused coverage source span-named"},
		{"absent source", func(g map[string]*GroupResult) { g["alpha-named"].CoveredByGroups = []string{"elsewhere"} }, "absent or unordered"},
		{"differently tagged source", func(g map[string]*GroupResult) {
			g["alpha-named"].CoveredByGroups = []string{"alpha-all", "alpha-tagged"}
		}, "different native context"},
		{"unused source", func(g map[string]*GroupResult) {
			g["alpha-tagged"].NativeContext = g["span-named"].NativeContext
			g["span-named"].CoveredByGroups = []string{"alpha-all", "alpha-tagged"}
		}, "unused coverage source alpha-tagged"},
		{"partial claim without launch", func(g map[string]*GroupResult) {
			g["alpha-named"].CoveredTests = g["alpha-named"].CoveredTests[:1]
		}, "incomplete native evidence"},
		{"uncredited expectation", func(g map[string]*GroupResult) {
			g["span-named"].CoveredTests = append(g["span-named"].CoveredTests, NativeTestIdentity{Report: "go-test-json",
				Classname: dedupModule + "/internal/beta", Name: "TestB2", Status: "passed"})
		}, "without its own passing expectation"},
		{"reused copy keeps coverage", func(g map[string]*GroupResult) {
			g["alpha-named"].Status, g["alpha-named"].ReuseAttempt = "reused", "attempt-elsewhere"
		}, "same-result coverage"},
	}
	for _, mutation := range mutations {
		groups := map[string]*GroupResult{}
		ordered := []GroupResult{cloneGroupResult(source), cloneGroupResult(named), cloneGroupResult(span), cloneGroupResult(tagged), cloneGroupResult(beta)}
		for index := range ordered {
			groups[ordered[index].ID] = &ordered[index]
		}
		mutation.mutate(groups)
		mutated := dedupTestResult("attempt-dedup", ordered...)
		if err := ValidateTestResult(mutated); err == nil || !strings.Contains(err.Error(), mutation.want) {
			t.Fatalf("%s: validation err=%v, want %q", mutation.name, err, mutation.want)
		}
	}

	f := newOwnershipFixture(t)
	identities := map[string]string{"alpha-all": source.ExecutionIdentity, "alpha-named": named.ExecutionIdentity, "span-named": span.ExecutionIdentity}
	attempt := f.reserveExecuted("goal-dedup", "plan-dedup", identities, "", "", 0)
	if len(attempt.TestOwned) != 3 {
		t.Fatalf("fixture did not own every group: %+v", attempt.TestOwned)
	}
	retained := dedupTestResult(attempt.AttemptID, source, named, span)
	if _, err := FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, TerminalSuccess, 0, "fixture", nil, &retained, f.now.Add(time.Second)); err != nil {
		t.Fatalf("retain covered result: %v", err)
	}
	reloaded, err := ReadAttempt(f.root, attempt.AttemptID)
	if err != nil || reloaded.TestResult == nil || !reflect.DeepEqual(reloaded.TestResult.Groups[1].CoveredTests, named.CoveredTests) {
		t.Fatalf("reload covered result: err=%v", err)
	}
	if !TestGroupProducer(reloaded, *reloaded.TestResult, reloaded.TestResult.Groups[1]) || NativeTestProducer(reloaded, reloaded.TestResult.Groups[1]) {
		t.Fatal("covered pass must be an owned producer without becoming a native producer")
	}
	if _, decision := f.reserve("goal-next", "plan-next", map[string]string{"alpha-named": named.ExecutionIdentity}, "", "", 2*time.Second); decision.Disposition != DispositionReusableSuccess {
		t.Fatalf("next admission did not accept the covered pass: %+v", decision)
	}
	next := f.reserveExecuted("goal-mixed", "plan-mixed", map[string]string{"alpha-named": named.ExecutionIdentity, "fresh": strings.Repeat("9", 64)}, "", "", 3*time.Second)
	if next.TestSources["alpha-named"] != attempt.AttemptID || next.TestOwned["fresh"] == "" {
		t.Fatalf("mixed admission did not source the covered pass: %+v", next)
	}
	original, err := nativeSourceGroup(next, GroupResult{ID: "alpha-named", ExecutionIdentity: named.ExecutionIdentity, ReuseAttempt: attempt.AttemptID}, f.now.Add(4*time.Second))
	if err != nil || original.Status != "passed" || !original.CollectionComplete || original.NativeLaunched {
		t.Fatalf("next admission source readback: %+v err=%v", original, err)
	}
}

func TestGoDedupFailedSourceGivesNoCredit(t *testing.T) {
	t.Parallel()
	results, runs, _, _ := runDedupStage(t, `t.Fatal("red")`, []testpolicy.Group{
		dedupGroup("alpha-all", []string{"internal/alpha"}, `"all"`),
		dedupGroup("alpha-named", []string{"internal/alpha"}, `["TestA1","TestA3"]`),
	})
	if results["alpha-all"].Status != "failed" {
		t.Fatalf("red source: %+v", results["alpha-all"])
	}
	named := results["alpha-named"]
	if named.Status != "passed" || !named.NativeLaunched || len(named.CoveredByGroups) != 0 || len(named.CoveredTests) != 0 {
		t.Fatalf("failed source lent credit: %+v", named)
	}
	if want := map[string]int{"TestA1": 2, "TestA2": 1, "TestA3": 2}; !reflect.DeepEqual(runs, want) {
		t.Fatalf("native run events=%v, want %v", runs, want)
	}
}

func dedupStubRequest(ids ...string) (TestRunRequest, map[string]testpolicy.Group) {
	prepared := map[string]PreparedGroupExecution{}
	groups := map[string]testpolicy.Group{}
	for index, id := range ids {
		expected := []NativeTestIdentity{{Classname: "fixture", Name: "TestShared", Status: "expected"}}
		for extra := 0; extra < len(ids)-index; extra++ {
			expected = append(expected, NativeTestIdentity{Classname: "fixture", Name: id + "-" + string(rune('a'+extra)), Status: "expected"})
		}
		prepared[id] = PreparedGroupExecution{EnvironmentDigest: strings.Repeat("e", 64), ToolIdentities: map[string]string{"go": "go"}, Expected: expected}
		groups[id] = testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: "fixture", TargetMS: 1}
	}
	return TestRunRequest{CandidateTree: strings.Repeat("d", 40), Workers: 2, Concurrency: len(ids), PreparedGroups: prepared}, groups
}

// The consumer launches only after its source's terminal, and receives that
// terminal. The source is held until the first admission pass has had every
// free slot, so an eager consumer would have launched with no terminal.
func TestGoDedupSchedulerDefersConsumerUntilSourceTerminal(t *testing.T) {
	t.Parallel()
	request, groups := dedupStubRequest("broad", "narrow")
	release := make(chan struct{})
	var releaseOnce sync.Once
	var consumerSources []GroupResult
	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{
		runGroup: func(ctx context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
			switch group.ID {
			case "broad":
				<-release
				return GroupResult{ID: "broad", Status: "passed", NativeLaunched: true}
			case "narrow":
				consumerSources, _ = coverageSourcesFromContext(ctx)
				return GroupResult{ID: "narrow", Status: "passed", NativeLaunched: true}
			}
			t.Errorf("unknown group dispatched: %s", group.ID)
			return GroupResult{ID: group.ID, Status: "invalid"}
		},
		afterAdmission: func(map[string]GroupResult) { releaseOnce.Do(func() { close(release) }) },
	})
	results, _, err := runStageGroups(ctx, request, groups, []string{"narrow", "broad"}, &progressWriter{}, false)
	if err != nil || results[0].Status != "passed" || results[1].Status != "passed" {
		t.Fatalf("stage results=%+v err=%v", results, err)
	}
	if len(consumerSources) != 1 || consumerSources[0].ID != "broad" || consumerSources[0].Status != "passed" {
		t.Fatalf("consumer did not receive its source terminal: %+v", consumerSources)
	}
}

// Cancelling while the source is active never dispatches the waiting
// consumer, and the consumer's record invents no execution.
func TestGoDedupCancellationLeavesWaitingConsumerUnlaunched(t *testing.T) {
	t.Parallel()
	request, groups := dedupStubRequest("broad", "narrow")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sourceStarted := make(chan struct{})
	go func() {
		<-sourceStarted
		cancel()
	}()
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{
		runGroup: func(ctx context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
			if group.ID != "broad" {
				t.Errorf("group dispatched after cancellation: %s", group.ID)
				return GroupResult{ID: group.ID, Status: "invalid"}
			}
			close(sourceStarted)
			<-ctx.Done()
			return GroupResult{ID: "broad", Status: "cancelled", NativeLaunched: true}
		},
	})
	results, _, err := runStageGroups(ctx, request, groups, []string{"broad", "narrow"}, &progressWriter{}, false)
	if err == nil || results[0].Status != "cancelled" || results[1].Status != "not-run" || results[1].NativeLaunched || results[1].StartedAt != "" {
		t.Fatalf("cancelled stage results=%+v err=%v", results, err)
	}
}

// Coverage waits never close a cycle with prerequisites, and distinct
// contexts or unprepared groups never wait.
func TestGoDedupPlanRespectsPrerequisitesAndContext(t *testing.T) {
	t.Parallel()
	request, groups := dedupStubRequest("broad", "narrow", "other")
	broad := groups["broad"]
	broad.Requires = []string{"narrow"}
	groups["broad"] = broad
	other := groups["other"]
	other.Race = true
	groups["other"] = other
	plan := planStageCoverage(request, groups, []string{"broad", "narrow", "other"})
	if len(plan[0]) != 0 || len(plan[1]) != 0 || len(plan[2]) != 0 {
		t.Fatalf("coverage plan waited across a prerequisite or context: %v", plan)
	}
	delete(request.PreparedGroups, "narrow")
	broad.Requires = nil
	groups["broad"] = broad
	if plan := planStageCoverage(request, groups, []string{"broad", "narrow"}); len(plan[0]) != 0 || len(plan[1]) != 0 {
		t.Fatalf("unprepared group joined coverage: %v", plan)
	}
}

// A different CPU budget or shard count is a different native execution:
// neither group may borrow the broader group's terminals.
func TestGoDedupKeepsDistinctBudgetAndShardConfigurations(t *testing.T) {
	t.Parallel()
	budget := int64(600)
	budgeted := dedupGroup("alpha-budget", []string{"internal/alpha"}, `["TestA1"]`)
	budgeted.CPUBudgetSeconds = &budget
	sharded := dedupGroup("alpha-sharded", []string{"internal/alpha"}, `["TestA3"]`)
	sharded.Shards = 1
	results, runs, request, _ := runDedupStage(t, "", []testpolicy.Group{
		dedupGroup("alpha-all", []string{"internal/alpha"}, `"all"`), budgeted, sharded,
	})
	byID := map[string]testpolicy.Group{}
	for _, group := range request.Contract.Groups {
		byID[group.ID] = group
	}
	if plan := planStageCoverage(request, byID, []string{"alpha-all", "alpha-budget", "alpha-sharded"}); len(plan[1]) != 0 || len(plan[2]) != 0 {
		t.Fatalf("distinct budget or shard configuration planned coverage: %v", plan)
	}
	for _, id := range []string{"alpha-budget", "alpha-sharded"} {
		if result := results[id]; result.Status != "passed" || !result.NativeLaunched || len(result.CoveredByGroups) != 0 {
			t.Fatalf("%s borrowed evidence: %+v", id, result)
		}
	}
	if want := map[string]int{"TestA1": 2, "TestA2": 1, "TestA3": 2}; !reflect.DeepEqual(runs, want) {
		t.Fatalf("native run events=%v, want %v", runs, want)
	}
	if shardLogs, _ := filepath.Glob(filepath.Join(request.LogRoot, "alpha-sharded.shard-*.log")); len(shardLogs) != 1 {
		t.Fatalf("one-shard group launched %d shard processes", len(shardLogs))
	}
}
