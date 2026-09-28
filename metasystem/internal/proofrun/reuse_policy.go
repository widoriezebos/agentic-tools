package proofrun

import "github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"

// ReusePolicy separates recording an attempt from forcing selected groups to
// execute. ForceAttempt belongs to admission; ForceGroups belongs here.
type ReusePolicy struct{ ForceGroups bool }

func ReusedTestResultWithPolicy(template TestResult, attempts []Attempt, identities map[string]string, contract testpolicy.Contract, policy ReusePolicy) TestResult {
	return reusedTestResult(template, attempts, identities, contract, "", policy)
}

func ReusedTestResultExcludingWithPolicy(template TestResult, attempts []Attempt, identities map[string]string, contract testpolicy.Contract, excludedAttempt string, policy ReusePolicy) TestResult {
	return reusedTestResult(template, attempts, identities, contract, excludedAttempt, policy)
}

// Go test-cache acceptance (A13). A proof group's go test argv carries
// -count=1 exactly when the engine has decided the launch must execute;
// everywhere else go test may serve a package from its own cache, keyed on
// the test binary, the cacheable flags, the variables and the module files
// the tests read. The reason is recorded with every package it governs.
const (
	GoTestCacheAllowed           = "go-test-cache-allowed"
	GoTestCountOneFresh          = "fresh-request"
	GoTestCountOneRerun          = "diagnostic-rerun"
	GoTestCountOneCoverage       = "coverage"
	GoTestCountOneExternal       = "external-inputs-changed"
	GoTestCountOneUnrecorded     = "external-inputs-unrecorded"
	GoTestCountOneUntrackedChild = "untracked-child-inputs"
	GoTestCountOneResultSchema   = "result-schema-without-execution-record"
	diagnosticRerunShard         = -1
)

// goCacheFacts are the inputs reusePolicy reads beyond the request and the
// group, gathered once per group launch.
type goCacheFacts struct {
	// ExternalInputsDigest is this launch's digest of the declared external
	// inputs; RecordedExternalDigest the one recorded with the newest
	// retained pass of the group whose every package executed.
	ExternalInputsDigest, RecordedExternalDigest string
	// ChildLaunches names the group's test files that start child processes,
	// whose file reads go test never records.
	ChildLaunches []string
}

// reusePolicy decides -count=1 for one shard of a go group (shard is
// diagnosticRerunShard for a failed-test rerun). It is a pure function of
// its inputs and only ever adds a reason to execute to Go's own rules.
func reusePolicy(request TestRunRequest, group testpolicy.Group, shard int, facts goCacheFacts) (bool, string) {
	switch {
	case shard == diagnosticRerunShard:
		return true, GoTestCountOneRerun
	case request.ResultSchema() < TestResultSchemaVersion:
		// A frontend that cannot read the execution record would take a
		// replay for an execution; its launches always execute.
		return true, GoTestCountOneResultSchema
	case request.FreshGroups[group.ID]:
		// Episode freshness, --no-reuse and a cadence run all land here
		// (testingFreshGroups).
		return true, GoTestCountOneFresh
	case group.Coverage:
		// -test.gocoverdir already disables Go's cache; the policy says so.
		return true, GoTestCountOneCoverage
	case len(group.ExternalInputs) != 0 && facts.RecordedExternalDigest == "":
		return true, GoTestCountOneUnrecorded
	case len(group.ExternalInputs) != 0 && facts.RecordedExternalDigest != facts.ExternalInputsDigest:
		return true, GoTestCountOneExternal
	case len(group.ExternalInputs) == 0 && len(facts.ChildLaunches) != 0:
		// A child's reads are invisible to go test; without declared
		// external inputs covering them a cached pass could be stale.
		return true, GoTestCountOneUntrackedChild
	}
	return false, GoTestCacheAllowed
}
