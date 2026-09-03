Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: code review of the tiering machinery, slice 2b (chain str-p2-build-2c)

Round budget: 1 focused round, then at most one correction and its
re-review (the ceremony m3 and m2 agreed under Wido's R-73-m3; a second
correction goes to Wido with evidence). R-60-m1's rule: a finding is
material only if it changes what gets built and names the artifact.

Threat model: a defect shipped into the close of a critic chain (the
material stop, the close table, the review obligations, the accepted-
risk transition, the out-of-scope write, the register union at merge);
a binding fixture obligation not actually proven by its test; a chain
dispatched before this slice (no artifact member, no register) that
this slice now refuses or, worse, closes wrongly; a write outside the
commit orders the briefs fix; a human-only verb reachable by an agent.
Out: part one (the tier, the tuple, the sweep; another chain on m2),
slice 2a (the risk answers; the next chain on this seat), the docs,
taste.

Scope: the computed diff of the implementer job under review (job
str-p2-build-2c, round 1, its diff.patch under the chain's round
directory; reviewed tree a1ce1f300cc05298196c6394fe428f8e045ca733, 42
files against main at 719f0cf0; the diff is the authority). Contract:
metasystem/plans/severity-tiered-rigor-design.md revision 3 part two
(sections 05 to 11 as amended), metasystem/plans/severity-tiered-rigor-p2-design.md
revisions 4.1 (fold 008: the register's second kind lands here) and
4.2, and the four briefs: metasystem/plans/severity-tiered-rigor-p2-build-brief-2b.md
(items 1 to 7, the fixture obligations, the diffBoundary),
metasystem/plans/severity-tiered-rigor-p2-build-2b-gap-brief.md (gaps 03,
04, 05), metasystem/plans/severity-tiered-rigor-p2-build-2b-gap2-brief.md
(out-of-scope write, discharge selection, the register line) and
metasystem/plans/severity-tiered-rigor-p2-build-2c-brief.md (the
finishing round and its gate findings). The implementer's return (return.json of that round) lists eleven
recorded decisions under `whatWasDone`; each is reviewable, none is a
gap.

# Mandate

1. The fixture obligations: every fixture named in the three 2b briefs
   (STR3-GAP03-OUTPUTS-GRAMMAR, STR3-GAP04-OBLIGATION-ROUNDTRIP,
   STR3-GAP05-ACCEPT-THEN-CLOSE, STR3-GAP-OOS-WRITE,
   STR3-GAP-DISCHARGE-SELECT, STR3-GAP-REGISTER-LINE, STR2B-SEAM-CONSTANT,
   STR4-R1-MISCLASSIFICATION-KIND, STR2B-RENAME-EITHER-SIDE,
   STR2B-CLOSE-ONE-WRITE, and revision 2's part-two list) has a test
   that exists, proves the demanded property, and is green; a test that
   proves less than the obligation demands is a material finding naming
   it.
2. The close table and the commit orders: the close writes its states
   in the order the design fixes and is idempotent on rerun; accepted-
   risk entries leave the unresolved set before the table is read; the
   out-of-scope write refuses severe and unproven before any write;
   `goal accept-risk` is human-only by the same proof as
   `SetBudgetApproved`.
3. Compatibility: the implementer's eleventh decision restores the
   pre-2b final-return path for a critic chain WITHOUT a register and
   keeps a malformed present register a refusal. Judge whether that
   line is the right one (a chain dispatched before this slice must
   still merge; a chain of this slice must not be able to dodge the
   register by omitting it) and whether
   TestMergeCritiqueKeepsRegisterlessChainCompatibility proves both
   halves.
4. The seam: `goalReviewRoundLimit` returns 3 with the comment naming
   part one's tuple member; nothing part one owns (Tier, the tuple,
   norm.go, config/budget.go, the sweep) is touched.
5. Paper lenses (not gates; name a finding if one bites): the material
   stop is a counted signal, not a shape rule (06-proof-over-trust);
   every refusal names its next step; a new enforced rule is born with
   owner, review date, known-bad case and appeal route (ch. 12).

Known and out of scope, do not report: the four reds on main from
landing c285d5a0 (TestAuthenticatedChannelApprovalRequiresTheTokenOnce,
the dispatch-fixtures `dispatch` scenario, the goal-cli scenarios
structured-budget, scope-bounds, archive-and-prune); the goal package's
full test run (27 minutes here; the dispatching seat ran it green but
for that one test on the identical goal code).

If nothing material remains, say so; that closes the chain and slice
2b lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema, with
the reviewedTree above. Read the chain's delegate worktree (the str-p2-build-2c worktree under
the agent worktrees directory) for context; the diff is the subject. Do not run scripts/agents/path-class-fixtures.sh
(ripgrep is absent on this host); run a test only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
