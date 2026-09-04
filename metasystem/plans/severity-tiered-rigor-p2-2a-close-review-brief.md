Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: closing review of the tiering machinery, slice 2a, the risk basis (str-p2-build-2a, round 3)

Round budget: 1 focused round; this closes the slice. R-60-m1's rule:
material only if it changes what gets built and names the artifact.

Subject: the computed diff of implementer job str-p2-build-2a round 3
(its diff.patch under that chain's round-3 directory; reviewed tree
279d0cad5889532963ba5a57c687cce9925b9748, 32 files against main at 2900b863; the diff is the
authority). Three rounds built it: round one stopped on four gaps with
no diff; round two built all six items of
metasystem/plans/severity-tiered-rigor-p2-build-brief-2a.md with the
answers of metasystem/plans/severity-tiered-rigor-p2-build-2a-gap-brief.md
in force (folded into the design as revision 4.3,
metasystem/plans/severity-tiered-rigor-p2-design.md); round three built
the fixtures of metasystem/plans/severity-tiered-rigor-p2-build-2a-r3-brief.md
and the production fixes those fixtures forced. Round three's return
(return.json beside the diff) lists the fixture-to-test table and its
`decisions`; read both.

# Mandate

1. The four questions are the only basis of a tier. A Risk record
   (severity, novelty, exposure, accumulation, each 1 to 3, with its
   basis text) derives the tier by the design's table; `goal open` and
   `goal edit` refuse `--tier` without the four answers ("answer the
   four questions"); an override above the derivation is recorded with
   `--why`; the pair's override below is refused and the human's is
   recorded; lowering after claim is refused for the pair. Nothing in
   the diff derives rigor from the shape of the change (path class,
   file count, diff size): a shape-derived tier anywhere is material.
2. The raise is one transaction and clears nothing. A raise after claim
   rewrites only `Claimed.Revision` and the stop capability's
   `Generation` and `Revision`; `ClaimEpoch`, `StopFence` and the
   governed `Obligation` stay byte-for-byte (gap answer 3). A raise
   that calls `bindClaim`, or that clears a fence or an obligation, is
   material. A root dispatched before the raise keeps its `goalTier`
   and `gateWidth`; the next dispatch reads the new tier.
3. The misclassification raise carries evidence in the grammar of the
   build brief's item 3, and `refusal:<code>` admits exactly
   `AdmissionRefusalCodes` (BUDGET_UNKNOWN, BUDGET_REFUSED,
   HAZARD_REFUSED, RISK_UNANSWERED; gap answer 2); an unlisted code is
   refused at edit time.
4. The risk gate has two modes from the conf (`mark` prints the
   RISK_UNANSWERED line and admits; `enforce` refuses with the same
   code); an unknown mode is refused by validate; a dispatched root
   under accumulation 2 carries `gateWidth: full`, and the landing
   owner (`observeChain`, metasystem/internal/landing/observe.go)
   refuses a full-width chain without the full battery receipt.
5. `BudgetExceptions` counts every over-box member (elapsed, attempts,
   minutes, active jobs, review rounds) and two over-box operations
   end the appetite line with `repeated exception: defect signal`.
6. The review-round seam (item 6, gap answer 4): a goal-bound root
   reads the goal's `reviewRoundLimit` tuple member capped at
   `config.ReviewRoundMax`; a `--goal none-explicit` root reads the
   ceiling alone; the literal constant is gone.
7. The sweep row is `<goal-id> <s>,<n>,<e>,<a> <basis>` and the tool
   renders the listing line (gap answer 1); a human never types a tier
   in the draft.
8. Every fixture named in the r3 brief is a named test that exists in
   the diff and asserts written state or refusal text, not a bare call
   for coverage; a fixture the return lists as unbuilt is material
   unless the return names it under `decisions` with a reason the
   design supports.
9. Nothing outside the build brief's diffBoundary changed: no plans,
   records, memory, or files the build brief does not name.

Known and out of scope, do not report: the `dispatch` scenario of
dispatch-fixtures.sh, red on main since the alias landing 2c3776b8;
the goal package's full coverage number (the seat's run, recorded at
landing); the docs re-touch (docs/orchestration.md and the two critique
skills), which the seat writes and lands with this chain.

Finding identifiers: this repository's register refuses an id another
chain already carries. Name any finding STR2P2A-01, STR2P2A-02, ...
and never F-n.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree above. The diff is the subject; do not run the full
goal package (17 minutes on this host); run tests by name only if a
finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
