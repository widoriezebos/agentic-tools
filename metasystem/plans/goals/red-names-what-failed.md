# red-names-what-failed

- State: approved
- Priority: 1
- Sequence: 51
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="findings 321, 311, 327: a red labelled a passing test as a flake; one flaky test filed twice; a confirmed-flake incident outlived its fix goal and three green proofs because clearLandingIncidents excludes flake entries and nothing feeds countFlakePass (goal/trunkred.go:909). Design plans/designs/red-names-what-failed.md (the former U2 of lane-proof-finds-its-own-engine, split out so each goal integrates once)"
- Tier: 2
- Intent: a red names exactly what failed, one flaky test is one incident, and the lane's proofs feed the existing three-pass flake closure so a fixed flake closes by itself
- Origin: human
- Next step: U1 brief, Astra to zero or stop rule, Codex by hand after steward-duties-run-on-every-tick, Opus read, land by hand
- OpenedAt: 2026-10-15T06:37:02Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T06:37:31Z revision=2 opid=2MERMXTQAF7QHKF0A6QXG25CE5-m1e-670f65d0 authority=proven digest=bc4293371071947946e4135b2bcffcd3e8c2c20ede2081d327f972d69946131c episode=2

History:
- 2026-10-15T06:37:02Z QVJR0MXVWYCRTS77HZHRZBAZSX-m1e-670f65d0 open actor=human:Wido targets=red-names-what-failed
- 2026-10-15T06:37:31Z 2MERMXTQAF7QHKF0A6QXG25CE5-m1e-670f65d0 approve actor=human:Wido targets=red-names-what-failed
- 2026-10-15T06:38:11Z 8CMW0878NANJRKED6CHNJ28YPQ-m1e-670f65d0 set-priority actor=human:Wido targets=red-names-what-failed reason=priority-order subject=red-names-what-failed from=unranked to=1:53 requested-sequence=append
- 2026-10-15T08:14:38Z 1FAA39GRBBSHDHG15EPFF0G3KK-m1e-670f65d0 done actor=human:Wido targets=a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,lane-proof-finds-its-own-engine,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does reason=priority-order from=1:53 to=1:52
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers reason=priority-order from=1:52 to=1:51
Integrity: sha256=5f0b507172ee716b583f7f5104d53864f4a5348e28ea1d484d9e60fb814c2aef
