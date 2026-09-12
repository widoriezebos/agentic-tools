Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-hook-never-forces-an-empty-turn)
Date: 2026-09-12

# Review brief: independent critique of the design page, second rostered read (chain stop-hook-design-190340)

FINDING IDS: chain-unique, SHK-03, SHK-04, ... never F-n; SHK-01 and SHK-02 are taken.

Why this review exists: the design page landed as the record (a314e17d3)
after three drafting rounds on Codex astra and three read-only reads on
Codex sol through the harness, all folded by the seat. The first rostered
read (job design-critic-9c6decadb978428a41209f9c) returned SHK-01 (the
Codex delegate boundary had no refusal lifecycle) and SHK-02 (the hook
log carried three duties without a record protocol; the eighth member
still named replaceable files). The seat folded both in the working tree
you read: the Codex adapter calls the gate after the provider returns and
before the job is terminal, excludes its own job from the live-work
exemption, and on a committed refusal re-invokes the provider once in the
same job under its remaining cap, recorded as a gate continuation; the
hook log has a line protocol (identity, kind, decision or incident fields,
arming aggregate), a checked append before emission, and acknowledgement
lines written by the drain; the eighth member counts hook-log decision
lines. Judge those two folds first; then the rest. This read reads the
page as it stands in the checkout, not the landed copy.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md. Contract:
the goal record metasystem/plans/goals/stop-hook-never-forces-an-empty-turn.md
(its DONE and the four absorbed clauses). The drafting briefs:
metasystem/plans/stop-hook-never-forces-an-empty-turn-design-brief.md,
metasystem/plans/stop-hook-never-forces-an-empty-turn-design-fold1-brief.md
and metasystem/plans/stop-hook-never-forces-an-empty-turn-design-fold2-brief.md.
Doctrine: metasystem/docs/orchestration.md (runtime independence) and
metasystem/records/goals/idle-with-backlog-alarm.md (the idle path that
must stay untouched).

What the three earlier reads found and the seat folded, so you need not
find them again: the census missed the Claude launcher's raw block on any
nonzero hook exit and collapsed the engine's fail-closed producers; runtime
independence was asserted, not designed, and the two-runtime proof was
fake plus Claude; the idle refusal offered no seat command (answer: the
claim verb is the first lawful act); the steward report had no fallback
when incident storage fails (answer: the hook log line is the second
channel, only both failing is unconfirmed); narrowing the idle digest was
an unauthorized amendment (dropped); the seen-state episode was a rolling
window (now keyed to turn generation and deadline end); the managed-seat
lifecycle for hosts without a native Stop was a second mechanism (moved to
a follow-on goal; the second production boundary is the Codex delegate
adapter's turn boundary); member order made members depend on later ones
(reordered, infrastructure first); the seven-day count had no owner or
stream (a member, fed by the hook log's decision lines).

# Mandate

1. Sections 1 and 2 against the code: every refusal site of
   metasystem/scripts/agents/supervision-hook.sh and every fail-closed
   producer in metasystem/internal/goal/turnverdict.go and
   metasystem/internal/goal/sessionstop.go is classified, and the
   classification is one an implementer can build without guessing.
2. DONE clauses 1 to 4 each have a home in section 7's members, and the
   idle-with-backlog path (its refusal, its counter, its three-refusals
   handoff, its digest) is untouched by every member.
3. Section 7: each of the eight members is one mechanism, lands green on
   its own in the stated order with the fixtures section 5 defines for it,
   and no member's DONE needs a later member.
4. The two design choices: an observed idle refusal that survives a lost
   verdict-state write by being recorded in the hook log line before
   emission; and the seven-day count from the hook log's decision lines.
   Say whether each holds against the code the page cites.
5. Runtime independence: the one Go gate at the Claude native Stop hook
   and at the Codex delegate adapter's turn boundary
   (metasystem/scripts/agents/adapters/codex.sh); nothing lives in one
   runtime's hook alone.
6. Plain English: no sentence a person outside this repository cannot
   follow.

If nothing material remains, say so; that closes the design chain and the
first member's build brief follows from section 7.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
