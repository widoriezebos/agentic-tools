# validation-harvest-mode

- State: queued
- Intent: One costly validation run yields ALL its defects, not the first: a keep-going mode runs every section past failures, preserves each red section's evidence, and reports the full list at the end — the fail-fast suite made seven latent defects cost seven ~30min cycles during suite-dispatch-exclusion when two harvesting runs would have caught them (Wido 2026-08-27: same disease as the battery's stop-at-first-problem)
- Origin: main
- Next step: Appetite: 2h, single slice. Build on m1's validate-section-selector.sh (landed fbbfbec-adjacent): a --keep-going flag or thin wrapper iterates 'list', runs each section trapping its exit, preserves per-section failure evidence exactly as the suite already does, and ends red with the COMPLETE defect list. Nested runs (adopt fixtures) inherit the flag so a nested red also harvests. The battery consumes the same mode — coordinate with m1 who owns battery process fixes and is mid-arc on the selector; label shared. Incident-derived, single-slice: direct to backlog.
- OpenedAt: 2026-08-27T06:02:25Z
- Revision: 2
- Labels: shared
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-27T06:02:25Z QCQH09VMNZR0S5Z6XY0X72AXWH-m2-bc1be9cb open actor=m2+mac-coordinator targets=validation-harvest-mode
- 2026-09-01T20:29:50Z MG3S0YXGJJWDZ7SP6FQWPT5HA8-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=validation-harvest-mode
Integrity: sha256=01f9e5b929768015078f8023d9707523acd5c175c12046fdc9e5c20ddfc6d8ac
