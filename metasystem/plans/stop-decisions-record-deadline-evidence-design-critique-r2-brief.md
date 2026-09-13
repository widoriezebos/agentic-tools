Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-decisions-record-deadline-evidence)
Date: 2026-09-13

# Review brief: second rostered read, after the first fold

FINDING IDS: chain-unique; SDE-01 to SDE-10 are taken. A finding of
round 1 that the fold resolved is returned under its OWN identifier with
`material: false` (that is how the register closes it); one that still
stands keeps its identifier with `material: true`; a new defect gets
SDE-11 onward. Never report a resolution as prose inside another finding.

The "Declared Outputs" section the dispatcher appends below names the
outputs manifest and gives that manifest's SHA-256 digest; the design
page carries no declared digest.

Why this review exists: round 1 returned SDE-01 to SDE-08 material
(SDE-01 critical: the lost-refusal race). The seat decided each and a
Codex fold round wrote the decisions into the landed page (the commit
named in the goal's next-step line); the fold brief
metasystem/plans/stop-decisions-record-deadline-evidence-design-fold1-brief.md
carries the decisions verbatim. Judge the folds first, then anything new.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/stop-decisions-record-deadline-evidence-design.md.
Contract: the umbrella's page
metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md (sections
3 and 7) and the member's goal record (`bin/metasystem goal show --id
stop-decisions-record-deadline-evidence`, field Intent). Code:
metasystem/scripts/agents/supervision-hook.sh,
metasystem/internal/report/stopblock.go,
metasystem/internal/goal/turnverdict.go,
metasystem/internal/steward/component_evidence.go,
metasystem/internal/atomicfile/atomicfile.go (the three write outcomes).

# Mandate

1. For each of SDE-01 to SDE-08: does the page now decide it so an
   implementer builds without guessing? Return each under its identifier.
2. SDE-01 in depth: with the worker spending only under the record lock,
   the verdict-state lock taken inside it, the parent waiting 2.5 of its
   3 reserved seconds and writing `deadline-emitted` when it cannot take
   the lock, is there still any interleaving in which a seen marker or an
   idle count is spent for a refusal the seat never received, or in which
   the parent and the worker both complete? Walk the interleavings.
3. The episode key and the uncertain form: any collision left?
4. Plain English, as before.

If nothing material remains, say so; that closes the design chain and
the build brief follows from section 6.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
