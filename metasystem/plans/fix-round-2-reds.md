# Fix round 2: the integration gate's six reds

Tree: the fix round 2 integration worktree (current directory), after the merge of unit u5b. The reporter's host leg on 32a1162f8 (main + six fix branches) ended `LANDING-FAILED` in six packages. Fix every red here, in the smallest change that keeps every test's intent. Never edit `testing.json`. Never open any `metasystem.conf.local`. Do not commit (the seat commits).

## 1. internal/audit, internal/adopt (same cause)

`TestShippedInstallationPassesMetasystemAudit` and both `TestAdoptGitIntegration*` tests refuse the shipped installation: `skills/landing-agent/SKILL.md:76 speaks the bare "this gate" on a traveling surface` and `:96 speaks the bare "its job"`. Qualify both phrases (the audit's rule: a traveling surface names whose gate or job it is, e.g. "the batch proof's gate", "the fix job's id"). Grep the file for other bare "this gate"/"its job"/"this job" phrases and qualify them too. Check: `go test -count=1 -timeout 30m ./internal/audit` and `go test -count=1 -timeout 30m ./internal/adopt`.

## 2. internal/testenv, internal/proofrun (same cause)

`TestTestExecutablesAreWrittenUnderTheForkLock`: `cmd/metasystem/intent_lane_fix_test.go` and `cmd/metasystem/landing_fix_test.go` write the pre-commit hook executable with `os.WriteFile(..., 0o755)`. Use `testexec.WriteFile` (import `github.com/widoriezebos/agentic-tools/metasystem/internal/testexec`; examples: `cmd/metasystem/ambient_controls_test.go:143`). Every executable write in both files. `TestTestEnvironmentStandardInventoryMatchesObserved` (internal/proofrun) runs the testenv package and fails with it; re-run it after the fix: `go test -count=1 -timeout 30m ./internal/proofrun -run TestTestEnvironmentStandardInventoryMatchesObserved` and `go test -count=1 -timeout 30m ./internal/testenv`.

## 3. internal/hostsetup

`TestShippedRepositoryHostRegistrationsAreConfigurationReady`: "host setup conflict: changed profile .claude/agents/code-critique.md". The eight files under `.claude/agents/` at the repository top level carry a hand-added `model:` line; the shipped agent profiles are host registrations and the roster is configuration, so restore them to main's content: `git checkout 2f4fa5e3b -- ../.claude/agents` (from the metasystem directory; the path is the repository's `.claude/agents`). Check: `go test -count=1 -timeout 30m ./internal/hostsetup`.

## 4. cmd/metasystem: three tests

a. `TestIntentBuildForeignClaimRefused` (`intent_claim_positive_test.go:113`) expects the summary to contain `held by another session, not mac-cli (m1)`; the refusal now names the holder by design (unit seat-recovers-a-dead-session/u1: "the claim refusal says who holds the goal"): `held by another session, mac-other (o1), not mac-cli (m1)`. Update the expectation to the designed text; keep every other assertion.

b. `TestProofAttemptBinaryLaunchesUseSharedIsolation` (`proof_fixture_test.go:155`): `intent_lane_fix_test.go` launches the test executable as a metasystem fixture with `exec.Command(executable, "fixture-lane-read-owner", ...)` and `exec.Command(executable, "fixture-lane-claim-owner", ...)` (around lines 498-520). The rule (read `proof_fixture_test.go`, `looksLikeFixtureEngine`, `proofBinaryFixture.command`) is that such launches go through `proofBinaryFixture.command` so they share the proof isolation; six other test files show the pattern (`grep -l 'proofBinaryFixture\|\.command(' cmd/metasystem/*_test.go`). Route both launches through it, keeping their environment entries.

c. `TestLaneQuestionAnswerIsPolledByTheLanesSteward` (`landing_agent_question_test.go:179`, "the answer is recorded in the lane's question store" but the question is still open). Deterministic. It passes at main 2f4fa5e3b and fails at 283f245a3, whose only changes are: `internal/testenv/testenv.go` adds `METASYSTEM_OWNER_LINEAGE` to the scrubbed inherited controls; `channel_verbs_test.go` turns `commandFakeBed` into a wrapper over `commandFakeBedWithClock` (fake channel server with a `Now` hook); `question_blocked_e2e_test.go` runs TestBlockedAgentAsksAndTheStopLetsItWait on a fixed clock (`METASYSTEM_GOAL_NOW` + the provider clock) and adds a clock-sharing test; `wait_verb_test.go` gives the pending-wait fixture its own lease lineage and a declared-child regression test. Find which of these the lane question test depended on (run it at both commits if useful: `git stash` is not available, use `git worktree` or read the diff) and fix the dependency at its root without weakening TestBlockedAgentAsksAndTheStopLetsItWait, TestPendingWaitInstalledVerdicts or the scrub. Say what the cause was.

Check: `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestIntentBuildForeignClaimRefused|TestLaneQuestionAnswerIsPolledByTheLanesSteward|TestProofAttemptBinaryLaunchesUseSharedIsolation|TestLaneQuestion|TestBlockedQuestion|TestBlockedAgent|TestPendingWait|TestLaneFix|TestLandingFix|TestIntentLaneFix|TestIntentBuild|TestWaitChannel'`.

## Report

Per red: the cause and the change (1-3 lines). Every check's exit code. `git diff --stat`. For 4c, the root cause in one sentence.
