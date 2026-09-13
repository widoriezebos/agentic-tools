Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Review brief: second rostered read, after the first fold

FINDING IDS: chain-unique; CWE-01 to CWE-09 are taken. A finding of
round 1 that the fold resolved is returned under its OWN identifier with
`material: false` (that is how the register closes it); one that still
stands keeps its identifier with `material: true`; a new defect gets
CWE-10 onward. Never report a resolution as prose inside another finding.

The "Declared Outputs" section the dispatcher appends below names the
outputs manifest and gives that manifest's SHA-256 digest; the design
page carries no declared digest.

Why this review exists: round 1 returned CWE-01 to CWE-08 material. The
seat decided each and a Codex fold round wrote the decisions into the
landed page (the commit named in the goal's next-step line); the fold
brief metasystem/plans/coordinator-wakes-on-events-not-polls-design-fold1-brief.md
carries the decisions verbatim. Judge the folds first, then anything new.

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
metasystem/scripts/agents/dispatch.sh, metasystem/cmd/metasystem/channel_verbs.go,
metasystem/internal/steward/revive.go.

# Mandate

1. For each of CWE-01 to CWE-08: does the page now decide it so an
   implementer builds without guessing? Return each under its identifier.
2. The stop table: do its five rows cover every combination the stop gate
   meets today (open work, unwatched work, idle backlog, waiting on a
   human, fences), and does row 1 keep the idle exemption exactly as a
   live delegate job's?
3. The three member goals in section 6: one mechanism each, own DONE, own
   fixtures, no member's DONE needing a later member.
4. Runtime independence, as in round 1.
5. Plain English: name any passage still too dense to quote into a build
   brief.

If nothing material remains, say so; that closes the design chain and
the first member's goal and build brief follow from section 6.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
