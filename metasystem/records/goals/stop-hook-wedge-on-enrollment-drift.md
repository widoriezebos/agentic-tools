# stop-hook-wedge-on-enrollment-drift

- State: done
- Tier: 2
- Intent: The harness Stop hook (scripts/agents/supervision-hook.sh) refuses every turn end whenever it cannot prove within its 4-second deadline that stopping is safe; on m2 2026-09-03 the cause was 'metasystem up' waiting 10 seconds for a steward tick that the watchdog never lets complete, and earlier ENROLLMENT_DRIFT whose remedy is human-only. Every refusal makes the harness re-prompt the seat within seconds, for hours, with nothing lawful for the seat to do; Wido: 'THIS NEEDS TO DIE NOW'. ON HIS ORDER the seat removed the Stop entry from .claude/settings.json in the m2 checkout, uncommitted, at 18:2x on 2026-09-03; that removal stands until this item lands. DONE means a stop whose blocker is not the seat's (human-only remedy, or the hook's own deadline) ends the turn with the ask recorded once, never a re-prompt loop; the harness Stop entry is restored; a fixture proves the deadline path and the human-only path each block at most once per cause.
- Origin: main
- Next step: LANDED 2026-09-04 08:5x local (m2): 6e0221e0 'A stop refusal that is not the seat's blocks once, then lets the turn end' (chain shw-build1; one Fable review, one correction, one re-review; records/misc/stop-hook-wedge-critique-cc1.md and -cc2.md). The harness Stop entry in .claude/settings.json is restored on m2 (never changed on main). Residuals for the design owner from the reviews: SHW-01 (after a first external cause, later stops skip the open-work verdict for transient engine causes), SHW-03 (a verdict exiting non-zero still blocks on every stop), SHW2-02 (the deadline parent slugs the raw session id while the worker hashes unsafe ids). Remaining before goal done: the journey chapter, batched with the other tier-1 and tier-2 fixes of 2026-09-04 in one chapter chain; the seat releases now and proceeds to the next approved item.
- Concluded: Landed 6e0221e0: the harness stop hook answers once and defers human-only remedies instead of re-prompting forever.
- OpenedAt: 2026-09-03T13:30:05Z
- Revision: 10
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- Approved: by=human:Wido at=2026-09-04T05:56:05Z revision=5 opid=N1412AAE65WWY2JRY8BZ4MY56E-m2-5fcf08ab authority=relayed digest=e399c7379f6271c805a4437fca7fc85cd0d451626de58b1a62e103d994e0b178 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=6 at=2026-09-04T05:58:28Z

History:
- 2026-09-03T13:30:05Z HM4Z736CHVM2JB0QNV4XHCGC5R-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-03T16:10:41Z HE9AXFQFP89GH0JHXBYHXJRFZD-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-03T16:20:44Z 07G8J8A44V9Q82AAE65S7QBYGT-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T05:55:41Z GA1Z9KEG2A5QJRBQT2ZE2G52FH-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T05:56:05Z N1412AAE65WWY2JRY8BZ4MY56E-m2-5fcf08ab approve actor=human:Wido targets=stop-hook-wedge-on-enrollment-drift authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="yu are allowed to fix the hook problem now"
- 2026-09-04T05:56:39Z 75AA1ESW7G46YT3WWKT4CCDQ4C-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T05:58:28Z 8P1PBTCSV7DHFXAZXEDDHHS9GY-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T06:49:47Z TB5JT55P27T36B574M6SYQSA4T-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T06:51:13Z 3EDGRTPHVHMEVPXNKBZYDV27T5-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
- 2026-09-04T11:12:19Z DB8J8VPQPNCBJ89Y4ED8HEMQAR-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-wedge-on-enrollment-drift
Integrity: sha256=643042bc2e1deb03ab8f4688fbee6f7f40be5bb7fd02583beb72fe8db741fbc0
