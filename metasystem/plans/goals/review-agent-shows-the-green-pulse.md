# review-agent-shows-the-green-pulse

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A visual indicator in one UI pane, reusing the Partner's existing busy state and pulse; nothing in the machinery changes"
- Tier: 1
- Intent: The agent that reviews a goal from the Review column shows the same green pulse while it works as the Project Partner does (Wido 2026-10-03 16:16 CEST: 'I just started a review and I see the agent used for the review does not have the green pulse indicator for activity. That needs to be added like it was done for the project partner'). Today the Partner pane pulses while its turn runs and the review sitting's agent shows nothing, so a person cannot tell whether the review is working or stuck.
- Origin: human
- Next step: Find the Partner's activity pulse (internal/ui/web/_app/src/partner: the busy state the store keeps and the pulse it renders) and the review sitting's agent view, and give the review the same indicator from the same state, with no second implementation. Done when: starting a review from the Review column shows the pulse while the agent works and it stops when the agent's turn ends, on the desktop and at phone width; a test covers both states.
- OpenedAt: 2026-10-03T14:16:52Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-03T14:16:52Z 2T5D8NDHX2CFRPSF5DZQWWAMEM-m1e-718ba0eb open actor=human:Wido targets=review-agent-shows-the-green-pulse
Integrity: sha256=9e0b930069cd41c95388444f8f3d0aab610af1d37b1fe36981fff0654902a3bc
