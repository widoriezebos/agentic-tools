# fixture-suite-drift-after-approval-gate

- State: queued
- Tier: 1
- Intent: Four fixture scripts are red on main independent of the tiering work, found by part one's re-review (records/misc/severity-tiered-rigor-build1-critique-cc2.md F-12 to F-16): channel-fixtures.sh claims goals with a budget tuple and --approved-ref, which the claim verb refuses since the execution-approval gate landed (c285d5a0); dispatch-fixtures.sh dies at its tuple-bearing claim and then at config tailor calls without --runtimes, and its steward-continuation leg times out on census freshness under load; the channel poll verb has a fixed 15-second context; adopt-fixtures.sh's vendored receipt leg depends on state-root resolution; supervision-fixtures.sh's stop-hook-monitor assertion expects the enrollment path. DONE means each script runs green on main on a Mac, with the claim calls rewritten as approve-then-claim and the poll context configurable.
- Origin: main
- Next step: TIER 1 per R-54-m1 (fixtures and one config value): build, run the five scripts, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-03T22:54:55Z
- Revision: 2
- Labels: robustness

History:
- 2026-09-03T22:54:55Z AZ98VP8YPTFQ144YY4DQE7AACJ-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T06:16:32Z YKAKN002ZNPM72Q4M5A4ANKVM3-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
Integrity: sha256=24e56eaddcceeb635b0588fc6172e33fddcf0f5abf27d68e7ecf12a1c9cf33bf
