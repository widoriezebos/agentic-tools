# Codex handshake design — Sol round-4 critique register

Job ch-crit-4b (design-critic, codex gpt-5.6-sol), 2026-09-02, reviewed
commit 3dbe5a5c, design revision 6 (plans/codex-handshake-design.md), brief
plans/codex-handshake-critique-r4-brief.md. One material finding; the fold's
idempotence on `complete_from_cli`, the `finish_running` fold scope, the
harness's ability to observe the folded patch, and everything rounds 1 to 3
left standing held. Full return: artifacts/agents/ch-crit-4b/rounds/1/return.json.

| Id | Severity | Claim (condensed) | Evidence |
| --- | --- | --- | --- |
| CHS-R4-FOLD-SCOPE-01 | high | The pair-only fold launders a pre-launch Claude adapter failure into a completed critic attempt: claude.sh:134 and :138 write `runtime_error handshake` when the engine's claude-command construction fails or returns an empty command, BEFORE the fork at :147; once `fail_pending` folds globally, a Claude critic in that shape reads `protocol_error handshake`, the follow-up gate (dispatch.sh:1727) admits it, and finding_register.go:96-98 creates a synthetic critique finding for work that never ran. | design line 46 (which names the consequence and accepts it); claude.sh:129-148; finding_register.go:96-98 |

Orchestrator decision (m0b, 2026-09-02 19:55Z), folded by revision 7: the two
pre-fork callers stop borrowing a runtime class. claude.sh:134 and :138 write
`fail_pending launch_failed handshake` — `launch_failed` is already the
dispatcher's own class for a launch that failed after reservation
(dispatch.sh:1618) and already sits in the never-started vocabulary
(internal/missionrunner/patience.go:47) — an unlisted pair that never folds
for any role. Rule of record for the fold table: a pair folds only when a
runtime actually produced it; pre-launch failures carry launch classes.
Budget note: 480 reserved minutes, 280 used after this round; the fold (20)
plus build (120) plus code critique (20) leave 40, so no fifth Sol design
round is dispatched — the code critique (Fable, against the design) is the
check on this one mechanical rename.
