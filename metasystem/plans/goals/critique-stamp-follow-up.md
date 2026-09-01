# critique-stamp-follow-up

- State: queued
- Intent: The closure gate refuses a chain whose independent critique references a non-terminal work round (correct), but a critique FOLLOW-UP round cannot carry --reviews, so every multi-round chain ends with a mandatory extra stamp-correction dispatch: a fresh critic job re-confirming what the follow-up already verified, purely to move the stamp. Twice on 2026-08-31/09-01 (mr-flake chain, fixture-quartet chain), ~120 reserved job-minutes each.
- Origin: main
- Next step: Appetite: 2h. Either delegate --follow-up accepts --reviews to restamp the chain onto the terminal round (with the gate verifying the follow-up actually examined that round's review object), or the closure gate accepts a critique chain whose LATEST round provably reviewed the terminal work round even when the root reviewed an earlier one. Evidence: the two stamp-correction dispatches (mr-stamp-crit, quartet-stamp-crit) and their identical confirm-everything returns.
- OpenedAt: 2026-08-31T22:07:07Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T22:07:07Z 6PKBR16H5Q7TWRACV1DXQHZAYV-m2-bc1be9cb open actor=m2+mac-coordinator targets=critique-stamp-follow-up
- 2026-09-01T20:26:35Z QHENQKX2MP7H0T1RBSECEV72N8-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=critique-stamp-follow-up
Integrity: sha256=ac431df6c8dbd40b168e4122f9b892ca368738309ca9c35356be7955262094fd
