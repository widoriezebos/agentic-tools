package main

// coverageDeltaRelaunchedVariable marks the admitted proof's suite command:
// the coverage delta relaunched under proof-run launch.
const coverageDeltaRelaunchedVariable = "METASYSTEM_COVERAGE_DELTA_RELAUNCHED"

// coverageDeltaGoTestArgv is one package's trimmed coverage run.
func coverageDeltaGoTestArgv(pkg string) []string {
	return []string{"go", "test", "-trimpath", "-cover", "-timeout", "30m", pkg}
}
