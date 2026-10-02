# blocked-agent-asks-the-human

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Decides whether unattended agents stop safely and get unblocked (severity 2); builds on existing question and channel verbs (novelty 1); every seat and the lane (exposure 2); every block of every run (accumulation 2)"
- Tier: 2
- Intent: Every agent (seat agents, the lane agent, delegates) that hits a block it cannot resolve in one obvious step asks the human instead of fighting or spinning, and the human unblocks it quickly from Telegram or the UI (Wido 2026-10-02: rely on the human to get in the loop easily; no perfection). Step 1: the ask-on-block rule in every agent skill and brief incl. the landing skill (refusal line 1, what it tried, the decision needed) via metasystem question ask; the agent waits cheaply on question wait (no turn burn, no Stop-hook loop); the answer is a decision, the human doing the act, or a bypass; the steward asks on an agent's behalf when a seat or the lane is silent without progress (the lane sooner, it blocks every seat); every unblock is logged (question, answer, time) as the backlog of blocks to fix by frequency. Step 2: the UI shows and answers questions per seat and carries free-form human messages to a seat's agent.
- Origin: main
- Next step: Inventory what question ask/answer/wait, the Telegram channel, the UI and the peer-message bridge already do; design step 1; critique; build
- OpenedAt: 2026-10-02T12:24:05Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-02T12:24:05Z FV4HRFVE842X06TWH326PZXQHJ-m1e-9c612d71 open actor=human:Wido targets=blocked-agent-asks-the-human
Integrity: sha256=650b8fe879d3b6979cb1081c3634ccf2fea7e2516f157c7f288b9f940a97af1e
