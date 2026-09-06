# adopt-bed-authority-probe-passes-tier

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: a fixture drift, no product path; novelty 1: one flag set follows the verb; exposure 2: every adopt bed run on every seat is red past its adoption legs; accumulation 2: a red bed on main teaches seats to skip it, which is how today's adoption legs went unexecuted for an hour"
- Tier: 2
- Intent: The adopt bed's post-adoption authority-probe leg (scripts/adopt-fixtures.sh line 551-559) opens a goal in the adopted target with --tier 3, a flag goal open stopped accepting when risk became four questions (b4ae9395); the leg expects a 'lease holder' refusal for a non-human caller and now gets 'flag provided but not defined: -tier', so the bed exits red at that leg on every run on main (seen 2026-09-06 18:58Z on m1, evidence artifacts/agents/suite-failures/20260906T185830Z-adopt-16865). DONE means the probe opens with the current intake flags (--risk and --basis), the leg's two assertions keep their meaning, and the bed passes that leg on main.
- Origin: main
- Next step: MECHANICAL: brief a one-leg fix (replace --tier 3 with --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis 'authority probe'), build, land as its own chain, proof is the leg passing inside the bed on m1.
- OpenedAt: 2026-09-06T19:00:03Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T19:00:03Z A25NNYKWPZWFZJXHPVRH0J16N9-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=adopt-bed-authority-probe-passes-tier
Integrity: sha256=4ce0b005fee2d5749a2707a06d60d8d87c7074d612482ebe57dd9dffe4ecec66
