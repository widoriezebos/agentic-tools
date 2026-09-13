# seat-work-continues-past-the-runtime-stop-cap

- State: queued
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a runaway continuation burns tokens but lands nothing destructive, and the spend fence already caps the day; novelty 3: no mechanism exists for continuing a seat past its host's cap, and the safe shape is the question; exposure 3: every seat on every runtime; accumulation 2: it touches the steward, the adapters and the stop path that three landings changed today"
- Tier: 3
- Intent: A seat keeps working while claimable work remains, even past the host runtime's own stop cap. Wido, 2026-09-13: 'do not stop work unless no goals can be claimed and all work is done', chosen as option 2 beside the console fix ('option 1 now, and option 2 opened as its own goal rather than dropped'). Goal stop-refusal-fits-on-one-screen closes the two allowances inside the hook (the third refusal on an unchanged backlog and the delegate-job exemption), but this runtime stops honouring a Stop hook after a fixed number of consecutive no-progress blocks, and nothing inside the hook can raise that; the same limit exists per runtime and is unobserved on codex and devin. DONE means: (1) the metasystem, not the agent, continues a seat's work when its session ends or its host stops honouring the block, through the Go steward and the adapter contract, never a scheduler inside one runtime; (2) the continuation is bounded and visible: the spend fence governs it, a person can stop it with one command, and every continuation is recorded where the human reads it; (3) a seat that has nothing lawful to do (every remaining goal needs a human word) does not spin: it records what it waits for and goes quiet; (4) proven on at least two runtimes, and on this one by a seat that reaches the host cap and still has its work continued.
- Origin: human
- Next step: Design first (Codex Astra), because the mechanism is a policy and safety question, not a patch: what continues the work (the steward launching a fresh session, a run record, or an adapter-side resume), what bounds it (the spend fence, a maximum, a human off-switch), what it must never do (restart into a loop, run without records, or hide the fact from the human), and how the per-runtime cap is observed rather than assumed. Read plans/stop-message-fits-one-screen-design.md (the console fix and the closed allowances), plans/stop-infrastructure-allows-the-seat-to-stop-design.md, the turn verdict's idle path and the steward's continuation intent, and the wait verb that already blocks a seat inside a tool call. Then Codex Sol reads the design, Codex Sol builds in slices, Opus reads each build.
- OpenedAt: 2026-09-13T17:29:59Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-13T17:29:59Z GYH8H8V7YEVQ9ZFY8MTTYB5QA8-m1e-c6925449 open actor=human:Wido targets=seat-work-continues-past-the-runtime-stop-cap
Integrity: sha256=aa701cd16e219fdc3f37689e87b21b8cda58670e99e198aff7f3680f3c582a53
