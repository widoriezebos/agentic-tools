# codex-handshake-budget-load-fragile

- State: queued
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: Small item (4h). Ladder per R-38-m2: design (one page: replace the fixed 10-second handshake cap with liveness-based patience - wait while the child process is alive and the launch is younger than a generous hang bound taken from the snapshot, fail only on exit-without-session or the hang bound; keep the recorded field honest; state what m0b's VM measured for comparison) -> Sol critique -> Sol build -> Fable code critique -> land with --chain. Until it lands, Codex work on m1 waits for a quieter machine or for Wido's word on a direct fix; m1's leaked fixture processes (a dozen agent-fixture steward runners from four days ago under /private/var/folders) are suspected load and belong to goal proof-harness-process-custody.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T12:14:59Z N4P2PM3GVJYH8GZAWQ9DEXXGV7-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T12:15:13Z JWXKVACXE2XHF69TZVBR2Y616P-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
Integrity: sha256=f27ad611db43bc2c3c9a47951bd3d4dd84f966f2d18ac0bd232624be1afc2bb0
