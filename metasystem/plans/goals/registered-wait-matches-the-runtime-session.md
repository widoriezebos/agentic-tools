# registered-wait-matches-the-runtime-session

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: the landed stop gate ignores every wait a seat actually registers from a shell, so the structural fix for seats stopping with work pending does not work in normal use; novelty 2: how a shell-registered watch learns the runtime session is not yet designed; exposure 3: every seat that waits on a delegate, proof or landing; accumulation 2: each ignored wait is a refused Stop or a seat that must poll"
- Tier: 2
- Intent: Member 3 of coordinator-wakes-on-events-not-polls (bf59fd0d) made the stop gate treat a registered wait as work in flight, but in normal use the gate never matches the wait. Evidence from m1b on 2026-09-14: at 18:48:20Z it registered metasystem job watch --root <checkout> --job member2-build-1-r28 --caller-pid $$ for a delegate job under its claimed goal; the waiter row job-member2-build-1-r28-30a7e1417b0a.json was schemaVersion 2, pending, with a live pid and session 'session-90295'; at 18:49:08Z its Stop was still refused with block source unwatched-work, naming that same job, and the same happened for round 27. Cause, read from internal/goal/turnverdict.go registeredWaitEligible: a row is eligible only if row.Session and row.RuntimeSession both equal the Stop's session id, which is the runtime's own session identifier from the Stop payload, while a watch registered from a shell records a pid-derived key ('session-90295') because the shell does not know the runtime session. So every wait a harness starts in the background is ineligible. DONE: a watch registered from a background shell under the seat's live main records the runtime session the Stop will present, or the gate resolves the waiter's session through the main's recorded lifecycle rather than a literal string comparison; a fixture registers a job watch from a child shell of the seat process and proves the next Stop treats it as work in flight; and a hostile wait from a different session or main is still refused.
- Origin: human
- Next step: Decide where the runtime session identity comes from for a shell-registered wait (the adapter exporting it into the seat's environment, or the gate resolving session through the main lifecycle) with a design page by a Claude Fable delegate, then build the fix and the child-shell fixture.
- OpenedAt: 2026-09-14T19:00:55Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T19:01:01Z revision=2 opid=XY5F3P5W42Z4P9KD41EFN4DQ2J-m1e-c6925449 authority=proven digest=61cffafd9a897b2d3f11b548d1f5d4e506711cd68a344ea85539d66f6fd1fcb6

History:
- 2026-09-14T19:00:55Z VW88ZP3E8RMAVCYCTGCXW0EZPQ-m1e-c6925449 open actor=human:Wido targets=registered-wait-matches-the-runtime-session reason=TierOverride: derived=3 set=2 why=one eligibility rule in the stop gate with a measured failure, below its tier-3 parent
- 2026-09-14T19:01:01Z XY5F3P5W42Z4P9KD41EFN4DQ2J-m1e-c6925449 approve actor=human:Wido targets=registered-wait-matches-the-runtime-session
Integrity: sha256=2109bd0e03333626944530ee98786ffee66743462aea32763d1f48d90f155531
