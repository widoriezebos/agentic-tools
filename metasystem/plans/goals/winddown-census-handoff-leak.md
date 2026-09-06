# winddown-census-handoff-leak

- State: approved
- Intent: The sharpened compression wind-down leak test (chain mr-flake-fix2, certified 2026-08-31) caught the real defect it was rebuilt to catch: in governed run governed-discharge-20260901-a, a live process group (pid-group 88961) survived the SIGKILL window, the wind-down lawfully abandoned it under the fail-closed ownership law, and the census NEVER ADOPTED IT - alive, unkilled, outside all custody (census enumeration returned empty, 0 of the abandoned groups in custody, 4 unknown within the signalable universe). This is the zombie-leak class the whole custody family exists to prevent, on the wind-down-to-census handoff seam. Evidence: artifacts/agents/suite-failures/20260831T224116Z-1742/go-engine-gate.log (winddown_test.go:225-229); the test passed in run -e and failed in -a, so the handoff loss is timing/load-dependent.
- Origin: main
- Next step: Appetite: 3h, HIGH priority - this blocks the first weight discharge (goal standing-validation, envelope at ceiling) and is a real custody hole. Triage first: is the census adoption of abandoned groups (a) never implemented for this path, (b) implemented but racing the group's process identity, or (c) implemented but the abandonment never reaches it. The dab1dbd fold contract and the lease-fold fix (3ba27a82) are adjacent - verify the handoff against both. Then fix on the owning seam with a test that constructs the survived-SIGKILL case deterministically. The discharge resumes on Wido's word after this lands.
- OpenedAt: 2026-08-31T22:42:31Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=519737a81d5ee24bbc3c30e0b46ea5c2545079be122e04aebbb31f1a92a3c2fa

History:
- 2026-08-31T22:42:31Z N02MC9XCBZ382B3ZSTNPPCR5MV-m2-bc1be9cb open actor=m2+mac-coordinator targets=winddown-census-handoff-leak
- 2026-09-01T20:29:54Z DGZC0JC3YEW8N2ZS31N0VJ8BT1-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=winddown-census-handoff-leak
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=winddown-census-handoff-leak reason=sweep
Integrity: sha256=a6d46a5396aa378e22d49273d8cb1549958d636a16b28e30f09553da1f69438d
