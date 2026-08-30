# register-id-minting

- State: claimed
- Intent: Two machines minted the same ruling id (R-15) in one hour - append-only registers need collision-free identifiers: reserved per-machine ranges or id-minting through a verb; the union-merge attribute (landed 9ae1696) removes only the textual half
- Origin: main
- Next step: Appetite: 1h — pick the mechanism (a goal-verb-style opid suffix, per-machine ranges, or a mint verb reading the register under the goal lock), migrate nothing retroactively, document in the register header
- OpenedAt: 2026-08-29T15:50:10Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T00:12:15Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-29T15:50:10Z 9CC40599A8D6G7E4X86KBF80Y7-m1-bf243850 open actor=m1+coordinator targets=register-id-minting
- 2026-08-30T00:12:00Z XFMJTV0EKK9JVQ6XB1QQ4TQPCZ-m2-bc1be9cb set-budget actor=human:wido targets=register-id-minting
- 2026-08-30T00:12:15Z 8HER8XGW2HN9B6Z5FZNWFP0D59-m2-bc1be9cb claim actor=m2+mac-coordinator targets=register-id-minting
Integrity: sha256=153e163afcea380ad91188a002dbffd380aaed082f6eadfd09477b424c4b5d6d
