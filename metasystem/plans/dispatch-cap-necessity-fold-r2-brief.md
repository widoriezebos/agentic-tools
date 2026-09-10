Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-09

# Fold round two: the combined refusal rendering, decided

Follow-up round on chain dispatch-cap-build1-20260909. Round one
stopped, correctly, at a gap: the design's test T10 wants the budget
refusal line to end after the reserved evidence, while the landed
change of 2026-09-04 (930147147, goal setup-refusals-consume-attempts)
appends the setup-refusal release rule to attempt-limit and
reserved-minute refusals and an existing test
(TestBudgetAdmissionRefusalNamesSetupRefusalReleaseRule in
internal/dispatch/budget_test.go) requires that clause. Both contracts
hold; here is the one rendering that satisfies both.

## The combined rendering (this page amends design T10)

A refusal line is the breach list, then the reserved evidence, then the
rule clause exactly as internal/dispatch/admission.go renders it today,
each segment joined with "; ":

    BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=1 limit=1; reserved observed=1 open-caps=0 limit=10000; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes

    BUDGET_REFUSED: goal bounded revision=3 admission closed: reservedJobMinutesLimit used=170+120 proposed limit=240; reserved observed=50 open-caps=120 limit=240; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes

The reserved evidence comes before the rule clause because the clause
explains what those two numbers exclude. The rule clause attaches
exactly where it attaches today (attempt-limit and reserved-minute
refusals) and nowhere new; a BUDGET_UNKNOWN line carries neither the
reserved segment nor the clause, as today. The governed refusal carries
the reserved segment through the shared helper as the design says, with
the rule clause only if it carries it today.

T10's exact assertions for (a) and (b) are the two full lines above;
(c) and the governed subtest are unchanged. The existing rule test
keeps passing untouched. Nothing else in the design's box or in the
landed brief changes.

## Mandate

Build the whole box now, as the seat's build brief of this morning
(the first brief of this chain, in the plans directory under this
goal's name) and the landed brief
metasystem/plans/dispatch-cap-settlement-build-brief.md within
metasystem/plans/dispatch-cap-settlement-scope-cut.md specify, with T10
as amended above. Report the round as your own.

## Constraints

As the landed brief; wall-clock budget 45 minutes. Stop at any other
gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
