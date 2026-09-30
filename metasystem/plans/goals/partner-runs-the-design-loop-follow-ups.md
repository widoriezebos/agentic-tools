# partner-runs-the-design-loop-follow-ups

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Three small corrections behind existing tests; nothing new reaches a record or a run (severity 1, novelty 1, exposure 1, accumulation 1)."
- Tier: 1
- Intent: What: Three small fixes left over from running the design loop (critique, fold, hand over) from the review room in the browser. Why: First, a section card's before-and-after comparison shows the Partner's draft as written, so when Use drops an opening heading, the person sees that heading in the comparison but not in the result. Second, a fold request sent before this feature shipped uses the old wording and is silently ignored. Third, the test of how a sitting closes its pending test-only obligations fakes the ledger, while the real close writes to the real one, so the test could pass while the real path breaks. Pros: The comparison matches what lands, old requests are not silently dropped, and the test proves the real path. Cons: About an hour of work on cases nobody has hit yet; the old-wording case may never occur.
- Origin: human
- Next step: Next: Make the three fixes, each with a test that fails first: show the comparison as the section will read after Use, handle or retire the old fold wording, and run the close-out test against a real scratch ledger. Done when: a section card whose draft loses its heading on Use shows the same text in the comparison as in the result, an old-wording fold request is either handled or refused out loud, and the close-out test writes and reads a real ledger.
- OpenedAt: 2026-09-29T07:20:04Z
- Revision: 2
- Labels: partner, ui
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-29T07:20:04Z 9PHSDMMY48D3034WGF8H5Z1HZJ-ui-966d857e open actor=human:Wido targets=partner-runs-the-design-loop-follow-ups
- 2026-09-30T18:51:54Z 9F18JD67W8GF7QXAAY6RQ67BTJ-ui-bc2fda53 edit actor=human:Wido targets=partner-runs-the-design-loop-follow-ups
Integrity: sha256=7e30cc484b35b9b68f09e97ad06ef409880d46995c088c0243bb0c0e6a0f0e2c
