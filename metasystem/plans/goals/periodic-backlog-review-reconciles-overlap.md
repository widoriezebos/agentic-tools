# periodic-backlog-review-reconciles-overlap

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Severity 2: missed duplicates waste budget and mistaken reconciliation can disrupt approved work; novelty 2: a recurring cross-goal semantic review is not an assigned live duty yet; exposure 3: the whole shared backlog and fleet; accumulation 2: duplicate records and unresolved reviews compound."
- Tier: 3
- Intent: What: On a regular schedule, the system reviews the whole backlog for goals that duplicate or overlap each other or have been overtaken, and presents the person and the seats with a proposed resolution backed by evidence (merge, hand over, or keep separate). Why: On 2026-09-10 two seats worked on the same repair under two different goal names, and nothing noticed; stale and duplicate goals also pile up, as the manual backlog clean-up of 2026-09-30 showed. Pros: Duplicate work is caught early and the backlog stays current without a big manual clean-up. Cons: A review that proposes too much becomes noise, and it must never change a goal without the normal approval.
- Origin: human
- Next step: Next: Write a short design that uses the 2026-09-30 manual backlog clean-up as the worked example: what the review compares, which existing role runs it and how often, and how its proposals reach the person. Done when: A design page in plans/designs is accepted by a person and names the owning role and the schedule.
- OpenedAt: 2026-09-10T08:21:09Z
- Revision: 2
- Labels: headless-fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:21:09Z YAJZA8Y2VKRR674WQXFTFN0KA7-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=periodic-backlog-review-reconciles-overlap
- 2026-09-30T18:52:10Z KETRSNTPHXE1CZ5HSNQ1G87GBP-ui-bc2fda53 edit actor=human:Wido targets=periodic-backlog-review-reconciles-overlap
Integrity: sha256=a2f82f6c6ecdec4e933e5ef6e0b2ab93cf5974ccc28ee6c1ad580204a234efcc
