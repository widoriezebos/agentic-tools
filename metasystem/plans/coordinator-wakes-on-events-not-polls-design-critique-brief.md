Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Review brief: independent critique of the design page, first rostered read

FINDING IDS: chain-unique, CWE-01, CWE-02, ... never F-n.

Why this review exists: the design page landed as the record (1f6cb0e27)
after one Codex drafting round (chain implementer-fa5b5830882bcf6ea864cdf6)
folded by the seat, which added the member-goal split in section 6. No
read has been made yet. This read reads the landed page.

The "Declared Outputs" section the dispatcher appends below carries the
SHA-256 digest of the OUTPUTS MANIFEST (the one-line list naming the design
page's path), not of the design page; the page has no declared digest. Do
not stop on that digest.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md. Contract:
the goal record metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md
(its DONE, five clauses, and the runtime-independence clause). The drafting
brief: metasystem/plans/coordinator-wakes-on-events-not-polls-design-brief.md.
Doctrine: metasystem/docs/orchestration.md (the engine decides, adapters
own provider flags and delivery, an accelerator never carries
correctness) and metasystem/docs/design/turn-verdict-delivery-contract.md.
Code the page builds on: metasystem/internal/run/waiter.go (the waiter
record it extends), metasystem/internal/dispatch/watch.go,
metasystem/internal/proofrun/attempt.go, metasystem/internal/goal/attention.go,
metasystem/internal/goal/turnverdict.go (the stop gate's work-in-flight and
idle decisions), metasystem/scripts/agents/adapters/runtime-common.sh and
the three adapters beside it.

# Mandate

1. Section 1 against the code: every wait site it lists exists at the
   cited line and waits on the record it names; name any wait site of a
   seat it missed.
2. Section 2 against the code: the version-2 waiter record, the verb's
   selectors and exit codes, the source readers, the hint pipe, the adapter
   delivery route, the stop gate's PendingWait fact and the restart
   recovery are each buildable without guessing; say where an implementer
   would have to invent something the page does not decide.
3. DONE clauses 1 to 5 each have a home in section 5's fixtures and in
   section 6's three member goals, and no member's DONE needs a later
   member; say whether the measurement rules of section 5 can be taken as
   written on two runtimes.
4. Runtime independence: correctness never depends on a Claude, Codex or
   Devin facility; a runtime without a native wake gets the same guarantee
   from the records and the verb; name any place the page lets an
   accelerator carry a decision.
5. The three risks that matter most: a stale registered wait exempting an
   idle seat from the stop gate; a wait that never returns; a lost event
   between reading the source and registering. Say whether the page's
   answers hold against the code it cites.
6. Plain English: no sentence a person outside this repository cannot
   follow; the page is dense, so name the passages that need a plain
   rewrite before a build brief can quote them.

If nothing material remains, say so; that closes the design chain and the
first member's goal and build brief follow from section 6.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
