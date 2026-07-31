# Instruction Ledger

Standing ledger of instruction changes adopted by retros (`skills/retro/SKILL.md`). Rows enter as `ADOPTED` with `Review by` naming the next retro; that retro replaces the status with a verdict: `KEPT` (the expected effect is visible), `KEPT-UNPROVEN` (cheap and uncontradicted, but no supporting evidence yet), `AMENDED`, or `REVERTED`. Two consecutive `KEPT-UNPROVEN` verdicts revert by default, which is how "unsupported after two reviews" stays trackable when each verdict overwrites the last.

| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
