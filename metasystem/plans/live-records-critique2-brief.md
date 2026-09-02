Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal digest-landing-race)
Date: 2026-09-02

# Goal

Round-2 critique of metasystem/plans/live-records-landing-design.md
revision 2 (landed, in your worktree), which folded your nine round-1
findings (metasystem/records/misc/live-records-critique-r1.md, landed) into
one mechanism: the checkout-wide reader/writer landing lock. Judge each
fold BY ID, and attack the lock itself: deadlock and starvation between a
long landing and a ticking steward; lock inheritance across land.sh's
subprocesses; a crashed holder's stale lock; the writer-pause honoring the
no-softening byte law; whether the conflict-trail explanation actually
matches the recorded git behavior. A clean return closes the design phase;
the build ships.

# Constraints

Wall-clock budget: 25 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
