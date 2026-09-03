Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Review brief: code review of the tiering machinery, part one (chain str-build1c)

Round budget: 1 focused round, then at most one correction and its
re-review (the ceremony m3 and m2 agreed under Wido's R-73-m3; a second
correction goes to Wido with evidence). R-60-m1's rule: a finding is
material only if it changes what gets built and names the artifact.

Threat model: a defect shipped into the goal ledger's tier, tuple,
approval digest, classify sweep, tombstone, or the goalTier plumbing on
chain roots and the claim launch path; a binding test obligation not
actually proven by its test; the three-way merge onto main losing the
model-alias resolution (2c3776b8) or the goalTier plumbing; a caller
left on the old four-member tuple. Out: parts two, three and four;
revision 4's risk answers (another seat's part two); taste.

Scope: the computed diff of the implementer job under review (55
files against main; diff.patch is the authority). Contract:
metasystem/plans/severity-tiered-rigor-design.md revision 3 (points 1
to 3 as amended; build list part one), the four build briefs
(metasystem/plans/severity-tiered-rigor-build1-brief.md,
severity-tiered-rigor-build1-gap-brief.md, -gap2-brief.md,
-gap3-brief.md) and the continuation brief
metasystem/plans/severity-tiered-rigor-build1c-brief.md.

# Mandate

1. The four binding test obligations (STR3-MIGRATION-BOOTSTRAP-01,
   STR3-TIER-SNAPSHOT-PLUMBING-02, STR3-MISSION-CAP-BYPASS-07,
   STR3-BUILD-LIST-COVERAGE-08, verbatim in the part-one build brief):
   for each, the test exists, proves the demanded property, and is
   green; a test that proves less than the obligation demands is a
   material finding naming it.
2. The two configuration keys and their semantics per the gap brief
   (`metasystem.budget.review-round-max`, 0 removes the ceiling;
   `dispatch.cap-max` enforced by ResolveCap on every source; the tier
   boxes as five-member notation; validate refuses a box above the
   ceiling).
3. The classify-sweep contract per the gap-2 brief: draft grammar,
   normalized listing and digest, the five refusal codes, confirm as
   per-goal edits under the human actor, the TierLaw marker, preview
   mutates nothing.
4. The merge: the alias resolution of 2c3776b8 and the goalTier
   plumbing both survive in dispatch_verbs.go, build.go, cap_test.go,
   provenance_test.go.
5. Paper lenses m3 asked for (not gates, but name a finding if one
   bites): a new enforced rule is born marked with owner, review date,
   known-bad case and appeal route (ch. 12); misclassification is a
   defect with a record in both directions (ch. 6).

If nothing material remains, say so; that closes the chain and part
one lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema, with
the reviewedTree from validate conformance --stage review for job
str-build1c. Do not run scripts/agents/path-class-fixtures.sh (ripgrep
is absent on this host); the goal package's tests take about 17
minutes here, run them only if a finding needs them.

# Gap Rule

stop and report a gap; never fill it silently.
