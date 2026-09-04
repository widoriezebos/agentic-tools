Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Review brief: part four of the tiering machinery, the docs (chain str-build4)

FINDING IDS: chain-unique, STR4D-01, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review. R-60-m1's rule: material only if it changes what gets built
(here: what the documents say) and names the artifact.

Threat model: a document stating a rule the machinery does not
enforce, or contradicting a landed name (parts one and three on main:
6c86953a and efaa5cf4) or a ruling (R-54-m1, R-60-m1, R-42-m0,
R-58-m1, R-73-m3 in metasystem/memory/rulings.md); a pending part-two
name presented as landed; a specimen paragraph rewritten as history
into something that did not happen; a change to a file the brief did
not name; a matrix row whose proof does not exist. Out: taste; the
paper's voice beyond the repository's own standard.

Scope: the computed diff of the implementer job under review. Contract:
metasystem/plans/severity-tiered-rigor-build4-brief.md; the design
revisions 2 to 4 (metasystem/plans/severity-tiered-rigor-design.md and
metasystem/plans/severity-tiered-rigor-p2-design.md).

# Mandate

1. Every sentence about the review budget, the stop criterion, the
   tier boxes, the config keys and the tier-1 landing matches the code
   on main (check the names against the landed files).
2. Every pending part-two mechanism is marked pending, in one clause.
3. The obligation matrix has one row per mechanism point of the three
   revisions, with proofs that exist for parts one and three (name a
   row whose proof you cannot find as material).
4. No file outside the named set changed.

If nothing material remains, say so; that closes the chain and part
four lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
str-build4.

# Gap Rule

stop and report a gap; never fill it silently.
