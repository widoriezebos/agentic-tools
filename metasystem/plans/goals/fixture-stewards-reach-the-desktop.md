# fixture-stewards-reach-the-desktop

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: nothing unsafe is permitted, but fake alerts on the operator's desktop teach the operator to ignore the real ones, and a real alert lost in that noise is the alert channel failing at its one job; novelty 1: one gate in a function that exists and one field on a record that exists; exposure 3: every macOS host that runs any suite arming a steward, and every builder sandbox on it - three seats on one host today; accumulation 2: every suite run on every seat adds pop-ups, so it grows with the fleet"
- Tier: 3
- Intent: The operator's desktop receives alerts from stewards that fixtures arm in temporary repositories. internal/steward/notify.go resolves the delivery command from the repository's git config and, when none is set, falls back on macOS to the platform notifier (osascript display notification) for ANY repository root; health-fixtures.sh, dispatch-fixtures.sh and supervision-hook-fixtures.sh arm stewards in temporary repositories, and every builder round runs those suites in its sandbox on this host, so Wido gets 'HEALTH unhealthy' pop-ups naming a repository under a temp folder (2026-09-06). Wido's word: the host receives real messages only, never test messages. The platform notifier is the operator's channel and only an installation the operator enrolled may use it: a steward enrolled by a human act (permanent generation at a terminal, or a temporary human word) reaches the desktop; a steward enrolled under fixture authority (the fixtureauth owner bound to a fake-runtime root) never does - its deliveries go to a notifications log under its own artifacts so a fixture can still assert that an alert was delivered. A real desktop message names its installation root, so an operator with several seats on one host can tell them apart.
- Origin: main
- Next step: BUILT, IN REVIEW (m1d, 2026-09-06 evening). Chain fsrd-build1b-20260906 round 1 (Sol): the steward identity carries its enrollment kind (human-terminal, temporary-word, fixture) stamped by every mint; lease.ClassifyAt marks a HUMAN class granted by a fixture authorization; the platform notifier serves only human enrollments and names its repository root; a fixture enrollment writes its alerts to notifications.log under its own steward artifacts; osascript shims in the health, hook and dispatch suites fail them on regression. Conformance review done (reviewed tree 5d19d3e93c592f050e11f023e909c37dc58abac9). Seat replays outside the sandbox on that tree: steward and lease packages green, health-fixtures.sh green, supervision-hook-fixtures.sh green, dispatch-fixtures.sh was running at the time of writing (log in the seat's scratchpad, fsrd-dispatch.log). NEXT: read the dispatch replay's exit code; if green, dispatch the code critic (brief plans/fixture-stewards-reach-the-desktop-code-critique-brief.md is landed; metasystem delegate --role code-critic --reviews fsrd-build1b-20260906 --op fsrd-cc1-20260906 --destructive-reach DESIGN-BEARING); then critique-register-advance, dispatch.sh close, git apply --index --directory=metasystem of rounds/1/diff.patch, land.sh --chain fsrd-build1b-20260906 --direct-fix register-carriage --goal fixture-stewards-reach-the-desktop --staged-only --allow-new-plan (reset records/narrator-digest.log first), rebuild the engine, goal done. No toolchain override on this host any more (go.mod pins Go 1.27).
- OpenedAt: 2026-09-06T07:46:47Z
- Revision: 6
- Pinned: m1d
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T08:37:53Z revision=2 opid=Y88BHYQXARNE0C3CSRZXRX5FMZ-m1-a4f8999f authority=proven digest=17f239dfcd727e1fc185e0bac94bb1e67c2174ad79a126af2745c15ffaa04ea9
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=4 at=2026-09-06T16:19:44Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-06T16:17:46Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T07:46:47Z H137WSFBYKWVX4CVENVAJGCWK4-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=fixture-stewards-reach-the-desktop
- 2026-09-06T08:37:53Z Y88BHYQXARNE0C3CSRZXRX5FMZ-m1-a4f8999f approve actor=human:Wido targets=fixture-stewards-reach-the-desktop
- 2026-09-06T08:37:55Z 2F806GC9BTA0E13RV32EZWVH6F-m1-a4f8999f set-pin actor=human:Wido targets=fixture-stewards-reach-the-desktop
- 2026-09-06T16:17:46Z WV3THRE719RN8P34TVG4E4WJ4Y-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=fixture-stewards-reach-the-desktop
- 2026-09-06T16:19:44Z 6ZPSMNJHSH5EV0HFSTW9BV5RBH-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=fixture-stewards-reach-the-desktop
- 2026-09-06T17:24:01Z 2J0HA8FDGXRTP26HYYJ7MZBTQJ-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=fixture-stewards-reach-the-desktop
Integrity: sha256=f78f6573e07d99587f691cf8d3b583b5800ab9594218ca33118afb500b6489bd
