# vm-epoch-identity-drift

- State: queued
- Intent: Process identity comparison fails on a one-second disagreement between recorded start times and fresh probes, failing the census and blocking dispatch admission while the processes are provably alive. Observed by m0 (Debian VM guest, aarch64) 2026-08-31 during a supervision re-arm; healed locally but the defect stands. EVIDENCE (verbatim): dispatch refused 'last census verdict is CENSUS-FAILED'; up refused 'the recorded owner identity is not live' while proc probe returned liveness alive; session announcement recorded pidStartedAt=1788178136 vs three consecutive 'proc started-at' reads of 1788178135 (exact micro 1788178135590000); owner record 1788178790 vs probe 1788178789 (micro 1788178789170000). In BOTH pairs the recorded epoch is exactly +1s over every fresh probe while pidStartTicks (139363017) and bootId were IDENTICAL between record and probe. DIAGNOSIS, not fact: the boot-epoch reference shifted -1s between enrollment and later probes, suspected NTP discipline on the virtualized guest; chrony logs unchecked, prober clock source unverified, macOS unchecked. OPEN QUESTION for the design lane, deliberately unchosen: tolerance band vs one canonical time source for write and probe vs stricter recorded resolution; (pid, startTicks, bootId) was stable throughout and the epoch also rides mainId lineage, coupling any fix to same-process-succession. SECOND SIGHTING of the start-time-identity class after KI-24's split-identity (fixed 2026-08-07); repetition earns a mechanism. Ruling R applies: the fixer runs every caller of the comparison.
- Origin: main
- Next step: AT THE FORK, AWAITING WIDO (2026-09-02): design revision 2 landed (104a2f2d, all 12 round-1 findings folded grounded); Sol round 2 finds 11 material (2 critical - the pairless-legacy-override and the merge-different-processes migration hole), trajectory 12->11 = the third same-signature stall (R-39-m0 minted per R-38-m0's clause). THE FORK: (a) joint round #3 - Sol designs the seams in place and builds the core comparator slice, Fable critiques after; twice-proven escape, needs Wido's per-case word; (b) fence - the chain holds with everything recorded. Budget: ~120m/1 launch left at 720m, one envelope raise to 900m available. The terminate-flake family stays red on m0 (blocking missionrunner-touching landings, incl. the parked fixture fix) until this goal's implementation lands - the cost curve argues for (a). m0 releases the claim so any seat can execute Wido's choice.
- OpenedAt: 2026-08-31T19:08:52Z
- Revision: 10
- Budget: elapsedLimit=2d attemptLimit=8 reservedJobMinutesLimit=720 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=6 at=2026-09-01T21:37:29Z

History:
- 2026-08-31T19:08:52Z XDNTREHCEDRG7S7TF62T9VWMEF-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T14:18:10Z CNB1FH512YJJ30JNCQ7RQF0HWK-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=vm-epoch-identity-drift
- 2026-09-01T14:27:15Z 34AQATQ7REJVZYFB62MQ3J0EF6-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=vm-epoch-identity-drift
- 2026-09-01T17:01:41Z EQA9TFFNMBDNMWVYGRHYGJSYJS-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T21:35:36Z GJ1JJDKHVK52GP2WERF0T4S4C7-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T21:35:39Z FS0QK29NAB6JS1H55DVTJ1X2TY-m0-c5dbf036 set-budget actor=human:Wido targets=vm-epoch-identity-drift
- 2026-09-01T21:37:29Z 2EQNXM3PG6417Q20PVD365TCDA-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T22:23:43Z PAJ3GNEM7P8BZX6R5REME0TVGQ-m0-c5dbf036 set-budget actor=human:Wido targets=vm-epoch-identity-drift
- 2026-09-01T22:37:17Z PBHBC8RWX5VGCHRWAG3KRY92KS-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-09-01T22:37:21Z ADWNSSW54E2SH74CWESN50RG84-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
Integrity: sha256=d14eca6aed53962a89f9ca59798f9dbaec7ce7d6abfb6b1552ac59f52563c2fc
