Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Review brief: the terminal grade for stopping acts (chain hp-terminal-build1-20260909)

FINDING IDS: chain-unique, HPT-01, HPT-02, ... never F-n. Report
`round` as 1 in your return: it is this job's own round.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

This is the first of eight successors of human-proof-fits-the-act,
whose design (metasystem/plans/human-proof-fits-the-act-design.md,
revision 2, sections "The two proofs" and the stopping rows of section
1) is the authority. What must remain impossible is the whole point:
an agent passes neither grade anywhere; acts that grant or widen
authority keep the enrolled walk unchanged.

Threat model: ProveTerminal admitting a walk with an adapter signature
at any node, a node whose terminal differs from the invoker's, a
headless invoker, or an unreadable or changed ancestry (compare it
node for node with the walk Enroll performs, and with Prove's; any
difference is a finding); Enroll no longer equal to ProveTerminal plus
the write; Grade set on a refused proof; ValidFor accepting a
terminal-grade proof (every caller that checks it today must still get
the full walk); a proof record written before the change read as
anything but enrolled; requireHuman applied to any row other than
park, release, unpark-to-queued and session stop, or any of those four
still carrying its old Actor.Human test beside the helper; the builder
calling ProveTerminal for a verb outside the four, or dropping the full
walk's refusal so a later grade refusal cannot be rendered as the
enrolled row; the derived lineage terminal-<id>-0 colliding with an
enrolled generation's lineage or being accepted where an enrolled
lineage is required; the new register row breaking the register's
uniqueness or shape checks; a fixture that is green for a reason other
than the one it names (an unenrolled human allowed to stop must be
refused for approve in the same scenario, and an agent shell refused
for both); any change outside the boundary the return declares. Out:
wording; the refusal texts (a sibling goal); taste.

Scope: the computed diff of the implementer job under review.
Contract: the build brief hp-terminal-grade-for-stopping-acts-build-brief.md
in the plans directory (new, not yet committed) and the goal record
metasystem/plans/goals/hp-terminal-grade-for-stopping-acts.md.

# Mandate

1. ProveTerminal is Enroll's walk without the write, node for node, and
   Enroll is ProveTerminal plus the write.
2. Prove's no-enrollment branch returns TERMINAL_NOT_ENROLLED only after
   the terminal walk reached the session leader, and returns that
   walk's refusal otherwise; the new code is registered.
3. ValidFor is unchanged in meaning; TerminalValidFor accepts either
   grade; pre-change records read as enrolled.
4. requireHuman replaces the Actor.Human tests of exactly the four rows,
   each at the terminal grade, and no other row changed.
5. The builder proves high then low for the four rows only and derives
   terminal-<id>-0 for a terminal-grade proof. The implementer stopped
   on a gap here, correctly: the brief asked it to keep the full walk's
   refusal for a later renderer but named no carrier; the orchestrator
   accepts that gap and assigns the carrier to the sibling goal
   hp-refusals-print-the-one-command (design finding HPA-14). Its absence
   in this diff is not a finding; a partial or invented carrier would be.
6. The fixtures prove: agent shell refused for stop and approve; human
   at an unenrolled terminal allowed to park, release, unpark and
   session-stop and refused for approve; headless caller refused. Each
   would fail on the untouched tree for its stated reason.
7. Nothing outside the declared boundary changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
hp-terminal-build1-20260909; if your sandbox cannot run it, compute the
tree by hand as earlier critics did and say so. The orchestrator runs
the full packages, the coverage floors and goal-cli-fixtures.sh before
your return lands.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
