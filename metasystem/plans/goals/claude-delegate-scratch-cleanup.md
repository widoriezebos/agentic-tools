# claude-delegate-scratch-cleanup

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: disk usage and a missing test, nothing unsafe ships; novelty 1: a cleanup trap and one assertion; exposure 2: every claude delegate round on every seat; accumulation 2: the directories pile up per round and a suite day multiplies them"
- Tier: 2
- Intent: Since goal code-critic-runtime-has-no-shell, every claude delegate round creates a private scratch directory under the system temporary directory (metasystem-claude/<job>-<round>, holding GOCACHE and GOTMPDIR) and nothing removes it; a full-module test run fills hundreds of megabytes per round, so a suite day accumulates gigabytes until the operating system sweeps (critique ccs-critic2, F-4). The same critique noted that no test pins the four sandbox switches (enabled, failIfUnavailable, autoAllowBashIfSandboxed, allowUnsandboxedCommands), so a flip of allowUnsandboxedCommands to true would pass the suite (F-7). DONE means the adapter removes the round's scratch directory when the round ends on every exit path, a fixture proves it, and a settings test pins the four switches.
- Origin: main
- Next step: RELEASED 2026-09-07 00:25Z by m1c, resumable: chain csc-build1 is built, conformed (tree f22f165d), bed-green and CLOSED; the goal's accumulation makes it full width and the receipt battery fails on an unrelated fixture that expired at midnight UTC (goal fixture-review-by-date-rolls-over, awaiting Wido's approval). Resume: claim, run scratchpad csc-land-full.sh (receipt + land.sh --chain --test-receipt) once the date fix has landed, rebuild, up, land the proof brief (scratchpad csc-proof-brief.landed.md), dispatch csc-proof1, run scratch-check.sh on it, goal done.
- OpenedAt: 2026-09-06T19:57:39Z
- Revision: 9
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:18:46Z revision=3 opid=10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 authority=proven digest=2d459cfa518d08b171abccb4a29abd3be2d003f7c9165f98559db1f200728f07
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=5 at=2026-09-06T23:39:16Z

History:
- 2026-09-06T19:57:39Z XEWQR8HCR7HANJYXCGEF3W4Z4P-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T20:34:24Z 0P7V8HRBJPXCM6PM83S7AB4VZ1-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T21:18:46Z 10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 approve actor=human:Wido targets=adopt-bed-authority-probe-passes-tier,adopt-bed-gate-unreachable-under-load,claude-critic-sandbox-allows-loopback,claude-critic-shell-network-deny,claude-delegate-scratch-cleanup,claude-implementer-read-roots-writable
- 2026-09-06T22:29:50Z KJ32NQTPC4D2P3WCW5ZVHASSD5-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T23:39:02Z P0GKAN2PEQ164BV67RTS7S66KN-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T23:39:16Z 9J1D1EQXBZ9QW7HZKFMDP92CGH-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T23:39:41Z PH2SP1V1Q1F4KFQD084TERESYK-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-07T00:15:01Z P5TB5JWSB7ZJG2FNC1Y0G8213B-m1c-7cd0bd60 release actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-07T00:16:24Z H88D30X5PBRH1RN3FBHFVRN12B-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
Integrity: sha256=b6b809e3e907365bd28066d2adc5a3818ec9d4044c431b403d8d1903f4a36477
