# account-provenance

- State: queued
- Intent: ACCOUNT PROVENANCE (m0, re-opened on the fleet line at reconciliation): m0's Claude runtime signs into a different account than m1/m2 — a separate capacity pool, which is why m0 exists. m0's landings are stamped m0 while another account paid for them. The run record should carry the account identity alongside runtime and model so cost and capacity attribution survive in the record. Wido's word (decision-ask 2026-08-31): the string m0 stamps in every landing message until this lands is 'Wido@M0'.
- Origin: main
- Next step: PARKED COLD-RESUMABLE (yields the claim slot to Wido's highest-priority seat-stop machinery): design landed (plans/account-provenance-design.md), round-1 critique returned eight material findings (records/misc/account-provenance-critique-r1.md). RESUME: fold all eight by id, closing critique, build, code critique — fits one box at the R-45 attempt count
- OpenedAt: 2026-08-31T19:09:19Z
- Revision: 6
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-02T05:51:12Z

History:
- 2026-08-31T19:09:19Z NHE37NCWZ1MATB7WFYK0PA87KP-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=account-provenance
- 2026-09-01T20:26:09Z JFCW3W9VE6G7C447JZD7CED29A-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T05:48:30Z FRWPFKA158EJ7XBMB71WV35FAN-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T05:51:12Z NN2V0KAD3WYWW1KPHEAX9PQJ0B-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T06:53:02Z M4RCPXH87FD8BTFP1GP8918PRA-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T06:53:06Z 01B6Z8XKR00BGT4FFD529DJ264-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
Integrity: sha256=530abf90da6af5b9036bdee5061cbd665c9f1ad962c4689e613ee3762c454c26
