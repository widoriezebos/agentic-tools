Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Review brief: the terminal grade for stopping acts after the fold (chain hp-terminal-build1-20260909, reviewing round hp-terminal-build1-20260909-r2)

FINDING IDS: chain-unique, continue the sequence: HPT-04, HPT-05, ...
never F-n and never a reused id. Report `round` as 1 in your return:
it is this job's own round.

You are the second critic on this chain. The first found HPT-01 (the
seeded opids used a letter outside the ledger's alphabet, so the goal
package was red), HPT-02 (the scenario's human assertions passed
--lineage, the one case the old builder let through unproven, so they
were green on the untouched tree) and HPT-03 (register row order); the
orchestrator's gate found the scenario's pseudo-terminal holder killed
before cleanup. All were folded in round two; the dispositions are in
the records directory under misc, in
hp-terminal-grade-for-stopping-acts-critique-r1-dispositions.md (new,
not yet committed). Two goal-cli scenarios, brain-stop-seeded and
brain-stop-corrupt, fail on main since d533caf17 and belong to goal
brain-summary-leads-the-stop-display; they are not this chain's.

Under R-60-m1 you stop at the first round with no material finding:
material only if it changes what gets built and names the artifact. If
nothing material remains, say so; that closes the chain and the fix
lands.

Threat model for this round: the fold weakening what round one got
right (ProveTerminal still Enroll's walk without the write, node for
node; Grade empty on refusal; ValidFor unchanged; the helper on exactly
the four rows); the goal test now green for a reason other than the
grade it names; the scenario's assertions still passing on the untouched
tree by any route (a supplied lineage, a fixture grant, a skipped
proof); the holder fix leaving a process behind or proving ownership by
command text alone; a change outside the declared boundary.

Scope: the computed diff of round hp-terminal-build1-20260909-r2 (the
chain's cumulative diff against its base). Contract: the build brief and
the fold brief in the plans directory
(hp-terminal-grade-for-stopping-acts-build-brief.md and
-fold-r2-brief.md, not yet committed) and the goal record
metasystem/plans/goals/hp-terminal-grade-for-stopping-acts.md. The
design is metasystem/plans/human-proof-fits-the-act-design.md revision 2.

# Mandate

1. HPT-01 closed: the test seeds valid ids and history and proves that
   unpark-to-queued takes the terminal grade while unpark that restores
   approval is refused at the terminal grade.
2. HPT-02 closed: no human_runs assertion supplies a lineage; the derived
   terminal-<id>-0 lineage is asserted where the record shows it; each
   assertion would fail on the untouched tree for its stated reason.
3. The holder scenario holds its own session leader deterministically
   and proves ownership by pid and start time, and leaves nothing behind.
4. Nothing round one certified has regressed; nothing outside the
   declared boundary changed.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
hp-terminal-build1-20260909-r2; if your sandbox cannot run it, read the
tree from the review record the orchestrator writes beside the diff, as
the earlier critic did, and say so. The orchestrator runs the full
packages, the coverage floors and the goal-cli bed before your return
lands.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
