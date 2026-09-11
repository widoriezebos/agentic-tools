# seat-opened-goals-name-their-blocker

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused open is re-issued with its blocker; novelty 1: dependency edges exist; exposure 2: seat opens only; accumulation 1: one verb"
- Tier: 2
- Intent: Seats opened 165 goals in five days against 62 concluded, growing the backlog by 117 (ledger.md section 5 of the delivery deep dive). DONE means: goal open from a seat (origin main) requires --blocks naming the claimed goal it unblocks, records the dependency edge, and refuses otherwise with the ruling; human-origin opens are unchanged; docs/orchestration.md carries the ruling that a seat opens only the defect that blocks its current goal and every other goal is opened by a person. The ruling is Wido's to confirm on the design page; the seat builds it as stated unless he names another. Goal 6 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read OpenTiered in internal/goal/verbs.go and the open flags in cmd/metasystem/goalsync_mutations.go, add the flag, the edge and the refusal, the docs line, land.
- OpenedAt: 2026-09-11T15:45:05Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:05Z 5W0FM0WGJ00Q1A7X06ESJFVC4X-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:47:14Z X1PRNWZEXP7ZK7V0Q8NE34S5BY-m1-c6925449 set-pin actor=human:Wido targets=seat-opened-goals-name-their-blocker
Integrity: sha256=b397c7336b1764bf1725c788de4580eec44c523f3ba0a504239e3b6b6322aacf
