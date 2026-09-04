# stop-hook-wedge-on-enrollment-drift

- State: queued
- Tier: 2
- Intent: The harness Stop hook (scripts/agents/supervision-hook.sh) refuses every turn end whenever it cannot prove within its 4-second deadline that stopping is safe; on m2 2026-09-03 the cause was 'metasystem up' waiting 10 seconds for a steward tick that the watchdog never lets complete, and earlier ENROLLMENT_DRIFT whose remedy is human-only. Every refusal makes the harness re-prompt the seat within seconds, for hours, with nothing lawful for the seat to do; Wido: 'THIS NEEDS TO DIE NOW'. ON HIS ORDER the seat removed the Stop entry from .claude/settings.json in the m2 checkout, uncommitted, at 18:2x on 2026-09-03; that removal stands until this item lands. DONE means a stop whose blocker is not the seat's (human-only remedy, or the hook's own deadline) ends the turn with the ask recorded once, never a re-prompt loop; the harness Stop entry is restored; a fixture proves the deadline path and the human-only path each block at most once per cause.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside the existing hook and its up call): build plus one code review, no design round; box 4h/6/240m/1. Waits for human approval for execution. Specimens: (1) this seat's takeover turn of 2026-09-03, six consecutive stop refusals with the same human-only cause after the handoff recorded the ask; (2) the same evening, once the steward was armed but its first tick could not complete (goal up-kills-runner-before-first-tick), the hook refused every turn end with 'Stop deadline expired before a safe turn verdict' for over an hour, re-prompting the seat every few seconds; Wido, verbatim: 'I see an insanely long ... which is not helpful' and 'If this is metasystem behaviour we need to change it'. The re-prompt loop is the hook's, not the seat's: a refusal whose remedy is not the seat's must end the turn with the ask recorded, once.
- OpenedAt: 2026-09-03T13:30:05Z
- Revision: 4
- Labels: robustness

History:
- 2026-09-03T13:30:05Z HM4Z736CHVM2JB0QNV4XHCGC5R-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-03T16:10:41Z HE9AXFQFP89GH0JHXBYHXJRFZD-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-03T16:20:44Z 07G8J8A44V9Q82AAE65S7QBYGT-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T05:55:41Z GA1Z9KEG2A5QJRBQT2ZE2G52FH-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
Integrity: sha256=6b8081598b09b650fc438c22f2a9779028af9137a00d3f6719251d4622087cde
