# design-loop-exit-in-the-skill

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: unattended seats loop for hours; novelty 1: the rules exist as rulings, they are not in the metasystem's own text; exposure 2: every design chain; accumulation 1: nothing built on it"
- Tier: 2
- Intent: The design-critique skill (skills/design-critique/SKILL.md) carries a round budget and 'the loop ends when further critique stops changing what an implementer would build', but not the two rules the seats learned the hard way and keep only in their private memory: (1) a DESIGN-BEARING chain closes only on a fresh read of the final round, so every fold invalidates the read that caused it and the loop has no natural exit; the coordinator counts fold-read cycles aloud, says so at cycle two, and before cycle three the land-or-fold call is the human's (Wido, 2026-09-08, after the stop-verb chain went five cycles unattended); (2) D81 (Wido, 2026-08-16): when two prose budgets are exhausted without convergence the exit is implementation-first behind fixtures, the page becomes the spec and standing findings become named fixtures, never a third prose budget. Evidence 2026-09-09: the abandoned-goal design ran four revisions and three reads (7, 11, 5 material findings) and grew 526 to 1733 lines before the coordinator applied D81 by hand. DONE means: both rules are written in the skill's Round Budget section and in docs/orchestration.md, the coordinator's stop-hook turn verdict prints the current fold-read cycle for a claimed design chain, and a fixture proves the verdict counts cycles.
- Origin: main
- Next step: Tier 2, MECHANICAL: two doc edits plus the cycle count in the turn verdict (internal/goal/turnverdict.go) with its test. Opened by m1b at Wido's ask, 2026-09-10 12:20Z.
- OpenedAt: 2026-09-10T08:45:27Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:45:27Z VM3W685EV9C2Q8EGNHQBWDAPW0-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=design-loop-exit-in-the-skill
Integrity: sha256=4c7dcbad125ac51babf14174d257416c4f1c647a8e5661b577efa575b0ca14af
