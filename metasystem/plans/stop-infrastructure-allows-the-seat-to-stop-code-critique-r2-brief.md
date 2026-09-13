# stop-infrastructure-allows-the-seat-to-stop: code critique, round 2

Working Mode: design

Goal stop-infrastructure-allows-the-seat-to-stop. The chain's round 1
(job member1-codecritic-232830) read round 4 of the build chain
implementer-b18d717d6d38a443b0cb339f and returned SIC-01 (the launcher
renderer), SIC-02 (a consume error under a decided refusal), SIC-03 (two
allow outcomes without a stop-condition line) and two non-material notes.
The seat folded them and landed the member as commit 529d8a649 on
origin/main. This round verifies the fold against the LANDED tree, not
the build worktree.

## What to verify

1. SIC-01: `metasystem/internal/hooks/setup.go` renders the installed
   Claude Stop line from the shipped template's fallback tail
   (`shippedFallback`, `renderGitRequiredCommand`), recognizes the retired
   block form (`legacyBlockFallback`) as this installation's so setup
   replaces it, and the tracked `.claude/settings.json` at the repository
   root carries the degraded fallback. Tests:
   `TestClaudeMergeReplacesTheBlockFallbackLauncher` in
   `metasystem/internal/hooks/setup_test.go`,
   `TestGeneratedShippedClaudeStopLauncherAllowsDegradedOutsideGit` and
   `TestGeneratedFreshNonGitLifecycleNoopsOnlyBeforeHandlerInvocation` in
   `metasystem/internal/hostsetup/setup_test.go`. `runtime setup --check`
   passes on the landed checkout.
2. SIC-02: `metasystem/internal/goal/turnverdict.go`: a consume error
   under a decided refusal keeps the block, leaves the marker unspent and
   invents no authorization; the authorization then covers one later
   quiet stop. Test `TestSessionStopConsumeErrorKeepsDecidedBlock` in
   `metasystem/internal/goal/turnverdict_idle_test.go`.
3. SIC-03: `metasystem/scripts/agents/supervision-hook.sh`: the advisor
   exit appends the stop-condition lines of the conditions recorded before
   it (`append_stop_condition` is defined before that exit), and the
   deadline parent's unreadable-worker-output branch logs
   `stop-condition infrastructure stop-hook-output-was-unreadable
   stop-worker` through `deadline_log_stop_condition`, shared with the
   expiry branch. The killed-worker leg of
   `metasystem/scripts/agents/supervision-hook-fixtures.sh` finds the line.
4. The wording note: an infrastructure-class verdict now renders under a
   fixed first line (`turn-verdict degraded: stopping is allowed on
   degraded infrastructure; the steward owns repair`, cause, component)
   in the hook; the stop-hook-monitor scenario of
   `metasystem/scripts/agents/supervision-fixtures.sh` asserts it.

## Return

Findings sorted by materiality against the landed tree at 529d8a649. A
finding is material only if it names a path where the landed code still
fails one of the four items above or introduces a new defect. Mark
SIC-01, SIC-02 and SIC-03 resolved or still open, each with the evidence
read or run. The design page is
`metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md`.
