Working Mode: design
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Design review of the tiering machinery, revision 2:
metasystem/plans/severity-tiered-rigor-design.md, the section
"Revision 2 (2026-09-03): the tier is the budget, the material stop
closes the loop". It re-aims the landed tasks 1 to 3 of that goal under
two human rulings, R-54-m1 (classify at intake, then budget by tier)
and R-60-m1 (review depth is part of the risk-based budget, no separate
cap machinery; a finding is material only if it changes what gets built
and names the artifact), both in metasystem/memory/rulings.md. This is
the goal's one design review under its own tier-3 budget.

# Mandate

Judge the seven mechanism points and the four-part build list against
the code they name. The questions that matter:

1. The tier field and the budget boxes: does the norm gate in
   metasystem/internal/goal/norm.go and the tuple law in
   metasystem/internal/goalbudget/budget.go admit a per-tier box without
   a second approval path; is deleting the single minute norm key safe
   for the conf loader in metasystem/internal/config/budget.go; is the
   sweep of open goals a lawful ledger mutation.
2. The ladder refusals in delegate and the tier-derived boundary that
   replaces the three round constants in
   metasystem/internal/dispatch/critique.go: does the register engine in
   metasystem/internal/dispatch/finding_register.go still have a
   well-defined boundary for every register state (all bounded, mixed,
   none open) once the constants go, and is "critic rounds per chain"
   countable from the records as they exist.
3. The artifact member and the demotion rule in
   metasystem/internal/critique/model.go: can a critic keep a loop alive
   through any path the rule does not cover (a NEW path that is never
   built, an artifact outside the diff, a rename), and is demotion
   without a synthetic finding safe for the exhaustion discipline in
   metasystem/internal/validate/conformance.go.
4. The close verb with its two exits: do obligations on the goal record
   (the Obligation field in metasystem/internal/goal/file.go) and the
   accepted-risk entry in metasystem/records/counselor/accepted-risk-register.jsonl
   give a landing what it needs, and can the zero-material rule read
   the register instead of the last return without reopening the
   critic-shopping hole the old task five guards.
5. The tier-1 landing class against the floor in
   metasystem/internal/landing/observe.go and the path-class manifest
   (metasystem/scripts/agents/path-classes.txt once landed): are the
   bounds checkable from the diff alone, and does bypassing the
   never-direct-fix list for behavior paths open anything a tier-1 item
   should not touch.
6. The open point for the human (reserved minutes per box while the
   dispatch reservation bug stands): state whether the recommendation
   is sound or name a better one.

A finding is material only if it changes what gets built and names the
artifact it changes. Findings that would only make the text longer are
not material; say so and stop. Return per the design-critic schema with
a verdict.

# Constraints

Wall-clock budget: 30 minutes. Read-only: do not modify the design;
propose changes as findings.

# Gap Rule

stop and report a gap; never fill it silently.
