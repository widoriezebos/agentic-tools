# ui-connects-to-a-running-agent

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Human control over running agents (severity 2); a new per-runtime adapter and a live view (novelty 2); every seat and runtime (exposure 2); used when a person looks in (accumulation 1)"
- Tier: 2
- Intent: A person can connect to any running agent from the UI as if opening its terminal (Wido 2026-10-02), generically: for every agent kind (seats, the landing lane agent, delegates) and every runtime (Claude, Codex, others) through one small adapter per runtime, and every UI action is a verb (UI parity). Step 1: watch a running agent live (its session stream: what it reads, runs and decides) and talk to it (a message into its inbox, answered in the same view). Step 2: take over: pause the agent and open its own session interactively in a browser terminal (the runtime's resume), and hand it back so the steward continues from there. No tmux dependence.
- Origin: main
- Next step: Design step 1 (verbs agent watch/say, the per-runtime session-stream adapter, the UI view on the fleet page), Astra critique, build
- OpenedAt: 2026-10-02T21:11:09Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-02T21:11:09Z 71EHXME1ZQTHJR4XME33T8GGG5-m1e-9c612d71 open actor=human:Wido targets=ui-connects-to-a-running-agent
Integrity: sha256=23ba7019f5967d280c7aed2d105cae779ff84de85ca1a03556fe4d495173d248
