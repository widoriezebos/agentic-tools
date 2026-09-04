# fixture-suite-drift-after-approval-gate

- State: claimed
- Tier: 1
- Intent: Four fixture scripts are red on main independent of the tiering work, found by part one's re-review (records/misc/severity-tiered-rigor-build1-critique-cc2.md F-12 to F-16): channel-fixtures.sh claims goals with a budget tuple and --approved-ref, which the claim verb refuses since the execution-approval gate landed (c285d5a0); dispatch-fixtures.sh dies at its tuple-bearing claim and then at config tailor calls without --runtimes, and its steward-continuation leg times out on census freshness under load; the channel poll verb has a fixed 15-second context; adopt-fixtures.sh's vendored receipt leg depends on state-root resolution; supervision-fixtures.sh's stop-hook-monitor assertion expects the enrollment path. DONE means each script runs green on main on a Mac, with the claim calls rewritten as approve-then-claim and the poll context configurable.
- Origin: main
- Next step: TIER 1 per R-54-m1 (fixtures and one config value): build, run the five scripts, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-03T22:54:55Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T06:16:57Z revision=3 opid=Y555HD4KRQXEX0DJZ01ZP5GSGA-m2-5fcf08ab authority=relayed digest=e945251022918379940a32cf630cf623d9672bdca0c55873e8def9099b719db9 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=4 at=2026-09-04T11:11:04Z
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T11:10:49Z revision=4
- StopCapability: generation=4 revision=4 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-03T22:54:55Z AZ98VP8YPTFQ144YY4DQE7AACJ-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T06:16:32Z YKAKN002ZNPM72Q4M5A4ANKVM3-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T06:16:57Z Y555HD4KRQXEX0DJZ01ZP5GSGA-m2-5fcf08ab approve actor=human:Wido targets=fixture-suite-drift-after-approval-gate authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
- 2026-09-04T11:10:49Z J20WYKKFNRQ5GNT0E6PSNHEM9J-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
- 2026-09-04T11:11:04Z GVFH5GSPRGE76RVRYHF8FM1KVS-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-after-approval-gate
Integrity: sha256=f6617e55d80148c005054299ef2ec7b27e3e5584c03f7f91331b6df18ecdd32f
