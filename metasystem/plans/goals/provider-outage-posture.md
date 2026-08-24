# provider-outage-posture

- State: queued
- Intent: A model-provider outage (529/overloaded) stalls the coordinator's brain but must never damage, mislead, or silence the metasystem: local machinery keeps working, the patience clocks stop counting, and a long outage reaches the human without the provider (Wido, 2026-08-24, after a live 529 halted everything)
- Origin: human
- Next step: Appetite: 1d SLICE ONE — outage truth and paused clocks. The supervision runner classifies provider-error exits (529/overloaded/5xx) and records a standing outage mark (consecutive failures, last error class, since-when) instead of hammering; retries back off with jitter; missed watchdog firings coalesce into one resume naming the gap. The steward reads the mark: during a standing outage a tick records the outage plainly in the narration ('the model provider is overloaded; local work continues; the clocks are paused'), and TicksSinceAdvance and DryRevivals DO NOT advance — outage time is never staleness, so revivals stop burning on workers that were never sick. Fixtures: a simulated outage mark freezes both counters and unfreezes cleanly; the narration line; the coalesced resume. SLICE TWO (tokened later): the provider-independent human alert past a threshold (local notification or webhook — a channel that does not ride the API), and the far rung recorded for someday: coordinator-seat failover to another runtime, the critic-bridge precedent generalized. What already coped, recorded as the design's foundation: records survived, local work continued, recovery was resumption not repair — the outage posture builds on crash-only ground that held.
- OpenedAt: 2026-08-24T05:59:11Z
- Revision: 1

History:
- 2026-08-24T05:59:11Z 2RJ9PF936ZPEWYD25Q0D93E565-m1-bf243850 open actor=human:wido targets=provider-outage-posture
Integrity: sha256=10033bf96a5ae8286b4adb8bc13b8d7b648745c1cf08ba53155c88e7b399d4d0
