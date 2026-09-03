Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

Author the design for goal token-spend-fence, STEP 1 (alert mode), as
its record fixes (read metasystem/plans/goals/token-spend-fence.md
first; Wido's words are rulings R-58-m1, R-60-m1 and R-61-m1 in
metasystem/memory/rulings.md). Tier 3 under R-54-m1: this design, one
Sol review, one fold, one closing review, then a Sol build and one
Fable code review. Under R-60-m1 a review finding is material only if
it changes what gets built and names the artifact; disputed points at
the budget become named test obligations, never another round.

The outcome: spend is measured in tokens and money per goal, per
machine and per day from the runtimes' own usage records; ceilings are
configurable in metasystem.conf with sane defaults; one health line
shows today's spend against the ceilings every tick; crossing a ceiling
raises an alert; NOTHING IS REFUSED until Wido agrees the calibration
(step 2, his word, not this design).

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one NEW file, token-spend-fence-design.md, in
the metasystem plans directory, under 300 lines.

# What the design must settle

1. THE TRUTH SOURCE. Every dispatched round's adapter writes a typed
   usage record (fields inputTokens, cachedInputTokens, outputTokens,
   reasoningTokens, cost {amount, currency} or null, availability,
   providerUnits; writers in metasystem/scripts/agents/adapters/claude.sh
   and codex.sh through metasystem/internal/usage) into
   artifacts/agents/<root>/rounds/<n>/usage.json, and the job record
   (artifacts/agents/jobs/<job>.json) carries goalId, machineId,
   canonicalModelKey, runtime, startedAt, endedAt and a usage object.
   The mission fence already aggregates typed usage across a mission's
   jobs (metasystem/internal/mission/fence.go, `usageTokenFields` and
   `AggregateUsage`, with its honesty rule: a terminal job without
   measured usage is recovered from its event stream only under proven
   group death, else counted unavailable, never zero). Specify the
   reader: which files, which fields, how a job maps to (goal, machine,
   day) — day is the UTC date of startedAt — and how unmeasured spend is
   reported (a count of jobs whose usage is unavailable, shown beside
   the totals, never folded into them).
2. THE SEAT'S OWN SPEND. Coordinating sessions are not jobs and write no
   usage record, yet on 2026-09-02 one seat on the maximal tier consumed
   about half of a week's budget. Decide what step 1 measures for seats:
   the Claude Code session transcripts under ~/.claude/projects/<slug>/
   carry per-message usage on the machine that runs them; the seat's
   enrollment (artifacts/agents/mains/) names the session. State
   exactly what is measurable today, attribute it to (machine, day,
   "seat" as the goal, or the goal claimed at the time if that is
   derivable from the ledger), and where it is not measurable say so as
   an explicit unmeasured line. Do not invent a meter that does not
   exist; a stated gap is the honest output.
3. MONEY. Claude usage carries a native cost; Codex usage carries none.
   Specify a price table in metasystem.conf, keys of the shape
   `spend.price.<runtime>.<canonical-model>.<input|cached|output|reasoning>`
   in currency per million tokens, currency `spend.currency` (default
   USD), with the rule: native cost wins when present; derived cost is
   tokens times price; a model with no price row yields money
   "unpriced" beside its token total, never zero. Tokens are the truth;
   money is a view.
4. CEILINGS. Keys in metasystem.conf, validated by the config validator
   (metasystem/internal/config/validate.go registers known keys; follow
   the budget keys' shape in metasystem/internal/config/budget.go):
   `spend.ceiling.day.tokens`, `spend.ceiling.day.money` (fleet-wide per
   machine-day is the unit a machine can measure; say whether fleet-wide
   means summed across machines through the shared ledger or per
   machine, and why), `spend.ceiling.goal.tokens`, `spend.ceiling.goal.money`,
   and `spend.mode` = alert (the only value this step accepts; `enforce`
   is refused by validation until step 2 lands). Sane defaults: derive
   them from the recorded 2026-09-02 counts (126 dispatches fleet-wide;
   the specimen totals on this checkout) and state the arithmetic.
5. THE HEALTH LINE. One new steward health role (the role vocabulary is
   metasystem/internal/steward/health.go, `HealthRole` constants; the
   verdict line is `HealthVerdict.Line`) printing, every tick, one line:
   today's tokens and money against the day ceilings, and the claimed
   goal's tokens and money against the goal ceilings, plus the unmeasured
   count. Specify the exact line format.
6. THE ALERT. Crossing a ceiling raises one alert episode per (ceiling,
   day or goal) crossing through the existing episode machinery
   (metasystem/internal/steward/alert_episode.go), carrying Option A
   facts (R-45-m0b): what crossed, the spend, the ceiling, where to look,
   who can raise it; delivered through the fleet channel once
   fleet-slack-channel lands and through the health path meanwhile. No
   repeat alert for the same crossing; a new one at each further
   multiple of the ceiling. Nothing refuses.
7. TESTS. Fixture-driven: a bed of job records and usage files replaying
   2026-09-02's shape (a mix of native-cost claude rounds, priced codex
   rounds, an unpriced model, an unavailable usage) against ceilings;
   assertions on the per-goal, per-machine, per-day totals, the
   unmeasured count, the health line bytes, the alert episode's facts,
   and that no admission path consults the fence (grep-level proof that
   step 1 touches no refusal). Name the Go packages and test names.
8. NON-GOALS. Step 2's refusal; the reserved-minute pool (its settlement
   is goal dispatch-cap-necessity); prices for models not in the roster;
   any change to adapters' usage writers.

Ground every claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade per the house
rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 30 minutes. Do not edit anything but the design
file. Reuse the mission fence's aggregation and the usage package
rather than a second reader.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.
