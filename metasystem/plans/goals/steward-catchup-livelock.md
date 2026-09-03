# steward-catchup-livelock

- State: queued
- Intent: A machine that rejoins after a long absence cannot get its steward alive: the ledger-attention stage replays every ledger commit between its last diffed tip and the accepted tip with a full tree projection each (m3 on 2026-09-03 after 48 hours offline: 405 commits at several seconds each, tens of minutes), while the runner counts as alive only through a completed steward-tick within twice the tick interval; the supervision watcher therefore stops and replaces the runner every minute (attemptSeq climbed past 17, lastCompletion never set), each replacement discards the in-memory stage, and 'metasystem up' ends every time with 'replacement runner did not complete generation N within 10s'. The catch-up is restarted from scratch forever; nothing on this machine is watched. DONE means a rejoining machine's first tick either persists its catch-up incrementally (a partial stage survives a restart and resumes from its last projected tip) or is recognised as live progress by the freshness rule, so that the runner is never replaced while it is making progress, proven by a fixture with a long ledger window and a short tick interval. Cousin of recovery-to-good-state; found by m3 at the fence takeover.
- Origin: main
- Next step: Classify at intake (R-54-m1); reproduce as a failing test in internal/steward (a ledger window longer than the freshness cap, the watcher replacing the runner mid-catch-up); then design the smallest fix (incremental persistence of the ledger-attention stage per projected tip, or a progress-aware freshness rule) and run the ladder for its tier.
- OpenedAt: 2026-09-03T13:35:53Z
- Revision: 1
- Labels: robustness, steward

History:
- 2026-09-03T13:35:53Z PB1RC3TNGD6NF5G1ZWZF9FV2VW-m3-a5da21ff open actor=m3+mac-m3 targets=steward-catchup-livelock
Integrity: sha256=05be5e796541364690a157dd4c1d7f3aa7184d63efb080b69f1a886a6c2c93af
