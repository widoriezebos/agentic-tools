# tests-observe-only-what-they-inject-2

- State: approved
- Priority: 1
- Sequence: 62
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="The same design and the same answers as tests-observe-only-what-they-inject: test-only changes to how fixtures observe processes; a wrong tag or audit rule hides a leftover (severity 3), the seam exists (novelty 2), every package's tests run under it (exposure 3), each unit adds a detector that stays (accumulation 3)."
- Tier: 3
- Intent: Tests observe only what they inject, part 2: every detached arming helper carries the binary-owned tag and a death witness proves that a dying supervise test binary leaves no helper behind (U2); an audit with a shrink-only baseline and reasons holds the line so no new violation lands (U3).
- Origin: main
- Next step: Brief U2 from the design page's inventory (internal/supervise/arming_test.go, 0 production lines expected); Codex builds it in a registered workspace, Opus reads, sharded gate, hand landing; then U3. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261019T150909.254823000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T150909.254823000Z.log; load 4.96044921875; seen 2 times. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261019T155542.811809000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T155542.811809000Z.log; load 5.28125; seen 3 times. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261019T164152.327067000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T164152.327067000Z.log; load 4.92138671875; seen 4 times.
- OpenedAt: 2026-10-19T15:09:36Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-19T15:09:45Z revision=2 opid=PHX5CQSB41NMJRG4HZNERFD8VZ-m1e-f2f350ed authority=proven digest=68d1a29a0a9a9bcd9de19f39efbae0c03166ef6ac17414181c3c8999479c1ba8 episode=2

History:
- 2026-10-19T15:09:36Z ME87ZJPNYV87M9VVW1PSZFR47H-m1e-f2f350ed open actor=human:Wido targets=tests-observe-only-what-they-inject-2
- 2026-10-19T15:09:45Z PHX5CQSB41NMJRG4HZNERFD8VZ-m1e-f2f350ed approve actor=human:Wido targets=tests-observe-only-what-they-inject-2
- 2026-10-19T15:09:53Z JWC4FSR06YWVMT5MZ6V12M0W1G-m1e-f2f350ed set-priority actor=human:Wido targets=tests-observe-only-what-they-inject-2 reason=priority-order subject=tests-observe-only-what-they-inject-2 from=unranked to=1:62 requested-sequence=append
- 2026-10-19T15:50:53Z 91710300F575C9FD1A3D41430C-landing-41ad7d6c record-flake actor=landing+landing-agent targets=tests-observe-only-what-they-inject-2
- 2026-10-19T16:36:13Z EEA52EE35577C1560859449A77-landing-41ad7d6c record-flake actor=landing+landing-agent targets=tests-observe-only-what-they-inject-2
- 2026-10-19T17:22:33Z 1DA095A7D5F3D7768C4BB4F75B-landing-41ad7d6c record-flake actor=landing+landing-agent targets=tests-observe-only-what-they-inject-2
Integrity: sha256=8adec3f12a8fc534c979fac89985da8ce00611021fc95c7bf1786b0ca7c9c8a6
