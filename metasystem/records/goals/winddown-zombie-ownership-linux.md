# winddown-zombie-ownership-linux

- State: done
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): the wind-down's tri-state ownership misreads a zombie-holding group on Linux - an empty /proc/<pid>/cmdline reads as KNOWN-empty argv, so a mid-reap zombie classifies NOT-OURS (provably foreign) instead of dead-or-indeterminate, and kill(-pgid, 0) over a zombie-only group differs between darwin and Linux. TestTerminateGroupKillsThroughATermImmuneOwnedGroup and TestTerminateGroupLeaksNoGroupsUnderCompression are red on the guest and green on darwin. The kill-through never signals a wrong group (zombies need no kill) - the defect is the misclassification, the loud false 'recycled group' evidence, and the red tests on the platform the missions actually run on.
- Origin: main
- Next step: Appetite: 2h. In the identity prober or the ownership fold: a readable-but-EMPTY argv must not produce NOT-OURS - a zombie is Dead-or-indeterminate for ownership purposes; align groupAlive's zombie semantics across platforms or make the wind-down treat zombie-only groups as down. Verify on the Debian guest (the sweep's lima VM, transport remote in place) before concluding - darwin green is not the proof.
- Concluded: Landed dab1dbd, fixed and proven on the platform that found it: the janitor fold's empty-scan row returned PROVABLY-NOT-OWNED (a mid-reap group read as positively foreign and the wind-down abandoned it with false recycled-group evidence) - NOT-OWNED now demands a positive not-ours observation, empty scans are INDETERMINATE, kill-through handles them, and the dispatch kill guard stays fail-closed. Linux's zombie-only-pgid-stays-signalable behavior (measured empirically on the guest: state alive, argv unreadable, kill0 true) gets its own honest verdict: down-to-zombies-pending-reap, typed and loud, never a leak error. The two red guest tests green twice with the fix; janitor + missionrunner packages green on BOTH platforms (guest 115s, darwin 899s); the darwin TestTerminateGroup load-flake was the same empty-scan false proof - one cause, both platforms, one fix. Dual-push carried it to the transport, guest re-syncable at will.
- OpenedAt: 2026-08-30T09:55:06Z
- Revision: 4
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-30T09:55:06Z DKZTPG05478QCWG2FKCCM5QH48-m2-bc1be9cb open actor=m2+mac-coordinator targets=winddown-zombie-ownership-linux
- 2026-08-30T10:59:18Z WAQ0N9M1P1QH8DE63MDQ45DVSK-m2-bc1be9cb set-budget actor=human:wido targets=winddown-zombie-ownership-linux
- 2026-08-30T10:59:32Z GTTGTQKBXV1H9K0G0P2HSNDC69-m2-bc1be9cb claim actor=m2+mac-coordinator targets=winddown-zombie-ownership-linux
- 2026-08-30T11:35:25Z S37BGT3TJHGB04TMGS0PRVZFW1-m2-bc1be9cb done actor=human:wido targets=winddown-zombie-ownership-linux
Integrity: sha256=2c8b2229c7861b402a608b11e61ce48b2eb39ee8f1ed0c83da84e75eb3db647b
