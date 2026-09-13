Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-decisions-record-deadline-evidence)
Date: 2026-09-13

# Review brief: independent critique of the design page, first rostered read

FINDING IDS: chain-unique, SDE-01, SDE-02, ... never F-n.

Why this review exists: member 2 of stop-hook-never-forces-an-empty-turn.
The seat wrote the page after member 1 landed (529d8a649 and its
fix-forwards); it is the record as landed. No read has been made yet.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/stop-decisions-record-deadline-evidence-design.md.
Contract: the umbrella's page
metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md (sections
3 and 7, the member's DONE and the version-2 record it asks for) and the
member's goal record (read it with `bin/metasystem goal show --id
stop-decisions-record-deadline-evidence`, the JSON field Intent). Code the
page builds on: metasystem/scripts/agents/supervision-hook.sh (the
deadline parent and the worker), metasystem/internal/report/stopblock.go
(the version-1 record), metasystem/internal/goal/turnverdict.go (the
verdict verb and its state), metasystem/internal/steward/component_evidence.go
(the hook attempt's generation and attempt sequence).

# Mandate

1. Section 1 against the code: every fact about today's two processes,
   the hook attempt and the record is true at the cited lines.
2. Section 2 against the code: the episode identity, the three verbs on
   the record, the verdict verb's completion under the record lock, the
   deadline parent's completion through the episode file, and the
   decision line are each buildable without guessing; name where an
   implementer would have to invent something.
3. The single-use rule: can a late worker still spend a seen marker or an
   idle count after the deadline completed the episode? Can the deadline
   parent's completion race the worker's under the two locks as ordered?
4. What member 1 landed (529d8a649, fed9f5d9e, 286efe41e, aa0f35e06):
   does this page contradict any of it, and does it keep the
   infrastructure allowance, the stop-condition lines and the launcher
   fallback as they are?
5. Plain English: name any passage a person outside this repository
   cannot follow.

If nothing material remains, say so; that closes the design chain and
the build brief follows from section 6.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
