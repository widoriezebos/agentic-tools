# plan-scanner-reads-fenced-code

- State: done
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: it misreports, it does not break a landing or lose work; novelty 1: skipping fenced regions while scanning Markdown is ordinary parsing; exposure 2: every Stop verdict on every seat carries the false line, and any design documenting a field grammar adds another; accumulation 2: it is permanent noise in the one report the seat reads at every turn end, and noise in that report is what makes a real open-work line easy to miss"
- Tier: 2
- Intent: The turn-end plan scanner reads fenced code blocks as document content, so a design that DOCUMENTS a field grammar trips TEMPLATE-UNFILLED on its own example and every Stop verdict carries the false alarm
- Origin: main
- Next step: Reproduce: plans/goal-scope-bounds-design.md lines 287-288 are inside a fenced block documenting the split member draft format, and internal/report/openwork.go openWork() reports TEMPLATE-UNFILLED on them. internal/report/openwork.go contains no fence handling at all (grep -cF for a triple backtick returns 0), so planField scans the whole document. Fix: skip fenced regions in planField and check whether openWork's sibling scanners (OPEN-WORK, STALE-PLAN, internal/report/scan.go:170) read the same text and need the same skip. Cover with a fixture whose plan documents a field grammar inside a fence.
- Concluded: Landed: Parked as a duplicate of open-work-scan-reads-fenced-examples and plan-fields-outside-fences, both concluded: landed d533caf1 (2026-09-09; planField in internal/report/openwork.go ignores fenced examples, with tests). Concluded 2026-09-11 in the backlog consolidation on Wido's word, no further work.
- OpenedAt: 2026-09-09T08:20:28Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-09T08:20:28Z 1ZG76B8TPY6QRT7GR6ADPSF6H8-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=plan-scanner-reads-fenced-code
- 2026-09-09T08:41:23Z M4XKMK45SS8VE6STTSWQV92WJ6-m1-1701c13c park actor=m1+main-1788940932-18533-7fa6c2 targets=plan-scanner-reads-fenced-code reason=duplicate: open-work-scan-reads-fenced-examples and plan-fields-outside-fences already carry this defect; opened 2026-09-09 before the backlog was searched
- 2026-09-11T22:04:44Z 2JGP4CP361QCS1JMTG1J49GYWF-m1-c6925449 done actor=human:Wido targets=plan-scanner-reads-fenced-code
Integrity: sha256=c4aa2878167d39c9f4962e44938110625a582cb193fccff4f786507b632e4248
