# kill-guard-fold-consumers

- State: claimed
- Intent: m2's janitor fold change (dab1dbd) made an empty member scan INDETERMINATE instead of NOT-OWNED: kill-guard consumers now see exit 3 (fail-closed) where they saw 1 - every m1-side consumer must be verified against the new contract (Ruling R: a changed contract runs its callers)
- Origin: main
- Next step: Appetite: 1h. Enumerate every caller consuming janitor fold/kill-guard exit codes on m1 surfaces (steward sweeps, cancel paths, cleanup scripts), verify each handles exit 3 as fail-closed correctly, fix any that treated 1 as the only refusal; prove with the affected fixtures
- OpenedAt: 2026-08-30T14:57:35Z
- Revision: 3
- Budget: elapsedLimit=3d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T18:18:28Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T14:57:35Z BHNDDTA3DWP99EGG7G2BB41N5H-m1-bf243850 open actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-30T15:17:24Z 91F0NGJS9FB66ZBNY0JE2DAADH-m1-bf243850 set-budget actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-31T18:18:28Z 3R4SDS7451VB5ZFEZJ1ZA2ZMJJ-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=kill-guard-fold-consumers
Integrity: sha256=e96cf6ab5916378ddef0858a7b1f6dd23b9248ba0fd34e6bbd11fd5c53fdf9f9
