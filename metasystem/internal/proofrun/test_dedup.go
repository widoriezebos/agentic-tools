package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Go groups inside one attempt often select the same tests: a whole-package
// group and a named subset spanning that package, for example. A group whose
// expected tests already reached a passing native terminal in a compatible
// group of the same result records those tests as covered and launches only
// its residual selection. Covered evidence never crosses attempts or
// contexts, and a source lends only terminals it produced natively, never
// tests that were credited to it.

const goNativeContextVersion = 1

type coverageSourcesKey struct{}

// withCoverageSources marks a group as taking part in same-result coverage,
// as a source, a consumer, or both, and hands a consumer its sources' terminal
// results. Groups outside any overlap keep their result shape unchanged.
func withCoverageSources(ctx context.Context, sources []GroupResult) context.Context {
	return context.WithValue(ctx, coverageSourcesKey{}, append([]GroupResult{}, sources...))
}

func coverageSourcesFromContext(ctx context.Context) ([]GroupResult, bool) {
	sources, ok := ctx.Value(coverageSourcesKey{}).([]GroupResult)
	return sources, ok
}

// goNativeContext digests everything that changes a native Go execution
// except the test selection. An empty context never shares evidence: coverage
// floors need their own instrumented run, and inputs outside the immutable
// candidate tree can change between two launches.
func goNativeContext(request TestRunRequest, group testpolicy.Group) string {
	prepared, ok := request.PreparedGroups[group.ID]
	if group.Adapter != "go" || group.Coverage || group.Kind == "performance" || group.Kind == "build" ||
		len(group.ExternalInputs) != 0 || !ok || prepared.Unavailable != "" || !validTreeDigest(request.CandidateTree) {
		return ""
	}
	definition := group
	definition.ID, definition.Kind, definition.Phase, definition.Requires = "", "", "", nil
	definition.Obligations, definition.Platforms, definition.Inputs = nil, nil, nil
	definition.Freshness, definition.FreshnessMaxAgeMS = "", nil
	// The CPU budget is a supervisor verdict and the shard count partitions
	// tests into processes, so both stay in the context.
	definition.TargetMS = 0
	definition.Packages, definition.PackageSelection, definition.Tests = nil, "", nil
	definition.Resources.Class, definition.Resources.Exclusive = "", nil
	// The steward package receives the candidate engine. A group that
	// selects it runs every package with that environment.
	stewardEngine := ""
	if filepath.Clean(group.CWD) == filepath.Clean(request.InstallationPrefix) && slices.ContainsFunc(group.Packages, func(pkg string) bool {
		return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(pkg)), "./") == "internal/steward"
	}) {
		stewardEngine = "steward:" + request.CandidateEngineDigest
	}
	groupWorkers, _ := EffectiveGroupWorkers(request, group)
	// The skip-verdict policy judges results and does not change the native
	// command. Only passing terminals are credited, and each consumer judges
	// its merged terminals under its own policy.
	encoded, _ := json.Marshal(struct {
		Version              int
		Definition           testpolicy.Group
		CandidateTree        string
		Environment          string
		Tools                map[string]string
		Executables          map[string]string
		AttemptWorkers       int
		GroupWorkers         int
		StewardEngine        string
		JudgeKey             string
		BehaviorPolicyDigest string
		Platform             string
	}{goNativeContextVersion, definition, request.CandidateTree, prepared.EnvironmentDigest, prepared.ToolIdentities,
		prepared.ExecutableDigests, EffectiveTestWorkers(request), groupWorkers, stewardEngine,
		judgeKeyOf(request), request.BehaviorPolicyDigest, runtime.GOOS + "/" + runtime.GOARCH})
	return digestBytes(encoded)
}

func nativeIdentityKey(identity NativeTestIdentity) string {
	return identity.Classname + "\x00" + identity.Name
}

// coverageSourceMatches reports why source cannot supply covered tests to
// consumer. Only a complete native pass in the identical context qualifies.
func coverageSourceMatches(consumer, source GroupResult) error {
	if source.ID == consumer.ID || source.Status != "passed" || !source.NativeLaunched || !source.CollectionComplete ||
		source.NativeExitStatus == nil || *source.NativeExitStatus != 0 || source.ReuseAttempt != "" {
		return errors.New("is not a complete native pass of another group")
	}
	if consumer.NativeContext == "" || source.NativeContext != consumer.NativeContext || source.CWD != consumer.CWD ||
		source.EnvironmentDigest != consumer.EnvironmentDigest || source.IdentityVersion != consumer.IdentityVersion ||
		!maps.Equal(source.ToolIdentities, consumer.ToolIdentities) || !maps.Equal(source.ExecutableDigests, consumer.ExecutableDigests) {
		return errors.New("ran in a different native context")
	}
	return nil
}

