# burn-without-delivery-tripwire

- State: queued
- Intent: The case-study day's class 1 (Wido 2026-08-25): hours and tokens burned with no product landed, noticed only by the human. The ledger already holds every fact needed - claim time, structured budget, landing receipts - but nothing compares them LIVE. ESCALATED TO HIGHEST PRIORITY by Wido 2026-09-01 after m0's 8-hour idle night (specimen on this goal): 'I need this to be fixed before you do anything else... proven with tests.' SCOPE BOUNDARY for fleet coordination (no duplicate work): this goal builds the DETECTION PRIMITIVE ONLY - a steward tick check per machine-held claim raising a steward alert through the existing alert/notify machinery when (a) a job under the claim is terminal-failed/process-lost with no landing receipt for the goal since that failure, or (b) the claim is older than 1.5x the slice norm with jobs reserved but zero landing receipts since claim. Notification only, no kill authority (the steward-watch relay delegation's bound). watch-verb (unclaimed) later CONSUMES these alerts to act on stalls; alert-escalation-channel (m3, in flight) later CARRIES them to Wido externally; ledger-attention (m2, in flight) is ledger-change noticing, disjoint. m0 builds only the primitive.
- Origin: main
- Next step: NEW SPECIMEN (m0, night of 2026-08-31 to 09-01): m0 held two-bars-for-changes and idled ~8 hours - its revision round died at 22:44 (runtime death mid-return) and the orchestrator's completion watch had been started as a detached shell process instead of the harness-tracked mechanism, so nothing woke the loop; zero landings from m0 between 22:35 and 06:40 while every other machine produced. The claim-time/landing-receipt comparison this goal proposes would have fired within the hour. Cause honestly split: one orchestrator conduct error (untracked watch - m0 now uses only tracked watches) and one machinery gap (a died delegate round notifies nobody; the steward saw a failed job and stayed silent - see also suite-outcomes-as-steward-incidents).
- OpenedAt: 2026-08-30T06:25:49Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=360 activeJobLimit=1

History:
- 2026-08-30T06:25:49Z 3JRRXNVNQF9RJAWABDDYSM7E16-m2-bc1be9cb open actor=m2+mac-coordinator targets=burn-without-delivery-tripwire
- 2026-09-01T06:38:49Z ZA805CNYYCAJ872YYWHYVR3KXP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:50:59Z NPC7W26TA6CPYJVAPN8S95BYMK-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:51:10Z B2AAZBJDZJKNYY4HQF4WFN56GY-m0-c5dbf036 set-budget actor=human:Wido targets=burn-without-delivery-tripwire
Integrity: sha256=df677a3fc1321efbbc37c13573ae9671794c7b460a08c8c22df44e08ee7cb785
