# register-id-minting

- State: done
- Intent: Two machines minted the same ruling id (R-15) in one hour - append-only registers need collision-free identifiers: reserved per-machine ranges or id-minting through a verb; the union-merge attribute (landed 9ae1696) removes only the textual half
- Origin: main
- Next step: Appetite: 1h — pick the mechanism (a goal-verb-style opid suffix, per-machine ranges, or a mint verb reading the register under the goal lock), migrate nothing retroactively, document in the register header
- Concluded: Landed 7751358: machine-suffixed ruling ids (R-<n>-<machine>) documented in the register header, enforced mechanically at the landing driver (a staged new bare R-<n> refuses; both grammar paths probed). Nothing migrated retroactively per the goal's own instruction; existing ids incl. R-20b stand as minted.
- OpenedAt: 2026-08-29T15:50:10Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1

History:
- 2026-08-29T15:50:10Z 9CC40599A8D6G7E4X86KBF80Y7-m1-bf243850 open actor=m1+coordinator targets=register-id-minting
- 2026-08-30T00:12:00Z XFMJTV0EKK9JVQ6XB1QQ4TQPCZ-m2-bc1be9cb set-budget actor=human:wido targets=register-id-minting
- 2026-08-30T00:12:15Z 8HER8XGW2HN9B6Z5FZNWFP0D59-m2-bc1be9cb claim actor=m2+mac-coordinator targets=register-id-minting
- 2026-08-30T00:14:03Z K893AR4D7T883XA0CWEZ07AQR2-m2-bc1be9cb done actor=human:wido targets=register-id-minting
Integrity: sha256=59133470475fe185fe0cc292b2f58c46183f8f7a88d7b6a6aaef29309c882859
