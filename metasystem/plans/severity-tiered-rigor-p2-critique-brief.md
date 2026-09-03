Working Mode: design-critique
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-03

# Goal

Review metasystem/plans/severity-tiered-rigor-p2-design.md (revision 4
of the tiering design: the tier derived from four recorded risk
answers) against its parent metasystem/plans/severity-tiered-rigor-design.md
(revisions 1 to 3, read all of it first, the build list of revision 3
last) and against the tree at your base. This is the ONE design review
of this slice: Wido's stop criterion (R-60-m1) applies, one round, then
the agreed parts build and every open finding becomes a named test
obligation of the build brief. Wido's order, verbatim: "I want the
second part to include the risk scoring 'Risk is four separate
questions, never the shape of the change' because without it is is a
stupid deterministic rue at best."

# What to check, in this order

1. Fidelity to the paper: metasystem/docs/paper/06-proof-over-trust.md
   line 51 and 11-economy.md lines 23 and 55. Are the four questions
   carried as four separate recorded answers, and does nothing in the
   derivation reintroduce size or kind as the risk? Is the accumulation
   rule (a floor of tier 2 and the full battery, never a design round)
   a faithful reading of "accumulation justifies a broad examination"?
2. Fit with part one as designed (parent 01, 02, 03, 12): field names,
   the digest, the edit rules on approved/claimed/parked goals, the
   sweep draft, `goalTier` on roots. Name every place where revision 4
   contradicts part one rather than amending it, with the parent line.
3. Code grounding at your base: internal/goal/file.go (ApprovalDigest
   at line 101), verbs.go (SetBudgetApproved at 585, the Edit refusal
   near 1193), approval.go (appendRootChange 117, ApproveSweep 359),
   internal/config/validate.go (Validate at 22), internal/steward/health.go
   (the claimed-goal-appetite role at 55), internal/counselor (the
   register writer and sources.go schema). A cite that does not hold is
   a finding.
4. The override and misclassification rules (16, 17): can a pair lower
   its own tier by any path? Can a raise after claim silently displace
   the claim or the approval? Is the evidence requirement checkable?
5. Marking mode (18): does it leave any goal dispatchable without a
   tier after the TierLaw marker? Is the known-bad case real
   (goal fable-model-alias, plans/goals/fable-model-alias.md, its
   history in memory/rulings.md R-71-m3, R-72-m3)?
6. The exception counter (19): a signal, never a gate; confirm no
   refusal path reads it.

# Rules of the return

Version-2 design-critic JSON. Every finding carries `material` per
R-60-m1 (it changes what gets built) and names the artifact it changes
(a path in the design or the tree, or NEW <path>). Mechanical-grain
findings (wording, a cite off by lines, a missing fixture name) are not
material: list them under one non-material entry. Do not propose a new
mechanism; the loop has one round. Do not run the engine (KNOWN SANDBOX
LIMIT: reading only). Wall-clock budget: 30 minutes.

# Gap Rule

stop and report a gap; never fill it silently.
