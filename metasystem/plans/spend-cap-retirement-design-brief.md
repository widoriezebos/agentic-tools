Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Author plans/spend-cap-retirement-design.md: the design that retires (or, if
the evidence refuses, right-sizes) the per-worker dollar cap the engine passes
to every Claude delegate. Wido's conditional mandate, verbatim on the goal
(plans/goals/native-spend-cap-retirement.md, read it first): the kill proceeds
only if two assumptions verify — (1) the machinery already bounds runaway
spend without this cap, and (2) this cap actually harms us. Your design must
verify or refute both against the code and the record, then specify the
change.

# Workspace

The delegate worktree the dispatcher created for this job. Write exactly one
new file: metasystem/plans/spend-cap-retirement-design.md. Read anything.

# The object under judgment

internal/adapter/claude.go, ClaudeBudget: hardcoded default budget "5.00",
overridable only by the METASYSTEM_CLAUDE_MAX_BUDGET_USD environment
variable; passed to every Claude worker as --max-budget-usd. Landed
2026-08-14 (commit 24345044) inside an argv consolidation, never designed,
never critiqued. The sibling turn guard (default 150) sits beside it and is
NOT in scope — it already had its recalibration and guards a different
failure (infinite loops), unless your protection inventory proves otherwise.

# Assumption 1 to verify: the protection inventory

Enumerate every spend-bounding or runaway-bounding mechanism that survives
the cap's death, from the code: the goal budget tuple's reservedJobMinutes
(internal/dispatch admission), the per-round cap-minutes reservation and its
watchdog kill (internal/dispatch/cap.go, the proof-run watchdog), the native
turn limit, attempt limits and breach fences (internal/dispatch/budget.go),
the stop machinery, and anything else you find. Then attack: enumerate
runaway scenarios (a worker looping on expensive re-reads within few turns;
a stuck stream; a worker spawning subagents; model-side pricing surprises)
and name which surviving mechanism bounds each, with the bound's rough
dollar equivalent at realistic burn rates. If a scenario has NO surviving
owner and a plausible dollar exposure beyond what a wall-clock kill bounds,
say so plainly — that refutes assumption 1 and the design must then propose
the minimal owner (e.g. a never-hit backstop at a stated level) instead of
the clean kill.

# Assumption 2 to verify: the harm record

The specimens: goal budget-death-on-return (three m1/m2 deaths at $5.07,
$8.19, $10.01 — products recovered by hand); goal alert-escalation-channel's
2026-09-01 next-step history (six deaths in one day, each finished work
killed before its return, each costing a paid recovery round and 40
reserved pool-minutes; the sixth was a recovery round itself). State what
the cap has ever actually PREVENTED, if the record shows anything.

# The specification

If both assumptions hold (enough — Wido's phrase), specify the kill exactly:
what ClaudeBudget returns when the environment variable is unset (no flag
passed at all, versus an explicit unlimited, versus a high backstop — choose
and justify against the Claude CLI's actual --max-budget-usd contract);
whether the environment override survives as an explicit operator tool;
what happens to the two protocol errors (invalid_native_budget stays for a
malformed override); the exact test changes (claudecommand_test.go pins the
current argv byte order); and whether codex.sh carries an equivalent cap
needing the same judgment (check scripts/agents/adapters/codex.sh and its
command builder — if yes, scope it in or explicitly defer it with a reason).

Self-grade per the house rule: confidence, weakest claim, and the reject
condition stated plainly.

# Constraints

Wall-clock budget: 25 minutes. This is a small, focused document — the
protection inventory and the specification, not an essay. The decision
criterion is Wido's stated assumptions, not cost-benefit invention.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/spend-cap-retirement-design.md.

# Gap Rule

stop and report a gap; never fill it silently.
