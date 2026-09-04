Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2-2a-carry)
Date: 2026-09-04

# Review brief: closing review of the docs re-touch of the risk basis (str-p2-2a-carry-docs, round 1)

Round budget: 1 focused round; this closes the carry goal. R-60-m1's
rule: material only if it changes what gets built and names the
artifact.

Subject: the computed diff of implementer job str-p2-2a-carry-docs
round 1 (its diff.patch under that chain's round-1 directory; reviewed
tree ff082d0515412ba8e553c948dd56b8807717283b, three prose files
against main at be154ed2; the diff is the authority). The risk basis
itself landed on main as b4ae9395 after two closing reviews
(str-p2-build-2a-cc2, str-p2-2a-carry-cc); this diff is the prose that
describes it: docs/orchestration.md and the two critique skills drop
the "PENDING Part Two" clauses that b4ae9395 made law and describe the
risk record, the review-round accounting, the register close and the
budget-exception signal. The build brief
(metasystem/plans/severity-tiered-rigor-p2-2a-carry-docs-brief.md)
carried the patch verbatim and asked the round to fact-check every
mechanism name; the round's return reports every name found and no
correction made.

# Mandate

1. Every mechanism the new prose names exists in the tree by that name
   and does what the sentence says: `job critique-budget-rebind`, `job
   critique-register-close`, `goal discharge-review-obligation`, `goal
   accept-risk`, `metasystem.budget.risk-gate` with modes `mark` and
   `enforce` and the refusal `RISK_UNANSWERED`,
   `metasystem.budget.review-round-max`, `gateWidth: full` under
   accumulation 2 or higher and the landing refusing a full-width chain
   without the full battery receipt, `--risk
   severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>` with
   `--basis` on `goal open` and `goal edit`, a bare `--tier` refused,
   `--why` for an override above the derivation, `--evidence` in the
   grammar `root:<jobId>`, `finding:<jobId>/<id>`, `refusal:<code>`,
   `BudgetExceptions` and the `repeated exception: defect signal` line,
   the sweep row `<s>,<n>,<e>,<a>`. A sentence that names a mechanism
   the tree lacks, or states a behavior the tree contradicts, is
   material; cite file:line for the contradiction.
2. The prose removes no rule that still binds: a "PENDING" clause
   dropped whose machinery did not in fact land is material.
3. The diff touches exactly the three prose files and nothing else.

Known and out of scope, do not report: wording, ordering, length; the
carry goal's own missing Risk record (opened before the law, approved,
claimed; the classification sweep is its path).

Finding identifiers: this repository's register refuses an id another
chain already carries. Name any new finding STR2P2A-16, STR2P2A-17, ...
and never F-n.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
the reviewedTree above. Search the tree; run no test packages.

# Gap Rule

stop and report a gap; never fill it silently.
