Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

Round 2 of your critique of metasystem/plans/two-bars-caller-class-design.md,
now revision 2 (landed, in your workspace). Round 1 closed on three
accepted findings — TBCC-R1-LAWFUL-DELEGATE-COMMIT-PATH,
TBCC-R1-FIXTURE-STUB-CONTRACT, TBCC-R1-NEGATIVE-BRANCH-PROOFS — with the
orchestrator's evidence in
metasystem/plans/two-bars-caller-class-dispositions.md. Revision 2 folds
them: a fourth verdict path, `worker`, for a delegate committing inside
its own dispatched worktree (custody join on process id and start time,
instance tag as a cross-check, worktree geometry from the git common
directory); a complete stub contract for the fixture beds; three
negative legs.

# Inputs: decisions already taken, so you do not re-raise them

- The design's open gap F13 (the pre-commit guard reads the wrapper
  token under the main root while a worktree wrapper mints it under the
  worktree) is DECIDED by the orchestrator as resolution (a): the guard
  derives its root from the committing repository's geometry. It is a
  pre-existing defect and rides as its own next slice on the goal; the
  wrapper's token placement is unchanged, so the design's reject
  condition (c) does not fire. Treat the guard as out of this design's
  box (its section 8 says so).
- The custody join on (pid, start) as the primary derivation with the
  tag optional, because the Devin ACP server carries no tag: accepted.
- `--push` refused on the worker path, and the worker trailer naming
  the custody-joined RUNNING job (a follow-up's -rN id): accepted by the
  orchestrator; confirm or attack them on their merits.

# Review brief

Round budget: this is round 2 of three on this chain; failsafe round 3.
Threat model, scope and materiality criterion unchanged from round 1
(metasystem/plans/two-bars-caller-class-critique-brief.md).

Verify first that each of your three round-1 findings is actually
folded — by reading the revised sections, not the changelog — and say
so per finding id. Then attack revision 2, in particular the worker
rule (design section 3, steps 1-5): can a delegate in someone else's
worktree, a stale worktree, or the main checkout obtain `worker`; is
the "a worktree commit never lands" premise true for every worktree
commit given land.sh's own push of agent branches; can a worker forge
or borrow the trailer suffix; do the geometry equalities survive
symlinked roots and an empty prefix; do the three negative legs
discriminate exactly the branches they claim; is the stub contract
executable as written against the beds at
metasystem/scripts/agents/static-reproof-fixtures.sh:220-252 and
:400-424.

Return format: the design-critic schema; stable identifiers
TBCC-R2-<name>; a clean verdict is `verdictMaterialCount: 0` with any
non-material observations recorded.

# Constraints

Wall-clock budget: 25 minutes. Do not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
