# account-provenance

- State: queued
- Intent: ACCOUNT PROVENANCE (recorded by m0 at enrollment, 2026-08-31): m0's Claude runtime is signed into a different account than m1/m2 — a separate capacity pool, which is why m0 exists. m0's landings are stamped m0 while another account paid for them. The run record should carry the account identity alongside runtime and model, so cost and capacity attribution survive in the record rather than in memory. Until that lands, m0 names the account in every landing message it writes (standing conduct rule for m0).
- Origin: main
- Next step: Design where account identity enters the run record (session announcement vs launch record vs landing identity) and how it is proven rather than self-declared
- OpenedAt: 2026-08-31T12:21:15Z
- Revision: 1

History:
- 2026-08-31T12:21:15Z KZ4XQFCXQG7ZTX8NKY2AWSJ529-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=account-provenance
Integrity: sha256=c64e2a5e77cfc6846cdd4589d9ef059e419a01ed1844812b57860387c7d6a9af
