# acp-late-window-flake-fix

- State: done
- Intent: TestTurnLateWindowTraffic reached three sightings in thirty days (2026-08-24 x2, 2026-08-29 under the L6 gate gauntlet) - the flake protocol earns its fix goal; a timing-shaped window test that fails only under load is a synthetic-clock candidate
- Origin: main
- Next step: Appetite: 1h — make the late-window assertion load-immune: drive the window from a synthetic clock or widen the deterministic seam (coordinate with timing-tests-synthetic-clock); acceptance = 20 consecutive passes inside a loaded full gauntlet
- Concluded: Fixed by design in the timing-class landing: synthetic deadline seam, 20 consecutive loaded passes at 0.236s; the registry row closed per the fixed-leg policy
- OpenedAt: 2026-08-28T23:35:43Z
- Revision: 2

History:
- 2026-08-28T23:35:43Z JXX90ED5YDGXMB9Q65VFAC7R94-m1-bf243850 open actor=m1+coordinator targets=acp-late-window-flake-fix
- 2026-08-29T18:15:51Z 0T7SKGM2TPTNAJY40C8EZJ5A2X-m1-bf243850 done actor=m1+coordinator targets=acp-late-window-flake-fix
Integrity: sha256=636c24bdf4b77ecad53e00980297288ff2034f490ab323b1ea88db40160acfb8
