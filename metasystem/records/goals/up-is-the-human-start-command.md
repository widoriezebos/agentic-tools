# up-is-the-human-start-command

- State: abandoned
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="Startup includes human authorization and process ownership, so incorrect routing can grant authority or disturb live supervision; reuse the existing owners, cover every supported runtime, and prevent recurring failed startup and recovery attempts."
- Tier: 3
- Intent: What: one plain command for a person to start the metasystem, and a clearly separate name for the agent-only startup step. Most of this is done by the new command set: system start is the person's command, session start is the agent's, and a signed-in browser counts as the person's terminal. Left: the old up name still works with no stated policy, and nothing proves that health and status read the same runner record (on 2026-09-10 status said the steward was running while health said it was dead). Why: confusing start commands and two views that disagree cost Wido interruptions and wrong diagnoses. Pros: a clean start story and one truth about whether the steward runs. Cons: removing up may break old scripts or habits; a compatibility notice or alias has to be decided.
- Origin: human
- Next step: Next: decide and apply one policy for the old up name (remove it, or keep it as a documented alias that points to the new command), and add a test where health and status are read after a start and must agree on the steward runner. Done when: up either refuses with the new command named or is documented as an alias, and the test passes on main.
- OpenedAt: 2026-09-10T06:03:24Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-10-02T21:01:05Z revision=5 opid=JQ930JFESK0CDEMC3EWY3R2BWQ-m1e-9c612d71 because=Superseded (audit 2026-10-02): system start is the human command; up is an internal entrypoint

History:
- 2026-09-10T06:03:24Z GFGTDQ5RH0TX7NY6KH8K03NBR9-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=up-is-the-human-start-command
- 2026-09-10T08:41:39Z 53XBY7GE1E5YMJJ867GGJ3H5RQ-m1d-a38bcdde edit actor=m1d+main-1788941004-20871-6e7a43 targets=up-is-the-human-start-command
- 2026-09-10T12:29:18Z CTWCDJ8FM7G01P9364JYKKMDDE-m1d-8651d169 edit actor=m1d+main-1788941004-20871-67f3a0 targets=up-is-the-human-start-command
- 2026-09-30T18:54:53Z Z26PHWQQSN57YXESEYR6ZK5Q75-ui-bc2fda53 edit actor=human:Wido targets=up-is-the-human-start-command
- 2026-10-02T21:01:05Z JQ930JFESK0CDEMC3EWY3R2BWQ-m1e-9c612d71 abandon actor=human:Wido targets=up-is-the-human-start-command reason=Superseded (audit 2026-10-02): system start is the human command; up is an internal entrypoint
Integrity: sha256=9eeb6614d3a94e4e5403e3dd6841a8ad3a2a8dc6d89839840f092983d64ec1c3
