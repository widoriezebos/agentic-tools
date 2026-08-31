# vm-epoch-identity-drift

- State: queued
- Intent: Process identity comparison fails on a one-second disagreement between recorded start times and fresh probes, failing the census and blocking dispatch admission while the processes are provably alive. Observed by m0 (Debian VM guest, aarch64) 2026-08-31 during a supervision re-arm; healed locally but the defect stands. EVIDENCE (verbatim): dispatch refused 'last census verdict is CENSUS-FAILED'; up refused 'the recorded owner identity is not live' while proc probe returned liveness alive; session announcement recorded pidStartedAt=1788178136 vs three consecutive 'proc started-at' reads of 1788178135 (exact micro 1788178135590000); owner record 1788178790 vs probe 1788178789 (micro 1788178789170000). In BOTH pairs the recorded epoch is exactly +1s over every fresh probe while pidStartTicks (139363017) and bootId were IDENTICAL between record and probe. DIAGNOSIS, not fact: the boot-epoch reference shifted -1s between enrollment and later probes, suspected NTP discipline on the virtualized guest; chrony logs unchecked, prober clock source unverified, macOS unchecked. OPEN QUESTION for the design lane, deliberately unchosen: tolerance band vs one canonical time source for write and probe vs stricter recorded resolution; (pid, startTicks, bootId) was stable throughout and the epoch also rides mainId lineage, coupling any fix to same-process-succession. SECOND SIGHTING of the start-time-identity class after KI-24's split-identity (fixed 2026-08-07); repetition earns a mechanism. Ruling R applies: the fixer runs every caller of the comparison.
- Origin: main
- Next step: Design leg in the Fable lane answering the open question (appetite 2h), then one implementation slice with a synthetic clock-shift fixture (appetite 4h). Budget tuple is Wido's word at claim. Not currently blocking any machine; m0's workaround was a lawful owner shutdown plus re-arm
- OpenedAt: 2026-08-31T19:08:52Z
- Revision: 1

History:
- 2026-08-31T19:08:52Z XDNTREHCEDRG7S7TF62T9VWMEF-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
Integrity: sha256=fb0c523b5b3e2d3e3f5fb3a76a88aab4a9f3be0e6b46c11aca7dfe3a0ed25291
