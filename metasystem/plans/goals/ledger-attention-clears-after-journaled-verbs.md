# ledger-attention-clears-after-journaled-verbs

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a false dead health signal on every stop report hides real deadness and trains seats to ignore it; novelty 2: cause not yet diagnosed; exposure 3: every seat's stop-status reads this health; accumulation 1: one verdict"
- Tier: 2
- Intent: On m1b the steward's ledger-attention health stays dead for over 90 minutes, reporting 'the shared ledger moved to <tip> ... and is unexamined past 30m'. Over that time the seat ran many journaling goal verbs (goal edit, claim, set-budget) at tips newer than the moved tip. The steward pass is alive: artifacts/agents/steward/ledger-attention.json shows lastAttemptAt within minutes and lastOutcome current. But its examinedTip stays behind remoteTip and journalReady is unset, and clearLedgerAttentionFromJournal in internal/steward/ledgerattention.go returns early unless JournalReady is true. So a seat that examines the tip is never recognised, and every stop-status report shows a false dead role. Observed 2026-09-15 by m1b. DONE: a journaling goal verb at or after the moved tip clears the dead verdict on the next steward pass, proven by a steward test that moves the remote, journals one verb and expects ledger-attention alive; the cause of the unset journalReady is named.
- Origin: human
- Next step: Diagnose first: trace when JournalReady is set true (only when the remote tip changes, with a baseline of journal ids) and when it is reset to false, against this seat's state file, and name why a later journal entry with a descendant fetched oid never clears the stale examinedTip. Then fix and add the steward test.
- OpenedAt: 2026-09-15T05:42:51Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T05:42:51Z Q4M6V96N3FAGXR2HZ2HBHQBSBH-m1e-c6925449 open actor=human:Wido targets=ledger-attention-clears-after-journaled-verbs
Integrity: sha256=8158618f5f39bd4245fc64cc5a0ea8669f4d4d54790428f0b97ca0d089dd5534
