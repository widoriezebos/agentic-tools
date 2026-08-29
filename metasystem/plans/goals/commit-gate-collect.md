# commit-gate-collect

- State: claimed
- Intent: Ruling P long tail: commit.sh's static re-proof runs gofmt/vet/staticcheck/build serially and halts at the first red tool - collect all four tools' verdicts per run; LOW payback (multi-tool reds are rare; each tool already batches internally), outside the current program horizon per the payback law
- Origin: human
- Next step: Appetite: 1h — run all four tools, report every red in one block, exit nonzero on any; no check weakens
- OpenedAt: 2026-08-29T05:45:15Z
- Revision: 3
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-29T19:33:25Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-29T05:45:15Z QR17F3WE5QS18YEV49FP3N07SE-m1-bf243850 open actor=human:wido targets=commit-gate-collect
- 2026-08-29T19:33:11Z GNN4XWVSH8SYMNWHE83QKH08XK-m2-bc1be9cb set-budget actor=human:wido targets=commit-gate-collect
- 2026-08-29T19:33:25Z 3BY0K88DSVMYPKSDVY9X6RBWWN-m2-bc1be9cb claim actor=m2+mac-coordinator targets=commit-gate-collect
Integrity: sha256=33c6aa5b52646bda26aeef06a127ed2615bbf59f9a2835dfb626706a990daf5e
