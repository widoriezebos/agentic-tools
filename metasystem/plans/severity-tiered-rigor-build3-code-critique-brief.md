Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Review brief: code review of the tiering machinery, part three (chain str-build3)

Round budget: 1 focused round, then at most one correction and its
re-review (R-73-m3's ceremony). R-60-m1's rule: material only if it
changes what gets built and names the artifact.

Threat model: a defect shipped into the landing evaluator's tier-1
class (floor, receipt binding, diff metric), the receipt verb, the
manifest's floor rows, land.sh or commit.sh; the binding obligation
STR3-TIER1-RECEIPT-PROOF-06 proven by less than it demands (a receipt
labelled with a tree but not bound to it); a tier-1 landing that can
bypass the second landing bar or the protected floor. Out: parts two
and four; revision 4's derivation (part three only reads gateWidth by
name); taste.

Scope: the computed diff of the implementer job under review. Contract:
metasystem/plans/severity-tiered-rigor-build3-brief.md (its obligation
verbatim) and metasystem/plans/severity-tiered-rigor-design.md revision
3, sections STR2-TIER1-PROTECTED-PATHS-12 and STR2-TIER1-EVIDENCE-13,
with revision 2's point 6.

# Mandate

1. The obligation: a receipt created while the checkout or index
   differs from the supplied tree is refused at creation, and
   observeDirectFix refuses a receipt whose tree is not the candidate
   tree; the test proves both.
2. The floor: every path point 12 lists is a floor row; a tier-1
   landing touching any of them refuses tier1-floor-refused; the four
   classes are unchanged.
3. The diff metric and shapes: changed lines counted as added plus
   deleted; binary, rename or copy, and mode-only changes refuse
   tier1-diff-shape-refused; the bound and its fixture (forty-one
   lines).
4. gateWidth: read from the root by name, absent means area, full
   requires the full battery command in the receipt.
5. land.sh --tests and commit.sh --test-receipt wired; the old message
   stamp gone; the second landing bar (direct-fix-floor-refused for
   behavior paths under carriage) unaffected.

If nothing material remains, say so; that closes the chain and part
three lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
str-build3. Do not run path-class-fixtures.sh.

# Gap Rule

stop and report a gap; never fill it silently.
