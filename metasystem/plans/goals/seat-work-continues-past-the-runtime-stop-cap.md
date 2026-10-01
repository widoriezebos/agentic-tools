# seat-work-continues-past-the-runtime-stop-cap

- State: queued
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a runaway continuation burns tokens but lands nothing destructive, and the spend fence already caps the day; novelty 3: no mechanism exists for continuing a seat past its host's cap, and the safe shape is the question; exposure 3: every seat on every runtime; accumulation 2: it touches the steward, the adapters and the stop path that three landings changed today"
- Tier: 3
- Intent: What: a seat keeps working while there is claimable work, even when the agent program itself (for example Claude Code) stops after its own fixed limit of refused stops. Most of this now exists: the steward starts a new agent when work waits and none is live, a person can stop it with one command, and a seat with nothing lawful to do goes quiet. What is left is proof that it really holds past that limit and on a second agent program. Why: Wido ruled that a seat must never stop while it can still lawfully work; the agent program's built-in stop limit is outside our control, so the metasystem itself has to carry the work on. Pros: no silent stops while work waits; the proof shows the guarantee is real, not assumed. Cons: the proof costs a long live run on two agent programs, and continued sessions spend tokens, so the spend limit must keep governing them.
- Origin: human
- Next step: Next: on a seat with the steward launch switched on, let one Claude session run into the agent program's own stop limit while work remains, and repeat the continuation check on a Codex seat; if both continue, conclude this goal as delivered by seat-works-without-a-person. Done when: the records show one Claude session that hit the stop limit followed by a steward-started session that carried on the same goal, and one Codex seat continued the same way.
- OpenedAt: 2026-09-13T17:29:59Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-13T17:29:59Z GYH8H8V7YEVQ9ZFY8MTTYB5QA8-m1e-c6925449 open actor=human:Wido targets=seat-work-continues-past-the-runtime-stop-cap
- 2026-09-14T10:32:26Z 5DGJN0VEP14Q1DV00S9TR3NTH0-m1c-69c9e454 edit actor=m1c+main-1789191340-90689-8975a9 targets=seat-work-continues-past-the-runtime-stop-cap
- 2026-09-30T18:53:23Z BJ14GQRTVZW8KSADK1N4H4NKKX-ui-bc2fda53 edit actor=human:Wido targets=seat-work-continues-past-the-runtime-stop-cap
Integrity: sha256=1e6035957e1b38c075a1528a67cdba69c819910cd670a298c598c26ebd50cb9f
