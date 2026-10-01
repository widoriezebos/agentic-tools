# ask-what-happened-follow-ups

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Three small interface corrections behind existing tests and a walkthrough; nothing recorded or written outside the page's state (severity 1, novelty 1, exposure 1, accumulation 1)."
- Tier: 1
- Intent: What: Three small fixes to "ask what happened", where any error on screen opens a conversation with the Partner about what went wrong. Why: First, when the Partner is busy, the waiting question is kept only in page memory, so a reload loses it even though its line stays on screen. Second, the chip on a notification-bell row names the page's section rather than the notification itself. Third, the Ask button on the failure pop-up is proved only by unit tests, never in a real browser. Pros: No lost questions, chips that say what happened, and browser proof of the Ask button. Cons: About an hour of work on cases that have not hurt yet; a first attempt on 30 September stopped at its budget limit with no output, so the next run should start with a smaller first step or a wider budget.
- Origin: human
- Next step: Next: Keep a waiting question where a reload finds it (the room's mark or the conversation's drafts), give the bell chip the notification's own words, and add the failure pop-up to the browser walkthrough. Done when: after a reload a waiting question is still there, ready to send; the bell chip shows the notification's text; and the browser walkthrough presses Ask on a failure pop-up and a conversation opens.
- OpenedAt: 2026-09-29T04:53:39Z
- Revision: 9
- Labels: partner, ui
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-30T18:56:36Z revision=9 opid=J2YDVV2YQ5JC8R14GGZFEPYAZZ-ui-bc2fda53 authority=proven digest=5ddbd3b1eb0137a66ed00d78b171aec33a9973fb4973b841b3f3b16913fd4482 episode=9

History:
- 2026-09-29T04:53:39Z QT1XCW5B3XB4D94Y71MDWHT547-ui-966d857e open actor=human:Wido targets=ask-what-happened-follow-ups
- 2026-09-29T04:54:51Z X51NVX7KA514QFZ66PCNKC9J63-ui-966d857e approve actor=human:Wido targets=ask-what-happened-follow-ups
- 2026-09-30T11:39:09Z 2B5N7BNNCXHAQXHX9MZCK749K6-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=ask-what-happened-follow-ups
- 2026-09-30T13:09:29Z GQ8MR4WHGG5TY3BQZMAC7CXPCZ-ui-43182c96 breach-stop actor=ui+goal-stop-custodian targets=ask-what-happened-follow-ups
- 2026-09-30T14:56:00Z ZQZ2WYEP9T2V160WV7NX68T15F-landing-3164cf85 resume actor=human:Wido targets=ask-what-happened-follow-ups
- 2026-09-30T14:56:14Z TPBFNPSC6ABQADRHP7S2W7NA8E-landing-3164cf85 release actor=human:Wido targets=ask-what-happened-follow-ups displaced=ui+main-1790664507-87928-61899b@2026-09-30T14:56:00Z reason=stranded by a failed steward launch
- 2026-09-30T18:56:27Z 245J0EB1KQQ75EV0A84VQ690XW-ui-bc2fda53 unapprove actor=human:Wido targets=ask-what-happened-follow-ups reason=the backlog clean-up of 2026-09-30 rewrites the intent in plain English; approved again with the same box
- 2026-09-30T18:56:31Z AVWWMN8EH8KK37XCZF5YZ5JE6Z-ui-bc2fda53 edit actor=human:Wido targets=ask-what-happened-follow-ups
- 2026-09-30T18:56:36Z J2YDVV2YQ5JC8R14GGZFEPYAZZ-ui-bc2fda53 approve actor=human:Wido targets=ask-what-happened-follow-ups
Integrity: sha256=bd90787f59ed8ccca6795b66d9cc15c2ae250c3aa62c4b8ddecb2b6c1b3ee197
