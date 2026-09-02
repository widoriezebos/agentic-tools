Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Round-2 implementation critique of the breach-clock fix build (job
breach-build-3b, Sol). Its worktree holds the round-1 build as a cherry-pick
(commit 0d1f4592, identical in content to 0d8e47ef, which round 1 reviewed)
and on top of it the fix commit e355c030 touching exactly
metasystem/internal/goal/verbs.go, metasystem/internal/goal/verbs_test.go and
metasystem/scripts/agents/goal-cli-fixtures.sh (three files). Round 1 is
metasystem/records/misc/breach-code-critique-r1.md (landed; read it first).
Scope is the fix commit's hunks against the three round-1 decisions; the
rest of the build stands certified by round 1 and the orchestrator's host
gate recorded there. Do not re-review it.

# Attack surface

- F-1: in the Done verb the `displaced` pair is now computed before the fence
  block. Confirm by reading that nothing else in the block moved (human
  guard, `VerifyStopBatchComplete`, the four clears, in that order), that the
  later computation was removed and not duplicated, and that the new test
  TestHumanDoneOverFencedForeignClaimAcksDisplacement asserts both the
  Displaced marker with pair A's prefix on the done history line AND the
  acknowledgment change set naming pair A, against a goal that is genuinely
  claimed by a foreign pair and fenced, not a tautology.
- F-2: TestSetBudgetInheritsEpisodeObligationRevision must reach a non-zero
  third episode key through the real writers (claim, set-obligation,
  discharge, SetBudget), with no live obligation at the raise, and assert the
  key survives unchanged. If it seeds the key by hand it is a finding.
- F-4: the day-refusal row now fails when `git cat-file -p` fails, and greps
  only the successful output.
- Any hunk in the fix commit outside these three purposes is a finding. Any
  weakening of an existing refusal or assertion is a finding. R-31: no
  benchmarks.

# Constraints

Wall-clock budget: 20 minutes. Your sandbox is read-only; verify by reading.
Return per the code-critic schema. Zero material findings is an acceptable,
closing answer if the reading supports it.

# Gap Rule

stop and report a gap; never fill it silently.
