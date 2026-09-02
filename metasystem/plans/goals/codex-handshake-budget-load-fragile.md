# codex-handshake-budget-load-fragile

- State: queued
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step:  MEASURED 2026-09-02 15:20 on m1 after the machine was healed (fseventsd restarted, 488 leaked fixture processes and 996 stale beds removed, load under 5): codex-cli 0.148.0 still takes 18 seconds from launch to thread.started, so the 10-second cap fails on a healthy m1 too; the cause is Codex's own cold start on this host, not load. The VM seats, where Codex dispatches succeed, are the comparison point for the design. CAUSE FOUND 15:30: with the operator's ten Codex plugins disabled for the run (codex exec -c 'plugins={}') thread.started arrives at 1 second on the same machine; with them, 16 to 18 seconds; disabling MCP servers changes nothing. So the design has two parts: the adapter passes plugins={} for every delegate run (delegates never need the operator's interactive plugins, and the run becomes deterministic across machines), and the handshake cap becomes liveness-based patience so the next slow-start cause does not refuse healthy work either.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T12:14:59Z N4P2PM3GVJYH8GZAWQ9DEXXGV7-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T12:15:13Z JWXKVACXE2XHF69TZVBR2Y616P-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:21:16Z XM6TG83VEP1NCDFEG1WX1EKQR7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:22:10Z XX7F2XMKNVB6SRDSDYF9NP85A9-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:23:30Z NVVHBA80G3HVCM9VVJ8DS8G2D0-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
Integrity: sha256=536f5ed8f2e3c98a86632a07aeec72ece3d08ebd3eb21bff310380fec586b70e
