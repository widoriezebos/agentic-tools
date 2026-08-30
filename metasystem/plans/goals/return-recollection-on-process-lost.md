# return-recollection-on-process-lost

- State: queued
- Intent: A job that dies process-lost with a complete schema-valid return on disk gets its return recovered and adjudicated, marked recollected — D64's delivery lesson at the job level (2026-08-24 collateral: killed critic's finished findings sat undelivered in raw.out)
- Origin: human
- Next step: Appetite: 2h. On process-lost, the dispatch custody path runs the collect walk over the round dir before finalizing: a complete valid return flips the round to delivered-recollected (provenance noted), only a truly absent or invalid return stays failed. Acceptance: kill a critic after its raw.out lands — the job record carries the adjudicated findings.
- OpenedAt: 2026-08-24T13:24:18Z
- Revision: 3
- Labels: custody
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-24T13:24:18Z VV6Z3A1846WEDT4KKRYEBWNVDD-m2-bc1be9cb open actor=human:wido targets=return-recollection-on-process-lost
- 2026-08-26T05:40:12Z 8G3VZJN0ABKSB9G17PHEFPHEE6-m2-bc1be9cb edit actor=m2+mac-coordinator targets=return-recollection-on-process-lost
- 2026-08-30T03:51:17Z 9ET7T4P95AVSZEA3BA39EYTAYM-m2-bc1be9cb set-budget actor=human:wido targets=return-recollection-on-process-lost
Integrity: sha256=41339f84e7fcc4555b88a895a2c38e607056cc2662c3b5836f894bef859288fb
