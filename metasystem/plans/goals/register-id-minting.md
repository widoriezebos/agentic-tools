# register-id-minting

- State: queued
- Intent: Two machines minted the same ruling id (R-15) in one hour - append-only registers need collision-free identifiers: reserved per-machine ranges or id-minting through a verb; the union-merge attribute (landed 9ae1696) removes only the textual half
- Origin: main
- Next step: Appetite: 1h — pick the mechanism (a goal-verb-style opid suffix, per-machine ranges, or a mint verb reading the register under the goal lock), migrate nothing retroactively, document in the register header
- OpenedAt: 2026-08-29T15:50:10Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1

History:
- 2026-08-29T15:50:10Z 9CC40599A8D6G7E4X86KBF80Y7-m1-bf243850 open actor=m1+coordinator targets=register-id-minting
- 2026-08-30T00:12:00Z XFMJTV0EKK9JVQ6XB1QQ4TQPCZ-m2-bc1be9cb set-budget actor=human:wido targets=register-id-minting
Integrity: sha256=afa0e9ff7af1f52707bbf1194d8d93acdfef3ab9da9cf8a3d84de289b4ddba8e
