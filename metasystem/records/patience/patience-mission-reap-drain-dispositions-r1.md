# Dispositions — patience-mission-reap-drain, critique round 1

Critic: design-critic-20260811t103819z-606d (codex gpt-5.6-sol, xhigh).
Verdict: 11 material. All 11 ACCEPTED; design amended in the same commit.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| 0 | high | Drain-stalled recovery not executable as specified in resumeState | ACCEPT — replaced with an explicit run-loop entry mode (normal / heal / drain-resume), decided once at startup. |
| 1 | high | Resumed cycle has no launch-handshake contract | ACCEPT — drain-resume launches no host; the start signal reports the mode; the launch handshake applies only to turns that launch hosts. |
| 2 | high | Park-time booking contradicts same-cycle conclusion | ACCEPT — exactly-once rule: the park writes no cycle line; the block is written at the eventual conclusion with the drain annotation. |
| 3 | high | Claim lifecycle and crash stages undefined | ACCEPT — created by the park state write, stripped by the same conclude write that books the cycle (both proposal builders); a stale claim is refused loudly at entry. |
| 4 | high | Faulted turns cannot be reconstructed from the claim | ACCEPT — the claim records concludePath (accepted / faulted) plus the fault detail; resumption re-runs the recorded continuation. |
| 5 | high | Mid-park answers had no policy | ACCEPT — safe by construction during the park (no turn in flight; conclusions read asks fresh); the separate mid-TURN race stays routed to its own fix. |
| 6 | high | Eligibility proved but terminal verdicts unmapped | ACCEPT — fixed mapping at the standing reaper's precedent: budget first (timeout/budget-cap), husk (failed/abandoned-setup), dead custodian (failed/process-lost); nothing else. |
| 7 | medium | reap-facts has handshakeWaiting, not an expiry fact | ACCEPT — the design names the shipped fields; the runner computes expiry from the record's deadline plus the backstop grace; false waiting proves nothing. |
| 8 | medium | Deadline underdetermined when the active set changes | ACCEPT — recomputed each pass over the current set; mid-drain follow-ups lawfully extend; the park condition uses the same pass's deadline. |
| 9 | medium | "Standing grace" had no owner | ACCEPT — no new grace exists: the handshake grace is the dispatch backstop's constant, the setup grace the standing reaper's; the design names which applies where. |
| 10 | medium | No heartbeat inside the wait | ACCEPT — every drain pass beats the existing runner heartbeat; a lawful long drain reads as a live runner. |
