# Dispositions: patience-satellite-4, round 8

Job: design-critic-20260811t193533z-38bc (codex gpt-5.6-sol, xhigh).
5 findings, 4 material, all five accepted (the non-material P4-055
is cheap and right). Trend: one high on a real definition gap, the
rest verification composition and wording — the convergence tail.

| id | disposition |
| --- | --- |
| P4-051 | accepted — the 20-line bound and the overflow cover ALL Patience lines, breaches and orphan reports alike; the overflow projection wording becomes "more chains need attention (see ledger)" so orphans can neither vanish past the bound nor be called floor-breaches. |
| P4-052 | accepted — verification gains: a singleton terminal orphan emits despite count one and a positive configured floor for its role; and a mixed breach-plus-orphan set at the nineteen-detail cutoff ranks breaches first, orphans after, overflow counting both. |
| P4-053 | accepted — verification gains the composition case: damaged-status jobs supply the model evidence that drives rows one and two (unknown-status jobs on model A select A's floor, not another model's). |
| P4-054 | accepted — every row names its EVIDENCE RECORD and the whole triple is drawn from that one record: rows 1-2 from the newest counted job with model evidence; rows 3-4 from the chain root; rows 5-8 from the newest counted job whose relevant fields are valid, root first for role. An evidence record with invalid role or runtime fields is treated as absent for that row and selection falls through. One record, one triple, no cross-record chimeras. |
| P4-055 | accepted (non-material) — docs/patience.md's opening and docs/glossary.md drop the remaining multi-observable, pure-replay-of-patience, and park-on-stall phrasings in favor of the settled design's language. |
