# vm-epoch-identity-drift

- State: queued
- Intent: Process identity breaks on VM guests when NTP steps the clock (m0 finding, 2026-08-31): pid start times derived as epoch seconds shifted by exactly -1s between m0's enrollment (12:19Z) and later probes — every stored record (session announcement start=1788178136, supervision owner pidStartedAt=1788178790) now disagrees with every fresh probe (135/789) while startTicks and bootId are unchanged. Epoch-based identity comparison then fails everywhere at once: census verdict CENSUS-FAILED, repo-watcher cannot reconcile, metasystem up refuses ('the recorded owner identity is not live' for a provably alive owner), and dispatch admission refuses on the stale census. The stable identity is (pid, startTicks, bootId), already recorded everywhere; the derived epoch is not stable on clock-disciplined VMs and must not be the comparator (or must tolerate bounded drift).
- Origin: main
- Next step: Reproduce in a test (synthetic boot-epoch shift), then fix the identity comparison to prefer startTicks+bootId with epoch as display only, or accept +-2s drift; the supervise/identity seam owns it. Immediate workaround on m0: lawful owner shutdown + re-arm re-mints records on the current epoch basis (blocked on session permissions at time of writing - operator action requested).
- OpenedAt: 2026-08-31T13:53:56Z
- Revision: 1

History:
- 2026-08-31T13:53:56Z 05EYZCGXMCFXCN1H3QCMDC6FZN-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=vm-epoch-identity-drift
Integrity: sha256=8a74446edc4bb7f801052c922e89ca67f5b9913440d2941199dfd90cf8534484
