# Codex handshake design — Sol round-2 critique register

Job ch-crit-2 (design-critic, codex gpt-5.6-sol, xhigh), 2026-09-02, design
revision 2 (plans/codex-handshake-design.md), brief
plans/codex-handshake-critique-r2-brief.md. One material finding; the six
round-1 folds otherwise held. Full return:
artifacts/agents/ch-crit-2/rounds/1/return.json (mirrored to evidence).

| Id | Severity | Claim (condensed) | Evidence |
| --- | --- | --- | --- |
| CHS-R2-CRITIC-FOLD-01 | high | Revision 2's D2.5 leaves `criticFailureFold` unchanged on the ground that `CritiqueRegisterAdvance` folds any failed critic round; but the register is only reached through the follow-up path, whose gate accepts `completed` or `failed && error == protocol_error` and then requires a resumable session. A critic that exits before its session under the new unfolded `handshake_failed:exit=N` is refused at the gate, never folded, and its chain wedges. The design must say how such a critic is folded and how its next round starts without a session; the exit-before-session fixture must exercise that path, not only the terminal record. | design line 39; dispatch.sh:1727-1757, 1855-1859; internal/dispatch/finding_register.go:51-59, 96-97; design line 100 |

Orchestrator note (m0b): the session half is pre-existing — dispatch.sh:1757
refuses every sessionless parent today, so a critic that reached
`fail-pending protocol_error handshake` already could not be followed up;
revision 3 is asked to close both halves, since the goal's own specimens
(breach-design-crit2, breach-design-crit2b) are exactly critics that died
pre-session.

Gaps recorded by the critic: read-only verification (no go or fixtures
run); the runtime notice classified the job as advisory.
