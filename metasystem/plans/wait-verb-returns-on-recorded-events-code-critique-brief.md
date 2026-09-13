Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the rostered read of the member-1 build, before its landing

FINDING IDS: chain-unique, WVB-40 onward (WVB-01 to WVB-36 were harness
reads' identifiers and are not in this register).

Why this review exists: the Codex sol build chain wait-member1-build-1
ran five rounds (round 1 gap-stopped on the brief; rounds 2 to 5 built
and folded three harness Opus reads of 10, 11 and 8 material findings;
the last read of round 5 found one low-severity ordering defect, which
the seat fixed in the worktree: a matched event now wins over a
claimable-frontier change seen in the same cycle). The seat's proof of
round 5 was green across the whole battery. This read is the gauntlet of
the landing: the seat lands the worktree as it stands if nothing
material remains.

Round budget: 1 focused round. A finding is material only if an
implementer must change the code or tests before this lands, and it
names the artifact it would change.

Specification under review: the worktree of the chain (the dispatcher
snapshots it for you). Contract: the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
(sections 2, 4, 5, 6) as landed, and the goal record's DONE (read it
with `bin/metasystem goal show --id wait-verb-returns-on-recorded-events`
from the installation; the JSON field Intent). Doctrine:
metasystem/docs/orchestration.md.

# Mandate

1. DONE, clause by clause: the four selectors and --resume with the
   typed exits; every adapter answering blocking and the row recording
   it; the version-2 row with its by-id pointer; restart recovery from
   the rows alone, with the WAITING lines from session start, goal next
   and report turn-verdict.
2. The named fixtures of section 5 for this member exist, assert what
   the page says, and use no wall time; the five bed legs launch the
   installed verb and Go owns their assertions.
3. Nothing in this member reaches into member two (publication hints,
   watch wrappers) or member three (the stop gate's PendingWait rows)
   beyond what the page assigns to member one.
4. The three decisions the seat made during the build and recorded on the
   page for the landing: the adapter operation's wire format
   (`wait-delivery --wait-id --nonce --deadline --session`, one word
   `blocking`), `ledgerCursor` on the question record, and the
   `question` field on the accepted answer History row. Say whether each
   holds against the code.
5. Plain English in error texts; source comments say what and why, never
   which round or finding.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
