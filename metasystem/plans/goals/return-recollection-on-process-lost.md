# return-recollection-on-process-lost

- State: claimed
- Intent: A job that dies process-lost with a complete schema-valid return on disk gets its return recovered and adjudicated, marked recollected — D64's delivery lesson at the job level (2026-08-24 collateral: killed critic's finished findings sat undelivered in raw.out)
- Origin: human
- Next step: Appetite: 2h. On process-lost, the dispatch custody path runs the collect walk over the round dir before finalizing: a complete valid return flips the round to delivered-recollected (provenance noted), only a truly absent or invalid return stays failed. Acceptance: kill a critic after its raw.out lands — the job record carries the adjudicated findings.
- OpenedAt: 2026-08-24T13:24:18Z
- Revision: 4
- Labels: custody
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T03:51:32Z revision=4
- StopCapability: generation=4 revision=4 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-24T13:24:18Z VV6Z3A1846WEDT4KKRYEBWNVDD-m2-bc1be9cb open actor=human:wido targets=return-recollection-on-process-lost
- 2026-08-26T05:40:12Z 8G3VZJN0ABKSB9G17PHEFPHEE6-m2-bc1be9cb edit actor=m2+mac-coordinator targets=return-recollection-on-process-lost
- 2026-08-30T03:51:17Z 9ET7T4P95AVSZEA3BA39EYTAYM-m2-bc1be9cb set-budget actor=human:wido targets=return-recollection-on-process-lost
- 2026-08-30T03:51:32Z EKYJBF02STWP1N7CPCX6DH22H2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=return-recollection-on-process-lost
Integrity: sha256=74c5bd24826ee4f64713989223ccd2a6e5feb1638540c52a91c90da5aea892b4