// coverageSourceBacks reports whether source expected identity and observed
// its passing terminal in its own native launch.
func coverageSourceBacks(source GroupResult, identity NativeTestIdentity) bool {
	key := nativeIdentityKey(identity)
	if identity.Classname == "" || slices.ContainsFunc(source.CoveredTests, func(item NativeTestIdentity) bool { return nativeIdentityKey(item) == key }) {
		return false
	}
	expected := slices.ContainsFunc(source.Expected, func(item NativeTestIdentity) bool { return nativeIdentityKey(item) == key })
	passed := slices.ContainsFunc(source.Observed, func(item NativeTestIdentity) bool {
		return nativeIdentityKey(item) == key && item.Status == "passed"
	})
	return expected && passed
}

// validateCoveredGroup is the one judge of covered evidence inside a result.
func validateCoveredGroup(result TestResult, group GroupResult) error {
	if len(group.CoveredByGroups) == 0 && len(group.CoveredTests) == 0 {
		return nil
	}
	if len(group.CoveredByGroups) == 0 || len(group.CoveredTests) == 0 || group.NativeContext == "" || group.ReuseAttempt != "" {
		return fmt.Errorf("testing group %s has incomplete coverage provenance", group.ID)
	}
	byID := make(map[string]GroupResult, len(result.Groups))
	for _, candidate := range result.Groups {
		byID[candidate.ID] = candidate
	}
	for index, id := range group.CoveredByGroups {
		source, ok := byID[id]
		if !ok || index > 0 && group.CoveredByGroups[index-1] >= id {
			return fmt.Errorf("testing group %s names an absent or unordered coverage source %s", group.ID, id)
		}
		if err := coverageSourceMatches(group, source); err != nil {
			return fmt.Errorf("testing group %s coverage source %s %v", group.ID, id, err)
		}
	}
	for pending, seen := append([]string(nil), group.CoveredByGroups...), map[string]bool{}; len(pending) != 0; {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if id == group.ID {
			return fmt.Errorf("testing group %s is in a coverage cycle", group.ID)
		}
		if !seen[id] {
			seen[id] = true
			pending = append(pending, byID[id].CoveredByGroups...)
		}
	}
	expected := map[string]bool{}
	for _, identity := range group.Expected {
		expected[nativeIdentityKey(identity)] = true
	}
	observed := map[string]string{}
	for _, identity := range group.Observed {
		observed[nativeIdentityKey(identity)] = identity.Status
	}
	credited, used := map[string]bool{}, map[string]bool{}
	for _, identity := range group.CoveredTests {
		key := nativeIdentityKey(identity)
		if identity.Classname == "" || !expected[key] || credited[key] || identity.Status != "passed" || observed[key] != "passed" {
			return fmt.Errorf("testing group %s covers %s.%s without its own passing expectation", group.ID, identity.Classname, identity.Name)
		}
		credited[key] = true
		backed := false
		for _, id := range group.CoveredByGroups {
			if coverageSourceBacks(byID[id], identity) {
				used[id], backed = true, true
			}
		}
		if !backed {
			return fmt.Errorf("testing group %s covers %s.%s without a passing source terminal", group.ID, identity.Classname, identity.Name)
		}
	}
	for _, id := range group.CoveredByGroups {
		if !used[id] {
			return fmt.Errorf("testing group %s names unused coverage source %s", group.ID, id)
		}
	}
	return nil
}

// CoveredTestPass reports whether group passed without a native launch
// because every expected test passed natively in other groups of result.
// Readers that hold only the result use it; attempt readers also need
// TestGroupProducer's ownership check.
func CoveredTestPass(result TestResult, group GroupResult) bool {
	return group.Status == "passed" && group.CollectionComplete && !group.NativeLaunched && group.NativeExitStatus == nil &&
		group.ReuseAttempt == "" && len(group.CoveredByGroups) != 0 && len(group.CoveredTests) == len(group.Expected) &&
		validateCoveredGroup(result, group) == nil
}

// TestGroupProducer reports whether attempt produced group's evidence in
// result: as a native producer, or as a covered pass whose every source is a
// native producer of the same result. It never makes a covered group native.
func TestGroupProducer(attempt Attempt, result TestResult, group GroupResult) bool {
	if NativeTestProducer(attempt, group) {
		return true
	}
	if !CoveredTestPass(result, group) {
		return false
	}
	if len(attempt.TestInventory) != 0 {
		if !admittedTestFreshnessMatches(attempt, result) || attempt.TestOwned[group.ID] != group.ExecutionIdentity {
			return false
		}
	} else if attempt.PendingTestGroups[group.ID] != group.ExecutionIdentity {
		return false
	}
	for _, id := range group.CoveredByGroups {
		index := slices.IndexFunc(result.Groups, func(candidate GroupResult) bool { return candidate.ID == id })
		if index < 0 || !NativeTestProducer(attempt, result.Groups[index]) {
			return false
		}
	}
	return true
}

// testCoverageCredit is one consumer's credited tests and residual selection.
type testCoverageCredit struct {
	sources  []string
	tests    []NativeTestIdentity
	observed []NativeTestIdentity
	residual []NativeTestIdentity
}

