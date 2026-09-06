# claude-delegate-scratch-cleanup

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: disk usage and a missing test, nothing unsafe ships; novelty 1: a cleanup trap and one assertion; exposure 2: every claude delegate round on every seat; accumulation 2: the directories pile up per round and a suite day multiplies them"
- Tier: 2
- Intent: Since goal code-critic-runtime-has-no-shell, every claude delegate round creates a private scratch directory under the system temporary directory (metasystem-claude/<job>-<round>, holding GOCACHE and GOTMPDIR) and nothing removes it; a full-module test run fills hundreds of megabytes per round, so a suite day accumulates gigabytes until the operating system sweeps (critique ccs-critic2, F-4). The same critique noted that no test pins the four sandbox switches (enabled, failIfUnavailable, autoAllowBashIfSandboxed, allowUnsandboxedCommands), so a flip of allowUnsandboxedCommands to true would pass the suite (F-7). DONE means the adapter removes the round's scratch directory when the round ends on every exit path, a fixture proves it, and a settings test pins the four switches.
- Origin: main
- Next step: MECHANICAL: brief the cleanup in scripts/agents/adapters/claude.sh (remove the scratch dir after the round's result is derived, on success and failure) plus the switch assertions in internal/adapter/runtime_test.go; build, land as its own chain.
- OpenedAt: 2026-09-06T19:57:39Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T19:57:39Z XEWQR8HCR7HANJYXCGEF3W4Z4P-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-delegate-scratch-cleanup
Integrity: sha256=679e5109455cdbfb6a194681b3272e77dfa5331f61c79886207c9a7f25de4005
