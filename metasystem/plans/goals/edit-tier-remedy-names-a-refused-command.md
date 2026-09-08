# edit-tier-remedy-names-a-refused-command

- State: queued
- Priority: 3
- Sequence: 5
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a human loses one round trip; novelty 1: message text and one flag rule; exposure 1: only humans re-tiering an approved goal; accumulation 1: repeats on each re-tier"
- Tier: 1
- Intent: goal edit's refusal on an approved goal says unapprove it, edit --tier, then approve it, but goal edit --tier alone is refused with answer the four questions, and a tier above the derived one also needs --why; the human ran the printed remedy 2026-09-06 20:02Z, the tier edit was refused, and the goal was re-approved unchanged. Also unneeded here: the review-round limit is counted per critic chain, so a fresh critic chain never needs a higher tier. DONE means the remedy prints a command the CLI accepts, or edit --tier alone is admitted when the risk record already stands, with a fixture
- Origin: main
- Next step: decide between admitting edit --tier alone against the standing risk record or printing the full command with --risk --basis --why; fixture; grep the remedy text in internal/goal/verbs.go
- OpenedAt: 2026-09-06T20:05:11Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T20:05:11Z 1VZ8G0MAHDCA3MPVD59YKGSZMN-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=edit-tier-remedy-names-a-refused-command
- 2026-09-08T15:59:22Z GNNH301B3DN9BQJD3A641D76CC-m1-7cd0bd60 set-priority actor=human:Wido targets=edit-tier-remedy-names-a-refused-command reason=priority-order subject=edit-tier-remedy-names-a-refused-command from=unranked to=3:5 requested-sequence=5
Integrity: sha256=f4a803c4da46ccb90c8bc75f623067d4f5712ba676597c372f561d2bc16daa97
