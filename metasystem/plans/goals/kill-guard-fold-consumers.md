# kill-guard-fold-consumers

- State: queued
- Intent: m2's janitor fold change (dab1dbd) made an empty member scan INDETERMINATE instead of NOT-OWNED: kill-guard consumers now see exit 3 (fail-closed) where they saw 1 - every m1-side consumer must be verified against the new contract (Ruling R: a changed contract runs its callers)
- Origin: main
- Next step: Appetite: 1h. Enumerate every caller consuming janitor fold/kill-guard exit codes on m1 surfaces (steward sweeps, cancel paths, cleanup scripts), verify each handles exit 3 as fail-closed correctly, fix any that treated 1 as the only refusal; prove with the affected fixtures
- OpenedAt: 2026-08-30T14:57:35Z
- Revision: 1

History:
- 2026-08-30T14:57:35Z BHNDDTA3DWP99EGG7G2BB41N5H-m1-bf243850 open actor=m1+coordinator targets=kill-guard-fold-consumers
Integrity: sha256=0def167fa648e8c967481589c5d335110b1a23d6e961ef2026fdf717956ae23d
