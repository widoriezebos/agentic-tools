# acp-late-window-flake-fix

- State: queued
- Intent: TestTurnLateWindowTraffic reached three sightings in thirty days (2026-08-24 x2, 2026-08-29 under the L6 gate gauntlet) - the flake protocol earns its fix goal; a timing-shaped window test that fails only under load is a synthetic-clock candidate
- Origin: main
- Next step: Appetite: 1h — make the late-window assertion load-immune: drive the window from a synthetic clock or widen the deterministic seam (coordinate with timing-tests-synthetic-clock); acceptance = 20 consecutive passes inside a loaded full gauntlet
- OpenedAt: 2026-08-28T23:35:43Z
- Revision: 1

History:
- 2026-08-28T23:35:43Z JXX90ED5YDGXMB9Q65VFAC7R94-m1-bf243850 open actor=m1+coordinator targets=acp-late-window-flake-fix
Integrity: sha256=04d6b6e4ba6d71a1dce8834cffdcae4e95487ade27ce23334f285f5f4029a2eb
