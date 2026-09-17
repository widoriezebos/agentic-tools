# glb-known-issues-are-fixed

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Fixes to landed goal-branch code, each with a witness; a wrong fix is caught by its witness and the goal's proof."
- Tier: 2
- Intent: The known issues that goals-live-on-branches merges with are fixed forward right after the merge. DONE, each fixed with a witness or recorded as not a defect with the reason: (1) the Build A read notes that C1 unit 0 did not build (hact-20260912/m1e-tools-0917/glb-read-glbA.md, hact-20260912/m1e-tools-0917/glb-read-glbA-fix.md); (2) Build B read notes 7-16 (hact-20260912/m1e-tools-0917/glb-read-glbB.md); (3) B fix read notes 2-9 (hact-20260912/m1e-tools-0917/glb-read-glbBX.md; note 1 is in C2); (4) C1 read notes 5-13 (hact-20260912/m1e-tools-0917/glb-read-glbC1.md; the three breaking items and note 4 are fixed before the merge); (5) the callers of the old land.sh (reset-land.sh and the rest) move to the new landing script installed under its own name; (6) C2's read non-breaking items once it returns; (7) the goal's Next lists each open item until it is fixed.
- Origin: human
- Next step: 2026-09-17 15:59 local (m1e): opened on Wido's word from m1c's note list. Before the glb merge (not here): C1 read's three data-loss items (amend drops unstaged edits, land.sh pane reset outside ledger-preserve, failed done sweep loses the arc retro debt) in m1c's fix2, and note 4 (new land.sh installed under a new name so old callers keep working). After the glb merge: one build for all items, sized under 1,500 lines, on this goal.
- OpenedAt: 2026-09-17T13:59:47Z
- Revision: 1
- Labels: known-issue
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-17T13:59:47Z THWS3ZAMZ1RYGBWEDQKYP2DFT5-m1e-c6925449 open actor=human:Wido targets=glb-known-issues-are-fixed
Integrity: sha256=6472a8e9f2577a6f4dc072256b2f448eb48d2edf23de3a3aec5f93e348dc57a0
