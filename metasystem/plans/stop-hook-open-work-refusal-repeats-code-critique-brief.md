Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-open-work-refusal-repeats)
Date: 2026-09-07

# Review brief: the open-work refusal that repeats (chain show-build1d-20260907)

FINDING IDS: chain-unique, SHO-01, SHO-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Threat model: a refusal that never fires (a seen-state written before
the first block, or shared across checkouts); a seen-state that
survives a plan line's change (a changed line must refuse once more);
a running job or open chain counted in flight when it is not (a
terminal job record; a chain whose newest round is terminal); a
placeholder classifier that swallows real open work; the deadline
path writing or reading a different file than the ordinary path; a
seen-state file that grows without bound or breaks the verdict when
unreadable (an unreadable file must fail open to the old behaviour,
never crash the hook). Out: the wording of verdict lines; taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-open-work-refusal-repeats-build-brief.md
and the goal record metasystem/plans/goals/stop-hook-open-work-refusal-repeats.md.

# Mandate

1. The once-only promise holds per plan line across turns and across a
   deadline expiry, keyed by plan path plus a digest of the line; the
   fixture proves two stops and an expiry.
2. A pending or running job record on the checkout, or a chain root
   with chainClosed false whose newest round is not terminal, makes
   the open-work verdict allow; the fixture proves the job case.
3. "<one line, required>" and the other template tokens render as
   TEMPLATE-UNFILLED on their own line and never as OPEN-WORK.
4. An unreadable or malformed seen-state fails open to today's
   behaviour with a visible line; nothing crashes the hook.
5. Nothing outside the named owners changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
show-build1d-20260907.

# Gap Rule

stop and report a gap; never fill it silently.
