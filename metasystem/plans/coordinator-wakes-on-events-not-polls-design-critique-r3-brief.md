Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Review brief: third rostered read, after the second fold, the last within the review budget

FINDING IDS: chain-unique; CWE-01 to CWE-10 are taken. A finding of an
earlier round that the fold resolved is returned under its OWN identifier
with `material: false`; one that still stands keeps its identifier with
`material: true`; a new defect gets CWE-11 onward. Never report a
resolution as prose inside another finding.

The "Declared Outputs" section the dispatcher appends below names the
outputs manifest and gives that manifest's SHA-256 digest; the design
page carries no declared digest.

Why this review exists: round 2 kept CWE-01, CWE-02, CWE-03, CWE-06 and
CWE-07 open and added CWE-10. The seat decided each and a Codex fold round
wrote the decisions into the landed page (the commit named in the goal's
next-step line); the fold brief
metasystem/plans/coordinator-wakes-on-events-not-polls-design-fold2-brief.md
carries the decisions verbatim. This is the third and last read the
goal's review budget allows; what remains material after it goes to the
human as a land-or-fold call. Judge the six folds first, then anything
new.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md. Contract:
metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md. Doctrine:
metasystem/docs/orchestration.md and
metasystem/docs/design/turn-verdict-delivery-contract.md. Code the page
builds on: metasystem/internal/run/waiter.go,
metasystem/internal/goal/turnverdict.go,
metasystem/scripts/agents/adapters/runtime-common.sh,
metasystem/scripts/agents/dispatch.sh,
metasystem/cmd/metasystem/channel_verbs.go.

# Mandate

1. For each of CWE-01, CWE-02, CWE-03, CWE-06, CWE-07 and CWE-10: does
   the page now decide it so an implementer builds without guessing?
   Return each under its identifier.
2. The one blocking route: is anything left that only made sense with the
   dropped resume route?
3. Section 6: does member one's DONE stand alone with the adapter answer
   inside it, and do members two and three each build on a verb that
   already works?
4. Plain English, as before.

If nothing material remains, say so; that closes the design chain and
the first member's goal and build brief follow from section 6.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
