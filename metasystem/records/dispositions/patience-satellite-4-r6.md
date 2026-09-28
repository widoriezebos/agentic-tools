# Dispositions: patience-satellite-4, round 6

Job: design-critic-20260811t190203z-44ad (codex gpt-5.6-sol, xhigh).
6 findings, 6 material, all accepted. Three are fold-consistency debt
(the sketch and verification tails not carrying round-5 fixes) — this
fold therefore ends with a full sketch/verification audit against all
46 dispositions.

| id | disposition |
| --- | --- |
| P4-041 | accepted — model evidence must be a canonical model key for the record's runtime; shipped sentinels (`unobserved`, `multi-model:<names>`) canonicalize nonempty but are NOT model evidence and fall through to the requestedModel row and onward. Silent infinite patience through a sentinel is closed. |
| P4-042 | accepted — the table gains a runtime-missing rule for reservation husks (pending-setup husks lawfully failed with no runtime, requested model, or effective model): with no usable runtime, match the role's entries across ALL runtimes and take the smallest floor; with no entries for the role at all, infinite. Damage never widens patience; husks count barren under the smallest configured tolerance. |
| P4-043 | accepted — the status rule becomes three-way: TerminalJobStatuses counts when uncertified; the KNOWN lawful nonterminal vocabulary (dispatch lifecycle statuses) does not count — those jobs are in flight; only a status missing or outside BOTH vocabularies is damaged and counts, fail-toward-vocal. The round-3 wording that counted anything nonterminal was wrong and would have counted running jobs as barren. |
| P4-044 | accepted — fold debt: sketch and verification now say three forms (chain, orphan, overflow) with round-trips for all three. |
| P4-045 | accepted — fold debt: the sketch's input boundary and the verification list now carry jobId-equals-filename-stem verbatim. |
| P4-046 | accepted — fold debt: verification names the ranking test — breach distance ordering with unequal floors, proving a count-descending comparator fails. |
