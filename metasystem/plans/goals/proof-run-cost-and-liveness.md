# proof-run-cost-and-liveness

- State: queued
- Intent: A proof run can never again be silent, unbounded, and quadratically priced: pre-landing validation proves the STAGED BATCH once and nested runs reuse it, every suite announces its expected cost up front, emits progress heartbeats, and dies loudly at a silence failsafe instead of stalling dark (Wido 2026-08-27 evening: very serious bug, must never be allowed to happen again — 2h dirty-tree adopt run, 50min operator silence)
- Origin: main
- Next step: Appetite: 1d, design-first (Wido's direct order counts as the draft discussion; his 2026-08-27 evening condition RECORDED: these features must MECHANICALLY prevent recurrence once landed — that guarantee is the acceptance bar). Design in critique (plans/proof-run-cost-and-liveness-design.md). AMENDMENT closing the audit's gap: the progress contract is STRUCTURAL, not conventional — the validation suite asserts that every section named by the section selector emitted its heartbeat during the run (a run ending with silent sections is RED), so a future suite or section cannot be added without liveness; the cost banner is likewise asserted present. With this, the four-leg pricing block + metrics watch/act gives: proofs paid once (witness), priced by risk and size (tiers, lane), at measured unit cost (clock audit), announced up front, alive throughout, killed loudly on silence, with crossings surfacing as unattended steward incidents and draft entries (metrics slices 2-3) — recurrence prevented by construction, not vigilance. Slices as designed; build after critique converges.
- OpenedAt: 2026-08-27T17:07:53Z
- Revision: 2
- Labels: shared

History:
- 2026-08-27T17:07:53Z 15PY3WX8E8D98882HN5W21B1ND-m2-bc1be9cb open actor=human:wido targets=proof-run-cost-and-liveness
- 2026-08-27T17:22:10Z RQJ353PZ87KVYTCBFWFTX7Q2CP-m2-bc1be9cb edit actor=human:wido targets=proof-run-cost-and-liveness
Integrity: sha256=6b881d96d5c4706e49268e39c5d615a2f0c1b807128b0c98dc8cfcc10e0642bc
