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
