# Instruction Ledger

Standing ledger of instruction changes adopted by retros (`skills/retro/SKILL.md`). Rows enter as `ADOPTED` with `Review by` naming the next retro; that retro replaces the status with a verdict: `KEPT` (the expected effect is visible), `KEPT-UNPROVEN` (cheap and uncontradicted, but no supporting evidence yet), `AMENDED`, or `REVERTED`. Two consecutive `KEPT-UNPROVEN` verdicts revert by default, which is how "unsupported after two reviews" stays trackable when each verdict overwrites the last.

| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| IL-1 | retro-2026-08-03 | Fixture waits carry named ceilings like every run | `docs/orchestration.md` | no hang-class receipt recurs | validation never exceeds its ceiling silently | next retro | ADOPTED |
| IL-2 | retro-2026-08-03 | Verify asserts world-state facts over agent narration | `skills/verify/SKILL.md` | no narration-marker defect recurs | selftest-class probes stay mechanical | next retro | ADOPTED |
| IL-3 | retro-2026-08-03 | Audit falls back to grep when rg is absent (implementation queued to Codex) | `scripts/audit-harness.sh` | audit runs on machines without ripgrep | the rg shim workaround disappears from receipts | next retro | ADOPTED |
| IL-4 | retro-2026-08-03 (amended same day) | Agents launch only through the dispatcher; recordless agent processes are UNTRACKED defects | `docs/orchestration.md` | no untracked-launch incident recurs | the watcher reports UNTRACKED for any recordless agent process | next retro | ADOPTED |
