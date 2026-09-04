# fixture-suite-drift-after-approval-gate

- State: approved
- Tier: 1
- Intent: Four fixture scripts are red on main independent of the tiering work, found by part one's re-review (records/misc/severity-tiered-rigor-build1-critique-cc2.md F-12 to F-16): channel-fixtures.sh claims goals with a budget tuple and --approved-ref, which the claim verb refuses since the execution-approval gate landed (c285d5a0); dispatch-fixtures.sh dies at its tuple-bearing claim and then at config tailor calls without --runtimes, and its steward-continuation leg times out on census freshness under load; the channel poll verb has a fixed 15-second context; adopt-fixtures.sh's vendored receipt leg depends on state-root resolution; supervision-fixtures.sh's stop-hook-monitor assertion expects the enrollment path. DONE means each script runs green on main on a Mac, with the claim calls rewritten as approve-then-claim and the poll context configurable.
- Origin: main
- Next step: TIER 1 per R-54-m1 (fixtures and one config value): build, run the five scripts, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-03T22:54:55Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T06:16:57Z revision=3 opid=Y555HD4KRQXEX0DJZ01ZP5GSGA-m2-5fcf08ab authority=relayed digest=e945251022918379940a32cf630cf623d9672bdca0c55873e8def9099b719db9 reviewBy=2026-09-06

History:
- 2026-09-03T22:54:55Z AZ98VP8YPTFQ144YY4DQE7AACJ-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T06:16:32Z YKAKN002ZNPM72Q4M5A4ANKVM3-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T06:16:57Z Y555HD4KRQXEX0DJZ01ZP5GSGA-m2-5fcf08ab approve actor=human:Wido targets=fixture-suite-drift-after-approval-gate authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
Integrity: sha256=84690579d757760d7b7bfe6cd5a61f5ece85386a6c5eb2fa84c64a099b38e3b4
