# Dispositions: patience-satellite-4, round 18

Job: design-critic-20260811t213328z-f805 (codex gpt-5.6-sol, xhigh).
2 findings, 2 material, both accepted. P4-086 exposed that several
earlier verification-section edits silently no-opped (unanchored
string replacement — the recurring tooling failure); the whole
Verification section is regenerated rather than patched again.

| id | disposition |
| --- | --- |
| P4-085 | accepted — resume_collision LEAVES the never-started vocabulary: it never belonged there. The vocabulary exists for errors that co-occur with a patched effectiveModel despite no work; a PRE-START resume collision carries no effectiveModel (no handshake happened), so the model requirement already excludes it, while a post-running collision is real lost work that must count. Residual honesty recorded: handshake_missing_session_id with all-null usage still classifies never-started — the conservative direction (a round is excluded, nothing over-nags), with unprovable spend remaining the usage jurisdiction (satellite 3). |
| P4-086 | accepted — the Verification section is regenerated whole, carrying every case rounds 13-17 promised: the never-started vocabulary enumerated against its writer sites, the failed-handshake patched-model case, all-null usage proving nothing, tokens-unavailable positive provider units counting, post-run mismatch with spend counting, and the spend-rule precedence. The fold discipline gains a hard rule: every replacement is grep-verified in the same command, no unconditional "applied". |
