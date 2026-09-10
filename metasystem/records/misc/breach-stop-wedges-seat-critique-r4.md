# Critique r4 of chain bsws-build1b-20260909 (goal breach-stop-wedges-seat; read charged to goal dispatch-admission-refuses-on-a-fenced-sibling under R-90-m1d's predecessor arrangement)

Critic bsws-crit5-20260910 (code-critic, claude claude-opus-5, xhigh,
read-only) on work round bsws-build1b-20260909-r7, reviewed tree
87717559a2947b2df9f5a39e0e44c4a063144fae, 2026-09-10 09:06 to 09:18Z. The
chain had reached breach-stop-wedges-seat's three-read ceiling (R-42-m0),
so this read was dispatched under the goal opened for finding BSW-12, which
it examines. Three findings, one material. Dispositions are the
orchestrator's (m1d).

| id | severity | finding, in one line | disposition |
|---|---|---|---|
| BSW-15 | medium | The chain cannot land on current trunk as reviewed: round 7 added internal/dispatch/admission_test.go, and trunk commit a633b7a08 (103 commits past the chain's base) created a different file at that path; the patch fails there, every other hunk applies; the resolution is mechanical (move the chain's three admission fixtures to another file name) but changes the tree, so it must be built and re-read, not hand-resolved at landing | accept; fold in round 8: rename the chain's file (admission_fenced_sibling_test.go), rebase onto current trunk, rerun; one more read |
| BSW-16 | low, not material | Of the three round-7 fixtures, TestGoalRevisionAdmissionStillRefusesTheFencedGoal stays green when the fold is reverted: it guards the unchanged per-goal check against a future loosening, not this fold; the other two go red | note; the fixture is worth keeping for what it does guard |
| BSW-17 | low, not material | The brief said the machine-wide scan skips fenced claims "only when the dispatch is for another goal"; the scan takes no goal id, so it skips them for every dispatch, and the stopped goal's own refusal comes from the per-goal check one step later in dispatch.sh, with a clearer message naming the stop. A dispatch with no goal on a machine whose only claim is stopped is now admitted (it charges nothing to the stopped goal) where before it was refused | note; both effects match the chain's intent; the no-goal case is worth a line on goal steward-continues-a-fenced-goal, whose intents were minted exactly there |

The critic's gaps: it used the round-7 review record for the reviewed tree;
ran the sixteen chain fixtures, revert probes and go vet on scratch copies,
not the full packages or beds; read but did not run dispatch.sh's paths for a
dispatch against the stopped goal; chose its own file name for the
trunk-merge scratch (the orchestrator's resolution needs its own run).
