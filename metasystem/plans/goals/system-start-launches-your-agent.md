# system-start-launches-your-agent

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Changes what system start does for a person at a terminal; an agent running system start must never spawn a nested agent; reverts cleanly with --no-agent or no config key"
- Tier: 2
- Intent: A person starts working in a checkout with one command: metasystem system start arms the machinery and then hands the terminal to the agent of choice (launch.agent in metasystem.conf.local, --agent NAME to override, --no-agent to only arm); the agent replaces the process so the person's shell stays its parent; only a person at a real terminal gets an agent, an agent or script running system start only arms; runtime-generic via the adapters
- Origin: main
- Next step: Write a short design (config key, flags, the person-and-terminal check, exec through the runtime adapter's launch command, how host setup ends with it, relation to the UI seat's unattended seat sessions), one critique round, then build; smallest thing that works
- OpenedAt: 2026-09-30T16:55:50Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T16:55:50Z EWJPHN6H6ZEZSRZ86G6BZ9C4E5-m1e-b6a4eb0a open actor=human:wido targets=system-start-launches-your-agent
Integrity: sha256=1239747f4b5290e27c5aa4e97074aca981fbb038a4bd3adf43d2169fab9ea50e
