# arming-dead-owner-takeover

- State: queued
- Intent: Supervision arming joins a provably dead owner and reports the failure as a census timeout: with a stale owner record standing (dead since 2026-08-16), plain arm-supervision.sh waited its full census cap and printed 'watcher, reaper, and a fresh successful census did not verify' while the standalone census verdict was SUCCESS — the real cause (dead owner, join instead of takeover) was invisible, and only --rearm recovered (found 2026-08-28 ~02:40 while restoring the lease for custodial critique rounds)
- Origin: main
- Next step: Appetite: 1h. The ordinary arm detects a provably dead recorded owner (exact identity, the same proof the reapers use) and takes over WITHOUT --rearm — a dead owner holds nothing; failing that, the timeout message must name the dead-owner join as the cause. Fixture: stale owner record + plain arm succeeds; live owner + plain arm still joins; --rearm semantics unchanged.
- OpenedAt: 2026-08-28T02:45:08Z
- Revision: 1

History:
- 2026-08-28T02:45:08Z E7XHX80WHY2P45Z2D0C8A9JB99-m1-bf243850 open actor=m1+coordinator targets=arming-dead-owner-takeover
Integrity: sha256=cd0e3fea54d3e9ce394135e5f2137e9e26a6ba91c3fa919a9368d2a71afdb52d
