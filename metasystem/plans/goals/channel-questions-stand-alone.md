# channel-questions-stand-alone

- State: approved
- Priority: 1
- Sequence: 3
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Human-facing message shape on an existing channel; reply binding must keep working, nothing else changes"
- Tier: 2
- Intent: Every question for a person arrives in the channel (Telegram, Slack) as its own short message that stands out and is easy to answer (Wido 2026-10-03: 'the telegram messages are very large still, which is not ok if they contain a question that is not related to most of the text. I want questions to be on their own (explaining what is asked) but every question should be a separate message to make them stand out (maybe even with an emoticon) so they can easily be spotted and replied to'). A question message holds only that question: a marker (for example a question emoji), the goal in plain words, what is asked, the choices, and how to reply; background, facts and evidence move to a follow-up message or a link, never ahead of the question. Two questions are never merged into one message, and a status or report message never carries a question.
- Origin: main
- Next step: Read how questions and reports are composed and sent (internal/channel/question.go, report.go, targeted.go, telegram/); collect real examples from today's channel; short tier-2 design: the one question-message shape with a length budget, where the background goes, how replies still bind to the question (thread or reply id); Astra critique; build; send a test question and check it on a phone. ADDED (Wido 2026-10-03): every channel message, question or notice, names the seat that sent it (for example 'm1h asks:') so the person always knows who is asking.
- OpenedAt: 2026-10-03T06:27:40Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T06:27:50Z revision=2 opid=N9QB2610BSMZG7GE50WRV1G6TD-m1e-718ba0eb authority=proven digest=06d931accb5825f233e2ca7a7d9cad029b95e0b6b89ac2d35a3c41f7127bb3c8 episode=2

History:
- 2026-10-03T06:27:40Z SJ01MJW34APWTQT62KBT6JGKHB-m1e-718ba0eb open actor=human:Wido targets=channel-questions-stand-alone
- 2026-10-03T06:27:50Z N9QB2610BSMZG7GE50WRV1G6TD-m1e-718ba0eb approve actor=human:Wido targets=channel-questions-stand-alone
- 2026-10-03T06:27:59Z R0V8D73MWHK7YMTG4T5M6HT6WC-m1e-718ba0eb set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,channel-questions-stand-alone,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-panel-ux,machinery-blocks-of-2026-10-02,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,overrides-state-their-impact,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=channel-questions-stand-alone from=unranked to=1:3 requested-sequence=3
- 2026-10-03T07:09:54Z Q2VYA9EJC65JSTY872G5YXPHB6-m1e-718ba0eb edit actor=human:Wido targets=channel-questions-stand-alone
Integrity: sha256=4b20c3265d2e545cf0795090712416aa7233c208f21a4d38d3c99fc98336af40
