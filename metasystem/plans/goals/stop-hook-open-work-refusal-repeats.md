# stop-hook-open-work-refusal-repeats

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the turn end is refused for work the seat cannot do, which costs turns and teaches seats to distrust the refusal, nothing unsafe is permitted; novelty 1: a marker file and two predicates on records that exist; exposure 3: every seat's every turn end while any stale plan exists in the tree; accumulation 1: it does not compound beyond the lost turns"
- Tier: 3
- Intent: The Stop hook's open-work verdict says 'This refusal does not repeat for the same work' and then repeats it: on m1d on 2026-09-06 the same two lines (OPEN-WORK plans/goal-scope-bounds-design.md with its literal '<one line, required>' placeholder, and OPEN-WORK plans/handoff-m1-2026-09-02.md naming a never-idle-analysis job that runs nowhere) refused three turn ends in a row, twice while a delegate job of this checkout was running and once together with a deadline expiry. Both plans are another seat's notes from 2026-09-03 and carry no work this seat can do or lawfully edit; the verdict also does not count a running delegate job as work in flight, so a seat that has dispatched and is waiting is told to 'do it now'. DONE means: the once-only promise holds per plan line across turns (a durable marker, not one that a deadline expiry loses); a running job or an open chain on the checkout counts as in flight for the verdict; and a plan file whose Next step is an unfilled template placeholder is reported as a template defect, not as open work for the seat.
- Origin: main
- Next step: MECHANICAL, one chain: in the turn-verdict path (internal/steward or wherever the open-work scan lives; grep for 'does not repeat for the same work'), persist the refused plan lines in the supervision artifacts keyed by plan path plus line digest and skip them on later stops; treat a job record on this checkout in status running, or a chain root with chainClosed false whose newest round is not terminal, as in flight; classify '<one line, required>' placeholders as template-unfilled with their own line. Fixtures: the supervision-hook fixture suite gains one scenario per rule.
- OpenedAt: 2026-09-06T10:53:06Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T10:53:06Z YP7WG37N4VN1X8VMS5RHV2JWXG-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
Integrity: sha256=f5fc48f9df9224f71e405d4ba0eda189a4bf073ac562455e1a6a15ec1a055d88
