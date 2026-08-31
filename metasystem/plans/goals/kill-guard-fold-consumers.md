# kill-guard-fold-consumers

- State: claimed
- Intent: m2's janitor fold change (dab1dbd) made an empty member scan INDETERMINATE instead of NOT-OWNED: kill-guard consumers now see exit 3 (fail-closed) where they saw 1 - every m1-side consumer must be verified against the new contract (Ruling R: a changed contract runs its callers)
- Origin: main
- Next step: Released untouched by m0 (2026-08-31, Wido's stop order): claimed but no work performed - no round dispatched, no attempt spent. The verification task stands exactly as m2 recorded it: every consumer of the kill-guard must be checked against dab1dbd's changed contract (empty member scan now INDETERMINATE/exit 3, fail-closed, where it was NOT-OWNED/exit 1).
- OpenedAt: 2026-08-30T14:57:35Z
- Revision: 4
- Budget: elapsedLimit=3d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T18:18:28Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T14:57:35Z BHNDDTA3DWP99EGG7G2BB41N5H-m1-bf243850 open actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-30T15:17:24Z 91F0NGJS9FB66ZBNY0JE2DAADH-m1-bf243850 set-budget actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-31T18:18:28Z 3R4SDS7451VB5ZFEZJ1ZA2ZMJJ-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=kill-guard-fold-consumers
- 2026-08-31T18:26:01Z FK4VD8XDK2E3JB6CEN2PBP8N6Y-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=kill-guard-fold-consumers
Integrity: sha256=4758b9fafb9c6b3e71dd66917c48d09da10c33dfea4ea3baa7e267f12f0e8545
