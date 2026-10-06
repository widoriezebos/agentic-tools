# Brief: lane-lands-finished-goals, unit internal-green (main's internal reds and this goal's)

Working Mode: Implement
`go test ./internal/...` is red on main (c2b5ae183) and on this goal branch. Make it green. Fix test hygiene and declarations only; change no production behaviour unless an item says so. Never open any metasystem.conf.local. Do not touch memory/ or records/.

Red on main (fix here; they integrate with this goal):
1. internal/mission TestDesignRoundRuleIsStatedBySkillAndDocs: skills/design-critique/SKILL.md lacks the exact phrase "round cap is the goal's review-round member under" (read the test for the full required sentence and where docs must state it; add it where it is true).
2. internal/testenv TestEveryPackageUsesSharedMain/fixture_engine_children_use_shared_reaper: internal/steward/runner_git_process_fixture_test.go starts an engine-pin or steward run child without testenv.ReapFixtureProcessGroups; use it as the audit requires.
3. internal/testenv TestTestExecutablesAreWrittenUnderTheForkLock: os.WriteFile of executables in cmd/metasystem/health_remedy_test.go:29, steward_probe_test.go:48 and :109, and this goal's cmd/metasystem/adopt_proof_settings_test.go:50 (and any other the audit lists): use testexec.WriteFile.
4. internal/testenv TestNoTestWaitsOnWallTime: internal/steward/runner_idle_test.go:46,199 time.Since; this goal's landing_merge_gate_test.go time.Now().Add and landing_stop_record_test.go time.After (and any other the audit lists): replace with the injected clocks or deadlines the audit accepts (Wido's rule: artificial clocks, never load-fragile waits).
5. internal/proofrun TestTestEnvironmentStandardInventoryMatchesObserved: nests the testenv audits; green once 2-4 are.

This goal's:
6. internal/pathclass TestRepositoryManifestClassifiesEveryTrackedPath: proof/full.sh and proof/main.go (and proof/.full-reporter ignore) need classes in the path manifest; classify them as the repository's committed install sources.
7. TestProofConfigurationDigestDecidesPerKeyFromTheCompiledTable: "read test run setting proof.cheap: must be a non-empty, one-line command"; its fixture needs the committed proof.cheap declaration (keep the test's intent: digest per key from the compiled table).

Check: go build ./... && go vet ./... && go test -count=1 -timeout 60m ./internal/... && go run ./cmd/devgate static
Then run the specific cmd tests you touched. If a test fails that none of these items covers, name it with its output and whether it fails at this unit's base, and leave it. t.Parallel() for new tests; -timeout on every go test line. Leave uncommitted. Return the exits, git diff --stat, and per item what changed.
