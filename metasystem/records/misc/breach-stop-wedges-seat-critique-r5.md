# Critique r5 of chain bsws-build1b-20260909 (goal breach-stop-wedges-seat; read charged to goal dispatch-admission-refuses-on-a-fenced-sibling, both in arc fenced-claim-wedge)

Critic bsws-crit6-20260910 (code-critic, claude claude-opus-5, xhigh,
read-only) on work round bsws-build1b-20260909-r8, reviewed tree
2bf712b19eb8b11805abd517769250e729732d44, 2026-09-10 10:28 to 10:38Z. Four
findings, none material. The chain closes on this read.

| id | severity | finding, in one line | disposition |
|---|---|---|---|
| BSW-18 | low, not material | The channel report's FENCED lines sit in the Next up block, which is dropped when nothing was delivered in the window; a quiet machine whose only claim is stopped shows no FENCED line in the report; same visibility as before the chain | note |
| BSW-19 | low, not material | In the turn verdict's normal branch, the early exit for "WORK IN FLIGHT ... claimable shared backlog also includes" leaves before the FENCED lines are appended, so in that one state the display omits a stopped claim beside the live one; display only, no decision reads it | note; one line added to goal stop-hook-treats-a-fenced-goal-as-current-work, which owns the verdict's fenced lines |
| BSW-20 | low, not material | Since round 7, a stopped sibling whose stop batch is still open no longer refuses the lineage's other dispatches; the steward's custodian walks the same routes each tick and reports unsettled batches, and resume still refuses until the batch is complete; the loud signal moved from dispatch to the steward report | note; matches the chain's intent |
| BSW-21 | low, not material | The brain declaration lists every held claim as an obstacle and says "goal release", which a stopped claim always refuses | already its own goal: brain-advice-names-a-refused-release |

The critic's gaps: it read but did not run dispatch.sh's path for a dispatch
against the fenced goal itself; it ran the chain's focused fixtures and
revert experiments on a scratch copy, not the full packages or beds (the
orchestrator ran the dispatch and goal-cli beds and the chain fixtures green
outside the sandbox on this tree); it inferred, not byte-compared, that a
refused resume leaves the goal file unchanged.
