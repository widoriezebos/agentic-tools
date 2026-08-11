# Stop-loss satellites — routed scope from the split

Owner: unclaimed (each satellite is its own design + critique loop).
Parent: `plans/stop-loss-last-defense.md` (critique-exhausted; split
approved by the human 2026-08-11). Core: `plans/stop-loss-core.md`.
Precondition for ALL satellites: first map the runner's ACTUAL cycle
sequence (internal/missionrunner/loop.go) — the parent died of designing
against an assumed sequence.

1. TURN IDENTITY (announced/observed sessions, honest-host acceptance,
   protocol-violation handling). Routed findings: parent r1[6][7][8],
   r3[9].
2. MISSION REAP + BOUNDED DRAIN (lease-authorized, custodian-proven
   reaping of the mission's own reservations; finite drain deadline;
   drain-stalled park with a cycle-consistent resume). Routed: parent
   r1[9][10], r2[8][9], r3[10][11].
3. ORPHAN + USAGE CAPTURE (applied-(chain,round) sets, scan points that
   close the race, single-writer usage with SIGKILL recovery from the
   runtime's surviving event stream). Routed: parent r1[11][12],
   r2[10][11][13], r3[12][13].
4. LOOP-ADVANCED CREDITS (deferred from the core; only worth designing if
   last-defense budgets prove insufficient in practice). Routed: parent
   r1[0][1], r2[2][3][4][5], r3[4][5][6].
