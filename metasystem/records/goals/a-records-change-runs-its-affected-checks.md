# a-records-change-runs-its-affected-checks

- State: done
- Priority: 1
- Sequence: 54
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="findings 320 and 321: a records-only push restarts a full 233-unit proof while the lane can scope checks to changed inputs, and a design page committed by hand turned main red because nothing checked it before the push; design plans/designs/a-records-change-runs-its-affected-checks.md (Astra 7 → 3 → 2 → 0)"
- Tier: 3
- Intent: a change to records runs exactly the checks its inputs reach, inherits nothing unproven, credits no clock it did not earn, and is validated by every publisher before it is pushed
- Origin: human
- Next step: U1 (changed-main scope, credit, triggers, unified classifier) then U2 (publisher validation at the transaction boundary incl. work land --records); briefs with Astra, Codex by hand, Opus read, land by hand, deploy
- Concluded: U1, U2 and U3 landed by hand under the broken-machine rule (efeb02804, 29e7d21d8, 4f0f6559d), each gated on the whole command package and proven green by the lane in full; finding 332 closed by U3: a records-only move on a red tree runs its affected checks and repeats nothing else
- OpenedAt: 2026-10-15T07:21:32Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T07:21:41Z revision=2 opid=X0NP1P4QRN0WW36P2JRAKHGJS7-m1e-670f65d0 authority=proven digest=a900af638b4c8e7897127471218f710d99b92d17f1c65508982ed7a3208fad9f episode=2

History:
- 2026-10-15T07:21:32Z MJPW8AP5GX04FYS0DRRFKHQKSR-m1e-670f65d0 open actor=human:Wido targets=a-records-change-runs-its-affected-checks
- 2026-10-15T07:21:41Z X0NP1P4QRN0WW36P2JRAKHGJS7-m1e-670f65d0 approve actor=human:Wido targets=a-records-change-runs-its-affected-checks
- 2026-10-15T07:21:49Z 1GH03AH8NR3GXN5A9DNZYCQXAS-m1e-670f65d0 set-priority actor=human:Wido targets=a-records-change-runs-its-affected-checks reason=priority-order subject=a-records-change-runs-its-affected-checks from=unranked to=1:56 requested-sequence=append
- 2026-10-15T08:14:38Z 1FAA39GRBBSHDHG15EPFF0G3KK-m1e-670f65d0 done actor=human:Wido targets=a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,lane-proof-finds-its-own-engine,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does reason=priority-order from=1:56 to=1:55
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers reason=priority-order from=1:55 to=1:54
- 2026-10-16T13:50:22Z QFQDE3W9XPYGPW7G66PC9ESSNX-m1e-670f65d0 done actor=human:Wido targets=a-done-goals-branch-is-swept,a-records-change-runs-its-affected-checks,the-lane-deploys-the-engine-it-runs
Integrity: sha256=73d7e25825a05857ad0a672c3eb7392677356f021c72b400ae2b65fab89e9902
