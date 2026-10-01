# machine-remove

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="Severity 3: it deletes a checkout and can delete evidence; a defect loses unlanded work or another seat's evidence. Novelty 2: composes existing stop, claim, presence, archive and disposal owners. Exposure 2: a person's act on this fleet's machines. Accumulation 1: rare, one-off acts."
- Tier: 3
- Intent: What: metasystem machine remove NAME, the reverse of machine start: one previewable plan that stops the seat, releases its claims and enrollment, archives all its branches and unsaved work to archive refs locally and on the remote, and removes its checkout; with --obliterate it also disposes of the seat's evidence. Only a person may run it; it can resume after a crash and is safe to repeat. Designed (revision 2), not built. Why: today removing a seat means many manual steps, and a forgotten claim or leftover folder confuses the fleet (for example a stale machine still shows in machine list). Pros: seats can be retired cleanly, with nothing lost and nothing left stuck. Cons: a command that deletes checkouts and evidence must be very careful; it needs a design review and Wido's approval before any build.
- Origin: human
- Next step: Next: fold the evidence retention rules that have now landed on main (evidence show, export and dispose, among others dc5ee51c8) into the design as revision 3, have Astra read it once, then ask Wido to approve the build. Done when: design revision 3 is written with Astra's read recorded and the build approval question is with Wido.
- OpenedAt: 2026-09-28T20:13:03Z
- Revision: 2
- Labels: disk
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-28T20:13:03Z 8NAS9X9WMSWH604A397YT0XFNM-m1e-c6925449 open actor=human:Wido targets=machine-remove
- 2026-09-30T18:51:28Z PTHFTYJ3JZP19Z2QPEGWSFS3QW-ui-bc2fda53 edit actor=human:Wido targets=machine-remove
Integrity: sha256=83126c3b51299abae32087f86eeb88ad32dba9b39ceb45ae2aefb7087f9f6bad
