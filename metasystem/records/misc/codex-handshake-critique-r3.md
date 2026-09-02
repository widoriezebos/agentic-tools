# Codex handshake design — Sol round-3 critique register

Job ch-crit-3 (design-critic, codex gpt-5.6-sol, xhigh), 2026-09-02, design
revision 4 (plans/codex-handshake-design.md), brief
plans/codex-handshake-critique-r3-brief.md. One material finding; the
critic fold, the sessionless follow-up and the orchestrator's claim decision
(no fingerprint version bump) otherwise held, and no regression was found in
Part 1 or D2.1 to D2.4 and D2.6. Full return:
artifacts/agents/ch-crit-3/rounds/1/return.json (mirrored to evidence).

| Id | Severity | Claim (condensed) | Evidence |
| --- | --- | --- | --- |
| CHS-R3-EXIT-01 | high | D2.5 promises `handshake_failed:exit=N` "for every status, zero included" and section 6 says Claude and Devin inherit the exit verdict through the shared library; but devin.sh's status-zero, no-session, empty-stdout branch calls `fail_pending` directly (`handshake_missing_session_id` or `empty_reply` by presence scan), bypassing `complete_from_cli`, adjudication, `criticFailureFold` and the `handshakeExitStatus` forwarding — so a Devin critic in that shape is not `protocol_error`, is refused at the follow-up gate, and its chain wedges. | design lines 39-42, 128; devin.sh:650-662 (bypass), :690 (the shared path); runtime-common.sh:359-368; dispatch.sh:1727 |

Orchestrator note (m0b): the wedge is pre-existing for that branch
(`empty_reply` written directly today also never reaches the fold); revision
5 closes it by moving the critic fold into `fail_pending` itself through an
engine verb, so every direct call in every adapter folds from the one Go
table.
