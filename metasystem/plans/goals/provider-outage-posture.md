# provider-outage-posture

- State: claimed
- Intent: A model-provider outage (529/overloaded) stalls the coordinator's brain but must never damage, mislead, or silence the metasystem: local machinery keeps working, the patience clocks stop counting, and a long outage reaches the human without the provider (Wido, 2026-08-24, after a live 529 halted everything)
- Origin: human
- Next step: Appetite: 1d SLICE ONE — outage truth and paused clocks, implementation map from reconnaissance: (1) THE MARK — a shared outage recorder (artifacts/agents/outage.json: consecutive failures, last error class, since-when), written when a provider call classifies as overloaded (529/overloaded/5xx), cleared by any provider success; detection points are the host adapters' exit classification (scripts/agents/hosts + the runner's host-exit ramp) and the dispatch delegate launches. (2) THE STEWARD'S CLOCKS — RunTick reads the mark: during a standing outage TicksSinceAdvance and DryRevivals do NOT advance and the narration says plainly 'the model provider is overloaded; local work continues; the clocks are paused'. (3) THE RUNNER'S BREAKER — a provider-overload host exit never counts toward the consecutive-host-failure breaker (it is nobody's failure): backoff and retry instead of a host-failure park; the outage mark rides the turn evidence. (4) OUT OF REPO SCOPE, recorded: the session-level dead-man's-switch firing loop lives in harness configuration, not repo code — its backoff/coalescing is slice-two territory with the provider-independent human alert. Fixtures: a simulated mark freezes both steward counters and unfreezes cleanly; a simulated overloaded host exit retries without feeding the breaker; the narration line. What already coped, the design's foundation: records survived, local work continued, recovery was resumption not repair.
- OpenedAt: 2026-08-24T05:59:11Z
- Revision: 3
- Claimed: machine=m1 lineage=coordinator at=2026-08-24T05:59:16Z

History:
- 2026-08-24T05:59:11Z 2RJ9PF936ZPEWYD25Q0D93E565-m1-bf243850 open actor=human:wido targets=provider-outage-posture
- 2026-08-24T05:59:16Z 40V9KXZ96SBH55P2NCWKV2ZGG2-m1-bf243850 claim actor=m1+coordinator targets=provider-outage-posture
- 2026-08-24T06:02:33Z BY47CNZP5795PVF45DGF5BXD5R-m1-bf243850 edit actor=m1+coordinator targets=provider-outage-posture
Integrity: sha256=1a3336d32efd5cfc2b4796151ec27948eefef4db9910b2199c126785138b95a7
