# account-provenance

- State: queued
- Intent: ACCOUNT PROVENANCE (m0, re-opened on the fleet line at reconciliation): m0's Claude runtime signs into a different account than m1/m2 — a separate capacity pool, which is why m0 exists. m0's landings are stamped m0 while another account paid for them. The run record should carry the account identity alongside runtime and model so cost and capacity attribution survive in the record. Wido's word (decision-ask 2026-08-31): the string m0 stamps in every landing message until this lands is 'Wido@M0'.
- Origin: main
- Next step: Design where account identity enters the run record (session announcement vs launch record vs landing identity) and how it is proven rather than self-declared
- OpenedAt: 2026-08-31T19:09:19Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T19:09:19Z NHE37NCWZ1MATB7WFYK0PA87KP-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=account-provenance
- 2026-09-01T20:26:09Z JFCW3W9VE6G7C447JZD7CED29A-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
Integrity: sha256=27221db250e56395e040defe55dbc9a94635e40c00495a38b76d55dcfb172935
