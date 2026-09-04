# channel-poll-refuses-legacy-budget-questions

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="The fleet channel is unread on any machine holding a budget question from before the tuple until this lands; the fix moves one validation from load to open; every channel-hooked machine is exposed; nothing accumulates once it lands."
- Tier: 2
- Intent: Since the budget-answer landing (3615da7a), internal/channel/question.go validates that every budget-above-norm question carries a complete five-limit budget, and it does so when loading the question records, so channel poll on m2 fails at once with 'a budget-above-norm question requires a complete proposed budget tuple' because two closed questions from earlier on 2026-09-04 predate the tuple. The fleet channel is unread until this is fixed. DONE means the tuple is required only when a question is opened; loading tolerates a budget question without one (closed or open; it simply cannot raise a box), and a test pins that a legacy record loads.
- Origin: main
- Next step: One validation moved and one test: build, go test ./internal/channel/..., land through a chain. First in line because the channel is unread. NOTE m3 2026-09-04 19:35: m3 hit the same refusal after rebuilding to 09678719, so no machine reads Telegram now; m3's bridge message to m2 was held for approval. m3 offers to take this the moment its design critique on fleet-channel-gateway returns (~20:05) unless m2 claims it first; the goal still needs Wido's approval (tier 2), which m3 will relay if he types it at the m3 terminal.
- Concluded: a7959217: validateQuestionBudget runs in Ask alone; ReadQuestion and listQuestions load a budget question without a tuple; a token answer to one records, closes and raises nothing (receipt: no proposed box on this question; nothing raised); TestLegacyBudgetQuestionLoadsWithoutTuple and TestLegacyBudgetQuestionAnswerRaisesNothing pin it; one review (lbq-cc-1), no material finding; channel poll on m3 reads again
- OpenedAt: 2026-09-04T16:40:08Z
- Revision: 6
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T17:30:45Z revision=3 opid=566P77BZNC51VNYBJTHRCRMBVN-m3-a5da21ff authority=relayed digest=b484f02736e830a71482a8d2b9db3f028c9a6622d0ed9fb134dce5b0444a8f6e reviewBy=2026-09-06
- Sliced: machine=m3 lineage=mac-m3 revision=4 at=2026-09-04T17:34:17Z

History:
- 2026-09-04T16:40:08Z J7JRA4V33FTVHSYMBSF4J5N6W1-m2-5fcf08ab open actor=human:Wido targets=channel-poll-refuses-legacy-budget-questions
- 2026-09-04T17:23:08Z 3JWPPZ4PDD9FRTSWF7JSFZYA4F-m3-a5da21ff edit actor=m3+mac-m3 targets=channel-poll-refuses-legacy-budget-questions
- 2026-09-04T17:30:45Z 566P77BZNC51VNYBJTHRCRMBVN-m3-a5da21ff approve actor=human:Wido targets=channel-poll-refuses-legacy-budget-questions authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="approve channel-poll-refuses-legacy-budget-questions for me"
- 2026-09-04T17:32:39Z GXFF7P9DK0QYN8730C9PABR71X-m3-a5da21ff claim actor=m3+mac-m3 targets=channel-poll-refuses-legacy-budget-questions
- 2026-09-04T17:34:17Z 8K0B9XSW0VD2RXHHGWCA79Y1TF-m3-a5da21ff slice-start actor=m3+mac-m3 targets=channel-poll-refuses-legacy-budget-questions
- 2026-09-04T17:51:29Z QZ6CCDD5YK3W4DG3Z9HDS3A2SM-m3-a5da21ff done actor=m3+mac-m3 targets=channel-poll-refuses-legacy-budget-questions
Integrity: sha256=b78a6cf8d0580a9774121b055cd1859143b379f5c7b4883d8a62946272d25aba
