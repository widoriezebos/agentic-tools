# burn-without-delivery-tripwire

- State: queued
- Intent: The case-study day's class 1 (Wido 2026-08-25): hours and tokens burned with no product landed, noticed only by the human. The ledger already holds every fact needed - claim time, structured budget, landing receipts - but nothing compares them LIVE. ESCALATED TO HIGHEST PRIORITY by Wido 2026-09-01 after m0's 8-hour idle night (specimen on this goal): 'I need this to be fixed before you do anything else... proven with tests.' SCOPE BOUNDARY for fleet coordination (no duplicate work): this goal builds the DETECTION PRIMITIVE ONLY - a steward tick check per machine-held claim raising a steward alert through the existing alert/notify machinery when (a) a job under the claim is terminal-failed/process-lost with no landing receipt for the goal since that failure, or (b) the claim is older than 1.5x the slice norm with jobs reserved but zero landing receipts since claim. Notification only, no kill authority (the steward-watch relay delegation's bound). watch-verb (unclaimed) later CONSUMES these alerts to act on stalls; alert-escalation-channel (m3, in flight) later CARRIES them to Wido externally; ledger-attention (m2, in flight) is ledger-change noticing, disjoint. m0 builds only the primitive.
- Origin: main
- Next step: DONE twice over, landed d252c785 + aad398c2 by m0 (account Wido@M0): detection primitive live fleet-wide at next engine rebuilds - failed-job-without-recovery alarms, and burn-without-delivery now measures each slice against ITS OWN reservation budget per Wido's correction (capDeadline + capMin/2; a 30-minute slice alarms at 46 minutes). Acting layer commissioned on watch-verb with Wido's order recorded.
- OpenedAt: 2026-08-30T06:25:49Z
- Revision: 11
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=360 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=8 at=2026-09-01T07:48:06Z

History:
- 2026-08-30T06:25:49Z 3JRRXNVNQF9RJAWABDDYSM7E16-m2-bc1be9cb open actor=m2+mac-coordinator targets=burn-without-delivery-tripwire
- 2026-09-01T06:38:49Z ZA805CNYYCAJ872YYWHYVR3KXP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:50:59Z NPC7W26TA6CPYJVAPN8S95BYMK-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:51:10Z B2AAZBJDZJKNYY4HQF4WFN56GY-m0-c5dbf036 set-budget actor=human:Wido targets=burn-without-delivery-tripwire
- 2026-09-01T06:51:13Z 5X20MDCT05M17XV60299TRY5HC-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:14:00Z J73M7W66438DHHKPS75JB83AMQ-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:14:04Z VV5RCCGSE4PHGR30STZM2PR2MQ-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:47:12Z 1K7JGRRP88S9SZX9WB6CN1C3MR-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:48:06Z PQ1MF5H6RJN58EAR71A30PZVGE-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T08:06:35Z E2TC9Q684V81TYSWJWA39ZDAJM-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T08:06:39Z F5E92C4B9HDH2X4BKXBN6GZB5Z-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
Integrity: sha256=f2e5996ce38847b9528bcc89ffa32db6ba77ef5a7708efc0087bd90d99d0bf75
