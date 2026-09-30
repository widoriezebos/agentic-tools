# main-push-watcher

- State: parked
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Read-only observer with alerts; a false alert costs attention, a miss leaves a bypass unnoticed; no write authority"
- Tier: 2
- Intent: What: an independent watcher, a separate process or agent outside the landing lane, that checks every push to GitHub main against the lane's publication records and the seats' own landing records, and raises an alert for any push that no record explains. Why: the landing lane's guard rails prevent the landing agent's own mistakes but only detect code that pushes on its own under the same OS user (Wido's D8, 2026-09-30: not a second OS user now); an independent eye makes that detection reliable. Pros: catches a bypassed or forged push quickly without a second user account. Cons: detection after the fact, not prevention; one more process to keep running.
- Origin: main
- Next step: Next: after landing-lane-runtime-redesign lands, design the smallest watcher (which records it reads, where it runs, what an alert says, how it uses steward-acts-on-behaviour-patterns alerts). Done when: a push to main with no matching lane or seat landing record raises one alert within minutes, and ordinary landings raise none.
- OpenedAt: 2026-09-30T19:34:04Z
- Revision: 1
- BlockedBy: landing-lane-runtime-redesign
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Parked: by=human:wido at=2026-09-30T19:34:04Z blocker=landing-lane-runtime-redesign because=blocked by landing-lane-runtime-redesign; returns when it is done

History:
- 2026-09-30T19:34:04Z 113TB633Q3T7V32WB1YPMM94EG-m1e-b6a4eb0a open actor=human:wido targets=main-push-watcher,landing-lane-runtime-redesign
Integrity: sha256=b1014b982b0fae49190e779b2fb8dcfa752beb07056489907b063e6a81845a39