// creditCoveredTests credits each expected identity to the first source, in
// the scheduler's broadest-first order, that passed it natively.
func creditCoveredTests(consumer GroupResult, sources []GroupResult) testCoverageCredit {
	var credit testCoverageCredit
	credited := map[string]bool{}
	used := map[string]bool{}
	for _, source := range sources {
		if coverageSourceMatches(consumer, source) != nil {
			continue
		}
		for _, identity := range consumer.Expected {
			key := nativeIdentityKey(identity)
			if credited[key] || !coverageSourceBacks(source, identity) {
				continue
			}
			credited[key], used[source.ID] = true, true
			credit.tests = append(credit.tests, NativeTestIdentity{Report: "go-test-json", Classname: identity.Classname, Name: identity.Name, Status: "passed"})
			for _, item := range source.Observed {
				if item.Classname == identity.Classname && (item.Name == identity.Name ||
					identity.Name != goPackageBuildIdentity && strings.HasPrefix(item.Name, identity.Name+"/")) {
					credit.observed = append(credit.observed, item)
				}
			}
		}
	}
	for _, identity := range consumer.Expected {
		if !credited[nativeIdentityKey(identity)] {
			credit.residual = append(credit.residual, identity)
		}
	}
	for id := range used {
		credit.sources = append(credit.sources, id)
	}
	sort.Strings(credit.sources)
	sortNative(credit.tests)
	return credit
}

// residualInventory keeps the declared packages that still have work: a
// package leaves the residual launch only when every expected identity it
// has was credited. An unresolved test name may match any package, so it
// keeps the complete inventory.
func (credit testCoverageCredit) residualInventory(inventory []string, modulePrefix string) []string {
	if len(credit.tests) == 0 {
		return inventory
	}
	residualPackages, coveredPackages := map[string]bool{}, map[string]bool{}
	for _, identity := range credit.residual {
		if identity.Classname == "" {
			return inventory
		}
		residualPackages[identity.Classname] = true
	}
	for _, identity := range credit.tests {
		coveredPackages[identity.Classname] = true
	}
	var kept []string
	for _, relative := range inventory {
		name := goInventoryPackages([]string{relative}, modulePrefix)[0]
		if residualPackages[name] || !coveredPackages[name] {
			kept = append(kept, relative)
		}
	}
	return kept
}

// planStageCoverage returns, for each stage group, the stage groups whose
// terminal it waits for before it may launch with covered tests. A source
// is broader (more expected identities, then lower id), shares the exact
// native context, overlaps the consumer, and never closes a wait cycle with
// the stage's prerequisites.
func planStageCoverage(request TestRunRequest, groups map[string]testpolicy.Group, ids []string) [][]int {
	sources := make([][]int, len(ids))
	contexts := make([]string, len(ids))
	eligible := 0
	for index, id := range ids {
		contexts[index] = goNativeContext(request, groups[id])
		if contexts[index] != "" {
			eligible++
		}
	}
	if eligible < 2 {
		return sources
	}
	position := make(map[string]int, len(ids))
	for index, id := range ids {
		position[id] = index
	}
	waits := make([][]int, len(ids))
	for index, id := range ids {
		for _, dependency := range groups[id].Requires {
			if at, ok := position[dependency]; ok {
				waits[index] = append(waits[index], at)
			}
		}
	}
	reaches := func(from, to int) bool {
		seen := make([]bool, len(ids))
		stack := []int{from}
		for len(stack) > 0 {
			at := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if at == to {
				return true
			}
			if seen[at] {
				continue
			}
			seen[at] = true
			stack = append(stack, waits[at]...)
		}
		return false
	}
	broader := func(a, b int) bool {
		sizeA, sizeB := len(request.PreparedGroups[ids[a]].Expected), len(request.PreparedGroups[ids[b]].Expected)
		if sizeA != sizeB {
			return sizeA > sizeB
		}
		return ids[a] < ids[b]
	}
	ranked := make([]int, len(ids))
	for index := range ranked {
		ranked[index] = index
	}
	sort.SliceStable(ranked, func(a, b int) bool { return broader(ranked[a], ranked[b]) })
	for _, consumer := range ranked {
		if contexts[consumer] == "" {
			continue
		}
		expected := map[string]bool{}
		for _, identity := range request.PreparedGroups[ids[consumer]].Expected {
			if identity.Classname != "" {
				expected[nativeIdentityKey(identity)] = true
			}
		}
		for _, source := range ranked {
			if source == consumer || contexts[source] != contexts[consumer] || !broader(source, consumer) ||
				!slices.ContainsFunc(request.PreparedGroups[ids[source]].Expected, func(identity NativeTestIdentity) bool {
					return expected[nativeIdentityKey(identity)]
				}) || reaches(source, consumer) {
				continue
			}
			sources[consumer] = append(sources[consumer], source)
			waits[consumer] = append(waits[consumer], source)
		}
	}
	return sources
}
