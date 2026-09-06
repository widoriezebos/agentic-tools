Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Goal stop-hook-budget-is-ours, slice 2 of 2 (tier 3, approved by Wido
at his terminal on 2026-09-06). The record,
metasystem/plans/goals/stop-hook-budget-is-ours.md, is the contract.
Slice 1 landed the measurement (commit fe61beb7): every Stop hook completion carries the
whole seconds elapsed since the deadline parent started, on the
supervision-hook component record (`lastStopElapsedSec` on the record,
`stopElapsedSec` on each history entry, absent when unmeasured) and in
the hooks log. This slice makes the steward read it: a health role flags
a Stop that is getting slow, or one that expired, before it starts
costing turns, delivered like every other health line through the alert
channel. The ceiling stops being a guess and the steward, not a human
reading a log, notices drift.

# The change

1. An expiry becomes a recorded completion. Today the deadline parent in
   metasystem/scripts/agents/supervision-hook.sh kills the worker on
   expiry and writes only a hooks-log line; the component record stays
   ATTEMPTING until the next turn marks it INTERRUPTED_BY_NEXT_TURN,
   which is indistinguishable from a hook killed by the runtime. Add a
   verb `steward hook-expire --repo <checkout> --elapsed-sec <n>` in
   metasystem/cmd/metasystem/steward_verbs.go backed by a function in
   metasystem/internal/steward/component_evidence.go that, under the
   component lock, loads the supervision-hook record and, when its
   outcome is ATTEMPTING, completes it with result ERROR, outcome
   DEADLINE_EXPIRED and the given elapsed seconds (record and history
   entry), appending history exactly as the interrupted-turn closure
   does; when the record is not ATTEMPTING it returns a typed error and
   changes nothing (the worker completed first; the parent's kill lost
   the race, which is fine). The deadline parent calls it right after
   the worker is dead, before it writes its own `deadline-expired-block`
   line, with the same elapsed number it logs, and ignores a non-zero
   exit (the log line is still written). Use `deadline_repo` as the
   checkout root the way `deadline_log_stop_outcome` resolves its
   supervision root.

2. The health role. In metasystem/internal/steward/health.go add
   `RoleStopHookDuration` with the name `stop-hook-duration`, placed
   right after `RoleHookFreshness` in `healthRoleOrder` and in
   `evaluateHealthRoles`. Its rule, reading the supervision-hook
   component record the way `checkHookFreshnessAt` does (reuse its
   loader; a missing record is alive with the reason "no Stop has been
   measured yet", so a fresh checkout is not unhealthy):
   - Take the newest completed attempt: the record itself when its
     outcome is not ATTEMPTING, else the last history entry; when
     neither carries a measurement, alive with "the last Stop carried no
     measurement".
   - Outcome DEADLINE_EXPIRED: dead, reason "the last Stop expired its
     deadline after <n>s of the <ceiling>s budget on <machine>", remedy
     naming goal stop-hook-health-cost as the fix for a hook that is too
     expensive and `metasystem health --repo <root>` to re-read.
   - Elapsed at or above the slow threshold: dead, reason "the last Stop
     took <n>s of the <ceiling>s budget on <machine>; the threshold is
     <t>s", same remedy.
   - Otherwise alive with "the last Stop took <n>s of the <ceiling>s
     budget".
   The machine is `goal.ResolveMachine(repoRoot)` (fall back to "this
   machine" as `checkSpendFence` does). The ceiling is a constant 60 in
   the steward package with a comment naming the registration templates
   under metasystem/scripts/enforcement as where the number is shipped.
   The threshold is the config key `steward.stop-slow-sec`, default 15
   (a quarter of the ceiling), read through `boundedConfig` the way
   `steward.tick-patience-sec` is read at health.go line 589, and
   registered in metasystem/internal/config/validate.go in the
   positive-integer knob list beside `steward.tick-patience-sec`; a value
   at or above 60 is refused there ("must be below the sixty-second
   Stop budget"). Document the key where `steward.tick-patience-sec` is
   documented (grep metasystem/metasystem.conf and the docs for it; if
   only the conf comments carry it, add the new key's comment beside it).

3. The unhealthy line reaches the alert channel through the existing
   path: a dead role makes the health verdict unhealthy and
   `UpdateAlertEpisodes` in metasystem/internal/steward/alert_episode.go
   delivers it. Verify by reading that nothing in the alert path filters
   roles by name; if something does, add the role there and say so.

4. Tests and fixtures:
   - Go unit tests in the steward package for the role: a synthetic
     record with `lastStopElapsedSec` 3 is alive; 15 is dead with the
     threshold text; a record whose last completion is DEADLINE_EXPIRED
     is dead with the expiry text; a record without the field is alive
     with the no-measurement text; a missing record is alive; a
     configured threshold of 5 flags a 6-second Stop. Unit tests for the
     expire function: ATTEMPTING becomes DEADLINE_EXPIRED with the
     elapsed on record and history; a completed record is refused
     unchanged. The config validator test gains the new knob's
     positive-integer and below-sixty cases.
   - metasystem/scripts/agents/health-fixtures.sh: the healthy line
     asserts `stop-hook-duration=alive` alongside the other roles (the
     bed's Stop hook has run by then, or the role is alive on the
     no-measurement rule; say which in the return). One direct-verdict
     leg writes a synthetic supervision-hook record with a 20-second
     elapsed and asserts `stop-hook-duration=dead` naming 20s and 60s,
     then restores the record.
   - metasystem/scripts/agents/supervision-hook-fixtures.sh: the
     deadline scenario additionally asserts that the supervision-hook
     component record under the scenario's stop root carries outcome
     DEADLINE_EXPIRED with `stopElapsedSec` after the first expiry.

Nothing else changes: the deadline numbers, the refusal text and the
hook's measurement from slice 1 stay as landed.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/steward/ ./internal/config/ -count=1` green;
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh scripts/agents/health-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` and
`bash scripts/agents/health-fixtures.sh` green where the sandbox allows
(the deadline scenario needs process inspection the sandbox denies; say
so and the orchestrator reruns both suites at the landing).

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Never touch plans. Gap rule: stop and report a gap with your
proposed contract written out.

# Expected Return

The implementer return per the role schema: `riskiestPart` first,
`diffBoundary` with every touched path relative to the repository root
(each starts with `metasystem/`), `whatWasDone`, `gaps`, and `evidence`
entries in the settled `{command, observed, level}` shape: the go
build, vet and gofmt line, the two package tests, and each fixture suite
with its exit status and wall time.

# Acceptance Criteria

- `metasystem health --repo <root>` prints `stop-hook-duration=<status>`
  with the reasons above; a record with a 15-second Stop makes the
  verdict unhealthy; a 3-second one does not.
- A deadline expiry leaves the component record at DEADLINE_EXPIRED
  with its elapsed seconds; the next turn's hook starts a new generation
  without marking it INTERRUPTED_BY_NEXT_TURN.
- `steward.stop-slow-sec` is validated (positive integer below 60) and
  read with the default 15.
- Both fixture suites pass with the new assertions.

# Gap Rule

stop and report a gap; never fill it silently.
