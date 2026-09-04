# channel-budget-answer-binds-nothing

- State: queued
- Tier: 1
- Intent: A budget-above-norm question opened with channel ask carries the proposed tuple (attempt, minute, elapsed, active-job and review-round limits) and, when the human answers with the token verbatim plus a valid code, the question closes as answered, but the goal's box is unchanged: on 2026-09-04 both such answers (question BWSHFBEK27TMT7YKJ80H4NMF18 'Yes', question CF77YSK1TTFRE26C0D9WNN8537 'allow two more rounds') left the goals at their tier box, and goal approve then refused a second relayed approval on the same goal, so the seat had to open an arc-split member goal each time. DONE means a verified channel answer to a budget question is itself the human's approval reference: the poll's disposition re-approves the goal with the question's tuple, recording the message reference and code step as the word, and the goal's admission reopens without a second question or a member goal; a test in internal/channel pins it.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one disposition branch and a test): build, go test ./internal/channel/... ./internal/goal/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T12:03:19Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T12:03:19Z PEG868QZM724RSFR0MG96HBJ15-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-budget-answer-binds-nothing
Integrity: sha256=59c89f69e01b03141ea14386ca957ef783f5c20cecf9d4c34dc635a5a0940a9a
