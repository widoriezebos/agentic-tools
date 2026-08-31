# vm-epoch-identity-drift

- State: queued
- Intent: Process identity breaks on VM guests when NTP steps the clock (m0 finding, 2026-08-31): pid start times derived as epoch seconds shifted by exactly -1s between m0's enrollment (12:19Z) and later probes — every stored record (session announcement start=1788178136, supervision owner pidStartedAt=1788178790) now disagrees with every fresh probe (135/789) while startTicks and bootId are unchanged. Epoch-based identity comparison then fails everywhere at once: census verdict CENSUS-FAILED, repo-watcher cannot reconcile, metasystem up refuses ('the recorded owner identity is not live' for a provably alive owner), and dispatch admission refuses on the stale census. The stable identity is (pid, startTicks, bootId), already recorded everywhere; the derived epoch is not stable on clock-disciplined VMs and must not be the comparator (or must tolerate bounded drift).
- Origin: main
- Next step: Workaround applied on m0 (2026-08-31 ~15:55Z): Wido ran the lawful owner shutdown in-session; up re-armed generation 2 with records minted on the current epoch basis; census green, dispatch admission restored. The defect stands for the fix leg: identity comparison must prefer startTicks+bootId (already recorded everywhere) over the NTP-unstable derived epoch, or tolerate bounded drift. Reproduce with a synthetic boot-epoch shift in a test; supervise/identity seam owns it. Note for any machine hitting this: the symptom cluster is CENSUS-FAILED + 'recorded owner identity is not live' for a live owner + every stored start epoch off by the same 1-2s from fresh probes.
- OpenedAt: 2026-08-31T13:53:56Z
- Revision: 2

History:
- 2026-08-31T13:53:56Z 05EYZCGXMCFXCN1H3QCMDC6FZN-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
- 2026-08-31T14:09:40Z A1D2EDVZDNVW42KX6PCDNNDPDQ-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
Integrity: sha256=e91bc16879e1f46ba2795ec3ff8e4f31fabede974606a77a0d467a09dbfde8a7
