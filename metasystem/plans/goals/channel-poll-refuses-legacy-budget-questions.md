# channel-poll-refuses-legacy-budget-questions

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="The fleet channel is unread on any machine holding a budget question from before the tuple until this lands; the fix moves one validation from load to open; every channel-hooked machine is exposed; nothing accumulates once it lands."
- Tier: 2
- Intent: Since the budget-answer landing (3615da7a), internal/channel/question.go validates that every budget-above-norm question carries a complete five-limit budget, and it does so when loading the question records, so channel poll on m2 fails at once with 'a budget-above-norm question requires a complete proposed budget tuple' because two closed questions from earlier on 2026-09-04 predate the tuple. The fleet channel is unread until this is fixed. DONE means the tuple is required only when a question is opened; loading tolerates a budget question without one (closed or open; it simply cannot raise a box), and a test pins that a legacy record loads.
- Origin: main
- Next step: One validation moved and one test: build, go test ./internal/channel/..., land through a chain. First in line because the channel is unread.
- OpenedAt: 2026-09-04T16:40:08Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-04T16:40:08Z J7JRA4V33FTVHSYMBSF4J5N6W1-m2-5fcf08ab open actor=human:Wido targets=channel-poll-refuses-legacy-budget-questions
Integrity: sha256=bf436fb1b63e39bd7f5812aa640bb922df8e7ba1f5ee9b5f5a69d716d16880a4
