# channel-questions-stand-alone

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Human-facing message shape on an existing channel; reply binding must keep working, nothing else changes"
- Tier: 2
- Intent: Every question for a person arrives in the channel (Telegram, Slack) as its own short message that stands out and is easy to answer (Wido 2026-10-03: 'the telegram messages are very large still, which is not ok if they contain a question that is not related to most of the text. I want questions to be on their own (explaining what is asked) but every question should be a separate message to make them stand out (maybe even with an emoticon) so they can easily be spotted and replied to'). A question message holds only that question: a marker (for example a question emoji), the goal in plain words, what is asked, the choices, and how to reply; background, facts and evidence move to a follow-up message or a link, never ahead of the question. Two questions are never merged into one message, and a status or report message never carries a question.
- Origin: main
- Next step: Read how questions and reports are composed and sent (internal/channel/question.go, report.go, targeted.go, telegram/); collect real examples from today's channel; short tier-2 design: the one question-message shape with a length budget, where the background goes, how replies still bind to the question (thread or reply id); Astra critique; build; send a test question and check it on a phone.
- OpenedAt: 2026-10-03T06:27:40Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T06:27:50Z revision=2 opid=N9QB2610BSMZG7GE50WRV1G6TD-m1e-718ba0eb authority=proven digest=06d931accb5825f233e2ca7a7d9cad029b95e0b6b89ac2d35a3c41f7127bb3c8 episode=2

History:
- 2026-10-03T06:27:40Z SJ01MJW34APWTQT62KBT6JGKHB-m1e-718ba0eb open actor=human:Wido targets=channel-questions-stand-alone
- 2026-10-03T06:27:50Z N9QB2610BSMZG7GE50WRV1G6TD-m1e-718ba0eb approve actor=human:Wido targets=channel-questions-stand-alone
Integrity: sha256=62f15cc6f655cab1edd39c7cb4733c1ca6dcc1d248c94f4d39dcb43017ea1910
