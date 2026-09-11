# landing-refuses-without-its-receipt-line

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a refused landing is retried with the line; novelty 1: land.sh already validates the commit; exposure 3: every landing; accumulation 1: one check"
- Tier: 2
- Intent: 48 of 67 code landings between 6 and 11 September carry no task receipt anywhere and only 7 carry it in the same commit (git-records.md section 2 of the delivery deep dive), although development/project-rules-local.md requires the receipt in the same commit as the work, so the retro's evidence base is gone. DONE means: land.sh refuses a code landing whose commit does not append a RECEIPT line for its goal to memory/receipts.log, names the missing line and the one command that writes it; records-only and receipt-only commits are exempt; proven by a fixture landing with and without the line. Goal 5 of plans/delivery-efficiency-plan.md. Tier 2 by Wido's choice 2026-09-11: one check on the landing script.
- Origin: human
- Next step: Read scripts/agents/land.sh and commit.sh and the memory/receipts.log format, add the check and its fixture, land.
- OpenedAt: 2026-09-11T15:50:31Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T15:50:31Z P8GF671DX0JP94MFS95CPM4H7G-m1-c6925449 open actor=human:Wido targets=landing-refuses-without-its-receipt-line reason=TierOverride: derived=3 set=2 why=Wido 2026-09-11: one check on the landing script; exposure alone does not make it tier 3, which goal 16 of the plan makes law
- 2026-09-11T15:50:38Z Y8KB1B921FG960ETKTTR5ATSR3-m1-c6925449 set-pin actor=human:Wido targets=landing-refuses-without-its-receipt-line
Integrity: sha256=db109bc056cefb55fdf246ad87164780715e75bc0d60c2221b29aa5aa3450aa1
