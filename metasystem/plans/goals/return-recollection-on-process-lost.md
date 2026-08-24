# return-recollection-on-process-lost

- State: queued
- Intent: A job that dies process-lost with a complete schema-valid return on disk gets its return recovered and adjudicated, marked recollected — D64's delivery lesson at the job level (2026-08-24 collateral: killed critic's finished findings sat undelivered in raw.out)
- Origin: human
- Next step: Appetite: 2h. On process-lost, the dispatch custody path runs the collect walk over the round dir before finalizing: a complete valid return flips the round to delivered-recollected (provenance noted), only a truly absent or invalid return stays failed. Acceptance: kill a critic after its raw.out lands — the job record carries the adjudicated findings.
- OpenedAt: 2026-08-24T13:24:18Z
- Revision: 1

History:
- 2026-08-24T13:24:18Z VV6Z3A1846WEDT4KKRYEBWNVDD-m2-bc1be9cb open actor=human:wido targets=return-recollection-on-process-lost
Integrity: sha256=b4420b49c6da567d029c161578493db0a6c2c999b341f4a336b88123d9ffef32
