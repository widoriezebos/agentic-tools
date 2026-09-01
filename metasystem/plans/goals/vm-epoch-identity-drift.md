# vm-epoch-identity-drift

- State: queued
- Intent: Process identity comparison fails on a one-second disagreement between recorded start times and fresh probes, failing the census and blocking dispatch admission while the processes are provably alive. Observed by m0 (Debian VM guest, aarch64) 2026-08-31 during a supervision re-arm; healed locally but the defect stands. EVIDENCE (verbatim): dispatch refused 'last census verdict is CENSUS-FAILED'; up refused 'the recorded owner identity is not live' while proc probe returned liveness alive; session announcement recorded pidStartedAt=1788178136 vs three consecutive 'proc started-at' reads of 1788178135 (exact micro 1788178135590000); owner record 1788178790 vs probe 1788178789 (micro 1788178789170000). In BOTH pairs the recorded epoch is exactly +1s over every fresh probe while pidStartTicks (139363017) and bootId were IDENTICAL between record and probe. DIAGNOSIS, not fact: the boot-epoch reference shifted -1s between enrollment and later probes, suspected NTP discipline on the virtualized guest; chrony logs unchecked, prober clock source unverified, macOS unchecked. OPEN QUESTION for the design lane, deliberately unchosen: tolerance band vs one canonical time source for write and probe vs stricter recorded resolution; (pid, startTicks, bootId) was stable throughout and the epoch also rides mainId lineage, coupling any fix to same-process-succession. SECOND SIGHTING of the start-time-identity class after KI-24's split-identity (fixed 2026-08-07); repetition earns a mechanism. Ruling R applies: the fixer runs every caller of the comparison.
- Origin: main
- Next step: SECOND STRIKE (2026-09-01 ~17:10Z, same machine): recorded owner start 1788202835 vs live probe 1788202834 - exactly -1s again; census CENSUS-FAILED, dispatch admission refused. Healed by the recorded sequence (lawful owner shutdown + re-arm), this time self-served under the R-34-m0 permission approval - downtime minutes, not hours. RECURRENCE CADENCE: twice in ~30 hours on the clock-disciplined guest; the design leg (tolerance band vs canonical time source vs stricter resolution, Fable lane) is now recurring-cost-justified under R-33 and should be claimed soon; the fixer runs every caller of the comparison (Ruling R) and reads missionrunner-terminate-flake, whose identity-proof failures share the signature.
- OpenedAt: 2026-08-31T19:08:52Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T19:08:52Z XDNTREHCEDRG7S7TF62T9VWMEF-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T14:18:10Z CNB1FH512YJJ30JNCQ7RQF0HWK-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=vm-epoch-identity-drift
- 2026-09-01T14:27:15Z 34AQATQ7REJVZYFB62MQ3J0EF6-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=vm-epoch-identity-drift
- 2026-09-01T17:01:41Z EQA9TFFNMBDNMWVYGRHYGJSYJS-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
Integrity: sha256=69d4d96d4d838d790bb88be5deca8c08046191ccd075838d6327293e5c333235
