# claude-delegate-scratch-cleanup

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: disk usage and a missing test, nothing unsafe ships; novelty 1: a cleanup trap and one assertion; exposure 2: every claude delegate round on every seat; accumulation 2: the directories pile up per round and a suite day multiplies them"
- Tier: 2
- Intent: Since goal code-critic-runtime-has-no-shell, every claude delegate round creates a private scratch directory under the system temporary directory (metasystem-claude/<job>-<round>, holding GOCACHE and GOTMPDIR) and nothing removes it; a full-module test run fills hundreds of megabytes per round, so a suite day accumulates gigabytes until the operating system sweeps (critique ccs-critic2, F-4). The same critique noted that no test pins the four sandbox switches (enabled, failIfUnavailable, autoAllowBashIfSandboxed, allowUnsandboxedCommands), so a flip of allowUnsandboxedCommands to true would pass the suite (F-7). DONE means the adapter removes the round's scratch directory when the round ends on every exit path, a fixture proves it, and a settings test pins the four switches.
- Origin: main
- Next step: MECHANICAL: brief the cleanup in scripts/agents/adapters/claude.sh (remove the scratch dir after the round's result is derived, on success and failure), the switch assertions in internal/adapter/runtime_test.go, and RESOLVE the scratch path (realpath: /var/folders is a symlink to /private/var/folders, and five adapter collect-port tests fail in a delegate shell on the mismatch, proof job lpb-proof1) and trim its trailing slash before composing (double slash, proof job ccs-proof1 F-4); build, land as its own chain.
- OpenedAt: 2026-09-06T19:57:39Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:18:46Z revision=3 opid=10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 authority=proven digest=2d459cfa518d08b171abccb4a29abd3be2d003f7c9165f98559db1f200728f07

History:
- 2026-09-06T19:57:39Z XEWQR8HCR7HANJYXCGEF3W4Z4P-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T20:34:24Z 0P7V8HRBJPXCM6PM83S7AB4VZ1-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
- 2026-09-06T21:18:46Z 10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 approve actor=human:Wido targets=adopt-bed-authority-probe-passes-tier,adopt-bed-gate-unreachable-under-load,claude-critic-sandbox-allows-loopback,claude-critic-shell-network-deny,claude-delegate-scratch-cleanup,claude-implementer-read-roots-writable
- 2026-09-06T22:29:50Z KJ32NQTPC4D2P3WCW5ZVHASSD5-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
Integrity: sha256=7c0d6b972e5b18e0d71ff6c58dc277541cd1cc856cfaeeccf3aba1bde3904459
