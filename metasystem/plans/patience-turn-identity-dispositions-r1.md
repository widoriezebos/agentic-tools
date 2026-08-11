# Dispositions — patience-turn-identity, critique round 1

Critic: design-critic-20260811t094448z-5deb (codex gpt-5.6-sol, xhigh).
Verdict: 7 material. All 7 ACCEPTED; design amended in the same commit.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| PTI-R1-001 | high | Host turns produce no session-established signal | ACCEPT — sources are per runtime capability (signal only where declared); the terminal envelope is the universal host source. |
| PTI-R1-002 | high | No-witness rule fails open on return application | ACCEPT — fail closed on application, fail open on blame: mutations not applied, T4 path runs, breaker not fed. |
| PTI-R1-003 | high | Exit 6 misstated; missing-session branch undefined | ACCEPT — duties separated: rotation reports, missing-session keeps exit 6 and feeds the no-witness branch. |
| PTI-R1-004 | high | Gate-pass on rejected return had no precedence | ACCEPT — measured truth wins: classification and gatePassed stand, mission completes, fault recorded. |
| PTI-R1-005 | high | Turn-log entry lacked the measurement outcome | ACCEPT — turn log carries the same outcome and fault as the ledger. |
| PTI-R1-006 | high | Observed session not propagated forward | ACCEPT — T4b: the next announcement derives from the last concluded turn's observedSession. |
| PTI-R1-007 | critical | Suffixes break the strict classification-line parsers | ACCEPT — classification line untouched; fault and cap are separate annotation lines, which all parsers tolerate (pinned pattern). |
