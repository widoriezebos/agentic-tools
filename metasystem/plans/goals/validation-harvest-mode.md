# validation-harvest-mode

- State: queued
- Intent: One costly validation run yields ALL its defects, not the first: a keep-going mode runs every section past failures, preserves each red section's evidence, and reports the full list at the end — the fail-fast suite made seven latent defects cost seven ~30min cycles during suite-dispatch-exclusion when two harvesting runs would have caught them (Wido 2026-08-27: same disease as the battery's stop-at-first-problem)
- Origin: main
- Next step: Appetite: 2h, single slice. Build on m1's validate-section-selector.sh (landed fbbfbec-adjacent): a --keep-going flag or thin wrapper iterates 'list', runs each section trapping its exit, preserves per-section failure evidence exactly as the suite already does, and ends red with the COMPLETE defect list. Nested runs (adopt fixtures) inherit the flag so a nested red also harvests. The battery consumes the same mode — coordinate with m1 who owns battery process fixes and is mid-arc on the selector; label shared. Incident-derived, single-slice: direct to backlog.
- OpenedAt: 2026-08-27T06:02:25Z
- Revision: 1
- Labels: shared

History:
- 2026-08-27T06:02:25Z QCQH09VMNZR0S5Z6XY0X72AXWH-m2-bc1be9cb open actor=m2+mac-coordinator targets=validation-harvest-mode
Integrity: sha256=0ba1de8393cf0e00893db9c7e15deba1a35bcf91334a35f508892f9f192f39dc
