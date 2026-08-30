# winddown-zombie-ownership-linux

- State: queued
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): the wind-down's tri-state ownership misreads a zombie-holding group on Linux - an empty /proc/<pid>/cmdline reads as KNOWN-empty argv, so a mid-reap zombie classifies NOT-OURS (provably foreign) instead of dead-or-indeterminate, and kill(-pgid, 0) over a zombie-only group differs between darwin and Linux. TestTerminateGroupKillsThroughATermImmuneOwnedGroup and TestTerminateGroupLeaksNoGroupsUnderCompression are red on the guest and green on darwin. The kill-through never signals a wrong group (zombies need no kill) - the defect is the misclassification, the loud false 'recycled group' evidence, and the red tests on the platform the missions actually run on.
- Origin: main
- Next step: Appetite: 2h. In the identity prober or the ownership fold: a readable-but-EMPTY argv must not produce NOT-OURS - a zombie is Dead-or-indeterminate for ownership purposes; align groupAlive's zombie semantics across platforms or make the wind-down treat zombie-only groups as down. Verify on the Debian guest (the sweep's lima VM, transport remote in place) before concluding - darwin green is not the proof.
- OpenedAt: 2026-08-30T09:55:06Z
- Revision: 1

History:
- 2026-08-30T09:55:06Z DKZTPG05478QCWG2FKCCM5QH48-m2-bc1be9cb open actor=m2+mac-coordinator targets=winddown-zombie-ownership-linux
Integrity: sha256=8495e616889d9f10f1fe164da20efabb3cfdf9ea71661021ad7d0404c6c2699a
